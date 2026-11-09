// Package ia é o porte de nucleo/ia (Node): funções de negócio, guarda com as regras, prompts e provedores.
// O núcleo só conhece as funções (triarVaga, responderPergunta...); o provedor é detalhe.
package ia

// ErroIA é o erro único da camada de IA. Tipo diz de onde veio:
// entrada | config | provedor | json | esquema | regra.
// Só erro de provedor com Reintentavel (rede, 5xx, 429) aciona a rota reserva.
type ErroIA struct {
	Tipo         string
	Mensagem     string
	Regra        string
	Erros        []string
	Provedor     string
	Status       int
	Reintentavel bool
}

func (e *ErroIA) Error() string { return e.Mensagem }

func erroIA(tipo, mensagem string) *ErroIA { return &ErroIA{Tipo: tipo, Mensagem: mensagem} }
