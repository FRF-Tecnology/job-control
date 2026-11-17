package nuvem

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FRF-Tecnology/job-control/internal/nuvem/contas"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/emails"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/google"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/pagamento"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/pgteste"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/senhas"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/sessoes"
)

func TestMain(m *testing.M) { os.Exit(pgteste.Principal(m)) }

const (
	termosAtual = "2026-11"
	senhaAna    = "cavalo-bateria-grampo-9"
	senhaNova   = "girassol-trovao-mesa-42"
	senhaErrada = "nao-e-essa-senha-77"
)

var dispositivo = strings.Repeat("ab", 32)

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

// bufLog é um buffer seguro para o log (os envios de email logam em goroutine).
type bufLog struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *bufLog) Write(p []byte) (int, error) { b.mu.Lock(); defer b.mu.Unlock(); return b.b.Write(p) }
func (b *bufLog) String() string              { b.mu.Lock(); defer b.mu.Unlock(); return b.b.String() }

type amb struct {
	s     *Servidor
	em    *emails.Falso
	rel   *relogio
	db    *pgxpool.Pool
	log   *bufLog
	ids   map[string]google.Identidade // codes que o Google falso aceita
	st    *pagamento.Falso
	notas *notasFalsas
}

func novoAmb(t *testing.T) *amb {
	t.Helper()
	db := pgteste.Banco(t)
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{
		ChavePrivada:   base64.StdEncoding.EncodeToString(priv),
		SegredoCodigos: "segredo-ficticio-dos-testes",
		TermosVersao:   termosAtual,
	}
	a := &amb{ids: map[string]google.Identidade{}, em: &emails.Falso{}, rel: &relogio{t: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)}, db: db, log: &bufLog{},
		st: pagamento.NovoFalso(segredoWebhook), notas: &notasFalsas{}}
	a.s = Novo(cfg, Dependencias{
		Banco: db, Agora: a.rel.agora, Aleatorio: rand.Reader, Emails: a.em, Google: google.Falso{Identidades: a.ids},
		Stripe: a.st, Notas: a.notas,
		Log: slog.New(slog.NewTextHandler(a.log, &slog.HandlerOptions{Level: slog.LevelDebug})),
	})
	return a
}

func (a *amb) postDe(t *testing.T, ip, rota string, corpo any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(corpo)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", rota, bytes.NewReader(b))
	req.RemoteAddr = ip + ":40000"
	w := httptest.NewRecorder()
	a.s.Handler().ServeHTTP(w, req)
	return w
}

func (a *amb) post(t *testing.T, rota string, corpo any) *httptest.ResponseRecorder {
	t.Helper()
	return a.postDe(t, "192.0.2.10", rota, corpo)
}

// emails devolve as mensagens enviadas a um endereço (espera os envios em andamento).
func (a *amb) emails(para string) []emails.Mensagem {
	a.s.envios.Wait()
	var out []emails.Mensagem
	for _, m := range a.em.Lista() {
		if m.Para == para {
			out = append(out, m)
		}
	}
	return out
}

var reCodigo = regexp.MustCompile(`\n(\d{6})\n`)

// codigo devolve o último código enviado ao endereço.
func (a *amb) codigo(t *testing.T, para string) string {
	t.Helper()
	ms := a.emails(para)
	for i := len(ms) - 1; i >= 0; i-- {
		if m := reCodigo.FindStringSubmatch(ms[i].Texto); m != nil {
			return m[1]
		}
	}
	t.Fatalf("nenhum código enviado para %s", para)
	return ""
}

func par(t *testing.T, w *httptest.ResponseRecorder) sessoes.Par {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, corpo = %s", w.Code, w.Body.String())
	}
	var p sessoes.Par
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || p.Acesso == "" || p.Renovacao == "" {
		t.Fatalf("par inválido: %s (%v)", w.Body.String(), err)
	}
	return p
}

func corpoJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	var m map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("corpo não é JSON: %q", w.Body.String())
	}
	return m
}

func confere(t *testing.T, w *httptest.ResponseRecorder, status int, erro string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d (queria %d), corpo = %s", w.Code, status, w.Body.String())
	}
	if erro != "" {
		if got := corpoJSON(t, w)["erro"]; got != erro {
			t.Fatalf("erro = %q, queria %q", got, erro)
		}
	}
}

// criarConfirmado cria e confirma a conta; devolve o par da confirmação.
func (a *amb) criarConfirmado(t *testing.T, email, senha string) sessoes.Par {
	t.Helper()
	w := a.post(t, "/v1/contas", map[string]string{"email": email, "senha": senha, "termos_versao": termosAtual})
	confere(t, w, http.StatusAccepted, "")
	return par(t, a.post(t, "/v1/contas/confirmar", map[string]string{"senha": senha,
		"email": email, "codigo": a.codigo(t, email), "dispositivo": dispositivo}))
}

