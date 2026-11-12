package api

import (
	"context"
	"net/http/httptest"
	"testing"
)

func TestMudancaDeRotaNaoSalvaPerfilComProvaAntiga(t *testing.T) {
	a := apiPrimeiroAcesso(t, t.TempDir())
	a.provaIA.confirmar(a.provaIA.invalidar())
	req := pedidoOnboarding(t, perfilInicialJSON, "cv.pdf", []byte("%PDF-1.4\n%%EOF"))
	req.Header.Set("X-Applyer-Verificacao", "3")
	a.provaIA.invalidar()
	resp := httptest.NewRecorder()
	a.Handler().ServeHTTP(resp, req)
	if resp.Code != 412 {
		t.Fatalf("status: %d", resp.Code)
	}
	if e := estadoInicial(t, a); e["onboardingConcluido"] != false {
		t.Fatal("confirmou com a rota antiga")
	}
	if err := a.verificarProvaDoPerfil(context.WithValue(context.Background(), chaveProvaDoPerfil{}, uint64(3))); err == nil {
		t.Fatal("a transação aceitou a prova antiga")
	}
}
