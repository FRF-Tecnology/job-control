package nuvem

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/FRF-Tecnology/job-control/internal/nuvem/codigos"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/contas"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/emails"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/senhas"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/sessoes"
)

// Rotas de conta por email e senha. Regras que valem para todas:
//   - nenhuma resposta revela se um email tem conta (mesmo status e corpo para quem tem e quem não tem);
//   - o log nunca leva senha, código, token nem o email inteiro (só mascarar(email) ou o id do usuário).

const (
	limitePorIP        = 20 // por minuto, somando todas as rotas de conta
	limitePorEmail     = 5  // a cada 15 min, em criar conta e recuperar
	prazoEmail         = time.Minute
	capacidadeSemConta = 100_000 // emails sem conta acompanhados em memória (teto de memória)

	msgEnviamos    = "Se o email estiver certo, enviamos um código"
	msgLimite      = "Muitas tentativas. Tente de novo em alguns minutos"
	msgBloqueado   = "Muitas tentativas. Tente de novo em 15 minutos"
	msgCredenciais = "Email ou senha incorretos"
	msgCodigo      = "Código inválido ou vencido"
	msgSessao      = "sessão inválida"
	msgTermos      = "Os termos mudaram"
	msgConfirme    = "Confirme seu email"
	msgPedido      = "Pedido inválido"
	msgEmail       = "Email inválido"
	msgDispositivo = "Dispositivo inválido"
	msgInterno     = "Erro interno. Tente de novo"
)

var reDispositivo = regexp.MustCompile(`^[0-9a-f]{64}$`)

// pedido é o corpo de todas as rotas de conta; cada rota usa só os campos dela.
type pedido struct {
	Email        string `json:"email"`
	Senha        string `json:"senha"`
	Codigo       string `json:"codigo"`
	Dispositivo  string `json:"dispositivo"`
	Renovacao    string `json:"renovacao"`
	TermosVersao string `json:"termos_versao"`
	// Login com Google (POST /v1/contas/google).
	Code        string `json:"code"`
	Verifier    string `json:"verifier"`
	RedirectURI string `json:"redirect_uri"`
}

func (s *Servidor) rotasContas() {
	for padrao, h := range map[string]http.HandlerFunc{
		"POST /v1/contas":            s.criarConta,
		"POST /v1/contas/confirmar":  s.confirmar,
		"POST /v1/contas/entrar":     s.entrar,
		"POST /v1/contas/recuperar":  s.recuperar,
		"POST /v1/contas/nova-senha": s.novaSenha,
		"POST /v1/contas/google":     s.entrarGoogle,
		"POST /v1/sessao/renovar":    s.renovar,
		"POST /v1/sessao/sair":       s.sair,
	} {
		s.mux.Handle(padrao, s.limitarIP(h))
	}
}

// limitarIP: até limitePorIP pedidos por minuto por IP. O IP é o da conexão (RemoteAddr); atrás de proxy
// todos chegam com o IP do proxy — ver o relatório da task antes de publicar atrás do Coolify/Cloudflare.
func (s *Servidor) limitarIP(h http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.porIP.Permitir("ip|" + ipDe(r)) {
			erro(w, http.StatusTooManyRequests, msgLimite)
			return
		}
		h(w, r)
	})
}

func ipDe(r *http.Request) string {
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return h
	}
	return r.RemoteAddr
}

func erro(w http.ResponseWriter, status int, msg string) {
	escreverJSON(w, status, map[string]string{"erro": msg})
}

func (s *Servidor) interno(w http.ResponseWriter, r *http.Request, onde string, err error) {
	s.log.Error("falha interna", "rota", r.URL.Path, "onde", onde, "erro", err.Error())
	erro(w, http.StatusInternalServerError, msgInterno)
}

func lerPedido(w http.ResponseWriter, r *http.Request, p *pedido) bool {
	if err := json.NewDecoder(r.Body).Decode(p); err != nil {
		erro(w, http.StatusBadRequest, msgPedido)
		return false
	}
	return true
}

// lerEmail normaliza o email do pedido; inválido → 400.
func lerEmail(w http.ResponseWriter, p pedido) (string, bool) {
	e, err := contas.NormalizarEmail(p.Email)
	if err != nil {
		erro(w, http.StatusBadRequest, msgEmail)
		return "", false
	}
	return e, true
}

func dispositivoOK(w http.ResponseWriter, p pedido) bool {
	if !reDispositivo.MatchString(p.Dispositivo) {
		erro(w, http.StatusBadRequest, msgDispositivo)
		return false
	}
	return true
}

