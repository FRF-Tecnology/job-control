// Package sessoes emite e confere os tokens da nuvem.
//
// Acesso: JWT EdDSA de 15 min (iss "applyer-nuvem", sub = usuário). Renovação: 32 bytes aleatórios,
// uso único, guardados só como SHA-256. Cada login abre uma família; cada renovação troca o token
// por um novo na mesma família. Um token já trocado que volta (reuso) revoga a família inteira.
// Sessão parada por 60 dias expira. Nada aqui registra tokens em log.
package sessoes

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	emissor        = "applyer-nuvem"
	duracaoAcesso  = 15 * time.Minute
	sessaoParada   = 60 * 24 * time.Hour
	bytesRenovacao = 32
)

var (
	// ErrReuso: o token de renovação já tinha sido usado; a família foi revogada.
	ErrReuso = errors.New("token de renovação reutilizado")
	// ErrSessao: token desconhecido, revogado ou de sessão parada há mais de 60 dias.
	ErrSessao = errors.New("sessão inválida")
)

// Par é o que o cliente recebe ao entrar ou renovar.
type Par struct {
	Acesso    string    `json:"acesso"`
	Renovacao string    `json:"renovacao"`
	AcessoAte time.Time `json:"acesso_ate"`
}

// Repo guarda as sessões no Postgres e assina os tokens de acesso.
type Repo struct {
	db        *pgxpool.Pool
	priv      ed25519.PrivateKey
	pub       ed25519.PublicKey
	agora     func() time.Time
	aleatorio io.Reader
}

// Novo cria o repositório. agora e aleatorio existem para os testes
// (em produção: time.Now e crypto/rand.Reader).
func Novo(db *pgxpool.Pool, priv ed25519.PrivateKey, agora func() time.Time, aleatorio io.Reader) *Repo {
	return &Repo{db: db, priv: priv, pub: priv.Public().(ed25519.PublicKey), agora: agora, aleatorio: aleatorio}
}

func hashToken(t string) string {
	s := sha256.Sum256([]byte(t))
	return hex.EncodeToString(s[:])
}

