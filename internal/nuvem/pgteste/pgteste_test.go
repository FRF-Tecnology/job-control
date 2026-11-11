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

func TestModoDe(t *testing.T) {
	casos := []struct {
		env  map[string]string
		want modo
	}{
		{map[string]string{"APPLYER_PG_TESTE": "postgres://x"}, modoExterno},
		{map[string]string{"APPLYER_PG_TESTE": "postgres://x", "CI": "true"}, modoExterno},
		{map[string]string{"CI": "true"}, modoPular},
		{map[string]string{}, modoEmbutido},
	}
	for _, c := range casos {
		if got := modoDe(func(k string) string { return c.env[k] }); got != c.want {
			t.Errorf("env %v: modo %d, esperava %d", c.env, got, c.want)
		}
	}
}