func senhaOK(w http.ResponseWriter, senha string) bool {
	err := senhas.Validar(senha)
	switch {
	case err == nil:
		return true
	case errors.Is(err, senhas.ErrCurta):
		erro(w, http.StatusBadRequest, "A senha precisa ter pelo menos 10 caracteres")
	case errors.Is(err, senhas.ErrLonga):
		erro(w, http.StatusBadRequest, "A senha pode ter no máximo 128 caracteres")
	default:
		erro(w, http.StatusBadRequest, "Essa senha é muito comum. Escolha outra")
	}
	return false
}

func (s *Servidor) termosMudaram(w http.ResponseWriter, status int) {
	escreverJSON(w, status, map[string]string{"erro": msgTermos, "motivo": "termos", "versao": s.cfg.TermosVersao})
}

// conferirTermos: usuário com versão antiga precisa trazer a atual no corpo (aí grava o aceite); senão 409.
func (s *Servidor) conferirTermos(w http.ResponseWriter, r *http.Request, u contas.Usuario, p pedido) bool {
	if u.TermosVersao == s.cfg.TermosVersao {
		return true
	}
	if p.TermosVersao != s.cfg.TermosVersao {
		s.termosMudaram(w, http.StatusConflict)
		return false
	}
	if err := s.contas.AceitarTermos(r.Context(), u.ID, s.cfg.TermosVersao, s.dep.Agora()); err != nil {
		s.interno(w, r, "aceitar termos", err)
		return false
	}
	return true
}

// enviar manda o email em segundo plano: a resposta não espera o SMTP (nem revela, pelo tempo, se mandou).
func (s *Servidor) enviar(m emails.Mensagem) {
	s.envios.Add(1)
	go func() {
		defer s.envios.Done()
		ctx, cancelar := context.WithTimeout(context.Background(), prazoEmail)
		defer cancelar()
		para := mascarar(m.Para)
		if err := s.dep.Emails.Enviar(ctx, m); err != nil {
			s.log.Error("falha ao enviar email", "para", para, "assunto", m.Assunto,
				"erro", strings.ReplaceAll(err.Error(), m.Para, para))
			return
		}
		s.log.Info("email enviado", "para", para, "assunto", m.Assunto)
	}()
}

// mandarCodigo gera e envia um código. Pedido antes de 60 s do anterior não gera outro e não é erro:
// responder diferente revelaria que a conta existe; o código anterior segue valendo.
func (s *Servidor) mandarCodigo(ctx context.Context, email string, t codigos.Tipo) error {
	cod, err := s.codigos.Gerar(ctx, email, t)
	if errors.Is(err, codigos.ErrEspere) {
		s.log.Info("código pedido cedo demais", "para", mascarar(email), "tipo", string(t))
		return nil
	}
	if err != nil {
		return err
	}
	s.enviar(emails.Codigo(email, cod, t))
	return nil
}

// abrirSessao responde 200 com um par novo.
func (s *Servidor) abrirSessao(w http.ResponseWriter, r *http.Request, usuarioID, dispositivo string) {
	p, err := s.sessoes.Abrir(r.Context(), usuarioID, dispositivo)
	if err != nil {
		s.interno(w, r, "abrir sessão", err)
		return
	}
	escreverJSON(w, http.StatusOK, p)
}

// POST /v1/contas
func (s *Servidor) criarConta(w http.ResponseWriter, r *http.Request) {
	var p pedido
	if !lerPedido(w, r, &p) {
		return
	}
	if p.TermosVersao != s.cfg.TermosVersao {
		s.termosMudaram(w, http.StatusBadRequest)
		return
	}
	email, ok := lerEmail(w, p)
	if !ok || !senhaOK(w, p.Senha) {
		return
	}
	if !s.porEmail.Permitir("contas|" + email) {
		erro(w, http.StatusTooManyRequests, msgLimite)
		return
	}
	// O resumo é feito sempre, antes de saber se a conta existe: mesmo custo nos dois casos.
	hash, err := senhas.Resumir(p.Senha)
	if err != nil {
		s.interno(w, r, "resumir senha", err)
		return
	}
	u, sit, err := s.contas.Criar(r.Context(), email, hash, s.cfg.TermosVersao)
	if err != nil {
		s.interno(w, r, "criar conta", err)
		return
	}
	switch sit {
	case contas.JaConfirmada:
		s.log.Info("cadastro com email que já tem conta", "usuario", u.ID)
		s.enviar(emails.JaTemConta(email))
	default:
		if sit == contas.Nova {
			s.log.Info("conta criada", "usuario", u.ID, "email", mascarar(email))
		}
		if err := s.mandarCodigo(r.Context(), email, codigos.Confirmar); err != nil {
			s.interno(w, r, "gerar código", err)
			return
		}
	}
	escreverJSON(w, http.StatusAccepted, map[string]string{"mensagem": msgEnviamos})
}