func (a *amb) entrar(t *testing.T, email, senha string) *httptest.ResponseRecorder {
	t.Helper()
	return a.post(t, "/v1/contas/entrar", map[string]string{"email": email, "senha": senha, "dispositivo": dispositivo})
}

func TestCriarConfirmarEntrar(t *testing.T) {
	a := novoAmb(t)
	w := a.post(t, "/v1/contas", map[string]string{"email": " Ana@Exemplo.com ", "senha": senhaAna, "termos_versao": termosAtual})
	confere(t, w, http.StatusAccepted, "")
	if m := corpoJSON(t, w)["mensagem"]; m != "Se o email estiver certo, enviamos um código" {
		t.Fatalf("mensagem = %q", m)
	}
	p := par(t, a.post(t, "/v1/contas/confirmar", map[string]string{"senha": senhaAna,
		"email": "ana@exemplo.com", "codigo": a.codigo(t, "ana@exemplo.com"), "dispositivo": dispositivo}))

	var confirmado, aceito *time.Time
	var versao string
	if err := a.db.QueryRow(context.Background(),
		`select email_confirmado_em, termos_aceitos_em, termos_versao from usuarios where email = 'ana@exemplo.com'`).
		Scan(&confirmado, &aceito, &versao); err != nil {
		t.Fatal(err)
	}
	if confirmado == nil || aceito == nil || versao != termosAtual {
		t.Fatalf("confirmado=%v aceito=%v versao=%q", confirmado, aceito, versao)
	}

	p2 := par(t, a.entrar(t, "ANA@exemplo.com", senhaAna))
	p3 := par(t, a.post(t, "/v1/sessao/renovar", map[string]string{"renovacao": p2.Renovacao}))
	w = a.post(t, "/v1/sessao/sair", map[string]string{"renovacao": p3.Renovacao})
	if w.Code != http.StatusNoContent {
		t.Fatalf("sair = %d", w.Code)
	}
	confere(t, a.post(t, "/v1/sessao/renovar", map[string]string{"renovacao": p3.Renovacao}), http.StatusUnauthorized, "sessão inválida")
	// A sessão da confirmação é outra família: segue valendo.
	par(t, a.post(t, "/v1/sessao/renovar", map[string]string{"renovacao": p.Renovacao}))
}

func TestCriarComEmailExistenteNaoRevela(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, "ana@exemplo.com", senhaAna)
	antes := len(a.emails("ana@exemplo.com"))

	wExiste := a.post(t, "/v1/contas", map[string]string{"email": "ana@exemplo.com", "senha": senhaNova, "termos_versao": termosAtual})
	wNovo := a.post(t, "/v1/contas", map[string]string{"email": "bia@exemplo.com", "senha": senhaNova, "termos_versao": termosAtual})
	if wExiste.Code != wNovo.Code || wExiste.Body.String() != wNovo.Body.String() {
		t.Fatalf("respostas diferentes: %d %s / %d %s", wExiste.Code, wExiste.Body, wNovo.Code, wNovo.Body)
	}
	ms := a.emails("ana@exemplo.com")
	if len(ms) != antes+1 || ms[len(ms)-1].Assunto != emails.JaTemConta("").Assunto {
		t.Fatalf("esperava JaTemConta para ana: %+v", ms)
	}
	// A senha da conta confirmada não muda.
	par(t, a.entrar(t, "ana@exemplo.com", senhaAna))
	confere(t, a.entrar(t, "ana@exemplo.com", senhaNova), http.StatusUnauthorized, "Email ou senha incorretos")
}

func TestCriarPendenteTrocaSenha(t *testing.T) {
	a := novoAmb(t)
	a.post(t, "/v1/contas", map[string]string{"email": "ana@exemplo.com", "senha": senhaAna, "termos_versao": termosAtual})
	a.rel.avancar(2 * time.Minute)
	confere(t, a.post(t, "/v1/contas", map[string]string{"email": "ana@exemplo.com", "senha": senhaNova, "termos_versao": termosAtual}),
		http.StatusAccepted, "")
	if n := len(a.emails("ana@exemplo.com")); n != 2 {
		t.Fatalf("esperava 2 códigos, veio %d", n)
	}
	par(t, a.post(t, "/v1/contas/confirmar", map[string]string{"senha": senhaNova,
		"email": "ana@exemplo.com", "codigo": a.codigo(t, "ana@exemplo.com"), "dispositivo": dispositivo}))
	par(t, a.entrar(t, "ana@exemplo.com", senhaNova))
}

