package pgteste

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) { os.Exit(Principal(m)) }

func TestBancosIsolados(t *testing.T) {
	a := Banco(t)
	b := Banco(t)
	if _, err := a.Exec(t.Context(), `insert into usuarios (email) values ('a@exemplo.com')`); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := b.QueryRow(t.Context(), `select count(*) from usuarios`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("bancos não isolados: n=%d err=%v", n, err)
	}
}
