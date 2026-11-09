package api

import (
	"context"
	"log"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Envio em segundo plano (jc-a2y.6.7; contrato 0.4.13). O navegador do envio abre sem janela: não pega o foco, o
// mouse nem o teclado de quem usa o PC. Janela visível só por ação da pessoa na tela: "Ver o envio"
// (POST /envio/janela) e "Entrar nas plataformas" (POST /navegador/entrar). Nada disso existe no build padrão (404).

// enderecoDeLogin: lista fechada. Gupy e InHire têm login por empresa: a pessoa entra pela página da vaga.
var enderecoDeLogin = map[string]string{"linkedin": "https://www.linkedin.com/login"}

const msgSemEnvio = "este build não traz o envio de candidaturas"

func (a *API) mudarJanela(ctx context.Context, c Objeto, autor string) (Resposta, error) {
	if err := soCampos(c, "visivel"); err != nil {
		return Resposta{}, err
	}
	visivel, eBool := c["visivel"].(bool)
	if !eBool {
		return Resposta{}, erroCampo("visivel: true ou false")
	}
	if !a.envioLigado {
		return Resposta{}, falha(404, msgSemEnvio)
	}
	if visivel && autor != "eu" {
		return Resposta{}, &dominio.ErroSoPessoa{Msg: "só a pessoa abre a janela do envio"}
	}
	a.mostrarEnvio(visivel)
	if visivel {
		log.Printf("envio: janela visível (%s)", autor) // só o fato e o autor
	} else {
		log.Printf("envio: janela oculta (%s)", autor)
	}
	return a.estadoEnvio(ctx, nil)
}

func (a *API) entrarNoNavegador(ctx context.Context, c Objeto, autor string) (Resposta, error) {
	if err := soCampos(c, "plataforma", "candidatura"); err != nil {
		return Resposta{}, err
	}
	plataforma, temP := c["plataforma"]
	candidatura, temC := c["candidatura"]
	if temP == temC {
		return Resposta{}, erroCampo("envie plataforma ou candidatura (exatamente um dos dois)")
	}
	if autor != "eu" {
		return Resposta{}, &dominio.ErroSoPessoa{Msg: "só a pessoa entra nas plataformas"}
	}
	if !a.envioLigado {
		return Resposta{}, falha(404, msgSemEnvio)
	}
	var endereco string
	if temP {
		p, _ := plataforma.(string)
		if endereco = enderecoDeLogin[p]; endereco == "" {
			return Resposta{}, erroCampo("plataforma: linkedin (Gupy e InHire: entre pela candidatura)")
		}
	} else {
		id, _ := candidatura.(string)
		e, err := a.arm.Carregar(ctx)
		if err != nil {
			return Resposta{}, err
		}
		var achada Objeto
		for _, o := range e.Candidaturas {
			if dominio.Igual(o["id"], id) {
				achada = o
			}
		}
		if achada == nil {
			return Resposta{}, falha(404, "candidatura não encontrada")
		}
		url, plat := dominio.Texto(achada["url"]), dominio.Texto(achada["plataforma"])
		// Só abre página de vaga da lista fechada da plataforma (a mesma conferência do executor, M5).
		if !dominio.MesmaVaga(plat, url, url) {
			return Resposta{}, erroCampo("candidatura: o endereço não é de uma vaga do LinkedIn, da Gupy ou da InHire")
		}
		endereco = url
	}
	if err := a.semEnvioEmAndamento(ctx, "há uma candidatura sendo enviada agora; espere terminar"); err != nil {
		if c, conflito := err.(*dominio.ErroConflito); conflito {
			com := *c
			com.Motivo = "enviando"
			return Resposta{}, &com
		}
		return Resposta{}, err
	}
	// O envio devolve um ErroConflito com texto fixo (login já aberto, envio em andamento, sem navegador).
	if err := a.entrar(endereco); err != nil {
		if c, conflito := err.(*dominio.ErroConflito); conflito && c.Motivo != "" {
			return Resposta{}, err
		}
		return Resposta{}, &dominio.ErroConflito{Msg: "não deu para abrir o navegador (Chrome ou Edge) neste computador", Motivo: "sem-navegador"}
	}
	a.limparAoVivo()                                            // o login nunca entra no ao vivo (jc-a2y.6.9)
	log.Printf("navegador: janela de login aberta (%s)", autor) // sem endereço (M7)
	return a.estadoEnvio(ctx, nil)
}
