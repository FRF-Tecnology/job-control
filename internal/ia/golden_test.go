package ia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

// testdata/golden-ia.json foi gravado da IA do Node (ferramentas/gerar-golden-ia.mjs).
// Prompts, guarda e pedidos dos provedores em Go precisam bater com ele.

type erroGolden struct {
	Tipo         string   `json:"tipo"`
	Mensagem     string   `json:"mensagem"`
	Regra        *string  `json:"regra"`
	Erros        []string `json:"erros"`
	Reintentavel *bool    `json:"reintentavel"`
}

var golden struct {
	Regras []struct {
		ID      string `json:"id"`
		Funcoes any    `json:"funcoes"`
	} `json:"regras"`
	Prompts []struct {
		ID      string `json:"id"`
		Funcao  string `json:"funcao"`
		Entrada Objeto `json:"entrada"`
		Prompt  struct {
			Sistema     string          `json:"sistema"`
			Mensagem    string          `json:"mensagem"`
			Esquema     json.RawMessage `json:"esquema"`
			NomeEsquema string          `json:"nomeEsquema"`
		} `json:"prompt"`
	} `json:"prompts"`
	Entradas []struct {
		Funcao  string      `json:"funcao"`
		Entrada Objeto      `json:"entrada"`
		Erro    *erroGolden `json:"erro"`
	} `json:"entradas"`
	EntradasGuarda []Objeto `json:"entradasGuarda"`
	Guarda         []struct {
		Funcao    string `json:"funcao"`
		Entrada   int    `json:"entrada"`
		Bruto     string `json:"bruto"`
		Resultado *struct {
			Saida     any      `json:"saida"`
			Aplicadas []string `json:"aplicadas"`
		} `json:"resultado"`
		Erro *erroGolden `json:"erro"`
	} `json:"guarda"`
	Provedores []struct {
		Provedor string         `json:"provedor"`
		Modelo   string         `json:"modelo"`
		Chave    string         `json:"chave"`
		Opcoes   OpcoesProvedor `json:"opcoes"`
		Prompt   Prompt         `json:"prompt"`
		Resposta any            `json:"resposta"`
		Pedido   struct {
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
			Corpo   any               `json:"corpo"`
		} `json:"pedido"`
		Resultado *RespostaProvedor `json:"resultado"`
		Erro      *erroGolden       `json:"erro"`
	} `json:"provedores"`
	ErrosHTTP []struct {
		Status int         `json:"status"`
		Corpo  string      `json:"corpo"`
		Chave  string      `json:"chave"`
		Erro   *erroGolden `json:"erro"`
	} `json:"errosHttp"`
	Custos []struct {
		Modelo  string   `json:"modelo"`
		Entrada *int     `json:"entrada"`
		Saida   *int     `json:"saida"`
		Custo   *float64 `json:"custo"`
	} `json:"custos"`
}

