package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/busca"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/triagem"
)

const limiteDaBusca = 10

// Prazos da busca. A coleta (os sites) tem o prazo dela, e a triagem não divide esse prazo: cada vaga tem o
// próprio, e a rodada inteira tem um teto. Com a IA levando de 20 a 80 s por vaga (Claude Code, Haiku), um prazo
// único de 3 minutos para a coleta e as 10 triagens cortava a rodada depois de 3 ou 4 vagas, e a tela pedia para
// conferir a conexão sem a conexão ter problema. São variáveis só para os testes encurtarem.
var (
	prazoDaColeta = 3 * time.Minute
	// Abaixo dos 120 s da própria CLI e do cliente HTTP da IA, para o prazo da vaga vencer primeiro e a vaga
	// lenta ficar pendente (sem parar a rodada).
	prazoPorVaga   = 110 * time.Second
	prazoDaTriagem = 12 * time.Minute
)

const (
	msgRodadaSemTempo = "A rodada usou o tempo dela antes de avaliar todas as vagas. Clique em Retomar avaliação das vagas para continuar de onde parou."
	msgVagaLenta      = "Uma ou mais vagas ficaram pendentes porque a IA demorou demais para responder. Elas entram na próxima rodada."
)

func assinaturaDeBusca(perfil Objeto) string {
	dados := Objeto{}
	for _, campo := range []string{"cargo", "titulosEquivalentes", "paisTrabalho", "cidadeTrabalho", "foco", "filtros", "contratos", "stackPrincipal", "stackSecundaria", "ingles", "pretensaoUSD", "pretensaoCLT", "pretensaoPJ", "experienciaAnos", "formacao", "cidade"} {
		dados[campo] = perfil[campo]
	}
	// Mantenha a assinatura de quem ainda usa o perfil anterior à faixa mensal.
	if faixa := perfil["faixaSalarial"]; faixa != nil {
		dados["faixaSalarial"] = faixa
	}
	b, _ := json.Marshal(dados)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

func puloDaBusca(e dominio.Estado, v Objeto) bool {
	if v["etapa"] != "pulada" || v["eventoPuloBusca"] == nil {
		return false
	}
	for i := len(e.Eventos) - 1; i >= 0; i-- {
		if e.Eventos[i]["vagaId"] == v["id"] {
			return e.Eventos[i]["id"] == v["eventoPuloBusca"]
		}
	}
	return false
}

type balancoBusca struct {
	perfilBuscaID string
	Encontradas   int            `json:"encontradas"`
	Novas         int            `json:"novas"`
	Grupos        map[string]int `json:"grupos"`
	Puladas       int            `json:"puladas"`
	Pendentes     int            `json:"pendentes"`
	Avisos        []string       `json:"avisos"`
}

func aguardandoTriagem(v Objeto) Objeto {
	v = maps.Clone(v)
	v["triagemBusca"] = "pendente"
	for _, campo := range []string{"aderencia", "triagemDecisao", "motivoClaude", "triagemConferir", "triagemMotivoPulo", "radarVersao", "radarClasse", "radarMotivo"} {
		delete(v, campo)
	}
	return v
}

// Salva as novas antes de chamar o provedor: fechar o app ou perder a conexão
// deixa uma vaga identificável para a rodada seguinte, sem gerar outra cópia.
func (a *API) prepararBusca(ctx context.Context, coletadas []Objeto, assinatura string, r *balancoBusca) ([]Objeto, error) {
	var lote []Objeto
	err := a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		if err := conferirPerfilBusca(e, r.perfilBuscaID); err != nil {
			return e, err
		}
		if assinaturaDeBusca(e.Perfil) != assinatura {
			return e, falha(409, "o perfil mudou; inicie outra busca")
		}
		ativo := e.PerfilBuscaAtivo()
		publicadas := map[string]Objeto{}
		var diretas []Objeto
		for _, v := range coletadas {
			publicadas[dominio.Texto(v["id"])] = v
			if v["fonteBusca"] == "Gupy" {
				diretas = append(diretas, v)
			}
		}
		var elegiveis []Objeto
		var existentes []Objeto
		agora := a.relogio()
		enfileirar := func(i int, v Objeto) {
			v = aguardandoTriagem(v)
			if v["triagemPerfil"] != assinatura {
				delete(v, "radarFalhasIndividuais")
			}
			classe, motivo := triagem.QualificarRadar(e.Perfil, v, agora)
			v["radarClasse"], v["radarMotivo"] = classe, motivo
			e.Vagas[i] = v
			elegiveis = append(elegiveis, v)
		}
		for i, v := range e.Vagas {
			if dominio.DonoDaBusca(v) != ativo {
				if v["etapa"] != "descoberta" && v["etapa"] != "pulada" {
					existentes = append(existentes, v)
				}
				continue
			}
			existentes = append(existentes, v)
			if v["etapa"] != "descoberta" && !puloDaBusca(e, v) {
				continue
			}
			id := dominio.Texto(v["anuncioId"])
			if id == "" {
				id = dominio.Texto(v["id"])
			}
			publica := publicadas[id]
			// A fonte oficial pode surgir em outra rodada com um ID próprio.
			// Notas particulares e vagas já movidas pela pessoa ficam fora da promoção.
			if v["triagemBusca"] != nil && v["fonteBusca"] != "Gupy" {
				if i := slices.IndexFunc(diretas, func(d Objeto) bool { return busca.MesmoAnuncio(v, d) }); i >= 0 {
					publica = diretas[i]
				}
			}
			if v["triagemBusca"] == nil && publica == nil {
				continue
			} // nota particular ou histórico importado não é entrada pública
			mudouFonte := false
			if publica != nil {
				v = maps.Clone(v)
				if plataforma := dominio.Texto(publica["plataforma"]); plataforma != "" && plataforma != v["plataforma"] {
					v["plataforma"] = plataforma
					mudouFonte = true
				}
				for _, k := range []string{"titulo", "empresa", "link", "fonteBusca", "fonteURL", "descricao", "publicadaEm", "expiraEm", "paisesElegiveis", "remotoMundial", "nivel", "modelo", "mercado", "categorias", "local", "contrato", "periodoSalario", "moeda", "salarioMin", "salarioMax", "salarioUSDMin", "salarioUSDMax", "salario", "stack", "intermediario"} {
					antes, _ := json.Marshal(v[k])
					depois, _ := json.Marshal(publica[k])
					if !bytes.Equal(antes, depois) {
						mudouFonte = true
					}
					if valor, ok := publica[k]; ok {
						v[k] = valor
					} else {
						delete(v, k)
					}
				}
				v["anuncioId"], v["perfilBuscaId"] = publica["id"], ativo
				if mudouFonte {
					delete(v, "radarFalhasIndividuais")
				}
				e.Vagas[i] = v
			}
			classe, _ := triagem.QualificarRadar(e.Perfil, v, agora)
			versao, _ := dominio.NumeroDe(v["radarVersao"])
			if v["triagemPerfil"] != assinatura || versao != 1 || v["triagemBusca"] == "pendente" || classe != v["radarClasse"] || mudouFonte {
				enfileirar(i, v)
			}
		}
		for _, raw := range coletadas {
			dados := maps.Clone(raw)
			id := dominio.Texto(dados["id"])
			dados["anuncioId"], dados["perfilBuscaId"] = id, ativo
			if ativo != dominio.PerfilOriginal {
				dados["id"] = ativo + "-" + id
			}
			dup := slices.ContainsFunc(existentes, func(v Objeto) bool { return busca.MesmoAnuncio(v, dados) })
			if dup || dominio.AnuncioReservado(e, dados) != nil {
				continue
			}
			dados["triagemBusca"] = "pendente"
			novo, v, err := dominio.AdicionarVaga(e, dados, a.ctx(e, "claude"), true)
			if err != nil {
				var d *dominio.ErroDuplicata
				var invalida *dominio.ErroValidacao
				if errors.As(err, &d) || errors.As(err, &invalida) {
					continue
				}
				return e, err
			}
			e = novo
			existentes = append(existentes, v)
			r.Novas++
			enfileirar(len(e.Vagas)-1, v)
		}
		// Não gaste o orçamento da IA com descartes. Salve toda a coleta antes da
		// primeira chamada: uma falha ou o fim da página não perde o restante.
		prioridade := func(v Objeto) int {
			if v["radarClasse"] == "excluir" {
				return 0
			}
			if v["radarClasse"] == "forte" {
				return 1
			}
			return 2
		}
		sort.SliceStable(elegiveis, func(i, j int) bool {
			a, b := elegiveis[i], elegiveis[j]
			// Entre vagas que requerem IA, dê a todas uma primeira tentativa
			// antes de repetir erros individuais que consomem o mesmo orçamento.
			if a["radarClasse"] != "excluir" && b["radarClasse"] != "excluir" {
				fa, _ := dominio.NumeroDe(a["radarFalhasIndividuais"])
				fb, _ := dominio.NumeroDe(b["radarFalhasIndividuais"])
				if fa != fb {
					return fa < fb
				}
			}
			if prioridade(a) != prioridade(b) {
				return prioridade(a) < prioridade(b)
			}
			return dominio.Texto(a["publicadaEm"]) > dominio.Texto(b["publicadaEm"])
		})
		orcam := 0
		for _, v := range elegiveis {
			if v["radarClasse"] != "excluir" {
				if orcam == limiteDaBusca {
					r.Pendentes++
					continue
				}
				orcam++
			}
			lote = append(lote, v)
		}
		return e, nil
	})
	return lote, err
}

