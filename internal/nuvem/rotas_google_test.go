package nuvem

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/nuvem/contas"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/google"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/senhas"
)

const (
	retorno       = "http://127.0.0.1:53682/retorno"
	verifierTeste = "verificador-ficticio-com-mais-de-quarenta-e-tres-caracteres-0123"
	msgGoogleErro = "Não foi possível entrar com o Google"
)

// google faz o POST /v1/contas/google com um code que o Google falso conhece (ou não).
func (a *amb) google(t *testing.T, code, termos string) *httptest.ResponseRecorder {
	t.Helper()
	corpo := map[string]string{"code": code, "verifier": verifierTeste, "redirect_uri": retorno, "dispositivo": dispositivo}
	if termos != "" {
		corpo["termos_versao"] = termos
	}
	return a.post(t, "/v1/contas/google", corpo)
}

func (a *amb) usuario(t *testing.T, email string) contas.Usuario {
	t.Helper()
	u, err := a.s.contas.PorEmail(context.Background(), email)
	if err != nil {
		t.Fatalf("ler %s: %v", email, err)
	}
	return u
}

func (a *amb) contarUsuarios(t *testing.T) int {
	t.Helper()
	var n int
	if err := a.db.QueryRow(context.Background(), `select count(*) from usuarios`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestGoogleCriaConta(t *testing.T) {
	a := novoAmb(t)
	a.ids["c1"] = google.Identidade{Sub: "1000001", Email: "ana@exemplo.com", Nome: "Ana Teste"}
	par(t, a.google(t, "c1", termosAtual))
	u := a.usuario(t, "ana@exemplo.com")
	if u.GoogleSub != "1000001" || !u.Confirmado() || u.SenhaHash != "" || u.Nome != "Ana Teste" {
		t.Fatalf("usuário = %+v", u)
	}
	if u.TermosVersao != termosAtual || u.TermosAceitosEm == nil || !u.TermosAceitosEm.Equal(a.rel.agora()) {
		t.Fatalf("aceite dos termos não gravado: %+v", u)
	}
	// Conta só com Google não entra por senha.
	confere(t, a.entrar(t, "ana@exemplo.com", senhaAna), http.StatusUnauthorized, "Email ou senha incorretos")
}

func TestGoogleContaNovaExigeTermos(t *testing.T) {
	a := novoAmb(t)
	a.ids["c1"] = google.Identidade{Sub: "1000001", Email: "ana@exemplo.com"}
	for _, v := range []string{"", "2020-01"} {
		w := a.google(t, "c1", v)
		confere(t, w, http.StatusConflict, "Os termos mudaram")
		if c := corpoJSON(t, w); c["motivo"] != "termos" || c["versao"] != termosAtual {
			t.Fatalf("corpo = %v", c)
		}
	}
	if n := a.contarUsuarios(t); n != 0 {
		t.Fatalf("criou %d contas sem aceite", n)
	}
}

func TestGoogleLigaPorEmail(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, "ana@exemplo.com", senhaAna)
	antes := a.usuario(t, "ana@exemplo.com")
	a.ids["c1"] = google.Identidade{Sub: "1000001", Email: "ana@exemplo.com", Nome: "Ana Teste", Autoritativo: true}
	// Conta existente com termos em dia: não precisa mandar a versão.
	par(t, a.google(t, "c1", ""))
	u := a.usuario(t, "ana@exemplo.com")
	if u.ID != antes.ID || u.GoogleSub != "1000001" || u.SenhaHash != antes.SenhaHash {
		t.Fatalf("ligação errada: %+v", u)
	}
	// A senha continua valendo.
	par(t, a.entrar(t, "ana@exemplo.com", senhaAna))
	if n := a.contarUsuarios(t); n != 1 {
		t.Fatalf("usuários = %d", n)
	}
}

// R13: email de domínio comum (sem hd) não prova a posse para o Google; a conta com senha não é ligada.
func TestGoogleNaoAutoritativoNaoLigaContaComSenha(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, "ana@exemplo.com", senhaAna)
	antes := a.usuario(t, "ana@exemplo.com")
	a.ids["c1"] = google.Identidade{Sub: "1000001", Email: "ana@exemplo.com"}
	w := a.google(t, "c1", termosAtual)
	confere(t, w, http.StatusConflict, "Este email já tem conta com senha. Entre com a senha.")
	if c := corpoJSON(t, w); c["motivo"] != "entrar-com-senha" {
		t.Fatalf("corpo = %v", c)
	}
	u := a.usuario(t, "ana@exemplo.com")
	if u.GoogleSub != "" || u.SenhaHash != antes.SenhaHash || a.contarUsuarios(t) != 1 {
		t.Fatalf("conta mudou: %+v", u)
	}
	par(t, a.entrar(t, "ana@exemplo.com", senhaAna))
}

