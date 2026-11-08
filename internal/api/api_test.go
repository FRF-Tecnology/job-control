package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fonteFixa int64

func (f fonteFixa) Versao(context.Context) (int64, error) { return int64(f), nil }

func chamar(t *testing.T, h http.Handler, metodo, caminho string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(metodo, caminho, nil))
	var corpo map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &corpo); err != nil {
		t.Fatalf("%s %s: resposta não é JSON: %q", metodo, caminho, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	return rec.Code, corpo
}

func TestRotas(t *testing.T) {
	h := Novo(fonteFixa(7))
	casos := []struct {
		nome, metodo, caminho string
		status                int
		campo                 string
		valor                 any
	}{
		{"saude", "GET", "/api/v1/saude", 200, "ok", true},
		{"versao vem como texto, igual à API Node", "GET", "/api/v1/versao", 200, "versao", "7"},
		{"rota desconhecida", "GET", "/api/v1/nada", 404, "erro", "rota não encontrada"},
		{"método errado", "POST", "/api/v1/saude", 405, "erro", "método não permitido nesta rota"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(c.metodo, c.caminho, nil))
			if rec.Code != c.status {
				t.Fatalf("status = %d; queria %d", rec.Code, c.status)
			}
			if c.campo == "" {
				return
			}
			_, corpo := chamar(t, h, c.metodo, c.caminho)
			if corpo[c.campo] != c.valor {
				t.Errorf("%s = %v; queria %v", c.campo, corpo[c.campo], c.valor)
			}
		})
	}
}
