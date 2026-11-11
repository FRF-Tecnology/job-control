package nuvem

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/stripe/stripe-go/v87"

	"github.com/FRF-Tecnology/job-control/internal/nuvem/contas"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/pagamento"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/sessoes"
)

// Rotas de pagamento. O log leva só ids (usuário, evento, assinatura) e o tipo do evento: nunca email,
// cartão, CPF/CNPJ nem o corpo do webhook. CPF/CNPJ ficam só no Stripe.

const (
	msgJaAssinante   = "Você já tem uma assinatura ativa. Use o portal para gerenciar"
	msgSemAssinatura = "Você ainda não tem assinatura"
	msgPagamento     = "Pagamento indisponível. Tente de novo"
	msgWebhook       = "assinatura inválida"
)

// AgendadorNotas agenda a nota fiscal de uma fatura paga. Tem de ser idempotente por invoiceID:
// se a gravação do evento falhar depois, o Stripe reenvia e Agendar roda de novo.
type AgendadorNotas interface {
	Agendar(ctx context.Context, invoiceID, usuarioID string) error
}

var reUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

const paginaVoltar = `<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Applyer</title></head>
<body style="font-family: system-ui, sans-serif; text-align: center; padding: 3rem 1rem;">
<p>Pode voltar para o Applyer.</p>
</body></html>
`

func (s *Servidor) rotasPagamento() {
	s.mux.Handle("POST /v1/assinar", sessoes.Middleware(s.sessoes, s.assinar))
	s.mux.Handle("POST /v1/portal", sessoes.Middleware(s.sessoes, s.portal))
	s.mux.HandleFunc("POST "+caminhoWebhook, s.webhook)
	s.mux.HandleFunc("GET "+pagamento.CaminhoVoltar, voltar)
}

// emVigor: estados do Stripe em que a assinatura ainda vale (past_due com a tolerância de estadoDa).
func emVigor(estado string) bool {
	return estado == "active" || estado == "trialing" || estado == "past_due"
}

func (s *Servidor) falhaStripe(w http.ResponseWriter, r *http.Request, onde string, err error) {
	s.log.Error("falha no Stripe", "rota", r.URL.Path, "onde", onde, "erro", pagamento.Resumo(err))
	erro(w, http.StatusBadGateway, msgPagamento)
}