// POST /v1/contas/confirmar
func (s *Servidor) confirmar(w http.ResponseWriter, r *http.Request) {
	var p pedido
	if !lerPedido(w, r, &p) || !dispositivoOK(w, p) {
		return
	}
	email, ok := lerEmail(w, p)
	if !ok {
		return
	}
	// O código vem antes de tudo: sem ele, nada (nem o 409 dos termos) revela que a conta existe.
	if !s.conferirCodigo(w, r, email, codigos.Confirmar, p.Codigo) {
		return
	}
	u, err := s.contas.PorEmail(r.Context(), email)
	if errors.Is(err, contas.ErrNaoEncontrado) {
		erro(w, http.StatusBadRequest, msgCodigo)
		return
	}
	if err != nil {
		s.interno(w, r, "ler conta", err)
		return
	}
	// A senha precisa bater com a pendente: quem recadastrou o email com outra senha não ganha a conta
	// quando o dono confirma (o dono recupera pelo email). O código já foi gasto; tudo bem.
	if u.SenhaHash == "" {
		erro(w, http.StatusUnauthorized, msgCredenciais)
		return
	}
	certa, err := senhas.Conferir(u.SenhaHash, p.Senha)
	if err != nil {
		s.interno(w, r, "conferir senha", err)
		return
	}
	if !certa {
		erro(w, http.StatusUnauthorized, msgCredenciais)
		return
	}
	if u.TermosVersao != s.cfg.TermosVersao && p.TermosVersao != s.cfg.TermosVersao {
		// O código foi gasto: manda outro para a nova tentativa, já com a versão atual.
		if err := s.mandarCodigo(r.Context(), email, codigos.Confirmar); err != nil {
			s.interno(w, r, "gerar código", err)
			return
		}
	}
	if !s.conferirTermos(w, r, u, p) {
		return
	}
	// Só confirma se a senha pendente ainda for a conferida acima (um recadastro no meio a troca).
	if err := s.contas.Confirmar(r.Context(), u.ID, u.SenhaHash, s.dep.Agora()); errors.Is(err, contas.ErrNaoEncontrado) {
		erro(w, http.StatusUnauthorized, msgCredenciais)
		return
	} else if err != nil {
		s.interno(w, r, "confirmar", err)
		return
	}
	s.log.Info("email confirmado", "usuario", u.ID)
	s.abrirSessao(w, r, u.ID, p.Dispositivo)
}

func (s *Servidor) conferirCodigo(w http.ResponseWriter, r *http.Request, email string, t codigos.Tipo, cod string) bool {
	err := s.codigos.Conferir(r.Context(), email, t, cod)
	switch {
	case err == nil:
		return true
	case errors.Is(err, codigos.ErrCodigoInvalido), errors.Is(err, codigos.ErrTentativas):
		erro(w, http.StatusBadRequest, msgCodigo)
	default:
		s.interno(w, r, "conferir código", err)
	}
	return false
}

// POST /v1/contas/entrar
func (s *Servidor) entrar(w http.ResponseWriter, r *http.Request) {
	var p pedido
	if !lerPedido(w, r, &p) || !dispositivoOK(w, p) {
		return
	}
	email, ok := lerEmail(w, p)
	if !ok {
		return
	}
	ctx := r.Context()
	u, err := s.contas.PorEmail(ctx, email)
	if err != nil && !errors.Is(err, contas.ErrNaoEncontrado) {
		s.interno(w, r, "ler conta", err)
		return
	}
	if err != nil || u.SenhaHash == "" {
		// Sem conta (ou só Google): mesmo custo e mesmo bloqueio de quem tem, para não revelar nada.
		if s.semConta.Bloqueada(email) {
			erro(w, http.StatusTooManyRequests, msgBloqueado)
			return
		}
		_, _ = senhas.Conferir(s.resumoFalso, p.Senha)
		s.semConta.Falhar(email)
		erro(w, http.StatusUnauthorized, msgCredenciais)
		return
	}
	agora := s.dep.Agora()
	if u.Bloqueado(agora) {
		erro(w, http.StatusTooManyRequests, msgBloqueado)
		return
	}
	certa, err := senhas.Conferir(u.SenhaHash, p.Senha)
	if err != nil {
		s.interno(w, r, "conferir senha", err)
		return
	}
	if !certa {
		bloqueou, err := s.contas.RegistrarFalha(ctx, u.ID, agora)
		if err != nil {
			s.interno(w, r, "registrar falha", err)
			return
		}
		if bloqueou {
			s.log.Warn("login bloqueado por senhas erradas", "usuario", u.ID, "ip", ipDe(r))
		}
		erro(w, http.StatusUnauthorized, msgCredenciais)
		return
	}
	if err := s.contas.ZerarFalhas(ctx, u.ID); err != nil {
		s.interno(w, r, "zerar falhas", err)
		return
	}
	if !s.conferirTermos(w, r, u, p) {
		return
	}
	if !u.Confirmado() {
		if err := s.mandarCodigo(ctx, email, codigos.Confirmar); err != nil {
			s.interno(w, r, "gerar código", err)
			return
		}
		escreverJSON(w, http.StatusForbidden, map[string]string{"erro": msgConfirme, "motivo": "confirmar"})
		return
	}
	s.abrirSessao(w, r, u.ID, p.Dispositivo)
}

