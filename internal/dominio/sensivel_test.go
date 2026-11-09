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

// Espanhol e inglês (público LATAM, jc-a2y.6.2.6). Rótulos fictícios.
func TestMotivoDoRotuloLATAM(t *testing.T) {
	casos := map[string]string{
		// Senha.
		"Contraseña": "senha", "Contrasena *": "senha", "Crea una contraseña": "senha", "Clave": "senha", "Clave de acceso": "senha", "Clave secreta": "senha",
		// Número fiscal / seguridade social.
		"RFC": "cpf", "RFC con homoclave": "cpf", "CUIT": "cpf", "CUIL": "cpf", "CUIT/CUIL": "cpf", "RUT": "cpf", "RUC": "cpf", "NIT": "cpf",
		"CNPJ": "cpf", "SSN": "cpf", "Social Security Number": "cpf", "Número de seguro social": "cpf", "Tax ID": "cpf", "Taxpayer identification number": "cpf", "ITIN": "cpf",
		// Identidade.
		"DNI": "rg", "Documento Nacional de Identidad": "rg", "CURP": "rg", "Cédula de ciudadanía": "rg", "Cédula de identidad": "rg",
		"Número de documento": "rg", "National ID": "rg", "National identification number": "rg", "Identity card number": "rg", "Government-issued ID": "rg",
		// Documento.
		"Pasaporte": "documento", "Passport": "documento", "Passport number": "documento", "Licencia de conducir": "documento", "Comprobante de domicilio": "documento",
		// Banco.
		"Bank account": "dado-bancario", "Bank account number": "dado-bancario", "Cuenta bancaria": "dado-bancario", "Número de cuenta": "dado-bancario",
		"CLABE": "dado-bancario", "CLABE interbancaria": "dado-bancario", "Routing number": "dado-bancario", "CBU": "dado-bancario", "IBAN": "dado-bancario",
		// Dado sensível.
		"Date of birth": "dado-sensivel", "Birth date": "dado-sensivel", "DOB": "dado-sensivel", "Fecha de nacimiento": "dado-sensivel", "Data de nascimento": "dado-sensivel",
		"Race": "dado-sensivel", "Race/Ethnicity": "dado-sensivel", "Ethnicity": "dado-sensivel", "Raza": "dado-sensivel", "Etnia": "dado-sensivel",
		"Disability status": "dado-sensivel", "¿Tienes alguna discapacidad?": "dado-sensivel", "Discapacidad": "dado-sensivel",
		"Género": "dado-sensivel", "Orientación sexual": "dado-sensivel", "Religión": "dado-sensivel", "¿Estás embarazada?": "dado-sensivel", "Edad": "dado-sensivel", "Age": "dado-sensivel",
		// Termos e captcha.
		"Acepto los términos y condiciones": "termos", "Política de privacidad": "termos", "No soy un robot": "captcha",
		// Perguntas comuns que não são sensíveis (não pode bloquear).
		"¿Cuáles son tus habilidades clave?": "", "Palabras clave de tu perfil": "", "Experiencia con bases de datos": "", "¿Cuántos años de experiencia con Go?": "",
		"Disponibilidad para trabajo remoto": "", "Nivel de inglés": "", "Nombre completo": "", "Correo electrónico": "", "Teléfono": "", "Ciudad": "",
		"Years of experience with Rust": "", "How do you manage stakeholders?": "", "Average team size": "", "Desired salary": "", "Why do you want to work here?": "",
		"Como você age sob pressão?": "", "Identidade visual do produto": "", "Carta de presentación": "", "Hoja de vida": "", "Salario esperado": "",
	}
	for rotulo, quer := range casos {
		if got := MotivoDoRotulo(rotulo); got != quer {
			t.Errorf("MotivoDoRotulo(%q) = %q; queria %q", rotulo, got, quer)
		}
	}
}

// O pacote não aceita resposta a pergunta sensível em espanhol ou inglês, de nenhuma origem.
func TestPacoteSemRespostaSensivelLATAM(t *testing.T) {
	perguntas := []string{"Contraseña", "DNI", "CURP", "RFC", "RUT", "CUIT/CUIL", "CNPJ", "SSN", "National ID", "Tax ID", "Pasaporte",
		"Cuenta bancaria", "CLABE", "Routing number", "Fecha de nacimiento", "Date of birth", "Raza/etnia", "Race/Ethnicity", "Discapacidad", "Disability"}
	for _, pergunta := range perguntas {
		for _, origem := range []string{"pessoa", "ia", "respostasPadrao"} {
			c := conteudoOK()
			c["respostas"] = []any{Objeto{"pergunta": pergunta, "resposta": "x", "origem": origem}}
			if erros := ValidarConteudo(c); len(erros) == 0 {
				t.Errorf("%q (%s) aceita", pergunta, origem)
			}
		}
	}
	c := conteudoOK()
	c["respostas"] = []any{Objeto{"pergunta": "¿Cuáles son tus habilidades clave?", "resposta": "Go, SQL", "origem": "pessoa"}}
	if erros := ValidarConteudo(c); len(erros) > 0 {
		t.Errorf("pergunta comum recusada: %v", erros)
	}
}
