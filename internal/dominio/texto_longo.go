package dominio

import (
	"strings"
	"unicode"
)

// TextoLongo arruma um texto longo de terceiro (a descrição de uma vaga) para guardar e mostrar:
//   - mantém os parágrafos (no máximo uma linha em branco entre eles) e junta os espaços dentro de cada linha;
//   - tira < e >, para o texto nunca virar marcação;
//   - acima do teto (em caracteres), corta no fim da última palavra inteira e termina com "…", em vez de parar no
//     meio de uma palavra.
func TextoLongo(s string, teto int) string {
	s = strings.NewReplacer("<", "", ">", "", "\r\n", "\n", "\r", "\n").Replace(s)
	var linhas []string
	branco := false
	for _, l := range strings.Split(s, "\n") {
		l = strings.Join(strings.Fields(l), " ")
		if l == "" {
			branco = len(linhas) > 0
			continue
		}
		if branco {
			linhas = append(linhas, "")
			branco = false
		}
		linhas = append(linhas, l)
	}
	s = strings.Join(linhas, "\n")
	r := []rune(s)
	if teto <= 0 || len(r) <= teto {
		return s
	}
	r = r[:teto-1] // sobra lugar para o "…"
	corte := len(r)
	for corte > 0 && !unicode.IsSpace(r[corte-1]) {
		corte--
	}
	if corte > teto/2 { // uma "palavra" do tamanho do texto inteiro (um link, um código): corta onde der
		r = r[:corte]
	}
	return strings.TrimRightFunc(string(r), func(c rune) bool { return unicode.IsSpace(c) || unicode.IsPunct(c) && c != ')' }) + "…"
}
