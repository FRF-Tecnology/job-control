//go:build contafalsa

package conta

import (
	"context"
	"crypto/ed25519"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/licenca"
)

// CodigoFalso é o código de confirmação e de recuperação da nuvem falsa.
const CodigoFalso = "123456"

type contaFalsa struct {
	email       string
	senha       string // vazia em conta criada só pelo Google
	confirmada  bool
	usuario     string
	testeFim    *time.Time
	assinante   bool
	cortesia    bool
	recuperando bool
}

// Falsa é uma nuvem em memória que segue as regras públicas da real. Segura para uso concorrente.
type Falsa struct {
	mu      sync.Mutex
	priv    ed25519.PrivateKey
	agora   func() time.Time
	desvio  time.Duration
	contas  map[string]*contaFalsa
	testes  map[string]string // dispositivo -> email que usou o teste
	sessoes map[string]string // token (acesso ou renovação) -> email
	semNet  bool
	seq     int
}

// NovaFalsa cria a nuvem falsa. agora nil usa time.Now.
func NovaFalsa(priv ed25519.PrivateKey, agora func() time.Time) *Falsa {
	if agora == nil {
		agora = time.Now
	}
	return &Falsa{priv: priv, agora: agora, contas: map[string]*contaFalsa{},
		testes: map[string]string{}, sessoes: map[string]string{}}
}

func (f *Falsa) agoraL() time.Time { return f.agora().Add(f.desvio) }

// Avancar adianta o relógio da nuvem falsa.
func (f *Falsa) Avancar(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.desvio += d
}

// Cortesia dá à conta acesso de assinante sem cobrança.
func (f *Falsa) Cortesia(email string) { f.marcar(email, func(c *contaFalsa) { c.cortesia = true }) }

// Assinante põe a conta como assinante (o Falsa.Assinar é o da interface Nuvem, que devolve a URL).
func (f *Falsa) Assinante(email string) { f.marcar(email, func(c *contaFalsa) { c.assinante = true }) }

func (f *Falsa) marcar(email string, fn func(*contaFalsa)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c := f.contas[norm(email)]; c != nil {
		fn(c)
	}
}

// SemInternet liga ou desliga a falha de rede em todas as chamadas.
func (f *Falsa) SemInternet(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.semNet = v
}