func TestEntrarSenhaErrada(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, "ana@exemplo.com", senhaAna)
	wErrada := a.entrar(t, "ana@exemplo.com", senhaErrada)
	confere(t, wErrada, http.StatusUnauthorized, "Email ou senha incorretos")
	wNinguem := a.entrar(t, "ze@exemplo.com", senhaErrada)
	if wNinguem.Code != wErrada.Code || wNinguem.Body.String() != wErrada.Body.String() {
		t.Fatalf("email inexistente responde diferente: %d %s", wNinguem.Code, wNinguem.Body)
	}
}

func TestDezErrosBloqueiamQuinzeMinutos(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, "ana@exemplo.com", senhaAna)
	for i := 1; i <= 10; i++ {
		confere(t, a.entrar(t, "ana@exemplo.com", senhaErrada), http.StatusUnauthorized, "Email ou senha incorretos")
	}
	// 11ª, mesmo com a senha certa: bloqueada sem conferir a senha.
	confere(t, a.entrar(t, "ana@exemplo.com", senhaAna), http.StatusTooManyRequests, "Muitas tentativas. Tente de novo em 15 minutos")
	a.rel.avancar(15*time.Minute + time.Second)
	par(t, a.entrar(t, "ana@exemplo.com", senhaAna))
	var falhas int
	var bloq *time.Time
	if err := a.db.QueryRow(context.Background(),
		`select falhas_login, bloqueado_ate from usuarios where email = 'ana@exemplo.com'`).Scan(&falhas, &bloq); err != nil {
		t.Fatal(err)
	}
	if falhas != 0 || bloq != nil {
		t.Fatalf("senha certa não zerou: falhas=%d bloqueado=%v", falhas, bloq)
	}
}

func TestErrosConcorrentesNaoSePerdem(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, "ana@exemplo.com", senhaAna)
	corpo, _ := json.Marshal(map[string]string{"email": "ana@exemplo.com", "senha": senhaErrada, "dispositivo": dispositivo})
	status := make([]int, 9)
	var wg sync.WaitGroup
	for i := range status {
		wg.Add(1)
		go func(i int) { // sem t aqui: o resultado é conferido depois, na goroutine do teste
			defer wg.Done()
			req := httptest.NewRequest("POST", "/v1/contas/entrar", bytes.NewReader(corpo))
			req.RemoteAddr = fmt.Sprintf("192.0.2.%d:40000", i+1)
			w := httptest.NewRecorder()
			a.s.Handler().ServeHTTP(w, req)
			status[i] = w.Code
		}(i)
	}
	wg.Wait()
	for i, c := range status {
		if c != http.StatusUnauthorized {
			t.Errorf("pedido %d: status %d", i, c)
		}
	}
	var falhas int
	if err := a.db.QueryRow(context.Background(),
		`select falhas_login from usuarios where email = 'ana@exemplo.com'`).Scan(&falhas); err != nil {
		t.Fatal(err)
	}
	if falhas != 9 {
		t.Fatalf("falhas = %d, queria 9", falhas)
	}
}

func TestEntrarSemConfirmarPedeCodigo(t *testing.T) {
	a := novoAmb(t)
	a.post(t, "/v1/contas", map[string]string{"email": "ana@exemplo.com", "senha": senhaAna, "termos_versao": termosAtual})
	a.rel.avancar(2 * time.Minute)
	confere(t, a.entrar(t, "ana@exemplo.com", senhaErrada), http.StatusUnauthorized, "Email ou senha incorretos")
	w := a.entrar(t, "ana@exemplo.com", senhaAna)
	confere(t, w, http.StatusForbidden, "Confirme seu email")
	if m := corpoJSON(t, w)["motivo"]; m != "confirmar" {
		t.Fatalf("motivo = %q", m)
	}
	if n := len(a.emails("ana@exemplo.com")); n != 2 {
		t.Fatalf("esperava código novo (2 emails), veio %d", n)
	}
	par(t, a.post(t, "/v1/contas/confirmar", map[string]string{"senha": senhaAna,
		"email": "ana@exemplo.com", "codigo": a.codigo(t, "ana@exemplo.com"), "dispositivo": dispositivo}))
	par(t, a.entrar(t, "ana@exemplo.com", senhaAna))
}

func TestConfirmarCodigoErrado(t *testing.T) {
	a := novoAmb(t)
	a.post(t, "/v1/contas", map[string]string{"email": "ana@exemplo.com", "senha": senhaAna, "termos_versao": termosAtual})
	cod := a.codigo(t, "ana@exemplo.com")
	errado := "000000"
	if cod == errado {
		errado = "111111"
	}
	confere(t, a.post(t, "/v1/contas/confirmar", map[string]string{"senha": senhaAna, "email": "ana@exemplo.com", "codigo": errado, "dispositivo": dispositivo}),
		http.StatusBadRequest, "Código inválido ou vencido")
	confere(t, a.post(t, "/v1/contas/confirmar", map[string]string{"senha": senhaAna, "email": "ze@exemplo.com", "codigo": errado, "dispositivo": dispositivo}),
		http.StatusBadRequest, "Código inválido ou vencido")
}

