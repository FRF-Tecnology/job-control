//go:build contafalsa

package conta

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// capturaGoogle guarda o que o app manda à nuvem na troca do código.
type capturaGoogle struct {
	*Falsa
	mu                          sync.Mutex
	chamadas                    int
	code, verifier, redirectURI string
}

func (c *capturaGoogle) Google(ctx context.Context, code, verifier, redirectURI, disp, termos string) (Par, error) {
	c.mu.Lock()
	c.chamadas++
	c.code, c.verifier, c.redirectURI = code, verifier, redirectURI
	c.mu.Unlock()
	return c.Falsa.Google(ctx, code, verifier, redirectURI, disp, termos)
}

func (c *capturaGoogle) vistos() (int, string, string, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.chamadas, c.code, c.verifier, c.redirectURI
}

func comClientID(t *testing.T) {
	t.Helper()
	velho := GoogleClientID
	GoogleClientID = "cliente-de-teste.apps.exemplo.com"
	t.Cleanup(func() { GoogleClientID = velho })
}

// iniciarGoogle começa o login e devolve a URL que o app abriria no navegador.
func iniciarGoogle(t *testing.T, c *Conta) *url.URL {
	t.Helper()
	comClientID(t)
	var aberta string
	err := c.IniciarGoogle(context.Background(), func(u string) error { aberta = u; return nil }, TermosPadrao)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.CancelarGoogle)
	u, err := url.Parse(aberta)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// enderecoOuvinte é o host:porta do redirect_uri.
func enderecoOuvinte(u *url.URL) string {
	r, _ := url.Parse(u.Query().Get("redirect_uri"))
	return r.Host
}

func retorno(t *testing.T, u *url.URL, consulta url.Values) (int, string, http.Header) {
	t.Helper()
	resp, err := http.Get(u.Query().Get("redirect_uri") + "?" + consulta.Encode())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b), resp.Header
}

func TestGoogleURLTemPKCEEState(t *testing.T) {
	a := novoAmbiente(t, 0)
	c := a.conta(nil)
	if c.EstadoGoogle() != "" {
		t.Errorf("estado antes de iniciar: %q", c.EstadoGoogle())
	}
	u := iniciarGoogle(t, c)
	if u.Scheme != "https" || u.Host != "accounts.google.com" || u.Path != "/o/oauth2/v2/auth" {
		t.Fatalf("URL: %s", u)
	}
	q := u.Query()
	quer := map[string]string{"client_id": "cliente-de-teste.apps.exemplo.com", "response_type": "code",
		"scope": "openid email profile", "code_challenge_method": "S256", "prompt": "select_account"}
	for k, v := range quer {
		if q.Get(k) != v {
			t.Errorf("%s = %q, queria %q", k, q.Get(k), v)
		}
	}
	if !strings.Contains(u.RawQuery, "scope=openid%20email%20profile") {
		t.Errorf("scope devia usar %%20: %s", u.RawQuery)
	}
	r, err := url.Parse(q.Get("redirect_uri"))
	if err != nil {
		t.Fatal(err)
	}
	host, porta, err := net.SplitHostPort(r.Host)
	if err != nil || host != "127.0.0.1" || porta == "" || porta == "0" || r.Path != "/retorno" || r.Scheme != "http" {
		t.Fatalf("redirect_uri: %s", r)
	}
	for _, k := range []string{"state", "code_challenge"} {
		b, err := base64.RawURLEncoding.DecodeString(q.Get(k))
		if err != nil || len(b) != 32 {
			t.Errorf("%s inválido: %q", k, q.Get(k))
		}
	}
	if q.Get("state") == q.Get("code_challenge") {
		t.Error("state e challenge iguais")
	}
}

