//go:build envio_dev

package envio

import (
	"context"
	"errors"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/aovivo"
	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/executor"
	"github.com/FRF-Tecnology/job-control/internal/navegador"
)

// Ligado diz se este binário traz o gatilho do envio.
const Ligado = true

// intervaloDaFila: de quanto em quanto tempo o executor olha se há candidatura aprovada.
const intervaloDaFila = 2 * time.Minute

// Ligar (só no build envio_dev) roda o executor em segundo plano: ao abrir, Iniciar (enviando vira incerta e
// aprovação vencida volta a preparada), e depois EnviarAprovadas de tempos em tempos. O navegador abre SEM JANELA
// (jc-a2y.6.7): só a pessoa, pela tela, mostra o envio (MostrarEnvio) ou abre o login (Entrar).
func Ligar(ctx context.Context, arm *armazem.Armazem, pastaDados string) Controle {
	return ligarExecutorDeDesenvolvimento(ctx, arm, pastaDados)
}

func ligarExecutorDeDesenvolvimento(ctx context.Context, arm *armazem.Armazem, pastaDados string) Controle {
	log.Printf("envio: executor LIGADO (build de desenvolvimento envio_dev)")
	av := aovivo.Novo(time.Now) // "Ao vivo" (jc-a2y.6.9): começa desligado a cada abertura do app
	nav, err := navegador.Novo(navegador.Opcoes{PastaDados: pastaDados, AoVivo: av})
	if err != nil {
		log.Printf("envio: navegador indisponível (%T); executor desligado", err)
		return desligado()
	}
	x := executor.Novo(arm, executor.Opcoes{Navegador: nav, PastaCurriculos: filepath.Join(pastaDados, "curriculos"), Etapa: av.Etapa})
	ctx, cancelar := context.WithCancel(ctx)
	feito := make(chan struct{})
	var ciclo sync.Mutex // um ciclo da fila por vez; Suspender segura até soltar
	go func() {
		defer close(feito)
		x.Rodar(ctx, intervaloDaFila, &ciclo) // Iniciar falhando não entra no laço
	}()
	return Controle{
		Parar: func() {
			cancelar()
			<-feito
			nav.Fechar()
			av.Ligar(false) // fechar o app (ou apagar os dados) apaga o quadro
			av.Limpar()
		},
		Suspender: func() func() {
			ciclo.Lock()
			return ciclo.Unlock
		},
		FecharNavegador: nav.Fechar, // espera o processo sair; a próxima página abre de novo, com o perfil recriado
		MostrarEnvio:    nav.MostrarEnvio,
		Entrar: func(endereco string) error {
			ciclo.Lock() // o ciclo atual termina antes; o login não corta um envio
			defer ciclo.Unlock()
			switch err := nav.AbrirLogin(endereco); {
			case err == nil:
				return nil
			case errors.Is(err, navegador.ErrLoginAberto):
				return ErrLoginAberto
			case errors.Is(err, navegador.ErrEmUso):
				return ErrEnviando
			default:
				log.Printf("envio: navegador para o login (%T)", err)
				return ErrSemNavegador
			}
		},
		Janela: nav.Estado,
		AoVivo: av,
	}
}