func TestRecuperarEmailInexistenteMesmaResposta(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, "ana@exemplo.com", senhaAna)
	antes := len(a.emails("ana@exemplo.com"))
	wAna := a.post(t, "/v1/contas/recuperar", map[string]string{"email": "ana@exemplo.com"})
	wZe := a.post(t, "/v1/contas/recuperar", map[string]string{"email": "ze@exemplo.com"})
	confere(t, wAna, http.StatusAccepted, "")
	if wAna.Code != wZe.Code || wAna.Body.String() != wZe.Body.String() {
		t.Fatalf("respostas diferentes: %s / %s", wAna.Body, wZe.Body)
	}
	if n := len(a.emails("ana@exemplo.com")); n != antes+1 {
		t.Fatalf("ana não recebeu o código de recuperar")
	}
	if n := len(a.emails("ze@exemplo.com")); n != 0 {
		t.Fatalf("email inexistente recebeu %d mensagens", n)
	}
	// Pedido repetido logo em seguida: mesma resposta, sem código novo (não revela a conta).
	confere(t, a.post(t, "/v1/contas/recuperar", map[string]string{"email": "ana@exemplo.com"}), http.StatusAccepted, "")
	if n := len(a.emails("ana@exemplo.com")); n != antes+1 {
		t.Fatalf("mandou código de novo antes de 60 s")
	}
}

func TestNovaSenhaDerrubaSessoes(t *testing.T) {
	a := novoAmb(t)
	p1 := a.criarConfirmado(t, "ana@exemplo.com", senhaAna)
	p2 := par(t, a.entrar(t, "ana@exemplo.com", senhaAna))
	confere(t, a.post(t, "/v1/contas/recuperar", map[string]string{"email": "ana@exemplo.com"}), http.StatusAccepted, "")
	cod := a.codigo(t, "ana@exemplo.com")

	confere(t, a.post(t, "/v1/contas/nova-senha", map[string]string{
		"email": "ana@exemplo.com", "codigo": cod, "senha": "curta", "dispositivo": dispositivo}), http.StatusBadRequest, "")
	p3 := par(t, a.post(t, "/v1/contas/nova-senha", map[string]string{
		"email": "ana@exemplo.com", "codigo": cod, "senha": senhaNova, "dispositivo": dispositivo}))

	for _, p := range []sessoes.Par{p1, p2} {
		confere(t, a.post(t, "/v1/sessao/renovar", map[string]string{"renovacao": p.Renovacao}), http.StatusUnauthorized, "sessão inválida")
	}
	par(t, a.post(t, "/v1/sessao/renovar", map[string]string{"renovacao": p3.Renovacao}))
	confere(t, a.entrar(t, "ana@exemplo.com", senhaAna), http.StatusUnauthorized, "Email ou senha incorretos")
	par(t, a.entrar(t, "ana@exemplo.com", senhaNova))
	ms := a.emails("ana@exemplo.com")
	if ms[len(ms)-1].Assunto != emails.SenhaTrocada("").Assunto {
		t.Fatalf("não avisou a troca de senha: %+v", ms[len(ms)-1])
	}
	// Código já usado não serve de novo.
	confere(t, a.post(t, "/v1/contas/nova-senha", map[string]string{
		"email": "ana@exemplo.com", "codigo": cod, "senha": senhaAna, "dispositivo": dispositivo}),
		http.StatusBadRequest, "Código inválido ou vencido")
}

func TestRenovarReusoEInvalidoIguais(t *testing.T) {
	a := novoAmb(t)
	p := a.criarConfirmado(t, "ana@exemplo.com", senhaAna)
	par(t, a.post(t, "/v1/sessao/renovar", map[string]string{"renovacao": p.Renovacao}))
	wReuso := a.post(t, "/v1/sessao/renovar", map[string]string{"renovacao": p.Renovacao})
	wLixo := a.post(t, "/v1/sessao/renovar", map[string]string{"renovacao": "lixo"})
	confere(t, wReuso, http.StatusUnauthorized, "sessão inválida")
	if wReuso.Body.String() != wLixo.Body.String() || wReuso.Code != wLixo.Code {
		t.Fatalf("reuso e inválido diferentes: %s / %s", wReuso.Body, wLixo.Body)
	}
}