func TestGoogleRetornoEntra(t *testing.T) {
	a := novoAmbiente(t, 0)
	n := &capturaGoogle{Falsa: a.f}
	c := a.conta(n)
	u := iniciarGoogle(t, c)
	if c.EstadoGoogle() != "aguardando" {
		t.Fatalf("estado: %q", c.EstadoGoogle())
	}
	q := u.Query()
	st, corpo, h := retorno(t, u, url.Values{"code": {"ana@exemplo.com"}, "state": {q.Get("state")}})
	if st != 200 || !strings.Contains(corpo, "Pode voltar para o Applyer") {
		t.Fatalf("resposta: %d %s", st, corpo)
	}
	if h.Get("Content-Type") != "text/html; charset=utf-8" || h.Get("Cache-Control") != "no-store" {
		t.Errorf("cabeçalhos: %v", h)
	}
	querAgir(t, c, true, "")
	if s := c.Situacao(); s.Email != "ana@exemplo.com" {
		t.Fatalf("situação: %+v", s)
	}
	if a.cofre[chaveRenovacao] == "" {
		t.Fatal("renovação devia estar no cofre")
	}
	chamadas, code, ver, red := n.vistos()
	sum := sha256.Sum256([]byte(ver))
	if chamadas != 1 || code != "ana@exemplo.com" || red != q.Get("redirect_uri") ||
		base64.RawURLEncoding.EncodeToString(sum[:]) != q.Get("code_challenge") {
		t.Fatalf("troca: %d %q %q %q", chamadas, code, ver, red)
	}
	esperar(t, func() bool { return c.EstadoGoogle() == "" })
	// o ouvinte fechou
	esperar(t, func() bool {
		cn, err := net.DialTimeout("tcp", enderecoOuvinte(u), time.Second)
		if err == nil {
			cn.Close()
		}
		return err != nil
	})
}

func TestGoogleStateErradoNaoEntra(t *testing.T) {
	a := novoAmbiente(t, 0)
	n := &capturaGoogle{Falsa: a.f}
	c := a.conta(n)
	u := iniciarGoogle(t, c)
	st, corpo, _ := retorno(t, u, url.Values{"code": {"ana@exemplo.com"}, "state": {"<b>errado</b>"}})
	if st == 200 || strings.Contains(corpo, "errado") || strings.Contains(corpo, "Pode voltar") {
		t.Fatalf("resposta: %d %s", st, corpo)
	}
	if ch, _, _, _ := n.vistos(); ch != 0 {
		t.Fatal("state errado não pode chamar a nuvem")
	}
	querAgir(t, c, false, MotivoSemConta)
	// o login certo ainda funciona depois de um intruso
	retorno(t, u, url.Values{"code": {"ana@exemplo.com"}, "state": {u.Query().Get("state")}})
	querAgir(t, c, true, "")
}

func TestGoogleCanceladoPeloUsuarioNoGoogle(t *testing.T) {
	a := novoAmbiente(t, 0)
	n := &capturaGoogle{Falsa: a.f}
	c := a.conta(n)
	u := iniciarGoogle(t, c)
	st, corpo, _ := retorno(t, u, url.Values{"error": {"access_denied"}, "state": {u.Query().Get("state")}})
	if st == 200 || strings.Contains(corpo, "access_denied") {
		t.Fatalf("resposta: %d %s", st, corpo)
	}
	if got := c.EstadoGoogle(); got != "erro:Login com Google cancelado" {
		t.Fatalf("estado: %q", got)
	}
	if ch, _, _, _ := n.vistos(); ch != 0 {
		t.Fatal("não devia chamar a nuvem")
	}
}

func TestGoogleCanceladoFechaOuvinte(t *testing.T) {
	a := novoAmbiente(t, 0)
	c := a.conta(nil)
	u := iniciarGoogle(t, c)
	ender := enderecoOuvinte(u)
	cn, err := net.Dial("tcp", ender)
	if err != nil {
		t.Fatalf("ouvinte devia estar aberto: %v", err)
	}
	cn.Close()
	c.CancelarGoogle()
	if cn, err := net.DialTimeout("tcp", ender, time.Second); err == nil {
		cn.Close()
		t.Fatal("o ouvinte devia ter fechado")
	}
	if c.EstadoGoogle() != "" {
		t.Fatalf("estado: %q", c.EstadoGoogle())
	}
	// pode começar outro
	if err := c.IniciarGoogle(context.Background(), func(string) error { return nil }, TermosPadrao); err != nil {
		t.Fatal(err)
	}
	c.CancelarGoogle()
}

