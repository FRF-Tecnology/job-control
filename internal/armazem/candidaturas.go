package armazem

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// MetaLimitesCandidatura guarda o limite diário por plataforma ({"linkedin": 30, ...}). Fica no SQLite, então entra
// no backup diário junto com a fila.
const MetaLimitesCandidatura = "candidaturas-limites"

// LimitesCandidatura lê os limites gravados. Valor fora de 1..40 ou plataforma sem limite é ignorado (vale o padrão).
func (a *Armazem) LimitesCandidatura(ctx context.Context) (map[string]int, error) {
	return lerLimites(ctx, a.db)
}

type leitorLinha interface {
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}

func lerLimites(ctx context.Context, q leitorLinha) (map[string]int, error) {
	var valor string
	err := q.QueryRowContext(ctx, `SELECT valor FROM meta WHERE chave = ?`, MetaLimitesCandidatura).Scan(&valor)
	if errors.Is(err, sql.ErrNoRows) {
		return map[string]int{}, nil
	}
	if err != nil {
		return nil, err
	}
	var bruto map[string]int
	if err := json.Unmarshal([]byte(valor), &bruto); err != nil {
		return nil, fmt.Errorf("limites de candidatura corrompidos: %w", err)
	}
	limites := map[string]int{}
	for p, n := range bruto {
		if _, tem := dominio.LimiteDe(nil, p); tem && dominio.ValidarLimite(n) == nil {
			limites[p] = n
		}
	}
	return limites, nil
}

// GravarLimiteCandidatura troca o limite de uma plataforma (linkedin, gupy ou inhire; de 1 a 40) numa transação.
func (a *Armazem) GravarLimiteCandidatura(ctx context.Context, plataforma string, n int) error {
	if _, tem := dominio.LimiteDe(nil, plataforma); !tem {
		return &dominio.ErroValidacao{Erros: []string{"plataforma sem limite diário: " + plataforma}}
	}
	if err := dominio.ValidarLimite(n); err != nil {
		return err
	}
	return a.Transacao(ctx, func(tx *sql.Tx) error {
		limites, err := lerLimites(ctx, tx)
		if err != nil {
			return err
		}
		limites[plataforma] = n
		b, err := json.Marshal(limites)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO meta (chave, valor) VALUES (?, ?) ON CONFLICT(chave) DO UPDATE SET valor = excluded.valor`, MetaLimitesCandidatura, string(b))
		return err
	})
}
