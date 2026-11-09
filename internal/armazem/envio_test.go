package armazem

import (
	"context"
	"errors"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Falha fechada: banco novo, valor estragado ou ausente = pausado; só false grava liberado.
func TestEnvioPausado(t *testing.T) {
	ctx := context.Background()
	a, err := Abrir(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Fechar()
	if !a.EnvioPausado(ctx) {
		t.Fatal("banco novo deveria começar pausado")
	}
	if err := a.GravarEnvioPausado(ctx, false); err != nil || a.EnvioPausado(ctx) {
		t.Fatalf("retomar = %v, pausado %v", err, a.EnvioPausado(ctx))
	}
	for _, estragado := range []string{`"nao"`, `{`, `1`, ``} {
		a.db.ExecContext(ctx, `UPDATE meta SET valor = ? WHERE chave = ?`, estragado, MetaEnvioPausado)
		if !a.EnvioPausado(ctx) {
			t.Errorf("valor %q deveria contar como pausado", estragado)
		}
	}
	a.db.ExecContext(ctx, `DELETE FROM meta WHERE chave = ?`, MetaEnvioPausado)
	if !a.EnvioPausado(ctx) {
		t.Fatal("sem valor deveria contar como pausado")
	}
}

// Com o envio pausado, a gravação de um envio é recusada e nada muda; liberado, grava.
func TestGravarSeEnvioLiberado(t *testing.T) {
	ctx := context.Background()
	a, _ := Abrir(ctx, ":memory:")
	defer a.Fechar()
	v0, _ := a.Versao(ctx)
	marca := func(e dominio.Estado) (dominio.Estado, error) {
		e.Candidaturas = append(e.Candidaturas, dominio.Objeto{"id": "cand-1"})
		return e, nil
	}
	if err := a.GravarSeEnvioLiberado(ctx, marca); !errors.Is(err, ErrEnvioPausado) {
		t.Fatalf("pausado = %v", err)
	}
	if e, _ := a.Carregar(ctx); len(e.Candidaturas) != 0 {
		t.Fatal("gravou com o envio pausado")
	}
	if v, _ := a.Versao(ctx); v != v0 {
		t.Fatal("subiu a versão com o envio pausado")
	}
	a.GravarEnvioPausado(ctx, false)
	if err := a.GravarSeEnvioLiberado(ctx, marca); err != nil {
		t.Fatal(err)
	}
	if e, _ := a.Carregar(ctx); len(e.Candidaturas) != 1 {
		t.Fatal("não gravou com o envio liberado")
	}
}
