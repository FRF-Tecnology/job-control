package dominio

import "testing"

// CPFs gerados só para teste (dígitos verificadores calculados), de ninguém.
func TestCPF(t *testing.T) {
	for entrada, quer := range map[string]string{
		"529.982.247-25":  "52998224725",
		"52998224725":     "52998224725",
		" 529 982 247 25": "52998224725",
		"529.982.247-24":  "", // dígito errado
		"111.111.111-11":  "", // todos iguais
		"5299822472":      "", // curto
		"529.982.247-25a": "",
		"":                "",
	} {
		got, ok := CPFValido(entrada)
		if got != quer || ok != (quer != "") {
			t.Errorf("CPFValido(%q) = %q %v, quer %q", entrada, got, ok, quer)
		}
	}
	if FormatarCPF("52998224725") != "529.982.247-25" || MascararCPF("52998224725") != "***.982.247-**" {
		t.Errorf("formato: %q %q", FormatarCPF("52998224725"), MascararCPF("52998224725"))
	}
	for rotulo, quer := range map[string]bool{
		"CPF *": true, "Seu CPF": true, "CPF/CNPJ": false, "CNPJ": false, "Cadastro de Pessoa Física": true,
		"SSN": false, "NIF": false, "Tax ID": false, "Nome": false,
	} {
		if RotuloDeCPF(rotulo) != quer {
			t.Errorf("RotuloDeCPF(%q) = %v", rotulo, !quer)
		}
	}
}
