package api

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

func TestSubstituirCVSoNoPerfilAberto(t *testing.T) {
	pasta := t.TempDir()
	a := apiPrimeiroAcesso(t, pasta)
	old := []byte("%PDF-1.5\n% inicial\n%%EOF")
	next := []byte("%PDF-1.5\n% revisado\n%%EOF")
	req := pedidoOnboarding(t, perfilInicialJSON, "original.pdf", old)
	inicial := httptest.NewRecorder()
	a.Handler().ServeHTTP(inicial, req)
	if inicial.Code != 200 {
		t.Fatal(inicial.Code)
	}
	enviar := func(id string, arquivo []byte) int {
		req := pedidoOnboarding(t, "{}", "novo.pdf", arquivo)
		req.URL.Path = "/api/v1/perfil/curriculo"
		req.URL.RawQuery = "perfilBuscaId=" + id
		r := httptest.NewRecorder()
		a.Handler().ServeHTTP(r, req)
		return r.Code
	}
	nome := func() string {
		p := dominio.ObjetoDe(estadoInicial(t, a)["perfil"])
		return dominio.Texto(dominio.ObjetoDe(p["curriculos"])["PT"])
	}
	anterior := nome()
	if code := enviar("inativo", next); code != 409 {
		t.Fatalf("perfil trocado: %d", code)
	}
	if code := enviar("original", []byte("invalido")); code != 400 {
		t.Fatalf("PDF inválido: %d", code)
	}
	if nome() != anterior {
		t.Fatal("alterou referência em falha")
	}
	if code := enviar("original", next); code != 200 {
		t.Fatalf("PDF novo: %d", code)
	}
	p := dominio.ObjetoDe(estadoInicial(t, a)["perfil"])
	rotulo := dominio.ObjetoDe(p["curriculoExibicao"])
	if rotulo["nome"] != "novo.pdf" || rotulo["arquivo"] != nome() {
		t.Fatalf("nome de exibição do PDF não foi preservado: %v", rotulo)
	}
	for n, esperado := range map[string][]byte{anterior: old, nome(): next} {
		b, err := os.ReadFile(filepath.Join(pasta, filepath.FromSlash(n)))
		if err != nil || string(b) != string(esperado) {
			t.Fatal("não preservou os dois arquivos")
		}
	}
}
