package pagamento

import (
	"testing"

	"github.com/stripe/stripe-go/v87"
)

func taxID(tipo stripe.TaxIDType, v string) *stripe.InvoiceCustomerTaxID {
	return &stripe.InvoiceCustomerTaxID{Type: &tipo, Value: v}
}

func TestFaturaDeDocumento(t *testing.T) {
	casos := []struct {
		nome string
		ids  []*stripe.InvoiceCustomerTaxID
		want string
	}{
		{"cpf preferido", []*stripe.InvoiceCustomerTaxID{taxID(stripe.TaxIDTypeBRCNPJ, "cnpj"), taxID(stripe.TaxIDTypeBRCPF, "cpf")}, "cpf"},
		{"cnpj", []*stripe.InvoiceCustomerTaxID{taxID(stripe.TaxIDTypeARCUIT, "20123456786"), taxID(stripe.TaxIDTypeBRCNPJ, "cnpj")}, "cnpj"},
		{"só estrangeiro", []*stripe.InvoiceCustomerTaxID{taxID(stripe.TaxIDTypeARCUIT, "20123456786")}, ""},
		{"nenhum", nil, ""},
	}
	for _, c := range casos {
		f := faturaDe(&stripe.Invoice{CustomerTaxIDs: c.ids, AmountPaid: 500, Currency: "usd",
			CustomerAddress: &stripe.Address{Country: "br", City: "X"}})
		if f.Documento != c.want {
			t.Errorf("%s: documento = %q, queria %q", c.nome, f.Documento, c.want)
		}
		if f.Moeda != "USD" || f.Pais != "BR" || f.ValorCentavos != 500 || f.Endereco["cidade"] != "X" {
			t.Errorf("%s: fatura = %+v", c.nome, f)
		}
	}
}
