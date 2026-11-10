package armazem

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// jc-a2y.7.4 (achado do CyberSec no #139): o banco do assistente nunca é criado nem migrado.
func TestAbrirExistente(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	caminho := filepath.Join(dir, "central.db")

	// Sem banco: erro, e nenhum arquivo aparece.
	if _, err := AbrirExistente(ctx, caminho); !errors.Is(err, ErrBancoAusente) {
		t.Fatalf("sem banco = %v", err)
	}
	if _, err := os.Stat(caminho); err == nil {
		t.Fatal("criou o banco")
	}
	// Mesmo sem a conferência do arquivo (o banco sumiu entre ela e a abertura): o mode=rw não cria.
	if _, err := abrirSemCriar(ctx, caminho); err == nil {
		t.Fatal("abriu sem banco")
	}
	if _, err := os.Stat(caminho); err == nil {
		t.Fatal("o mode=rw criou o banco")
	}

	// O banco do app (criado e migrado pelo Abrir): abre, lê e grava.
	a, err := Abrir(ctx, caminho)
	if err != nil {
		t.Fatal(err)
	}
	a.Fechar()
	b, err := AbrirExistente(ctx, caminho)
	if err != nil {
		t.Fatalf("banco do app = %v", err)
	}
	if err := b.Gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) { return e, nil }); err != nil {
		t.Fatalf("gravar = %v", err)
	}
	b.Fechar()

	quer, _ := versaoEmbutida()
	versao := func() int64 {
		db, _ := sql.Open("sqlite", "file:"+caminho+"?mode=ro")
		defer db.Close()
		var v int64
		db.QueryRow(`SELECT MAX(version_id) FROM goose_db_version`).Scan(&v)
		return v
	}
	mexer := func(sqlTexto string) {
		db, _ := sql.Open("sqlite", "file:"+caminho)
		defer db.Close()
		if _, err := db.Exec(sqlTexto); err != nil {
			t.Fatal(err)
		}
	}
	// Esquema mais antigo (um app antigo criou): erro, e nada é migrado.
	mexer(`DELETE FROM goose_db_version WHERE version_id = (SELECT MAX(version_id) FROM goose_db_version)`)
	if v := versao(); v >= quer {
		t.Fatalf("o teste não envelheceu o esquema: %d", v)
	}
	antes := versao()
	if _, err := AbrirExistente(ctx, caminho); !errors.Is(err, ErrEsquemaDiferente) {
		t.Fatalf("esquema antigo = %v", err)
	}
	if versao() != antes {
		t.Fatal("o assistente migrou o esquema")
	}
	// Esquema mais novo (o app é mais novo que o MCP): erro.
	mexer(`INSERT INTO goose_db_version (version_id, is_applied) VALUES (9999, 1)`)
	if _, err := AbrirExistente(ctx, caminho); !errors.Is(err, ErrEsquemaDiferente) {
		t.Fatalf("esquema novo = %v", err)
	}
	// Um arquivo que não é banco do Applyer: erro, sem criar tabela.
	outro := filepath.Join(dir, "outro.db")
	os.WriteFile(outro, nil, 0o600)
	if _, err := AbrirExistente(ctx, outro); !errors.Is(err, ErrEsquemaDiferente) {
		t.Fatalf("arquivo vazio = %v", err)
	}
	if fi, _ := os.Stat(outro); fi.Size() != 0 {
		t.Fatal("escreveu no arquivo que não é banco")
	}
}
