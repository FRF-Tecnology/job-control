package nuvem

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v87"

	"github.com/FRF-Tecnology/job-control/internal/licenca"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/pagamento"
)

const segredoWebhook = "whsec_ficticio_dos_testes"

// notasFalsas guarda as chamadas a Agendar.
type notasFalsas struct {
	mu       sync.Mutex
	chamadas [][2]string // invoiceID, usuarioID
}

func (n *notasFalsas) Agendar(_ context.Context, invoiceID, usuarioID string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.chamadas = append(n.chamadas, [2]string{invoiceID, usuarioID})
	return nil
}

func (n *notasFalsas) lista() [][2]string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([][2]string(nil), n.chamadas...)
}

// postAuth faz um POST autenticado sem corpo.
func (a *amb) postAuth(rota, acesso string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", rota, nil)
	req.RemoteAddr = "192.0.2.10:40000"
	if acesso != "" {
		req.Header.Set("Authorization", "Bearer "+acesso)
	}
	w := httptest.NewRecorder()
	a.s.Handler().ServeHTTP(w, req)
	return w
}

// webhookCru manda o corpo com o cabeçalho dado.
func (a *amb) webhookCru(corpo []byte, cabecalho string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/v1/stripe/webhook", bytes.NewReader(corpo))
	if cabecalho != "" {
		req.Header.Set("Stripe-Signature", cabecalho)
	}
	w := httptest.NewRecorder()
	a.s.Handler().ServeHTTP(w, req)
	return w
}

// webhook monta um evento do Stripe (modo de teste), assina com o segredo dos testes e manda.
func (a *amb) webhook(t *testing.T, id, tipo string, objeto map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return a.webhookModo(t, id, tipo, objeto, false)
}

// webhookModo é webhook com o livemode do evento escolhido.
func (a *amb) webhookModo(t *testing.T, id, tipo string, objeto map[string]any, livemode bool) *httptest.ResponseRecorder {
	t.Helper()
	corpo, err := json.Marshal(map[string]any{
		"id": id, "object": "event", "type": tipo, "api_version": stripe.APIVersion, "livemode": livemode,
		"created": time.Now().Unix(), "data": map[string]any{"object": objeto},
	})
	if err != nil {
		t.Fatal(err)
	}
	sp := stripe.GenerateTestSignedPayload(&stripe.UnsignedPayload{Payload: corpo, Secret: segredoWebhook, Timestamp: time.Now()})
	return a.webhookCru(corpo, sp.Header)
}

func urlDe(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	confere(t, w, http.StatusOK, "")
	u := corpoJSON(t, w)["url"]
	if !strings.HasPrefix(u, "https://") {
		t.Fatalf("url = %q", u)
	}
	return u
}

func (a *amb) stripeCliente(t *testing.T, usuarioID string) string {
	t.Helper()
	var c *string
	if err := a.db.QueryRow(context.Background(), `select stripe_cliente from usuarios where id = $1::uuid`, usuarioID).Scan(&c); err != nil {
		t.Fatal(err)
	}
	if c == nil {
		return ""
	}
	return *c
}

func (a *amb) lerAss(t *testing.T, usuarioID string) *Assinatura {
	t.Helper()
	as, err := lerAssinatura(context.Background(), a.db, usuarioID)
	if err != nil {
		t.Fatal(err)
	}
	return as
}

// assinante cria a conta confirmada, assina (cliente no Stripe falso) e devolve id, cliente e acesso.
func (a *amb) assinante(t *testing.T, email string) (usuarioID, cliente, acesso string) {
	t.Helper()
	p := a.criarConfirmado(t, email, senhaAna)
	urlDe(t, a.postAuth("/v1/assinar", p.Acesso))
	usuarioID = a.usuario(t, email).ID
	return usuarioID, a.stripeCliente(t, usuarioID), p.Acesso
}

