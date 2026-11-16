package executor

import (
	"maps"
	"strings"
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
		"linkedin.com/in/x":                  "https://linkedin.com/in/x",
		"https://www.linkedin.com/in/x":      "https://www.linkedin.com/in/x",
		"http://linkedin.com/in/x":           "https://linkedin.com/in/x", // o LinkedIn só responde em https
		"https://golpe.example/linkedin.com": "",
		"javascript:alert(1)//linkedin.com":  "",
		"":                                   "",
	} {
		if got := linkPublico(v, "linkedin.com"); got != quer {
			t.Errorf("linkPublico(%q) = %q, quer %q", v, got, quer)
		}
	}
}

func TestPlanejarAssistidoNomeCidadeEstadoSeparados(t *testing.T) {
	perfil := dominio.Objeto{"nome": "Maria Clara Souza Ficticia", "primeiroNome": "Maria Clara", "sobrenome": "Souza Ficticia",
		"cidade": "Recife", "estado": "PE", "paisResidencia": "BR"}
	campos := []Campo{
		{ID: "1", Tipo: CampoPergunta, Rotulo: "First name"},
		{ID: "2", Tipo: CampoPergunta, Rotulo: "Last name"},
		{ID: "3", Tipo: CampoNome, Rotulo: "Nome completo"},
		{ID: "4", Tipo: CampoCidade, Rotulo: "Cidade"},
		{ID: "5", Tipo: CampoPergunta, Rotulo: "Estado"},
		{ID: "6", Tipo: CampoOutro, Rotulo: "Country"},
		{ID: "7", Tipo: CampoCidade, Rotulo: "Location"},
		{ID: "8", Tipo: CampoPergunta, Rotulo: "Estado civil"}, // não é o estado onde mora
	}
	plano, _ := PlanejarAssistido(dominio.ConteudoAssistido(perfil, nil), perfil, campos, false)
	type v struct {
		valor string
		alt   string
	}
	quer := map[string]v{"1": {"Maria Clara", ""}, "2": {"Souza Ficticia", ""}, "3": {"Maria Clara Souza Ficticia", ""},
		"4": {"Recife", ""}, "5": {"Pernambuco", "PE"}, "6": {"Brasil", "Brazil BR"}, "7": {"Recife, PE", ""}}
	got := map[string]v{}
	for _, p := range plano {
		got[p.Campo.ID] = v{p.Valor, strings.Join(p.Alternativas, " ")}
	}
	if !maps.Equal(got, quer) {
		t.Errorf("plano:\n%v\nquer\n%v", got, quer)
	}
	// Perfil antigo, só com o nome completo e a cidade junto do estado: o nome é dividido na primeira palavra.
	antigo := dominio.Objeto{"nome": "Fulana de Tal", "cidade": "Recife, PE"}
	plano, _ = PlanejarAssistido(dominio.ConteudoAssistido(antigo, nil), antigo, campos[:2], false)
	if len(plano) != 2 || plano[0].Valor != "Fulana" || plano[1].Valor != "de Tal" {
		t.Errorf("perfil antigo: %v", plano)
	}
}

func TestTelefoneComESemCodigoDoPais(t *testing.T) {
	for tel, quer := range map[string]string{
		"+55 34 99999-0000": "34 99999-0000",
		"+351 912 345 678":  "912 345 678",
		"34 99999-0000":     "34 99999-0000",
		"+5534999990000":    "+5534999990000", // sem separador não dá para saber onde o código acaba
		"":                  "",
	} {
		if got := telefoneSemDDI(tel); got != quer {
			t.Errorf("telefoneSemDDI(%q) = %q, quer %q", tel, got, quer)
		}
	}
	perfil := dominio.Objeto{"ddi": "+55", "telefone": "34 99999-0000"}
	conteudo := dominio.ConteudoAssistido(perfil, nil)
	tel := Campo{ID: "t", Tipo: CampoTelefone, Rotulo: "Phone"}
	ddi := Campo{ID: "d", Tipo: CampoDDI, Rotulo: "Country code"}
	valorDe := func(plano []Preenchimento, id string) string {
		for _, p := range plano {
			if p.Campo.ID == id {
				return p.Valor
			}
		}
		return ""
	}
	// Site de fora, sem campo de código: o número completo.
	if p, _ := PlanejarAssistido(conteudo, perfil, []Campo{tel}, false); valorDe(p, "t") != "+55 34 99999-0000" {
		t.Errorf("site de fora: %v", p)
	}
	// Com o campo do código: cada um no seu.
	if p, _ := PlanejarAssistido(conteudo, perfil, []Campo{ddi, tel}, false); valorDe(p, "t") != "34 99999-0000" || valorDe(p, "d") != "+55" {
		t.Errorf("com código à parte: %v", p)
	}
	// Site brasileiro: o número nacional.
	nacional := maps.Clone(conteudo)
	nacional["siteNacional"] = true
	if p, _ := PlanejarAssistido(nacional, perfil, []Campo{tel}, false); valorDe(p, "t") != "34 99999-0000" {
		t.Errorf("site brasileiro: %v", p)
	}
	// O envio (LinkedIn) também: o código no seletor, o número sem ele.
	plano, motivo := planejar(dominio.Objeto{"contato": dominio.Objeto{"telefone": "+55 34 99999-0000"}}, []Campo{ddi, tel})
	if motivo != "" || len(plano) != 2 || plano[0].valor != "+55" || plano[1].valor != "34 99999-0000" {
		t.Errorf("envio: %+v %q", plano, motivo)
	}
}
