package armazem

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// LerMeta lê uma configuração do app (JSON) para dentro de destino. Devolve false se não existe.
func (a *Armazem) LerMeta(ctx context.Context, chave string, destino any) (bool, error) {
	var valor string
	err := a.db.QueryRowContext(ctx, `SELECT valor FROM meta WHERE chave = ?`, chave).Scan(&valor)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal([]byte(valor), destino)
}

// GravarMeta grava uma configuração do app (JSON).
func (a *Armazem) GravarMeta(ctx context.Context, chave string, valor any) error {
	b, err := json.Marshal(valor)
	if err != nil {
		return err
	}
	_, err = a.db.ExecContext(ctx, `INSERT INTO meta (chave, valor) VALUES (?, ?) ON CONFLICT(chave) DO UPDATE SET valor = excluded.valor`, chave, string(b))
	return err
}

// RegistrarUso guarda um registro de uso de IA (JSON).
func (a *Armazem) RegistrarUso(ctx context.Context, data string, registro any) error {
	b, err := json.Marshal(registro)
	if err != nil {
		return err
	}
	_, err = a.db.ExecContext(ctx, `INSERT INTO uso_ia (data, dados) VALUES (?, ?)`, data, string(b))
	return err
}

// UsosDesde lê os registros de uso de IA a partir de uma data (AAAA-MM-DD), do mais antigo ao mais novo.
func (a *Armazem) UsosDesde(ctx context.Context, data string) ([]json.RawMessage, error) {
	linhas, err := a.db.QueryContext(ctx, `SELECT dados FROM uso_ia WHERE data >= ? ORDER BY id`, data)
	if err != nil {
		return nil, err
	}
	defer linhas.Close()
	var saida []json.RawMessage
	for linhas.Next() {
		var d string
		if err := linhas.Scan(&d); err != nil {
			return nil, err
		}
		saida = append(saida, json.RawMessage(d))
	}
	return saida, linhas.Err()
}
