// Package codigos gera e confere os códigos de 6 dígitos enviados por email
// (confirmar o email, recuperar a senha). Só o HMAC do código fica no banco.
package codigos

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Tipo diz para que serve o código.
type Tipo string

const (
	Confirmar Tipo = "confirmar"
	Recuperar Tipo = "recuperar"
)

const (
	validade      = 15 * time.Minute
	intervaloMin  = 60 * time.Second
	maxTentativas = 5
	digitosCodigo = 6
	limiteCodigo  = 1000000

	// Orçamento por email+tipo, somando todos os pedidos (criar, entrar pendente, recuperar, 409 dos termos).
	maxEnviosCurto  = 5
	janelaCurta     = 15 * time.Minute
	maxEnviosDia    = 20
	maxFalhasDia    = 20 // códigos errados em 24 h, somando todos os códigos
	janelaDia       = 24 * time.Hour
	eventoEnvio     = "envio"
	eventoFalha     = "falha"
	sqlContarEvento = `select count(*) filter (where em > $4), count(*) from eventos_codigo
		 where email = $1 and tipo = $2 and evento = $3 and em > $5`
)

// contar devolve quantos eventos do email+tipo houve desde curto e desde dia.
func contar(ctx context.Context, tx pgx.Tx, email string, t Tipo, evento string, curto, dia time.Time) (int, int, error) {
	var nCurto, nDia int
	err := tx.QueryRow(ctx, sqlContarEvento, email, string(t), evento, curto, dia).Scan(&nCurto, &nDia)
	return nCurto, nDia, err
}

func registrar(ctx context.Context, tx pgx.Tx, email string, t Tipo, evento string, agora time.Time) error {
	_, err := tx.Exec(ctx, `insert into eventos_codigo (email, tipo, evento, em) values ($1, $2, $3, $4)`,
		email, string(t), evento, agora)
	return err
}

var (
	ErrEspere         = errors.New("espere antes de pedir outro código")
	ErrCodigoInvalido = errors.New("código inválido")
	ErrTentativas     = errors.New("tentativas demais")
	ErrOrcamento      = errors.New("códigos demais para este email")
)

// Repo guarda os códigos no Postgres.
type Repo struct {
	db        *pgxpool.Pool
	segredo   []byte
	agora     func() time.Time
	aleatorio io.Reader
}

// Novo cria o repositório. agora e aleatorio existem para os testes
// (em produção: time.Now e crypto/rand.Reader).
func Novo(db *pgxpool.Pool, segredo []byte, agora func() time.Time, aleatorio io.Reader) *Repo {
	return &Repo{db: db, segredo: segredo, agora: agora, aleatorio: aleatorio}
}

func (r *Repo) hash(email string, t Tipo, codigo string) string {
	m := hmac.New(sha256.New, r.segredo)
	m.Write([]byte(email + "\x00" + string(t) + "\x00" + codigo))
	return hex.EncodeToString(m.Sum(nil))
}

// trava serializa as chamadas do mesmo email+tipo até o fim da transação.
func trava(ctx context.Context, tx pgx.Tx, email string, t Tipo) error {
	_, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext($1))`, email+"|"+string(t))
	return err
}

// Gerar cria um código novo (e invalida os anteriores). ErrEspere se o último
// do mesmo tipo foi criado há menos de 60 s; ErrOrcamento acima de 5 envios em 15 min
// ou 20 em 24 h. O email já deve estar normalizado.
func (r *Repo) Gerar(ctx context.Context, email string, t Tipo) (string, error) {
	n, err := sortear(r.aleatorio)
	if err != nil {
		return "", err
	}
	codigo := fmt.Sprintf("%0*d", digitosCodigo, n)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	if err := trava(ctx, tx, email, t); err != nil {
		return "", err
	}
	agora := r.agora()
	var ultimo time.Time
	err = tx.QueryRow(ctx,
		`select criado_em from codigos where email = $1 and tipo = $2 order by criado_em desc limit 1`,
		email, string(t)).Scan(&ultimo)
	if err == nil && agora.Sub(ultimo) < intervaloMin {
		return "", ErrEspere
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	curto, dia, err := contar(ctx, tx, email, t, eventoEnvio, agora.Add(-janelaCurta), agora.Add(-janelaDia))
	if err != nil {
		return "", err
	}
	if curto >= maxEnviosCurto || dia >= maxEnviosDia {
		return "", ErrOrcamento
	}
	if err := registrar(ctx, tx, email, t, eventoEnvio, agora); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx,
		`update codigos set usado_em = $3 where email = $1 and tipo = $2 and usado_em is null`,
		email, string(t), agora); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx,
		`insert into codigos (email, tipo, hash, expira_em, criado_em) values ($1, $2, $3, $4, $5)`,
		email, string(t), r.hash(email, t, codigo), agora.Add(validade), agora); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return codigo, nil
}

// sortear devolve um número uniforme em [0, 1000000) lendo de l (rejeição evita viés).
func sortear(l io.Reader) (int64, error) {
	for {
		var b [4]byte
		if _, err := io.ReadFull(l, b[:]); err != nil {
			return 0, err
		}
		v := int64(binary.BigEndian.Uint32(b[:]))
		if v < (1<<32)/limiteCodigo*limiteCodigo {
			return v % limiteCodigo, nil
		}
	}
}

// Conferir valida o código. Cada erro conta uma tentativa; a 6ª (mesmo certa) dá ErrTentativas.
// Com 20 erros em 24 h no email+tipo (somando todos os códigos), todo código dá ErrTentativas.
// No acerto marca o código como usado.
func (r *Repo) Conferir(ctx context.Context, email string, t Tipo, codigo string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := trava(ctx, tx, email, t); err != nil {
		return err
	}
	agora := r.agora()
	var id, hash string
	var tentativas int
	err = tx.QueryRow(ctx,
		`select id::text, hash, tentativas from codigos
		 where email = $1 and tipo = $2 and usado_em is null and expira_em > $3
		 order by criado_em desc limit 1 for update`,
		email, string(t), agora).Scan(&id, &hash, &tentativas)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCodigoInvalido
	}
	if err != nil {
		return err
	}
	if tentativas >= maxTentativas {
		return ErrTentativas
	}
	_, falhasDia, err := contar(ctx, tx, email, t, eventoFalha, agora, agora.Add(-janelaDia))
	if err != nil {
		return err
	}
	if falhasDia >= maxFalhasDia {
		return ErrTentativas
	}
	if hmac.Equal([]byte(hash), []byte(r.hash(email, t, codigo))) {
		if _, err := tx.Exec(ctx, `update codigos set usado_em = $2 where id = $1::uuid`, id, agora); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `update codigos set tentativas = tentativas + 1 where id = $1::uuid`, id); err != nil {
		return err
	}
	if err := registrar(ctx, tx, email, t, eventoFalha, agora); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return ErrCodigoInvalido
}
