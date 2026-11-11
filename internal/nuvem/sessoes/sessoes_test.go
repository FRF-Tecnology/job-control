package sessoes_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FRF-Tecnology/job-control/internal/nuvem/pgteste"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/sessoes"
)

func TestMain(m *testing.M) { os.Exit(pgteste.Principal(m)) }

type relogio struct {
	mu sync.Mutex
	t  time.Time
}

func (r *relogio) agora() time.Time { r.mu.Lock(); defer r.mu.Unlock(); return r.t }
func (r *relogio) avancar(d time.Duration) {
	r.mu.Lock()
	r.t = r.t.Add(d)
	r.mu.Unlock()
}

type ambiente struct {
	repo *sessoes.Repo
	rel  *relogio
	db   *pgxpool.Pool
	priv ed25519.PrivateKey
}

func novo(t *testing.T) *ambiente {
	t.Helper()
	db := pgteste.Banco(t)
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	rel := &relogio{t: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)}
	return &ambiente{repo: sessoes.Novo(db, priv, rel.agora, rand.Reader), rel: rel, db: db, priv: priv}
}

// usuario cria um usuário fictício e devolve o id.
func (a *ambiente) usuario(t *testing.T, email string) string {
	t.Helper()
	var id string
	if err := a.db.QueryRow(context.Background(),
		`insert into usuarios (email) values ($1) returning id::text`, email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestAbrirERenovar(t *testing.T) {
	a := novo(t)
	ctx := context.Background()
	ana := a.usuario(t, "ana@exemplo.com")
	p, err := a.repo.Abrir(ctx, ana, "disp-1")
	if err != nil {
		t.Fatal(err)
	}
	if p.Acesso == "" || p.Renovacao == "" {
		t.Fatalf("par vazio: %+v", p)
	}
	if !p.AcessoAte.Equal(a.rel.agora().Add(15 * time.Minute)) {
		t.Errorf("acesso_ate = %v", p.AcessoAte)
	}
	if strings.ContainsAny(p.Renovacao, "=+/") || len(p.Renovacao) != 43 {
		t.Errorf("renovação deveria ser base64url sem padding de 32 bytes: %q", p.Renovacao)
	}
	if id, err := a.repo.ConferirAcesso(p.Acesso); err != nil || id != ana {
		t.Fatalf("ConferirAcesso = %q, %v", id, err)
	}

	novo, err := a.repo.Renovar(ctx, p.Renovacao)
	if err != nil {
		t.Fatal(err)
	}
	if novo.Renovacao == p.Renovacao {
		t.Fatal("renovação deveria trocar o token")
	}
	if id, err := a.repo.ConferirAcesso(novo.Acesso); err != nil || id != ana {
		t.Fatalf("acesso novo = %q, %v", id, err)
	}
	if _, err := a.repo.Renovar(ctx, p.Renovacao); err == nil {
		t.Fatal("o token antigo não deveria valer mais")
	}
}

func TestDispositivoGuardado(t *testing.T) {
	a := novo(t)
	ctx := context.Background()
	ana := a.usuario(t, "ana@exemplo.com")
	if _, err := a.repo.Abrir(ctx, ana, "hash-do-pc"); err != nil {
		t.Fatal(err)
	}
	var d string
	if err := a.db.QueryRow(ctx, `select dispositivo_hash from sessoes where usuario_id = $1::uuid`, ana).Scan(&d); err != nil {
		t.Fatal(err)
	}
	if d != "hash-do-pc" {
		t.Fatalf("dispositivo_hash = %q", d)
	}
}

func TestReusoRevogaFamilia(t *testing.T) {
	a := novo(t)
	ctx := context.Background()
	ana := a.usuario(t, "ana@exemplo.com")
	velho, _ := a.repo.Abrir(ctx, ana, "")
	novo, err := a.repo.Renovar(ctx, velho.Renovacao)
	if err != nil {
		t.Fatal(err)
	}
	// Outra sessão da mesma pessoa (outra família) não deve cair junto.
	outra, _ := a.repo.Abrir(ctx, ana, "")

	if _, err := a.repo.Renovar(ctx, velho.Renovacao); !errors.Is(err, sessoes.ErrReuso) {
		t.Fatalf("reuso: %v", err)
	}
	if _, err := a.repo.Renovar(ctx, novo.Renovacao); !errors.Is(err, sessoes.ErrSessao) {
		t.Fatalf("o par novo deveria cair junto: %v", err)
	}
	if _, err := a.repo.Renovar(ctx, outra.Renovacao); err != nil {
		t.Fatalf("outra família não deveria cair: %v", err)
	}
}

func TestRenovarConcorrente(t *testing.T) {
	a := novo(t)
	ctx := context.Background()
	ana := a.usuario(t, "ana@exemplo.com")
	p, _ := a.repo.Abrir(ctx, ana, "")

	const n = 20
	var wg sync.WaitGroup
	type resultado struct {
		par sessoes.Par
		err error
	}
	res := make(chan resultado, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			par, err := a.repo.Renovar(ctx, p.Renovacao)
			res <- resultado{par, err}
		}()
	}
	wg.Wait()
	close(res)
	var vencedor sessoes.Par
	ok := 0
	for r := range res {
		switch {
		case r.err == nil:
			ok++
			vencedor = r.par
		case errors.Is(r.err, sessoes.ErrReuso), errors.Is(r.err, sessoes.ErrSessao):
		default:
			t.Errorf("erro inesperado: %v", r.err)
		}
	}
	if ok != 1 {
		t.Fatalf("%d renovações passaram, queria 1", ok)
	}
	// Estado final, sempre o mesmo: a 1ª transação troca o token; as outras 19 veem o
	// token já trocado (reuso) e revogam a família. Logo o par vencedor também não
	// renova mais, e nenhuma linha da família fica ativa.
	if _, err := a.repo.Renovar(ctx, vencedor.Renovacao); !errors.Is(err, sessoes.ErrSessao) {
		t.Fatalf("o par vencedor deveria estar revogado: %v", err)
	}
	var ativas int
	if err := a.db.QueryRow(ctx,
		`select count(*) from sessoes where usuario_id = $1::uuid and revogada_em is null`, ana).Scan(&ativas); err != nil {
		t.Fatal(err)
	}
	if ativas != 0 {
		t.Fatalf("%d sessões ativas, queria 0", ativas)
	}
}

func TestSessaoParadaExpira(t *testing.T) {
	a := novo(t)
	ctx := context.Background()
	ana := a.usuario(t, "ana@exemplo.com")
	p, _ := a.repo.Abrir(ctx, ana, "")
	a.rel.avancar(59 * 24 * time.Hour)
	p, err := a.repo.Renovar(ctx, p.Renovacao)
	if err != nil {
		t.Fatalf("usada antes de 60 dias deveria renovar: %v", err)
	}
	a.rel.avancar(60*24*time.Hour + time.Second)
	if _, err := a.repo.Renovar(ctx, p.Renovacao); !errors.Is(err, sessoes.ErrSessao) {
		t.Fatalf("parada 60 dias + 1 s: %v", err)
	}
}

func TestRenovarTokenDesconhecido(t *testing.T) {
	a := novo(t)
	for _, tok := range []string{"", "abc", strings.Repeat("A", 43)} {
		if _, err := a.repo.Renovar(context.Background(), tok); !errors.Is(err, sessoes.ErrSessao) {
			t.Errorf("%q: %v", tok, err)
		}
	}
}

func TestAcessoExpira(t *testing.T) {
	a := novo(t)
	ana := a.usuario(t, "ana@exemplo.com")
	p, _ := a.repo.Abrir(context.Background(), ana, "")
	a.rel.avancar(14*time.Minute + 59*time.Second)
	if _, err := a.repo.ConferirAcesso(p.Acesso); err != nil {
		t.Fatalf("ainda deveria valer: %v", err)
	}
	a.rel.avancar(2 * time.Second)
	if _, err := a.repo.ConferirAcesso(p.Acesso); err == nil {
		t.Fatal("15m1s depois deveria falhar")
	}
}

func claims(sub string, agora time.Time) jwt.RegisteredClaims {
	return jwt.RegisteredClaims{
		Issuer:    "applyer-nuvem",
		Subject:   sub,
		IssuedAt:  jwt.NewNumericDate(agora),
		ExpiresAt: jwt.NewNumericDate(agora.Add(15 * time.Minute)),
	}
}

func TestAcessoRecusaAlgNone(t *testing.T) {
	a := novo(t)
	tok, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims("x", a.rel.agora())).
		SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.repo.ConferirAcesso(tok); err == nil {
		t.Fatal("alg none aceito")
	}
	// Confusão de algoritmo: HS256 com a chave pública como segredo.
	pub := a.priv.Public().(ed25519.PublicKey)
	tok, err = jwt.NewWithClaims(jwt.SigningMethodHS256, claims("x", a.rel.agora())).SignedString([]byte(pub))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.repo.ConferirAcesso(tok); err == nil {
		t.Fatal("HS256 aceito")
	}
}

