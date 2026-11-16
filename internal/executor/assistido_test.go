package executor

import (
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Preenchimento assistido: dados fictícios.

var perfilAssistido = dominio.Objeto{"nome": "Fulana de Tal Ficticia", "email": "fulana@example.com", "telefone": "+55 11 90000-0000",
	"cidade": "São Paulo", "linkedin": "linkedin.com/in/fulana-ficticia", "github": "https://github.com/fulana",
	"respostasPadrao": []any{dominio.Objeto{"pergunta": "Você precisa de visto?", "resposta": "Não"}}}

func TestPlanejarAssistido(t *testing.T) {
	conteudo := dominio.ConteudoAssistido(perfilAssistido, dominio.Objeto{"curriculo": dominio.Objeto{"arquivo": "cv.pdf"}})
	campos := []Campo{
		{ID: "1", Tipo: CampoNome, Rotulo: "Nome", Obrigatorio: true},
		{ID: "2", Tipo: CampoPergunta, Rotulo: "Sobrenome *", Obrigatorio: true},
		{ID: "3", Tipo: CampoEmail, Rotulo: "E-mail", Obrigatorio: true},
		{ID: "4", Tipo: CampoTelefone, Rotulo: "Phone"},
		{ID: "5", Tipo: CampoPergunta, Rotulo: "LinkedIn Profile"},
		{ID: "6", Tipo: CampoPergunta, Rotulo: "GitHub"},
		{ID: "7", Tipo: CampoCurriculo, Rotulo: "Resume/CV", Obrigatorio: true},
		{ID: "8", Tipo: "cpf", Rotulo: "CPF", Obrigatorio: true},                         // sensível: nunca
		{ID: "9", Tipo: CampoPretensao, Rotulo: "Pretensão salarial", Obrigatorio: true}, // a pessoa preenche
		{ID: "10", Tipo: CampoPergunta, Rotulo: "Você precisa de visto?"},
		{ID: "11", Tipo: CampoPergunta, Rotulo: "Por que você quer trabalhar aqui?", Obrigatorio: true},
		{ID: "12", Tipo: CampoOutro, Rotulo: "Aceito os termos", Obrigatorio: true},
		{ID: "13", Tipo: CampoCidade, Rotulo: "Cidade", Preenchido: true}, // a página já trouxe: fica
		{ID: "14", Tipo: "senha", Rotulo: "Crie uma senha", Obrigatorio: true},
	}
	plano, faltando := PlanejarAssistido(conteudo, perfilAssistido, campos, true)
	quer := map[string]string{"1": "Fulana", "2": "de Tal Ficticia", "3": "fulana@example.com", "4": "+55 11 90000-0000",
		"5": "https://linkedin.com/in/fulana-ficticia", "6": "https://github.com/fulana", "7": "[anexo]", "10": "Não"}
	got := map[string]string{}
	for _, p := range plano {
		v := p.Valor
		if p.Anexar {
			v = "[anexo]"
		}
		got[p.Campo.ID] = v
	}
	for id, v := range quer {
		if got[id] != v {
			t.Errorf("campo %s = %q, quer %q", id, got[id], v)
		}
	}
	if len(got) != len(quer) {
		t.Errorf("preencheu demais: %v", got)
	}
	// obrigatórios em branco: CPF, pretensão, a pergunta aberta, os termos e a senha
	if faltando != 5 {
		t.Errorf("faltando = %d, quer 5", faltando)
	}
}

func TestPlanejarAssistidoSemCurriculoENomeCompleto(t *testing.T) {
	campos := []Campo{{ID: "1", Tipo: CampoNome, Rotulo: "Nome completo"}, {ID: "2", Tipo: CampoCurriculo, Rotulo: "Currículo", Obrigatorio: true}}
	plano, faltando := PlanejarAssistido(dominio.ConteudoAssistido(perfilAssistido, nil), perfilAssistido, campos, false)
	if len(plano) != 1 || plano[0].Valor != "Fulana de Tal Ficticia" || faltando != 1 {
		t.Fatalf("plano %v, faltando %d", plano, faltando)
	}
}

func TestLinkPublico(t *testing.T) {
	for v, quer := range map[string]string{
		"linkedin.com/in/x":                 "https://linkedin.com/in/x",
		"https://www.linkedin.com/in/x":     "https://www.linkedin.com/in/x",
		"http://linkedin.com/in/x":          "",
		"javascript:alert(1)//linkedin.com": "",
		"":                                  "",
	} {
		if got := linkPublico(v, "linkedin.com"); got != quer {
			t.Errorf("linkPublico(%q) = %q, quer %q", v, got, quer)
		}
	}
}
