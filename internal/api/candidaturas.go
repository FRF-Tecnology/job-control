package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/curriculos"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Rotas da fila de candidaturas (P5, jc-a2y.6.2.2; contrato 0.4.8). Só a tela grava com autor "eu"; o assistente
// (CLI, MCP) recebe 403 nas ações só da pessoa. O envio (enviando, enviada, bloqueada, falhou, incerta) é do
// executor interno e não tem rota: nenhuma função daqui chama IniciarEnvio, MarcarEnviada, BloquearCandidatura,
// FalharCandidatura, MarcarIncertas, VencerAprovacoes, EncaminharExterna ou TornarIncerta (candidaturas_test.go confere).
//
// Log (M7): estas rotas não registram nada; o 500 genérico do http.go leva só método, caminho sem query e tipo do erro.

var (
	reCandidatura       = regexp.MustCompile(`^/candidaturas/([^/]+)$`)
	reCandidaturaAcao   = regexp.MustCompile(`^/candidaturas/([^/]+)/(conteudo|aprovar|revogar|cancelar|reabrir|concluir-manual)$`)
	reLimitePlataforma  = regexp.MustCompile(`^/candidaturas/limites/([^/]+)$`)
	erroCurriculoFora   = "conteudo.curriculo.arquivo: não está na pasta de currículos"
	erroCurriculoGrande = "conteudo.curriculo.arquivo: maior que 20 MB"
)

func (a *API) rotasCandidaturas(ctx context.Context, m, p string, q map[string]string, c Objeto, autor string) (Resposta, bool, error) {
	switch {
	case p == "/candidaturas/limites":
		if m != "GET" {
			return Resposta{}, false, nil
		}
		r, err := a.limitesCandidatura(ctx)
		return r, true, err

	case reLimitePlataforma.MatchString(p):
		if m != "PUT" {
			return Resposta{}, false, nil
		}
		plataforma, err := decodificar(reLimitePlataforma.FindStringSubmatch(p)[1])
		if err != nil {
			return Resposta{}, true, err
		}
		r, err := a.gravarLimite(ctx, plataforma, c, autor)
		return r, true, err

	case p == "/candidaturas" && m == "GET":
		r, err := a.listarCandidaturas(ctx, q)
		return r, true, err

	case p == "/candidaturas" && m == "POST":
		r, err := a.criarCandidatura(ctx, c, autor)
		return r, true, err

	case p == "/candidaturas/aprovar-lote" && m == "POST":
		r, err := a.aprovarLote(ctx, c, autor)
		return r, true, err

	case reCandidatura.MatchString(p) && m == "GET":
		id, err := decodificar(reCandidatura.FindStringSubmatch(p)[1])
		if err != nil {
			return Resposta{}, true, err
		}
		r, err := a.lerCandidatura(ctx, id)
		return r, true, err

	case reCandidaturaAcao.MatchString(p):
		g := reCandidaturaAcao.FindStringSubmatch(p)
		if (g[2] == "conteudo") != (m == "PUT") || (g[2] != "conteudo" && m != "POST") {
			return Resposta{}, false, nil
		}
		id, err := decodificar(g[1])
		if err != nil {
			return Resposta{}, true, err
		}
		r, err := a.acaoCandidatura(ctx, id, g[2], c, autor)
		return r, true, err
	}
	return Resposta{}, false, nil
}

// soCampos: corpo com campo fora do esquema dá 400 e nada é gravado (additionalProperties: false).
func soCampos(c Objeto, campos ...string) error {
	var erros []string
	for k := range c {
		if !contem(campos, k) {
			erros = append(erros, "campo não permitido: "+k)
		}
	}
	if len(erros) == 0 {
		return nil
	}
	sort.Strings(erros)
	return &dominio.ErroValidacao{Erros: erros}
}

func erroCampo(msg string) error { return &dominio.ErroValidacao{Erros: []string{msg}} }

// inteiro aceita só número JSON sem parte decimal.
func inteiro(v any) (int, bool) {
	f, ok := v.(float64)
	if !ok || f != math.Trunc(f) || math.Abs(f) > 1e9 {
		return 0, false
	}
	return int(f), true
}

