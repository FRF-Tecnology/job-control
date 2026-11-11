package nuvem

import (
	"context"
	"errors"
	"net/http"

	"github.com/FRF-Tecnology/job-control/internal/nuvem/contas"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/google"
)

// Login com Google: o app abre o navegador com PKCE e retorno em loopback e manda para cá o code, o
// verifier e o redirect_uri. A nuvem troca o code (só ela tem o segredo), confere o id_token e abre a
// sessão. O log nunca leva o code, o verifier, tokens nem o email inteiro.

const (
	msgGoogle   = "Não foi possível entrar com o Google"
	msgRetorno  = "Endereço de retorno inválido"
	msgComSenha = "Este email já tem conta com senha. Entre com a senha."
)

type resultadoGoogle int

const (
	googleOK       resultadoGoogle = iota
	googleTermos                   // conta nova ou pendente sem o aceite da versão atual no corpo
	googleOutroSub                 // o email já está ligado a outra conta Google
	googleComSenha                 // conta com senha e o Google não é dono do email (R13)
)

// POST /v1/contas/google
func (s *Servidor) entrarGoogle(w http.ResponseWriter, r *http.Request) {
	var p pedido
	if !lerPedido(w, r, &p) || !dispositivoOK(w, p) {
		return
	}
	if p.Code == "" || p.Verifier == "" {
		erro(w, http.StatusBadRequest, msgPedido)
		return
	}
	if !google.RedirectValido(p.RedirectURI) {
		erro(w, http.StatusBadRequest, msgRetorno)
		return
	}
	id, err := s.dep.Google.Trocar(r.Context(), p.Code, p.Verifier, p.RedirectURI)
	if errors.Is(err, google.ErrRedirect) {
		erro(w, http.StatusBadRequest, msgRetorno)
		return
	}
	if err != nil {
		// Os erros do pacote google não trazem o texto do Google nem o code.
		s.log.Warn("login com Google recusado", "ip", ipDe(r), "erro", err.Error())
		erro(w, http.StatusUnauthorized, msgGoogle)
		return
	}

	// Duas tentativas: se outra requisição criou ou ligou a mesma conta no meio, a segunda leitura a acha.
	var u contas.Usuario
	res := googleOK
	for tentativa := 0; ; tentativa++ {
		u, res, err = s.contaGoogle(r.Context(), id, p.TermosVersao)
		if !errors.Is(err, contas.ErrConflito) || tentativa == 1 {
			break
		}
	}
	if err != nil {
		s.interno(w, r, "conta Google", err)
		return
	}
	switch res {
	case googleTermos:
		s.termosMudaram(w, http.StatusConflict)
		return
	case googleOutroSub:
		s.log.Warn("email já ligado a outra conta Google", "usuario", u.ID)
		erro(w, http.StatusUnauthorized, msgGoogle)
		return
	case googleComSenha:
		s.log.Info("Google não autoritativo para conta com senha; não ligada", "usuario", u.ID)
		escreverJSON(w, http.StatusConflict, map[string]string{"erro": msgComSenha, "motivo": "entrar-com-senha"})
		return
	}
	if !s.conferirTermos(w, r, u, p) {
		return
	}
	s.abrirSessao(w, r, u.ID, p.Dispositivo)
}

// contaGoogle acha a conta da identidade: pelo sub (o Google manda: mesmo sub, mesma conta, mesmo que o
// email tenha mudado); senão pelo email, ligando o sub; senão cria uma confirmada.
func (s *Servidor) contaGoogle(ctx context.Context, id google.Identidade, termos string) (contas.Usuario, resultadoGoogle, error) {
	u, err := s.contas.PorGoogle(ctx, id.Sub)
	if err == nil || !errors.Is(err, contas.ErrNaoEncontrado) {
		return u, googleOK, err
	}
	aceitou := termos == s.cfg.TermosVersao
	u, err = s.contas.PorEmail(ctx, id.Email)
	switch {
	case err == nil:
		if u.GoogleSub != "" {
			return u, googleOutroSub, nil
		}
		// Pendente: o aceite gravado foi de quem cadastrou, que pode não ser o dono do email.
		if !u.Confirmado() && !aceitou {
			return u, googleTermos, nil
		}
		// R13: conta confirmada com senha só liga se o Google for dono do email; senão, quem controla
		// a conta Google (ex.: um email de domínio próprio cadastrado no Google) tomaria a conta.
		if u.Confirmado() && u.SenhaHash != "" && !id.Autoritativo {
			return u, googleComSenha, nil
		}
		u, err = s.contas.LigarGoogle(ctx, u.ID, id.Sub, id.Nome, s.cfg.TermosVersao, id.Autoritativo, s.dep.Agora())
		if err != nil {
			return contas.Usuario{}, googleOK, err
		}
		s.log.Info("conta ligada ao Google", "usuario", u.ID)
		return u, googleOK, nil
	case errors.Is(err, contas.ErrNaoEncontrado):
		if !aceitou {
			return contas.Usuario{}, googleTermos, nil
		}
		u, err = s.contas.CriarGoogle(ctx, id.Email, id.Sub, id.Nome, s.cfg.TermosVersao, s.dep.Agora())
		if err != nil {
			return contas.Usuario{}, googleOK, err
		}
		s.log.Info("conta criada com Google", "usuario", u.ID, "email", mascarar(id.Email))
		return u, googleOK, nil
	default:
		return contas.Usuario{}, googleOK, err
	}
}
