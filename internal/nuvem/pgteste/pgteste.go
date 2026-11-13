// Package pgteste dá um Postgres de verdade para os testes da nuvem.
//
// Com APPLYER_PG_TESTE (DSN de um Postgres já rodando, como o do CI) usa esse servidor. Sem ela, sobe um Postgres
// embutido (sem janela, sem Docker) uma vez por processo de teste; os binários ficam em
// os.UserCacheDir()/applyer-pg e só baixam na primeira vez. Cada Banco(t) ganha um banco novo, apagado no fim.
// Os pacotes chamam Principal no TestMain para o servidor embutido ser parado ao fim dos testes.
package pgteste

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FRF-Tecnology/job-control/internal/nuvem/banco"
)

var (
	uma      sync.Once
	dsnBase  string // DSN do banco administrativo ("postgres")
	errBase  error
	servidor *embeddedpostgres.EmbeddedPostgres
	pastas   []string
)

// Principal roda os testes do pacote e depois para o Postgres embutido (se foi iniciado). Uso:
//
//	func TestMain(m *testing.M) { os.Exit(pgteste.Principal(m)) }
func Principal(m *testing.M) int {
	codigo := m.Run()
	if servidor != nil {
		_ = servidor.Stop()
	}
	for _, p := range pastas {
		_ = os.RemoveAll(p)
	}
	return codigo
}

func iniciar() (string, error) {
	uma.Do(func() {
		if modoDe(os.Getenv) == modoExterno {
			dsnBase = os.Getenv("APPLYER_PG_TESTE")
			return
		}
		dsnBase, errBase = subirEmbutido()
	})
	return dsnBase, errBase
}

func subirEmbutido() (string, error) {
	porta, err := portaLivre()
	if err != nil {
		return "", err
	}
	raiz, err := os.MkdirTemp("", "applyer-pg-")
	if err != nil {
		return "", err
	}
	pastas = append(pastas, raiz)
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	binarios := filepath.Join(cache, "applyer-pg")
	cfg := embeddedpostgres.DefaultConfig().
		Version(embeddedpostgres.V17).
		Port(uint32(porta)).
		DataPath(filepath.Join(raiz, "dados")).
		RuntimePath(filepath.Join(raiz, "exec")).
		BinariesPath(filepath.Join(binarios, "bin")).
		CachePath(filepath.Join(binarios, "cache")).
		Username("postgres").Password("teste").Database("postgres").
		StartTimeout(120 * time.Second)
	servidor = embeddedpostgres.NewDatabase(cfg)
	// Pacotes de teste rodam em paralelo (um processo cada): só um por vez baixa e extrai os binários.
	soltar, err := travar(filepath.Join(binarios, "trava"))
	if err != nil {
		return "", err
	}
	defer soltar()
	marcarExtraido(binarios)
	if err := servidor.Start(); err != nil {
		servidor = nil
		_ = os.RemoveAll(filepath.Join(binarios, "ok")) // extração pode estar pela metade: refaz na próxima
		_ = os.RemoveAll(filepath.Join(binarios, "bin"))
		return "", fmt.Errorf("subir o Postgres embutido: %w", err)
	}
	concluirExtracao(binarios)
	return fmt.Sprintf("postgres://postgres:teste@127.0.0.1:%d/postgres?sslmode=disable", porta), nil
}

func portaLivre() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// Banco cria um banco novo e vazio, aplica as migrações e o devolve; apaga tudo no t.Cleanup.
func Banco(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if modoDe(os.Getenv) == modoPular {
		t.Skip("Postgres dos testes roda no job nuvem do CI")
	}
	base, err := iniciar()
	if err != nil {
		t.Fatalf("Postgres de teste: %v", err)
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, base)
	if err != nil {
		t.Fatalf("conectar no Postgres de teste: %v", err)
	}
	defer admin.Close()

	var b [8]byte
	_, _ = rand.Read(b[:])
	nome := "applyer_t_" + hex.EncodeToString(b[:])
	if _, err := admin.Exec(ctx, "create database "+nome); err != nil {
		t.Fatalf("criar banco de teste: %v", err)
	}
	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + nome
	pool, err := banco.Abrir(ctx, u.String())
	if err != nil {
		t.Fatalf("abrir banco de teste: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		a, err := pgxpool.New(ctx, base)
		if err != nil {
			return
		}
		defer a.Close()
		_, _ = a.Exec(ctx, "drop database if exists "+nome+" with (force)")
	})
	return pool
}

// travar é uma trava entre processos: mkdir é atômico. Trava abandonada há mais de 3 min é refeita.
func travar(dir string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return nil, err
	}
	limite := time.Now().Add(5 * time.Minute)
	for {
		err := os.Mkdir(dir, 0o755)
		if err == nil {
			return func() { _ = os.Remove(dir) }, nil
		}
		if !os.IsExist(err) {
			return nil, err
		}
		if st, e := os.Stat(dir); e == nil && time.Since(st.ModTime()) > 3*time.Minute {
			_ = os.Remove(dir)
			continue
		}
		if time.Now().After(limite) {
			return nil, fmt.Errorf("trava de %s não liberou", dir)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// marcarExtraido evita reextrair os binários a cada processo no Windows: a biblioteca procura "bin/bin/pg_ctl" sem
// ".exe" e, não achando, extrai de novo por cima de arquivos que outro Postgres está usando. Só grava a marca se
// um Start anterior terminou bem (arquivo "ok"); extração interrompida nunca é marcada.
func marcarExtraido(binarios string) {
	if _, err := os.Stat(filepath.Join(binarios, "ok")); err != nil {
		return
	}
	marca := filepath.Join(binarios, "bin", "bin", "pg_ctl")
	if _, err := os.Stat(marca); err != nil {
		_ = os.WriteFile(marca, nil, 0o644)
	}
}

// concluirExtracao registra que o primeiro Start deu certo, então os binários estão completos.
func concluirExtracao(binarios string) {
	_ = os.WriteFile(filepath.Join(binarios, "ok"), nil, 0o644)
	marcarExtraido(binarios)
}

// modo diz de onde vem o Postgres dos testes, só olhando o ambiente.
type modo int

const (
	modoExterno  modo = iota // APPLYER_PG_TESTE aponta para um servidor já rodando
	modoEmbutido             // sobe o Postgres embutido (uso local)
	modoPular                // no CI sem APPLYER_PG_TESTE: quem roda o Postgres é o job nuvem
)

func modoDe(env func(string) string) modo {
	switch {
	case env("APPLYER_PG_TESTE") != "":
		return modoExterno
	case env("CI") != "":
		return modoPular
	}
	return modoEmbutido
}