func TestGooglePrazoCincoMinutos(t *testing.T) {
	if prazoGoogle != 5*time.Minute {
		t.Fatalf("prazo = %v", prazoGoogle)
	}
	a := novoAmbiente(t, 0)
	c := a.conta(nil)
	c.google.prazo = 50 * time.Millisecond
	u := iniciarGoogle(t, c)
	esperar(t, func() bool { return strings.HasPrefix(c.EstadoGoogle(), "erro:") })
	if cn, err := net.DialTimeout("tcp", enderecoOuvinte(u), time.Second); err == nil {
		cn.Close()
		t.Fatal("o ouvinte devia ter fechado no prazo")
	}
	if err := c.IniciarGoogle(context.Background(), func(string) error { return nil }, TermosPadrao); err != nil {
		t.Fatalf("depois do prazo pode começar outro: %v", err)
	}
	c.CancelarGoogle()
}

func TestGoogleUmDeCadaVez(t *testing.T) {
	a := novoAmbiente(t, 0)
	c := a.conta(nil)
	iniciarGoogle(t, c)
	err := c.IniciarGoogle(context.Background(), func(string) error { t.Error("não devia abrir"); return nil }, TermosPadrao)
	if !errors.Is(err, ErrGoogleEmAndamento) {
		t.Fatalf("queria ErrGoogleEmAndamento, veio %v", err)
	}
}

func TestGoogleAbrirFalhaLiberaTudo(t *testing.T) {
	comClientID(t)
	a := novoAmbiente(t, 0)
	c := a.conta(nil)
	boom := errors.New("sem navegador")
	if err := c.IniciarGoogle(context.Background(), func(string) error { return boom }, TermosPadrao); !errors.Is(err, boom) {
		t.Fatalf("veio %v", err)
	}
	if c.EstadoGoogle() != "" {
		t.Fatalf("estado: %q", c.EstadoGoogle())
	}
	iniciarGoogle(t, c)
}

func TestGoogleErroDaNuvemViraFrase(t *testing.T) {
	a := novoAmbiente(t, 0)
	a.parConfirmado(t, "ana@exemplo.com") // conta com senha
	c := a.conta(nil)
	u := iniciarGoogle(t, c)
	retorno(t, u, url.Values{"code": {"ana@exemplo.com"}, "state": {u.Query().Get("state")}})
	if got := c.EstadoGoogle(); got != "erro:Esta conta entra com senha" || c.EstadoGoogleMotivo() != "entrar-com-senha" {
		t.Fatalf("estado %q motivo %q", got, c.EstadoGoogleMotivo())
	}
	querAgir(t, c, false, MotivoSemConta)

	// termos mudaram: motivo e versão ficam à mostra
	a.f.MudarTermos("2-novo")
	c2 := a.conta(nil)
	u = iniciarGoogle(t, c2)
	retorno(t, u, url.Values{"code": {"bia@exemplo.com"}, "state": {u.Query().Get("state")}})
	if c2.EstadoGoogleMotivo() != "termos" || c2.EstadoGoogleVersao() != "2-novo" {
		t.Fatalf("motivo %q versão %q", c2.EstadoGoogleMotivo(), c2.EstadoGoogleVersao())
	}
}

func TestGoogleSemInternet(t *testing.T) {
	a := novoAmbiente(t, 0)
	a.f.SemInternet(true)
	c := a.conta(nil)
	u := iniciarGoogle(t, c)
	retorno(t, u, url.Values{"code": {"ana@exemplo.com"}, "state": {u.Query().Get("state")}})
	if got := c.EstadoGoogle(); got != "erro:Sem conexão com a internet" {
		t.Fatalf("estado: %q", got)
	}
}

