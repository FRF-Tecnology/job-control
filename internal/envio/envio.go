// Package envio liga o executor de candidaturas no app (P5, jc-a2y.6.2.7).
//
// O app envia sozinho só as candidaturas que a pessoa aprovou na tela, e ela pausa quando quiser. Não há build
// separado para o envio: o app é um só.
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

// Controle do envio ligado. Sem navegador no computador, nada faz nada.
//   - Parar: cancela o executor, espera o ciclo acabar e fecha o navegador esperando o processo sair (ao fechar o
//     app ou antes de apagar os dados). Para de vez.
//   - Suspender: impede ciclos novos e espera o atual acabar; devolve a função que solta (sair das plataformas).
//   - FecharNavegador: fecha o navegador e espera o processo sair (o perfil não pode estar em uso ao apagar).
//   - MostrarEnvio: "Ver o envio" (jc-a2y.6.7). true abre o navegador do envio numa janela a partir da próxima
//     candidatura; false volta ao envio sem janela, que é o padrão a cada abertura do app.
//   - Entrar: "Entrar nas plataformas". Espera o ciclo atual acabar e abre o endereço numa janela visível, para a
//     pessoa entrar com a conta dela; até ela fechar a janela, o envio espera.
//   - Janela: se "Ver o envio" está ligado e se a janela de login está aberta.
//   - AoVivo (jc-a2y.6.9): o último quadro da aba do envio e a etapa, só em memória. nil sem o envio (a rota dá 404).
//   - LerFormulario (jc-a2y.7.2): lê a estrutura de um formulário sem interagir, depois do ciclo atual do executor.
//     nil sem o envio (a rota dá 404).
type Controle struct {
	Parar           func()
	Suspender       func() (soltar func())
	FecharNavegador func()
	MostrarEnvio    func(visivel bool)
	Entrar          func(endereco string) error
	Janela          func() (visivel, login bool)
	AoVivo          *aovivo.AoVivo
	LerFormulario   func(ctx context.Context, endereco string) (dominio.LeituraFormulario, error)
}

func nada()                                {}
func suspenderNada() func()                { return nada }
func mostrarNada(bool)                     {}
func janelaNenhuma() (visivel, login bool) { return false, false }

// Erros de Entrar: textos fixos para a tela (409), sem o erro do sistema, que pode trazer caminho.
var (
	ErrSemNavegador = &dominio.ErroConflito{Msg: "não deu para abrir o navegador (Chrome ou Edge) neste computador", Motivo: "sem-navegador"}
	ErrLoginAberto  = &dominio.ErroConflito{Msg: "já há uma janela de login aberta: entre por ela ou feche-a antes", Motivo: "login-aberto"}
	ErrEnviando     = &dominio.ErroConflito{Msg: "há uma candidatura sendo enviada agora; espere terminar", Estado: dominio.CandEnviando, Motivo: "enviando"}
)

func entrarNada(string) error { return ErrSemNavegador }

// desligado é o Controle que não faz nada (sem navegador).
func desligado() Controle {
	return Controle{Parar: nada, Suspender: suspenderNada, FecharNavegador: nada, MostrarEnvio: mostrarNada, Entrar: entrarNada, Janela: janelaNenhuma}
}

// intervaloDaFila: de quanto em quanto tempo o executor olha se há candidatura aprovada.
const intervaloDaFila = 2 * time.Minute

// Ligar roda o executor em segundo plano: ao abrir, Iniciar (enviando vira incerta e
// aprovação vencida volta a preparada), e depois EnviarAprovadas de tempos em tempos. O navegador abre SEM JANELA
// (jc-a2y.6.7): só a pessoa, pela tela, mostra o envio (MostrarEnvio) ou abre o login (Entrar).
func Ligar(ctx context.Context, arm *armazem.Armazem, pastaDados string) Controle {
	log.Printf("envio: executor ligado")
	av := aovivo.Novo(time.Now) // "Ao vivo" (jc-a2y.6.9): começa desligado a cada abertura do app
	nav, err := navegador.Novo(navegador.Opcoes{PastaDados: pastaDados, AoVivo: av})
	if err != nil {
		log.Printf("envio: navegador indisponível (%T); executor desligado", err)
		return desligado()
	}
	x := executor.Novo(arm, executor.Opcoes{Navegador: nav, PastaCurriculos: filepath.Join(pastaDados, "curriculos"), Etapa: av.Etapa})
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
	}
}
