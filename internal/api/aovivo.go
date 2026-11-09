package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/aovivo"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// "Ao vivo" (jc-a2y.6.9, contrato 0.8.0, docs/propostas/ao-vivo.md): a etapa do envio e o último quadro do navegador
// do envio, só para ver. O quadro fica só em memória (internal/aovivo): não passa pelo banco, pelo backup nem pelo
// log. Ler e ligar: só a tela da janela do app (DaJanela, marcada pelo Handler). Desligar: qualquer autor.

// hostsDaJanela: o Host que a janela do Wails manda (wails.localhost no Windows, localhost no macOS e no Linux), sem
// porta. A janela não usa porta de rede; o central-dev (127.0.0.1:4500) e um programa local ficam de fora.
var hostsDaJanela = map[string]bool{"wails.localhost": true, "localhost": true}

// daJanela: o pedido HTTP veio da própria janela do app.
func daJanela(r *http.Request) bool {
	return r.Header.Get("Sec-Fetch-Site") == "same-origin" && hostsDaJanela[r.Host]
}

func (a *API) rotaAoVivo(ctx context.Context, m string, c Objeto, autor string, janela bool) (Resposta, error) {
	if m != "GET" && m != "POST" {
		return Resposta{}, falha(404, "rota não encontrada")
	}
	if a.aoVivo == nil {
		return Resposta{}, falha(404, msgSemEnvio)
	}
	soTela := autor == "eu" && janela
	switch m {
	case "GET":
		if !soTela {
			return Resposta{}, &dominio.ErroSoPessoa{Msg: "só a tela do app vê o envio ao vivo"}
		}
		return ok(a.estadoAoVivo(ctx))
	case "POST":
		if err := soCampos(c, "ligado"); err != nil {
			return Resposta{}, err
		}
		ligado, eBool := c["ligado"].(bool)
		if !eBool {
			return Resposta{}, erroCampo("ligado: true ou false")
		}
		if ligado && !soTela {
			return Resposta{}, &dominio.ErroSoPessoa{Msg: "só a pessoa, pela tela do app, liga o envio ao vivo"}
		}
		if a.aoVivo.Ligar(ligado) {
			if ligado {
				log.Printf("ao vivo: ligado (%s)", autor) // só o fato e o autor
			} else {
				log.Printf("ao vivo: desligado (%s)", autor)
			}
		}
		if !soTela { // quem não é a tela só desliga: recebe o estado sem ler (não mantém ligado)
			return ok(objetoAoVivo(aovivo.Estado{Etapa: aovivo.Desligado}))
		}
		return ok(a.estadoAoVivo(ctx))
	}
	return Resposta{}, falha(404, "rota não encontrada")
}

// estadoAoVivo: login aberto e envio pausado não mostram quadro, e o quadro em memória é apagado.
func (a *API) estadoAoVivo(ctx context.Context) Objeto {
	e := a.aoVivo.Ler()
	if !e.Ligado {
		return objetoAoVivo(e)
	}
	_, login := a.janela()
	switch {
	case login:
		a.aoVivo.Limpar()
		e = aovivo.Estado{Ligado: true, Etapa: aovivo.LoginAberto}
	case a.arm.EnvioPausado(ctx):
		a.aoVivo.Limpar()
		e = aovivo.Estado{Ligado: true, Etapa: aovivo.Pausado}
	}
	return objetoAoVivo(e)
}

func objetoAoVivo(e aovivo.Estado) Objeto {
	o := Objeto{"ligado": e.Ligado, "etapa": e.Etapa, "candidatura": nil, "quadro": nil}
	if e.Candidatura != "" {
		o["candidatura"] = e.Candidatura
	}
	if q := e.Quadro; q != nil {
		o["quadro"] = Objeto{"imagem": q.Imagem, "em": q.Em.Format(time.RFC3339), "largura": q.Largura, "altura": q.Altura}
	}
	return o
}

// limparAoVivo: pausar, abrir o login, sair das plataformas e apagar os dados apagam o quadro.
func (a *API) limparAoVivo() {
	if a.aoVivo != nil {
		a.aoVivo.Limpar()
	}
}
