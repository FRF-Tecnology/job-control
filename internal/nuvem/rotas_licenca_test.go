package nuvem

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/licenca"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/contas"
)

const (
	emailLic  = "ana@exemplo.com"
	emailLic2 = "bia@exemplo.com"
)

var (
	pcA = strings.Repeat("a1", 32)
	pcB = strings.Repeat("b2", 32)
)

// licencaDe entra de novo (o acesso dura pouco) e pede GET /v1/licenca para o computador dado.
func (a *amb) licencaDe(t *testing.T, email, senha, pc string) *httptest.ResponseRecorder {
	t.Helper()
	p := par(t, a.post(t, "/v1/contas/entrar", map[string]string{"email": email, "senha": senha, "dispositivo": dispositivo}))
	return a.getLicenca(pc, p.Acesso)
}

func (a *amb) getLicenca(pc, acesso string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/v1/licenca?dispositivo="+pc, nil)
	req.RemoteAddr = "192.0.2.10:40000"
	if acesso != "" {
		req.Header.Set("Authorization", "Bearer "+acesso)
	}
	w := httptest.NewRecorder()
	a.s.Handler().ServeHTTP(w, req)
	return w
}

// lic confere a assinatura com a chave pública e devolve a licença.
func (a *amb) lic(t *testing.T, w *httptest.ResponseRecorder) licenca.Licenca {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, corpo = %s", w.Code, w.Body.String())
	}
	var d licenca.Documento
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	priv, err := a.s.cfg.Chave()
	if err != nil {
		t.Fatal(err)
	}
	l, err := licenca.Conferir(priv.Public().(ed25519.PublicKey), d, a.rel.agora())
	if err != nil {
		t.Fatalf("licença não confere: %v", err)
	}
	return l
}

func (a *amb) contar(t *testing.T, tabela string) int {
	t.Helper()
	var n int
	if err := a.db.QueryRow(context.Background(), `select count(*) from `+tabela).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestPrimeiraLicencaComecaTeste(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, emailLic, senhaAna)
	t0 := a.rel.agora()
	l := a.lic(t, a.licencaDe(t, emailLic, senhaAna, pcA))
	if l.Estado != licenca.Teste {
		t.Fatalf("estado = %s", l.Estado)
	}
	if l.TesteFim == nil || !l.TesteFim.Equal(t0.Add(24*time.Hour)) {
		t.Fatalf("teste_fim = %v", l.TesteFim)
	}
	if !l.EmitidaEm.Equal(t0) || !l.ValidaAte.Equal(t0.Add(30*time.Minute)) {
		t.Fatalf("emitida %v valida_ate %v", l.EmitidaEm, l.ValidaAte)
	}
	if l.Email != emailLic || l.Usuario != a.usuario(t, emailLic).ID {
		t.Fatalf("identidade errada: %+v", l)
	}
	// Chamar de novo não estende nem recomeça.
	a.rel.avancar(time.Hour)
	l = a.lic(t, a.licencaDe(t, emailLic, senhaAna, pcA))
	if !l.TesteFim.Equal(t0.Add(24 * time.Hour)) {
		t.Fatalf("teste_fim mudou: %v", l.TesteFim)
	}
}

func TestTesteAcaba(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, emailLic, senhaAna)
	a.lic(t, a.licencaDe(t, emailLic, senhaAna, pcA))
	a.rel.avancar(24*time.Hour - time.Second)
	if l := a.lic(t, a.licencaDe(t, emailLic, senhaAna, pcA)); l.Estado != licenca.Teste {
		t.Fatalf("aos 23h59m59s ainda é teste, veio %s", l.Estado)
	}
	a.rel.avancar(2 * time.Second)
	if l := a.lic(t, a.licencaDe(t, emailLic, senhaAna, pcA)); l.Estado != licenca.Bloqueado {
		t.Fatalf("estado = %s", l.Estado)
	}
}