// visao é a candidatura como sai na API: sem as tentativas (uso interno do limite diário).
func visao(o Objeto) Objeto {
	v := Objeto{}
	for k, x := range o {
		if k != "tentativas" {
			v[k] = x
		}
	}
	return v
}

func (a *API) listarCandidaturas(ctx context.Context, q map[string]string) (Resposta, error) {
	estado, filtra := q["estado"]
	if filtra && !contem(dominio.EstadosCandidatura, estado) {
		return Resposta{}, erroCampo("estado desconhecido: " + estado)
	}
	e, err := a.arm.Carregar(ctx)
	if err != nil {
		return Resposta{}, err
	}
	lista := []Objeto{}
	for _, o := range e.Candidaturas {
		if !filtra || dominio.Igual(o["estado"], estado) {
			lista = append(lista, visao(o))
		}
	}
	return ok(lista)
}

func (a *API) lerCandidatura(ctx context.Context, id string) (Resposta, error) {
	e, err := a.arm.Carregar(ctx)
	if err != nil {
		return Resposta{}, err
	}
	for _, o := range e.Candidaturas {
		if dominio.Igual(o["id"], id) {
			return ok(visao(o))
		}
	}
	return Resposta{}, &dominio.ErroNaoEncontrado{Msg: "candidatura não encontrada: " + id}
}

// prepararConteudo copia o conteúdo e põe o sha256 do currículo calculado aqui, do arquivo na pasta de currículos.
// O sha256 que vier no pedido é sempre descartado. Os demais campos o domínio confere (ValidarConteudo).
func (a *API) prepararConteudo(v any) (Objeto, error) {
	orig, ok := v.(Objeto)
	if !ok {
		return nil, erroCampo("conteudo: obrigatório (objeto)")
	}
	conteudo := Objeto{}
	for k, x := range orig {
		conteudo[k] = x
	}
	cvOrig, ok := orig["curriculo"].(Objeto)
	if !ok {
		return conteudo, nil
	}
	cv := Objeto{}
	for k, x := range cvOrig {
		if k != "sha256" {
			cv[k] = x
		}
	}
	conteudo["curriculo"] = cv
	nome, _ := cv["arquivo"].(string)
	if !dominio.NomeDeCurriculoValido(nome) {
		return conteudo, nil
	}
	sha, err := a.sha256Curriculo(nome)
	if err != nil {
		return nil, err
	}
	cv["sha256"] = sha
	return conteudo, nil
}

// sha256Curriculo lê o PDF da pasta de currículos (curriculos.Ler: arquivo comum, bem dentro da pasta, o mesmo
// depois de aberto, até 20 MB) e devolve o sha256.
func (a *API) sha256Curriculo(nome string) (string, error) {
	b, err := curriculos.Ler(a.pastaCurriculos(), nome)
	switch {
	case errors.Is(err, curriculos.ErrGrande):
		return "", erroCampo(erroCurriculoGrande)
	case errors.Is(err, curriculos.ErrFora):
		return "", erroCampo(erroCurriculoFora)
	case err != nil:
		return "", err
	}
	return curriculos.SHA256(b), nil
}

func (a *API) criarCandidatura(ctx context.Context, c Objeto, autor string) (Resposta, error) {
	if err := soCampos(c, "vagaId", "conteudo"); err != nil {
		return Resposta{}, err
	}
	vagaID, ok := c["vagaId"].(string)
	if !ok || vagaID == "" {
		return Resposta{}, erroCampo("vagaId: obrigatório")
	}
	if autor != "eu" { // antes de ler o currículo: o assistente não chega a tocar no arquivo
		return Resposta{}, &dominio.ErroSoPessoa{Msg: "só a pessoa pode criar uma candidatura"}
	}
	conteudo, err := a.prepararConteudo(c["conteudo"])
	if err != nil {
		return Resposta{}, err
	}
	agora := a.relogio()
	var res Objeto
	err = a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		novo, o, err := dominio.CriarCandidatura(e, vagaID, conteudo, agora, a.ctx(e, autor))
		res = o
		return novo, err
	})
	if err != nil {
		return Resposta{}, err
	}
	return criado(visao(res))
}

