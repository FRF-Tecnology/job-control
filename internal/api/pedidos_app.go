package api

import (
	"context"
	"log"
	"regexp"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Pedidos atendidos pelo próprio app (jc-a2y.7.1, contrato 0.5.0, docs/propostas/pedidos-no-app.md):
//   - POST /pedidos/{id}/interpretar (só a tela; uma por vez): a IA da pessoa propõe UMA ação do catálogo fechado; o
//     servidor guarda a proposta (resumo com frases fixas, chamada exata e impressão). Nada grava no domínio;
//   - POST /pedidos/{id}/descartar (qualquer autor): a proposta sai, o pedido volta a aberto;
//   - POST /vagas e POST /vagas/{id}/mover com X-Pedido e X-Impressao (só a tela): confirma a proposta. A chamada tem
//     de ser a guardada, e a ação e o pedido feito gravam na mesma transação.
//
// O log leva só o tipo da ação e o resultado: nada do texto do pedido nem dos parâmetros.

var (
	rePedidoNoApp   = regexp.MustCompile(`^/pedidos/([^/]+)/(interpretar|descartar)$`)
	reXPedido       = regexp.MustCompile(`^pd-[A-Za-z0-9._-]{1,80}$`)
	reXImpressao    = regexp.MustCompile(`^sha256:v1:[0-9a-f]{64}$`)
	reConfirmaMover = regexp.MustCompile(`^/vagas/[^/]+/mover$`)
)

func (a *API) rotasPedidosNoApp(ctx context.Context, m, p string, c Objeto, autor string) (Resposta, bool, error) {
	g := rePedidoNoApp.FindStringSubmatch(p)
	if g == nil || m != "POST" {
		return Resposta{}, false, nil
	}
	id, err := decodificar(g[1])
	if err != nil {
		return Resposta{}, true, err
	}
	if err := soCampos(c); err != nil {
		return Resposta{}, true, err
	}
	if g[2] == "descartar" {
		var ped Objeto
		err := a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
			novo, x, err := dominio.DescartarProposta(e, id)
			ped = x
			return novo, err
		})
		if err != nil {
			return Resposta{}, true, err
		}
		log.Printf("pedidos: proposta descartada (%s)", autor)
		r, err := ok(ped)
		return r, true, err
	}
	r, err := a.interpretarPedido(ctx, id, autor)
	return r, true, err
}

// interpretarPedido gasta a IA da pessoa: só pela tela e uma por vez.
func (a *API) interpretarPedido(ctx context.Context, id, autor string) (Resposta, error) {
	if autor != "eu" {
		return Resposta{}, &dominio.ErroSoPessoa{Msg: "só a pessoa pede para a IA interpretar um pedido (pela tela)"}
	}
	if !a.interpretando.TryLock() {
		return Resposta{}, &dominio.ErroConflito{Msg: "já há uma interpretação em andamento"}
	}
	defer a.interpretando.Unlock()
	e, err := a.arm.Carregar(ctx)
	if err != nil {
		return Resposta{}, err
	}
	var pedido Objeto
	for _, p := range e.Pedidos {
		if dominio.Igual(p["id"], id) {
			pedido = p
		}
	}
	if pedido == nil {
		return Resposta{}, falha(404, "pedido não encontrado: "+id)
	}
	if !dominio.Igual(pedido["status"], dominio.PedidoAberto) {
		return Resposta{}, &dominio.ErroConflito{Msg: "o pedido não está aberto"}
	}
	x, err := a.novaIA(ctx)
	if err != nil {
		return Resposta{}, err
	}
	// Só o mínimo das vagas (id, empresa, título, etapa e datas); o prompt também corta (internal/ia).
	vagas := []any{}
	for _, v := range e.Vagas {
		o := Objeto{}
		for _, k := range []string{"id", "empresa", "titulo", "etapa", "criadaEm", "dataAplicacao"} {
			if val, tem := v[k]; tem && val != nil {
				o[k] = val
			}
		}
		vagas = append(vagas, o)
	}
	saida, err := x.Executar(ctx, "interpretarPedido", Objeto{"pedido": dominio.Texto(pedido["texto"]), "vagas": vagas})
	if err != nil {
		log.Printf("pedidos: interpretação recusada pela IA ou pela guarda")
		return respostaIA(err)
	}
	var res Objeto
	err = a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		novo, p, err := dominio.InterpretarPedido(e, id, saida, a.relogio(), a.ctx(e, autor))
		res = p
		return novo, err
	})
	if err != nil {
		return Resposta{}, err
	}
	log.Printf("pedidos: interpretado (%s, %s)", dominio.Texto(saida["acao"]), dominio.Texto(res["status"])) // só códigos
	return ok(res)
}

// confirmarPedido: a pessoa confirma, pela tela, a proposta de um pedido chamando a rota que já existe com
// X-Pedido e X-Impressao. Só POST /vagas e POST /vagas/{id}/mover; corpo objeto, igual ao guardado.
func (a *API) confirmarPedido(ctx context.Context, m, p string, corpo any, xPedido, xImpressao, autor string) (Resposta, error) {
	if autor != "eu" {
		return Resposta{}, &dominio.ErroSoPessoa{Msg: "só a pessoa confirma a proposta de um pedido (pela tela)"}
	}
	if !reXPedido.MatchString(xPedido) || !reXImpressao.MatchString(xImpressao) {
		return Resposta{}, erroCampo("X-Pedido e X-Impressao: os dois, no formato da proposta")
	}
	criar := m == "POST" && p == "/vagas"
	if !criar && !(m == "POST" && reConfirmaMover.MatchString(p)) {
		return Resposta{}, erroCampo("X-Pedido só confirma POST /vagas ou POST /vagas/{id}/mover")
	}
	if !ehObjeto(corpo) {
		return Resposta{}, erroCampo("envie o corpo da proposta (um objeto JSON)")
	}
	// O caminho chega como veio (escapes inclusive): a comparação é com o guardado, byte a byte.
	var vaga Objeto
	var acao string
	err := a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		novo, v, err := dominio.ConfirmarProposta(e, xPedido, xImpressao, m, p, corpo, a.ctx(e, autor))
		vaga = v
		if err == nil {
			for _, ped := range novo.Pedidos {
				if dominio.Igual(ped["id"], xPedido) {
					acao = dominio.Texto(dominio.ObjetoDe(ped["proposta"])["acao"])
				}
			}
		}
		return novo, err
	})
	if err != nil {
		log.Printf("pedidos: confirmação recusada (%s)", tipoDoErroPedido(err))
		return Resposta{}, err
	}
	log.Printf("pedidos: proposta confirmada (%s)", acao)
	if criar {
		return criado(vaga)
	}
	return ok(vaga)
}

// tipoDoErroPedido: só a classe do erro no log (nada do pedido).
func tipoDoErroPedido(err error) string {
	t := strings.TrimPrefix(strings.TrimPrefix(tipoDoErro(err), "*dominio."), "*")
	return t
}
