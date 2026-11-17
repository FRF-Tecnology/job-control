// Package banco abre o Postgres da nuvem e aplica as migrações (goose, esquema `applyer`).
package banco

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migracoes/*.sql
var migracoes embed.FS

// Esquema é o esquema Postgres onde ficam as tabelas e a tabela de versões do goose.
const Esquema = "applyer"

// Abrir conecta em dsn, cria o esquema, aplica as migrações pendentes e devolve o pool
// (cujas conexões já usam `applyer` como search_path).
func Abrir(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("DATABASE_URL inválida: %w", err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = Esquema

	if err := migrar(ctx, cfg); err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("abrir o pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("conectar no banco: %w", err)
	}
	return pool, nil
}

func migrar(ctx context.Context, cfg *pgxpool.Config) error {
	db := stdlib.OpenDB(*cfg.ConnConfig)
	defer db.Close()
	// Sem esquema não há onde pôr a tabela de versões do goose.
	if _, err := db.ExecContext(ctx, "create schema if not exists "+Esquema); err != nil {
		return fmt.Errorf("criar esquema: %w", err)
	}
	sub, err := fs.Sub(migracoes, "migracoes")
	if err != nil {
		return err
	}
	prov, err := goose.NewProvider(goose.DialectPostgres, db, sub, goose.WithTableName(Esquema+".goose_versao"))
	if err != nil {
		return fmt.Errorf("preparar migrações: %w", err)
	}
	if _, err := prov.Up(ctx); err != nil {
		return fmt.Errorf("aplicar migrações: %w", err)
	}
	return nil
}
