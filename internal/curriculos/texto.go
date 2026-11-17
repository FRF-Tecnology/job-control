package curriculos

import (
	"bytes"
	"context"
	"errors"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
	leitorpdf "github.com/ledongthuc/pdf"
)

var ErrSemTexto = errors.New("o PDF não contém texto legível para sugerir o perfil")

var contatosCV = regexp.MustCompile(`(?i)[^\s@]+@[^\s@]+\.[^\s@]+|(?:https?://|www\.|linkedin\.com/|github\.com/)\S+|\+?\d[\d ().-]{7,}\d`)

// TextoProfissional lê a camada de texto localmente; o PDF e seu nome não entram no prompt.
func TextoProfissional(ctx context.Context, arquivo []byte) (texto string, err error) {
	defer func() {
		if recover() != nil {
			texto, err = "", ErrSemTexto
		}
	}()
	r, err := leitorpdf.NewReader(bytes.NewReader(arquivo), int64(len(arquivo)))
	if err != nil {
		return "", ErrSemTexto
	}
	const maxTexto = 32000 // caracteres de texto profissional para a chamada ao provedor
	restante := maxTexto
	var b strings.Builder
	for i, total := 1, min(r.NumPage(), 30); i <= total && restante > 0; i++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		pagina, err := textoDaPagina(r.Page(i))
		if err != nil {
			continue
		}
		for _, linha := range strings.Split(pagina, "\n") {
			switch dominio.MotivoDoRotulo(linha) {
			case "cpf", "rg", "senha", "dado-bancario", "documento", "dado-sensivel":
				continue
			}
			if dominio.ContemDocumento(linha) {
				continue
			}
			limpa := strings.Join(strings.Fields(contatosCV.ReplaceAllString(linha, " ")), " ")
			if limpa == "" {
				continue
			}
			letras := []rune(limpa)
			n := min(len(letras), restante)
			b.WriteString(string(letras[:n]))
			b.WriteByte('\n')
			restante -= n + 1
			if restante <= 0 {
				break
			}
		}
	}
	if b.Len() == 0 {
		return "", ErrSemTexto
	}
	return strings.TrimSpace(b.String()), nil
}

// textoDaPagina usa o texto simples da página e, se ele vier colado, remonta os espaços pela posição de cada letra.
// Alguns geradores de PDF (Canva, certos modelos) desenham cada palavra no lugar sem gravar o caractere de espaço:
// o texto simples virava "SoftwareEngineerwith…" (um currículo real deu 5 "palavras" em 3,5 mil caracteres). Com o
// texto colado, as citações da IA (com espaços) não batiam com o texto e todas as sugestões eram descartadas, e o
// filtro de contatos e documentos, que trabalha por linha, ficava com poucas linhas enormes.
func textoDaPagina(p leitorpdf.Page) (string, error) {
	simples, err := p.GetPlainText(nil)
	if err != nil || !colado(simples) {
		return simples, err
	}
	if remontado := textoPorPosicao(p.Content().Text); !colado(remontado) {
		return remontado, nil
	}
	return simples, nil
}

// colado: texto longo com "palavras" de mais de 20 letras em média (texto comum fica perto de 6).
func colado(s string) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(s))
	palavras := len(strings.Fields(s))
	return n >= 200 && palavras > 0 && n/palavras > 20
}

// textoPorPosicao agrupa as letras em linhas pela altura (de cima para baixo), ordena cada linha da esquerda para a
// direita e põe espaço onde há um vão entre uma letra e a seguinte.
func textoPorPosicao(letras []leitorpdf.Text) string {
	letras = append([]leitorpdf.Text(nil), letras...)
	sort.SliceStable(letras, func(i, j int) bool { return letras[i].Y > letras[j].Y })
	var linhas [][]leitorpdf.Text
	for _, l := range letras {
		if n := len(linhas); n > 0 && math.Abs(linhas[n-1][0].Y-l.Y) <= math.Max(l.FontSize, 1)*0.6 {
			linhas[n-1] = append(linhas[n-1], l)
			continue
		}
		linhas = append(linhas, []leitorpdf.Text{l})
	}
	var b strings.Builder
	for _, linha := range linhas {
		sort.SliceStable(linha, func(i, j int) bool { return linha[i].X < linha[j].X })
		for i, l := range linha {
			if i > 0 && l.X-(linha[i-1].X+linha[i-1].W) > math.Max(l.FontSize, 1)*0.15 {
				b.WriteByte(' ')
			}
			b.WriteString(l.S)
		}
		b.WriteByte('\n')
	}
	return b.String()
}
