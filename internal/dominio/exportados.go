package dominio

// Auxiliares com a semântica do JavaScript, expostos para a API (internal/api) usar as mesmas regras.

// ObjetoDe devolve v como Objeto, ou nil.
func ObjetoDe(v any) Objeto { return obj(v) }

// ListaDe devolve v como lista, ou nil.
func ListaDe(v any) []any { return lista(v) }

// Texto imita String(v ?? ”).
func Texto(v any) string { return texto(v) }

// JSString imita String(v).
func JSString(v any) string { return jsString(v) }

// CampoJS imita `${o.k}` (ausente vira "undefined").
func CampoJS(o Objeto, k string) string { return campo(o, k) }

// NumeroDe imita Number(v); ok=false quando seria NaN.
func NumeroDe(v any) (float64, bool) { return numeroDe(v) }

// Verdadeiro imita o "truthy" do JavaScript.
func Verdadeiro(v any) bool { return verdadeiro(v) }

// Igual imita a === b.
func Igual(a, b any) bool { return igual(a, b) }

// Ou imita a ?? b.
func Ou(a, b any) any { return ou(a, b) }

// OuV imita a || b.
func OuV(a, b any) any { return ouV(a, b) }
