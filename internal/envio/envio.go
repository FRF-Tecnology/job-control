// Package envio liga o executor de candidaturas no app (P5, jc-a2y.6.2.7).
//
// Até a jc-a2y.6.5 (primeiro envio de verdade, com o Vinicius), o envio fica DESLIGADO: no build padrão, Ligar não
// faz nada e o binário nem traz o executor nem o navegador (envio_padrao.go). O gatilho só existe num build de
// desenvolvimento com a tag envio_dev (envio_dev.go): go build -tags envio_dev ./app. Não há variável de ambiente
// nem botão que ligue o envio.
package envio

import (
	"context"

	"github.com/FRF-Tecnology/job-control/internal/aovivo"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Controle do envio ligado. No build padrão, nada faz nada.
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

// desligado é o Controle que não faz nada (build padrão, ou sem navegador).
func desligado() Controle {
	return Controle{Parar: nada, Suspender: suspenderNada, FecharNavegador: nada, MostrarEnvio: mostrarNada, Entrar: entrarNada, Janela: janelaNenhuma}
}