func TestAcessoRecusaOutraChaveEOutroEmissor(t *testing.T) {
	a := novo(t)
	_, outra, _ := ed25519.GenerateKey(rand.Reader)
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims("x", a.rel.agora())).SignedString(outra)
	if _, err := a.repo.ConferirAcesso(tok); err == nil {
		t.Fatal("chave alheia aceita")
	}
	c := claims("x", a.rel.agora())
	c.Issuer = "outro"
	tok, _ = jwt.NewWithClaims(jwt.SigningMethodEdDSA, c).SignedString(a.priv)
	if _, err := a.repo.ConferirAcesso(tok); err == nil {
		t.Fatal("outro emissor aceito")
	}
	c = claims("", a.rel.agora())
	tok, _ = jwt.NewWithClaims(jwt.SigningMethodEdDSA, c).SignedString(a.priv)
	if _, err := a.repo.ConferirAcesso(tok); err == nil {
		t.Fatal("sem sub aceito")
	}
	c = claims("x", a.rel.agora())
	c.ExpiresAt = nil
	tok, _ = jwt.NewWithClaims(jwt.SigningMethodEdDSA, c).SignedString(a.priv)
	if _, err := a.repo.ConferirAcesso(tok); err == nil {
		t.Fatal("sem exp aceito")
	}
}

