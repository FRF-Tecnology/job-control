package armazem

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// O banco para o assistente (jc-a2y.7.4, achado do CyberSec no #139). O MCP e a CLI nunca criam o banco nem migram
// o esquema: quem cria e migra é o app, ao abrir. Um MCP de outra versão que migrasse o banco com o app antigo
// aberto, ou que recriasse um central.db vazio numa corrida com o "Apagar meus dados", estragaria os dados.

var (
	// ErrBancoAusente: não há banco no caminho (nada é criado). Sem caminho na mensagem.
	ErrBancoAusente = errors.New("o banco do Applyer não existe nesta pasta: abra o Applyer uma vez")
	// ErrEsquemaDiferente: o banco está noutra versão do esquema que a deste binário (nada é migrado).
	ErrEsquemaDiferente = errors.New("o banco do Applyer está noutra versão: abra o Applyer e use o MCP e a CLI da mesma versão")
)

// AbrirExistente abre um banco que JÁ existe, sem criar (mode=rw) e sem migrar, e confere que a versão do esquema é
// a maior das migrações embutidas. O resto é igual ao Abrir (transação imediata, espera a vez).
func AbrirExistente(ctx context.Context, caminho string) (*Armazem, error) {
	if fi, err := os.Lstat(caminho); err != nil || !fi.Mode().IsRegular() {
		return nil, ErrBancoAusente
	}
	return abrirSemCriar(ctx, caminho)
}

// abrirSemCriar: o banco em mode=rw, que nunca cria o arquivo (mesmo se ele sumir entre a conferência acima e esta
// abertura, numa corrida com o "Apagar meus dados"), e a conferência da versão.
func abrirSemCriar(ctx context.Context, caminho string) (*Armazem, error) {
	dsn := "file:" + caminho + "?mode=rw&_txlock=immediate&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, ErrBancoAusente
	}
	quer, err := versaoEmbutida()
	if err != nil {
		db.Close()
		return nil, err
	}
	// A versão como o goose grava (goose_db_version), lida sem o goose: ele cria a tabela se ela não existe.
	var v sql.NullInt64
	if err := db.QueryRowContext(ctx, `SELECT MAX(version_id) FROM goose_db_version`).Scan(&v); err != nil {
		db.Close()
		if _, e := os.Lstat(caminho); e != nil {
			return nil, ErrBancoAusente
		}
		return nil, ErrEsquemaDiferente
	}
	if !v.Valid || v.Int64 != quer {
		db.Close()
		return nil, ErrEsquemaDiferente
	}
	return &Armazem{db: db, pasta: filepath.Dir(caminho), backupsMemoria: map[string]bool{}}, nil
}

// versaoEmbutida: a maior versão das migrações deste binário (o número no começo do nome, como o goose lê).
func versaoEmbutida() (int64, error) {
	nomes, err := fs.Glob(migracoes, "migracoes/*.sql")
	if err != nil || len(nomes) == 0 {
		return 0, errors.New("sem migrações embutidas")
	}
	var maior int64
	for _, n := range nomes {
		base := filepath.Base(n)
		num, _, _ := strings.Cut(base, "_")
		v, err := strconv.ParseInt(num, 10, 64)
		if err != nil {
			return 0, errors.New("migração fora do formato: " + base)
		}
		maior = max(maior, v)
	}
	return maior, nil
}
