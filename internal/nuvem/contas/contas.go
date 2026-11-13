// Package contas guarda os usuários da nuvem (tabela usuarios). Os emails chegam aqui já normalizados.
package contas

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// MaxFalhas senhas erradas seguidas bloqueiam o login por Bloqueio.
	MaxFalhas = 10
	Bloqueio  = 15 * time.Minute
)

// ErrNaoEncontrado: não há usuário com esse email, id ou conta Google.
var ErrNaoEncontrado = errors.New("usuário não encontrado")

// Usuario é uma linha de usuarios (só o que as rotas usam).
type Usuario struct {
	ID                string
	Email             string
	Nome              string
	SenhaHash         string // vazio: conta só com Google
	GoogleSub         string
	EmailConfirmadoEm *time.Time
	TermosVersao      string
	TermosAceitosEm   *time.Time
	FalhasLogin       int
	BloqueadoAte      *time.Time
	TesteInicio       *time.Time
	TesteFim          *time.Time
	Cortesia          bool
}

// Confirmado diz se o email já foi confirmado.
func (u Usuario) Confirmado() bool { return u.EmailConfirmadoEm != nil }

// Bloqueado diz se o login está bloqueado por senhas erradas.
func (u Usuario) Bloqueado(agora time.Time) bool {
	return u.BloqueadoAte != nil && agora.Before(*u.BloqueadoAte)
}

// Situacao é o resultado de Criar.
type Situacao int

const (
	Nova         Situacao = iota // usuário criado agora
	Pendente                     // já existia sem confirmar: senha e termos trocados
	JaConfirmada                 // já existia confirmado: nada mudou
)

const colunas = `id::text, email, nome, coalesce(senha_hash, ''), coalesce(google_sub, ''), email_confirmado_em,
	termos_versao, termos_aceitos_em, falhas_login, bloqueado_ate,
	teste_inicio, teste_fim, cortesia`

// Repo lê e grava usuarios.
type Repo struct{ db *pgxpool.Pool }

func Novo(db *pgxpool.Pool) *Repo { return &Repo{db: db} }

func ler(row pgx.Row, extra ...any) (Usuario, error) {
	var u Usuario
	dest := append([]any{&u.ID, &u.Email, &u.Nome, &u.SenhaHash, &u.GoogleSub, &u.EmailConfirmadoEm,
		&u.TermosVersao, &u.TermosAceitosEm, &u.FalhasLogin, &u.BloqueadoAte, &u.TesteInicio, &u.TesteFim, &u.Cortesia}, extra...)
	err := row.Scan(dest...)
	if errors.Is(err, pgx.ErrNoRows) {
		return Usuario{}, ErrNaoEncontrado
	}
	return u, err
}

func (r *Repo) PorEmail(ctx context.Context, email string) (Usuario, error) {
	return ler(r.db.QueryRow(ctx, `select `+colunas+` from usuarios where email = $1`, email))
}

func (r *Repo) PorID(ctx context.Context, id string) (Usuario, error) {
	return ler(r.db.QueryRow(ctx, `select `+colunas+` from usuarios where id = $1::uuid`, id))
}

func (r *Repo) PorGoogle(ctx context.Context, sub string) (Usuario, error) {
	return ler(r.db.QueryRow(ctx, `select `+colunas+` from usuarios where google_sub = $1`, sub))
}

// Criar cria o usuário sem confirmar. Se o email já existe sem confirmar, troca a senha pendente e a versão
// dos termos (numa só instrução, sem corrida); se já existe confirmado, não muda nada.
func (r *Repo) Criar(ctx context.Context, email, senhaHash, termosVersao string) (Usuario, Situacao, error) {
	var nova bool
	u, err := ler(r.db.QueryRow(ctx,
		`insert into usuarios (email, senha_hash, termos_versao) values ($1, $2, $3)
		 on conflict (email) do update set senha_hash = excluded.senha_hash, termos_versao = excluded.termos_versao
		 where usuarios.email_confirmado_em is null
		 returning `+colunas+`, (xmax = 0)`, email, senhaHash, termosVersao), &nova)
	if errors.Is(err, ErrNaoEncontrado) { // conflito com conta confirmada: o update não rodou
		u, err = r.PorEmail(ctx, email)
		return u, JaConfirmada, err
	}
	if err != nil {
		return Usuario{}, 0, err
	}
	if nova {
		return u, Nova, nil
	}
	return u, Pendente, nil
}