// POST /v1/contas/recuperar
func (s *Servidor) recuperar(w http.ResponseWriter, r *http.Request) {
	var p pedido
	if !lerPedido(w, r, &p) {
		return
	}
	email, ok := lerEmail(w, p)
	if !ok {
		return
	}
	if !s.porEmail.Permitir("recuperar|" + email) {
		erro(w, http.StatusTooManyRequests, msgLimite)
		return
	}
	_, err := s.contas.PorEmail(r.Context(), email)
	switch {
	case err == nil:
		if err := s.mandarCodigo(r.Context(), email, codigos.Recuperar); err != nil {
			s.interno(w, r, "gerar código", err)
			return
		}
	case !errors.Is(err, contas.ErrNaoEncontrado):
		s.interno(w, r, "ler conta", err)
		return
	}
	escreverJSON(w, http.StatusAccepted, map[string]string{"mensagem": msgEnviamos})
}

// POST /v1/contas/nova-senha
func (s *Servidor) novaSenha(w http.ResponseWriter, r *http.Request) {
	var p pedido
	if !lerPedido(w, r, &p) || !dispositivoOK(w, p) {
		return
	}
	email, ok := lerEmail(w, p)
	// A senha é validada antes do código, para uma senha fraca não gastar o código.
	if !ok || !senhaOK(w, p.Senha) {
		return
	}
	if !s.conferirCodigo(w, r, email, codigos.Recuperar, p.Codigo) {
		return
	}
	ctx := r.Context()
	u, err := s.contas.PorEmail(ctx, email)
	if errors.Is(err, contas.ErrNaoEncontrado) {
		erro(w, http.StatusBadRequest, msgCodigo)
		return
	}
	if err != nil {
		s.interno(w, r, "ler conta", err)
		return
	}
	hash, err := senhas.Resumir(p.Senha)
	if err != nil {
		s.interno(w, r, "resumir senha", err)
		return
	}
	// Revoga antes de trocar: se a troca falhar, sobra só a senha antiga sem sessões, nunca a senha
	// nova com as sessões antigas vivas.
	if err := s.sessoes.RevogarTodas(ctx, u.ID); err != nil {
		s.interno(w, r, "revogar sessões", err)
		return
	}
	if err := s.contas.TrocarSenha(ctx, u.ID, hash, s.dep.Agora()); err != nil {
		s.interno(w, r, "trocar senha", err)
		return
	}
	s.log.Info("senha trocada; sessões revogadas", "usuario", u.ID)
	s.enviar(emails.SenhaTrocada(email))
	s.abrirSessao(w, r, u.ID, p.Dispositivo)
}

// POST /v1/sessao/renovar — reuso e sessão inválida respondem igual.
func (s *Servidor) renovar(w http.ResponseWriter, r *http.Request) {
	var p pedido
	if !lerPedido(w, r, &p) {
		return
	}
	if p.Renovacao == "" {
		erro(w, http.StatusUnauthorized, msgSessao)
		return
	}
	par, err := s.sessoes.Renovar(r.Context(), p.Renovacao)
	switch {
	case err == nil:
		escreverJSON(w, http.StatusOK, par)
	case errors.Is(err, sessoes.ErrReuso):
		s.log.Warn("renovação reutilizada; família revogada", "ip", ipDe(r))
		erro(w, http.StatusUnauthorized, msgSessao)
	case errors.Is(err, sessoes.ErrSessao):
		erro(w, http.StatusUnauthorized, msgSessao)
	default:
		s.interno(w, r, "renovar", err)
	}
}

// POST /v1/sessao/sair
func (s *Servidor) sair(w http.ResponseWriter, r *http.Request) {
	var p pedido
	if !lerPedido(w, r, &p) {
		return
	}
	if p.Renovacao != "" {
		if err := s.sessoes.Sair(r.Context(), p.Renovacao); err != nil {
			s.interno(w, r, "sair", err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// mascarar troca o email por "a***@dominio" para o log.
func mascarar(email string) string {
	i := strings.LastIndex(email, "@")
	if i <= 0 {
		return "***"
	}
	_, n := utf8.DecodeRuneInString(email)
	return email[:n] + "***" + email[i:]
}
