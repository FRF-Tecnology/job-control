package api

import (
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/conta"
)

// Trava das ações: sem licença válida (o teste acabou, sem assinatura, sem internet ou sem conta), toda escrita dá
// 402 com a frase fixa do motivo, venha da tela, da CLI ou do MCP. Ver continua liberado.

// frasesDaTrava: a frase fixa de cada motivo de conta.PodeAgir.
var frasesDaTrava = map[string]string{
	conta.MotivoTesteAcabou:   "Seu teste acabou. Você ainda vê tudo; para buscar e se candidatar, assine.",
	conta.MotivoSemAssinatura: "Sua assinatura não está ativa. Você ainda vê tudo; para buscar e se candidatar, assine.",
	conta.MotivoSemInternet:   "Sem conexão: dá para ver, mas não para buscar ou se candidatar.",
	conta.MotivoSemConta:      "Entre na sua conta para buscar e se candidatar.",
}

// liberadoSemLicenca: a lista "Sempre liberado". Ler (GET, HEAD, OPTIONS), a conta, a assinatura, apagar os dados, a
// janela do app e as chaves de IA (a pessoa precisa delas para sair da trava ou levar os dados embora).
func liberadoSemLicenca(metodo, caminho string) bool {
	switch strings.ToUpper(metodo) {
	case "GET", "HEAD", "OPTIONS":
		return true
	}
	c, _, _ := strings.Cut(caminho, "?")
	if !strings.HasPrefix(c, Prefixo+"/") {
		return false // fora da API: o rotear responde 404
	}
	p := strings.TrimPrefix(c, Prefixo)
	switch p {
	case "/conta", "/assinatura", "/dados/apagar", "/app/janela", "/app/janela/padrao":
		return true
	}
	return strings.HasPrefix(p, "/conta/") || strings.HasPrefix(p, "/assinatura/") || reIAChave.MatchString(p) // reIAChave: ia.go
}

// respostaTrava: o 402 com a frase fixa do motivo. Motivo desconhecido trava do mesmo jeito (falha fechada), com a
// frase de quem não tem assinatura; o motivo vai como veio.
func respostaTrava(motivo string) Resposta {
	frase, ok := frasesDaTrava[motivo]
	if !ok {
		frase = frasesDaTrava[conta.MotivoSemAssinatura]
	}
	return Resposta{Status: 402, Corpo: Objeto{"erro": frase, "motivo": motivo}}
}

// travado: a resposta 402 quando este pedido é uma escrita e a licença não deixa agir. podeAgir nil: tudo liberado.
func (a *API) travado(p Pedido) (Resposta, bool) {
	if a.podeAgir == nil || liberadoSemLicenca(p.Metodo, p.Caminho) {
		return Resposta{}, false
	}
	if pode, motivo := a.podeAgir(); !pode {
		return respostaTrava(motivo), true
	}
	return Resposta{}, false
}
