package nuvem

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// A limpeza (pelo relógio injetado) apaga conta nunca confirmada com mais de 7 dias e os códigos dela,
// e códigos vencidos há mais de 1 dia; o resto fica.
func TestLimpezaApagaPendentesECodigosVelhos(t *testing.T) {
	a := novoAmb(t)
	ctx := context.Background()
	a.criarConfirmado(t, "ana@exemplo.com", senhaAna) // confirmada: fica, mesmo velha
	confere(t, a.post(t, "/v1/contas", map[string]string{"email": "bia@exemplo.com", "senha": senhaAna, "termos_versao": termosAtual}),
		http.StatusAccepted, "")
	a.emails("bia@exemplo.com")
	// criado_em vem do now() do banco: as duas contas passam a ter sido criadas no instante do relógio do teste.
	if _, err := a.db.Exec(ctx, `update usuarios set criado_em = $1`, a.rel.agora()); err != nil {
		t.Fatal(err)
	}

	// 6 dias depois: a pendente ainda fica; os códigos (vencidos há mais de 1 dia) somem.
	a.rel.avancar(6 * 24 * time.Hour)
	confere(t, a.post(t, "/v1/contas", map[string]string{"email": "cris@exemplo.com", "senha": senhaAna, "termos_versao": termosAtual}),
		http.StatusAccepted, "")
	a.emails("cris@exemplo.com")
	if err := a.s.Limpar(ctx); err != nil {
		t.Fatal(err)
	}
	if n := a.contarOnde(t, "usuarios", "email = 'bia@exemplo.com'"); n != 1 {
		t.Fatalf("pendente de 6 dias apagada")
	}
	if n := a.contarOnde(t, "codigos", "email in ('ana@exemplo.com', 'bia@exemplo.com')"); n != 0 {
		t.Fatalf("códigos vencidos há mais de 1 dia ficaram: %d", n)
	}
	if n := a.contarOnde(t, "codigos", "email = 'cris@exemplo.com'"); n != 1 {
		t.Fatalf("código em vigor apagado: %d", n)
	}
	if n := a.contarOnde(t, "eventos_codigo", "true"); n != 1 {
		t.Fatalf("eventos de código: %d, queria só o de cris (os de mais de 1 dia somem)", n)
	}

	// 7 dias e 1 hora depois da criação: a pendente some; a confirmada fica.
	a.rel.avancar(24*time.Hour + time.Hour)
	if err := a.s.Limpar(ctx); err != nil {
		t.Fatal(err)
	}
	if n := a.contarOnde(t, "usuarios", "email = 'bia@exemplo.com'"); n != 0 {
		t.Fatalf("pendente de mais de 7 dias ficou")
	}
	if n := a.contarOnde(t, "usuarios", "email in ('ana@exemplo.com', 'cris@exemplo.com')"); n != 2 {
		t.Fatalf("apagou conta que devia ficar: %d", n)
	}
	if n := a.contarOnde(t, "eventos_codigo", "true"); n != 0 {
		t.Fatalf("eventos de código de mais de 1 dia ficaram: %d", n)
	}
}

// RodarLimpeza para quando o ctx acaba.
func TestRodarLimpezaParaNoDesligamento(t *testing.T) {
	a := novoAmb(t)
	ctx, cancelar := context.WithCancel(context.Background())
	fim := make(chan struct{})
	go func() { a.s.RodarLimpeza(ctx); close(fim) }()
	cancelar()
	select {
	case <-fim:
	case <-time.After(5 * time.Second):
		t.Fatal("RodarLimpeza não parou")
	}
}
