package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
)

func TestMain(m *testing.M) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		panic(err)
	}
	time.Local = loc
	os.Exit(m.Run())
}

func novaAPI(t *testing.T, hoje *string, sufixo string) *API {
	t.Helper()
	arm, err := armazem.Abrir(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	return Nova(arm, Opcoes{Hoje: func() string { return *hoje }, Aleatorio: func() string { return sufixo }})
}

type chamadaGolden struct {
	Metodo   string `json:"metodo"`
	Caminho  string `json:"caminho"`
	Autor    string `json:"autor"`
	Hoje     string `json:"hoje"`
	Corpo    any    `json:"corpo"`
	TemCorpo bool   `json:"-"`
	Resposta struct {
		Status     int               `json:"status"`
		Corpo      any               `json:"corpo"`
		Tipo       *string           `json:"tipo"`
		Cabecalhos map[string]string `json:"cabecalhos"`
	} `json:"resposta"`
}

// A sessão de testdata/golden-api.json foi gravada contra a API Node (ferramentas/gerar-golden-api.mjs).
// A API Go, partindo de um banco vazio, precisa responder igual em cada chamada.
func TestGoldenDaApiNode(t *testing.T) {
	b, err := os.ReadFile("testdata/golden-api.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Sufixo   string            `json:"sufixo"`
		Chamadas []json.RawMessage `json:"chamadas"`
	}
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatal(err)
	}
	hoje := ""
	a := novaAPI(t, &hoje, g.Sufixo)
	for i, bruta := range g.Chamadas {
		var c chamadaGolden
		json.Unmarshal(bruta, &c)
		var campos map[string]json.RawMessage
		json.Unmarshal(bruta, &campos)
		_, c.TemCorpo = campos["corpo"]
		nome := fmt.Sprintf("%02d %s %s", i+1, c.Metodo, c.Caminho)
		hoje = c.Hoje
		corpo := c.Corpo
		if !c.TemCorpo {
			corpo = Objeto{}
		}
		r, err := a.Chamar(context.Background(), Pedido{Metodo: c.Metodo, Caminho: c.Caminho, Corpo: corpo, Autor: c.Autor})
		if err != nil {
			t.Fatalf("%s: erro inesperado: %v", nome, err)
		}
		if r.Status != c.Resposta.Status {
			t.Fatalf("%s: status %d; Node deu %d (corpo Go: %s)", nome, r.Status, c.Resposta.Status, emJSON(r.Corpo))
		}
		tipo := ""
		if c.Resposta.Tipo != nil {
			tipo = *c.Resposta.Tipo
		}
		if r.Tipo != tipo || !reflect.DeepEqual(r.Cabecalhos, c.Resposta.Cabecalhos) && !(len(r.Cabecalhos) == 0 && len(c.Resposta.Cabecalhos) == 0) {
			t.Fatalf("%s: tipo/cabeçalhos %q %v; Node deu %q %v", nome, r.Tipo, r.Cabecalhos, tipo, c.Resposta.Cabecalhos)
		}
		got := normal(r.Corpo)
		if c.Caminho == Prefixo+"/estado" && c.Metodo == "GET" {
			// Recursos da janela Go posteriores ao retrato fixo do Node.
			delete(got.(map[string]any), "onboardingConcluido")
			delete(got.(map[string]any), "perfilBuscaAtivo")
			delete(got.(map[string]any), "perfisBusca")
		}
		if !reflect.DeepEqual(got, c.Resposta.Corpo) {
			t.Fatalf("%s: corpo diferente do Node\nGo:   %s\nNode: %s", nome, cortar(emJSON(got)), cortar(emJSON(c.Resposta.Corpo)))
		}
	}
}

func normal(v any) any {
	var saida any
	json.Unmarshal([]byte(emJSON(v)), &saida)
	return saida
}

func emJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func cortar(s string) string {
	if len(s) > 1500 {
		return s[:1500] + "…"
	}
	return s
}

// Camada HTTP: as mesmas checagens do servidor Node.
func TestHTTP(t *testing.T) {
	hoje := "2026-11-08"
	h := novaAPI(t, &hoje, "abcd").Handler()
	pedir := func(metodo, caminho, corpo string, cab map[string]string) (int, map[string]any, http.Header) {
		req := httptest.NewRequest(metodo, caminho, strings.NewReader(corpo))
		for k, v := range cab {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var c map[string]any
		json.Unmarshal(rec.Body.Bytes(), &c)
		return rec.Code, c, rec.Header()
	}
	json_ := map[string]string{"Content-Type": "application/json"}

	if s, c, _ := pedir("GET", "/api/v1/saude", "", nil); s != 200 || c["ok"] != true {
		t.Errorf("saude = %d %v", s, c)
	}
	if s, c, _ := pedir("GET", "/api/versao", "", nil); s != 200 || c["versao"] != "0" {
		t.Errorf("rota antiga /api/versao = %d %v", s, c)
	}
	if s, _, _ := pedir("POST", "/api/v1/pedidos", `{"texto":"x"}`, map[string]string{"Content-Type": "text/plain"}); s != 415 {
		t.Errorf("sem JSON = %d; queria 415", s)
	}
	if s, _, _ := pedir("POST", "/api/v1/pedidos", `{"texto":"x"}`, map[string]string{"Content-Type": "application/json", "Sec-Fetch-Site": "cross-site"}); s != 403 {
		t.Errorf("de outro site = %d; queria 403", s)
	}
	if s, c, _ := pedir("POST", "/api/v1/pedidos", `{"texto":`, json_); s != 400 || c["erro"] != "JSON inválido" {
		t.Errorf("JSON quebrado = %d %v", s, c)
	}
	if s, _, _ := pedir("POST", "/api/v1/pedidos", `{"texto":"`+strings.Repeat("x", limiteCorpo)+`"}`, json_); s != 413 {
		t.Errorf("corpo enorme = %d; queria 413", s)
	}
	if s, c, _ := pedir("POST", "/api/v1/pedidos", `{"texto":"Buscar vagas"}`, map[string]string{"Content-Type": "application/json", "Sec-Fetch-Site": "same-origin"}); s != 201 || c["status"] != "aberto" {
		t.Errorf("criar pedido = %d %v", s, c)
	}
	if s, _, cab := pedir("GET", "/api/v1/exportar.csv", "", nil); s != 200 || !strings.Contains(cab.Get("Content-Disposition"), "candidaturas-2026-11-08.csv") || cab.Get("Cache-Control") != "no-store" {
		t.Errorf("csv = %d %v", s, cab)
	}
}
