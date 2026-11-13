// Package emails envia os emails da nuvem (códigos, avisos). Texto simples, pt-BR.
package emails

import (
	"context"
	"sync"
)

// Mensagem é um email de texto simples.
type Mensagem struct{ Para, Assunto, Texto string }

// Enviador manda uma mensagem.
type Enviador interface {
	Enviar(ctx context.Context, m Mensagem) error
}

// Falso guarda as mensagens em memória (testes e desenvolvimento). Seguro para uso concorrente
// desde que a leitura use Lista.
type Falso struct {
	mu       sync.Mutex
	Enviadas []Mensagem
}

// Enviar guarda a mensagem.
func (f *Falso) Enviar(_ context.Context, m Mensagem) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Enviadas = append(f.Enviadas, m)
	return nil
}

// Lista devolve uma cópia das mensagens enviadas.
func (f *Falso) Lista() []Mensagem {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Mensagem(nil), f.Enviadas...)
}
