package armazem

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Coleções guardadas na tabela itens, na ordem em que aparecem no estado.
var colecoes = []string{"vagas", "eventos", "plataformas", "rodadas", "pedidos", "emails"}

func listaDe(e *dominio.Estado, colecao string) *[]dominio.Objeto {
	switch colecao {
	case "vagas":
		return &e.Vagas
	case "eventos":
		return &e.Eventos
	case "plataformas":
		return &e.Plataformas
	case "rodadas":
		return &e.Rodadas
	case "pedidos":
		return &e.Pedidos
	case "emails":
		return &e.Emails
	}
	panic("coleção desconhecida: " + colecao)
}

// consultor é o que *sql.DB e *sql.Tx têm em comum.
type consultor interface {
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
}

// fotos guarda o JSON de cada item como estava no banco, para gravar só o que mudou.
type fotos map[string][]string

func carregar(ctx context.Context, q consultor) (dominio.Estado, fotos, error) {
	e := dominio.EstadoVazio()
	f := fotos{}
	linhas, err := q.QueryContext(ctx, `SELECT colecao, dados FROM itens ORDER BY colecao, ordem`)
	if err != nil {
		return e, nil, fmt.Errorf("ler dados: %w", err)
	}
	defer linhas.Close()
	for linhas.Next() {
		var colecao, dados string
		if err := linhas.Scan(&colecao, &dados); err != nil {
			return e, nil, err
		}
		var o dominio.Objeto
		if err := json.Unmarshal([]byte(dados), &o); err != nil {
			return e, nil, fmt.Errorf("dados corrompidos em %s: %w", colecao, err)
		}
		f[colecao] = append(f[colecao], dados)
		if colecao == "perfil" {
			e.Perfil = o
			continue
		}
		l := listaDe(&e, colecao)
		*l = append(*l, o)
	}
	return e, f, linhas.Err()
}

// emJSON grava sem escapar <, > e & (o texto fica legível no banco).
func emJSON(o dominio.Objeto) (string, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(o); err != nil {
		return "", err
	}
	return string(bytes.TrimRight(b.Bytes(), "\n")), nil
}

func salvarColecao(ctx context.Context, tx *sql.Tx, colecao string, itens []dominio.Objeto, antes []string) error {
	for i, o := range itens {
		dados, err := emJSON(o)
		if err != nil {
			return fmt.Errorf("gravar %s: %w", colecao, err)
		}
		switch {
		case i >= len(antes):
			_, err = tx.ExecContext(ctx, `INSERT INTO itens (colecao, ordem, dados) VALUES (?, ?, ?)`, colecao, i, dados)
		case antes[i] != dados:
			_, err = tx.ExecContext(ctx, `UPDATE itens SET dados = ? WHERE colecao = ? AND ordem = ?`, dados, colecao, i)
		}
		if err != nil {
			return err
		}
	}
	if len(itens) < len(antes) {
		_, err := tx.ExecContext(ctx, `DELETE FROM itens WHERE colecao = ? AND ordem >= ?`, colecao, len(itens))
		return err
	}
	return nil
}

// Carregar lê o estado inteiro da conta.
func (a *Armazem) Carregar(ctx context.Context) (dominio.Estado, error) {
	e, _, err := carregar(ctx, a.db)
	return e, err
}

// Gravar roda fn sobre o estado atual numa transação: grava o que mudou e sobe a versão.
// Se fn devolver erro, nada muda. Vale entre processos (app, CLI e MCP abertos juntos).
func (a *Armazem) Gravar(ctx context.Context, fn func(dominio.Estado) (dominio.Estado, error)) error {
	return a.Transacao(ctx, func(tx *sql.Tx) error {
		antes, f, err := carregar(ctx, tx)
		if err != nil {
			return err
		}
		novo, err := fn(antes)
		if err != nil {
			return err
		}
		for _, c := range colecoes {
			if err := salvarColecao(ctx, tx, c, *listaDe(&novo, c), f[c]); err != nil {
				return err
			}
		}
		perfil := novo.Perfil
		if perfil == nil {
			perfil = dominio.Objeto{}
		}
		if err := salvarColecao(ctx, tx, "perfil", []dominio.Objeto{perfil}, f["perfil"]); err != nil {
			return err
		}
		_, err = SubirVersao(ctx, tx)
		return err
	})
}

// BackupDiario copia o banco para backups/central-AAAA-MM-DD.db uma vez por dia.
// Devolve false se o backup de hoje já existe. Banco em memória: só lembra a data.
func (a *Armazem) BackupDiario(ctx context.Context, hoje string) (bool, error) {
	a.travaBackup.Lock()
	defer a.travaBackup.Unlock()
	if a.pasta == "" {
		if a.backupsMemoria[hoje] {
			return false, nil
		}
		a.backupsMemoria[hoje] = true
		return true, nil
	}
	destino := filepath.Join(a.pasta, "backups", "central-"+hoje+".db")
	if _, err := os.Stat(destino); err == nil {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(destino), 0o700); err != nil {
		return false, err
	}
	if _, err := a.db.ExecContext(ctx, `VACUUM INTO ?`, destino); err != nil {
		return false, fmt.Errorf("backup: %w", err)
	}
	return true, nil
}

// Pasta é onde o banco mora ("" para banco em memória).
func (a *Armazem) Pasta() string { return a.pasta }
