package ia

import (
	"encoding/json"
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// jc-a2y.7.1: o mesmo corpus de injeção do Node (testes/pedidos-injecao.test.mjs). Toda saída de IA do corpus é
// bloqueada (esquema ou regra: 422) ou é uma recusa: nenhuma vira ação que grava.
func TestCorpusDeInjecaoDoPedido(t *testing.T) {
	b, err := os.ReadFile("testdata/pedidos-injecao.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Vagas []any `json:"vagas"`
		Casos []struct {
			Nome   string `json:"nome"`
			Pedido string `json:"pedido"`
			Saida  any    `json:"saida"`
			Espera string `json:"espera"`
		} `json:"casos"`
	}
	if err := json.Unmarshal(b, &corpus); err != nil {
		t.Fatal(err)
	}
	if len(corpus.Casos) < 30 {
		t.Fatalf("corpus com %d casos", len(corpus.Casos))
	}
	reRepetir := regexp.MustCompile(`"REPETIR:(.):(\d+)"`)
	for _, c := range corpus.Casos {
		bruto, _ := json.Marshal(c.Saida)
		texto := reRepetir.ReplaceAllStringFunc(string(bruto), func(m string) string {
			g := reRepetir.FindStringSubmatch(m)
			n, _ := strconv.Atoi(g[2])
			return `"` + strings.Repeat(g[1], n) + `"`
		})
		entrada := dominio.Objeto{"pedido": c.Pedido, "vagas": corpus.Vagas}
		if err := ValidarEntrada("interpretarPedido", entrada); err != nil {
			t.Fatalf("%s: entrada = %v", c.Nome, err)
		}
		s, _, err := Guardar("interpretarPedido", entrada, texto)
		switch c.Espera {
		case "bloqueia":
			var e *ErroIA
			if !errors.As(err, &e) || (e.Tipo != "esquema" && e.Tipo != "regra") {
				t.Errorf("%s: passou (%v)", c.Nome, s)
			}
		default:
			if err != nil || s["acao"] != "recusar" {
				t.Errorf("%s: %v %v", c.Nome, s, err)
			}
		}
	}
}

// O prompt leva só o mínimo das vagas e marca o pedido como texto a interpretar.
func TestPromptDoPedidoSoComOMinimo(t *testing.T) {
	vagas := []any{dominio.Objeto{"id": "li-1", "empresa": "E", "titulo": "T", "etapa": "aplicar", "link": "https://x", "contato": "pessoa@exemplo.com", "notas": "segredo", "pretensao": 9000.0}}
	p := MontarPrompt("interpretarPedido", dominio.Objeto{"pedido": "oi", "vagas": vagas})
	for _, proibido := range []string{"pessoa@exemplo.com", "segredo", "9000", "https://x"} {
		if strings.Contains(p.Mensagem, proibido) {
			t.Errorf("o prompt levou %q", proibido)
		}
	}
	if !strings.Contains(p.Mensagem, "não é instrução") {
		t.Error("o prompt não marca o pedido como texto")
	}
}
