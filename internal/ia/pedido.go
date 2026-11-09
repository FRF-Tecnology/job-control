package ia

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Pedidos atendidos pelo app (jc-a2y.7.1, docs/propostas/pedidos-no-app.md): porte de nucleo/ia/funcoes.mjs
// (conferirParametros) e dos padrões da regra pedido-catalogo de nucleo/ia/guarda.mjs. A IA só PROPÕE uma ação do
// catálogo fechado; o servidor monta o resumo e a chamada, e nada grava sem a pessoa confirmar na tela.

// AcoesPedido: o catálogo fechado.
var AcoesPedido = []string{"adicionar-vaga", "registrar-etapa", "preparar-pacote", "responder", "recusar"}

// MotivosRecusa do pedido.
var MotivosRecusa = []string{"so-a-pessoa", "fora-do-app", "nao-entendi"}

// camposAcao: [obrigatórios, opcionais] de cada ação. Qualquer outro campo em parametros é recusado.
var camposAcao = map[string][2][]string{
	"adicionar-vaga":  {{"url"}, {"empresa", "titulo"}},
	"registrar-etapa": {{"vagaId", "etapa"}, {"motivo", "sub"}},
	"preparar-pacote": {{"vagaId"}, nil},
	"responder":       {{"texto"}, nil},
	"recusar":         {{"motivo"}, nil},
}

// maxPedido: tamanhos máximos em caracteres (pontos de código, como o [...s].length do Node), nesta ordem.
var maxPedido = []struct {
	campo string
	max   int
}{{"url", 500}, {"empresa", 120}, {"titulo", 120}, {"texto", 1500}, {"vagaId", 80}}

var (
	motivosPulo         = []string{"Sênior", "Inglês", "Diploma", "Stack", "Presencial/Local", "Documento", "Duplicada", "Vaga fechada", "Outro"}
	motivosEncerramento = []string{"Reprovada", "Sem retorno", "Vaga fechada", "Desisti", "Outro"}

	reVagaIDPedido    = regexp.MustCompile(`^[A-Za-z0-9._-]{1,80}$`)
	reInvisivel       = regexp.MustCompile(`[\x{00AD}\x{061C}\x{180E}\x{200B}-\x{200F}\x{202A}-\x{202E}\x{2060}-\x{2064}\x{2066}-\x{2069}\x{FEFF}]`)
	reEsquemaPerigoso = regexp.MustCompile(`(?i)\b(javascript|data|vbscript|file):`)
	reSegredo         = regexp.MustCompile(`(?i)\b(senha|password|contrase[nñ]a)\b|dados banc|conta banc|\biban\b|\bpix\b|\bswift\b|routing number`)
	// https, host só com ASCII (letras, dígitos, ponto e hífen), porta opcional, caminho só com ASCII visível sem
	// barra invertida. Sem usuário/senha (o @ cai no host). Sem // depois do host (conferido à parte).
	reURLPedido = regexp.MustCompile(`^https://[A-Za-z0-9.-]+(:\d{1,5})?(/[!-\[\]-~]*)?$`)
)

// URLSeguraPedido: o link de adicionar-vaga no formato seguro (o servidor nunca busca a URL).
func URLSeguraPedido(v any) bool {
	u, ok := v.(string)
	return ok && reURLPedido.MatchString(u) && !strings.Contains(u[len("https://"):], "//")
}

func contemTexto(l []string, s any) bool {
	t, ok := s.(string)
	if !ok {
		return false
	}
	for _, x := range l {
		if x == t {
			return true
		}
	}
	return false
}

// conferirParametros: as regras de cada ação que o esquema não expressa (igual ao Node, na mesma ordem das
// mensagens). Sem valores nas mensagens: a saída vem da IA (jc-a2y.23).
func conferirParametros(s Objeto) []string {
	erros := []string{}
	acao, _ := s["acao"].(string)
	campos, conhecida := camposAcao[acao]
	p, ehObjeto := s["parametros"].(map[string]any)
	if !conhecida || !ehObjeto {
		return erros // o esquema já reprovou
	}
	obrigatorios, opcionais := campos[0], campos[1]
	chaves := make([]string, 0, len(p))
	for k := range p {
		chaves = append(chaves, k)
	}
	sort.Strings(chaves)
	for _, k := range chaves {
		if !contem(obrigatorios, k) && !contem(opcionais, k) {
			erros = append(erros, "$.parametros: campo não previsto para a ação")
		}
	}
	for _, k := range obrigatorios {
		if v, tem := p[k]; !tem || v == nil || v == "" {
			erros = append(erros, fmt.Sprintf("$.parametros.%s: obrigatório para a ação", k))
		}
	}
	for _, m := range maxPedido {
		if v, ok := p[m.campo].(string); ok && utf8.RuneCountInString(v) > m.max {
			erros = append(erros, fmt.Sprintf("$.parametros.%s: maior que %d", m.campo, m.max))
		}
	}
	if id, ok := p["vagaId"].(string); ok && !reVagaIDPedido.MatchString(id) {
		erros = append(erros, "$.parametros.vagaId: fora do formato")
	}
	if acao == "recusar" && !contemTexto(MotivosRecusa, p["motivo"]) {
		erros = append(erros, "$.parametros.motivo: fora da lista da ação")
	}
	if acao == "registrar-etapa" {
		var pede []string
		switch p["etapa"] {
		case "pulada":
			pede = motivosPulo
		case "encerrada":
			pede = motivosEncerramento
		}
		if pede != nil && !contemTexto(pede, p["motivo"]) {
			erros = append(erros, "$.parametros.motivo: obrigatório e da lista da etapa")
		}
		if pede == nil && p["motivo"] != nil {
			erros = append(erros, "$.parametros.motivo: só em pulada ou encerrada")
		}
		if p["sub"] != nil && p["etapa"] != "em-processo" {
			erros = append(erros, "$.parametros.sub: só em em-processo")
		}
	}
	return erros
}