func TestGoogleGmailLiga(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, "ana@gmail.com", senhaAna)
	a.ids["c1"] = google.Identidade{Sub: "1000001", Email: "ana@gmail.com", Autoritativo: true}
	par(t, a.google(t, "c1", ""))
	if u := a.usuario(t, "ana@gmail.com"); u.GoogleSub != "1000001" || u.SenhaHash == "" {
		t.Fatalf("ligação errada: %+v", u)
	}
}

func TestGoogleHDLiga(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, "ana@exemplo.com", senhaAna)
	a.ids["c1"] = google.Identidade{Sub: "1000001", Email: "ana@exemplo.com", Autoritativo: true}
	par(t, a.google(t, "c1", ""))
	if u := a.usuario(t, "ana@exemplo.com"); u.GoogleSub != "1000001" {
		t.Fatalf("ligação errada: %+v", u)
	}
}

// A regra do hd mora no google.Real (TestRealAutoritativo); aqui, a identidade que ele devolve para hd
// de outro domínio (não autoritativa) não liga a conta com senha.
func TestGoogleHDDeOutroDominioNaoAutoritativo(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, "ana@exemplo.com", senhaAna)
	a.ids["c1"] = google.Identidade{Sub: "1000001", Email: "ana@exemplo.com", Autoritativo: false}
	confere(t, a.google(t, "c1", ""), http.StatusConflict, "Este email já tem conta com senha. Entre com a senha.")
	if u := a.usuario(t, "ana@exemplo.com"); u.GoogleSub != "" {
		t.Fatalf("ligou: %+v", u)
	}
}

func TestGoogleLigaContaNaoConfirmada(t *testing.T) {
	a := novoAmb(t)
	// Alguém cadastrou o email da Ana com uma senha (talvez um atacante) e nunca confirmou.
	const senhaPlantada = "atacante-quer-entrar-31"
	confere(t, a.post(t, "/v1/contas", map[string]string{"email": "ana@exemplo.com", "senha": senhaPlantada, "termos_versao": termosAtual}),
		http.StatusAccepted, "")
	antes := a.usuario(t, "ana@exemplo.com")
	a.ids["c1"] = google.Identidade{Sub: "1000001", Email: "ana@exemplo.com"}
	// O aceite pendente foi de quem cadastrou, não da Ana: precisa trazer a versão.
	confere(t, a.google(t, "c1", ""), http.StatusConflict, "Os termos mudaram")
	if u := a.usuario(t, "ana@exemplo.com"); u.GoogleSub != "" || u.Confirmado() {
		t.Fatalf("o 409 não deveria ligar: %+v", u)
	}
	par(t, a.google(t, "c1", termosAtual))
	u := a.usuario(t, "ana@exemplo.com")
	if u.ID != antes.ID || u.GoogleSub != "1000001" || !u.Confirmado() || u.TermosAceitosEm == nil {
		t.Fatalf("ligação errada: %+v", u)
	}
	// A senha plantada não vale mais.
	if u.SenhaHash != "" {
		t.Fatal("a senha pendente deveria ser apagada")
	}
	a.rel.avancar(time.Minute)
	confere(t, a.entrar(t, "ana@exemplo.com", senhaPlantada), http.StatusUnauthorized, "Email ou senha incorretos")
}