func TestGoogleSoGetERetorno(t *testing.T) {
	a := novoAmbiente(t, 0)
	n := &capturaGoogle{Falsa: a.f}
	c := a.conta(n)
	u := iniciarGoogle(t, c)
	base := u.Query().Get("redirect_uri")
	q := "?code=ana@exemplo.com&state=" + u.Query().Get("state")
	resp, err := http.Post(base+q, "text/plain", strings.NewReader("x"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST: %d", resp.StatusCode)
	}
	resp, err = http.Get(strings.TrimSuffix(base, "/retorno") + "/outro" + q)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("outro caminho: %d", resp.StatusCode)
	}
	if ch, _, _, _ := n.vistos(); ch != 0 {
		t.Fatal("não devia chamar a nuvem")
	}
}

func TestGoogleSoUmRetornoValido(t *testing.T) {
	a := novoAmbiente(t, 0)
	n := &capturaGoogle{Falsa: a.f}
	c := a.conta(n)
	u := iniciarGoogle(t, c)
	q := url.Values{"code": {"ana@exemplo.com"}, "state": {u.Query().Get("state")}}
	retorno(t, u, q)
	if resp, err := http.Get(u.Query().Get("redirect_uri") + "?" + q.Encode()); err == nil {
		resp.Body.Close()
	}
	if ch, _, _, _ := n.vistos(); ch != 1 {
		t.Fatalf("a nuvem foi chamada %d vezes", ch)
	}
}

// googleBloqueado segura a troca do código até soltar, para provar o uso único do retorno.
type googleBloqueado struct {
	*capturaGoogle
	chegou chan struct{}
	solta  chan struct{}
}

func (g *googleBloqueado) Google(ctx context.Context, code, v, r, d, termos string) (Par, error) {
	g.chegou <- struct{}{}
	<-g.solta
	return g.capturaGoogle.Google(ctx, code, v, r, d, termos)
}

func TestGoogleRetornoSimultaneoSoUmPassa(t *testing.T) {
	a := novoAmbiente(t, 0)
	cap := &capturaGoogle{Falsa: a.f}
	n := &googleBloqueado{capturaGoogle: cap, chegou: make(chan struct{}, 2), solta: make(chan struct{})}
	c := a.conta(n)
	u := iniciarGoogle(t, c)
	q := url.Values{"code": {"ana@exemplo.com"}, "state": {u.Query().Get("state")}}
	primeiro := make(chan int, 1)
	go func() { st, _, _ := retorno(t, u, q); primeiro <- st }()
	<-n.chegou // a primeira já está na nuvem
	st, _, _ := retorno(t, u, q)
	if st != http.StatusConflict {
		t.Fatalf("segunda: %d, queria 409", st)
	}
	close(n.solta)
	if st := <-primeiro; st != http.StatusOK {
		t.Fatalf("primeira: %d", st)
	}
	if ch, _, _, _ := cap.vistos(); ch != 1 {
		t.Fatalf("a nuvem foi chamada %d vezes", ch)
	}
	querAgir(t, c, true, "")
}

// googleLento responde certo mas só depois do prazo da troca.
type googleLento struct{ *Falsa }

func (g googleLento) Google(ctx context.Context, code, v, r, d, termos string) (Par, error) {
	p, err := g.Falsa.Google(ctx, code, v, r, d, termos)
	<-ctx.Done()
	return p, err
}

func TestGooglePrazoDaTrocaEstouraNaoEntra(t *testing.T) {
	velho := prazoTrocaGoogle
	prazoTrocaGoogle = 50 * time.Millisecond
	t.Cleanup(func() { prazoTrocaGoogle = velho })
	a := novoAmbiente(t, 0)
	c := a.conta(googleLento{a.f})
	u := iniciarGoogle(t, c)
	st, corpo, _ := retorno(t, u, url.Values{"code": {"ana@exemplo.com"}, "state": {u.Query().Get("state")}})
	if st == http.StatusOK || strings.Contains(corpo, "Pode voltar") {
		t.Fatalf("o navegador não pode ver sucesso: %d %s", st, corpo)
	}
	if got := c.EstadoGoogle(); !strings.HasPrefix(got, "erro:") {
		t.Fatalf("estado: %q", got)
	}
	querAgir(t, c, false, MotivoSemConta)
}