// Confirmar marca o email como confirmado (mantém a data da primeira vez) e grava o aceite dos termos,
// só se a senha ainda for a que acabou de ser conferida (senhaHash). Se um recadastro trocou a senha
// pendente no meio, nada muda e volta ErrNaoEncontrado.
func (r *Repo) Confirmar(ctx context.Context, id, senhaHash string, agora time.Time) error {
	tag, err := r.db.Exec(ctx,
		`update usuarios set email_confirmado_em = coalesce(email_confirmado_em, $2), termos_aceitos_em = $2
		 where id = $1::uuid and senha_hash = $3`, id, agora, senhaHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNaoEncontrado
	}
	return nil
}

// ErrConflito: outra requisição criou ou ligou a mesma conta Google ao mesmo tempo; leia de novo.
var ErrConflito = errors.New("conta alterada ao mesmo tempo")

func conflito(err error) error {
	var pg *pgconn.PgError
	if errors.Is(err, ErrNaoEncontrado) || (errors.As(err, &pg) && pg.Code == "23505") {
		return ErrConflito
	}
	return err
}

// CriarGoogle cria a conta já confirmada (o Google verificou o email), sem senha, com o aceite dos termos.
// Se o email ou o sub já existem (corrida), volta ErrConflito.
func (r *Repo) CriarGoogle(ctx context.Context, email, sub, nome, termosVersao string, agora time.Time) (Usuario, error) {
	u, err := ler(r.db.QueryRow(ctx,
		`insert into usuarios (email, nome, google_sub, email_confirmado_em, termos_versao, termos_aceitos_em)
		 values ($1, $2, $3, $4, $5, $4) on conflict do nothing returning `+colunas,
		email, nome, sub, agora, termosVersao))
	if err != nil {
		return Usuario{}, conflito(err)
	}
	return u, nil
}

// LigarGoogle liga o sub a uma conta que ainda não tem Google. Conta pendente (email nunca confirmado)
// fica confirmada, com o aceite de termosVersao, e perde a senha pendente: quem cadastrou pode não ser
// o dono do email, e o Google acabou de provar quem é. Conta confirmada mantém a senha e os termos.
// Se a conta ganhou um sub no meio (ou o sub já está em outra), volta ErrConflito.
// Sem autoritativo (R13), conta confirmada com senha não é ligada (volta ErrConflito; quem chama relê).
func (r *Repo) LigarGoogle(ctx context.Context, id, sub, nome, termosVersao string, autoritativo bool, agora time.Time) (Usuario, error) {
	// No update, as expressões leem a linha de antes: o "pendente" é o de antes da confirmação.
	u, err := ler(r.db.QueryRow(ctx,
		`update usuarios set google_sub = $2,
		   nome                = case when nome = '' then $3 else nome end,
		   senha_hash          = case when email_confirmado_em is null then null else senha_hash end,
		   termos_versao       = case when email_confirmado_em is null then $5 else termos_versao end,
		   termos_aceitos_em   = case when email_confirmado_em is null then $4 else termos_aceitos_em end,
		   email_confirmado_em = coalesce(email_confirmado_em, $4)
		 where id = $1::uuid and google_sub is null
		   and ($6 or email_confirmado_em is null or senha_hash is null)
		 returning `+colunas, id, sub, nome, agora, termosVersao, autoritativo))
	if err != nil {
		return Usuario{}, conflito(err)
	}
	return u, nil
}

// AceitarTermos grava a versão dos termos aceita agora.
func (r *Repo) AceitarTermos(ctx context.Context, id, versao string, agora time.Time) error {
	_, err := r.db.Exec(ctx,
		`update usuarios set termos_versao = $2, termos_aceitos_em = $3 where id = $1::uuid`, id, versao, agora)
	return err
}

