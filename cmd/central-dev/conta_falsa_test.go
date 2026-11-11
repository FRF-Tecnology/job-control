//go:build contafalsa

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/conta"
)

// Com -tags contafalsa, o central-dev liga a conta na nuvem falsa e tem a rota de teste que adianta o relógio
// (para o Playwright da Task 7). O resto do /api/ vai para a API.
func TestContaFalsaNoDev(t *testing.T) {
	c, envolver := contaDoDev(t.TempDir())
	if c == nil || envolver == nil {
		t.Fatal("sem conta no build contafalsa")
	}
	if pode, motivo := c.PodeAgir(); pode || motivo != conta.MotivoSemConta {
		t.Fatalf("antes de entrar = %v %q", pode, motivo)
	}
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(299) })
	h := envolver(api)
	pedir := func(m, caminho, corpo string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		r := httptest.NewRequest(m, caminho, strings.NewReader(corpo))
		r.Header.Set("Content-Type", "application/json")
		h.ServeHTTP(rec, r)
		return rec
	}
	if rec := pedir("GET", "/api/v1/saude", ""); rec.Code != 299 {
		t.Fatalf("/api/v1/saude não foi para a API: %d", rec.Code)
	}
	for _, corpo := range []string{`{}`, `{"horas":0}`, `{"horas":-1}`, `{"horas":100000}`, `nada`} {
		if rec := pedir("POST", "/api/v1/_teste/avancar", corpo); rec.Code != 400 {
			t.Errorf("avancar %s = %d", corpo, rec.Code)
		}
	}
	if rec := pedir("GET", "/api/v1/_teste/avancar", ""); rec.Code != 405 {
		t.Errorf("GET avancar = %d", rec.Code)
	}

	// Entra, usa o teste de 24 h e adianta 25 h: a trava liga com teste-acabou.
	if err := c.Criar(t.Context(), "ana@exemplo.com", "senha-forte-1", conta.TermosVersao); err != nil {
		t.Fatal(err)
	}
	if err := c.Confirmar(t.Context(), "ana@exemplo.com", conta.CodigoFalso, "senha-forte-1", conta.TermosVersao); err != nil {
		t.Fatal(err)
	}
	if pode, motivo := c.PodeAgir(); !pode {
		t.Fatalf("no teste = %q", motivo)
	}
	rec := pedir("POST", "/api/v1/_teste/avancar", `{"horas":25}`)
	var s conta.Situacao
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &s) != nil || s.Motivo != conta.MotivoTesteAcabou {
		t.Fatalf("avancar 25 h = %d %s", rec.Code, rec.Body.String())
	}
	if pode, motivo := c.PodeAgir(); pode || motivo != conta.MotivoTesteAcabou {
		t.Fatalf("depois de 25 h = %v %q", pode, motivo)
	}
}