func norm(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

func erro(status int, msg, motivo string) error {
	return &ErroNuvem{Status: status, Msg: msg, Motivo: motivo}
}

func (f *Falsa) novoPar(email string) Par {
	f.seq++
	n := strconv.Itoa(f.seq)
	p := Par{Acesso: "acesso-" + n, Renovacao: "renovacao-" + n, AcessoAte: f.agoraL().Add(15 * time.Minute)}
	f.sessoes[p.Acesso] = email
	f.sessoes[p.Renovacao] = email
	return p
}

func (f *Falsa) novoUsuario() string {
	f.seq++
	return "usuario-" + strconv.Itoa(f.seq)
}

func (f *Falsa) entrada(ctx context.Context) error {
	if f.semNet {
		return ErrSemInternet
	}
	return ctx.Err()
}

func (f *Falsa) Criar(ctx context.Context, email, senha, termos string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.entrada(ctx); err != nil {
		return err
	}
	e := norm(email)
	if !strings.Contains(e, "@") {
		return erro(400, "Email inválido", "")
	}
	if len(senha) < 10 || len(senha) > 128 {
		return erro(400, "A senha precisa ter de 10 a 128 caracteres", "")
	}
	if termos == "" {
		return erro(400, "Aceite os termos para continuar", "")
	}
	if c := f.contas[e]; c != nil && c.confirmada {
		return nil // a real manda um aviso em vez de código; a resposta é a mesma
	}
	f.contas[e] = &contaFalsa{email: e, senha: senha, usuario: f.novoUsuario()}
	return nil
}

func (f *Falsa) Confirmar(ctx context.Context, email, codigo, senha, dispositivo, termos string) (Par, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.entrada(ctx); err != nil {
		return Par{}, err
	}
	c := f.contas[norm(email)]
	if c == nil || c.confirmada || codigo != CodigoFalso {
		return Par{}, erro(400, "Código inválido ou vencido", "")
	}
	if dispositivo == "" {
		return Par{}, erro(400, "Dispositivo inválido", "")
	}
	if senha != c.senha {
		return Par{}, erro(401, "Email ou senha incorretos", "")
	}
	c.confirmada = true
	return f.novoPar(c.email), nil
}

func (f *Falsa) Entrar(ctx context.Context, email, senha, dispositivo, termos string) (Par, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.entrada(ctx); err != nil {
		return Par{}, err
	}
	if dispositivo == "" {
		return Par{}, erro(400, "Dispositivo inválido", "")
	}
	c := f.contas[norm(email)]
	if c == nil || c.senha == "" || c.senha != senha {
		return Par{}, erro(401, "Email ou senha incorretos", "")
	}
	if !c.confirmada {
		return Par{}, erro(403, "Confirme seu email com o código que enviamos", "confirmar")
	}
	return f.novoPar(c.email), nil
}

func (f *Falsa) Recuperar(ctx context.Context, email string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.entrada(ctx); err != nil {
		return err
	}
	if c := f.contas[norm(email)]; c != nil {
		c.recuperando = true
	}
	return nil
}

func (f *Falsa) NovaSenha(ctx context.Context, email, codigo, senha, dispositivo string) (Par, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.entrada(ctx); err != nil {
		return Par{}, err
	}
	if len(senha) < 10 || len(senha) > 128 {
		return Par{}, erro(400, "A senha precisa ter de 10 a 128 caracteres", "")
	}
	c := f.contas[norm(email)]
	if c == nil || !c.recuperando || codigo != CodigoFalso || dispositivo == "" {
		return Par{}, erro(400, "Código inválido ou vencido", "")
	}
	c.senha, c.recuperando, c.confirmada = senha, false, true
	f.revogar(c.email) // troca de senha revoga as sessões antigas
	return f.novoPar(c.email), nil
}

func (f *Falsa) revogar(email string) {
	for t, e := range f.sessoes {
		if e == email {
			delete(f.sessoes, t)
		}
	}
}

// Google na falsa: o code é o email da conta Google (ex.: "ana@exemplo.com"); cria e confirma a conta.
func (f *Falsa) Google(ctx context.Context, code, verifier, redirectURI, dispositivo, termos string) (Par, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.entrada(ctx); err != nil {
		return Par{}, err
	}
	e := norm(code)
	if !strings.Contains(e, "@") || verifier == "" || dispositivo == "" {
		return Par{}, erro(401, "O Google recusou o acesso", "")
	}
	c := f.contas[e]
	if c != nil && c.confirmada && c.senha != "" {
		return Par{}, erro(409, "Esta conta entra com senha", "entrar-com-senha")
	}
	if c == nil || !c.confirmada {
		if termos == "" {
			return Par{}, erro(409, "Os termos mudaram", "termos")
		}
		c = &contaFalsa{email: e, usuario: f.novoUsuario()}
		f.contas[e] = c
	}
	c.confirmada = true
	return f.novoPar(e), nil
}

func (f *Falsa) Renovar(ctx context.Context, renovacao string) (Par, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.entrada(ctx); err != nil {
		return Par{}, err
	}
	e, ok := f.sessoes[renovacao]
	if !ok || !strings.HasPrefix(renovacao, "renovacao-") {
		return Par{}, erro(401, "sessão inválida", "")
	}
	delete(f.sessoes, renovacao) // uso único
	return f.novoPar(e), nil
}

func (f *Falsa) Sair(ctx context.Context, renovacao string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.entrada(ctx); err != nil {
		return err
	}
	delete(f.sessoes, renovacao)
	return nil
}

func (f *Falsa) conta(acesso string) (*contaFalsa, error) {
	e, ok := f.sessoes[acesso]
	if !ok || !strings.HasPrefix(acesso, "acesso-") {
		return nil, erro(401, "sessão inválida", "")
	}
	c := f.contas[e]
	if c == nil {
		return nil, erro(401, "sessão inválida", "")
	}
	return c, nil
}

func (f *Falsa) Licenca(ctx context.Context, acesso, dispositivo string) (licenca.Documento, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.entrada(ctx); err != nil {
		return licenca.Documento{}, err
	}
	c, err := f.conta(acesso)
	if err != nil {
		return licenca.Documento{}, err
	}
	if dispositivo == "" {
		return licenca.Documento{}, erro(400, "Dispositivo inválido", "")
	}
	agora := f.agoraL()
	if c.testeFim == nil && c.confirmada {
		if _, usado := f.testes[dispositivo]; !usado {
			f.testes[dispositivo] = c.email
			fim := agora.Add(24 * time.Hour)
			c.testeFim = &fim
		}
	}
	estado := licenca.Bloqueado
	switch {
	case c.cortesia || c.assinante:
		estado = licenca.Assinante
	case c.testeFim != nil && agora.Before(*c.testeFim):
		estado = licenca.Teste
	}
	l := licenca.Licenca{Usuario: c.usuario, Email: c.email, Estado: estado, TesteFim: c.testeFim,
		EmitidaEm: agora, ValidaAte: agora.Add(30 * time.Minute)}
	return licenca.Assinar(f.priv, l)
}

func (f *Falsa) Assinar(ctx context.Context, acesso string) (string, error) {
	return f.stripe(ctx, acesso, "checkout")
}

func (f *Falsa) Portal(ctx context.Context, acesso string) (string, error) {
	return f.stripe(ctx, acesso, "portal")
}

func (f *Falsa) stripe(ctx context.Context, acesso, tipo string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.entrada(ctx); err != nil {
		return "", err
	}
	if _, err := f.conta(acesso); err != nil {
		return "", err
	}
	return "https://stripe.exemplo.com/" + tipo, nil
}

func (f *Falsa) Notas(ctx context.Context, acesso string) ([]Nota, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.entrada(ctx); err != nil {
		return nil, err
	}
	if _, err := f.conta(acesso); err != nil {
		return nil, err
	}
	return []Nota{}, nil
}

func (f *Falsa) Excluir(ctx context.Context, acesso, email string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.entrada(ctx); err != nil {
		return err
	}
	c, err := f.conta(acesso)
	if err != nil {
		return err
	}
	if norm(email) != c.email {
		return erro(400, "Confirme digitando o email da conta", "")
	}
	delete(f.contas, c.email)
	f.revogar(c.email)
	return nil
}

var _ Nuvem = (*Falsa)(nil)