func TestAssinarCriaClienteUmaVez(t *testing.T) {
	a := novoAmb(t)
	if w := a.postAuth("/v1/assinar", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("sem sessão = %d", w.Code)
	}
	p := a.criarConfirmado(t, emailLic, senhaAna)
	u1 := urlDe(t, a.postAuth("/v1/assinar", p.Acesso))
	u2 := urlDe(t, a.postAuth("/v1/assinar", p.Acesso))
	if n := a.st.Clientes(); n != 1 {
		t.Fatalf("clientes criados no Stripe = %d", n)
	}
	id := a.usuario(t, emailLic).ID
	cliente := a.stripeCliente(t, id)
	if cliente == "" {
		t.Fatal("stripe_cliente não foi gravado")
	}
	if !strings.Contains(u1, cliente) || u1 != u2 {
		t.Fatalf("checkout com cliente errado: %q %q (cliente %q)", u1, u2, cliente)
	}
	if got := a.st.Checkouts(); len(got) != 2 || got[0] != [2]string{cliente, id} {
		t.Fatalf("checkouts = %v", got)
	}
	if strings.Contains(a.log.String(), emailLic) {
		t.Fatal("o log levou o email inteiro")
	}
}

func TestAssinarJaAssinante409(t *testing.T) {
	a := novoAmb(t)
	id, cliente, acesso := a.assinante(t, emailLic)
	fim := a.rel.agora().Add(30 * 24 * time.Hour)
	a.st.DefinirAssinatura(pagamento.EstadoStripe{ID: "sub_1", Status: "active", Cliente: cliente, PeriodoFim: &fim,
		Metadata: map[string]string{"usuario_id": id}})
	confere(t, a.webhook(t, "evt_1", "customer.subscription.created", map[string]any{"id": "sub_1", "object": "subscription"}), http.StatusOK, "")
	confere(t, a.postAuth("/v1/assinar", acesso), http.StatusConflict, msgJaAssinante)
	urlDe(t, a.postAuth("/v1/portal", acesso))
}

func TestPortalSemCliente409(t *testing.T) {
	a := novoAmb(t)
	p := a.criarConfirmado(t, emailLic, senhaAna)
	confere(t, a.postAuth("/v1/portal", p.Acesso), http.StatusConflict, msgSemAssinatura)
	urlDe(t, a.postAuth("/v1/assinar", p.Acesso))
	u := urlDe(t, a.postAuth("/v1/portal", p.Acesso))
	if !strings.Contains(u, a.stripeCliente(t, a.usuario(t, emailLic).ID)) {
		t.Fatalf("portal de outro cliente: %q", u)
	}
	if w := a.postAuth("/v1/portal", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("sem sessão = %d", w.Code)
	}
}

func TestWebhookAssinaturaInvalida(t *testing.T) {
	a := novoAmb(t)
	corpo := []byte(`{"id":"evt_x","object":"event","type":"invoice.paid","data":{"object":{"id":"in_1"}}}`)
	outro := stripe.GenerateTestSignedPayload(&stripe.UnsignedPayload{Payload: corpo, Secret: "whsec_outro", Timestamp: time.Now()})
	velho := stripe.GenerateTestSignedPayload(&stripe.UnsignedPayload{Payload: corpo, Secret: segredoWebhook, Timestamp: time.Now().Add(-time.Hour)})
	for nome, cab := range map[string]string{"sem cabeçalho": "", "segredo errado": outro.Header, "vencido": velho.Header, "lixo": "t=1,v1=zz"} {
		if w := a.webhookCru(corpo, cab); w.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d", nome, w.Code)
		}
	}
	if n := a.contar(t, "eventos_stripe"); n != 0 {
		t.Fatalf("eventos_stripe = %d", n)
	}
	if len(a.notas.lista()) != 0 {
		t.Fatal("agendou nota com assinatura inválida")
	}
}

