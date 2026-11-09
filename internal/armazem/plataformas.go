package armazem

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// MetaEnvioPlataformas guarda o estado do envio por plataforma (jc-a2y.6.11). Sem o valor (banco novo), todas
// ativas: o envio geral já começa pausado. Valor estragado: falha fechada, todas pausadas.
const MetaEnvioPlataformas = "envio-plataformas"

func lerPlataformas(ctx context.Context, q leitorLinha) dominio.Plataformas {
	var valor string
	err := q.QueryRowContext(ctx, `SELECT valor FROM meta WHERE chave = ?`, MetaEnvioPlataformas).Scan(&valor)
	if errors.Is(err, sql.ErrNoRows) {
		return dominio.Plataformas{}
	}
	var p dominio.Plataformas
	if err != nil || json.Unmarshal([]byte(valor), &p) != nil || p == nil {
		return todasPausadas()
	}
	for nome, s := range p {
		switch s.Estado {
		case dominio.PlataformaAtiva, dominio.PlataformaPausada, dominio.PlataformaPrecisaEntrar, dominio.PlataformaParadaHoje:
		default:
			p[nome] = dominio.EstadoPlataforma{Estado: dominio.PlataformaPausada} // estado desconhecido: pausada
		}
	}
	return p
}

func todasPausadas() dominio.Plataformas {
	p := dominio.Plataformas{}
	for _, nome := range dominio.PlataformasDoEnvio {
		p[nome] = dominio.EstadoPlataforma{Estado: dominio.PlataformaPausada}
	}
	return p
}

func gravarPlataformas(ctx context.Context, tx *sql.Tx, p dominio.Plataformas) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO meta (chave, valor) VALUES (?, ?) ON CONFLICT(chave) DO UPDATE SET valor = excluded.valor`, MetaEnvioPlataformas, string(b))
	return err
}

// Plataformas lê o estado gravado (falha fechada).
func (a *Armazem) Plataformas(ctx context.Context) dominio.Plataformas {
	return lerPlataformas(ctx, a.db)
}

// MudarPlataformas lê, muda e grava o estado das plataformas numa transação. A mudança devolve erro para não gravar.
func (a *Armazem) MudarPlataformas(ctx context.Context, fn func(p dominio.Plataformas, pausado bool) (dominio.Plataformas, error)) (dominio.Plataformas, error) {
	var novo dominio.Plataformas
	err := a.Transacao(ctx, func(tx *sql.Tx) error {
		p, err := fn(lerPlataformas(ctx, tx), lerPausado(ctx, tx))
		if err != nil {
			return err
		}
		novo = p
		return gravarPlataformas(ctx, tx, p)
	})
	return novo, err
}

// ErrPlataformaParada: a gravação que começaria um envio foi recusada porque a plataforma não está ativa.
var ErrPlataformaParada = errors.New("a plataforma está parada")

// GravarSeLiberado confere, na mesma transação, o envio geral e a plataforma: pausado volta ErrEnvioPausado;
// plataforma fora de ativa volta ErrPlataformaParada. Nada é gravado nos dois casos.
func (a *Armazem) GravarSeLiberado(ctx context.Context, plataforma string, agora time.Time, fn func(dominio.Estado) (dominio.Estado, error)) error {
	return a.gravarCom(ctx, func(tx *sql.Tx, e dominio.Estado) (dominio.Estado, error) {
		if lerPausado(ctx, tx) {
			return e, ErrEnvioPausado
		}
		if !lerPlataformas(ctx, tx).Ativa(plataforma, agora) {
			return e, ErrPlataformaParada
		}
		return fn(e)
	})
}

// GravarComPlataformas grava o estado e o das plataformas na mesma transação (o desfecho da candidatura e o efeito
// dele na plataforma nunca ficam um sem o outro).
func (a *Armazem) GravarComPlataformas(ctx context.Context, fn func(dominio.Estado, dominio.Plataformas) (dominio.Estado, dominio.Plataformas, error)) error {
	return a.gravarCom(ctx, func(tx *sql.Tx, e dominio.Estado) (dominio.Estado, error) {
		novo, p, err := fn(e, lerPlataformas(ctx, tx))
		if err != nil {
			return e, err
		}
		if err := gravarPlataformas(ctx, tx, p); err != nil {
			return e, err
		}
		return novo, nil
	})
}
