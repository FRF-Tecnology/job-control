// Package pagamento fala com o Stripe: cliente, Checkout, Portal, cancelamento, consulta e webhooks.
// Um plano mensal só, cartão só. O app abre as URLs no navegador; os webhooks mantêm assinaturas em dia.
package pagamento

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/stripe/stripe-go/v87"
)

// Stripe é o que o servidor usa do Stripe; os testes usam Falso.
type Stripe interface {
	// CriarCliente cria o cliente do usuário (metadata usuario_id) e devolve o id (cus_...).
	CriarCliente(ctx context.Context, email, usuarioID string) (string, error)
	// Checkout abre a sessão de assinatura e devolve a URL.
	Checkout(ctx context.Context, cliente, usuarioID string) (url string, err error)
	// Portal abre o portal do cliente e devolve a URL.
	Portal(ctx context.Context, cliente string) (url string, err error)
	// Cancelar encerra a assinatura agora.
	Cancelar(ctx context.Context, assinatura string) error
	// Evento confere a assinatura do webhook (cabeçalho Stripe-Signature) e decodifica o evento.
	Evento(corpo []byte, assinaturaCabecalho string) (stripe.Event, error)
	// Assinatura busca o estado atual da assinatura no Stripe.
	Assinatura(ctx context.Context, id string) (EstadoStripe, error)
}

// EstadoStripe é o que importa de uma assinatura do Stripe.
type EstadoStripe struct {
	ID         string
	Status     string // active, trialing, past_due, canceled, unpaid, incomplete...
	Cliente    string
	PeriodoFim *time.Time
	Metadata   map[string]string
}

// ChaveUsuario é a chave de metadata com o id do usuário (cliente e assinatura).
const ChaveUsuario = "usuario_id"

// CaminhoVoltar é a página estática para onde o Checkout e o Portal voltam.
const CaminhoVoltar = "/v1/pagamento/voltar"

// Real usa a API do Stripe pelo cliente da biblioteca (sem chave global).
type Real struct {
	api            *stripe.Client
	segredoWebhook string
	preco          string
	voltar         string
}

// NovoReal monta o Stripe real. enderecoPublico vem sem barra no fim (LerConfig tira).
func NovoReal(chave, segredoWebhook, preco, enderecoPublico string) *Real {
	return novoReal(chave, segredoWebhook, preco, enderecoPublico, nil)
}

// novoReal aceita outro endereço da API (os testes usam um servidor local).
func novoReal(chave, segredoWebhook, preco, enderecoPublico string, api *string) *Real {
	cfg := &stripe.BackendConfig{
		URL: api,
		// A biblioteca loga erros com a mensagem do Stripe, que pode citar o email: o servidor loga por Resumo.
		LeveledLogger: &stripe.LeveledLogger{Level: stripe.LevelNull},
	}
	return &Real{api: stripe.NewClient(chave, stripe.WithBackends(stripe.NewBackendsWithConfig(cfg))),
		segredoWebhook: segredoWebhook, preco: preco, voltar: enderecoPublico + CaminhoVoltar}
}

func (r *Real) CriarCliente(ctx context.Context, email, usuarioID string) (string, error) {
	p := &stripe.CustomerCreateParams{Email: stripe.String(email), Metadata: map[string]string{ChaveUsuario: usuarioID}}
	// Duas chamadas juntas (ou uma repetida após falha de rede) devolvem o mesmo cliente por 24h.
	p.SetIdempotencyKey("cliente-" + usuarioID)
	c, err := r.api.V1Customers.Create(ctx, p)
	if err != nil {
		return "", err
	}
	return c.ID, nil
}