func TestSair(t *testing.T) {
	a := novo(t)
	ctx := context.Background()
	ana := a.usuario(t, "ana@exemplo.com")
	p, _ := a.repo.Abrir(ctx, ana, "")
	if err := a.repo.Sair(ctx, p.Renovacao); err != nil {
		t.Fatal(err)
	}
	if _, err := a.repo.Renovar(ctx, p.Renovacao); !errors.Is(err, sessoes.ErrSessao) {
		t.Fatalf("depois de sair: %v", err)
	}
}

func TestRevogarTodas(t *testing.T) {
	a := novo(t)
	ctx := context.Background()
	ana := a.usuario(t, "ana@exemplo.com")
	bia := a.usuario(t, "bia@exemplo.com")
	p1, _ := a.repo.Abrir(ctx, ana, "pc")
	p2, _ := a.repo.Abrir(ctx, ana, "notebook")
	pb, _ := a.repo.Abrir(ctx, bia, "")
	if err := a.repo.RevogarTodas(ctx, ana); err != nil {
		t.Fatal(err)
	}
	for _, p := range []sessoes.Par{p1, p2} {
		if _, err := a.repo.Renovar(ctx, p.Renovacao); !errors.Is(err, sessoes.ErrSessao) {
			t.Errorf("sessão da ana ainda vale: %v", err)
		}
	}
	if _, err := a.repo.Renovar(ctx, pb.Renovacao); err != nil {
		t.Fatalf("sessão da bia caiu: %v", err)
	}
}

func TestSoHashNoBanco(t *testing.T) {
	a := novo(t)
	ctx := context.Background()
	ana := a.usuario(t, "ana@exemplo.com")
	p, _ := a.repo.Abrir(ctx, ana, "")
	p2, _ := a.repo.Renovar(ctx, p.Renovacao)
	rows, err := a.db.Query(ctx, `select s::text from sessoes s`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var linha string
		if err := rows.Scan(&linha); err != nil {
			t.Fatal(err)
		}
		n++
		for _, tok := range []string{p.Renovacao, p2.Renovacao, p.Acesso, p2.Acesso} {
			if strings.Contains(linha, tok) {
				t.Fatalf("token aparece na linha: %s", linha)
			}
		}
	}
	if n != 2 {
		t.Fatalf("%d linhas, queria 2", n)
	}
}

func TestMiddleware(t *testing.T) {
	a := novo(t)
	ana := a.usuario(t, "ana@exemplo.com")
	p, _ := a.repo.Abrir(context.Background(), ana, "")
	h := sessoes.Middleware(a.repo, func(w http.ResponseWriter, _ *http.Request, usuarioID string) {
		_, _ = w.Write([]byte(usuarioID))
	})

	casos := map[string]string{
		"sem cabeçalho": "",
		"sem Bearer":    p.Acesso,
		"Basic":         "Basic " + p.Acesso,
		"token ruim":    "Bearer xyz",
		"Bearer vazio":  "Bearer ",
	}
	for nome, cab := range casos {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if cab != "" {
			req.Header.Set("Authorization", cab)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: código %d", nome, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("%s: content-type %q", nome, ct)
		}
		if strings.TrimSpace(rec.Body.String()) != `{"erro":"sessão inválida"}` {
			t.Errorf("%s: corpo %q", nome, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+p.Acesso)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != ana {
		t.Fatalf("válido: %d %q", rec.Code, rec.Body.String())
	}
}