func TestMain(m *testing.M) {
	b, err := os.ReadFile("testdata/golden-ia.json")
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(b, &golden); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func paraJSON(v any) any {
	b, _ := json.Marshal(v)
	var s any
	json.Unmarshal(b, &s)
	return s
}

func conferirErro(t *testing.T, err error, g *erroGolden) {
	t.Helper()
	var e *ErroIA
	if !errors.As(err, &e) {
		t.Fatalf("queria ErroIA %s %q; veio %v", g.Tipo, g.Mensagem, err)
	}
	if e.Tipo != g.Tipo || e.Mensagem != g.Mensagem {
		t.Fatalf("erro = %s %q\n  queria %s %q", e.Tipo, e.Mensagem, g.Tipo, g.Mensagem)
	}
	if g.Regra != nil && e.Regra != *g.Regra {
		t.Fatalf("regra = %q; queria %q", e.Regra, *g.Regra)
	}
	if g.Erros != nil && !reflect.DeepEqual(e.Erros, g.Erros) {
		t.Fatalf("erros = %q\n  queria %q", e.Erros, g.Erros)
	}
	if g.Reintentavel != nil && e.Reintentavel != *g.Reintentavel {
		t.Fatalf("reintentável = %v; queria %v", e.Reintentavel, *g.Reintentavel)
	}
}

func TestRegrasNaMesmaOrdem(t *testing.T) {
	if len(Regras) != len(golden.Regras) {
		t.Fatalf("%d regras; o Node tem %d", len(Regras), len(golden.Regras))
	}
	for i, r := range golden.Regras {
		if Regras[i].ID != r.ID {
			t.Errorf("regra %d = %s; queria %s", i, Regras[i].ID, r.ID)
		}
	}
}

func TestPromptsIguaisAoNode(t *testing.T) {
	for _, c := range golden.Prompts {
		t.Run(c.ID, func(t *testing.T) {
			if err := ValidarEntrada(c.Funcao, c.Entrada); err != nil {
				t.Fatalf("entrada recusada: %v", err)
			}
			p := MontarPrompt(c.Funcao, c.Entrada)
			if p.Sistema != c.Prompt.Sistema {
				t.Fatalf("sistema diferente:\n%s", diferencaTexto(p.Sistema, c.Prompt.Sistema))
			}
			if p.Mensagem != c.Prompt.Mensagem {
				t.Fatalf("mensagem diferente:\n%s", diferencaTexto(p.Mensagem, c.Prompt.Mensagem))
			}
			if !reflect.DeepEqual(paraJSON(p.Esquema), paraJSON(c.Prompt.Esquema)) || p.NomeEsquema != c.Prompt.NomeEsquema {
				t.Fatal("esquema diferente")
			}
		})
	}
}

func TestEntradasInvalidas(t *testing.T) {
	for i, c := range golden.Entradas {
		t.Run(fmt.Sprintf("%d-%s", i, c.Funcao), func(t *testing.T) {
			err := ValidarEntrada(c.Funcao, c.Entrada)
			if c.Erro == nil {
				if err != nil {
					t.Fatalf("queria aceitar; veio %v", err)
				}
				return
			}
			conferirErro(t, err, c.Erro)
		})
	}
}

func TestGuardaIgualAoNode(t *testing.T) {
	for i, c := range golden.Guarda {
		t.Run(fmt.Sprintf("%03d-%s", i, c.Funcao), func(t *testing.T) {
			saida, aplicadas, err := Guardar(c.Funcao, golden.EntradasGuarda[c.Entrada], c.Bruto)
			if c.Erro != nil {
				conferirErro(t, err, c.Erro)
				return
			}
			if err != nil {
				t.Fatalf("erro inesperado: %v\nbruto: %s", err, c.Bruto)
			}
			if !reflect.DeepEqual(paraJSON(saida), paraJSON(c.Resultado.Saida)) {
				t.Fatalf("saída diferente\nGo:   %v\nNode: %v\nbruto: %s", paraJSON(saida), c.Resultado.Saida, c.Bruto)
			}
			if !reflect.DeepEqual(aplicadas, c.Resultado.Aplicadas) && !(len(aplicadas) == 0 && len(c.Resultado.Aplicadas) == 0) {
				t.Fatalf("regras aplicadas = %v; queria %v", aplicadas, c.Resultado.Aplicadas)
			}
		})
	}
}

// Servidor falso no lugar do provedor: guarda o pedido e devolve a resposta do golden.
func TestPedidosDosProvedores(t *testing.T) {
	for i, c := range golden.Provedores {
		t.Run(fmt.Sprintf("%d-%s", i, c.Provedor), func(t *testing.T) {
			var url string
			var cabecalhos http.Header
			var corpo any
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				url, cabecalhos = r.URL.Path, r.Header
				b, _ := io.ReadAll(r.Body)
				json.Unmarshal(b, &corpo)
				json.NewEncoder(w).Encode(c.Resposta)
			}))
			defer srv.Close()
			opcoes := c.Opcoes
			base := c.Pedido.URL[:strings.Index(c.Pedido.URL[8:], "/")+8]
			opcoes.URL = srv.URL
			prov := Nova(Config{}, nil, nil).Provedores[c.Provedor]
			r, err := prov.Chamar(context.Background(), PedidoProvedor{Modelo: c.Modelo, Chave: c.Chave, Opcoes: opcoes, Prompt: c.Prompt})
			if url != strings.TrimPrefix(c.Pedido.URL, base) {
				t.Errorf("caminho = %s; queria %s", url, strings.TrimPrefix(c.Pedido.URL, base))
			}
			for k, v := range c.Pedido.Headers {
				if cabecalhos.Get(k) != v {
					t.Errorf("cabeçalho %s = %q; queria %q", k, cabecalhos.Get(k), v)
				}
			}
			if !reflect.DeepEqual(corpo, c.Pedido.Corpo) {
				t.Errorf("corpo diferente\nGo:   %v\nNode: %v", corpo, c.Pedido.Corpo)
			}
			if c.Erro != nil {
				conferirErro(t, err, c.Erro)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(paraJSON(r), paraJSON(c.Resultado)) {
				t.Errorf("leitura = %v; queria %v", paraJSON(r), paraJSON(c.Resultado))
			}
		})
	}
}

func TestErrosHTTPeChave(t *testing.T) {
	p := MontarPrompt("triarVaga", Objeto{"vaga": Objeto{}, "perfil": Objeto{}})
	for i, c := range golden.ErrosHTTP {
		t.Run(fmt.Sprint(i, "-", c.Status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(c.Status)
				io.WriteString(w, c.Corpo)
			}))
			defer srv.Close()
			chave := "chave-anthropic"
			if c.Chave != "" {
				chave = c.Chave
			}
			if c.Status == 0 {
				chave = ""
			}
			// O cliente de verdade da IA (com a política de não seguir redirecionamento).
			_, err := Nova(Config{}, nil, nil).Provedores["anthropic"].Chamar(context.Background(), PedidoProvedor{Modelo: "m", Chave: chave, Opcoes: OpcoesProvedor{URL: srv.URL}, Prompt: p})
			conferirErro(t, err, c.Erro)
		})
	}
}

func TestCusto(t *testing.T) {
	for _, c := range golden.Custos {
		got := CustoEstimado(c.Modelo, c.Entrada, c.Saida)
		if (got == nil) != (c.Custo == nil) || (got != nil && *got != *c.Custo) {
			t.Errorf("%s: custo = %v; queria %v", c.Modelo, got, c.Custo)
		}
	}
}

func diferencaTexto(a, b string) string {
	la, lb := strings.Split(a, "\n"), strings.Split(b, "\n")
	for i := 0; i < len(la) && i < len(lb); i++ {
		if la[i] != lb[i] {
			return fmt.Sprintf("linha %d\nGo:   %q\nNode: %q", i+1, la[i], lb[i])
		}
	}
	return fmt.Sprintf("tamanhos: Go %d linhas, Node %d", len(la), len(lb))
}
