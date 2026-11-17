package licenca

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"reflect"
	"testing"
	"time"
)

func chaves(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return pub, priv
}

func exemplo() Licenca {
	t0 := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	teste := t0.Add(14 * 24 * time.Hour)
	return Licenca{
		Usuario:   "u-1",
		Email:     "ana@exemplo.com",
		Estado:    Teste,
		TesteFim:  &teste,
		EmitidaEm: t0,
		ValidaAte: t0.Add(72 * time.Hour),
	}
}

func TestAssinarEConferir(t *testing.T) {
	pub, priv := chaves(t)
	l := exemplo()
	d, err := Assinar(priv, l)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Conferir(pub, d, l.EmitidaEm.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, l) {
		t.Fatalf("got %+v, want %+v", got, l)
	}
}

func TestConferirRecusaDadosAlterados(t *testing.T) {
	pub, priv := chaves(t)
	l := exemplo()
	d, _ := Assinar(priv, l)
	b := []byte(d.Dados)
	if b[10] == 'A' {
		b[10] = 'B'
	} else {
		b[10] = 'A'
	}
	d.Dados = string(b)
	if _, err := Conferir(pub, d, l.EmitidaEm); !errors.Is(err, ErrAssinatura) {
		t.Fatalf("err = %v, want ErrAssinatura", err)
	}
}

func TestConferirRecusaLixo(t *testing.T) {
	pub, _ := chaves(t)
	for _, d := range []Documento{{}, {Dados: "@@@", Assinatura: "@@@"}} {
		if _, err := Conferir(pub, d, time.Now()); !errors.Is(err, ErrAssinatura) {
			t.Fatalf("err = %v, want ErrAssinatura", err)
		}
	}
}

func TestConferirRecusaOutraChave(t *testing.T) {
	_, priv := chaves(t)
	outra, _ := chaves(t)
	l := exemplo()
	d, _ := Assinar(priv, l)
	if _, err := Conferir(outra, d, l.EmitidaEm); !errors.Is(err, ErrAssinatura) {
		t.Fatalf("err = %v, want ErrAssinatura", err)
	}
}

func TestConferirVencida(t *testing.T) {
	pub, priv := chaves(t)
	l := exemplo()
	d, _ := Assinar(priv, l)
	if _, err := Conferir(pub, d, l.ValidaAte); err != nil {
		t.Fatalf("no limite deveria valer: %v", err)
	}
	if _, err := Conferir(pub, d, l.ValidaAte.Add(time.Second)); !errors.Is(err, ErrVencida) {
		t.Fatalf("err = %v, want ErrVencida", err)
	}
}

func TestPodeAgir(t *testing.T) {
	casos := map[Estado]bool{Teste: true, Assinante: true, Bloqueado: false}
	for e, want := range casos {
		if got := (Licenca{Estado: e}).PodeAgir(); got != want {
			t.Errorf("%s: got %v, want %v", e, got, want)
		}
	}
}
