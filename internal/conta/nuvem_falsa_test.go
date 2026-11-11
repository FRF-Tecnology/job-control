//go:build contafalsa

package conta

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/licenca"
)

const (
	dispA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	dispB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func novaFalsaDeTeste(t *testing.T) (*Falsa, ed25519.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return NovaFalsa(priv, func() time.Time { return time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC) }), pub
}

func estadoDe(t *testing.T, f *Falsa, pub ed25519.PublicKey, acesso, disp string) licenca.Licenca {
	t.Helper()
	doc, err := f.Licenca(context.Background(), acesso, disp)
	if err != nil {
		t.Fatal(err)
	}
	l, err := licenca.Conferir(pub, doc, f.agoraL())
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestFalsaFluxoCompleto(t *testing.T) {
	f, pub := novaFalsaDeTeste(t)
	ctx := context.Background()
	if err := f.Criar(ctx, "ana@exemplo.com", "senha-forte-1", "2026-11"); err != nil {
		t.Fatal(err)
	}
	// não confirmada: 403 confirmar
	_, err := f.Entrar(ctx, "ana@exemplo.com", "senha-forte-1", dispA, "")
	var e *ErroNuvem
	if !errors.As(err, &e) || e.Status != 403 || e.Motivo != "confirmar" {
		t.Fatalf("queria 403 confirmar, veio %v", err)
	}
	// senha errada na confirmação
	if _, err := f.Confirmar(ctx, "ana@exemplo.com", "123456", "outra-senha-1", dispA, ""); err == nil {
		t.Fatal("confirmar com outra senha devia falhar")
	}
	if _, err := f.Confirmar(ctx, "ana@exemplo.com", "000000", "senha-forte-1", dispA, ""); err == nil {
		t.Fatal("código errado devia falhar")
	}
	p, err := f.Confirmar(ctx, "ana@exemplo.com", "123456", "senha-forte-1", dispA, "")
	if err != nil {
		t.Fatal(err)
	}
	l := estadoDe(t, f, pub, p.Acesso, dispA)
	if l.Estado != licenca.Teste || l.TesteFim == nil || l.Email != "ana@exemplo.com" {
		t.Fatalf("queria teste, veio %+v", l)
	}
	if !l.ValidaAte.Equal(l.EmitidaEm.Add(30 * time.Minute)) {
		t.Fatalf("valida_ate errado: %+v", l)
	}
	f.Avancar(24*time.Hour + time.Second)
	if l := estadoDe(t, f, pub, p.Acesso, dispA); l.Estado != licenca.Bloqueado {
		t.Fatalf("queria bloqueado, veio %+v", l)
	}
	f.Assinante("ana@exemplo.com")
	if l := estadoDe(t, f, pub, p.Acesso, dispA); l.Estado != licenca.Assinante {
		t.Fatalf("queria assinante, veio %+v", l)
	}
	if u, err := f.Assinar(ctx, p.Acesso); err != nil || u == "" {
		t.Fatalf("url: %v %q", err, u)
	}
}

func TestFalsaSenhaErradaEUmTestePorDispositivo(t *testing.T) {
	f, pub := novaFalsaDeTeste(t)
	ctx := context.Background()
	for _, em := range []string{"ana@exemplo.com", "bia@exemplo.com"} {
		_ = f.Criar(ctx, em, "senha-forte-1", "2026-11")
		_, _ = f.Confirmar(ctx, em, "123456", "senha-forte-1", dispA, "")
	}
	_, err := f.Entrar(ctx, "ana@exemplo.com", "errada-errada", dispA, "")
	var e *ErroNuvem
	if !errors.As(err, &e) || e.Status != 401 || e.Msg != "Email ou senha incorretos" {
		t.Fatalf("erro errado: %v", err)
	}
	pa, _ := f.Entrar(ctx, "ana@exemplo.com", "senha-forte-1", dispA, "")
	pb, _ := f.Entrar(ctx, "bia@exemplo.com", "senha-forte-1", dispA, "")
	if l := estadoDe(t, f, pub, pa.Acesso, dispA); l.Estado != licenca.Teste {
		t.Fatalf("ana: %+v", l)
	}
	if l := estadoDe(t, f, pub, pb.Acesso, dispA); l.Estado != licenca.Bloqueado {
		t.Fatalf("bia no mesmo computador devia ficar bloqueada: %+v", l)
	}
	f.Cortesia("bia@exemplo.com")
	if l := estadoDe(t, f, pub, pb.Acesso, dispA); l.Estado != licenca.Assinante {
		t.Fatalf("cortesia: %+v", l)
	}
}

func TestFalsaSemInternetERenovar(t *testing.T) {
	f, _ := novaFalsaDeTeste(t)
	ctx := context.Background()
	_ = f.Criar(ctx, "ana@exemplo.com", "senha-forte-1", "2026-11")
	p, _ := f.Confirmar(ctx, "ana@exemplo.com", "123456", "senha-forte-1", dispB, "")
	p2, err := f.Renovar(ctx, p.Renovacao)
	if err != nil || p2.Acesso == p.Acesso {
		t.Fatalf("renovar: %v", err)
	}
	if _, err := f.Renovar(ctx, p.Renovacao); err == nil {
		t.Fatal("renovação é de uso único")
	}
	f.SemInternet(true)
	if _, err := f.Licenca(ctx, p2.Acesso, dispB); !errors.Is(err, ErrSemInternet) {
		t.Fatalf("queria ErrSemInternet, veio %v", err)
	}
	if err := f.Sair(ctx, p2.Renovacao); !errors.Is(err, ErrSemInternet) {
		t.Fatalf("queria ErrSemInternet, veio %v", err)
	}
	f.SemInternet(false)
	if err := f.Excluir(ctx, p2.Acesso, "outra@exemplo.com"); err == nil {
		t.Fatal("excluir exige o email da conta")
	}
	if err := f.Excluir(ctx, p2.Acesso, "ana@exemplo.com"); err != nil {
		t.Fatal(err)
	}
}
