package executor

import (
	"context"
	"errors"

	"github.com/FRF-Tecnology/job-control/internal/aovivo"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Envio de formulários externos (jc-a2y.7.2, parte 3b, docs/propostas/formularios.md), combinado com o Faker:
//   - só o despacho fica no Enviar (depois do IniciarEnvio); concluir e tentativa são os de sempre;
//   - a página do formulário é uma interface pequena, separada da Pagina: sem ela, erro técnico, nunca envio;
//   - o endereço é conferido contra o destino guardado na leitura (estrutura.urlFinal), em cada passo e logo antes
//     do clique final; o log leva só a categoria;
//   - marcas do .6.11: estrutura relida diferente, leitura que falha ou bloqueio param ANTES de qualquer marca; a
//     marca "interagiu" vem antes da primeira ação que digita, clica ou marca; "clicou-enviar" antes do clique final;
//     marca que não grava = o passo não acontece;
//   - a caixa "enviar uma cópia para o meu e-mail" nunca fica marcada: marcada pela página, o app grava a marca,
//     desmarca e confere; erro = falhou.

// PaginaFormulario: o que o envio de um formulário pede além da Pagina. Nada do que ela devolve vai para o log.
type PaginaFormulario interface {
	// LerEstrutura relê a estrutura sem interagir, como a leitura da rota ler-formulario: bloqueio da lista fechada,
	// perguntas e o destino canônico.
	LerEstrutura(ctx context.Context) (dominio.LeituraFormulario, error)
	// RespondidasPelaPagina: os ids das perguntas que a página já trouxe com valor. O valor nunca volta.
	RespondidasPelaPagina(ctx context.Context) ([]string, error)
	// CopiaMarcada: se a caixa "enviar uma cópia das respostas para o meu e-mail" está marcada (sem caixa: false).
	CopiaMarcada(ctx context.Context) (bool, error)
	// DesmarcarCopia desmarca a caixa da cópia e confere que ficou desmarcada. Nunca marca.
	DesmarcarCopia(ctx context.Context) error
	// Responder põe o valor do pacote na pergunta: texto digitado; opção escolhida pelo rótulo exato (ErrSemOpcao
	// quando não há essa opção). Nunca avança nem envia.
	Responder(ctx context.Context, p dominio.PerguntaFormulario, valor string) error
	// EnviarFormulario clica uma vez no botão final do formulário (ErrSemBotaoEnviar quando não há botão: nada clicado).
	EnviarFormulario(ctx context.Context) error
	// RespostaRegistrada: a página mostrou uma das frases fixas de resposta registrada. A URL e o link "Editar sua
	// resposta" da página final nunca saem da página.
	RespostaRegistrada(ctx context.Context) (bool, error)
}

var (
	errSemPaginaFormulario = errors.New("executor: a página não sabe enviar formulário")
	errCopiaMarcada        = errors.New("executor: a cópia por e-mail continuou marcada")
)

func (x *Executor) preencherFormulario(ctx context.Context, c dominio.Objeto, t *tentativa) passo {
	id, plataforma, aprovada := t.id, dominio.Texto(c["plataforma"]), dominio.Texto(c["url"])
	destino := dominio.Texto(dominio.ObjetoDe(c["estrutura"])["urlFinal"]) // o canônico lido (o do forms.gle)
	if destino == "" {
		return bloqueada("outro")
	}
	pag, err := x.o.Navegador.NovaPagina(ctx)
	if err != nil {
		return falhou("erro-tecnico", err)
	}
	defer pag.Fechar()
	pf, ok := pag.(PaginaFormulario)
	if !ok {
		return falhou("erro-tecnico", errSemPaginaFormulario)
	}

	x.o.Etapa(id, aovivo.Abrindo)
	final, err := pag.Abrir(ctx, aprovada)
	if err != nil {
		return falhou("erro-tecnico", err)
	}
	if p, parou := x.motivoDoEndereco(plataforma, destino, final); parou {
		return p
	}
	// Antes de qualquer marca: relê a estrutura e confere com a aprovada (hash, destino, valores só do pacote).
	leitura, err := pf.LerEstrutura(ctx)
	if err != nil {
		return falhou("erro-tecnico", err)
	}
	if leitura.Bloqueio != "" {
		return bloqueada(leitura.Bloqueio)
	}
	plano, motivo := dominio.PlanoDoFormulario(c, leitura.Perguntas, leitura.URLFinal)
	if motivo != "" {
		return bloqueada(motivo)
	}
	// Campo que a página já trouxe preenchido: nunca vai sem a pessoa aprovar, e o app não apaga nem troca (#89).
	respondidas, err := pf.RespondidasPelaPagina(ctx)
	if err != nil {
		return falhou("erro-tecnico", err)
	}
	if len(respondidas) > 0 {
		return bloqueada("pergunta-nova")
	}
	marcada, err := pf.CopiaMarcada(ctx)
	if err != nil {
		return falhou("erro-tecnico", err)
	}
	if marcada {
		if err := t.marcar(ctx, dominio.MarcaInteragiu); err != nil {
			return falhou("erro-tecnico", err)
		}
		if err := pf.DesmarcarCopia(ctx); err != nil {
			return falhou("erro-tecnico", err)
		}
		x.o.Log("executor: cópia por e-mail desmarcada")
	}

	x.o.Etapa(id, aovivo.Preenchendo)
	for _, r := range plano {
		if err := x.pausa(ctx, x.o.EntreAcoes); err != nil {
			return falhou("erro-tecnico", err)
		}
		if p, parou := x.conferirEndereco(ctx, pag, plataforma, destino); parou {
			return p
		}
		if err := t.marcar(ctx, dominio.MarcaInteragiu); err != nil {
			return falhou("erro-tecnico", err)
		}
		if err := pf.Responder(ctx, r.Pergunta, r.Valor); err != nil {
			if errors.Is(err, ErrSemOpcao) {
				return bloqueada("pergunta-nova") // outro valor nunca vai no lugar
			}
			return falhou("erro-tecnico", err)
		}
	}

	// Logo antes do clique: no destino guardado e com a cópia desmarcada.
	x.o.Etapa(id, aovivo.Revisando)
	if err := x.pausa(ctx, x.o.EntreAcoes); err != nil {
		return falhou("erro-tecnico", err)
	}
	if p, parou := x.conferirEndereco(ctx, pag, plataforma, destino); parou {
		return p
	}
	if marcada, err := pf.CopiaMarcada(ctx); err != nil || marcada {
		if err == nil {
			err = errCopiaMarcada
		}
		return falhou("erro-tecnico", err)
	}
	// M3: do clique em diante, sem confirmação, fica incerta.
	if ctx.Err() != nil {
		return falhou("erro-tecnico", ctx.Err())
	}
	if err := t.marcar(ctx, dominio.MarcaClicouEnviar); err != nil {
		return falhou("erro-tecnico", err)
	}
	x.o.Etapa(id, aovivo.Enviando)
	if err := pf.EnviarFormulario(ctx); err != nil {
		if errors.Is(err, ErrSemBotaoEnviar) {
			return bloqueada("outro") // nada foi clicado: a pessoa confere
		}
		return passo{dominio.CandIncerta, "", err}
	}
	if err := x.pausa(context.WithoutCancel(ctx), x.o.EntreAcoes); err != nil {
		return passo{dominio.CandIncerta, "", err}
	}
	confirmou, err := pf.RespostaRegistrada(context.WithoutCancel(ctx))
	if err != nil || !confirmou {
		return passo{dominio.CandIncerta, "", err}
	}
	return passo{estado: dominio.CandEnviada}
}
