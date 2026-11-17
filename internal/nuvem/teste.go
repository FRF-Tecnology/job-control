package nuvem

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/FRF-Tecnology/job-control/internal/licenca"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/contas"
)

const (
	duracaoTeste   = 24 * time.Hour
	toleranciaPago = 72 * time.Hour // past_due ainda vale até 72h depois do fim do período
	validadeLic    = 30 * time.Minute
)

// Assinatura é a linha de assinaturas (espelho do Stripe, gravado pelos webhooks).
type Assinatura struct {
	Estado     string
	PeriodoFim *time.Time
}

// lerAssinatura devolve a assinatura do usuário, ou nil se não há.
func lerAssinatura(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, usuarioID string) (*Assinatura, error) {
	var a Assinatura
	err := q.QueryRow(ctx, `select estado, periodo_fim from assinaturas where usuario_id = $1::uuid`, usuarioID).
		Scan(&a.Estado, &a.PeriodoFim)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// gravarAssinatura grava o estado atual de uma assinatura do Stripe. Se o usuário já tem outra assinatura
// valendo (active, trialing, past_due), uma diferente que não vale (ex.: a antiga cancelada) não a apaga.
func gravarAssinatura(ctx context.Context, q interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}, usuarioID, stripeAssinatura, estado string, periodoFim *time.Time, agora time.Time) error {
	_, err := q.Exec(ctx, `
		insert into assinaturas (usuario_id, stripe_assinatura, estado, periodo_fim, atualizada_em)
		values ($1::uuid, $2, $3, $4, $5)
		on conflict (usuario_id) do update
		set stripe_assinatura = excluded.stripe_assinatura, estado = excluded.estado,
		    periodo_fim = excluded.periodo_fim, atualizada_em = excluded.atualizada_em
		where assinaturas.stripe_assinatura = excluded.stripe_assinatura
		   or excluded.estado in ('active', 'trialing', 'past_due')
		   or assinaturas.estado not in ('active', 'trialing', 'past_due')`,
		usuarioID, stripeAssinatura, estado, periodoFim, agora)
	return err
}

// estadoDa aplica a regra, nesta ordem: cortesia, assinatura em dia, past_due até 72h depois do fim do período
// (inclusive, como nos testes do plano), teste, bloqueado.
func estadoDa(u contas.Usuario, a *Assinatura, agora time.Time) licenca.Estado {
	if u.Cortesia {
		return licenca.Assinante
	}
	if a != nil {
		switch a.Estado {
		case "active", "trialing":
			return licenca.Assinante
		case "past_due":
			if a.PeriodoFim != nil && !agora.After(a.PeriodoFim.Add(toleranciaPago)) {
				return licenca.Assinante
			}
		}
	}
	if u.Confirmado() && u.TesteFim != nil && agora.Before(*u.TesteFim) {
		return licenca.Teste
	}
	return licenca.Bloqueado
}

// iniciarTeste começa o teste de 1 dia da conta, uma vez só. Com a linha do usuário travada, duas primeiras
// chamadas juntas não começam dois testes. O computador só fica registrado se este teste for o primeiro dele;
// se o computador já foi usado por outra conta, o teste nasce vencido.
func (s *Servidor) iniciarTeste(ctx context.Context, usuarioID, hash string, agora time.Time) error {
	tx, err := s.dep.Banco.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var inicio *time.Time
	if err := tx.QueryRow(ctx, `select teste_inicio from usuarios where id = $1::uuid for update`, usuarioID).Scan(&inicio); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return contas.ErrNaoEncontrado
		}
		return err
	}
	if inicio != nil {
		return tx.Commit(ctx) // outra chamada já começou
	}
	tag, err := tx.Exec(ctx,
		`insert into computadores_teste (hash, usuario_id, usado_em) values ($1, $2::uuid, $3) on conflict do nothing`,
		hash, usuarioID, agora)
	if err != nil {
		return err
	}
	fim := agora
	if tag.RowsAffected() == 1 {
		fim = agora.Add(duracaoTeste)
	}
	if _, err := tx.Exec(ctx, `update usuarios set teste_inicio = $2, teste_fim = $3 where id = $1::uuid`, usuarioID, agora, fim); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
