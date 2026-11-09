package armazem

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// jc-a2y.6.11: estado por plataforma gravado na meta, com falha fechada.
func TestPlataformasGravadas(t *testing.T) {
	ctx := context.Background()
	a, err := Abrir(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Fechar()
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600))
	if !a.Plataformas(ctx).Ativa("linkedin", agora) {
		t.Fatal("banco novo: plataforma deveria estar ativa (o envio geral já começa pausado)")
	}
	if _, err := a.MudarPlataformas(ctx, func(p dominio.Plataformas, _ bool) (dominio.Plataformas, error) {
		return p.PausarPlataforma("linkedin", agora)
	}); err != nil {
		t.Fatal(err)
	}
	if a.Plataformas(ctx).Ativa("linkedin", agora) || !a.Plataformas(ctx).Ativa("gupy", agora) {
		t.Fatal("a pausa não ficou gravada só no linkedin")
	}
	for _, estragado := range []string{`{`, `"x"`, `[]`, `null`, `{"gupy":{"estado":"liberada"}}`} {
		a.db.ExecContext(ctx, `UPDATE meta SET valor = ? WHERE chave = ?`, estragado, MetaEnvioPlataformas)
		p := a.Plataformas(ctx)
		if p.Ativa("gupy", agora) {
			t.Errorf("valor %q deveria deixar a gupy pausada", estragado)
		}
	}
}

// Com a plataforma fora de ativa, a gravação que começaria um envio é recusada e nada muda.
func TestGravarSeLiberadoConfereAPlataforma(t *testing.T) {
	ctx := context.Background()
	a, err := Abrir(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Fechar()
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600))
	a.GravarEnvioPausado(ctx, false)
	a.MudarPlataformas(ctx, func(p dominio.Plataformas, _ bool) (dominio.Plataformas, error) {
		return p.RegistrarResultado("linkedin", "captcha", agora), nil
	})
	chamou := false
	err = a.GravarSeLiberado(ctx, "linkedin", agora, func(e dominio.Estado) (dominio.Estado, error) { chamou = true; return e, nil })
	if !errors.Is(err, ErrPlataformaParada) || chamou {
		t.Fatalf("linkedin parada: %v, chamou %v", err, chamou)
	}
	if err := a.GravarSeLiberado(ctx, "gupy", agora, func(e dominio.Estado) (dominio.Estado, error) { return e, nil }); err != nil {
		t.Fatalf("gupy ativa: %v", err)
	}
	// Dia seguinte: a parada vence.
	amanha := time.Date(2026, 11, 10, 0, 0, 0, 0, time.FixedZone("BRT", -3*3600))
	if err := a.GravarSeLiberado(ctx, "linkedin", amanha, func(e dominio.Estado) (dominio.Estado, error) { return e, nil }); err != nil {
		t.Fatalf("linkedin no dia seguinte: %v", err)
	}
	a.GravarEnvioPausado(ctx, true)
	if err := a.GravarSeLiberado(ctx, "gupy", agora, func(e dominio.Estado) (dominio.Estado, error) { return e, nil }); !errors.Is(err, ErrEnvioPausado) {
		t.Fatalf("envio geral pausado: %v", err)
	}
}
