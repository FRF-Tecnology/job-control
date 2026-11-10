package dominio

import (
	"encoding/json"
	"os"
	"testing"
)

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

// Vetores compartilhados com a tela (app/frontend/src/app/sensivel.test.ts lê o mesmo arquivo): mudar a regra
// num lado sem o outro quebra o CI. Rótulos fictícios, em português, espanhol e inglês.
func TestVetoresSensiveis(t *testing.T) {
	b, err := os.ReadFile("testdata/sensivel.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Casos []struct {
			Texto  string `json:"texto"`
			Motivo string `json:"motivo"`
		} `json:"casos"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatal(err)
	}
	porMotivo := map[string]int{}
	for _, c := range v.Casos {
		porMotivo[c.Motivo]++
		if got := MotivoDoRotulo(c.Texto); got != c.Motivo {
			t.Errorf("MotivoDoRotulo(%q) = %q; queria %q", c.Texto, got, c.Motivo)
		}
		quer := c.Motivo
		if quer == "termos" || quer == "captcha" {
			quer = ""
		}
		if got := perguntaSensivel(c.Texto); got != quer {
			t.Errorf("perguntaSensivel(%q) = %q; queria %q", c.Texto, got, quer)
		}
	}
	// Guarda contra o arquivo encolher em silêncio: todos os motivos e os casos que não podem bloquear.
	for _, m := range []string{"cpf", "rg", "senha", "dado-bancario", "documento", "dado-sensivel", "termos", "captcha", ""} {
		if porMotivo[m] < 3 {
			t.Errorf("vetores com só %d caso(s) de %q", porMotivo[m], m)
		}
	}
	if len(v.Casos) < 120 {
		t.Fatalf("só %d vetores", len(v.Casos))
	}
}
