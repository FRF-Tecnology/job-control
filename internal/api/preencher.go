package api

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/curriculos"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Preencher no site da empresa (docs/propostas/preencher-site-externo.md), só da tela, para a vaga que a pessoa já
// reservou com "Aplicar na vaga" (aplicacaoManual, em aplicar):
//   - POST /vagas/{id}/abrir-para-preencher {}: abre a vaga numa janela visível do navegador do app e tenta clicar em
//     "Candidatar-se", que leva ao site da empresa;
//   - POST /vagas/{id}/preencher {}: devolve {dominio}, o domínio da aba do site, para a pessoa conferir;
//   - POST /vagas/{id}/preencher {dominio}: preenche, se a aba ainda está nesse domínio, e devolve só números.
// O app nunca avança nem envia: a pessoa revisa, envia no site e usa "Já enviei" (confirmar-envio-manual).

func (a *API) vagaReservada(ctx context.Context, id string) (dominio.Estado, Objeto, error) {
	e, err := a.arm.Carregar(ctx)
	if err != nil {
		return e, nil, err
	}
	for _, v := range e.Vagas {
		if v["id"] != id {
			continue
		}
		if v["aplicacaoManual"] != true || v["etapa"] != "aplicar" || dominio.Texto(v["candidaturaManualId"]) == "" {
			return e, nil, &dominio.ErroConflito{Msg: "clique em Aplicar na vaga antes de preencher", Motivo: "sem-reserva"}
		}
		return e, v, nil
	}
	return e, nil, falha(404, "vaga não encontrada")
}

func (a *API) conferirPreencher(autor string) error {
	if a.abrirParaPreencher == nil || a.siteParaPreencher == nil || a.preencherSite == nil {
		return falha(404, msgSemEnvio)
	}
	if autor != "eu" {
		return &dominio.ErroSoPessoa{Msg: "só a pessoa, pela tela, abre e preenche o site da empresa"}
	}
	return nil
}

func (a *API) abrirVagaParaPreencher(ctx context.Context, id string, c Objeto, autor string) (Resposta, error) {
	if err := a.conferirPreencher(autor); err != nil {
		return Resposta{}, err
	}
	if err := soCampos(c); err != nil {
		return Resposta{}, err
	}
	if err := a.semEnvioEmAndamento(ctx, "há uma candidatura sendo enviada agora; espere terminar"); err != nil {
		return Resposta{}, err
	}
	_, v, err := a.vagaReservada(ctx, id)
	if err != nil {
		return Resposta{}, err
	}
	endereco := dominio.Texto(v["urlCandidaturaManual"])
	if endereco == "" {
		endereco = dominio.Texto(v["link"])
	}
	if !dominio.URLDaPlataforma("outra", endereco) {
		return Resposta{}, &dominio.ErroConflito{Msg: "a vaga não tem um endereço https", Motivo: "sem-endereco"}
	}
	if err := a.abrirParaPreencher(endereco); err != nil {
		return Resposta{}, erroDoPreencher(err)
	}
	log.Printf("preencher: janela aberta")
	return ok(Objeto{"aberta": true})
}

func (a *API) preencherSiteDaVaga(ctx context.Context, id string, c Objeto, autor string) (Resposta, error) {
	if err := a.conferirPreencher(autor); err != nil {
		return Resposta{}, err
	}
	if err := soCampos(c, "dominio"); err != nil {
		return Resposta{}, err
	}
	e, v, err := a.vagaReservada(ctx, id)
	if err != nil {
		return Resposta{}, err
	}
	conferido, informado := c["dominio"]
	if !informado {
		host, err := a.siteParaPreencher(ctx)
		if err != nil {
			return Resposta{}, erroDoPreencher(err)
		}
		return ok(Objeto{"dominio": host})
	}
	dom, _ := conferido.(string)
	dom = strings.ToLower(strings.TrimSpace(dom))
	if dom == "" || len(dom) > 253 || strings.ContainsAny(dom, "/:@ ") {
		return Resposta{}, erroCampo("dominio: o domínio que a tela mostrou")
	}

	// O pacote reservado dá o currículo; o perfil dá o contato e as respostas padrão.
	var pacote Objeto
	for _, cand := range e.Candidaturas {
		if cand["id"] == v["candidaturaManualId"] {
			pacote = dominio.ObjetoDe(cand["conteudo"])
		}
	}
	if pacote == nil {
		return Resposta{}, &dominio.ErroConflito{Msg: "clique em Aplicar na vaga antes de preencher", Motivo: "sem-reserva"}
	}
	nome, bytes := "", []byte(nil)
	if cv := dominio.ObjetoDe(pacote["curriculo"]); dominio.NomeDeCurriculoValido(dominio.Texto(cv["arquivo"])) {
		b, err := curriculos.Ler(a.pastaCurriculos(), dominio.Texto(cv["arquivo"]))
		if err != nil || curriculos.SHA256(b) != dominio.Texto(cv["sha256"]) {
			return Resposta{}, &dominio.ErroConflito{Msg: "o currículo mudou depois que você aplicou; aplique de novo na vaga", Motivo: "curriculo-mudou"}
		}
		nome, bytes = dominio.Texto(cv["arquivo"]), b
	}
	conteudo := dominio.ConteudoAssistido(e.Perfil, pacote)
	r, err := a.preencherSite(ctx, dom, conteudo, e.Perfil, nome, bytes)
	if err != nil {
		return Resposta{}, erroDoPreencher(err)
	}
	log.Printf("preencher: %d campo(s), currículo %t, %d obrigatório(s) em branco", r.Preenchidos, r.Anexou, r.Faltando) // só números
	return ok(r)
}

// erroDoPreencher: os erros fixos do envio passam; o resto vira uma frase fixa (o log leva só o tipo).
func erroDoPreencher(err error) error {
	var ec *dominio.ErroConflito
	if errors.As(err, &ec) {
		return err
	}
	log.Printf("preencher: falhou (%s)", tipoDoErro(err))
	return &dominio.ErroConflito{Msg: "não deu para usar a janela agora; tente de novo", Motivo: "falhou"}
}