func TestWebhookRepetidoIdempotente(t *testing.T) {
	a := novoAmb(t)
	id, cliente, _ := a.assinante(t, emailLic)
	fim := a.rel.agora().Add(30 * 24 * time.Hour)
	a.st.DefinirAssinatura(pagamento.EstadoStripe{ID: "sub_1", Status: "active", Cliente: cliente, PeriodoFim: &fim})
	for range 3 {
		confere(t, a.webhook(t, "evt_rep", "customer.subscription.updated", map[string]any{"id": "sub_1", "object": "subscription"}), http.StatusOK, "")
		confere(t, a.webhook(t, "evt_nota", "invoice.paid", map[string]any{"id": "in_1", "object": "invoice", "customer": cliente}), http.StatusOK, "")
	}
	if n := a.st.Consultas(); n != 1 {
		t.Fatalf("consultou a assinatura no Stripe %d vezes", n)
	}
	if n := a.contar(t, "eventos_stripe"); n != 2 {
		t.Fatalf("eventos_stripe = %d", n)
	}
	if n := len(a.notas.lista()); n != 1 {
		t.Fatalf("Agendar chamado %d vezes", n)
	}
	if as := a.lerAss(t, id); as == nil || as.Estado != "active" || !as.PeriodoFim.Equal(fim) {
		t.Fatalf("assinatura = %+v", as)
	}
}

func TestWebhookForaDeOrdem(t *testing.T) {
	a := novoAmb(t)
	id, cliente, _ := a.assinante(t, emailLic)
	fim := a.rel.agora().Add(30 * 24 * time.Hour)
	// O Stripe já está active; os eventos chegam fora de ordem e o corpo de cada um traz um estado velho.
	a.st.DefinirAssinatura(pagamento.EstadoStripe{ID: "sub_1", Status: "active", Cliente: cliente, PeriodoFim: &fim,
		Metadata: map[string]string{"usuario_id": id}})
	confere(t, a.webhook(t, "evt_2", "customer.subscription.updated",
		map[string]any{"id": "sub_1", "object": "subscription", "status": "active"}), http.StatusOK, "")
	confere(t, a.webhook(t, "evt_1", "customer.subscription.created",
		map[string]any{"id": "sub_1", "object": "subscription", "status": "incomplete"}), http.StatusOK, "")
	confere(t, a.webhook(t, "evt_0", "checkout.session.completed", map[string]any{"id": "cs_1", "object": "checkout.session",
		"mode": "subscription", "subscription": "sub_1", "customer": cliente, "client_reference_id": id}), http.StatusOK, "")
	as := a.lerAss(t, id)
	if as == nil || as.Estado != "active" || as.PeriodoFim == nil || !as.PeriodoFim.Equal(fim) {
		t.Fatalf("assinatura = %+v", as)
	}
	var sub string
	if err := a.db.QueryRow(context.Background(), `select stripe_assinatura from assinaturas where usuario_id = $1::uuid`, id).Scan(&sub); err != nil || sub != "sub_1" {
		t.Fatalf("stripe_assinatura = %q (%v)", sub, err)
	}
}

func TestWebhookPorMetadataSemCliente(t *testing.T) {
	// O cliente ainda não está gravado (ex.: o webhook chegou antes): a metadata da assinatura resolve.
	a := novoAmb(t)
	a.criarConfirmado(t, emailLic, senhaAna)
	id := a.usuario(t, emailLic).ID
	a.st.DefinirAssinatura(pagamento.EstadoStripe{ID: "sub_9", Status: "trialing", Cliente: "cus_desconhecido",
		Metadata: map[string]string{"usuario_id": id}})
	confere(t, a.webhook(t, "evt_9", "customer.subscription.created", map[string]any{"id": "sub_9", "object": "subscription"}), http.StatusOK, "")
	if as := a.lerAss(t, id); as == nil || as.Estado != "trialing" {
		t.Fatalf("assinatura = %+v", as)
	}
}

