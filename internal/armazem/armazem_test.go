package armazem

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestAbrirCriaBancoComVersaoZero(t *testing.T) {
	ctx := context.Background()
	caminho := filepath.Join(t.TempDir(), "central.db")
	a, err := Abrir(ctx, caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Fechar()
	v, err := a.Versao(ctx)
	if err != nil || v != 0 {
		t.Fatalf("versao = %d, %v; queria 0", v, err)
	}
}

func TestReabrirNaoRepeteMigracaoEMantemDados(t *testing.T) {
	ctx := context.Background()
	caminho := filepath.Join(t.TempDir(), "central.db")
	a, err := Abrir(ctx, caminho)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Transacao(ctx, func(tx *sql.Tx) error { _, err := SubirVersao(ctx, tx); return err }); err != nil {
		t.Fatal(err)
	}
	a.Fechar()

	b, err := Abrir(ctx, caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Fechar()
	if v, _ := b.Versao(ctx); v != 1 {
		t.Fatalf("versao depois de reabrir = %d; queria 1", v)
	}
}

func TestTransacaoComErroDesfaz(t *testing.T) {
	ctx := context.Background()
	a, err := Abrir(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Fechar()
	falha := errors.New("falhou no meio")
	err = a.Transacao(ctx, func(tx *sql.Tx) error {
		if _, err := SubirVersao(ctx, tx); err != nil {
			return err
		}
		return falha
	})
	if !errors.Is(err, falha) {
		t.Fatalf("erro = %v; queria o erro da função", err)
	}
	if v, _ := a.Versao(ctx); v != 0 {
		t.Fatalf("versao = %d; a transação deveria ter sido desfeita", v)
	}
}