func TestGoogleMesmoSubEntraNaMesmaConta(t *testing.T) {
	a := novoAmb(t)
	a.ids["c1"] = google.Identidade{Sub: "1000001", Email: "ana@exemplo.com"}
	par(t, a.google(t, "c1", termosAtual))
	id := a.usuario(t, "ana@exemplo.com").ID
	par(t, a.google(t, "c1", ""))
	// O email mudou no Google: o sub manda, sem conta nova.
	a.ids["c2"] = google.Identidade{Sub: "1000001", Email: "ana.nova@exemplo.com"}
	par(t, a.google(t, "c2", ""))
	if n := a.contarUsuarios(t); n != 1 {
		t.Fatalf("usuários = %d", n)
	}
	u, err := a.s.contas.PorGoogle(context.Background(), "1000001")
	if err != nil || u.ID != id {
		t.Fatalf("%+v %v", u, err)
	}
	// Mesmo com outra conta já dona do email novo, o sub entra na dele.
	a.criarConfirmado(t, "bia@exemplo.com", senhaAna)
	a.ids["c3"] = google.Identidade{Sub: "1000001", Email: "bia@exemplo.com"}
	par(t, a.google(t, "c3", ""))
	if b := a.usuario(t, "bia@exemplo.com"); b.GoogleSub != "" {
		t.Fatalf("ligou o sub na conta de outra pessoa: %+v", b)
	}
}

func TestGoogleEmailLigadoAOutroSub(t *testing.T) {
	a := novoAmb(t)
	a.ids["c1"] = google.Identidade{Sub: "1000001", Email: "ana@exemplo.com"}
	par(t, a.google(t, "c1", termosAtual))
	// Outra conta Google com o mesmo email (ex.: conta recriada): não toma a conta ligada ao sub antigo.
	a.ids["c2"] = google.Identidade{Sub: "2000002", Email: "ana@exemplo.com"}
	confere(t, a.google(t, "c2", termosAtual), http.StatusUnauthorized, msgGoogleErro)
	if u := a.usuario(t, "ana@exemplo.com"); u.GoogleSub != "1000001" {
		t.Fatalf("sub trocado: %+v", u)
	}
}

func TestGoogleTermosMudaram(t *testing.T) {
	a := novoAmb(t)
	a.ids["c1"] = google.Identidade{Sub: "1000001", Email: "ana@exemplo.com"}
	par(t, a.google(t, "c1", termosAtual))
	if _, err := a.db.Exec(context.Background(), `update usuarios set termos_versao = '2025-01'`); err != nil {
		t.Fatal(err)
	}
	confere(t, a.google(t, "c1", ""), http.StatusConflict, "Os termos mudaram")
	par(t, a.google(t, "c1", termosAtual))
	if u := a.usuario(t, "ana@exemplo.com"); u.TermosVersao != termosAtual {
		t.Fatalf("aceite não gravado: %+v", u)
	}
}

func TestGoogleErros(t *testing.T) {
	a := novoAmb(t)
	a.ids["c1"] = google.Identidade{Sub: "1000001", Email: "ana@exemplo.com"}
	// redirect_uri fora do loopback: 400, sem chamar o Google.
	w := a.post(t, "/v1/contas/google", map[string]string{"code": "c1", "verifier": verifierTeste,
		"redirect_uri": "http://evil.com/retorno", "dispositivo": dispositivo, "termos_versao": termosAtual})
	confere(t, w, http.StatusBadRequest, "")
	// Code que o Google recusa: 401 com a mensagem fixa.
	confere(t, a.google(t, "desconhecido", termosAtual), http.StatusUnauthorized, msgGoogleErro)
	// Sem code ou verifier, ou dispositivo inválido: 400.
	confere(t, a.post(t, "/v1/contas/google", map[string]string{"verifier": verifierTeste, "redirect_uri": retorno,
		"dispositivo": dispositivo}), http.StatusBadRequest, "")
	confere(t, a.post(t, "/v1/contas/google", map[string]string{"code": "c1", "redirect_uri": retorno,
		"dispositivo": dispositivo}), http.StatusBadRequest, "")
	confere(t, a.post(t, "/v1/contas/google", map[string]string{"code": "c1", "verifier": verifierTeste,
		"redirect_uri": retorno, "dispositivo": "x"}), http.StatusBadRequest, "Dispositivo inválido")
	if n := a.contarUsuarios(t); n != 0 {
		t.Fatalf("usuários = %d", n)
	}
	// O log não leva code, verifier, tokens nem o email inteiro.
	p := par(t, a.google(t, "c1", termosAtual))
	log := a.log.String()
	for _, s := range []string{"c1\"", "=c1 ", verifierTeste, p.Acesso, p.Renovacao, "ana@exemplo.com"} {
		if strings.Contains(log, s) {
			t.Errorf("log contém %q", s)
		}
	}
}

