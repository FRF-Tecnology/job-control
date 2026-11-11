package pagamento

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v87"
)

// apiFalsa responde como a API do Stripe e guarda o formulário de cada pedido por caminho.
type apiFalsa struct {
	mu      sync.Mutex
	pedidos map[string]url.Values
	idem    map[string]string
}

func novaAPI(t *testing.T, respostas map[string]string) (*apiFalsa, string) {
	t.Helper()
	a := &apiFalsa{pedidos: map[string]url.Values{}, idem: map[string]string{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		v, _ := url.ParseQuery(string(b))
		a.mu.Lock()
		a.pedidos[r.Method+" "+r.URL.Path] = v
		a.idem[r.URL.Path] = r.Header.Get("Idempotency-Key")
		a.mu.Unlock()
		resp, ok := respostas[r.Method+" "+r.URL.Path]
		if !ok {
			w.WriteHeader(404)
			_, _ = io.WriteString(w, `{"error":{"type":"invalid_request_error","message":"rota falsa"}}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, resp)
	}))
	t.Cleanup(srv.Close)
	return a, srv.URL
}

func TestCheckoutParametros(t *testing.T) {
	api, base := novaAPI(t, map[string]string{
		"POST /v1/checkout/sessions": `{"id":"cs_1","object":"checkout.session","url":"https://checkout.exemplo.com/cs_1"}`,
	})
	r := novoReal("sk_test_ficticia", "whsec_ficticio", "price_ficticio", "https://nuvem.exemplo.com", &base)
	u, err := r.Checkout(context.Background(), "cus_1", "uid-1")
	if err != nil || u != "https://checkout.exemplo.com/cs_1" {
		t.Fatalf("url %q err %v", u, err)
	}
	v := api.pedidos["POST /v1/checkout/sessions"]
	voltar := "https://nuvem.exemplo.com/v1/pagamento/voltar"
	for k, quer := range map[string]string{
		"mode": "subscription", "line_items[0][price]": "price_ficticio", "line_items[0][quantity]": "1",
		"client_reference_id": "uid-1", "customer": "cus_1", "tax_id_collection[enabled]": "true",
		"billing_address_collection": "required", "customer_update[name]": "auto", "customer_update[address]": "auto",
		"success_url": voltar, "cancel_url": voltar, "allowed_payment_method_types[0]": "card",
		"subscription_data[metadata][usuario_id]": "uid-1",
	} {
		if got := v.Get(k); got != quer {
			t.Errorf("%s = %q, queria %q", k, got, quer)
		}
	}
}

func TestClientePortalAssinatura(t *testing.T) {
	api, base := novaAPI(t, map[string]string{
		"POST /v1/customers":               `{"id":"cus_9","object":"customer"}`,
		"POST /v1/billing_portal/sessions": `{"id":"bps_1","object":"billing_portal.session","url":"https://portal.exemplo.com/p"}`,
		"GET /v1/subscriptions/sub_1":      `{"id":"sub_1","object":"subscription","status":"past_due","customer":"cus_9","metadata":{"usuario_id":"uid-9"},"items":{"object":"list","data":[{"id":"si_1","object":"subscription_item","current_period_end":1798761600}]}}`,
		"DELETE /v1/subscriptions/sub_1":   `{"id":"sub_1","object":"subscription","status":"canceled"}`,
	})
	r := novoReal("sk_test_ficticia", "whsec_ficticio", "price_ficticio", "https://nuvem.exemplo.com", &base)
	ctx := context.Background()
	c, err := r.CriarCliente(ctx, "ana@exemplo.com", "uid-9")
	if err != nil || c != "cus_9" {
		t.Fatalf("cliente %q err %v", c, err)
	}
	if v := api.pedidos["POST /v1/customers"]; v.Get("email") != "ana@exemplo.com" || v.Get("metadata[usuario_id]") != "uid-9" {
		t.Errorf("cliente = %v", v)
	}
	if k := api.idem["/v1/customers"]; k != "cliente-uid-9" {
		t.Errorf("idempotency-key = %q", k)
	}
	p, err := r.Portal(ctx, "cus_9")
	if err != nil || p != "https://portal.exemplo.com/p" {
		t.Fatalf("portal %q err %v", p, err)
	}
	if v := api.pedidos["POST /v1/billing_portal/sessions"]; v.Get("customer") != "cus_9" || v.Get("return_url") != "https://nuvem.exemplo.com/v1/pagamento/voltar" {
		t.Errorf("portal = %v", v)
	}
	e, err := r.Assinatura(ctx, "sub_1")
	if err != nil {
		t.Fatal(err)
	}
	fim := time.Unix(1798761600, 0).UTC()
	if e.ID != "sub_1" || e.Status != "past_due" || e.Cliente != "cus_9" || e.Metadata[ChaveUsuario] != "uid-9" || e.PeriodoFim == nil || !e.PeriodoFim.Equal(fim) {
		t.Fatalf("estado = %+v", e)
	}
	if err := r.Cancelar(ctx, "sub_1"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Assinatura(ctx, "sub_nao"); err == nil {
		t.Fatal("assinatura inexistente sem erro")
	} else if res := Resumo(err); res != "stripe: tipo=invalid_request_error codigo= http=404" {
		t.Errorf("resumo = %q", res)
	}
}

func TestEventoConfereAssinatura(t *testing.T) {
	r := novoReal("sk_test_ficticia", "whsec_ficticio", "price_ficticio", "https://nuvem.exemplo.com", nil)
	corpo := []byte(`{"id":"evt_1","object":"event","type":"invoice.paid","api_version":"2020-01-01","data":{"object":{"id":"in_1"}}}`)
	bom := stripe.GenerateTestSignedPayload(&stripe.UnsignedPayload{Payload: corpo, Secret: "whsec_ficticio", Timestamp: time.Now()})
	ev, err := r.Evento(corpo, bom.Header)
	if err != nil || ev.ID != "evt_1" {
		t.Fatalf("evento %+v err %v (versão diferente da API tem de passar)", ev, err)
	}
	ruim := stripe.GenerateTestSignedPayload(&stripe.UnsignedPayload{Payload: corpo, Secret: "whsec_outro", Timestamp: time.Now()})
	if _, err := r.Evento(corpo, ruim.Header); err != ErrAssinaturaWebhook {
		t.Fatalf("segredo errado: %v", err)
	}
}

func TestJaCancelada(t *testing.T) {
	casos := []struct {
		nome string
		err  error
		quer bool
	}{
		{"nil", nil, false},
		{"resource_missing", &stripe.Error{Code: stripe.ErrorCodeResourceMissing, HTTPStatusCode: 404}, true},
		{"404 sem código", &stripe.Error{HTTPStatusCode: 404}, true},
		{"embrulhado", fmt.Errorf("x: %w", &stripe.Error{Code: stripe.ErrorCodeResourceMissing}), true},
		{"500", &stripe.Error{HTTPStatusCode: 500}, false},
		{"401", &stripe.Error{HTTPStatusCode: 401, Code: stripe.ErrorCodeAPIKeyExpired}, false},
		{"rede", errors.New("connection reset"), false},
	}
	for _, c := range casos {
		if got := jaCancelada(c.err); got != c.quer {
			t.Errorf("%s: jaCancelada = %v, quer %v", c.nome, got, c.quer)
		}
	}
}
