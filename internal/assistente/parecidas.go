package assistente

import (
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// A resposta padrão mais parecida com a pergunta do formulário, como na Node (mcp/ferramentas.mjs): sobreposição de
// palavras, com os termos técnicos pesando mais. Só sugere: a decisão é do modelo, e sem par o certo é 0 / Não.

var vazias = map[string]bool{}

func init() {
	for _, p := range strings.Fields("de da do com em a o e voce você sua seu qual quantos the you your how many do have with of in what is are for to work experience experiencia anos years") {
		vazias[p] = true
	}
}

var (
	reJS      = regexp.MustCompile(`\.js\b`)
	reSeparar = regexp.MustCompile(`[^a-z0-9+#.]+`)
	rePontos  = regexp.MustCompile(`\.+$`)
	reCurta   = regexp.MustCompile(`^[a-z]+$`)
)

// palavras: "React.js" = "React", "Node.js" = "Node"; ".NET" e "C#" ficam inteiros.
func palavras(s string) map[string]bool {
	n := reSeparar.ReplaceAllString(reJS.ReplaceAllString(dominio.Normalizar(s), ""), " ")
	saida := map[string]bool{}
	for _, p := range strings.Split(n, " ") {
		p = rePontos.ReplaceAllString(p, "")
		if p != "" && !vazias[p] {
			saida[p] = true
		}
	}
	return saida
}

// RespostasParecidas devolve até limite respostas padrão, da mais parecida para a menos.
func RespostasParecidas(respostasPadrao []any, pergunta string, limite int) []map[string]any {
	alvo := palavras(pergunta)
	if len(alvo) == 0 {
		return []map[string]any{}
	}
	type item struct {
		o    map[string]any
		nota float64
		i    int
	}
	var itens []item
	for i, x := range respostasPadrao {
		r := dominio.ObjetoDe(x)
		p := palavras(dominio.Texto(r["pergunta"]))
		soma := 0.0
		for w := range alvo {
			if p[w] {
				if reCurta.MatchString(w) && len(w) <= 3 {
					soma++
				} else {
					soma += 2
				}
			}
		}
		nota := math.Round(soma/float64(len(alvo)+len(p))*100) / 100
		if nota > 0 {
			itens = append(itens, item{map[string]any{"pergunta": r["pergunta"], "resposta": r["resposta"], "semelhanca": nota}, nota, i})
		}
	}
	sort.SliceStable(itens, func(a, b int) bool { return itens[a].nota > itens[b].nota })
	saida := []map[string]any{}
	for i := 0; i < len(itens) && i < limite; i++ {
		saida = append(saida, itens[i].o)
	}
	return saida
}