func (r *Repo) sortearRenovacao() (string, error) {
	b := make([]byte, bytesRenovacao)
	if _, err := io.ReadFull(r.aleatorio, b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (r *Repo) assinar(usuarioID string, agora time.Time) (string, time.Time, error) {
	ate := agora.Add(duracaoAcesso)
	tok, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.RegisteredClaims{
		Issuer:    emissor,
		Subject:   usuarioID,
		IssuedAt:  jwt.NewNumericDate(agora),
		ExpiresAt: jwt.NewNumericDate(ate),
	}).SignedString(r.priv)
	return tok, ate, err
}

// trava serializa, até o fim da transação, tudo o que mexe nas sessões do usuário. Sem ela,
// uma revogação poderia não ver a linha que uma renovação concorrente acabou de inserir.
func trava(ctx context.Context, tx pgx.Tx, usuarioID string) error {
	_, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext('sessoes|' || $1))`, usuarioID)
	return err
}

// inserir grava um token novo na família e devolve o par.
func (r *Repo) inserir(ctx context.Context, tx pgx.Tx, usuarioID, familia, dispositivo string, agora time.Time) (Par, error) {
	ren, err := r.sortearRenovacao()
	if err != nil {
		return Par{}, err
	}
	if _, err := tx.Exec(ctx,
		`insert into sessoes (usuario_id, familia, hash_renovacao, dispositivo_hash, criada_em, usada_em)
		 values ($1::uuid, $2::uuid, $3, $4, $5, $5)`,
		usuarioID, familia, hashToken(ren), dispositivo, agora); err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23503" { // a conta foi apagada agora há pouco
			return Par{}, ErrSessao
		}
		return Par{}, err
	}
	acesso, ate, err := r.assinar(usuarioID, agora)
	if err != nil {
		return Par{}, err
	}
	return Par{Acesso: acesso, Renovacao: ren, AcessoAte: ate}, nil
}

// Abrir começa uma família nova de sessão (login). dispositivoHash só é guardado.
func (r *Repo) Abrir(ctx context.Context, usuarioID, dispositivoHash string) (Par, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Par{}, err
	}
	defer tx.Rollback(ctx)
	if err := trava(ctx, tx, usuarioID); err != nil {
		return Par{}, err
	}
	var familia string
	if err := tx.QueryRow(ctx, `select gen_random_uuid()::text`).Scan(&familia); err != nil {
		return Par{}, err
	}
	p, err := r.inserir(ctx, tx, usuarioID, familia, dispositivoHash, r.agora())
	if err != nil {
		return Par{}, err
	}
	return p, tx.Commit(ctx)
}

// Renovar troca o token de renovação por um par novo. Token já trocado → revoga a família e
// devolve ErrReuso; desconhecido, revogado ou parado há mais de 60 dias → ErrSessao.
func (r *Repo) Renovar(ctx context.Context, renovacao string) (Par, error) {
	h := hashToken(renovacao)
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Par{}, err
	}
	defer tx.Rollback(ctx)

	var usuarioID string
	err = tx.QueryRow(ctx, `select usuario_id::text from sessoes where hash_renovacao = $1`, h).Scan(&usuarioID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Par{}, ErrSessao
	}
	if err != nil {
		return Par{}, err
	}
	if err := trava(ctx, tx, usuarioID); err != nil {
		return Par{}, err
	}
	// Relê com a trava e a linha presas: o que vale é o estado depois de quem chegou antes.
	var id, familia, dispositivo string
	var usada time.Time
	var substituida, revogada *time.Time
	err = tx.QueryRow(ctx,
		`select id::text, familia::text, dispositivo_hash, usada_em, substituida_em, revogada_em
		 from sessoes where hash_renovacao = $1 for update`, h).
		Scan(&id, &familia, &dispositivo, &usada, &substituida, &revogada)
	if errors.Is(err, pgx.ErrNoRows) { // a linha sumiu entre as duas leituras (ex.: conta apagada)
		return Par{}, ErrSessao
	}
	if err != nil {
		return Par{}, err
	}
	agora := r.agora()
	if substituida != nil {
		if _, err := tx.Exec(ctx,
			`update sessoes set revogada_em = $2 where familia = $1::uuid and revogada_em is null`,
			familia, agora); err != nil {
			return Par{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Par{}, err
		}
		return Par{}, ErrReuso
	}
	if revogada != nil || agora.Sub(usada) > sessaoParada {
		return Par{}, ErrSessao
	}
	if _, err := tx.Exec(ctx,
		`update sessoes set substituida_em = $2, usada_em = $2 where id = $1::uuid`, id, agora); err != nil {
		return Par{}, err
	}
	p, err := r.inserir(ctx, tx, usuarioID, familia, dispositivo, agora)
	if err != nil {
		return Par{}, err
	}
	return p, tx.Commit(ctx)
}

// Sair revoga a família do token (logout neste dispositivo). Token desconhecido não é erro.
func (r *Repo) Sair(ctx context.Context, renovacao string) error {
	h := hashToken(renovacao)
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var usuarioID, familia string
	err = tx.QueryRow(ctx, `select usuario_id::text, familia::text from sessoes where hash_renovacao = $1`, h).
		Scan(&usuarioID, &familia)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := trava(ctx, tx, usuarioID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`update sessoes set revogada_em = $2 where familia = $1::uuid and revogada_em is null`,
		familia, r.agora()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RevogarTodas derruba todas as sessões do usuário (troca de senha, "sair de todos").
func (r *Repo) RevogarTodas(ctx context.Context, usuarioID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := trava(ctx, tx, usuarioID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`update sessoes set revogada_em = $2 where usuario_id = $1::uuid and revogada_em is null`,
		usuarioID, r.agora()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ConferirAcesso valida o JWT de acesso e devolve o usuário. Só aceita EdDSA com a nossa chave.
func (r *Repo) ConferirAcesso(token string) (string, error) {
	var c jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &c,
		func(*jwt.Token) (any, error) { return r.pub, nil },
		jwt.WithValidMethods([]string{"EdDSA"}),
		jwt.WithIssuer(emissor),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(r.agora),
	)
	if err != nil {
		return "", ErrSessao
	}
	if c.Subject == "" {
		return "", ErrSessao
	}
	return c.Subject, nil
}

// Middleware exige "Authorization: Bearer <acesso>" e passa o usuário para next.
func Middleware(r *Repo, next func(w http.ResponseWriter, req *http.Request, usuarioID string)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		tok, ok := strings.CutPrefix(req.Header.Get("Authorization"), "Bearer ")
		if ok && tok != "" {
			if id, err := r.ConferirAcesso(tok); err == nil {
				next(w, req, id)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"erro": "sessão inválida"})
	})
}