func (a *API) salvarParecer(ctx context.Context, original Objeto, assinatura string, nota triagem.Resultado) (bool, error) {
	aplicado := false
	err := a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		if err := conferirPerfilBusca(e, dominio.DonoDaBusca(original)); err != nil {
			return e, err
		}
		if assinaturaDeBusca(e.Perfil) != assinatura {
			return e, falha(409, "o perfil mudou durante a triagem; busque novamente")
		}
		for i, atual := range e.Vagas {
			if atual["id"] != original["id"] {
				continue
			}
			// A pessoa ou outra janela pode ter movido/editado a vaga durante a chamada à IA.
			// SQLite devolve listas JSON como []any mesmo se a fonte usou []string.
			aJSON, err := json.Marshal(atual)
			if err != nil {
				return e, err
			}
			bJSON, err := json.Marshal(original)
			if err != nil {
				return e, err
			}
			if !bytes.Equal(aJSON, bJSON) {
				return e, nil
			}
			if nota.Estado == "pendente" {
				v := maps.Clone(atual)
				falhas, _ := dominio.NumeroDe(v["radarFalhasIndividuais"])
				v["radarFalhasIndividuais"] = falhas + 1
				v["triagemPerfil"] = assinatura
				e.Vagas[i], aplicado = v, true
				break
			}
			etapa := "descoberta"
			extras := Objeto{}
			if nota.Estado == "pulada" {
				etapa = "pulada"
				extras["motivo"] = nota.Motivo
			}
			if atual["etapa"] != etapa {
				var err error
				e, _, err = dominio.MoverVaga(e, atual["id"], etapa, extras, a.ctx(e, "claude"))
				if err != nil {
					return e, err
				}
			}
			v := maps.Clone(e.Vagas[i])
			delete(v, "radarFalhasIndividuais")
			v["triagemPerfil"] = assinatura
			v["radarVersao"] = 1
			v["triagemBusca"] = nota.Estado
			v["triagemDecisao"] = nota.Decisao
			v["motivoClaude"] = nota.Motivo
			if len(nota.Conferir) > 0 {
				v["triagemConferir"] = nota.Conferir
			} else {
				delete(v, "triagemConferir")
			}
			if nota.MotivoPulo != "" {
				v["triagemMotivoPulo"] = nota.MotivoPulo
			} else {
				delete(v, "triagemMotivoPulo")
			}
			delete(v, "aderencia")
			if nota.Aderencia != "" {
				v["aderencia"] = nota.Aderencia
			}
			if etapa == "pulada" {
				v["motivo"] = nota.Motivo
				if atual["etapa"] != "pulada" {
					v["eventoPuloBusca"] = e.Eventos[len(e.Eventos)-1]["id"]
				}
			} else {
				delete(v, "eventoPuloBusca")
			}
			if problemas := dominio.ValidarVaga(v); len(problemas) > 0 {
				return e, &dominio.ErroValidacao{Erros: problemas}
			}
			e.Vagas[i] = v
			aplicado = true
			break
		}
		return e, nil
	})
	return aplicado, err
}