func (r *Real) Checkout(ctx context.Context, cliente, usuarioID string) (string, error) {
	p := &stripe.CheckoutSessionCreateParams{
		Mode:                      stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems:                 []*stripe.CheckoutSessionCreateLineItemParams{{Price: stripe.String(r.preco), Quantity: stripe.Int64(1)}},
		ClientReferenceID:         stripe.String(usuarioID),
		Customer:                  stripe.String(cliente),
		AllowedPaymentMethodTypes: []*string{stripe.String("card")},
		TaxIDCollection:           &stripe.CheckoutSessionCreateTaxIDCollectionParams{Enabled: stripe.Bool(true)},
		BillingAddressCollection:  stripe.String(string(stripe.CheckoutSessionBillingAddressCollectionRequired)),
		CustomerUpdate: &stripe.CheckoutSessionCreateCustomerUpdateParams{
			Name: stripe.String("auto"), Address: stripe.String("auto"),
		},
		// Os eventos customer.subscription.* acham o usuário mesmo sem a sessão.
		SubscriptionData: &stripe.CheckoutSessionCreateSubscriptionDataParams{Metadata: map[string]string{ChaveUsuario: usuarioID}},
		SuccessURL:       stripe.String(r.voltar),
		CancelURL:        stripe.String(r.voltar),
	}
	s, err := r.api.V1CheckoutSessions.Create(ctx, p)
	if err != nil {
		return "", err
	}
	return s.URL, nil
}

func (r *Real) Portal(ctx context.Context, cliente string) (string, error) {
	s, err := r.api.V1BillingPortalSessions.Create(ctx, &stripe.BillingPortalSessionCreateParams{
		Customer: stripe.String(cliente), ReturnURL: stripe.String(r.voltar),
	})
	if err != nil {
		return "", err
	}
	return s.URL, nil
}

func (r *Real) Cancelar(ctx context.Context, assinatura string) error {
	_, err := r.api.V1Subscriptions.Cancel(ctx, assinatura, nil)
	return err
}

func (r *Real) Evento(corpo []byte, cabecalho string) (stripe.Event, error) {
	return construir(corpo, cabecalho, r.segredoWebhook)
}

func (r *Real) Assinatura(ctx context.Context, id string) (EstadoStripe, error) {
	s, err := r.api.V1Subscriptions.Retrieve(ctx, id, nil)
	if err != nil {
		return EstadoStripe{}, err
	}
	return estadoDe(s), nil
}

// construir confere a assinatura com a tolerância padrão (5 min). A versão da API do evento pode diferir
// da biblioteca: do corpo só usamos ids, e o estado vem sempre de uma consulta nova à API.
func construir(corpo []byte, cabecalho, segredo string) (stripe.Event, error) {
	ev, err := stripe.ConstructEvent(corpo, cabecalho, segredo, stripe.WithIgnoreAPIVersionMismatch())
	if err != nil {
		return stripe.Event{}, ErrAssinaturaWebhook
	}
	return ev, nil
}

// ErrAssinaturaWebhook: o corpo não confere com o cabeçalho (ou está vencido ou ilegível).
var ErrAssinaturaWebhook = errors.New("assinatura do webhook inválida")

// estadoDe tira da assinatura o estado; o fim do período mora nos itens desde a API 2025-03.
func estadoDe(s *stripe.Subscription) EstadoStripe {
	e := EstadoStripe{ID: s.ID, Status: string(s.Status), Metadata: s.Metadata}
	if s.Customer != nil {
		e.Cliente = s.Customer.ID
	}
	var fim int64
	if s.Items != nil {
		for _, it := range s.Items.Data {
			if it != nil && it.CurrentPeriodEnd > fim {
				fim = it.CurrentPeriodEnd
			}
		}
	}
	if fim > 0 {
		t := time.Unix(fim, 0).UTC()
		e.PeriodoFim = &t
	}
	return e
}

// Resumo descreve o erro do Stripe para o log sem dados do cliente: só tipo, código e status HTTP.
func Resumo(err error) string {
	var se *stripe.Error
	if errors.As(err, &se) {
		return fmt.Sprintf("stripe: tipo=%s codigo=%s http=%d", se.Type, se.Code, se.HTTPStatusCode)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "stripe: " + err.Error()
	}
	return fmt.Sprintf("stripe: falha de rede (%T)", err)
}