func TestSegundaContaMesmoPCJaBloqueada(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, emailLic, senhaAna)
	a.criarConfirmado(t, emailLic2, senhaAna)
	a.lic(t, a.licencaDe(t, emailLic, senhaAna, pcA))
	if l := a.lic(t, a.licencaDe(t, emailLic2, senhaAna, pcA)); l.Estado != licenca.Bloqueado {
		t.Fatalf("estado = %s", l.Estado)
	}
	// O teste não volta se a conta tentar de outro PC depois.
	if l := a.lic(t, a.licencaDe(t, emailLic2, senhaAna, pcB)); l.Estado != licenca.Bloqueado {
		t.Fatalf("estado = %s", l.Estado)
	}
}

func TestMesmaContaOutroPCNaoGanhaOutroDia(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, emailLic, senhaAna)
	t0 := a.rel.agora()
	a.lic(t, a.licencaDe(t, emailLic, senhaAna, pcA))
	a.rel.avancar(2 * time.Hour)
	l := a.lic(t, a.licencaDe(t, emailLic, senhaAna, pcB))
	if l.Estado != licenca.Teste || !l.TesteFim.Equal(t0.Add(24*time.Hour)) {
		t.Fatalf("estado %s fim %v", l.Estado, l.TesteFim)
	}
	if n := a.contar(t, "computadores_teste"); n != 1 {
		t.Fatalf("computadores_teste = %d", n)
	}
}

func TestEmailNaoConfirmadoNaoComecaTeste(t *testing.T) {
	a := novoAmb(t)
	a.post(t, "/v1/contas", map[string]string{"email": emailLic, "senha": senhaAna, "termos_versao": termosAtual})
	u := a.usuario(t, emailLic)
	p, err := a.s.sessoes.Abrir(context.Background(), u.ID, dispositivo)
	if err != nil {
		t.Fatal(err)
	}
	if l := a.lic(t, a.getLicenca(pcA, p.Acesso)); l.Estado != licenca.Bloqueado {
		t.Fatalf("estado = %s", l.Estado)
	}
	if a.contar(t, "computadores_teste") != 0 || a.usuario(t, emailLic).TesteInicio != nil {
		t.Fatal("o teste não podia começar")
	}
}

func TestCortesia(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, emailLic, senhaAna)
	a.lic(t, a.licencaDe(t, emailLic, senhaAna, pcA))
	if _, err := a.db.Exec(context.Background(), `update usuarios set cortesia = true where email = $1`, emailLic); err != nil {
		t.Fatal(err)
	}
	a.rel.avancar(10 * 24 * time.Hour)
	if l := a.lic(t, a.licencaDe(t, emailLic, senhaAna, pcA)); l.Estado != licenca.Assinante {
		t.Fatalf("estado = %s", l.Estado)
	}
}

func (a *amb) assinatura(t *testing.T, email, estado string, fim *time.Time) {
	t.Helper()
	u := a.usuario(t, email)
	if _, err := a.db.Exec(context.Background(),
		`insert into assinaturas (usuario_id, estado, periodo_fim) values ($1::uuid, $2, $3)
		 on conflict (usuario_id) do update set estado = $2, periodo_fim = $3`, u.ID, estado, fim); err != nil {
		t.Fatal(err)
	}
}

func TestAssinaturaAtiva(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, emailLic, senhaAna)
	a.lic(t, a.licencaDe(t, emailLic, senhaAna, pcA))
	fim := a.rel.agora().Add(30 * 24 * time.Hour)
	for _, est := range []string{"active", "trialing"} {
		a.assinatura(t, emailLic, est, &fim)
		a.rel.avancar(48 * time.Hour) // passa do teste
		l := a.lic(t, a.licencaDe(t, emailLic, senhaAna, pcA))
		if l.Estado != licenca.Assinante || l.PeriodoFim == nil || !l.PeriodoFim.Equal(fim) {
			t.Fatalf("%s: estado %s periodo_fim %v", est, l.Estado, l.PeriodoFim)
		}
	}
	a.assinatura(t, emailLic, "canceled", &fim)
	if l := a.lic(t, a.licencaDe(t, emailLic, senhaAna, pcA)); l.Estado != licenca.Bloqueado {
		t.Fatalf("cancelada: estado %s", l.Estado)
	}
}

