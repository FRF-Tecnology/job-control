package banco_test

import (
	"context"
	"os"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/nuvem/banco"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/pgteste"
)

func TestMain(m *testing.M) { os.Exit(pgteste.Principal(m)) }

func TestMigracoesSobem(t *testing.T) {
	pool := pgteste.Banco(t)
	rows, err := pool.Query(context.Background(),
		`select table_name from information_schema.tables where table_schema = 'applyer'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	achou := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		achou[n] = true
	}
	for _, n := range []string{"usuarios", "codigos", "sessoes", "computadores_teste", "assinaturas", "eventos_stripe", "notas"} {
		if !achou[n] {
			t.Errorf("falta a tabela %s", n)
		}
	}
}

func TestAbrirDeNovoNaoQuebra(t *testing.T) {
	pool := pgteste.Banco(t)
	dsn := pool.Config().ConnString()
	p2, err := banco.Abrir(context.Background(), dsn)
	if err != nil {
		t.Fatalf("segunda abertura: %v", err)
	}
	p2.Close()
}

func TestRestricoes(t *testing.T) {
	pool := pgteste.Banco(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `insert into usuarios (email) values ('ana@exemplo.com')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `insert into usuarios (email) values ('ana@exemplo.com')`); err == nil {
		t.Error("email duplicado deveria falhar")
	}
	var cortesia bool
	if err := pool.QueryRow(ctx, `select cortesia from usuarios`).Scan(&cortesia); err != nil || cortesia {
		t.Errorf("cortesia padrão deveria ser false (%v)", err)
	}
}
