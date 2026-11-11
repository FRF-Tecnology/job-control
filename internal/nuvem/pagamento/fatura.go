package pagamento

import (
	"context"
	"strings"

	"github.com/stripe/stripe-go/v87"
)

// Fatura é o que a nota fiscal precisa de uma fatura paga. Documento (CPF/CNPJ) vem do Stripe só para
// ser repassado ao emissor: nunca vai para o nosso banco nem para o log.
type Fatura struct {
	Nome          string
	Email         string
	Documento     string
	Pais          string // ISO 3166-1 alfa-2 do endereço de cobrança
	ValorCentavos int64
	Moeda         string // maiúsculas: BRL, USD...
	Endereco      map[string]string
}

// Fatura busca a fatura no Stripe (nome, tax id, endereço, valor pago).
func (r *Real) Fatura(ctx context.Context, id string) (Fatura, error) {
	inv, err := r.api.V1Invoices.Retrieve(ctx, id, nil)
	if err != nil {
		return Fatura{}, err
	}
	return faturaDe(inv), nil
}

// faturaDe pega só tax id brasileiro (br_cpf, depois br_cnpj); sem eles, Documento fica vazio.
func faturaDe(inv *stripe.Invoice) Fatura {
	f := Fatura{Nome: inv.CustomerName, Email: inv.CustomerEmail, ValorCentavos: inv.AmountPaid,
		Moeda: strings.ToUpper(string(inv.Currency)), Endereco: map[string]string{}}
	if a := inv.CustomerAddress; a != nil {
		f.Pais = strings.ToUpper(a.Country)
		for k, v := range map[string]string{"linha1": a.Line1, "linha2": a.Line2, "cidade": a.City,
			"estado": a.State, "cep": a.PostalCode} {
			if v != "" {
				f.Endereco[k] = v
			}
		}
	}
	// Só CPF/CNPJ vão para a nota; outro tipo (ex.: CUIT argentino) fica de fora: o contador define o caso estrangeiro.
	for _, t := range inv.CustomerTaxIDs {
		if t != nil && t.Type != nil && (*t.Type == stripe.TaxIDTypeBRCPF || *t.Type == stripe.TaxIDTypeBRCNPJ) {
			f.Documento = t.Value
			if *t.Type == stripe.TaxIDTypeBRCPF {
				break // CPF vence; se só houver CNPJ, fica o CNPJ
			}
		}
	}
	return f
}

// DefinirFatura cadastra a fatura que o Falso devolve para o id.
func (f *Falso) DefinirFatura(id string, fat Fatura) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.faturas == nil {
		f.faturas = map[string]Fatura{}
	}
	f.faturas[id] = fat
}

func (f *Falso) Fatura(_ context.Context, id string) (Fatura, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fat, ok := f.faturas[id]
	if !ok {
		return Fatura{}, ErrNaoExiste
	}
	return fat, nil
}
