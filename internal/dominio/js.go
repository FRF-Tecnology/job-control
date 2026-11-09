// Package dominio é o porte em Go das regras de negócio de nucleo/ (Node).
//
// Vagas, eventos, pedidos e emails são objetos JSON flexíveis (Objeto), como no Node:
// guardam campos livres vindos das buscas e aceitam null explícito. Os auxiliares deste
// arquivo imitam a semântica do JavaScript que o código original usa (??, ||, String()),
// para o Go dar exatamente as mesmas respostas (ver testdata/golden.json).
package dominio

import (
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"
)

// Objeto é um objeto JSON: valores são nil, bool, float64, string, []any ou map[string]any.
type Objeto = map[string]any

// obj devolve v como Objeto, ou nil se não for objeto.
func obj(v any) Objeto {
	o, _ := v.(map[string]any)
	return o
}

// lista devolve v como lista, ou nil.
func lista(v any) []any {
	l, _ := v.([]any)
	return l
}

// verdadeiro imita o "truthy" do JavaScript.
func verdadeiro(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case float64:
		return x != 0 && !math.IsNaN(x)
	case int:
		return x != 0
	case string:
		return x != ""
	default:
		return true
	}
}

// ou imita a ?? b: devolve b quando a é nulo (ausente ou null).
func ou(a, b any) any {
	if a == nil {
		return b
	}
	return a
}

// ouV imita a || b.
func ouV(a, b any) any {
	if verdadeiro(a) {
		return a
	}
	return b
}

// texto imita String(v ?? ''): nulo vira "", o resto vira o texto que o JS mostraria.
func texto(v any) string {
	if v == nil {
		return ""
	}
	return jsString(v)
}

// jsString imita String(v) para um valor presente (nil = null).
func jsString(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return numeroJS(x)
	case int:
		return strconv.Itoa(x)
	case []any:
		partes := make([]string, len(x))
		for i, e := range x {
			partes[i] = texto(e)
		}
		return strings.Join(partes, ",")
	default:
		return "[object Object]"
	}
}

// campo imita `${o.k}` num template: ausente vira "undefined", null vira "null".
func campo(o Objeto, k string) string {
	v, ok := o[k]
	if !ok {
		return "undefined"
	}
	return jsString(v)
}

// numeroJS imita Number.prototype.toString().
func numeroJS(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "Infinity"
	case math.IsInf(f, -1):
		return "-Infinity"
	case f == 0:
		return "0"
	}
	a := math.Abs(f)
	if a >= 1e21 || a < 1e-6 {
		s := strconv.FormatFloat(f, 'e', -1, 64)
		return strings.Replace(strings.Replace(s, "e+0", "e+", 1), "e-0", "e-", 1)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// numeroDe imita Number(v). ok=false quando o resultado seria NaN.
func numeroDe(v any) (float64, bool) {
	switch x := v.(type) {
	case nil:
		return 0, true
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	case float64:
		return x, !math.IsNaN(x)
	case int:
		return float64(x), true
	case string:
		s := aparar(x)
		if s == "" {
			return 0, true
		}
		if s == "Infinity" || s == "+Infinity" {
			return math.Inf(1), true
		}
		if s == "-Infinity" {
			return math.Inf(-1), true
		}
		if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
			n, err := strconv.ParseInt(s[2:], 16, 64)
			return float64(n), err == nil
		}
		if strings.ContainsAny(s, "_xXpP") || strings.HasSuffix(s, "Inf") || strings.EqualFold(s, "nan") || strings.HasSuffix(strings.ToLower(s), "inf") || strings.HasSuffix(strings.ToLower(s), "infinity") {
			return math.NaN(), false
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return math.NaN(), false
		}
		return f, true
	case []any:
		switch len(x) {
		case 0:
			return 0, true
		case 1:
			return numeroDe(texto(x[0]))
		}
		return math.NaN(), false
	default:
		return math.NaN(), false
	}
}

// finito imita Number.isFinite(Number(v)).
func finito(v any) bool {
	f, ok := numeroDe(v)
	return ok && !math.IsInf(f, 0)
}

// aparar imita String.prototype.trim (espaços Unicode e o BOM).
func aparar(s string) string {
	return strings.TrimFunc(s, func(r rune) bool { return unicode.IsSpace(r) || r == 0xFEFF })
}

// igual imita a === b para valores JSON (objetos e listas nunca são iguais entre si).
func igual(a, b any) bool {
	switch x := a.(type) {
	case nil:
		return b == nil
	case string:
		y, ok := b.(string)
		return ok && x == y
	case float64:
		y, ok := b.(float64)
		return ok && x == y
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	default:
		return false
	}
}

func contem(l []string, v any) bool {
	s, ok := v.(string)
	if !ok {
		return false
	}
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// tamanhoJS é o .length do JavaScript (unidades UTF-16).
func tamanhoJS(s string) int { return len(utf16.Encode([]rune(s))) }

// cortarJS imita s.slice(0, n) (em unidades UTF-16).
func cortarJS(s string, n int) string {
	u := utf16.Encode([]rune(s))
	if len(u) <= n {
		return s
	}
	return string(utf16.Decode(u[:n]))
}

// clonar copia fundo um valor JSON.
func clonar(v any) any {
	switch x := v.(type) {
	case map[string]any:
		c := make(map[string]any, len(x))
		for k, e := range x {
			c[k] = clonar(e)
		}
		return c
	case []any:
		c := make([]any, len(x))
		for i, e := range x {
			c[i] = clonar(e)
		}
		return c
	default:
		return v
	}
}

func clonarObj(o Objeto) Objeto {
	if o == nil {
		return nil
	}
	return clonar(o).(map[string]any)
}

// copiaRasa imita {...o}.
func copiaRasa(o Objeto) Objeto {
	c := make(Objeto, len(o))
	for k, v := range o {
		c[k] = v
	}
	return c
}

// listaDeTextos imita Array.isArray(x) && x.every(i => typeof i === 'string').
func listaDeTextos(x any) bool {
	l, ok := x.([]any)
	if !ok {
		return false
	}
	for _, e := range l {
		if _, ok := e.(string); !ok {
			return false
		}
	}
	return true
}

// ehTextoOuNumero imita typeof v === 'string' || typeof v === 'number'.
func ehTextoOuNumero(v any) bool {
	switch v.(type) {
	case string, float64, int:
		return true
	}
	return false
}
