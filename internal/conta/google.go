package conta

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// GoogleClientID é o client_id do app no Google (o ldflags da compilação preenche; os testes trocam).
var GoogleClientID string

// prazoGoogle é quanto o ouvinte espera a pessoa voltar do navegador.
const prazoGoogle = 5 * time.Minute

// prazoTrocaGoogle limita a troca do código na nuvem (e a entrada que vem junto).
var prazoTrocaGoogle = 30 * time.Second

// ErrGoogleEmAndamento: já existe um login com Google esperando a volta do navegador.
var ErrGoogleEmAndamento = errors.New("já existe um login com Google em andamento")

const (
	urlAutorizacaoGoogle = "https://accounts.google.com/o/oauth2/v2/auth"
	caminhoRetorno       = "/retorno"
)

// Páginas que o navegador vê: texto fixo, nunca com valores da consulta.
const (
	paginaOK = `<!doctype html><html lang="pt-BR"><head><meta charset="utf-8"><title>Applyer</title></head>` +
		`<body style="font-family:sans-serif;text-align:center;margin-top:20vh"><h1>Pode voltar para o Applyer</h1>` +
		`<p>O login foi concluído. Você já pode fechar esta aba.</p></body></html>`
	paginaFalha = `<!doctype html><html lang="pt-BR"><head><meta charset="utf-8"><title>Applyer</title></head>` +
		`<body style="font-family:sans-serif;text-align:center;margin-top:20vh"><h1>Não foi possível entrar</h1>` +
		`<p>Volte para o Applyer e tente de novo.</p></body></html>`
	paginaCancelada = `<!doctype html><html lang="pt-BR"><head><meta charset="utf-8"><title>Applyer</title></head>` +
		`<body style="font-family:sans-serif;text-align:center;margin-top:20vh"><h1>Login cancelado</h1>` +
		`<p>Você pode fechar esta aba e voltar para o Applyer.</p></body></html>`
	paginaUsada = `<!doctype html><html lang="pt-BR"><head><meta charset="utf-8"><title>Applyer</title></head>` +
		`<body style="font-family:sans-serif;text-align:center;margin-top:20vh"><h1>Este link já foi usado</h1>` +
		`<p>Volte para o Applyer.</p></body></html>`
)

// estadoGoogle é o que a Conta guarda do login com Google.
type estadoGoogle struct {
	mu     sync.Mutex
	prazo  time.Duration // 0 usa prazoGoogle (os testes encurtam)
	ativa  *sessaoGoogle
	estado string // "" | "aguardando" | "erro:<frase>"
	motivo string // motivo do ErroNuvem ("termos", "entrar-com-senha"), se houve
	versao string // versão dos termos, quando motivo é "termos"
}

// sessaoGoogle é um login em andamento: um ouvinte e o que ele precisa para trocar o código.
type sessaoGoogle struct {
	state, verifier, redirect, termos string
	ctx                               context.Context
	cancel                            context.CancelFunc
	usado                             atomic.Bool   // o primeiro retorno válido (ou o prazo) fecha a porta aos outros
	feito                             chan struct{} // o retorno terminou (resposta já escrita)
	fim                               chan struct{} // o ouvinte fechou
}

// IniciarGoogle abre o ouvinte em 127.0.0.1 (porta livre qualquer), chama abrir com a URL de login do Google e
// volta; o resto corre em segundo plano (veja EstadoGoogle). Só um login por vez. O ouvinte não depende do ctx
// de quem chamou: só CancelarGoogle, o prazo ou o fim do login o fecham.
func (c *Conta) IniciarGoogle(ctx context.Context, abrir func(url string) error, termos string) error {
	if GoogleClientID == "" {
		return errors.New("login com Google não configurado")
	}
	g := &c.google
	g.mu.Lock()
	if g.ativa != nil {
		g.mu.Unlock()
		return ErrGoogleEmAndamento
	}
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		g.mu.Unlock()
		return err
	}
	state, err1 := aleatorioURL(32)
	verifier, err2 := aleatorioURL(32)
	if err := errors.Join(err1, err2); err != nil {
		g.mu.Unlock()
		ln.Close()
		return err
	}
	sum := sha256.Sum256([]byte(verifier))
	s := &sessaoGoogle{state: state, verifier: verifier, termos: termos,
		redirect: "http://" + ln.Addr().String() + caminhoRetorno,
		feito:    make(chan struct{}, 1), fim: make(chan struct{})}
	s.ctx, s.cancel = context.WithCancel(context.WithoutCancel(ctx))
	prazo := g.prazo
	if prazo <= 0 {
		prazo = prazoGoogle
	}
	g.ativa, g.estado, g.motivo, g.versao = s, "aguardando", "", ""
	g.mu.Unlock()

	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { c.retornoGoogle(s, w, r) }),
		ReadHeaderTimeout: 10 * time.Second}
	go srv.Serve(ln)
	go c.supervisarGoogle(s, srv, prazo)

	q := url.Values{
		"client_id":             {GoogleClientID},
		"redirect_uri":          {s.redirect},
		"response_type":         {"code"},
		"scope":                 {"openid email profile"},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
		"code_challenge_method": {"S256"},
		"state":                 {state},
		"prompt":                {"select_account"},
	}
	// url.Values usa "+" para espaço; o Google aceita, mas %20 é o que a documentação mostra.
	if err := abrir(urlAutorizacaoGoogle + "?" + strings.ReplaceAll(q.Encode(), "+", "%20")); err != nil {
		s.cancel()
		<-s.fim
		return err
	}
	return nil
}