func TestPastDueTresDias(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, emailLic, senhaAna)
	a.lic(t, a.licencaDe(t, emailLic, senhaAna, pcA))
	fim := a.rel.agora().Add(48 * time.Hour)
	a.assinatura(t, emailLic, "past_due", &fim)
	a.rel.avancar(48*time.Hour + 72*time.Hour)
	if l := a.lic(t, a.licencaDe(t, emailLic, senhaAna, pcA)); l.Estado != licenca.Assinante {
		t.Fatalf("+72h: estado %s", l.Estado)
	}
	a.rel.avancar(time.Second)
	if l := a.lic(t, a.licencaDe(t, emailLic, senhaAna, pcA)); l.Estado != licenca.Bloqueado {
		t.Fatalf("+72h1s: estado %s", l.Estado)
	}
}

func TestPastDueSemPeriodoBloqueia(t *testing.T) {
	if got := estadoDa(contas.Usuario{}, &Assinatura{Estado: "past_due"}, time.Now()); got != licenca.Bloqueado {
		t.Fatalf("estado = %s", got)
	}
}

func TestLicencaConfereComChavePublica(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, emailLic, senhaAna)
	w := a.licencaDe(t, emailLic, senhaAna, pcA)
	var d licenca.Documento
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
		t.Fatal(err)
	}
	priv, _ := a.s.cfg.Chave()
	pub := priv.Public().(ed25519.PublicKey)
	if _, err := licenca.Conferir(pub, d, a.rel.agora()); err != nil {
		t.Fatal(err)
	}
	if _, err := licenca.Conferir(pub, d, a.rel.agora().Add(31*time.Minute)); err != licenca.ErrVencida {
		t.Fatalf("err = %v", err)
	}
	outra, _, _ := ed25519.GenerateKey(rand.Reader)
	if _, err := licenca.Conferir(outra, d, a.rel.agora()); err != licenca.ErrAssinatura {
		t.Fatalf("err = %v", err)
	}
}

func TestLicencaDispositivoInvalido(t *testing.T) {
	a := novoAmb(t)
	p := a.criarConfirmado(t, emailLic, senhaAna)
	for _, pc := range []string{"", "xyz", strings.Repeat("A", 64), strings.Repeat("a", 63), strings.Repeat("a", 65)} {
		confere(t, a.getLicenca(pc, p.Acesso), http.StatusBadRequest, msgDispositivo)
	}
	if a.usuario(t, emailLic).TesteInicio != nil {
		t.Fatal("teste não podia começar")
	}
}

func TestLicencaSemSessao(t *testing.T) {
	a := novoAmb(t)
	confere(t, a.getLicenca(pcA, ""), http.StatusUnauthorized, "")
	confere(t, a.getLicenca(pcA, "lixo"), http.StatusUnauthorized, "")
}

func TestLicencaConcorrenteComecaUmaVez(t *testing.T) {
	a := novoAmb(t)
	p := a.criarConfirmado(t, emailLic, senhaAna)
	t0 := a.rel.agora()
	done := make(chan *httptest.ResponseRecorder, 8)
	for i := 0; i < 8; i++ {
		go func() { done <- a.getLicenca(pcA, p.Acesso) }()
	}
	for i := 0; i < 8; i++ {
		l := a.lic(t, <-done)
		if l.Estado != licenca.Teste || !l.TesteFim.Equal(t0.Add(24*time.Hour)) {
			t.Fatalf("estado %s fim %v", l.Estado, l.TesteFim)
		}
	}
	if n := a.contar(t, "computadores_teste"); n != 1 {
		t.Fatalf("computadores_teste = %d", n)
	}
}

func TestLicencaNaoLogaDispositivoNemEmail(t *testing.T) {
	a := novoAmb(t)
	a.criarConfirmado(t, emailLic, senhaAna)
	a.lic(t, a.licencaDe(t, emailLic, senhaAna, pcA))
	log := a.log.String()
	if strings.Contains(log, pcA) || strings.Contains(log, emailLic) {
		t.Fatalf("log vaza dado: %s", log)
	}
}
