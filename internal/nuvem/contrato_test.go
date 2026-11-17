package nuvem

import (
	"os"
	"sort"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// O contrato (api/nuvem.yaml) e as rotas do servidor andam juntos. Este arquivo confere:
//   - toda rota registrada no ServeMux (método e caminho) está no contrato, e o contrato não tem operação que o
//     servidor não atende;
//   - toda operação tem respostas, e só as rotas que passam por sessoes.Middleware pedem bearer;
//   - toda referência $ref do contrato aponta para algo que existe.
//
// Limite: o teste não compara os esquemas dos corpos com o que cada rota devolve; isso se lê no código.

const caminhoContrato = "../../api/nuvem.yaml"

// comBearer são as rotas atrás de sessoes.Middleware (Authorization: Bearer).
var comBearer = map[string]bool{
	"GET /v1/licenca":      true,
	"POST /v1/assinar":     true,
	"POST /v1/portal":      true,
	"GET /v1/notas":        true,
	"DELETE /v1/contas/eu": true,
}

var metodosHTTP = map[string]bool{"get": true, "put": true, "post": true, "patch": true, "delete": true}

func lerContrato(t *testing.T) (map[string]any, map[string]map[string]any) {
	t.Helper()
	b, err := os.ReadFile(caminhoContrato)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatalf("nuvem.yaml inválido: %v", err)
	}
	ops := map[string]map[string]any{} // "MÉTODO /caminho" → operação
	caminhos, _ := doc["paths"].(map[string]any)
	for caminho, item := range caminhos {
		for m, op := range item.(map[string]any) {
			if !metodosHTTP[m] {
				continue
			}
			ops[strings.ToUpper(m)+" "+caminho] = op.(map[string]any)
		}
	}
	return doc, ops
}

func TestRotasDoServidorEstaoNoContrato(t *testing.T) {
	a := novoAmb(t)
	_, ops := lerContrato(t)
	rotas := a.s.padroes
	if len(rotas) < 15 {
		t.Fatalf("poucas rotas registradas (%d): %v", len(rotas), rotas)
	}
	no := map[string]bool{}
	for _, p := range rotas {
		no[p] = true
		if _, ok := ops[p]; !ok {
			t.Errorf("a rota %q do servidor não está em api/nuvem.yaml", p)
		}
	}
	var sobra []string
	for p := range ops {
		if !no[p] {
			sobra = append(sobra, p)
		}
	}
	sort.Strings(sobra)
	if len(sobra) > 0 {
		t.Errorf("api/nuvem.yaml tem operações que o servidor não atende: %v", sobra)
	}
}

func TestContratoEstruturaEAutenticacao(t *testing.T) {
	doc, ops := lerContrato(t)
	if v, _ := doc["openapi"].(string); !strings.HasPrefix(v, "3.1") {
		t.Errorf("openapi = %q, quer 3.1.x", v)
	}
	info, _ := doc["info"].(map[string]any)
	if info["title"] != "Applyer nuvem" || info["version"] != "0.1.0" {
		t.Errorf("info = %v", info)
	}
	for p, op := range ops {
		if r, _ := op["responses"].(map[string]any); len(r) == 0 {
			t.Errorf("%s sem respostas", p)
		}
		seg, tem := op["security"].([]any)
		if !tem {
			t.Errorf("%s sem security (use security: [] nas rotas públicas)", p)
		}
		pedeBearer := len(seg) == 1 && seg[0].(map[string]any)["bearerAuth"] != nil
		switch {
		case comBearer[p] && !pedeBearer:
			t.Errorf("%s deveria pedir bearer", p)
		case !comBearer[p] && pedeBearer:
			t.Errorf("%s não passa pelo middleware de sessão e não deveria pedir bearer", p)
		case p == "POST "+caminhoWebhook && (len(seg) != 1 || seg[0].(map[string]any)["stripeSignature"] == nil):
			t.Errorf("o webhook deve ser documentado como assinado pelo Stripe (stripeSignature)")
		}
	}
	for p := range comBearer {
		if _, ok := ops[p]; !ok {
			t.Errorf("%s ausente do contrato", p)
		}
	}
}

func TestContratoReferenciasResolvem(t *testing.T) {
	doc, _ := lerContrato(t)
	var visitar func(v any)
	visitar = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if ref, ok := x["$ref"].(string); ok {
				alvo := any(doc)
				for _, parte := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
					m, _ := alvo.(map[string]any)
					if alvo = m[parte]; alvo == nil {
						t.Errorf("$ref %q não existe", ref)
						break
					}
				}
			}
			for _, f := range x {
				visitar(f)
			}
		case []any:
			for _, f := range x {
				visitar(f)
			}
		}
	}
	visitar(doc)
}