func TestWebhookClienteDesconhecido(t *testing.T) {
	a := novoAmb(t)
	a.st.DefinirAssinatura(pagamento.EstadoStripe{ID: "sub_x", Status: "active", Cliente: "cus_de_ninguem"})
	confere(t, a.webhook(t, "evt_x", "customer.subscription.updated", map[string]any{"id": "sub_x", "object": "subscription"}), http.StatusOK, "")
	confere(t, a.webhook(t, "evt_y", "invoice.paid", map[string]any{"id": "in_x", "object": "invoice", "customer": "cus_de_ninguem"}), http.StatusOK, "")
	confere(t, a.webhook(t, "evt_z", "charge.refunded", map[string]any{"id": "ch_1", "object": "charge"}), http.StatusOK, "")
	if n := a.contar(t, "assinaturas"); n != 0 {
		t.Fatalf("assinaturas = %d", n)
	}
	if len(a.notas.lista()) != 0 {
		t.Fatal("agendou nota de cliente desconhecido")
	}
	if l := a.log.String(); !strings.Contains(l, "evt_x") || strings.Contains(l, "cus_de_ninguem") {
		t.Fatalf("log = %s", l)
	}
}

func TestWebhookStripeForaDoAr500(t *testing.T) {
	a := novoAmb(t)
	_, cliente, _ := a.assinante(t, emailLic)
	// A assinatura não existe no falso: a consulta falha e o Stripe tem de tentar de novo.
	if w := a.webhook(t, "evt_f", "customer.subscription.updated", map[string]any{"id": "sub_nao", "object": "subscription", "customer": cliente}); w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", w.Code)
	}
	if n := a.contar(t, "eventos_stripe"); n != 0 {
		t.Fatalf("marcou o evento como visto sem aplicar: %d", n)
	}
}

func TestCanceladaBloqueia(t *testing.T) {
	a := novoAmb(t)
	id, cliente, _ := a.assinante(t, emailLic)
	a.lic(t, a.licencaDe(t, emailLic, senhaAna, pcA)) // começa o teste
	a.rel.avancar(48 * time.Hour)                     // teste vencido
	fim := a.rel.agora().Add(30 * 24 * time.Hour)
	a.st.DefinirAssinatura(pagamento.EstadoStripe{ID: "sub_1", Status: "active", Cliente: cliente, PeriodoFim: &fim})
	confere(t, a.webhook(t, "evt_1", "customer.subscription.created", map[string]any{"id": "sub_1", "object": "subscription"}), http.StatusOK, "")
	if l := a.lic(t, a.licencaDe(t, emailLic, senhaAna, pcA)); l.Estado != licenca.Assinante {
		t.Fatalf("estado = %s", l.Estado)
	}
	a.st.DefinirAssinatura(pagamento.EstadoStripe{ID: "sub_1", Status: "canceled", Cliente: cliente, PeriodoFim: &fim})
	confere(t, a.webhook(t, "evt_2", "customer.subscription.deleted", map[string]any{"id": "sub_1", "object": "subscription"}), http.StatusOK, "")
	if l := a.lic(t, a.licencaDe(t, emailLic, senhaAna, pcA)); l.Estado != licenca.Bloqueado {
		t.Fatalf("estado = %s", l.Estado)
	}
	if as := a.lerAss(t, id); as.Estado != "canceled" {
		t.Fatalf("assinatura = %+v", as)
	}
}

