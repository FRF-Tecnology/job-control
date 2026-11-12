package curriculos

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"

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
		pagina, err := r.Page(i).GetPlainText(nil)
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
