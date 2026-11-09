package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/ia"
)

// jc-a2y.35: no e2e, a tela sai do Conectar IA antes de o GET /ia terminar (ele roda as CLIs para detectar) e o
// navegador cancela o pedido. A leitura do banco devolve "context canceled" (*errors.errorString) e o Handler
// registrava "erro 500". Pedido cancelado pelo cliente não é erro do app.
func TestPedidoCanceladoPeloClienteNaoViraErro500(t *testing.T) {
	ctx, cancelar := context.WithCancel(context.Background())
	a := apiCom(t, nil)
	a.detectarCLIs = func(context.Context) []ia.EstadoCLI { cancelar(); return nil } // a pessoa sai da tela no meio
	lerLog := capturarLog(t)
	req := httptest.NewRequest("GET", "http://127.0.0.1"+Prefixo+"/ia", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, req)
	log := lerLog()
	if strings.Contains(log, "erro 500") {
		t.Fatalf("pedido cancelado virou erro 500 no log: %s", log)
	}
	if w.Code == 500 {
		t.Fatalf("pedido cancelado respondeu 500: %s", w.Body.String())
	}
}

// Gravação com o pedido já cancelado: nada é gravado pela metade e nada vira erro 500.
func TestGravacaoCanceladaNaoGravaNemViraErro500(t *testing.T) {
	a := apiCom(t, nil)
	chamarHTTP(t, a, "PUT", Prefixo+"/perfil", `{"nome":"Pessoa Exemplo"}`)
	antes, _ := a.arm.Carregar(context.Background())
	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()
	lerLog := capturarLog(t)
	req := httptest.NewRequest("PUT", "http://127.0.0.1"+Prefixo+"/perfil", strings.NewReader(`{"nome":"Outra Pessoa"}`)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, req)
	if strings.Contains(lerLog(), "erro 500") || w.Code == 500 {
		t.Fatalf("gravação cancelada virou erro 500: %d", w.Code)
	}
	depois, _ := a.arm.Carregar(context.Background())
	if depois.Perfil["nome"] != antes.Perfil["nome"] {
		t.Fatalf("gravação cancelada mudou o perfil: %v", depois.Perfil["nome"])
	}
}

// Erro de verdade continua sendo 500 com log: só o cancelamento pelo cliente sai do log.
func TestErroDeVerdadeContinuaSendo500(t *testing.T) {
	a := apiCom(t, nil)
	a.arm.Fechar()
	lerLog := capturarLog(t)
	if st, _ := chamarHTTP(t, a, "GET", Prefixo+"/estado", ""); st != 500 {
		t.Fatalf("banco fechado = %d", st)
	}
	if !strings.Contains(lerLog(), "api: erro 500 em GET "+Prefixo+"/estado") {
		t.Fatal("erro de verdade sumiu do log")
	}
}
