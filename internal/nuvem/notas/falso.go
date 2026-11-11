package notas

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Falso é o emissor dos testes e das rodadas locais: não emite nada de verdade, só marca a nota como
// emitida com um número inventado. Falha as primeiras Falhas chamadas.
type Falso struct {
	Falhas int           // quantas chamadas ainda devem falhar
	Atraso time.Duration // espera dentro de Emitir (simula emissor lento)

	mu       sync.Mutex
	chamadas int
	pedidos  []Pedido
}

func (f *Falso) Emitir(ctx context.Context, p Pedido) (string, string, error) {
	f.mu.Lock()
	f.chamadas++
	falha := f.Falhas > 0
	if falha {
		f.Falhas--
	}
	atraso := f.Atraso
	f.mu.Unlock()
	if atraso > 0 {
		select {
		case <-time.After(atraso):
		case <-ctx.Done():
			return "", "", ctx.Err()
		}
	}
	if falha {
		return "", "", &ErroEmissor{Classe: "falso"}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pedidos = append(f.pedidos, p)
	return fmt.Sprintf("FALSO-%d", len(f.pedidos)), "", nil
}

// Chamadas conta as chamadas a Emitir (com ou sem falha).
func (f *Falso) Chamadas() int { f.mu.Lock(); defer f.mu.Unlock(); return f.chamadas }

// Pedidos lista os pedidos emitidos com sucesso.
func (f *Falso) Pedidos() []Pedido {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Pedido(nil), f.pedidos...)
}
