package ia

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Validador de um subconjunto de JSON Schema (type, enum, properties, required,
// additionalProperties:false, items), igual ao de nucleo/ia/erro.mjs.
type esquema struct {
	Tipos         []string
	Enum          []any
	temEnum       bool
	Propriedades  map[string]*esquema
	Obrigatorios  []string
	SemExtras     bool
	Itens         *esquema
	temPropriedad bool
}

func lerEsquema(bruto json.RawMessage) *esquema {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(bruto, &m); err != nil {
		panic("esquema: " + err.Error())
	}
	e := &esquema{}
	if t, ok := m["type"]; ok {
		var um string
		if json.Unmarshal(t, &um) == nil {
			e.Tipos = []string{um}
		} else {
			json.Unmarshal(t, &e.Tipos)
		}
	}
	if en, ok := m["enum"]; ok {
		e.temEnum = true
		json.Unmarshal(en, &e.Enum)
	}
	if p, ok := m["properties"]; ok {
		e.temPropriedad = true
		var props map[string]json.RawMessage
		json.Unmarshal(p, &props)
		e.Propriedades = map[string]*esquema{}
		for k, v := range props {
			e.Propriedades[k] = lerEsquema(v)
		}
	}
	if r, ok := m["required"]; ok {
		json.Unmarshal(r, &e.Obrigatorios)
	}
	if a, ok := m["additionalProperties"]; ok && string(a) == "false" {
		e.SemExtras = true
	}
	if i, ok := m["items"]; ok {
		e.Itens = lerEsquema(i)
	}
	return e
}

func tipoDe(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case []any:
		return "array"
	case float64:
		if !math.IsInf(x, 0) && x == math.Trunc(x) {
			return "integer"
		}
		return "number"
	case string:
		return "string"
	case bool:
		return "boolean"
	}
	return "object"
}

// chavesEmOrdem: objeto ordenado na ordem guardada; mapa comum em ordem alfabética.
func chavesEmOrdem(v any) ([]string, func(string) (any, bool)) {
	switch x := v.(type) {
	case *ordenado:
		return x.chaves, func(k string) (any, bool) { val, ok := x.valores[k]; return val, ok }
	case map[string]any:
		c := make([]string, 0, len(x))
		for k := range x {
			c = append(c, k)
		}
		sort.Strings(c)
		return c, func(k string) (any, bool) { val, ok := x[k]; return val, ok }
	}
	return nil, func(string) (any, bool) { return nil, false }
}

func validarEsquema(valor any, e *esquema, caminho string) []string {
	t := tipoDe(valor)
	if len(e.Tipos) > 0 {
		casa := false
		for _, x := range e.Tipos {
			if x == t || (x == "number" && t == "integer") {
				casa = true
			}
		}
		if !casa {
			return []string{fmt.Sprintf("%s: esperado %s, veio %s", caminho, strings.Join(e.Tipos, "|"), t)}
		}
	}
	erros := []string{}
	if e.temEnum {
		achou := false
		for _, x := range e.Enum {
			if igualJS(x, valor) {
				achou = true
			}
		}
		if !achou {
			erros = append(erros, fmt.Sprintf("%s: valor fora da lista (%s)", caminho, stringify(simples(valor), "")))
		}
	}
	if t == "object" && e.temPropriedad {
		chaves, pegar := chavesEmOrdem(valor)
		for _, k := range e.Obrigatorios {
			if _, tem := pegar(k); !tem {
				erros = append(erros, fmt.Sprintf("%s.%s: obrigatório", caminho, k))
			}
		}
		for _, k := range chaves {
			v, _ := pegar(k)
			if p, ok := e.Propriedades[k]; ok {
				erros = append(erros, validarEsquema(v, p, caminho+"."+k)...)
			} else if e.SemExtras {
				erros = append(erros, fmt.Sprintf("%s.%s: campo não previsto", caminho, k))
			}
		}
	}
	if t == "array" && e.Itens != nil {
		for i, v := range valor.([]any) {
			erros = append(erros, validarEsquema(v, e.Itens, fmt.Sprintf("%s[%d]", caminho, i))...)
		}
	}
	return erros
}

func igualJS(a, b any) bool {
	switch x := a.(type) {
	case string:
		y, ok := b.(string)
		return ok && x == y
	case float64:
		y, ok := b.(float64)
		return ok && x == y
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case nil:
		return b == nil
	}
	return false
}