// TrocarSenha grava a senha nova depois de um código de recuperação: o código prova o email, então ele
// também fica confirmado; zera as falhas e o bloqueio. Email fora de @gmail.com/@googlemail.com perde o
// Google ligado: num domínio próprio, a conta Google pode ser de um dono antigo do email, e quem acabou de
// provar o email é o dono de agora.
func (r *Repo) TrocarSenha(ctx context.Context, id, senhaHash string, agora time.Time) error {
	_, err := r.db.Exec(ctx,
		`update usuarios set senha_hash = $2, falhas_login = 0, bloqueado_ate = null,
		 email_confirmado_em = coalesce(email_confirmado_em, $3),
		 google_sub = case when split_part(email, '@', 2) in ('gmail.com', 'googlemail.com') then google_sub end
		 where id = $1::uuid`, id, senhaHash, agora)
	return err
}

// Reservar reserva uma tentativa de senha antes de conferi-la, numa só instrução: falhas_login conta as
// tentativas em andamento e as erradas. Devolve false (sem conferir a senha) se o login está bloqueado ou
// se já há MaxFalhas tentativas contadas; nesse caso o bloqueio começa agora (assim um contador que ficou
// cheio por um pedido interrompido não prende a conta para sempre). Tentativas simultâneas não passam
// de MaxFalhas senhas conferidas.
func (r *Repo) Reservar(ctx context.Context, id string, agora time.Time) (bool, error) {
	var livre bool
	err := r.db.QueryRow(ctx,
		`update usuarios set
		   falhas_login  = case when bloqueado_ate > $2 then falhas_login
		                        when falhas_login >= $3 then 0 else falhas_login + 1 end,
		   bloqueado_ate = case when bloqueado_ate > $2 then bloqueado_ate
		                        when falhas_login >= $3 then $4::timestamptz else bloqueado_ate end
		 where id = $1::uuid
		 returning not coalesce(bloqueado_ate > $2, false)`,
		id, agora, MaxFalhas, agora.Add(Bloqueio)).Scan(&livre)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNaoEncontrado
	}
	return livre, err
}

// RegistrarFalha fecha uma tentativa reservada que errou a senha (a reserva já a contou).
// Quando o contador chega a MaxFalhas, bloqueia até agora+Bloqueio, zera o contador e devolve true.
func (r *Repo) RegistrarFalha(ctx context.Context, id string, agora time.Time) (bool, error) {
	var bloqueou bool
	err := r.db.QueryRow(ctx,
		`update usuarios set
		   falhas_login  = case when falhas_login >= $3 then 0 else falhas_login end,
		   bloqueado_ate = case when falhas_login >= $3 then $2::timestamptz else bloqueado_ate end
		 where id = $1::uuid
		 returning bloqueado_ate is not null and bloqueado_ate = $2::timestamptz`,
		id, agora.Add(Bloqueio), MaxFalhas).Scan(&bloqueou)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNaoEncontrado
	}
	return bloqueou, err
}

// ZerarFalhas limpa o contador e o bloqueio (login certo).
func (r *Repo) ZerarFalhas(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx,
		`update usuarios set falhas_login = 0, bloqueado_ate = null
		 where id = $1::uuid and (falhas_login <> 0 or bloqueado_ate is not null)`, id)
	return err
}

// StripeCliente devolve o cliente Stripe do usuário ("" se ainda não tem).
func (r *Repo) StripeCliente(ctx context.Context, id string) (string, error) {
	var c string
	err := r.db.QueryRow(ctx, `select coalesce(stripe_cliente, '') from usuarios where id = $1::uuid`, id).Scan(&c)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNaoEncontrado
	}
	return c, err
}

// GravarStripeCliente grava o cliente se o usuário ainda não tem um e devolve o que ficou gravado
// (numa corrida, o primeiro vence).
func (r *Repo) GravarStripeCliente(ctx context.Context, id, cliente string) (string, error) {
	if _, err := r.db.Exec(ctx, `update usuarios set stripe_cliente = $2 where id = $1::uuid and stripe_cliente is null`, id, cliente); err != nil {
		return "", err
	}
	return r.StripeCliente(ctx, id)
}

// PorStripeCliente devolve o id do usuário dono do cliente Stripe.
func (r *Repo) PorStripeCliente(ctx context.Context, cliente string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `select id::text from usuarios where stripe_cliente = $1`, cliente).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNaoEncontrado
	}
	return id, err
}
