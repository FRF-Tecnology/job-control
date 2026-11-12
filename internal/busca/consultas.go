package busca

import (
	"regexp"
	"slices"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

var termosProfissionais = regexp.MustCompile(`[\pL\pN.+#]+`)
var grafiasCompostas = strings.NewReplacer("back-end", "backend", "back end", "backend", "front-end", "frontend", "front end", "frontend", "full-stack", "fullstack", "full stack", "fullstack")
var sinonimos = map[string]string{
	"de": "", "do": "", "da": "", "em": "", "e": "", "of": "", "a": "", "o": "",
	"desenvolvimento": "developer", "desenvolvedor": "developer", "desenvolvedora": "developer",
	"development": "developer", "entwickler": "developer", "entwicklerin": "developer", "softwareentwickler": "software developer",
	"softwareentwicklung": "software developer", "softwareentwicklerin": "software developer",
	"engenheiro": "engineer", "engenheira": "engineer", "engenharia": "engineer", "engineering": "engineer",
	"analista": "analyst", "analise": "analyst", "dados": "data", "daten": "data",
	"react.js": "react", "reactjs": "react", "node.js": "nodejs",
}

func palavras(s string) []string {
	s = grafiasCompostas.Replace(dominio.Normalizar(s))
	var saida []string
	for _, p := range termosProfissionais.FindAllString(s, -1) {
		p = strings.TrimRight(p, ".")
		if traducao, ok := sinonimos[p]; ok {
			p = traducao
		}
		saida = append(saida, strings.Fields(p)...)
	}
	return saida
}

type familiaDeCargo struct {
	marcadores []string
	titulos    []string
}

var familias = []familiaDeCargo{
	{[]string{"machine learning", "aprendizado maquina", "ai engineer", "engineer ia", "inteligencia artificial", "artificial intelligence", "engineer ml", "ml engineer"}, []string{"Machine Learning Engineer", "Engenheiro de IA", "AI Engineer"}},
	{[]string{"data scientist", "cientista data"}, []string{"Data Scientist", "Cientista de Dados"}},
	{[]string{"data engineer", "engineer data"}, []string{"Data Engineer", "Engenheiro de Dados"}},
	{[]string{"data analyst", "analyst data"}, []string{"Data Analyst", "Analista de Dados"}},
	{[]string{"backend"}, []string{"Backend Engineer", "Desenvolvedor Backend", "Backend Developer"}},
	{[]string{"frontend"}, []string{"Frontend Engineer", "Desenvolvedor Frontend", "Frontend Developer"}},
	{[]string{"fullstack"}, []string{"Full Stack Developer", "Desenvolvedor Full Stack", "Full Stack Engineer"}},
	{[]string{"devops"}, []string{"DevOps Engineer", "Engenheiro DevOps"}},
	{[]string{"product designer", "designer produto"}, []string{"Product Designer", "Designer de Produto"}},
	{[]string{"product manager", "gerente produto"}, []string{"Product Manager", "Gerente de Produto"}},
	{[]string{"customer support", "atendimento ao cliente", "suporte ao cliente"}, []string{"Customer Support", "Atendimento ao Cliente"}},
	{[]string{"financial analyst", "analyst financeiro", "analyst financeira"}, []string{"Financial Analyst", "Analista Financeiro"}},
	{[]string{"account executive", "executivo contas", "executiva contas"}, []string{"Account Executive", "Executivo de Contas"}},
	{[]string{"nurse", "enfermeiro", "enfermeira"}, []string{"Nurse", "Enfermeiro", "Enfermeira"}},
	{[]string{"software engineer", "engineer software", "software developer", "developer software"}, []string{"Software Engineer", "Desenvolvedor de Software", "Software Developer"}},
}

// Skills só complementam um cargo de desenvolvimento revisado no perfil.
// Para os perfis antigos com apenas uma linguagem como cargo, forme títulos.
func consultasDoPerfil(perfil dominio.Objeto) []string {
	cargo := texto(dominio.Texto(perfil["cargo"]), 120)
	if cargo == "" || dominio.Normalizar(cargo) == dominio.Normalizar(perfil["nome"]) {
		return nil
	}
	tokens := palavras(cargo)
	nome := " " + strings.Join(tokens, " ") + " "
	if len(tokens) == 0 {
		return nil
	}
	termos := []string{cargo}
	desenvolvimento := slices.Contains(tokens, "backend") || slices.Contains(tokens, "frontend") || slices.Contains(tokens, "fullstack") || slices.Contains(tokens, "developer") || (slices.Contains(tokens, "engineer") && slices.Contains(tokens, "software"))
	linguagem := linguagemDoTitulo(tokens)
	soLinguagem := len(tokens) == 1 && linguagem != ""
	if desenvolvimento && linguagem == "" {
		linguagem = linguagemDoTitulo(palavras(strings.Join(listaTextos(perfil["stackPrincipal"]), " ")))
	}
	if linguagem != "" && (soLinguagem || desenvolvimento || slices.Contains(tokens, "engineer")) {
		if soLinguagem {
			termos = nil
		}
		termos = append(termos, linguagem+" Engineer", "Desenvolvedor "+linguagem, linguagem+" Developer")
	}
	for _, f := range familias {
		if slices.ContainsFunc(f.marcadores, func(m string) bool { return strings.Contains(nome, " "+m+" ") }) {
			termos = append(termos, f.titulos...)
			break
		}
	}
	if linguagem != "" && !soLinguagem && (desenvolvimento || slices.Contains(tokens, "engineer")) {
		termos = append(termos, "Backend Engineer", "Software Engineer")
	}
	if eCargoIA(tokens) {
		termos = append(termos, "Software Engineer")
	}
	// Variações revisadas pela pessoa têm precedência sobre o vocabulário sugerido.
	revisados := []string{termos[0]}
	for _, t := range listaTextos(perfil["titulosEquivalentes"]) {
		if alias := texto(t, 120); alias != "" && dominio.Normalizar(alias) != dominio.Normalizar(perfil["nome"]) {
			revisados = append(revisados, alias)
		}
	}
	termos = append(revisados, termos[1:]...)
	vistos := map[string]bool{}
	var unicos []string
	for _, t := range termos {
		n := dominio.Normalizar(t)
		if !vistos[n] {
			vistos[n] = true
			unicos = append(unicos, t)
		}
		if len(unicos) == 7 {
			break
		}
	}
	return unicos
}

func linguagemDoTitulo(palavras []string) string {
	for _, p := range palavras {
		for _, linguagem := range []string{"Java", "JavaScript", "Python", "Kotlin", "C#", "PHP", "Ruby", "TypeScript", "Golang"} {
			if strings.EqualFold(p, linguagem) {
				return linguagem
			}
		}
	}
	return ""
}

// As fontes recebem apenas o código do país informado no perfil.
func codigoPais(nome string) string {
	nome = dominio.Normalizar(nome)
	if nome == "" {
		return ""
	}
	if r, err := language.ParseRegion(nome); err == nil && r.IsCountry() {
		return r.String()
	}
	pt, en := display.Regions(language.Portuguese), display.Regions(language.English)
	for _, r := range language.Supported.Regions() {
		if r.IsCountry() && (dominio.Normalizar(pt.Name(r)) == nome || dominio.Normalizar(en.Name(r)) == nome) {
			return r.String()
		}
	}
	return ""
}

// No feed sem busca por cargo, uma tag isolada também aparece em funções alheias.
func combinaTitulo(termos []string, titulo string) bool {
	disponiveis := palavras(titulo)
	for _, t := range termos {
		grupo := palavras(t)
		if len(grupo) > 0 && !slices.ContainsFunc(grupo, func(p string) bool { return !slices.Contains(disponiveis, p) }) {
			return true
		}
	}
	return false
}

func eCargoIA(cargo []string) bool {
	nome := " " + strings.Join(cargo, " ") + " "
	for _, m := range familias[0].marcadores {
		if strings.Contains(nome, " "+m+" ") {
			return true
		}
	}
	return false
}

func cargoNasAtribuicoes(termos []string, titulo, descricao string) bool {
	if combinaTitulo(termos, titulo) {
		return true
	}
	tituloPartes := palavras(titulo)
	partes := append(slices.Clone(tituloPartes), palavras(descricao)...)
	for _, termo := range termos {
		alvo := palavras(termo)
		if len(alvo) < 2 {
			continue
		}
		completo, funcao := true, false
		for _, p := range alvo {
			switch p {
			case "junior", "senior", "pleno", "sr", "jr", "lead", "staff", "principal", "pessoa":
				continue
			}
			if !slices.Contains(partes, p) {
				completo = false
				break
			}
			if slices.Contains(tituloPartes, p) && linguagemDoTitulo([]string{p}) == "" {
				funcao = true
			}
		}
		if completo && funcao {
			return true
		}
	}
	return false
}

func especialidadeNoAnuncio(perfil dominio.Objeto, titulo, descricao string) bool {
	cargo := palavras(dominio.Texto(perfil["cargo"]))
	var evidencias []string
	if linguagem := linguagemDoTitulo(cargo); linguagem != "" {
		evidencias = []string{strings.ToLower(linguagem)}
	}
	if eCargoIA(cargo) {
		evidencias = []string{"ai", "ia", "machine learning", "inteligencia artificial", "artificial intelligence", "deep learning", "llm", "llms"}
	}
	if len(evidencias) == 0 {
		return true
	}
	anuncio := " " + strings.Join(palavras(titulo+" "+descricao), " ") + " "
	return slices.ContainsFunc(evidencias, func(e string) bool { return strings.Contains(anuncio, " "+e+" ") })
}
