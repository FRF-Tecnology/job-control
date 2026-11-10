package dominio

import "testing"

// jc-a2y.7.5: o perfil não guarda resposta para pergunta de número fiscal, documento de identidade, senha ou banco.
// Resposta de sim/não não é o dado e passa. Dados fictícios.
func TestRespostaPadraoSensivel(t *testing.T) {
	par := func(pergunta, resposta string) Objeto {
		return Objeto{"respostasPadrao": []any{
			Objeto{"pergunta": "Anos com Go?", "resposta": "5"},
			Objeto{"pergunta": pergunta, "resposta": resposta},
		}}
	}
	recusa := map[string]string{
		"Agência e conta para depósito": "0001 / 12345-6",
		"Chave PIX":                     "pessoa.teste@exemplo.com",
		"Bank account number":           "000123",
		"IBAN":                          "XX00 0000 0000",
		"Número de cuenta":              "000-000",
		"Contraseña del portal":         "ficticia",
		"PIN":                           "0000",
		"Senha de acesso":               "senha-ficticia",
		"RFC":                           "XAXX010101000",
		"DNI":                           "00.000.000",
		"Número de documento":           "A000000",
		"SSN":                           "last four 0000",
		"Qual o seu CNPJ?":              "sim, é 00.000.000/0001-00",
	}
	for pergunta, resposta := range recusa {
		if got := RespostaPadraoSensivel(par(pergunta, resposta)); got != pergunta {
			t.Errorf("%q com %q: devolveu %q; queria a pergunta", pergunta, resposta, got)
		}
	}
	passa := map[string]string{
		"Possui CNPJ para emitir nota?":   "Sim",
		"Do you have a US bank account?":  "No, only in Brazil",
		"¿Tienes RFC?":                    "Sí",
		"Tem conta bancária no exterior?": "Não, só no Brasil",
		"Qual o seu CPF?":                 "",
		"Senha de acesso":                 "  ",
		"IBAN":                            "N/A",
		"Pretensão salarial":              "USD 5.000",
		"Tem passaporte válido?":          "Sim, até 2030",
		"Nível de inglês":                 "Avançado",
	}
	for pergunta, resposta := range passa {
		if got := RespostaPadraoSensivel(par(pergunta, resposta)); got != "" {
			t.Errorf("%q com %q: recusou (%q)", pergunta, resposta, got)
		}
	}
	for _, v := range []any{nil, Objeto{}, Objeto{"respostasPadrao": "texto"}, Objeto{"respostasPadrao": []any{"solto", nil, 3}}} {
		if got := RespostaPadraoSensivel(v); got != "" {
			t.Errorf("%v: devolveu %q", v, got)
		}
	}
}
