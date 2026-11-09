package api

import (
	"context"
	"errors"
	"log"
	"regexp"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Formulários externos (jc-a2y.7.2, docs/propostas/formularios.md). POST /candidaturas/{id}/ler-formulario: o leitor
// do executor (Opcoes.LerFormulario) abre a URL canônica no navegador do envio, sem janela, e lê a estrutura sem
// interagir. Sem leitor (build padrão e central-dev): 404. O texto das perguntas é dado não confiável: não vai para
// a IA nem para o log; o log leva só números e códigos.

var reLerFormulario = regexp.MustCompile(`^/candidaturas/([^/]+)/ler-formulario$`)

func (a *API) lerFormularioDe(ctx context.Context, id string, c Objeto, autor string) (Resposta, error) {
	if a.lerFormulario == nil {
		return Resposta{}, falha(404, msgSemEnvio)
	}
	if autor != "eu" {
		return Resposta{}, &dominio.ErroSoPessoa{Msg: "só a pessoa, pela tela, lê o formulário"}
	}
	if err := soCampos(c); err != nil {
		return Resposta{}, err
	}
	if _, login := a.janela(); login {
		return Resposta{}, &dominio.ErroConflito{Msg: "feche a janela de login antes de ler o formulário", Motivo: "login-aberto"}
	}
	if err := a.semEnvioEmAndamento(ctx, "há uma candidatura sendo enviada agora; espere terminar"); err != nil {
		return Resposta{}, err
	}
	// A leitura conta no teto de páginas antes de abrir (no teto, 429 e nada abre).
	agora := a.relogio()
	var endereco string
	if err := a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		novo, o, err := dominio.RegistrarLeitura(e, id, agora)
		endereco = dominio.Texto(o["url"])
		return novo, err
	}); err != nil {
		return Resposta{}, err
	}
	// Fora da transação: o navegador pode levar segundos.
	leitura, err := a.lerFormulario(ctx, endereco)
	if err != nil {
		log.Printf("formulario: leitura falhou (%s)", tipoDoErro(err)) // só o tipo
		return Resposta{}, &dominio.ErroConflito{Msg: "não deu para ler o formulário agora; tente de novo", Estado: dominio.CandPreparada}
	}
	var res dominio.Objeto
	var motivo string
	err = a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		motivo = leitura.Bloqueio
		if motivo == "" {
			novo, o, err := dominio.GravarEstruturaFormulario(e, id, leitura.Perguntas, leitura.URLFinal, agora)
			var ev *dominio.ErroValidacao
			if !errors.As(err, &ev) {
				res = o
				return novo, err
			}
			motivo = "outro" // estrutura que o domínio não aceita: a pessoa envia
		}
		if _, conhecido := dominio.PendenciaDoMotivo[motivo]; !conhecido {
			motivo = "outro"
		}
		novo, o, err := dominio.BloquearLeitura(e, id, motivo, agora, a.ctx(e, autor))
		res = o
		return novo, err
	})
	if err != nil {
		return Resposta{}, err
	}
	if motivo != "" {
		log.Printf("formulario: bloqueado na leitura (%s)", motivo) // só o código
	} else {
		log.Printf("formulario: lido (%d perguntas, %d pendente(s))", len(leitura.Perguntas), len(dominio.ListaDe(res["pendentes"])))
	}
	return ok(visao(res))
}
