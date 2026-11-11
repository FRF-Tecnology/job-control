// Package notas emite a NFS-e de cada fatura paga do Stripe por um emissor externo (NFE.io), com uma fila
// de tentativas no Postgres. O CPF/CNPJ do cliente só passa pela memória: vem do Stripe e vai ao emissor,
// nunca para o nosso banco nem para o log; erros guardados são genéricos (status ou classe).
package notas

import (
	"context"
	"fmt"
)

// Pedido é o que o emissor precisa para uma nota. Documento é o tax id do Stripe (não é gravado).
type Pedido struct {
	InvoiceID, UsuarioID, Nome, Documento, Email, Pais string
	ValorCentavos                                      int64
	Moeda                                              string
	Endereco                                           map[string]string // linha1, linha2, cidade, estado, cep
}

// Emissor emite uma nota e devolve o número e o link do PDF (o link pode vir vazio).
// Os erros devolvidos têm de ser genéricos: nunca o corpo da resposta nem dados do cliente.
type Emissor interface {
	Emitir(ctx context.Context, p Pedido) (numero, linkPDF string, err error)
}

// ErroEmissor é a falha do emissor em termos seguros para gravar em notas.erro.
type ErroEmissor struct {
	Status int    // HTTP, 0 se não chegou resposta
	Classe string // ex.: "rede", "resposta ilegível", "pedido incompleto", "falso"
}

func (e *ErroEmissor) Error() string {
	if e.Status != 0 {
		return fmt.Sprintf("emissor respondeu HTTP %d", e.Status)
	}
	return "emissor: " + e.Classe
}
