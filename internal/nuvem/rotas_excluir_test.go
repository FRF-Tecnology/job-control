package nuvem

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/licenca"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/pagamento"
)

const emailExcluir = "ana@exemplo.com"

func (a *amb) excluir(corpo any, acesso string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(corpo)
	req := httptest.NewRequest("DELETE", "/v1/contas/eu", bytes.NewReader(b))
	req.RemoteAddr = "192.0.2.10:40000"
	if acesso != "" {
		req.Header.Set("Authorization", "Bearer "+acesso)
	}
	w := httptest.NewRecorder()
	a.s.Handler().ServeHTTP(w, req)
	return w
}

func (a *amb) getAuth(rota, acesso string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", rota, nil)
	req.RemoteAddr = "192.0.2.10:40000"
	req.Header.Set("Authorization", "Bearer "+acesso)
	w := httptest.NewRecorder()
	a.s.Handler().ServeHTTP(w, req)
	return w
}

// contarOnde conta as linhas de uma tabela que casam com a condição.
func (a *amb) contarOnde(t *testing.T, tabela, cond string, args ...any) int {
	t.Helper()
	var n int
	if err := a.db.QueryRow(context.Background(), `select count(*) from `+tabela+` where `+cond, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// deixarCodigo deixa um código de recuperação pendente para o email (para provar que é apagado).
func (a *amb) deixarCodigo(t *testing.T) {
	t.Helper()
	confere(t, a.post(t, "/v1/contas/recuperar", map[string]string{"email": emailExcluir}), http.StatusAccepted, "")
	a.s.Esperar(context.Background())
	if a.contarOnde(t, "codigos", "email = $1", emailExcluir) == 0 {
		t.Fatal("sem códigos para apagar")
	}
}

func TestExcluirApagaTudo(t *testing.T) {
	a := novoAmb(t)
	p := a.criarConfirmado(t, emailExcluir, senhaAna)
	id := a.usuario(t, emailExcluir).ID
	a.deixarCodigo(t)
	confere(t, a.getLicenca(pcA, p.Acesso), http.StatusOK, "") // começa o teste e registra o PC
	a.assinatura(t, emailExcluir, "canceled", nil)
	if _, err := a.db.Exec(context.Background(), `insert into notas (invoice_id, usuario_id, estado) values ('in_1', $1::uuid, 'emitida')`, id); err != nil {
		t.Fatal(err)
	}

	confere(t, a.excluir(map[string]string{"email": " Ana@Exemplo.com "}, p.Acesso), http.StatusNoContent, "")

	if n := a.contarOnde(t, "usuarios", "id = $1::uuid", id); n != 0 {
		t.Errorf("usuarios: %d", n)
	}
	if n := a.contarOnde(t, "sessoes", "usuario_id = $1::uuid", id); n != 0 {
		t.Errorf("sessoes: %d", n)
	}
	if n := a.contarOnde(t, "assinaturas", "usuario_id = $1::uuid", id); n != 0 {
		t.Errorf("assinaturas: %d", n)
	}
	if n := a.contarOnde(t, "codigos", "email = $1", emailExcluir); n != 0 {
		t.Errorf("codigos: %d", n)
	}
	// O PC do teste continua registrado, sem dono; a nota fica com o id antigo.
	if n := a.contarOnde(t, "computadores_teste", "hash = $1 and usuario_id is null", pcA); n != 1 {
		t.Errorf("computadores_teste sem dono = %d, quer 1", n)
	}
	if n := a.contarOnde(t, "notas", "invoice_id = 'in_1' and usuario_id = $1::uuid", id); n != 1 {
		t.Errorf("a nota sumiu ou perdeu o dono (%d)", n)
	}
	if strings.Contains(a.log.String(), emailExcluir) {
		t.Error("o log levou o email inteiro")
	}
}

func TestExcluirEmailErrado400(t *testing.T) {
	a := novoAmb(t)
	p := a.criarConfirmado(t, emailExcluir, senhaAna)
	for _, corpo := range []map[string]string{{"email": "outra@exemplo.com"}, {"email": ""}, {}} {
		confere(t, a.excluir(corpo, p.Acesso), http.StatusBadRequest, "Confirme digitando o email da conta")
	}
	if a.contarOnde(t, "usuarios", "email = $1", emailExcluir) != 1 {
		t.Fatal("a conta foi apagada")
	}
	confere(t, a.excluir(map[string]string{"email": emailExcluir}, ""), http.StatusUnauthorized, msgSessao)
}

func TestExcluirCancelaStripe(t *testing.T) {
	a := novoAmb(t)
	id, cliente, acesso := a.assinante(t, emailExcluir)
	fim := a.rel.agora().Add(30 * 24 * time.Hour)
	a.st.DefinirAssinatura(pagamento.EstadoStripe{ID: "sub_1", Status: "active", Cliente: cliente, PeriodoFim: &fim,
		Metadata: map[string]string{"usuario_id": id}})
	confere(t, a.webhook(t, "evt_1", "customer.subscription.created", map[string]any{"id": "sub_1", "object": "subscription"}), http.StatusOK, "")

	confere(t, a.excluir(map[string]string{"email": emailExcluir}, acesso), http.StatusNoContent, "")
	if c := a.st.Canceladas(); len(c) != 1 || c[0] != "sub_1" {
		t.Fatalf("canceladas = %v", c)
	}
	if a.contarOnde(t, "usuarios", "id = $1::uuid", id) != 0 {
		t.Fatal("a conta não foi apagada")
	}
}

func TestExcluirSemAssinaturaVigenteNaoChamaStripe(t *testing.T) {
	a := novoAmb(t)
	p := a.criarConfirmado(t, emailExcluir, senhaAna)
	a.assinatura(t, emailExcluir, "canceled", nil)
	confere(t, a.excluir(map[string]string{"email": emailExcluir}, p.Acesso), http.StatusNoContent, "")
	if c := a.st.Canceladas(); len(c) != 0 {
		t.Fatalf("canceladas = %v", c)
	}
}

func TestExcluirFalhaStripeNaoApaga(t *testing.T) {
	a := novoAmb(t)
	id, _, acesso := a.assinante(t, emailExcluir)
	// Vigente no nosso banco, mas o Stripe falso não a conhece: Cancelar falha.
	if _, err := a.db.Exec(context.Background(),
		`insert into assinaturas (usuario_id, stripe_assinatura, estado) values ($1::uuid, 'sub_some', 'active')`, id); err != nil {
		t.Fatal(err)
	}
	confere(t, a.excluir(map[string]string{"email": emailExcluir}, acesso), http.StatusBadGateway,
		"Não deu para cancelar a assinatura agora. Tente de novo.")
	if a.contarOnde(t, "usuarios", "id = $1::uuid", id) != 1 || a.contarOnde(t, "assinaturas", "usuario_id = $1::uuid", id) != 1 {
		t.Fatal("algo foi apagado apesar da falha do Stripe")
	}
	if a.contarOnde(t, "sessoes", "usuario_id = $1::uuid and revogada_em is null", id) == 0 {
		t.Fatal("as sessões foram revogadas apesar da falha")
	}
}

func TestExcluirERecriarSemNovoTeste(t *testing.T) {
	a := novoAmb(t)
	p := a.criarConfirmado(t, emailExcluir, senhaAna)
	if l := a.lic(t, a.getLicenca(pcA, p.Acesso)); l.Estado != licenca.Teste {
		t.Fatalf("estado = %s, quer teste", l.Estado)
	}
	confere(t, a.excluir(map[string]string{"email": emailExcluir}, p.Acesso), http.StatusNoContent, "")

	p2 := a.criarConfirmado(t, emailExcluir, senhaAna)
	if l := a.lic(t, a.getLicenca(pcA, p2.Acesso)); l.Estado != licenca.Bloqueado {
		t.Fatalf("estado = %s: recriar a conta deu outro teste no mesmo PC", l.Estado)
	}
}

func TestRotasDeContaExcluida401(t *testing.T) {
	a := novoAmb(t)
	p := a.criarConfirmado(t, emailExcluir, senhaAna)
	confere(t, a.excluir(map[string]string{"email": emailExcluir}, p.Acesso), http.StatusNoContent, "")
	// O token ainda confere por até 15 min; as rotas que carregam o usuário têm de responder 401, não 500.
	confere(t, a.getLicenca(pcA, p.Acesso), http.StatusUnauthorized, msgSessao)
	confere(t, a.postAuth("/v1/assinar", p.Acesso), http.StatusUnauthorized, msgSessao)
	confere(t, a.postAuth("/v1/portal", p.Acesso), http.StatusUnauthorized, msgSessao)
	confere(t, a.excluir(map[string]string{"email": emailExcluir}, p.Acesso), http.StatusUnauthorized, msgSessao)
	if r := a.getAuth("/v1/notas", p.Acesso); r.Code != http.StatusOK {
		t.Fatalf("notas = %d", r.Code)
	}
}

func TestExcluirLimitaPorIP(t *testing.T) {
	a := novoAmb(t)
	p := a.criarConfirmado(t, emailExcluir, senhaAna)
	var ultimo *httptest.ResponseRecorder
	for i := 0; i < limitePorIP+2; i++ {
		ultimo = a.excluir(map[string]string{"email": "outra@exemplo.com"}, p.Acesso)
	}
	if ultimo.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d", ultimo.Code)
	}
}
