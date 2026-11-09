package ia

import (
	"encoding/json"
	"os"
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

// jc-a2y.60: o que o validador não aplicaria (palavra-chave desconhecida, additionalProperties diferente de false,
// required sem properties...) também falha ao iniciar. Os mesmos casos valem para o Node (testes/ia.test.mjs).
func TestEsquemaComRegraNaoTratadaFalha(t *testing.T) {
	b, err := os.ReadFile("testdata/esquemas-recusados.json")
	if err != nil {
		t.Fatal(err)
	}
	var casos struct {
		Recusados []struct {
			Nome    string          `json:"nome"`
			Esquema json.RawMessage `json:"esquema"`
			Contem  string          `json:"contem"`
		} `json:"recusados"`
		Aceitos []json.RawMessage `json:"aceitos"`
	}
	if err := json.Unmarshal(b, &casos); err != nil {
		t.Fatal(err)
	}
	if len(casos.Recusados) == 0 || len(casos.Aceitos) == 0 {
		t.Fatal("sem casos")
	}
	for _, c := range casos.Recusados {
		func() {
			defer func() {
				r := recover()
				msg, _ := r.(string)
				if !strings.Contains(msg, c.Contem) {
					t.Errorf("%s: deveria falhar com %q, veio %v", c.Nome, c.Contem, r)
				}
			}()
			lerEsquema(c.Esquema)
		}()
	}
	for _, a := range casos.Aceitos {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("%s: deveria aceitar, veio %v", a, r)
				}
			}()
			lerEsquema(a)
		}()
	}
}
