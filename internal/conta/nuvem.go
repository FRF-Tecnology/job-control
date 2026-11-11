// Package conta é o lado do app da conta, da licença e da trava: o cliente da nuvem
// (HTTP e uma versão falsa em memória para testes e para o modo de desenvolvimento).
package conta

import (
	"context"
	"errors"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/licenca"
)

// Par é a sessão que a nuvem devolve: token de acesso (curto) e de renovação (uso único).
type Par struct {
	Acesso    string    `json:"acesso"`
	Renovacao string    `json:"renovacao"`
	AcessoAte time.Time `json:"acesso_ate"`
}

// Nota é uma nota fiscal da conta.
type Nota struct {
	Numero   string    `json:"numero"`
	LinkPDF  string    `json:"link_pdf"`
	Estado   string    `json:"estado"`
	CriadaEm time.Time `json:"criada_em"`
}

// ErroNuvem é uma resposta de erro da nuvem. Msg é a frase da nuvem, mostrada como está;
// Motivo ("termos", "confirmar", "entrar-com-senha") diz ao app o que fazer; Versao é a
// versão atual dos termos quando Motivo é "termos".
type ErroNuvem struct {
	Status int
	Msg    string
	Motivo string
	Versao string
}

func (e *ErroNuvem) Error() string { return e.Msg }

// ErrSemInternet: qualquer falha de rede ou de tempo esgotado.
var ErrSemInternet = errors.New("sem internet")

// Nuvem é o que o app usa da nuvem (contrato em api/nuvem.yaml).
type Nuvem interface {
	Criar(ctx context.Context, email, senha, termos string) error
	Confirmar(ctx context.Context, email, codigo, senha, dispositivo, termos string) (Par, error)
	Entrar(ctx context.Context, email, senha, dispositivo, termos string) (Par, error)
	Recuperar(ctx context.Context, email string) error
	NovaSenha(ctx context.Context, email, codigo, senha, dispositivo string) (Par, error)
	Google(ctx context.Context, code, verifier, redirectURI, dispositivo, termos string) (Par, error)
	Renovar(ctx context.Context, renovacao string) (Par, error)
	Sair(ctx context.Context, renovacao string) error
	Licenca(ctx context.Context, acesso, dispositivo string) (licenca.Documento, error)
	Assinar(ctx context.Context, acesso string) (url string, err error)
	Portal(ctx context.Context, acesso string) (url string, err error)
	Notas(ctx context.Context, acesso string) ([]Nota, error)
	Excluir(ctx context.Context, acesso, email string) error
}
