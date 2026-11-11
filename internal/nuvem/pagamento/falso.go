package pagamento

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/stripe/stripe-go/v87"
)

// ErrNaoExiste: a assinatura não está no Falso (simula Stripe fora do ar ou id errado).
var ErrNaoExiste = errors.New("assinatura não existe no Stripe falso")

// Falso é o Stripe dos testes: guarda tudo em memória e confere webhooks com o mesmo código do Real.
type Falso struct {
	segredo string

	mu          sync.Mutex
	clientes    []string    // usuarioID de cada cliente criado (cus_falso_<n> = índice+1)
	checkouts   [][2]string // cliente, usuarioID
	assinaturas map[string]EstadoStripe
	canceladas  []string
	consultas   int
}

// NovoFalso cria o Falso; os webhooks dos testes são assinados com segredo.
func NovoFalso(segredo string) *Falso {
	return &Falso{segredo: segredo, assinaturas: map[string]EstadoStripe{}}
}

func (f *Falso) CriarCliente(_ context.Context, _, usuarioID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.clientes = append(f.clientes, usuarioID)
	return fmt.Sprintf("cus_falso_%d", len(f.clientes)), nil
}

func (f *Falso) Checkout(_ context.Context, cliente, usuarioID string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checkouts = append(f.checkouts, [2]string{cliente, usuarioID})
	return "https://checkout.exemplo.com/c/" + cliente, nil
}

func (f *Falso) Portal(_ context.Context, cliente string) (string, error) {
	return "https://portal.exemplo.com/p/" + cliente, nil
}

func (f *Falso) Cancelar(_ context.Context, assinatura string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	a, ok := f.assinaturas[assinatura]
	if !ok {
		return ErrNaoExiste
	}
	a.Status = "canceled"
	f.assinaturas[assinatura] = a
	f.canceladas = append(f.canceladas, assinatura)
	return nil
}

func (f *Falso) Evento(corpo []byte, cabecalho string) (stripe.Event, error) {
	return construir(corpo, cabecalho, f.segredo)
}

func (f *Falso) Assinatura(_ context.Context, id string) (EstadoStripe, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.consultas++
	a, ok := f.assinaturas[id]
	if !ok {
		return EstadoStripe{}, ErrNaoExiste
	}
	return a, nil
}

// DefinirAssinatura troca o estado "atual no Stripe" de uma assinatura.
func (f *Falso) DefinirAssinatura(e EstadoStripe) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.assinaturas[e.ID] = e
}

// Clientes conta os clientes criados.
func (f *Falso) Clientes() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.clientes) }

// Checkouts lista as sessões abertas (cliente, usuarioID).
func (f *Falso) Checkouts() [][2]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][2]string(nil), f.checkouts...)
}

// Consultas conta as chamadas a Assinatura.
func (f *Falso) Consultas() int { f.mu.Lock(); defer f.mu.Unlock(); return f.consultas }

// Canceladas lista as assinaturas canceladas por Cancelar.
func (f *Falso) Canceladas() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.canceladas...)
}
