package codigos_test

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/nuvem/codigos"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/pgteste"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMain(m *testing.M) { os.Exit(pgteste.Principal(m)) }

type relogio struct {
	mu sync.Mutex
	t  time.Time
}

func (r *relogio) agora() time.Time { r.mu.Lock(); defer r.mu.Unlock(); return r.t }
func (r *relogio) avancar(d time.Duration) {
	r.mu.Lock()
	r.t = r.t.Add(d)
	r.mu.Unlock()
}

func novo(t *testing.T) (*codigos.Repo, *relogio, *pgxpool.Pool) {
	db := pgteste.Banco(t)
	rel := &relogio{t: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)}
	return codigos.Novo(db, []byte("segredo-de-teste"), rel.agora, rand.Reader), rel, db
}

const ana = "ana@exemplo.com"

func TestGerarSeisDigitos(t *testing.T) {
	r, _, _ := novo(t)
	c, err := r.Gerar(context.Background(), ana, codigos.Confirmar)
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^\d{6}$`).MatchString(c) {
		t.Fatalf("código %q", c)
	}
}

func TestCodigoGuardadoSoComoHMAC(t *testing.T) {
	r, _, db := novo(t)
	c, err := r.Gerar(context.Background(), ana, codigos.Confirmar)
	if err != nil {
		t.Fatal(err)
	}
	var linha string
	if err := db.QueryRow(context.Background(), `select c::text from codigos c where email = $1`, ana).Scan(&linha); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(linha, c) {
		t.Fatalf("o código %s aparece na linha: %s", c, linha)
	}
}

func TestConferirCerto(t *testing.T) {
	r, _, _ := novo(t)
	ctx := context.Background()
	c, _ := r.Gerar(ctx, ana, codigos.Confirmar)
	if err := r.Conferir(ctx, ana, codigos.Confirmar, c); err != nil {
		t.Fatal(err)
	}
	if err := r.Conferir(ctx, ana, codigos.Confirmar, c); !errors.Is(err, codigos.ErrCodigoInvalido) {
		t.Errorf("reuso: %v", err)
	}
}

func TestConferirTipoErrado(t *testing.T) {
	r, _, _ := novo(t)
	ctx := context.Background()
	c, _ := r.Gerar(ctx, ana, codigos.Confirmar)
	if err := r.Conferir(ctx, ana, codigos.Recuperar, c); !errors.Is(err, codigos.ErrCodigoInvalido) {
		t.Errorf("outro tipo: %v", err)
	}
}

func TestConferirVencido(t *testing.T) {
	r, rel, _ := novo(t)
	ctx := context.Background()
	c, _ := r.Gerar(ctx, ana, codigos.Confirmar)
	rel.avancar(15*time.Minute + time.Second)
	if err := r.Conferir(ctx, ana, codigos.Confirmar, c); !errors.Is(err, codigos.ErrCodigoInvalido) {
		t.Fatalf("got %v", err)
	}
}

func errado(c string) string {
	if c == "000000" {
		return "000001"
	}
	return "000000"
}

func TestQuintaTentativaAindaValeSextaNao(t *testing.T) {
	r, _, _ := novo(t)
	ctx := context.Background()
	c, _ := r.Gerar(ctx, ana, codigos.Confirmar)
	for i := 0; i < 4; i++ {
		if err := r.Conferir(ctx, ana, codigos.Confirmar, errado(c)); !errors.Is(err, codigos.ErrCodigoInvalido) {
			t.Fatalf("tentativa %d: %v", i+1, err)
		}
	}
	if err := r.Conferir(ctx, ana, codigos.Confirmar, c); err != nil {
		t.Fatalf("a 5ª certa deveria valer: %v", err)
	}

	bia := "bia@exemplo.com"
	c, _ = r.Gerar(ctx, bia, codigos.Confirmar)
	for i := 0; i < 5; i++ {
		_ = r.Conferir(ctx, bia, codigos.Confirmar, errado(c))
	}
	if err := r.Conferir(ctx, bia, codigos.Confirmar, c); !errors.Is(err, codigos.ErrTentativas) {
		t.Fatalf("a 6ª deveria dar ErrTentativas: %v", err)
	}
}

func TestEspereSessentaSegundos(t *testing.T) {
	r, rel, _ := novo(t)
	ctx := context.Background()
	if _, err := r.Gerar(ctx, ana, codigos.Confirmar); err != nil {
		t.Fatal(err)
	}
	rel.avancar(59 * time.Second)
	if _, err := r.Gerar(ctx, ana, codigos.Confirmar); !errors.Is(err, codigos.ErrEspere) {
		t.Fatalf("got %v", err)
	}
	if _, err := r.Gerar(ctx, ana, codigos.Recuperar); err != nil {
		t.Fatalf("outro tipo não espera: %v", err)
	}
	rel.avancar(time.Second)
	if _, err := r.Gerar(ctx, ana, codigos.Confirmar); err != nil {
		t.Fatalf("após 60 s: %v", err)
	}
}

func TestCodigoNovoInvalidaAnterior(t *testing.T) {
	r, rel, _ := novo(t)
	ctx := context.Background()
	velho, _ := r.Gerar(ctx, ana, codigos.Confirmar)
	rel.avancar(61 * time.Second)
	atual, _ := r.Gerar(ctx, ana, codigos.Confirmar)
	if velho != atual {
		if err := r.Conferir(ctx, ana, codigos.Confirmar, velho); !errors.Is(err, codigos.ErrCodigoInvalido) {
			t.Errorf("antigo: %v", err)
		}
	}
	if err := r.Conferir(ctx, ana, codigos.Confirmar, atual); err != nil {
		t.Errorf("novo: %v", err)
	}
}

func TestGerarConcorrenteSoUmPassa(t *testing.T) {
	r, _, _ := novo(t)
	ctx := context.Background()
	var wg sync.WaitGroup
	res := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := r.Gerar(ctx, ana, codigos.Confirmar); res <- err }()
	}
	wg.Wait()
	close(res)
	ok := 0
	for err := range res {
		if err == nil {
			ok++
		} else if !errors.Is(err, codigos.ErrEspere) {
			t.Error(err)
		}
	}
	if ok != 1 {
		t.Fatalf("%d gerações passaram, queria 1", ok)
	}
}