// assinar abre o Checkout. Cria o cliente Stripe na primeira vez e grava usuarios.stripe_cliente.
func (s *Servidor) assinar(w http.ResponseWriter, r *http.Request, usuarioID string) {
	ctx := r.Context()
	u, err := s.contas.PorID(ctx, usuarioID)
	if err != nil {
		s.interno(w, r, "usuario", err)
		return
	}
	if !u.Confirmado() {
		erro(w, http.StatusForbidden, msgConfirme)
		return
	}
	a, err := lerAssinatura(ctx, s.dep.Banco, u.ID)
	if err != nil {
		s.interno(w, r, "assinatura", err)
		return
	}
	if a != nil && emVigor(a.Estado) {
		erro(w, http.StatusConflict, msgJaAssinante) // evita cobrar duas vezes
		return
	}
	cliente, err := s.contas.StripeCliente(ctx, u.ID)
	if err != nil {
		s.interno(w, r, "cliente", err)
		return
	}
	if cliente == "" {
		novo, err := s.dep.Stripe.CriarCliente(ctx, u.Email, u.ID)
		if err != nil {
			s.falhaStripe(w, r, "criar cliente", err)
			return
		}
		if cliente, err = s.contas.GravarStripeCliente(ctx, u.ID, novo); err != nil {
			s.interno(w, r, "gravar cliente", err)
			return
		}
	}
	url, err := s.dep.Stripe.Checkout(ctx, cliente, u.ID)
	if err != nil {
		s.falhaStripe(w, r, "checkout", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	escreverJSON(w, http.StatusOK, map[string]string{"url": url})
}

// portal abre o portal do cliente (trocar cartão, cancelar, faturas). Sem cliente Stripe: 409.
func (s *Servidor) portal(w http.ResponseWriter, r *http.Request, usuarioID string) {
	ctx := r.Context()
	cliente, err := s.contas.StripeCliente(ctx, usuarioID)
	if err != nil {
		s.interno(w, r, "cliente", err)
		return
	}
	if cliente == "" {
		erro(w, http.StatusConflict, msgSemAssinatura)
		return
	}
	url, err := s.dep.Stripe.Portal(ctx, cliente)
	if err != nil {
		s.falhaStripe(w, r, "portal", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	escreverJSON(w, http.StatusOK, map[string]string{"url": url})
}

func voltar(w http.ResponseWriter, _ *http.Request) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, paginaVoltar)
}

// webhook recebe os eventos do Stripe. Regras:
//   - a assinatura do cabeçalho é conferida antes de tudo (400 se não confere, sem tocar no banco);
//   - evento já visto ou que não tratamos: 200;
//   - o estado vem de uma consulta nova ao Stripe, então a ordem dos eventos não importa;
//   - o evento é marcado como visto na mesma transação que aplica o efeito; falha passageira: 500 (o Stripe reenvia).
func (s *Servidor) webhook(w http.ResponseWriter, r *http.Request) {
	corpo, err := io.ReadAll(r.Body)
	if err != nil {
		var grande *http.MaxBytesError
		if errors.As(err, &grande) {
			erro(w, http.StatusRequestEntityTooLarge, msgPedido)
			return
		}
		erro(w, http.StatusBadRequest, msgPedido)
		return
	}
	ev, err := s.dep.Stripe.Evento(corpo, r.Header.Get("Stripe-Signature"))
	if err != nil {
		s.log.Warn("webhook recusado: assinatura inválida")
		erro(w, http.StatusBadRequest, msgWebhook)
		return
	}
	ctx := r.Context()
	tipo := string(ev.Type)
	var visto bool
	if err := s.dep.Banco.QueryRow(ctx, `select exists (select 1 from eventos_stripe where event_id = $1)`, ev.ID).Scan(&visto); err != nil {
		s.interno(w, r, "evento visto", err)
		return
	}
	if visto {
		escreverJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	switch {
	case tipo == "checkout.session.completed":
		var cs stripe.CheckoutSession
		if err := json.Unmarshal(ev.Data.Raw, &cs); err != nil {
			s.eventoIlegivel(w, ev)
			return
		}
		if cs.Mode != stripe.CheckoutSessionModeSubscription || cs.Subscription == nil || cs.Subscription.ID == "" {
			break
		}
		s.sincronizar(w, r, ev, cs.Subscription.ID, cs.ClientReferenceID)
		return
	case strings.HasPrefix(tipo, "customer.subscription."):
		var sub stripe.Subscription
		if err := json.Unmarshal(ev.Data.Raw, &sub); err != nil || sub.ID == "" {
			s.eventoIlegivel(w, ev)
			return
		}
		s.sincronizar(w, r, ev, sub.ID, "")
		return
	case tipo == "invoice.paid":
		var inv stripe.Invoice
		if err := json.Unmarshal(ev.Data.Raw, &inv); err != nil || inv.ID == "" {
			s.eventoIlegivel(w, ev)
			return
		}
		s.faturaPaga(w, r, ev, &inv)
		return
	}
	escreverJSON(w, http.StatusOK, map[string]bool{"ok": true}) // evento que não tratamos
}

// eventoIlegivel: assinado pelo Stripe mas num formato que não entendemos. 200 para não gerar reenvios
// sem fim; o log diz qual evento olhar no painel.
func (s *Servidor) eventoIlegivel(w http.ResponseWriter, ev stripe.Event) {
	s.log.Error("webhook ilegível", "tipo", string(ev.Type), "evento", ev.ID)
	escreverJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// sincronizar busca a assinatura atual no Stripe e grava o estado do dono.
func (s *Servidor) sincronizar(w http.ResponseWriter, r *http.Request, ev stripe.Event, assinaturaID, referencia string) {
	ctx := r.Context()
	est, err := s.dep.Stripe.Assinatura(ctx, assinaturaID)
	if err != nil {
		s.log.Error("falha no Stripe", "rota", r.URL.Path, "onde", "consultar assinatura", "tipo", string(ev.Type),
			"evento", ev.ID, "erro", pagamento.Resumo(err))
		erro(w, http.StatusInternalServerError, msgInterno)
		return
	}
	dono, err := s.donoDa(ctx, est.Metadata[pagamento.ChaveUsuario], referencia, est.Cliente)
	if err != nil {
		s.donoFalhou(w, r, ev, err)
		return
	}
	agora := s.dep.Agora().UTC()
	s.aplicarEvento(w, r, ev, func(tx pgx.Tx) error {
		return gravarAssinatura(ctx, tx, dono, est.ID, est.Status, est.PeriodoFim, agora)
	})
}

// faturaPaga agenda a nota fiscal da fatura.
func (s *Servidor) faturaPaga(w http.ResponseWriter, r *http.Request, ev stripe.Event, inv *stripe.Invoice) {
	ctx := r.Context()
	var meta, cliente string
	if inv.Parent != nil && inv.Parent.SubscriptionDetails != nil {
		meta = inv.Parent.SubscriptionDetails.Metadata[pagamento.ChaveUsuario]
	}
	if inv.Customer != nil {
		cliente = inv.Customer.ID
	}
	dono, err := s.donoDa(ctx, meta, "", cliente)
	if err != nil {
		s.donoFalhou(w, r, ev, err)
		return
	}
	s.aplicarEvento(w, r, ev, func(pgx.Tx) error {
		return s.dep.Notas.Agendar(ctx, inv.ID, dono)
	})
}

// donoFalhou: cliente desconhecido é ignorado (200, só tipo e id no log); erro de banco é 500.
func (s *Servidor) donoFalhou(w http.ResponseWriter, r *http.Request, ev stripe.Event, err error) {
	if errors.Is(err, contas.ErrNaoEncontrado) {
		s.log.Info("webhook ignorado: cliente desconhecido", "tipo", string(ev.Type), "evento", ev.ID)
		escreverJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}
	s.interno(w, r, "dono", err)
}

// donoDa acha o usuário: metadata usuario_id (cliente/assinatura), client_reference_id do Checkout,
// e por fim usuarios.stripe_cliente. Nenhum: contas.ErrNaoEncontrado.
func (s *Servidor) donoDa(ctx context.Context, meta, referencia, cliente string) (string, error) {
	for _, id := range []string{meta, referencia} {
		if !reUUID.MatchString(id) {
			continue
		}
		u, err := s.contas.PorID(ctx, id)
		if err == nil {
			return u.ID, nil
		}
		if !errors.Is(err, contas.ErrNaoEncontrado) {
			return "", err
		}
	}
	if cliente == "" {
		return "", contas.ErrNaoEncontrado
	}
	return s.contas.PorStripeCliente(ctx, cliente)
}

// aplicarEvento marca o evento como visto e aplica o efeito na mesma transação. Se outro pedido já
// marcou (reenvio simultâneo), não aplica de novo.
func (s *Servidor) aplicarEvento(w http.ResponseWriter, r *http.Request, ev stripe.Event, efeito func(pgx.Tx) error) {
	ctx := r.Context()
	tx, err := s.dep.Banco.Begin(ctx)
	if err != nil {
		s.interno(w, r, "webhook", err)
		return
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `insert into eventos_stripe (event_id, recebido_em) values ($1, $2) on conflict do nothing`,
		ev.ID, s.dep.Agora().UTC())
	if err != nil {
		s.interno(w, r, "webhook", err)
		return
	}
	if tag.RowsAffected() == 1 {
		if err := efeito(tx); err != nil {
			s.log.Error("webhook não aplicado", "tipo", string(ev.Type), "evento", ev.ID, "erro", err.Error())
			erro(w, http.StatusInternalServerError, msgInterno)
			return
		}
		if err := tx.Commit(ctx); err != nil {
			s.interno(w, r, "webhook", err)
			return
		}
	}
	escreverJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
