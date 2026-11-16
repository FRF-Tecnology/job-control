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
	"github.com/FRF-Tecnology/job-control/internal/golden"
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

const caminhoGoldenAPI = "testdata/golden-api.json"

// A sessão de testdata/golden-api.json nasceu da API Node e agora é do Go:
// partindo de um banco vazio, a API responde igual em cada chamada. Mudou uma resposta de propósito?
// go test ./internal/api -run Golden -atualizar, e confira o diff.
func TestGoldenDaApi(t *testing.T) {
	var doc *golden.Obj
	if *golden.Atualizar {
		doc = golden.Ler(t, caminhoGoldenAPI)
	}
	b, err := os.ReadFile(caminhoGoldenAPI)
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
		tipo := ""
		if c.Resposta.Tipo != nil {
			tipo = *c.Resposta.Tipo
		}
		got := normal(r.Corpo)
		dif := ""
		switch {
		case r.Status != c.Resposta.Status:
			dif = fmt.Sprintf("%s: status %d; o golden diz %d (corpo Go: %s)", nome, r.Status, c.Resposta.Status, emJSON(r.Corpo))
		case r.Tipo != tipo || !reflect.DeepEqual(r.Cabecalhos, c.Resposta.Cabecalhos) && !(len(r.Cabecalhos) == 0 && len(c.Resposta.Cabecalhos) == 0):
			dif = fmt.Sprintf("%s: tipo/cabeçalhos %q %v; o golden diz %q %v", nome, r.Tipo, r.Cabecalhos, tipo, c.Resposta.Cabecalhos)
		case !reflect.DeepEqual(got, c.Resposta.Corpo):
			dif = fmt.Sprintf("%s: corpo diferente do golden\nGo:     %s\ngolden: %s", nome, cortar(emJSON(got)), cortar(emJSON(c.Resposta.Corpo)))
		}
		if dif == "" {
			continue
		}
		if doc == nil {
			t.Fatal(dif + "\n(se a mudança é de propósito: go test ./internal/api -run Golden -atualizar)")
		}
		resp := doc.Item("chamadas", i).Get("resposta").(*golden.Obj)
		resp.Set("status", json.Number(fmt.Sprint(r.Status)))
		resp.Trocar("corpo", got)
		if r.Tipo == "" {
			resp.Set("tipo", nil)
		} else {
			resp.Set("tipo", r.Tipo)
		}
		resp.Trocar("cabecalhos", r.Cabecalhos)
	}
	if doc != nil {
		golden.Gravar(t, caminhoGoldenAPI, doc)
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
