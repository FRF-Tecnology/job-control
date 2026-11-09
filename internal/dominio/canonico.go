package dominio

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// JSONCanonico escreve v no JSON canônico da RFC 8785 (JCS): chaves em ordem de unidades UTF-16, sem espaços,
// números no formato do ECMAScript e strings com o escape mínimo. É o que a tela e o servidor usam para chegar à
// mesma impressão de uma candidatura (proposta da fila, R3). Aceita o que sai de json.Unmarshal (map, slice,
// string, float64, bool, nil) e também int.
func JSONCanonico(v any) ([]byte, error) {
	var b bytes.Buffer
	if err := escreverCanonico(&b, v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func escreverCanonico(b *bytes.Buffer, v any) error {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case string:
		escreverStringCanonica(b, x)
	case float64:
		s, err := numeroES(x)
		if err != nil {
			return err
		}
		b.WriteString(s)
	case int:
		return escreverCanonico(b, float64(x))
	case int64:
		return escreverCanonico(b, float64(x))
	case Objeto: // Objeto é map[string]any
		return escreverObjetoCanonico(b, x)
	case []any:
		b.WriteByte('[')
		for i, y := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := escreverCanonico(b, y); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case []Objeto:
		l := make([]any, len(x))
		for i, o := range x {
			l[i] = o
		}
		return escreverCanonico(b, l)
	default:
		return fmt.Errorf("JSON canônico: tipo não suportado %T", v)
	}
	return nil
}

func escreverObjetoCanonico(b *bytes.Buffer, o map[string]any) error {
	chaves := make([]string, 0, len(o))
	for k := range o {
		chaves = append(chaves, k)
	}
	sort.Slice(chaves, func(i, j int) bool { return menorUTF16(chaves[i], chaves[j]) })
	b.WriteByte('{')
	for i, k := range chaves {
		if i > 0 {
			b.WriteByte(',')
		}
		escreverStringCanonica(b, k)
		b.WriteByte(':')
		if err := escreverCanonico(b, o[k]); err != nil {
			return err
		}
	}
	b.WriteByte('}')
	return nil
}

// menorUTF16 compara pelas unidades UTF-16, como a RFC 8785 manda (não pela ordem de bytes do UTF-8).
func menorUTF16(a, b string) bool {
	ua, ub := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(ua) && i < len(ub); i++ {
		if ua[i] != ub[i] {
			return ua[i] < ub[i]
		}
	}
	return len(ua) < len(ub)
}

// escreverStringCanonica: escape do JSON.stringify do ECMAScript. Só " \ e os controles abaixo de 0x20; o resto,
// inclusive /, < > & e não-ASCII, vai como está.
func escreverStringCanonica(b *bytes.Buffer, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

// numeroES formata como o Number.prototype.toString do ECMAScript (RFC 8785, seção 3.2.2.3).
func numeroES(f float64) (string, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "", fmt.Errorf("JSON canônico: número inválido %v", f)
	}
	if f == 0 {
		return "0", nil // inclusive -0
	}
	sinal := ""
	if f < 0 {
		sinal, f = "-", -f
	}
	// Menor representação exata: dígitos e expoente decimal.
	e := strconv.FormatFloat(f, 'e', -1, 64) // ex.: 3.333333333333333e+08
	mant, expS, _ := strings.Cut(e, "e")
	digitos := strings.Replace(mant, ".", "", 1)
	exp, _ := strconv.Atoi(expS)
	k, n := len(digitos), exp+1 // n = posição da vírgula
	var s string
	switch {
	case k <= n && n <= 21:
		s = digitos + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		s = digitos[:n] + "." + digitos[n:]
	case -6 < n && n <= 0:
		s = "0." + strings.Repeat("0", -n) + digitos
	default:
		d := n - 1
		sinalExp := "+"
		if d < 0 {
			sinalExp, d = "-", -d
		}
		if k == 1 {
			s = digitos + "e" + sinalExp + strconv.Itoa(d)
		} else {
			s = digitos[:1] + "." + digitos[1:] + "e" + sinalExp + strconv.Itoa(d)
		}
	}
	return sinal + s, nil
}

// VersaoImpressao entra no prefixo: se o formato do JSON canônico mudar, as aprovações antigas deixam de valer.
const VersaoImpressao = "v1"

// ImpressaoCandidatura é o sha256 do JSON canônico de {vagaId, plataforma, url, conteudo}, com o prefixo
// "sha256:v1:". A aprovação vale para essa impressão; qualquer mudança no pacote muda a impressão.
func ImpressaoCandidatura(vagaID, plataforma, url string, conteudo Objeto) (string, error) {
	b, err := JSONCanonico(Objeto{"vagaId": vagaID, "plataforma": plataforma, "url": url, "conteudo": conteudo})
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return "sha256:" + VersaoImpressao + ":" + hex.EncodeToString(h[:]), nil
}
