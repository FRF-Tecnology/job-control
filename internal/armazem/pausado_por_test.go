package armazem

import (
	"context"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// jc-a2y.7.4: quem pausou o envio fica à parte da trava (que continua só um booleano com falha fechada).
func TestEnvioPausadoPor(t *testing.T) {
	ctx := context.Background()
	a, err := Abrir(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Fechar()
	if err := a.GravarEnvioPausadoPor(ctx, &dominio.Quem{Autor: "claude", Canal: "mcp"}); err != nil {
		t.Fatal(err)
	}
	if q := a.EnvioPausadoPor(ctx); !a.EnvioPausado(ctx) || q == nil || q.Canal != "mcp" {
		t.Fatalf("pausado por = %+v", q)
	}
	// Combinação inválida: pausa, sem quem.
	if err := a.GravarEnvioPausadoPor(ctx, &dominio.Quem{Autor: "eu", Canal: "mcp"}); err != nil {
		t.Fatal(err)
	}
	if q := a.EnvioPausadoPor(ctx); !a.EnvioPausado(ctx) || q != nil {
		t.Fatalf("inválido gravado: %+v", q)
	}
	// Valor estragado no banco: sem quem.
	if _, err := a.db.ExecContext(ctx, `INSERT INTO meta (chave, valor) VALUES (?, ?) ON CONFLICT(chave) DO UPDATE SET valor = excluded.valor`, MetaEnvioPausadoPor, `{"autor":"eu","canal":"cli"}`); err != nil {
		t.Fatal(err)
	}
	if q := a.EnvioPausadoPor(ctx); q != nil {
		t.Fatalf("estragado = %+v", q)
	}
	// Retomar (ou abrir o app) apaga.
	a.GravarEnvioPausadoPor(ctx, &dominio.Quem{Autor: "eu", Canal: "tela"})
	if err := a.GravarEnvioPausado(ctx, false); err != nil {
		t.Fatal(err)
	}
	if q := a.EnvioPausadoPor(ctx); a.EnvioPausado(ctx) || q != nil {
		t.Fatalf("depois de retomar = %+v", q)
	}
}
