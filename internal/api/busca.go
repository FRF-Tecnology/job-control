package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/triagem"
)

const limiteDaBusca = 8

func assinaturaDeBusca(perfil Objeto) string {
	dados := Objeto{}
	for _, campo := range []string{"cargo", "foco", "filtros", "contratos", "stackPrincipal", "stackSecundaria", "ingles", "pretensaoUSD", "pretensaoCLT", "pretensaoPJ", "experienciaAnos", "formacao", "cidade"} {
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
	Encontradas int            `json:"encontradas"`
	Novas       int            `json:"novas"`
	Grupos      map[string]int `json:"grupos"`
	Puladas     int            `json:"puladas"`
	Pendentes   int            `json:"pendentes"`
	Avisos      []string       `json:"avisos"`
}

func aguardandoTriagem(v Objeto) Objeto {
	v = maps.Clone(v)
	v["triagemBusca"] = "pendente"
	for _, campo := range []string{"aderencia", "triagemDecisao", "motivoClaude", "triagemConferir", "triagemMotivoPulo"} {
		delete(v, campo)
	}
	return v
}

// Salva as novas antes de chamar o provedor: fechar o app ou perder a conexão
// deixa uma vaga identificável para a rodada seguinte, sem gerar outra cópia.
func (a *API) prepararBusca(ctx context.Context, coletadas []Objeto, assinatura string, r *balancoBusca) ([]Objeto, error) {
	var lote []Objeto
	err := a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		if assinaturaDeBusca(e.Perfil) != assinatura {
			return e, falha(409, "o perfil mudou; inicie outra busca")
		}
		ids := make(map[string]bool, len(e.Vagas))
		publicadas := make(map[string]Objeto, len(coletadas))
		for _, v := range coletadas {
			publicadas[dominio.Texto(v["id"])] = v
		}
		for i, v := range e.Vagas {
			id := dominio.Texto(v["id"])
			ids[id] = true
			// Uma vaga importada por outros caminhos pode conter anotações particulares.
			// Só entra na rodada se a fonte voltou a publicar seu anúncio público.
			reencontrada := false
			if v["triagemBusca"] == nil {
				publica := publicadas[id]
				if publica == nil || v["etapa"] != "descoberta" {
					continue
				}
				v = maps.Clone(v)
				for _, k := range []string{"fonteBusca", "fonteURL", "descricao", "expiraEm", "nivel", "modelo", "mercado", "categorias", "local", "contrato", "periodoSalario", "moeda", "salarioMin", "salarioMax", "salarioUSDMin", "salarioUSDMax"} {
					if valor, ok := publica[k]; ok {
						v[k] = valor
					}
				}
				e.Vagas[i] = v
				reencontrada = true
			}
			if expira, err := time.Parse(time.RFC3339, dominio.Texto(v["expiraEm"])); err == nil && !expira.After(time.Now()) {
				if v["etapa"] != "descoberta" {
					continue
				}
				novo, atual, err := dominio.MoverVaga(e, v["id"], "pulada", Objeto{"motivo": "Vaga expirada na fonte."}, a.ctx(e, "claude"))
				if err != nil {
					return e, err
				}
				delete(atual, "aderencia")
				delete(atual, "eventoPuloBusca")
				atual["triagemBusca"] = "expirada"
				e = novo
				r.Puladas++
				continue
			}
			semGrupo := dominio.Texto(v["aderencia"]) == ""
			mudouPerfil := v["triagemPerfil"] != nil && v["triagemPerfil"] != assinatura
			if (v["etapa"] == "descoberta" && (semGrupo || mudouPerfil || reencontrada)) || (mudouPerfil && puloDaBusca(e, v)) {
				v = aguardandoTriagem(v)
				e.Vagas[i] = v
				if len(lote) < limiteDaBusca {
					lote = append(lote, v)
				} else {
					r.Pendentes++
				}
			}
		}
		for _, dados := range coletadas {
			if len(lote) >= limiteDaBusca {
				break
			}
			id := dominio.Texto(dados["id"])
			if ids[id] {
				continue
			}
			dados = maps.Clone(dados)
			dados["triagemBusca"] = "pendente"
			novo, v, err := dominio.AdicionarVaga(e, dados, a.ctx(e, "claude"), false)
			if err != nil {
				var dup *dominio.ErroDuplicata
				var invalida *dominio.ErroValidacao
				if errors.As(err, &dup) || errors.As(err, &invalida) {
					continue
				}
				return e, err
			}
			e = novo
			ids[id] = true
			lote = append(lote, v)
			r.Novas++
		}
		return e, nil
	})
	return lote, err
}

func (a *API) salvarParecer(ctx context.Context, original Objeto, assinatura string, nota triagem.Resultado) (bool, error) {
	aplicado := false
	err := a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
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
			v["triagemPerfil"] = assinatura
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

func (a *API) buscarETriar(ctx context.Context) (Resposta, error) {
	if !a.buscaEmCurso.TryLock() {
		return Resposta{}, falha(409, "já existe uma busca em andamento")
	}
	defer a.buscaEmCurso.Unlock()
	e, err := a.arm.Carregar(ctx)
	if err != nil {
		return Resposta{}, err
	}
	if strings.TrimSpace(dominio.Texto(e.Perfil["cargo"])) == "" {
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
	prazo, cancelar := context.WithTimeout(ctx, 3*time.Minute)
	defer cancelar()
	coleta := a.consultarVagas(prazo, e.Perfil)
	if ctx.Err() != nil {
		return Resposta{}, ctx.Err()
	}
	r := balancoBusca{Encontradas: len(coleta.Vagas), Grupos: map[string]int{"A": 0, "B": 0, "C": 0}, Avisos: append([]string{}, coleta.Avisos...)}
	assinatura := assinaturaDeBusca(e.Perfil)
	lote, err := a.prepararBusca(ctx, coleta.Vagas, assinatura, &r)
	if err != nil {
		return Resposta{}, err
	}
	falhaIndividual := false
	for i, v := range lote {
		nota, err := triagem.Avaliar(prazo, motor, e.Perfil, v)
		if ctx.Err() != nil {
			return Resposta{}, ctx.Err()
		}
		if err != nil {
			var daIA *ia.ErroIA
			if errors.As(err, &daIA) && (daIA.Tipo == "regra" || daIA.Tipo == "esquema" || daIA.Tipo == "entrada") {
				r.Pendentes++
				falhaIndividual = true
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
	err = a.gravar(ctx, func(estado dominio.Estado) (dominio.Estado, error) {
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
		estado.Rodadas = append(estado.Rodadas, Objeto{"n": numero + 1, "data": a.hoje(), "fontes": fontes, "encontradas": r.Encontradas, "puladas": r.Puladas, "resumo": resumo})
		return estado, nil
	})
	if err != nil {
		return Resposta{}, err
	}
	return ok(r)
}