// CancelarGoogle fecha o ouvinte e limpa o estado. Sem login em andamento, não faz nada.
func (c *Conta) CancelarGoogle() {
	g := &c.google
	g.mu.Lock()
	s := g.ativa
	g.mu.Unlock()
	if s == nil {
		return
	}
	s.cancel()
	<-s.fim
}

// EstadoGoogle é "" (nada em andamento, ou já entrou), "aguardando" ou "erro:<frase>".
func (c *Conta) EstadoGoogle() string {
	c.google.mu.Lock()
	defer c.google.mu.Unlock()
	return c.google.estado
}

// EstadoGoogleMotivo é o motivo do erro da nuvem ("termos", "entrar-com-senha"), ou "".
func (c *Conta) EstadoGoogleMotivo() string {
	c.google.mu.Lock()
	defer c.google.mu.Unlock()
	return c.google.motivo
}

// EstadoGoogleVersao é a versão atual dos termos quando o motivo é "termos".
func (c *Conta) EstadoGoogleVersao() string {
	c.google.mu.Lock()
	defer c.google.mu.Unlock()
	return c.google.versao
}

// terminar encerra a sessão s no estado dado, se ela ainda for a atual.
func (g *estadoGoogle) terminar(s *sessaoGoogle, estado, motivo, versao string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.ativa == s {
		g.ativa, g.estado, g.motivo, g.versao = nil, estado, motivo, versao
	}
}

// supervisarGoogle fecha o ouvinte quando o login termina, o prazo acaba ou CancelarGoogle é chamado.
func (c *Conta) supervisarGoogle(s *sessaoGoogle, srv *http.Server, prazo time.Duration) {
	defer close(s.fim)
	timer := time.NewTimer(prazo)
	defer timer.Stop()
	espera := timer.C
	for {
		select {
		case <-s.ctx.Done():
			srv.Close()
			c.google.terminar(s, "", "", "")
			return
		case <-espera:
			if s.usado.CompareAndSwap(false, true) {
				srv.Close()
				c.google.terminar(s, "erro:O tempo para entrar com o Google acabou", "", "")
				return
			}
			espera = nil // um retorno já está sendo tratado: espera ele terminar
		case <-s.feito:
			ctx, cancelar := context.WithTimeout(context.Background(), 2*time.Second)
			if srv.Shutdown(ctx) != nil {
				srv.Close()
			}
			cancelar()
			s.cancel()
			return
		}
	}
}

func escreverPagina(w http.ResponseWriter, status int, corpo string) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(corpo))
}

// retornoGoogle trata a volta do navegador: só GET /retorno, com o state certo, uma vez.
func (c *Conta) retornoGoogle(s *sessaoGoogle, w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != caminhoRetorno {
		escreverPagina(w, http.StatusNotFound, paginaFalha)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		escreverPagina(w, http.StatusMethodNotAllowed, paginaFalha)
		return
	}
	if "http://"+r.Host+caminhoRetorno != s.redirect {
		escreverPagina(w, http.StatusBadRequest, paginaFalha)
		return
	}
	q := r.URL.Query()
	if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(s.state)) != 1 {
		escreverPagina(w, http.StatusBadRequest, paginaFalha) // não gasta a vez: o retorno certo ainda pode chegar
		return
	}
	code := q.Get("code")
	if q.Get("error") == "" && code == "" {
		escreverPagina(w, http.StatusBadRequest, paginaFalha)
		return
	}
	if !s.usado.CompareAndSwap(false, true) {
		escreverPagina(w, http.StatusConflict, paginaUsada)
		return
	}
	defer func() { s.feito <- struct{}{} }()
	if q.Get("error") != "" {
		c.google.terminar(s, "erro:Login com Google cancelado", "", "")
		escreverPagina(w, http.StatusBadRequest, paginaCancelada)
		return
	}
	estado, motivo, versao := c.trocarGoogle(s, code)
	if s.ctx.Err() != nil { // CancelarGoogle durante a troca: a pessoa desistiu
		escreverPagina(w, http.StatusBadRequest, paginaCancelada)
		return
	}
	c.google.terminar(s, estado, motivo, versao)
	if estado != "" {
		escreverPagina(w, http.StatusBadRequest, paginaFalha)
		return
	}
	escreverPagina(w, http.StatusOK, paginaOK)
}

// trocarGoogle troca o código na nuvem (com contexto próprio de 30 s, não o do pedido HTTP) e guarda a sessão.
// Devolve o estado final: "" se entrou, senão "erro:<frase>" (e motivo/versão da nuvem, se vieram).
func (c *Conta) trocarGoogle(s *sessaoGoogle, code string) (estado, motivo, versao string) {
	ctx, cancelar := context.WithTimeout(s.ctx, prazoTrocaGoogle)
	defer cancelar()
	disp, err := c.idDispositivo()
	var par Par
	if err == nil {
		par, err = c.nuvem.Google(ctx, code, s.verifier, s.redirect, disp, s.termos)
	}
	if err == nil && ctx.Err() != nil {
		err = ctx.Err() // prazo estourado (ou cancelado) logo depois da resposta: não entra
	}
	if err == nil {
		// A sessão já está guardada mesmo que a licença não venha agora (ex.: caiu a internet): o ciclo de
		// Iniciar pede de novo.
		if errEntrou := c.Entrou(ctx, par); errEntrou != nil && !c.Situacao().Entrada {
			err = errEntrou
		}
	}
	if err == nil {
		return "", "", ""
	}
	var en *ErroNuvem
	switch {
	case errors.As(err, &en):
		return "erro:" + en.Msg, en.Motivo, en.Versao
	case errors.Is(err, ErrSemInternet):
		return "erro:Sem conexão com a internet", "", ""
	default:
		return "erro:Não foi possível entrar com o Google", "", ""
	}
}

func aleatorioURL(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