func TestTermosVersaoErrada(t *testing.T) {
	a := novoAmb(t)
	confere(t, a.post(t, "/v1/contas", map[string]string{"email": "ana@exemplo.com", "senha": senhaAna, "termos_versao": "2020-01"}),
		http.StatusBadRequest, "Os termos mudaram")
	confere(t, a.post(t, "/v1/contas", map[string]string{"email": "ana@exemplo.com", "senha": senhaAna}),
		http.StatusBadRequest, "Os termos mudaram")
	var n int
	_ = a.db.QueryRow(context.Background(), `select count(*) from usuarios`).Scan(&n)
	if n != 0 || len(a.emails("ana@exemplo.com")) != 0 {
		t.Fatalf("criou conta ou mandou email com termos errados")
	}
}

func TestTermosMudaramPedeAceite(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, "ana@exemplo.com", senhaAna)
	if _, err := a.db.Exec(context.Background(), `update usuarios set termos_versao = '2025-01'`); err != nil {
		t.Fatal(err)
	}
	a.rel.avancar(time.Hour)

	// Senha errada não revela o 409.
	confere(t, a.entrar(t, "ana@exemplo.com", senhaErrada), http.StatusUnauthorized, "Email ou senha incorretos")
	w := a.entrar(t, "ana@exemplo.com", senhaAna)
	confere(t, w, http.StatusConflict, "Os termos mudaram")
	if c := corpoJSON(t, w); c["motivo"] != "termos" || c["versao"] != termosAtual {
		t.Fatalf("corpo = %v", c)
	}
	par(t, a.post(t, "/v1/contas/entrar", map[string]string{
		"email": "ana@exemplo.com", "senha": senhaAna, "dispositivo": dispositivo, "termos_versao": termosAtual}))
	var versao string
	var aceito time.Time
	if err := a.db.QueryRow(context.Background(),
		`select termos_versao, termos_aceitos_em from usuarios where email = 'ana@exemplo.com'`).Scan(&versao, &aceito); err != nil {
		t.Fatal(err)
	}
	if versao != termosAtual || !aceito.Equal(a.rel.agora()) {
		t.Fatalf("aceite não gravado: %q %v", versao, aceito)
	}
	par(t, a.entrar(t, "ana@exemplo.com", senhaAna))
}

func TestTermosMudaramNaConfirmacao(t *testing.T) {
	a := novoAmb(t)
	a.post(t, "/v1/contas", map[string]string{"email": "ana@exemplo.com", "senha": senhaAna, "termos_versao": termosAtual})
	if _, err := a.db.Exec(context.Background(), `update usuarios set termos_versao = '2025-01'`); err != nil {
		t.Fatal(err)
	}
	cod := a.codigo(t, "ana@exemplo.com")
	// Código errado: 400 normal, sem revelar que a conta existe com termos antigos.
	errado := "000000"
	if cod == errado {
		errado = "111111"
	}
	confere(t, a.post(t, "/v1/contas/confirmar", map[string]string{"senha": senhaAna, "email": "ana@exemplo.com", "codigo": errado, "dispositivo": dispositivo}),
		http.StatusBadRequest, "Código inválido ou vencido")
	a.rel.avancar(2 * time.Minute) // passa dos 60 s: o 409 pode mandar código novo
	w := a.post(t, "/v1/contas/confirmar", map[string]string{"senha": senhaAna, "email": "ana@exemplo.com", "codigo": cod, "dispositivo": dispositivo})
	confere(t, w, http.StatusConflict, "Os termos mudaram")
	// O 409 vem depois do código certo, que foi gasto; junto vai um código novo.
	novo := a.codigo(t, "ana@exemplo.com")
	if novo == cod || len(a.emails("ana@exemplo.com")) != 2 {
		t.Fatal("o 409 deveria mandar um código novo")
	}
	par(t, a.post(t, "/v1/contas/confirmar", map[string]string{"senha": senhaAna,
		"email": "ana@exemplo.com", "codigo": novo, "dispositivo": dispositivo, "termos_versao": termosAtual}))
}