func (a *API) acaoCandidatura(ctx context.Context, id, acao string, c Objeto, autor string) (Resposta, error) {
	agora := a.relogio()
	var op func(e dominio.Estado, d dominio.Ctx) (dominio.Estado, Objeto, error)
	switch acao {
	case "conteudo":
		if err := soCampos(c, "conteudo", "versao"); err != nil {
			return Resposta{}, err
		}
		versao, ok := inteiro(c["versao"])
		if !ok {
			return Resposta{}, erroCampo("versao: número inteiro obrigatório")
		}
		if autor != "eu" {
			return Resposta{}, &dominio.ErroSoPessoa{Msg: "só a pessoa pode editar uma candidatura"}
		}
		conteudo, err := a.prepararConteudo(c["conteudo"])
		if err != nil {
			return Resposta{}, err
		}
		op = func(e dominio.Estado, d dominio.Ctx) (dominio.Estado, Objeto, error) {
			return dominio.EditarConteudo(e, id, conteudo, versao, agora, d)
		}
	case "aprovar":
		if err := soCampos(c, "impressao"); err != nil {
			return Resposta{}, err
		}
		impressao, ok := c["impressao"].(string)
		if !ok || impressao == "" {
			return Resposta{}, erroCampo("impressao: obrigatória")
		}
		limites, err := a.arm.LimitesCandidatura(ctx)
		if err != nil {
			return Resposta{}, err
		}
		op = func(e dominio.Estado, d dominio.Ctx) (dominio.Estado, Objeto, error) {
			return dominio.AprovarCandidatura(e, id, impressao, agora, limites, d)
		}
	default:
		if err := soCampos(c); err != nil {
			return Resposta{}, err
		}
		f := map[string]func(dominio.Estado, string, time.Time, dominio.Ctx) (dominio.Estado, Objeto, error){
			"revogar":         dominio.RevogarCandidatura,
			"cancelar":        dominio.CancelarCandidatura,
			"reabrir":         dominio.ReabrirCandidatura,
			"concluir-manual": dominio.ConcluirManual,
		}[acao]
		op = func(e dominio.Estado, d dominio.Ctx) (dominio.Estado, Objeto, error) { return f(e, id, agora, d) }
	}
	var res Objeto
	err := a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		novo, o, err := op(e, a.ctx(e, autor))
		res = o
		return novo, err
	})
	if err != nil {
		return Resposta{}, err
	}
	return ok(visao(res))
}

var reImpressaoLote = regexp.MustCompile(`^sha256:v1:[0-9a-f]{64}$`)

// aprovarLote (jc-a2y.6.13): aprova a "Lista do dia" de uma vez, só pela tela. Cada item com a impressão que a tela
// mostrou; numa transação só; item recusado não afeta os outros. O log leva só os números.
func (a *API) aprovarLote(ctx context.Context, c Objeto, autor string) (Resposta, error) {
	if err := soCampos(c, "itens"); err != nil {
		return Resposta{}, err
	}
	if autor != "eu" {
		return Resposta{}, &dominio.ErroSoPessoa{Msg: "só a pessoa pode aprovar candidaturas"}
	}
	lista, eLista := c["itens"].([]any)
	if !eLista || len(lista) == 0 || len(lista) > dominio.MaxItensLote {
		return Resposta{}, erroCampo(fmt.Sprintf("itens: lista de 1 a %d itens {id, impressao}", dominio.MaxItensLote))
	}
	itens := make([]dominio.ItemLote, 0, len(lista))
	for i, x := range lista {
		o, eObj := x.(map[string]any)
		if !eObj {
			return Resposta{}, erroCampo(fmt.Sprintf("itens[%d]: objeto {id, impressao}", i))
		}
		if err := soCampos(o, "id", "impressao"); err != nil {
			return Resposta{}, err
		}
		id, _ := o["id"].(string)
		impressao, _ := o["impressao"].(string)
		if !strings.HasPrefix(id, "cand-") || !reImpressaoLote.MatchString(impressao) {
			return Resposta{}, erroCampo(fmt.Sprintf("itens[%d]: id (cand-…) e impressao (sha256:v1: e 64 hex) obrigatórios", i))
		}
		itens = append(itens, dominio.ItemLote{ID: id, Impressao: impressao})
	}
	limites, err := a.arm.LimitesCandidatura(ctx)
	if err != nil {
		return Resposta{}, err
	}
	agora := a.relogio()
	var aprovadas []string
	var recusadas []dominio.Recusa
	err = a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		novo, ap, rec, err := dominio.AprovarLote(e, itens, agora, limites, a.ctx(e, autor))
		aprovadas, recusadas = ap, rec
		return novo, err
	})
	if err != nil {
		return Resposta{}, err
	}
	log.Printf("fila: lote aprovado (%s): %d aprovadas, %d recusada(s)", autor, len(aprovadas), len(recusadas)) // só números
	rec := make([]Objeto, 0, len(recusadas))
	for _, r := range recusadas {
		rec = append(rec, Objeto{"id": r.ID, "motivo": r.Motivo})
	}
	return ok(Objeto{"aprovadas": aprovadas, "recusadas": rec})
}

