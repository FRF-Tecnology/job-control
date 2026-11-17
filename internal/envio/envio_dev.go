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
	"github.com/FRF-Tecnology/job-control/internal/dominio"
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
func Ligar(ctx context.Context, arm *armazem.Armazem, pastaDados string, podeAgir func() (bool, string)) Controle {
	return ligarExecutorDeDesenvolvimento(ctx, arm, pastaDados, podeAgir)
}

func ligarExecutorDeDesenvolvimento(ctx context.Context, arm *armazem.Armazem, pastaDados string, podeAgir func() (bool, string)) Controle {
	log.Printf("envio: executor LIGADO (build de desenvolvimento envio_dev)")
	av := aovivo.Novo(time.Now) // "Ao vivo" (jc-a2y.6.9): começa desligado a cada abertura do app
	nav, err := navegador.Novo(navegador.Opcoes{PastaDados: pastaDados, AoVivo: av})
	if err != nil {
		log.Printf("envio: navegador indisponível (%T); executor desligado", err)
		return desligado()
	}
	x := executor.Novo(arm, executor.Opcoes{Navegador: nav, PastaCurriculos: filepath.Join(pastaDados, "curriculos"), Etapa: av.Etapa, PodeAgir: liberado(podeAgir)})
	ctx, cancelar := context.WithCancel(ctx)
	// A pessoa fechou a janela de login: plataformas em precisa-entrar voltam, se o envio geral estiver liberado
	// (jc-a2y.6.11).
	nav.QuandoFecharLogin(func() { x.LiberarDepoisDoLogin(ctx) })
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
		// Ler um formulário (jc-a2y.7.2): nenhum envio começa no meio da leitura. Com um ciclo em andamento, recusa na
		// hora (a rota dá 409), sem esperar sem limite (baixa do Fallen no #124).
		LerFormulario: func(ctx context.Context, endereco string) (dominio.LeituraFormulario, error) {
			if !ciclo.TryLock() {
				return dominio.LeituraFormulario{}, ErrEnviando
			}
			defer ciclo.Unlock()
			return nav.LerFormulario(ctx, endereco)
		},
		// Ler a descrição de uma vaga do Radar: como o LerFormulario, nunca no meio de um envio.
		LerDescricao: func(ctx context.Context, plataforma, endereco string) (string, string, error) {
			if !ciclo.TryLock() {
				return "", "", ErrEnviando
			}
			defer ciclo.Unlock()
			return nav.LerDescricao(ctx, plataforma, endereco)
		},
		AbrirParaPreencher: func(endereco string) error {
			ciclo.Lock() // o ciclo atual termina antes, como no login
			defer ciclo.Unlock()
			return erroDoPreencher(nav.AbrirParaPreencher(endereco))
		},
		SiteParaPreencher: func(ctx context.Context) (string, error) {
			h, err := nav.SiteParaPreencher(ctx)
			return h, erroDoPreencher(err)
		},
		PreencherSite: func(ctx context.Context, dominioConferido string, conteudo, perfil dominio.Objeto, nomeCurriculo string, curriculo []byte) (Preenchidos, error) {
			r, err := nav.PreencherSite(ctx, dominioConferido, conteudo, perfil, nomeCurriculo, curriculo)
			return r, erroDoPreencher(err)
		},
	}
}

// erroDoPreencher troca os erros do navegador pelos erros fixos da tela; o resto segue (a API loga só o tipo).
func erroDoPreencher(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, navegador.ErrLoginAberto):
		return ErrLoginAberto
	case errors.Is(err, navegador.ErrEmUso):
		return ErrEnviando
	case errors.Is(err, navegador.ErrSemJanela):
		return ErrSemJanela
	case errors.Is(err, navegador.ErrSemSite):
		return ErrSemSite
	case errors.Is(err, navegador.ErrSiteMudou):
		return ErrSiteMudou
	case errors.Is(err, navegador.ErrSemNavegador):
		return ErrSemNavegador
	}
	return err
}