func TestSobrescreverPendenteNaoTomaConta(t *testing.T) {
	a := novoAmb(t)
	const senhaAtacante = "atacante-quer-entrar-31"
	a.post(t, "/v1/contas", map[string]string{"email": "ana@exemplo.com", "senha": senhaAna, "termos_versao": termosAtual})
	a.rel.avancar(2 * time.Minute)
	// O atacante recadastra o email da vítima com a senha dele: troca a senha pendente.
	confere(t, a.post(t, "/v1/contas", map[string]string{"email": "ana@exemplo.com", "senha": senhaAtacante, "termos_versao": termosAtual}),
		http.StatusAccepted, "")
	// A vítima confirma com o código (que chega a ela) e a senha dela: recusado, não cai no hash do atacante.
	confere(t, a.post(t, "/v1/contas/confirmar", map[string]string{"senha": senhaAna,
		"email": "ana@exemplo.com", "codigo": a.codigo(t, "ana@exemplo.com"), "dispositivo": dispositivo}),
		http.StatusUnauthorized, "Email ou senha incorretos")
	// O atacante nunca entra: a conta segue sem confirmar.
	a.rel.avancar(2 * time.Minute)
	confere(t, a.entrar(t, "ana@exemplo.com", senhaAtacante), http.StatusForbidden, "Confirme seu email")
	// A vítima recupera pelo email e fica com a senha dela.
	a.rel.avancar(2 * time.Minute)
	confere(t, a.post(t, "/v1/contas/recuperar", map[string]string{"email": "ana@exemplo.com"}), http.StatusAccepted, "")
	par(t, a.post(t, "/v1/contas/nova-senha", map[string]string{
		"email": "ana@exemplo.com", "codigo": a.codigo(t, "ana@exemplo.com"), "senha": senhaAna, "dispositivo": dispositivo}))
	confere(t, a.entrar(t, "ana@exemplo.com", senhaAtacante), http.StatusUnauthorized, "Email ou senha incorretos")
	par(t, a.entrar(t, "ana@exemplo.com", senhaAna))
}

func TestBloqueioIgualParaEmailInexistente(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, "ana@exemplo.com", senhaAna)
	tentar := func(email string) *httptest.ResponseRecorder {
		var w *httptest.ResponseRecorder
		for i := 1; i <= 11; i++ {
			// IPs diferentes para o limite por IP não interferir.
			w = a.postDe(t, fmt.Sprintf("203.0.113.%d", i), "/v1/contas/entrar",
				map[string]string{"email": email, "senha": senhaErrada, "dispositivo": dispositivo})
			if i <= 10 && w.Code != http.StatusUnauthorized {
				t.Fatalf("%s, tentativa %d: %d %s", email, i, w.Code, w.Body)
			}
		}
		return w
	}
	wAna, wZe := tentar("ana@exemplo.com"), tentar("ze@exemplo.com")
	confere(t, wAna, http.StatusTooManyRequests, "Muitas tentativas. Tente de novo em 15 minutos")
	if wZe.Code != wAna.Code || wZe.Body.String() != wAna.Body.String() {
		t.Fatalf("11ª diferente: existente %d %s / inexistente %d %s", wAna.Code, wAna.Body, wZe.Code, wZe.Body)
	}
	a.rel.avancar(15*time.Minute + time.Second)
	confere(t, a.entrar(t, "ze@exemplo.com", senhaErrada), http.StatusUnauthorized, "Email ou senha incorretos")
}

func TestEsperarEnvios(t *testing.T) {
	a := novoAmb(t)
	a.post(t, "/v1/contas", map[string]string{"email": "ana@exemplo.com", "senha": senhaAna, "termos_versao": termosAtual})
	ctx, cancelar := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelar()
	a.s.Esperar(ctx)
	if len(a.em.Lista()) != 1 {
		t.Fatal("Esperar voltou antes do envio")
	}
}

func TestConfirmarSemSenha(t *testing.T) {
	a := novoAmb(t)
	a.post(t, "/v1/contas", map[string]string{"email": "ana@exemplo.com", "senha": senhaAna, "termos_versao": termosAtual})
	confere(t, a.post(t, "/v1/contas/confirmar", map[string]string{
		"email": "ana@exemplo.com", "codigo": a.codigo(t, "ana@exemplo.com"), "dispositivo": dispositivo}),
		http.StatusUnauthorized, "Email ou senha incorretos")
}

func TestDispositivoInvalido(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, "ana@exemplo.com", senhaAna)
	for _, d := range []string{"", "abc", strings.Repeat("AB", 32), strings.Repeat("g", 64)} {
		w := a.post(t, "/v1/contas/entrar", map[string]string{"email": "ana@exemplo.com", "senha": senhaAna, "dispositivo": d})
		confere(t, w, http.StatusBadRequest, "Dispositivo inválido")
	}
}

func TestLimitePorIP(t *testing.T) {
	a := novoAmb(t)
	for i := 0; i < 20; i++ {
		confere(t, a.postDe(t, "198.51.100.7", "/v1/contas/recuperar", map[string]string{"email": fmt.Sprintf("p%d@exemplo.com", i)}),
			http.StatusAccepted, "")
	}
	confere(t, a.postDe(t, "198.51.100.7", "/v1/contas/entrar", map[string]string{"email": "x@exemplo.com", "senha": senhaAna, "dispositivo": dispositivo}),
		http.StatusTooManyRequests, "Muitas tentativas. Tente de novo em alguns minutos")
	confere(t, a.postDe(t, "198.51.100.8", "/v1/contas/recuperar", map[string]string{"email": "x@exemplo.com"}), http.StatusAccepted, "")
	a.rel.avancar(time.Minute + time.Second)
	confere(t, a.postDe(t, "198.51.100.7", "/v1/contas/recuperar", map[string]string{"email": "y@exemplo.com"}), http.StatusAccepted, "")
}

