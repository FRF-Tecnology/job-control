package ia

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Objeto ordenado: guarda a ordem das chaves (o JSON que a IA devolve e o resumo do perfil, que tem ordem fixa).
type ordenado struct {
	chaves  []string
	valores map[string]any
}

func (o *ordenado) por(k string, v any) { o.chaves = append(o.chaves, k); o.valores[k] = v }

func novoOrdenado() *ordenado { return &ordenado{valores: map[string]any{}} }

// lerOrdenado decodifica JSON guardando a ordem das chaves dos objetos.
func lerOrdenado(dados []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(dados))
	dec.UseNumber()
	v, err := lerValor(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err == nil {
		return nil, fmt.Errorf("sobrou conteúdo depois do JSON")
	}
	return v, nil
}

func lerValor(dec *json.Decoder) (any, error) {
	t, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch x := t.(type) {
	case json.Delim:
		switch x {
		case '{':
			o := novoOrdenado()
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				k := kt.(string)
				v, err := lerValor(dec)
				if err != nil {
					return nil, err
				}
				if _, existe := o.valores[k]; existe { // chave repetida: a última vence, na posição da primeira (como no JS)
					o.valores[k] = v
					continue
				}
				o.por(k, v)
			}
			_, err := dec.Token()
			return o, err
		case '[':
			l := []any{}
			for dec.More() {
				v, err := lerValor(dec)
				if err != nil {
					return nil, err
				}
				l = append(l, v)
			}
			_, err := dec.Token()
			return l, err
		}
	case json.Number:
		f, err := x.Float64()
		return f, err
	}
	return t, nil
}

// simples converte objetos ordenados em mapas comuns (o resto do código trabalha com dominio.Objeto).
func simples(v any) any {
	switch x := v.(type) {
	case *ordenado:
		m := make(map[string]any, len(x.chaves))
		for _, k := range x.chaves {
			m[k] = simples(x.valores[k])
		}
		return m
	case []any:
		l := make([]any, len(x))
		for i, e := range x {
			l[i] = simples(e)
		}
		return l
	}
	return v
}

// stringify imita JSON.stringify(v, null, 1) (recuo de 1 espaço) ou JSON.stringify(v) (recuo vazio).
// Mapas comuns saem com as chaves em ordem alfabética; objetos ordenados, na ordem guardada.
func stringify(v any, recuo string) string {
	var b strings.Builder
	escrever(&b, v, recuo, "")
	return b.String()
}

func escrever(b *strings.Builder, v any, recuo, atual string) {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case float64, int:
		b.WriteString(dominio.JSString(x))
	case string:
		escreverTexto(b, x)
	case []any:
		if len(x) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			if recuo != "" {
				b.WriteString("\n" + atual + recuo)
			}
			escrever(b, e, recuo, atual+recuo)
		}
		if recuo != "" {
			b.WriteString("\n" + atual)
		}
		b.WriteByte(']')
	case []string:
		l := make([]any, len(x))
		for i, s := range x {
			l[i] = s
		}
		escrever(b, l, recuo, atual)
	case map[string]any:
		chaves := make([]string, 0, len(x))
		for k := range x {
			chaves = append(chaves, k)
		}
		sort.Strings(chaves)
		escreverObjeto(b, chaves, x, recuo, atual)
	case *ordenado:
		escreverObjeto(b, x.chaves, x.valores, recuo, atual)
	default:
		bs, _ := json.Marshal(x)
		b.Write(bs)
	}
}

func escreverObjeto(b *strings.Builder, chaves []string, valores map[string]any, recuo, atual string) {
	if len(chaves) == 0 {
		b.WriteString("{}")
		return
	}
	b.WriteByte('{')
	for i, k := range chaves {
		if i > 0 {
			b.WriteByte(',')
		}
		if recuo != "" {
			b.WriteString("\n" + atual + recuo)
		}
		escreverTexto(b, k)
		b.WriteByte(':')
		if recuo != "" {
			b.WriteByte(' ')
		}
		escrever(b, valores[k], recuo, atual+recuo)
	}
	if recuo != "" {
		b.WriteString("\n" + atual)
	}
	b.WriteByte('}')
}

// escreverTexto escapa como o JSON.stringify: aspas, barra invertida e caracteres de controle.
func escreverTexto(b *strings.Builder, s string) {
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
			if r < 0x20 || r == utf8.RuneError {
				fmt.Fprintf(b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}
