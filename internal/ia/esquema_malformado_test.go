package ia

import (
	"encoding/json"
	"strings"
	"testing"
)

// jc-a2y.15: um campo malformado no contrato não é ignorado em silêncio (enfraqueceria a validação): falha.
func TestEsquemaMalformadoFalha(t *testing.T) {
	for _, bruto := range []string{
		`{"type": 3}`,
		`{"enum": "aplicar"}`,
		`{"type": "object", "properties": ["decisao"]}`,
		`{"type": "object", "required": "decisao"}`,
	} {
		func() {
			defer func() {
				r := recover()
				if r == nil || !strings.Contains(r.(string), "malformado") {
					t.Errorf("%s: deveria falhar com campo malformado, veio %v", bruto, r)
				}
			}()
			lerEsquema(json.RawMessage(bruto))
		}()
	}
	// O contrato de verdade continua válido (o pacote iniciou, e todas as funções têm esquema).
	if len(contratos.Funcoes) == 0 {
		t.Fatal("contratos vazios")
	}
}