func TestLimitePorEmail(t *testing.T) {
	a := novoAmb(t)
	for i := 0; i < 5; i++ {
		confere(t, a.postDe(t, fmt.Sprintf("198.51.100.%d", i+1), "/v1/contas/recuperar", map[string]string{"email": "ana@exemplo.com"}),
			http.StatusAccepted, "")
	}
	confere(t, a.postDe(t, "203.0.113.1", "/v1/contas/recuperar", map[string]string{"email": "ANA@exemplo.com"}),
		http.StatusTooManyRequests, "Muitas tentativas. Tente de novo em alguns minutos")
	a.rel.avancar(15*time.Minute + time.Second)
	confere(t, a.postDe(t, "203.0.113.1", "/v1/contas/recuperar", map[string]string{"email": "ana@exemplo.com"}), http.StatusAccepted, "")
}

func TestLogsSemSegredos(t *testing.T) {
	a := novoAmb(t)
	p1 := a.criarConfirmado(t, "ana@exemplo.com", senhaAna)
	segredos := []string{senhaAna, senhaNova, senhaErrada, p1.Acesso, p1.Renovacao, "ana@exemplo.com"}
	cods := []string{a.codigo(t, "ana@exemplo.com")}

	a.entrar(t, "ana@exemplo.com", senhaErrada)
	p2 := par(t, a.entrar(t, "ana@exemplo.com", senhaAna))
	a.post(t, "/v1/contas/recuperar", map[string]string{"email": "ana@exemplo.com"})
	cod := a.codigo(t, "ana@exemplo.com")
	cods = append(cods, cod)
	p3 := par(t, a.post(t, "/v1/contas/nova-senha", map[string]string{
		"email": "ana@exemplo.com", "codigo": cod, "senha": senhaNova, "dispositivo": dispositivo}))
	p4 := par(t, a.post(t, "/v1/sessao/renovar", map[string]string{"renovacao": p3.Renovacao}))
	a.post(t, "/v1/sessao/renovar", map[string]string{"renovacao": p2.Renovacao})
	a.post(t, "/v1/contas", map[string]string{"email": "ana@exemplo.com", "senha": senhaAna, "termos_versao": termosAtual})
	a.s.envios.Wait()
	segredos = append(segredos, p2.Acesso, p2.Renovacao, p3.Acesso, p3.Renovacao, p4.Acesso, p4.Renovacao)

	log := a.log.String()
	if !strings.Contains(log, "a***@exemplo.com") {
		t.Fatalf("o log deveria registrar o email mascarado: %s", log)
	}
	for _, s := range segredos {
		if strings.Contains(log, s) {
			t.Errorf("log contém segredo %q", s)
		}
	}
	for _, c := range cods {
		if strings.Contains(log, c) {
			t.Errorf("log contém o código %q", c)
		}
	}
}

func TestMascararEmail(t *testing.T) {
	for in, quer := range map[string]string{
		"ana@exemplo.com": "a***@exemplo.com",
		"a@b.co":          "a***@b.co",
		"semarroba":       "***",
		"":                "***",
	} {
		if got := mascarar(in); got != quer {
			t.Errorf("mascarar(%q) = %q, queria %q", in, got, quer)
		}
	}
}

// Todos os pedidos de código do mesmo email (criar, entrar pendente) dividem o orçamento: acima dele, a
// resposta é a mesma (R8) e nenhum email sai.
func TestOrcamentoDeEnvioPorEmail(t *testing.T) {
	a := novoAmb(t)
	confere(t, a.post(t, "/v1/contas", map[string]string{"email": "ana@exemplo.com", "senha": senhaAna, "termos_versao": termosAtual}),
		http.StatusAccepted, "")
	for i := 2; i <= 6; i++ {
		a.rel.avancar(61 * time.Second)
		w := a.postDe(t, fmt.Sprintf("198.51.100.%d", i), "/v1/contas/entrar",
			map[string]string{"email": "ana@exemplo.com", "senha": senhaAna, "dispositivo": dispositivo})
		confere(t, w, http.StatusForbidden, "Confirme seu email")
	}
	if n := len(a.emails("ana@exemplo.com")); n != 5 {
		t.Fatalf("esperava 5 códigos em 15 min, veio %d", n)
	}
	a.rel.avancar(61 * time.Second)
	w := a.post(t, "/v1/contas", map[string]string{"email": "ana@exemplo.com", "senha": senhaAna, "termos_versao": termosAtual})
	confere(t, w, http.StatusAccepted, "")
	if n := len(a.emails("ana@exemplo.com")); n != 5 {
		t.Fatalf("acima do orçamento não deveria mandar código: %d", n)
	}
	a.rel.avancar(15 * time.Minute)
	confere(t, a.entrar(t, "ana@exemplo.com", senhaAna), http.StatusForbidden, "Confirme seu email")
	if n := len(a.emails("ana@exemplo.com")); n != 6 {
		t.Fatalf("depois de 15 min deveria mandar de novo: %d", n)
	}
}

