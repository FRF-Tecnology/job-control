//go:build envio_dev

package envio

import (
	"context"
	"log"
	"path/filepath"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/executor"
	"github.com/FRF-Tecnology/job-control/internal/navegador"
)

// Ligado diz se este binário traz o gatilho do envio.
const Ligado = true

// intervaloDaFila: de quanto em quanto tempo o executor olha se há candidatura aprovada.
const intervaloDaFila = 2 * time.Minute

// Ligar (só no build envio_dev) roda o executor em segundo plano: ao abrir, Iniciar (enviando vira incerta e
// aprovação vencida volta a preparada), e depois EnviarAprovadas de tempos em tempos, com o navegador visível.
func Ligar(ctx context.Context, arm *armazem.Armazem, pastaDados string) Controle {
	return ligarExecutorDeDesenvolvimento(ctx, arm, pastaDados)
}

func ligarExecutorDeDesenvolvimento(ctx context.Context, arm *armazem.Armazem, pastaDados string) Controle {
	log.Printf("envio: executor LIGADO (build de desenvolvimento envio_dev)")
	nav, err := navegador.Novo(navegador.Opcoes{PastaDados: pastaDados, Visivel: true})
	if err != nil {
		log.Printf("envio: navegador indisponível (%T); executor desligado", err)
		return Controle{Parar: func() {}, FecharNavegador: func() {}}
	}
	x := executor.Novo(arm, executor.Opcoes{Navegador: nav, PastaCurriculos: filepath.Join(pastaDados, "curriculos")})
	ctx, cancelar := context.WithCancel(ctx)
	feito := make(chan struct{})
	go func() {
		defer close(feito)
		if _, _, err := x.Iniciar(ctx); err != nil {
			log.Printf("envio: ao abrir (%T)", err)
		}
		t := time.NewTicker(intervaloDaFila)
		defer t.Stop()
		for {
			if _, err := x.EnviarAprovadas(ctx); err != nil && ctx.Err() == nil {
				log.Printf("envio: fila (%T)", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
		}
	}()
	return Controle{
		Parar: func() {
			cancelar()
			<-feito
			nav.Fechar()
		},
		FecharNavegador: nav.Fechar, // a próxima página abre o navegador de novo, com o perfil recriado
	}
}
