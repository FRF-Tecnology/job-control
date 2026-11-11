package api

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/FRF-Tecnology/job-control/internal/curriculos"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Valores canônicos do formulário, com o vocabulário que pode sustentá-los no PDF.
var niveisCV = map[string]string{
	"Estágio":   "estagio|estagiari[oa]|intern|internship",
	"Júnior":    "junior|jr|entry.level",
	"Pleno":     "pleno|mid.level|midlevel|intermediate",
	"Sênior":    "senior|sr",
	"Liderança": "lider|lead|head|manager|gerente|coordenador[a]?|diretor[a]?|director",
}
var inglesCV = map[string]string{
	"Não falo inglês": "nao falo|sem conhecimento|no proficiency|no english",
	"Básico":          "basico|basic|beginner|elementary|a1|a2",
	"Intermediário":   "intermediario|intermediate|b1|b2",
	"Avançado":        "avancado|advanced|c1",
	"Fluente":         "fluente|fluent|nativo|native|proficient|bilingue|bilingual|c2",
}
var negacaoCV = regexp.MustCompile(`\b(nao|not|never|sem|without)\b`)

func valorExplicito(campo, valor, evidencia string) bool {
	ev := dominio.Normalizar(evidencia)
	var expresso string
	switch campo {
	case "ingles":
		expresso = inglesCV[valor]
		if expresso == "" || valor != "Não falo inglês" && negacaoCV.MatchString(ev) {
			return false
		}
		idioma := `\b(?:ingles|english)\b`
		nivel := `\b(?:` + expresso + `)\b`
		// Exige proximidade: "Português fluente; inglês básico" sustenta apenas Básico.
		entre := `[\s():,–—-]*(?:(?:at|em|nivel|level|proficiency|profissional|professional|in|de|no|do|idioma)\b[\s():,–—-]*){0,3}`
		return regexp.MustCompile(idioma+entre+nivel+"|"+nivel+entre+idioma).MatchString(ev) ||
			valor == "Não falo inglês" && strings.Contains(ev, "no english")
	case "niveis":
		expresso = niveisCV[valor]
	default:
		v := dominio.Normalizar(valor)
		return v != "" && regexp.MustCompile(`(?:^|[^\pL\pN_])`+regexp.QuoteMeta(v)+`(?:$|[^\pL\pN_])`).MatchString(ev)
	}
	return expresso != "" && regexp.MustCompile(`\b(?:`+expresso+`)\b`).MatchString(ev)
}

func sugestoesComFonte(texto string, saida Objeto) []Objeto {
	fonte := strings.Join(strings.Fields(texto), " ")
	itens := []Objeto{}
	vistos := map[string]bool{}
	for _, s := range dominio.ListaDe(saida["sugestoes"]) {
		item := dominio.ObjetoDe(s)
		campo, _ := item["campo"].(string)
		valor := strings.TrimSpace(dominio.Texto(item["valor"]))
		evidencia := strings.Join(strings.Fields(dominio.Texto(item["evidencia"])), " ")
		if evidencia == "" || utf8.RuneCountInString(evidencia) > 280 || utf8.RuneCountInString(valor) > 120 ||
			!strings.Contains(fonte, evidencia) || !valorExplicito(campo, valor, evidencia) {
			continue
		}
		chave := campo + "/" + strings.ToLower(valor)
		if campo == "cargo" || campo == "ingles" {
			chave = campo
		}
		if campo == "stackPrincipal" || campo == "stackSecundaria" {
			chave = "stack/" + strings.ToLower(valor)
		}
		if vistos[chave] {
			continue
		}
		vistos[chave] = true
		itens = append(itens, Objeto{"campo": campo, "valor": valor, "evidencia": evidencia})
		if len(itens) >= 32 {
			break
		}
	}
	return itens
}

func (a *API) sugerirPeloCurriculo(ctx context.Context, autor string, dados Objeto) (Resposta, error) {
	if autor != "eu" {
		return Resposta{}, falha(403, msgForaDoAssistente)
	}
	if err := soCampos(dados, "curriculo"); err != nil {
		return Resposta{}, err
	}
	arquivo, err := pdfDoOnboarding(dados)
	if err != nil {
		return Resposta{}, err
	}
	iaPerfil, err := a.novaIA(ctx)
	if err != nil {
		return Resposta{}, err
	}
	if !iaPerfil.Configurada() {
		return Resposta{}, falha(409, "conecte uma IA antes de usar as sugestões do currículo")
	}
	texto, err := curriculos.TextoProfissional(ctx, arquivo)
	if errors.Is(err, curriculos.ErrSemTexto) {
		return Resposta{}, falha(422, "não consegui ler o texto deste PDF. Use um PDF com texto selecionável ou preencha o perfil manualmente")
	}
	if err != nil {
		return Resposta{}, err
	}
	saida, err := iaPerfil.Executar(ctx, "sugerirPerfilDoCV", Objeto{"curriculo": texto})
	if err != nil {
		return respostaIA(err)
	}
	return ok(Objeto{"sugestoes": sugestoesComFonte(texto, saida)})
}