func limiteDiario(e dominio.Estado, limites map[string]int, plataforma string, agora time.Time) Objeto {
	n, _ := dominio.LimiteDe(limites, plataforma)
	return Objeto{"limite": n, "usadas": dominio.UsadasNoDia(e, plataforma, agora), "renovaEm": dominio.RenovaEm(agora).Format(time.RFC3339)}
}

func (a *API) limitesCandidatura(ctx context.Context) (Resposta, error) {
	e, err := a.arm.Carregar(ctx)
	if err != nil {
		return Resposta{}, err
	}
	limites, err := a.arm.LimitesCandidatura(ctx)
	if err != nil {
		return Resposta{}, err
	}
	agora := a.relogio()
	res := Objeto{}
	for _, p := range dominio.PlataformasComLimite {
		res[p] = limiteDiario(e, limites, p, agora)
	}
	return ok(res)
}

func (a *API) gravarLimite(ctx context.Context, plataforma string, c Objeto, autor string) (Resposta, error) {
	if !contem(dominio.PlataformasComLimite, plataforma) {
		return Resposta{}, falha(404, "plataforma sem limite diário não encontrada: "+plataforma)
	}
	if err := soCampos(c, "limite"); err != nil {
		return Resposta{}, err
	}
	n, ok := inteiro(c["limite"])
	if !ok {
		return Resposta{}, erroCampo("limite: número inteiro de 1 a 40")
	}
	if autor != "eu" {
		return Resposta{}, &dominio.ErroSoPessoa{Msg: "só a pessoa muda o limite diário"}
	}
	if err := a.arm.GravarLimiteCandidatura(ctx, plataforma, n); err != nil {
		return Resposta{}, err
	}
	return a.limiteGravado(ctx, plataforma)
}

func (a *API) limiteGravado(ctx context.Context, plataforma string) (Resposta, error) {
	e, err := a.arm.Carregar(ctx)
	if err != nil {
		return Resposta{}, err
	}
	limites, err := a.arm.LimitesCandidatura(ctx)
	if err != nil {
		return Resposta{}, err
	}
	return ok(limiteDiario(e, limites, plataforma, a.relogio()))
}

// respostaDaFila transforma os erros do domínio da fila em status: 403 só da pessoa, 409 conflito, 429 limite.
func respostaDaFila(err error) (Resposta, bool) {
	var es *dominio.ErroSoPessoa
	var ec *dominio.ErroConflito
	var el *dominio.ErroLimite
	switch {
	case errors.As(err, &es):
		return Resposta{Status: 403, Corpo: Objeto{"erro": es.Error()}}, true
	case errors.As(err, &ec):
		corpo := Objeto{"erro": ec.Error()}
		for k, v := range map[string]string{"estado": ec.Estado, "impressaoAtual": ec.ImpressaoAtual, "existente": ec.Existente, "motivo": ec.Motivo} {
			if v != "" {
				corpo[k] = v
			}
		}
		return Resposta{Status: 409, Corpo: corpo}, true
	case errors.As(err, &el):
		return Resposta{Status: 429, Corpo: Objeto{"erro": el.Error(), "plataforma": el.Plataforma, "limite": el.Limite, "usadas": el.Usadas, "renovaEm": el.RenovaEm}}, true
	}
	return Resposta{}, false
}