func TestCanceladaVelhaNaoDerrubaNova(t *testing.T) {
	// Duas assinaturas do mesmo usuário: o cancelamento da antiga não apaga a nova em dia.
	a := novoAmb(t)
	id, cliente, _ := a.assinante(t, emailLic)
	a.st.DefinirAssinatura(pagamento.EstadoStripe{ID: "sub_nova", Status: "active", Cliente: cliente})
	a.st.DefinirAssinatura(pagamento.EstadoStripe{ID: "sub_velha", Status: "canceled", Cliente: cliente})
	confere(t, a.webhook(t, "evt_1", "customer.subscription.created", map[string]any{"id": "sub_nova", "object": "subscription"}), http.StatusOK, "")
	confere(t, a.webhook(t, "evt_2", "customer.subscription.deleted", map[string]any{"id": "sub_velha", "object": "subscription"}), http.StatusOK, "")
	if as := a.lerAss(t, id); as.Estado != "active" {
		t.Fatalf("assinatura = %+v", as)
	}
}

func TestInvoicePaidAgendaNota(t *testing.T) {
	a := novoAmb(t)
	id, cliente, _ := a.assinante(t, emailLic)
	confere(t, a.webhook(t, "evt_n", "invoice.paid", map[string]any{"id": "in_123", "object": "invoice", "customer": cliente}), http.StatusOK, "")
	if got := a.notas.lista(); len(got) != 1 || got[0] != [2]string{"in_123", id} {
		t.Fatalf("Agendar = %v", got)
	}
}

func TestPaginaVoltar(t *testing.T) {
	a := novoAmb(t)
	w := httptest.NewRecorder()
	a.s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/v1/pagamento/voltar", nil))
	if w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("status %d tipo %q", w.Code, w.Header().Get("Content-Type"))
	}
	if b := w.Body.String(); !strings.Contains(b, "Pode voltar para o Applyer.") || strings.Contains(b, "<script") {
		t.Fatalf("corpo = %s", b)
	}
}

// Assinatura que o Stripe diz não existir (404 resource_missing) não tem como sincronizar: 200 e log, sem
// reenvios por 3 dias.
func TestWebhookAssinaturaInexistente200(t *testing.T) {
	a := novoAmb(t)
	_, cliente, _ := a.assinante(t, emailLic)
	a.st.Sumir("sub_apagada")
	confere(t, a.webhook(t, "evt_404", "customer.subscription.deleted",
		map[string]any{"id": "sub_apagada", "object": "subscription", "customer": cliente}), http.StatusOK, "")
	if n := a.contar(t, "assinaturas"); n != 0 {
		t.Fatalf("assinaturas = %d", n)
	}
	if l := a.log.String(); !strings.Contains(l, "evt_404") || !strings.Contains(l, "customer.subscription.deleted") {
		t.Fatalf("log = %s", l)
	}
}

// Evento de outro modo (live com chave de teste, ou o contrário) é ignorado: 200, sem consultar o Stripe.
func TestWebhookLivemodeDiferenteDaChave(t *testing.T) {
	a := novoAmb(t)
	id, cliente, _ := a.assinante(t, emailLic)
	a.st.DefinirAssinatura(pagamento.EstadoStripe{ID: "sub_1", Status: "active", Cliente: cliente})
	sub := map[string]any{"id": "sub_1", "object": "subscription"}

	a.s.cfg.StripeSecretKey = "sk_test_ficticia"
	confere(t, a.webhookModo(t, "evt_live", "customer.subscription.updated", sub, true), http.StatusOK, "")
	a.s.cfg.StripeSecretKey = "sk_live_ficticia"
	confere(t, a.webhookModo(t, "evt_teste", "customer.subscription.updated", sub, false), http.StatusOK, "")
	if n := a.st.Consultas(); n != 0 || a.lerAss(t, id) != nil {
		t.Fatalf("evento de outro modo foi aplicado (consultas = %d)", n)
	}
	if l := a.log.String(); !strings.Contains(l, "evt_live") || !strings.Contains(l, "evt_teste") {
		t.Fatalf("log = %s", l)
	}
	confere(t, a.webhookModo(t, "evt_ok", "customer.subscription.updated", sub, true), http.StatusOK, "")
	if as := a.lerAss(t, id); as == nil || as.Estado != "active" {
		t.Fatalf("evento do mesmo modo não aplicado: %+v", as)
	}
}
