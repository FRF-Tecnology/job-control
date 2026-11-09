package armazem

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// MetaEnvioPausado guarda se o envio de candidaturas está pausado (jc-a2y.51). Falha fechada: sem o valor, ou com
// valor estragado, o envio está PAUSADO. Por isso começa pausado num banco novo e depois de "apagar meus dados".
const MetaEnvioPausado = "envio-pausado"

func lerPausado(ctx context.Context, q leitorLinha) bool {
	var valor string
	if err := q.QueryRowContext(ctx, `SELECT valor FROM meta WHERE chave = ?`, MetaEnvioPausado).Scan(&valor); err != nil {
		return true
	}
	var pausado bool
	if err := json.Unmarshal([]byte(valor), &pausado); err != nil {
		return true
	}
	return pausado
}

// EnvioPausado lê o estado gravado (falha fechada: na dúvida, pausado).
func (a *Armazem) EnvioPausado(ctx context.Context) bool { return lerPausado(ctx, a.db) }

// GravarEnvioPausado grava o estado. Só a tela retoma (a API confere o autor).
func (a *Armazem) GravarEnvioPausado(ctx context.Context, pausado bool) error {
	b, _ := json.Marshal(pausado)
	return a.Transacao(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO meta (chave, valor) VALUES (?, ?) ON CONFLICT(chave) DO UPDATE SET valor = excluded.valor`, MetaEnvioPausado, string(b))
		return err
	})
}

// ErrEnvioPausado: a gravação que começaria um envio foi recusada porque o envio está pausado.
var ErrEnvioPausado = errors.New("o envio está pausado")

// GravarSeEnvioLiberado é o Gravar com a conferência do pausado na mesma transação: com o envio pausado, nada é
// gravado e volta ErrEnvioPausado. Assim, pausar e entrar em enviando nunca se cruzam.
func (a *Armazem) GravarSeEnvioLiberado(ctx context.Context, fn func(dominio.Estado) (dominio.Estado, error)) error {
	return a.gravarCom(ctx, func(tx *sql.Tx, e dominio.Estado) (dominio.Estado, error) {
		if lerPausado(ctx, tx) {
			return e, ErrEnvioPausado
		}
		return fn(e)
	})
}
