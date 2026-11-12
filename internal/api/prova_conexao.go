package api

import (
	"context"
	"sync/atomic"
)

type chaveProvaDoPerfil struct{}

func (a *API) verificarProvaDoPerfil(ctx context.Context) error {
	if esperada, ok := ctx.Value(chaveProvaDoPerfil{}).(uint64); ok && esperada != a.provaIA.estado.Load() {
		return falha(412, "a conexão de IA mudou; verifique novamente")
	}
	return nil
}

// Uma verificação antiga não pode validar uma credencial ou rota recém-trocada.
// O último bit significa sucesso; a geração muda a cada alteração ou nova tentativa.
type provaConexao struct{ estado atomic.Uint64 }

func (p *provaConexao) invalidar() uint64 {
	for {
		v := p.estado.Load()
		prox := (v &^ 1) + 2
		if p.estado.CompareAndSwap(v, prox) {
			return prox
		}
	}
}
func (p *provaConexao) confirmar(geracao uint64) bool {
	return p.estado.CompareAndSwap(geracao, geracao|1)
}
func (p *provaConexao) valida() bool { return p.estado.Load()&1 == 1 }
