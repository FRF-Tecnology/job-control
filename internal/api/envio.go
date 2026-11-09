package api

import (
	"context"
	"log"
	"sort"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Estado do envio, pausar e retomar (jc-a2y.51; contrato 0.4.12). O executor só existe no build envio_dev
// (Opcoes.EnvioLigado); "pausado" fica na meta do banco, com falha fechada (armazem.EnvioPausado). Os desfechos
// saem da fila, sem conteúdo (M7): só id, estado, motivo e horário.

// maxDesfechos: quantos desfechos o GET /envio devolve, do mais novo para o mais antigo.
const maxDesfechos = 20

func (a *API) rotasEnvio(ctx context.Context, m, p string, q map[string]string, c Objeto, autor string) (Resposta, bool, error) {
	switch {
	case m == "GET" && p == "/envio":
		r, err := a.estadoEnvio(ctx, q)
		return r, true, err
	case m == "POST" && p == "/envio/pausar":
		if err := soCampos(c); err != nil {
			return Resposta{}, true, err
		}
		r, err := a.mudarPausa(ctx, true)
		if err == nil {
			a.limparAoVivo()                         // pausado: sem quadro (jc-a2y.6.9)
			log.Printf("envio: pausado (%s)", autor) // só o fato e o autor
		}
		return r, true, err
	case m == "POST" && p == "/envio/retomar":
		if err := soCampos(c); err != nil {
			return Resposta{}, true, err
		}
		if autor != "eu" {
			return Resposta{}, true, &dominio.ErroSoPessoa{Msg: "só a pessoa retoma o envio"}
		}
		r, err := a.mudarPausa(ctx, false)
		if err == nil {
			log.Printf("envio: retomado (%s)", autor)
		}
		return r, true, err
	case m == "POST" && p == "/envio/janela":
		r, err := a.mudarJanela(ctx, c, autor)
		return r, true, err
	}
	return Resposta{}, false, nil
}

func (a *API) mudarPausa(ctx context.Context, pausado bool) (Resposta, error) {
	if err := a.arm.GravarEnvioPausado(ctx, pausado); err != nil {
		return Resposta{}, err
	}
	return a.estadoEnvio(ctx, nil)
}

// desfechoDe devolve o desfecho de uma candidatura que o executor concluiu, ou ok=false. Enviada à mão
// (concluir-manual) não é desfecho do executor.
func desfechoDe(o Objeto) (Objeto, time.Time, bool) {
	estado := dominio.Texto(o["estado"])
	resultado, bloqueio := dominio.ObjetoDe(o["resultado"]), dominio.ObjetoDe(o["bloqueio"])
	var em string
	var motivo any
	switch estado {
	case dominio.CandBloqueada:
		em, motivo = dominio.Texto(bloqueio["em"]), dominio.Texto(bloqueio["motivo"])
	case dominio.CandEnviada, dominio.CandFalhou, dominio.CandIncerta:
		if dominio.Igual(resultado["tipo"], "manual") {
			return nil, time.Time{}, false
		}
		em = dominio.Texto(resultado["em"])
	default:
		return nil, time.Time{}, false
	}
	quando, err := time.Parse(time.RFC3339, em)
	if err != nil {
		return nil, time.Time{}, false
	}
	return Objeto{"id": o["id"], "estado": estado, "motivo": motivo, "em": em}, quando, true
}

func (a *API) estadoEnvio(ctx context.Context, q map[string]string) (Resposta, error) {
	var desde time.Time
	if d, tem := q["desde"]; tem {
		t, err := time.Parse(time.RFC3339, d)
		if err != nil {
			return Resposta{}, erroCampo("desde: data e hora no formato ISO 8601 (ex.: 2026-11-09T10:00:00-03:00)")
		}
		desde = t
	}
	e, err := a.arm.Carregar(ctx)
	if err != nil {
		return Resposta{}, err
	}
	var enviando any
	type item struct {
		o      Objeto
		quando time.Time
	}
	var itens []item
	for _, o := range e.Candidaturas {
		if dominio.Igual(o["estado"], dominio.CandEnviando) && enviando == nil {
			desdeEnvio := dominio.Texto(o["atualizadaEm"])
			if t := dominio.ListaDe(o["tentativas"]); len(t) > 0 {
				desdeEnvio = dominio.Texto(t[len(t)-1])
			}
			enviando = Objeto{"id": o["id"], "desde": desdeEnvio}
		}
		if d, quando, ok := desfechoDe(o); ok && (desde.IsZero() || quando.After(desde)) {
			itens = append(itens, item{d, quando})
		}
	}
	sort.SliceStable(itens, func(i, j int) bool { return itens[i].quando.After(itens[j].quando) })
	desfechos := []Objeto{}
	for i := 0; i < len(itens) && i < maxDesfechos; i++ {
		desfechos = append(desfechos, itens[i].o)
	}
	janela, login := "oculta", false
	if a.envioLigado {
		var visivel bool
		if visivel, login = a.janela(); visivel {
			janela = "visivel"
		}
	}
	return ok(Objeto{"ligado": a.envioLigado, "pausado": a.arm.EnvioPausado(ctx), "janela": janela, "login": login, "enviandoAgora": enviando, "desfechos": desfechos})
}
