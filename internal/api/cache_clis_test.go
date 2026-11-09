package api

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/ia"
)

// jc-a2y.44: o GET /ia roda as CLIs para detectar versão e login e fica lento. Dentro da janela curta, a detecção
// vem do cache; mudar a configuração, testar a conexão ou passar a janela faz detectar de novo.
func TestDeteccaoDeCLIsComCacheCurto(t *testing.T) {
	var vezes atomic.Int32
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	a := apiCom(t, nil)
	a.detectarCLIs = func(context.Context) []ia.EstadoCLI {
		vezes.Add(1)
		return []ia.EstadoCLI{{InfoCLI: ia.CLIs[0], Instalada: true, Pronta: true}}
	}
	a.relogio = func() time.Time { return agora }
	get := func() {
		t.Helper()
		if st, c := chamarHTTP(t, a, "GET", Prefixo+"/ia", ""); st != 200 {
			t.Fatalf("GET /ia = %d %s", st, c)
		}
	}
	quer := func(n int32, passo string) {
		t.Helper()
		if v := vezes.Load(); v != n {
			t.Fatalf("%s: detectou %d vezes; queria %d", passo, v, n)
		}
	}
	get()
	get()
	get()
	quer(1, "três GET na janela")
	agora = agora.Add(janelaDeteccaoCLIs + time.Second)
	get()
	quer(2, "depois da janela")
	chamarHTTP(t, a, "PUT", Prefixo+"/ia/config", `{"provedor":"anthropic"}`) // sem chave: o testar falha na hora, sem rede nem CLI de verdade
	get()
	quer(3, "depois de mudar a configuração")
	chamarHTTP(t, a, "POST", Prefixo+"/ia/testar", "{}")
	get()
	quer(4, "depois de testar a conexão")
}

// Detecção cortada por pedido cancelado não fica no cache (seria um resultado pela metade por 45 s).
func TestDeteccaoCanceladaNaoEntraNoCache(t *testing.T) {
	var vezes atomic.Int32
	a := apiCom(t, nil)
	a.detectarCLIs = func(ctx context.Context) []ia.EstadoCLI { vezes.Add(1); return nil }
	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()
	a.clisDetectadas(ctx)
	a.clisDetectadas(context.Background())
	if vezes.Load() != 2 {
		t.Fatalf("detecção cancelada entrou no cache: %d vezes", vezes.Load())
	}
}
