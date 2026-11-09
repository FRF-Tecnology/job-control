package main

// Sonda do teste: um binário mínimo que liga o envio, para conferir que o build padrão não traz o gatilho.

import (
	"context"

	"github.com/FRF-Tecnology/job-control/internal/envio"
)

func main() {
	parar := envio.Ligar(context.Background(), nil, "")
	parar()
}
