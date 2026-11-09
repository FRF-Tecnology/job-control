//go:build !envio_dev

package envio

import (
	"context"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
)

// Ligado diz se este binário traz o gatilho do envio.
const Ligado = false

// Ligar não faz nada no build padrão: nenhuma candidatura é enviada.
func Ligar(ctx context.Context, arm *armazem.Armazem, pastaDados string) Controle {
	return Controle{Parar: func() {}, FecharNavegador: func() {}}
}
