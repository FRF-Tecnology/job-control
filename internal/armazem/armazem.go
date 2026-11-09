// Package armazem guarda os dados da Central num arquivo SQLite local.
// As migrações ficam embutidas no binário e rodam ao abrir.
package armazem

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // SQLite em Go puro: sem cgo, compila igual em Windows, Mac e Linux.
)

//go:embed migracoes/*.sql
var migracoes embed.FS

// Armazem é a conexão com o banco local.
type Armazem struct {
	db    *sql.DB
	pasta string // pasta do banco; "" para banco em memória

	travaBackup    sync.Mutex
	backupsMemoria map[string]bool
}

// Abrir abre (ou cria) o banco em caminho e aplica as migrações pendentes.
// caminho ":memory:" serve para testes.
func Abrir(ctx context.Context, caminho string) (*Armazem, error) {
	// _txlock=immediate: quem grava pega a vez no começo da transação (app, CLI e MCP não se atropelam).
	dsn := "file::memory:?_txlock=immediate&_pragma=foreign_keys(1)"
	pasta := ""
	if caminho != ":memory:" {
		// WAL: leituras não esperam escritas. busy_timeout: espera a vez em vez de falhar.
		dsn = "file:" + caminho + "?_txlock=immediate&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
		pasta = filepath.Dir(caminho)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("abrir banco: %w", err)
	}
	if caminho == ":memory:" {
		db.SetMaxOpenConns(1) // cada conexão ":memory:" seria um banco diferente
	}
	if err := migrar(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return &Armazem{db: db, pasta: pasta, backupsMemoria: map[string]bool{}}, nil
}

func migrar(ctx context.Context, db *sql.DB) error {
	prov, err := goose.NewProvider(goose.DialectSQLite3, db, mustSub(migracoes, "migracoes"))
	if err != nil {
		return fmt.Errorf("preparar migrações: %w", err)
	}
	if _, err := prov.Up(ctx); err != nil {
		return fmt.Errorf("aplicar migrações: %w", err)
	}
	return nil
}

// Fechar fecha o banco.
func (a *Armazem) Fechar() error { return a.db.Close() }

// Versao é um contador que sobe a cada gravação. A tela consulta para saber se precisa recarregar.
func (a *Armazem) Versao(ctx context.Context) (int64, error) {
	var v int64
	err := a.db.QueryRowContext(ctx, `SELECT valor FROM versao WHERE id = 1`).Scan(&v)
	return v, err
}

// SubirVersao incrementa o contador dentro de uma transação de gravação.
func SubirVersao(ctx context.Context, tx *sql.Tx) (int64, error) {
	var v int64
	err := tx.QueryRowContext(ctx, `UPDATE versao SET valor = valor + 1 WHERE id = 1 RETURNING valor`).Scan(&v)
	return v, err
}

// Transacao roda fn numa transação; erro desfaz tudo.
func (a *Armazem) Transacao(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}
