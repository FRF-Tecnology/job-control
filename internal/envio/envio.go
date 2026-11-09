// Package envio liga o executor de candidaturas no app (P5, jc-a2y.6.2.7).
//
// Até a jc-a2y.6.5 (primeiro envio de verdade, com o Vinicius), o envio fica DESLIGADO: no build padrão, Ligar não
// faz nada e o binário nem traz o executor nem o navegador (envio_padrao.go). O gatilho só existe num build de
// desenvolvimento com a tag envio_dev (envio_dev.go): go build -tags envio_dev ./app. Não há variável de ambiente
// nem botão que ligue o envio.
package envio

// Controle do envio ligado. No build padrão, nada faz nada.
//   - Parar: cancela o executor, espera o ciclo acabar e fecha o navegador esperando o processo sair (ao fechar o
//     app ou antes de apagar os dados). Para de vez.
//   - Suspender: impede ciclos novos e espera o atual acabar; devolve a função que solta (sair das plataformas).
//   - FecharNavegador: fecha o navegador e espera o processo sair (o perfil não pode estar em uso ao apagar).
type Controle struct {
	Parar           func()
	Suspender       func() (soltar func())
	FecharNavegador func()
}

func nada()                 {}
func suspenderNada() func() { return nada }