func (a *API) buscarETriar(ctx context.Context, esperado string) (Resposta, error) {
	if !a.buscaEmCurso.TryLock() {
		return Resposta{}, falha(409, "já existe uma busca em andamento")
	}
	defer a.buscaEmCurso.Unlock()
	e, err := a.arm.Carregar(ctx)
	if err != nil {
		return Resposta{}, err
	}
	if err := conferirPerfilBusca(e, esperado); err != nil {
		return Resposta{}, err
	}
	if cargo := strings.TrimSpace(dominio.Texto(e.Perfil["cargo"])); cargo == "" || dominio.Normalizar(cargo) == dominio.Normalizar(e.Perfil["nome"]) {
		return Resposta{}, falha(400, "informe o cargo no Perfil antes de buscar")
	}
	cfg, err := a.configIA(ctx)
	if err != nil {
		return Resposta{}, err
	}
	if cfg.Rotas["padrao"].Provedor == "" && cfg.Rotas["triarVaga"].Provedor == "" {
		return Resposta{}, falha(409, "conecte uma IA para buscar e classificar as vagas")
	}
	motor, err := a.novaIA(ctx)
	if err != nil {
		return Resposta{}, err
	}
	prazoColeta, cancelarColeta := context.WithTimeout(ctx, prazoDaColeta)
	coleta := a.consultarVagas(prazoColeta, e.Perfil)
	cancelarColeta()
	if ctx.Err() != nil {
		return Resposta{}, ctx.Err()
	}
	r := balancoBusca{perfilBuscaID: e.PerfilBuscaAtivo(), Encontradas: len(coleta.Vagas), Grupos: map[string]int{"A": 0, "B": 0, "C": 0}, Avisos: append([]string{}, coleta.Avisos...)}
	assinatura := assinaturaDeBusca(e.Perfil)
	lote, err := a.prepararBusca(ctx, coleta.Vagas, assinatura, &r)
	if err != nil {
		return Resposta{}, err
	}
	rodada, cancelarRodada := context.WithTimeout(ctx, prazoDaTriagem)
	defer cancelarRodada()
	falhaIndividual, vagaLenta := false, false
	for i, v := range lote {
		var nota triagem.Resultado
		var err error
		venceuDaVaga := false
		if v["radarClasse"] == "excluir" {
			nota = triagem.Resultado{Estado: "pulada", Decisao: "pular", Motivo: dominio.Texto(v["radarMotivo"]), MotivoPulo: dominio.Texto(v["radarMotivo"])}
		} else {
			porVaga, cancelarVaga := context.WithTimeout(rodada, prazoPorVaga)
			nota, err = triagem.AvaliarRadar(porVaga, motor, e.Perfil, v)
			venceuDaVaga = errors.Is(porVaga.Err(), context.DeadlineExceeded) && rodada.Err() == nil
			cancelarVaga()
			if nota.Aderencia == "A" && v["radarClasse"] != "forte" {
				nota.Aderencia = "B"
			}
			if nota.Decisao == "pular" && nota.Aderencia != "" {
				nota.Aderencia = "C"
			}
		}
		if ctx.Err() != nil {
			return Resposta{}, ctx.Err()
		}
		if err != nil {
			if rodada.Err() != nil {
				// O teto da rodada venceu: as que faltam ficam para o Retomar, sem culpar a conexão.
				r.Avisos = append(r.Avisos, msgRodadaSemTempo)
				r.Pendentes += len(lote) - i
				break
			}
			var daIA *ia.ErroIA
			if venceuDaVaga || errors.As(err, &daIA) && (daIA.Tipo == "regra" || daIA.Tipo == "esquema" || daIA.Tipo == "entrada") {
				aplicado, err := a.salvarParecer(ctx, v, assinatura, triagem.Resultado{Estado: "pendente"})
				if err != nil {
					return Resposta{}, err
				}
				if aplicado {
					r.Pendentes++
					if venceuDaVaga {
						vagaLenta = true
					} else {
						falhaIndividual = true
					}
				}
				continue
			}
			r.Avisos = append(r.Avisos, "A IA não concluiu a triagem. Confira a conexão e busque novamente para retomar.")
			r.Pendentes += len(lote) - i
			break
		}
		aplicado, err := a.salvarParecer(ctx, v, assinatura, nota)
		if err != nil {
			return Resposta{}, err
		}
		if !aplicado {
			continue
		}
		if nota.Estado == "pulada" {
			r.Puladas++
		} else {
			r.Grupos[nota.Aderencia]++
		}
	}
	if falhaIndividual {
		r.Avisos = append(r.Avisos, "Uma ou mais vagas ficaram pendentes porque a IA não devolveu uma avaliação válida.")
	}
	if vagaLenta {
		r.Avisos = append(r.Avisos, msgVagaLenta)
	}
	err = a.gravar(ctx, func(estado dominio.Estado) (dominio.Estado, error) {
		if err := conferirPerfilBusca(estado, r.perfilBuscaID); err != nil {
			return estado, err
		}
		if assinaturaDeBusca(estado.Perfil) != assinatura {
			return estado, falha(409, "o perfil mudou durante a busca")
		}
		numero := 0
		for _, rodada := range estado.Rodadas {
			if n, ok := dominio.NumeroDe(rodada["n"]); ok && int(n) > numero {
				numero = int(n)
			}
		}
		fontes := []any{}
		nomes := map[string]bool{}
		for _, v := range coleta.Vagas {
			if nome, ok := v["fonteBusca"].(string); ok && nome != "" && !nomes[nome] {
				fontes = append(fontes, nome)
				nomes[nome] = true
			}
		}
		resumo := fmt.Sprintf("%d A · %d B · %d C · %d pendentes", r.Grupos["A"], r.Grupos["B"], r.Grupos["C"], r.Pendentes)
		if len(r.Avisos) > 0 {
			resumo += ". " + strings.Join(r.Avisos, " ")
		}
		estado.Rodadas = append(estado.Rodadas, Objeto{"n": numero + 1, "perfilBuscaId": r.perfilBuscaID, "data": a.hoje(), "fontes": fontes, "encontradas": r.Encontradas, "puladas": r.Puladas, "resumo": resumo})
		return estado, nil
	})
	if err != nil {
		return Resposta{}, err
	}
	return ok(r)
}
