package api

import (
	"context"
	"log"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/janela"
)

// Janela do app de PC (contrato 0.15.0). O app abre num tamanho padrão e lembra o que a pessoa deixar; isso é do app
// (app/janela.go) e não passa pela API. Pela API, e só pela tela, passam as duas escolhas da pessoa no Perfil:
// "Continuar aberto em segundo plano" (PUT /app/janela) e "Voltar ao tamanho padrão" (POST /app/janela/padrao).
// Fora do app de PC (central-dev, MCP, CLI) não há janela: GET responde disponivel=false e as outras duas dão 404.

// JanelaApp são os ganchos da janela do app; só o app de PC passa (Opcoes.JanelaApp).
type JanelaApp struct {
	// SegundoPlano liga ou desliga "fechar a janela mantém o app aberto" e devolve se ficou como pedido (ligar
	// falha quando o sistema não deixa criar o ícone da bandeja).
	SegundoPlano func(ligar bool) bool
	// TamanhoPadrao põe a janela de volta no tamanho padrão, no centro da tela.
	TamanhoPadrao func()
}

const msgSemJanela = "só o app instalado no computador tem janela para ajustar"

func (a *API) rotasJanelaApp(ctx context.Context, m, p string, c Objeto, autor string) (Resposta, bool, error) {
	switch {
	case m == "GET" && p == "/app/janela":
		r, err := a.estadoJanelaApp(ctx)
		return r, true, err
	case m == "PUT" && p == "/app/janela":
		r, err := a.mudarSegundoPlano(ctx, c, autor)
		return r, true, err
	case m == "POST" && p == "/app/janela/padrao":
		r, err := a.voltarJanelaAoPadrao(ctx, c, autor)
		return r, true, err
	}
	return Resposta{}, false, nil
}

func (a *API) estadoJanelaApp(ctx context.Context) (Resposta, error) {
	var segundoPlano bool
	if a.janelaApp != nil {
		if _, err := a.arm.LerMeta(ctx, janela.ChaveSegundoPlano, &segundoPlano); err != nil {
			return Resposta{}, err
		}
	}
	return ok(Objeto{"disponivel": a.janelaApp != nil, "segundoPlano": segundoPlano})
}

func (a *API) mudarSegundoPlano(ctx context.Context, c Objeto, autor string) (Resposta, error) {
	if err := soCampos(c, "segundoPlano"); err != nil {
		return Resposta{}, err
	}
	ligar, eBool := c["segundoPlano"].(bool)
	if !eBool {
		return Resposta{}, erroCampo("segundoPlano: true ou false")
	}
	if autor != "eu" {
		return Resposta{}, &dominio.ErroSoPessoa{Msg: "só a pessoa escolhe se o app fica aberto em segundo plano"}
	}
	if a.janelaApp == nil {
		return Resposta{}, falha(404, msgSemJanela)
	}
	if !a.janelaApp.SegundoPlano(ligar) {
		return Resposta{}, &dominio.ErroConflito{Msg: "não deu para pôr o ícone do Applyer perto do relógio; o app continua fechando junto com a janela", Motivo: "sem-bandeja"}
	}
	if err := a.arm.GravarMeta(ctx, janela.ChaveSegundoPlano, ligar); err != nil {
		a.janelaApp.SegundoPlano(!ligar) // não gravou: volta ao que estava, para a tela e o app não divergirem
		return Resposta{}, err
	}
	log.Printf("janela: segundo plano %t (%s)", ligar, autor) // só o fato e o autor
	return a.estadoJanelaApp(ctx)
}

func (a *API) voltarJanelaAoPadrao(ctx context.Context, c Objeto, autor string) (Resposta, error) {
	if err := soCampos(c); err != nil {
		return Resposta{}, err
	}
	if autor != "eu" {
		return Resposta{}, &dominio.ErroSoPessoa{Msg: "só a pessoa muda o tamanho da janela"}
	}
	if a.janelaApp == nil {
		return Resposta{}, falha(404, msgSemJanela)
	}
	a.janelaApp.TamanhoPadrao()
	return a.estadoJanelaApp(ctx)
}
