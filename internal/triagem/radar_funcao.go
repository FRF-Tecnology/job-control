package triagem

import (
	"regexp"
	"slices"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Os títulos orientam a leitura das atribuições. Empresa e setor não participam desta comparação.
var termosDeCargo = strings.NewReplacer(
	"machine learning", "ml", "aprendizado de maquina", "ml", "aprendizagem de maquina", "ml",
	"artificial intelligence", "ml", "inteligencia artificial", "ml",
	"back-end", "backend", "back end", "backend", "front-end", "frontend", "front end", "frontend",
)
var traducoesCargo = map[string]string{
	"engenharia": "engineer", "engenheira": "engineer", "engenheiro": "engineer", "engineering": "engineer",
	"desenvolvedor": "engineer", "desenvolvedora": "engineer", "programador": "engineer", "programadora": "engineer", "developer": "engineer",
	"analista": "analyst", "analise": "analyst", "cientista": "scientist", "dados": "data", "ia": "ml", "ai": "ml",
	"gerente": "manager", "gestor": "manager", "gestora": "manager", "produto": "product", "projetos": "project",
	"vendas": "sales", "comercial": "sales", "atendimento": "support", "suporte": "support", "cliente": "customer",
	"enfermeira": "nurse", "enfermeiro": "nurse", "contabilidade": "accounting", "contador": "accountant", "contadora": "accountant",
}
var omitidosCargo = strings.Fields("de do da dos das em and of for para com senioridade senior sr pleno mid level junior jr staff lead principal pessoa especialista remote remoto remota")

func termosCargo(s string) []string {
	var termos []string
	for _, p := range palavrasRegra.FindAllString(termosDeCargo.Replace(dominio.Normalizar(s)), -1) {
		if slices.Contains(omitidosCargo, p) {
			continue
		}
		if v, ok := traducoesCargo[p]; ok {
			p = v
		}
		termos = append(termos, p)
	}
	return termos
}

var familiasCargo = []struct {
	nome   string
	sinais []string
}{
	{"support", []string{"support", "success", "atendente", "helpdesk"}}, // "Support Engineer" atende clientes.
	{"manager", []string{"manager", "director", "diretor", "diretora"}},
	{"designer", []string{"designer"}},
	{"nurse", []string{"nurse"}},
	{"scientist", []string{"scientist"}},
	{"analyst", []string{"analyst"}},
	{"engineer", []string{"engineer", "backend", "frontend", "devops"}},
}

func familiaCargo(s []string) string {
	for _, f := range familiasCargo {
		if compativel(s, f.sinais) {
			return f.nome
		}
	}
	return ""
}

var especialidades = strings.Fields("ml backend frontend devops security civil electrical mechanical marketing sales product accounting")
var atributosEngenharia = regexp.MustCompile(`\b(?:build(?:ing)?|develop(?:ing|ment)?|design(?:ing)?|architect(?:ing|ure)?|implement(?:ing|ation)?|maintain(?:ing)?|maintenance|deliver(?:ing)?|deploy(?:ing|ment)?|train(?:ing)?|test(?:ing)?|optimi[sz](?:e|ing|ation)|desenvolv\w*|desenhar|implement\w*|constru\w*|manter|mantendo|manutencao|criacao|criar|modelagem|trein\w*|implantar|projetar|integrar|integracao)\b`)
var outrasAtividades = regexp.MustCompile(`\b(?:analy[sz]\w*|analis\w*|otimiz\w*|gerenci\w*|manage\w*|planej\w*|plan\w*|pesquis\w*|research\w*|monitor\w*|coorden\w*|coordinat\w*|produc\w*|produz\w*|atend\w*|resolv\w*|assist\w*|support\w*|negoci\w*|auditar|audit\w*|avali\w*|assess\w*|elabor\w*)\b`)

// Atribuições em infinitivo, em uma seção de responsabilidades ou que explicitem
// o uso de um instrumento não precisam estar em um vocabulário de profissões.
var descreveAtribuicao = regexp.MustCompile(`^(?:[-• ]*\pL{3,}(?:ar|er|ir)\s+)|\b(?:you will|you'll|your responsibilities|responsibilities|duties|your role|voce vai|voce ira|suas atribuicoes|responsabilidades|atribuicoes|using|utilizando)\b`)
var contextoInstitucional = regexp.MustCompile(`\b(?:our (?:customers|clients|company|product) (?:builds?|develops?|uses?|is|are)|noss[oa]s? (?:clientes|empresa|produto) (?:usa\w*|desenvolve\w*|e|sao)|we were founded|fomos fundados)\b`)
var setorNoFim = regexp.MustCompile(`\b(?:para (?:a |uma |nossa )?empresa (?:de|do setor)|for (?:a |our )?(?:company|client) in)\b.*$`)
var atendeClientes = regexp.MustCompile(`\b(?:customer (?:tickets|complaints|inquiries)|(?:atender|atendimento (?:ao|a)) clientes?|responder chamados|provide customer support)\b`)
var requisitoDeclarado = regexp.MustCompile(`\b(?:requirements?|qualifications?|must have|required skills|requisitos?|habilidades obrigatorias)\s*:\s*`)
var linguagensCargo = strings.Fields("java javascript python kotlin typescript golang php ruby c++ c#")
var inicioDasAtividades = regexp.MustCompile(`(?i)\b(?:what you(?:'ll| will) do|your responsibilities|key responsibilities|suas (?:responsabilidades|atribui[çc][õo]es)|responsabilidades e atribui[çc][õo]es)\b\s*[:–-]?\s*`)
var contextoSecundario = regexp.MustCompile(`(?i)\b(?:(?:(?:while|by)\s+)?(?:collaborat(?:e|ing)|partner(?:ing)?|liais(?:e|ing)|consult(?:ing)?)\s+(?:closely\s+)?(?:with|across)|(?:enquanto\s+)?colabor\pL*\s+com|em (?:parceria|colabora[çc][ãa]o|contato) com|(?:nice to have|differential|diferenciais|ser[aá] um diferencial))\b`)
var culturaInstitucional = regexp.MustCompile(`\b(?:welcoming workplace|(?:diverse|inclusive|welcoming)(?: engineering)? culture|nossa cultura|cultura (?:inclusiva|da empresa)|our (?:culture|values|benefits)|employee benefits)\b`)
var atividadePrincipal = regexp.MustCompile(`(?i)\b(?:primarily|predominantly|mainly|principalmente|predominantemente)\s+(?:in|with|using|em|n[oa]|com)\s+([^,.;:!?]{1,60})`)
var modificadorDeAtividade = regexp.MustCompile(`(?i)\s+(?:while|enquanto|when|quando|but|mas|as needed)\b`)

// A fonte pode achatar uma lista HTML em uma frase enorme. Só compare e cite
// competências perto da atividade, sem cruzar colaboração, benefícios ou seções.
func trechosDoTrabalho(descricao string) []string {
	const maxPalavras, maxCaracteres = 22, 160
	var saida []string
	for _, frase := range frasesDeclarativas(inicioDasAtividades.ReplaceAllString(descricao, "\n")) {
		if i := contextoSecundario.FindStringIndex(frase); i != nil {
			frase = frase[:i[0]]
		}
		frase = strings.TrimSpace(frase)
		palavras := strings.Fields(frase)
		if len(palavras) <= maxPalavras && len([]rune(frase)) <= maxCaracteres {
			saida = append(saida, frase)
			continue
		}
		for i := range palavras {
			n := dominio.Normalizar(palavras[i])
			if !atributosEngenharia.MatchString(n) && !outrasAtividades.MatchString(n) && !(i == 0 && descreveAtribuicao.MatchString(n+" ")) {
				continue
			}
			fim := min(i+maxPalavras, len(palavras))
			for fim > i && len([]rune(strings.Join(palavras[i:fim], " "))) > maxCaracteres {
				fim--
			}
			if fim > i {
				saida = append(saida, strings.Join(palavras[i:fim], " "))
			}
		}
	}
	return saida
}

func contemTermoRadar(s, termo string) bool {
	alvo := palavrasRegra.FindAllString(dominio.Normalizar(termo), -1)
	palavras := palavrasRegra.FindAllString(dominio.Normalizar(s), -1)
	for i := 0; len(alvo) > 0 && i+len(alvo) <= len(palavras); i++ {
		if slices.Equal(palavras[i:i+len(alvo)], alvo) {
			return true
		}
	}
	return false
}

func funcaoRadar(perfil, vaga dominio.Objeto) (string, string) {
	cargo, titulo := texto(perfil["cargo"]), texto(vaga["titulo"])
	stack, descricao := lista(perfil["stackPrincipal"]), texto(vaga["descricao"])
	requisitos := strings.Join(lista(vaga["stack"]), ", ")
	for _, frase := range trechosDoTrabalho(descricao) {
		n := dominio.Normalizar(frase)
		if i := requisitoDeclarado.FindStringIndex(n); i != nil && !incerteza.MatchString(n) {
			requisitos += ", " + n[i[1]:]
		}
	}
	var obrigatorias []string
	for _, habilidade := range stack {
		if contemTermoRadar(cargo, habilidade) {
			obrigatorias = append(obrigatorias, habilidade)
		}
	}
	classe, motivo := compararFuncao(cargo, titulo, descricao, requisitos, stack, obrigatorias)
	if classe == "forte" {
		return classe, motivo
	}
	for _, equivalente := range lista(perfil["titulosEquivalentes"]) {
		if strings.TrimSpace(equivalente) == "" {
			continue
		}
		c, m := compararFuncao(equivalente, titulo, descricao, requisitos, stack, obrigatorias)
		if c == "forte" {
			return c, m
		}
		if c == "conferir" && classe == "excluir" {
			classe, motivo = c, m
		}
	}
	return classe, motivo
}

func compararFuncao(cargo, titulo, descricao, requisitos string, stack, tecnologiasDoCargo []string) (string, string) {
	alvo, oferta := termosCargo(cargo), termosCargo(titulo)
	incompativel := "A função “" + titulo + "” difere do alvo “" + cargo + "”."
	if len(alvo) == 0 || len(oferta) == 0 {
		return "conferir", "Confirme o cargo desejado e o título do anúncio."
	}
	familia, outra := familiaCargo(alvo), familiaCargo(oferta)
	if familia != "" && outra != "" && familia != outra {
		return "excluir", incompativel
	}
	if !compativel(alvo, oferta) {
		return "conferir", "Falta comprovar se “" + titulo + "” exerce a função de “" + cargo + "”."
	}

	// "Engineer" ou "Analyst" sozinho não torna duas especialidades equivalentes.
	var foco []string
	for _, t := range alvo {
		if t != familia {
			foco = append(foco, t)
		}
	}
	areaDistinta := compativel(oferta, especialidades) && !compativel(foco, oferta)
	if areaDistinta && (compativel(foco, especialidades) || len(tecnologiasDoCargo) > 0 && compativel(oferta, []string{"ml", "frontend", "civil", "mechanical", "electrical"})) {
		return "excluir", incompativel
	}
	if compativel(foco, linguagensCargo) && compativel(oferta, linguagensCargo) && !compativel(foco, oferta) {
		return "excluir", incompativel
	}
	// Se o anúncio nomeia expressamente o núcleo da atividade, uma referência
	// em outro trecho não comprova a competência que dá nome ao cargo-alvo.
	if len(tecnologiasDoCargo) > 0 {
		centrais := atividadePrincipal.FindAllStringSubmatch(descricao, -1)
		var destaques []string
		for _, m := range centrais {
			destaques = append(destaques, modificadorDeAtividade.Split(m[1], 2)[0])
		}
		centro := strings.Join(destaques, ", ")
		if centro != "" && !slices.ContainsFunc(tecnologiasDoCargo, func(h string) bool { return contemTermoRadar(centro, h) }) {
			return "conferir", "O anúncio indica atuação principal em “" + strings.TrimSpace(destaques[0]) + "”. Confirme se a função envolve " + strings.Join(tecnologiasDoCargo, ", ") + "."
		}
	}
	for _, frase := range trechosDoTrabalho(descricao) {
		n := dominio.Normalizar(frase)
		if contextoInstitucional.MatchString(n) || culturaInstitucional.MatchString(n) || incerteza.MatchString(n) || outra != "support" && atendeClientes.MatchString(n) {
			continue
		}
		if !atributosEngenharia.MatchString(n) && !outrasAtividades.MatchString(n) && !descreveAtribuicao.MatchString(n) {
			continue
		}
		evidencias := []string{}
		for _, habilidade := range stack {
			if contemTermoRadar(frase, habilidade) || contemTermoRadar(requisitos, habilidade) {
				evidencias = append(evidencias, habilidade)
			}
		}
		if len(stack) > 0 && len(evidencias) == 0 {
			continue
		}
		if len(tecnologiasDoCargo) > 0 && !compativel(evidencias, tecnologiasDoCargo) {
			continue
		}
		atributos := termosCargo(setorNoFim.ReplaceAllString(n, "") + " " + requisitos)
		// Compartilhar "técnico", "auxiliar" ou uma ferramenta não comprova
		// a especialidade restante (por exemplo, logística versus laboratório).
		if slices.ContainsFunc(foco, func(termo string) bool { return !slices.Contains(oferta, termo) && !slices.Contains(atributos, termo) }) {
			continue
		}
		citacao := strings.TrimRight(strings.TrimSpace(frase), ".")
		motivo := "Para “" + titulo + "”, o anúncio descreve “" + citacao + "”."
		if len(evidencias) > 0 {
			motivo += " O anúncio e o perfil incluem " + strings.Join(evidencias, ", ") + "."
		}
		return "forte", motivo
	}
	pendencia := "Confira as atribuições de “" + titulo + "” para o alvo “" + cargo + "”"
	if len(stack) > 0 {
		pendencia += " e o uso de " + strings.Join(stack, ", ")
	}
	return "conferir", pendencia + "."
}