// Tentativas simultâneas não passam do bloqueio: cada uma reserva a vez antes de conferir a senha, então
// no máximo 10 senhas são conferidas e as outras levam 429 (vale também para email sem conta, R11).
func TestTentativasConcorrentesRespeitamBloqueio(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, "ana@exemplo.com", senhaAna)
	for _, email := range []string{"ana@exemplo.com", "ze@exemplo.com"} {
		corpo, _ := json.Marshal(map[string]string{"email": email, "senha": senhaErrada, "dispositivo": dispositivo})
		status := make([]int, 30)
		var wg sync.WaitGroup
		for i := range status {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				req := httptest.NewRequest("POST", "/v1/contas/entrar", bytes.NewReader(corpo))
				req.RemoteAddr = fmt.Sprintf("203.0.113.%d:40000", i+1)
				w := httptest.NewRecorder()
				a.s.Handler().ServeHTTP(w, req)
				status[i] = w.Code
			}(i)
		}
		wg.Wait()
		conferidas, bloqueadas := 0, 0
		for i, c := range status {
			switch c {
			case http.StatusUnauthorized:
				conferidas++
			case http.StatusTooManyRequests:
				bloqueadas++
			default:
				t.Errorf("%s, pedido %d: status %d", email, i, c)
			}
		}
		if conferidas > contas.MaxFalhas || conferidas+bloqueadas != 30 {
			t.Fatalf("%s: %d senhas conferidas (máximo %d), %d bloqueadas", email, conferidas, contas.MaxFalhas, bloqueadas)
		}
	}
	// A conta ficou bloqueada: nem a senha certa entra.
	confere(t, a.postDe(t, "198.51.100.200", "/v1/contas/entrar",
		map[string]string{"email": "ana@exemplo.com", "senha": senhaAna, "dispositivo": dispositivo}),
		http.StatusTooManyRequests, "Muitas tentativas. Tente de novo em 15 minutos")
}

// Sem vaga para o Argon2 (pedidos demais ao mesmo tempo), as rotas que resumem senha respondem 503,
// igual para email com e sem conta.
func TestSenhasOcupadas503(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, "ana@exemplo.com", senhaAna)
	defer senhas.TrocarLimite(0, 10*time.Millisecond)()
	const msg = "Serviço ocupado. Tente de novo em instantes"
	confere(t, a.entrar(t, "ana@exemplo.com", senhaAna), http.StatusServiceUnavailable, msg)
	confere(t, a.entrar(t, "ze@exemplo.com", senhaAna), http.StatusServiceUnavailable, msg)
	confere(t, a.post(t, "/v1/contas", map[string]string{"email": "bia@exemplo.com", "senha": senhaAna, "termos_versao": termosAtual}),
		http.StatusServiceUnavailable, msg)
}

// recuperar responde sem esperar o banco nem o email (o tempo não revela se a conta existe): com a geração
// do código travada, a resposta sai mesmo assim e o código chega depois.
func TestRecuperarEmSegundoPlano(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, "ana@exemplo.com", senhaAna)
	ctx := context.Background()
	tx, err := a.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	// A mesma trava que codigos.Gerar pega para o email+tipo.
	if _, err := tx.Exec(ctx, `select pg_advisory_xact_lock(hashtext('ana@exemplo.com|recuperar'))`); err != nil {
		t.Fatal(err)
	}
	resp := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		req := httptest.NewRequest("POST", "/v1/contas/recuperar", strings.NewReader(`{"email":"ana@exemplo.com"}`))
		req.RemoteAddr = "192.0.2.10:40000"
		w := httptest.NewRecorder()
		a.s.Handler().ServeHTTP(w, req)
		resp <- w
	}()
	select {
	case w := <-resp:
		confere(t, w, http.StatusAccepted, "")
	case <-time.After(3 * time.Second):
		tx.Rollback(ctx)
		<-resp
		t.Fatal("recuperar esperou a geração do código para responder")
	}
	antes := len(a.em.Lista())
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if n := len(a.emails("ana@exemplo.com")); n != antes+1 {
		t.Fatalf("o código não chegou depois: %d emails", n)
	}
}
