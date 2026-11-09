package dominio

import "testing"

// Rótulos fictícios, em português e inglês, com e sem acento.
func TestMotivoDoRotulo(t *testing.T) {
	casos := map[string]string{
		"CPF *": "cpf", "Informe seu CPF": "cpf", "Cadastro de Pessoa Física": "cpf",
		"RG": "rg", "Número de identidade": "rg", "Órgão emissor": "rg",
		"Senha": "senha", "Crie uma senha": "senha", "Password": "senha",
		"Agência": "dado-bancario", "Conta corrente": "dado-bancario", "Chave PIX": "dado-bancario", "Bank account number": "dado-bancario",
		"Anexe sua CNH": "documento", "Comprovante de residência": "documento", "Passaporte": "documento", "Carteira de trabalho (CTPS)": "documento",
		"Raça/cor": "dado-sensivel", "Você é PcD?": "dado-sensivel", "Possui alguma deficiência?": "dado-sensivel",
		"Identidade de gênero": "dado-sensivel", "Orientação sexual": "dado-sensivel", "Religião": "dado-sensivel", "Gender": "dado-sensivel", "Pronouns": "dado-sensivel",
		"Li e concordo com os termos de uso": "termos", "Aceito a política de privacidade": "termos", "I agree to the Terms": "termos", "Consentimento LGPD": "termos",
		"Não sou um robô": "captcha",
		// Perguntas comuns que não são sensíveis.
		"Anos de experiência com Go": "", "Experiência com banco de dados?": "", "Disponível para trabalho remoto?": "",
		"Nível de inglês": "", "Pretensão salarial (PJ)": "", "Nome completo": "", "E-mail": "", "Telefone": "",
		"Por que você quer trabalhar aqui?": "", "LinkedIn": "", "Cidade": "", "Corporativo": "", "Contratação": "", "": "",
	}
	for rotulo, quer := range casos {
		if got := MotivoDoRotulo(rotulo); got != quer {
			t.Errorf("MotivoDoRotulo(%q) = %q; queria %q", rotulo, got, quer)
		}
	}
}

// O pacote não pode trazer resposta para pergunta sensível, de nenhuma origem.
func TestPacoteSemRespostaSensivel(t *testing.T) {
	for _, pergunta := range []string{"Qual sua senha?", "Informe a agência", "Raça/cor", "Número do RG", "Possui CNH?"} {
		for _, origem := range []string{"pessoa", "ia", "respostasPadrao"} {
			c := conteudoOK()
			c["respostas"] = []any{Objeto{"pergunta": pergunta, "resposta": "x", "origem": origem}}
			if erros := ValidarConteudo(c); len(erros) == 0 {
				t.Errorf("%q (%s) aceita", pergunta, origem)
			}
		}
	}
	// Aceite de termos não é resposta de texto: não recusa o pacote (a página bloqueia na hora).
	c := conteudoOK()
	c["respostas"] = []any{Objeto{"pergunta": "Anos de Go?", "resposta": "3", "origem": "pessoa"}}
	if erros := ValidarConteudo(c); len(erros) > 0 {
		t.Errorf("pacote bom recusado: %v", erros)
	}
}