// A confirmação só vale para a senha que acabou de ser conferida: se um recadastro trocou a senha
// pendente no meio, o update não pega nenhuma linha.
func TestConfirmarComHashVelhoNaoConfirma(t *testing.T) {
	a := novoAmb(t)
	ctx := context.Background()
	h1, err := senhas.Resumir(senhaAna)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := senhas.Resumir(senhaNova)
	if err != nil {
		t.Fatal(err)
	}
	u, _, err := a.s.contas.Criar(ctx, "ana@exemplo.com", h1, termosAtual)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.s.contas.Criar(ctx, "ana@exemplo.com", h2, termosAtual); err != nil {
		t.Fatal(err)
	}
	if err := a.s.contas.Confirmar(ctx, u.ID, h1, a.rel.agora()); err != contas.ErrNaoEncontrado {
		t.Fatalf("hash velho: erro = %v", err)
	}
	if a.usuario(t, "ana@exemplo.com").Confirmado() {
		t.Fatal("confirmou com o hash velho")
	}
	if err := a.s.contas.Confirmar(ctx, u.ID, h2, a.rel.agora()); err != nil {
		t.Fatal(err)
	}
	if !a.usuario(t, "ana@exemplo.com").Confirmado() {
		t.Fatal("não confirmou com o hash atual")
	}
}

// A R13 também vale dentro do UPDATE: se a conta foi confirmada com senha depois da leitura, o
// LigarGoogle não autoritativo não pega a linha.
func TestLigarGoogleNaoAutoritativoNoBanco(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, "ana@exemplo.com", senhaAna)
	u := a.usuario(t, "ana@exemplo.com")
	ctx := context.Background()
	if _, err := a.s.contas.LigarGoogle(ctx, u.ID, "1000001", "", termosAtual, false, a.rel.agora()); err != contas.ErrConflito {
		t.Fatalf("erro = %v", err)
	}
	if v, err := a.s.contas.LigarGoogle(ctx, u.ID, "1000001", "", termosAtual, true, a.rel.agora()); err != nil || v.GoogleSub != "1000001" {
		t.Fatalf("%+v %v", v, err)
	}
}

// Trocar a senha pelo código do email desliga o Google de email de domínio próprio (a conta Google pode ser de
// um dono antigo do domínio); em @gmail.com o Google é do mesmo dono do email e fica ligado.
func TestNovaSenhaDesligaGoogleDeDominioProprio(t *testing.T) {
	a := novoAmb(t)
	a.ids["c1"] = google.Identidade{Sub: "1000001", Email: "ana@exemplo.com"}
	a.ids["c2"] = google.Identidade{Sub: "1000002", Email: "bia@gmail.com"}
	par(t, a.google(t, "c1", termosAtual))
	par(t, a.google(t, "c2", termosAtual))
	for _, email := range []string{"ana@exemplo.com", "bia@gmail.com"} {
		confere(t, a.post(t, "/v1/contas/recuperar", map[string]string{"email": email}), http.StatusAccepted, "")
		par(t, a.post(t, "/v1/contas/nova-senha", map[string]string{
			"email": email, "codigo": a.codigo(t, email), "senha": senhaNova, "dispositivo": dispositivo}))
	}
	if u := a.usuario(t, "ana@exemplo.com"); u.GoogleSub != "" {
		t.Fatalf("domínio próprio manteve o Google: %q", u.GoogleSub)
	}
	if u := a.usuario(t, "bia@gmail.com"); u.GoogleSub != "1000002" {
		t.Fatalf("gmail perdeu o Google: %q", u.GoogleSub)
	}
	// O Google antigo não entra mais na conta de domínio próprio (agora com senha, não autoritativo: 409).
	if w := a.google(t, "c1", termosAtual); w.Code == http.StatusOK {
		t.Fatalf("Google antigo ainda entra: %s", w.Body)
	}
}
