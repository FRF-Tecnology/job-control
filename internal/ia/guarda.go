package ia

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Regras como código: toda saída de IA passa por aqui antes de voltar ao app, venha de qual provedor vier.
// O prompt pede; a guarda garante. (Porte de nucleo/ia/guarda.mjs.)

type Objeto = dominio.Objeto

var (
	reSensivel     = regexp.MustCompile(`(?i)\b(cpf|rg|senha|password|passcode|pin)\b|dados banc|conta banc|ag[eê]ncia banc|bank (account|details)|routing number|\biban\b|\bpix\b|cri(e|ar) (uma |sua )?conta|create (an |your )?account|sign ?up|cadastre-se|\blog ?in\b|fazer login`)
	reExperiencia  = regexp.MustCompile(`(?i)experi[eê]ncia|experience|\banos\b|\byears?\b|profici|familiar|conhecimento|knowledge|dom[ií]nio|skill|habilidade|n[ií]vel (de|em)|level (of|in)|trabalhou com|worked with|j[aá] (usou|utilizou|trabalhou)|have you (used|worked)|how (long|many)`)
	reIdade        = regexp.MustCompile(`(?i)idade|\bage\b|old are you|nasceu|born`)
	reAcentoPT     = regexp.MustCompile(`(?i)[ãõçáéíóúâêô]|\b(você|voce|tem|possui|qual|quantos)\b`)
	rePalavraEN    = regexp.MustCompile(`(?i)\b(do|does|have|are|is|can|will|you|your)\b`)
	reConservadora = regexp.MustCompile(`^(nao|no|none|nenhum|nenhuma|sem experiencia|0|zero|never|nunca|less than|menos de)\b`)
	reSoDigitos    = regexp.MustCompile(`^\d+$`)
	reNumeros      = regexp.MustCompile(`\d[\d.,]*`)
	reMilhares     = regexp.MustCompile(`^\d{1,3}([.,]\d{3})+$`)
	reNaoDigito    = regexp.MustCompile(`\D`)
	reCercaInicio  = regexp.MustCompile(`(?i)^` + "```" + `(?:json)?\s*`)
	reCercaFim     = regexp.MustCompile(`\s*` + "```" + `$`)
)

func emIngles(s string) bool { return !reAcentoPT.MatchString(s) && rePalavraEN.MatchString(s) }

func bloquear(regra, msg string) *ErroIA { return &ErroIA{Tipo: "regra", Mensagem: msg, Regra: regra} }

// Resposta conservadora para experiência sem fonte: nunca inventar. nil = nenhuma opção conservadora.
func conservadora(e Objeto) any {
	en := emIngles(dominio.Texto(e["pergunta"]))
	switch e["tipoCampo"] {
	case "numero":
		return "0"
	case "sim-nao":
		if en {
			return "No"
		}
		return "Não"
	case "opcao":
		for _, o := range dominio.ListaDe(e["opcoes"]) {
			if reConservadora.MatchString(dominio.Normalizar(o)) {
				return o
			}
		}
		return nil
	}
	if en {
		return "I don't have experience with this."
	}
	return "Não tenho experiência com isso."
}

// Fonte declarada precisa existir: "curriculo" sem currículo ou "respostasPadrao" sem respostas é o mesmo que "nenhuma".
func fonteReal(s, e Objeto) any {
	if s["fonte"] == "curriculo" && strings.TrimSpace(dominio.Texto(e["curriculo"])) == "" {
		return "nenhuma"
	}
	if s["fonte"] == "respostasPadrao" && len(dominio.ListaDe(dominio.ObjetoDe(e["perfil"])["respostasPadrao"])) == 0 {
		return "nenhuma"
	}
	return dominio.Ou(s["fonte"], "nenhuma")
}

func com(s Objeto, pares ...any) Objeto {
	n := make(Objeto, len(s)+len(pares)/2)
	for k, v := range s {
		n[k] = v
	}
	for i := 0; i+1 < len(pares); i += 2 {
		n[pares[i].(string)] = pares[i+1]
	}
	return n
}

// Regra muda (ou barra) a saída de uma função. Ordem importa: parar vem antes das correções; documento é a última barreira.
type Regra struct {
	ID        string
	Funcoes   []string // vazio = todas
	Descricao string
	aplicar   func(s, e Objeto) (Objeto, *ErroIA)
}

// Regras é a lista, na ordem em que são aplicadas.
var Regras = []Regra{
	{ID: "parar-dado-sensivel", Funcoes: []string{"responderPergunta"},
		Descricao: "Pergunta pede CPF, RG, senha, dados bancários ou criar conta/login: parar, sem resposta.",
		aplicar: func(s, e Objeto) (Objeto, *ErroIA) {
			// Se o modelo já parou sozinho (sem resposta), a saída dele fica como está.
			if !reSensivel.MatchString(dominio.Texto(e["pergunta"])) || (s["parar"] == true && strings.TrimSpace(dominio.Texto(s["resposta"])) == "") {
				return s, nil
			}
			return Objeto{"resposta": "", "fonte": "nenhuma", "confianca": float64(1), "parar": true, "motivoParar": "Pergunta pede dado sensível ou conta/login: fica com a pessoa."}, nil
		}},
	{ID: "nunca-inventar", Funcoes: []string{"responderPergunta"},
		Descricao: "Experiência sem fonte real (currículo, respostasPadrao, perfil) vira a resposta conservadora.",
		aplicar: func(s, e Objeto) (Objeto, *ErroIA) {
			p := dominio.Texto(e["pergunta"])
			if dominio.Verdadeiro(s["parar"]) || fonteReal(s, e) != "nenhuma" || !reExperiencia.MatchString(p) || reIdade.MatchString(p) {
				return s, nil
			}
			r := conservadora(e)
			if r == nil {
				return com(s, "resposta", "", "fonte", "nenhuma", "parar", true, "motivoParar", "Sem experiência e nenhuma opção conservadora na lista."), nil
			}
			return com(s, "resposta", r, "fonte", "nenhuma"), nil
		}},
	{ID: "numero-so-digitos", Funcoes: []string{"responderPergunta"},
		Descricao: `Campo numérico: só dígitos. "R$ 5.000" vira "5000"; texto ou faixa é bloqueado.`,
		aplicar: func(s, e Objeto) (Objeto, *ErroIA) {
			r := dominio.JSString(s["resposta"])
			if dominio.Verdadeiro(s["parar"]) || e["tipoCampo"] != "numero" || reSoDigitos.MatchString(r) {
				return s, nil
			}
			numeros := reNumeros.FindAllString(r, -1)
			if len(numeros) == 1 && reMilhares.MatchString(numeros[0]) {
				return com(s, "resposta", reNaoDigito.ReplaceAllString(numeros[0], "")), nil
			}
			if len(numeros) == 1 && reSoDigitos.MatchString(numeros[0]) {
				return com(s, "resposta", numeros[0]), nil
			}
			return nil, bloquear("numero-so-digitos", "campo numérico com resposta não numérica")
		}},
	{ID: "opcao-da-lista", Funcoes: []string{"responderPergunta"},
		Descricao: "Campo de opção: a resposta tem de ser uma das opções (devolvida com o texto exato da lista).",
		aplicar: func(s, e Objeto) (Objeto, *ErroIA) {
			if dominio.Verdadeiro(s["parar"]) || e["tipoCampo"] != "opcao" {
				return s, nil
			}
			for _, o := range dominio.ListaDe(e["opcoes"]) {
				if dominio.Normalizar(o) == dominio.Normalizar(s["resposta"]) {
					return com(s, "resposta", o), nil
				}
			}
			return nil, bloquear("opcao-da-lista", "a resposta não é uma das opções do campo")
		}},
	{ID: "sim-nao-valido", Funcoes: []string{"responderPergunta"},
		Descricao: "Campo sim/não: só Sim, Não, Yes ou No.",
		aplicar: func(s, e Objeto) (Objeto, *ErroIA) {
			if dominio.Verdadeiro(s["parar"]) || e["tipoCampo"] != "sim-nao" {
				return s, nil
			}
			mapa := map[string]string{"sim": "Sim", "nao": "Não", "yes": "Yes", "no": "No"}
			if v, ok := mapa[dominio.Normalizar(s["resposta"])]; ok {
				return com(s, "resposta", v), nil
			}
			return nil, bloquear("sim-nao-valido", "campo sim/não com resposta que não é sim nem não")
		}},
	{ID: "pulo-com-motivo", Funcoes: []string{"triarVaga"},
		Descricao: `Triagem "pular" sempre tem motivoPulo (padrão "Outro").`,
		aplicar: func(s, e Objeto) (Objeto, *ErroIA) {
			if s["decisao"] == "pular" && !dominio.Verdadeiro(s["motivoPulo"]) {
				return com(s, "motivoPulo", "Outro"), nil
			}
			return s, nil
		}},
	{ID: "revisao-coerente", Funcoes: []string{"revisarAntesDeEnviar"},
		Descricao: "Revisão com problemas nunca sai como ok.",
		aplicar: func(s, e Objeto) (Objeto, *ErroIA) {
			return com(s, "ok", len(dominio.ListaDe(s["problemas"])) == 0), nil
		}},
	{ID: "vaga-conhecida", Funcoes: []string{"classificarEmail"},
		Descricao: "vagaId do email tem de ser uma das vagas enviadas; id inventado vira null.",
		aplicar: func(s, e Objeto) (Objeto, *ErroIA) {
			if s["vagaId"] == nil {
				return s, nil
			}
			for _, v := range dominio.ListaDe(e["vagas"]) {
				if dominio.Igual(dominio.ObjetoDe(v)["id"], s["vagaId"]) {
					return s, nil
				}
			}
			return com(s, "vagaId", nil), nil
		}},
	{ID: "mensagem-rascunho", Funcoes: []string{"escreverMensagem"},
		Descricao: "Mensagem é sempre rascunho: nunca é enviada sem aprovação da pessoa.",
		aplicar:   func(s, e Objeto) (Objeto, *ErroIA) { return com(s, "rascunho", true), nil }},
	{ID: "pedido-catalogo", Funcoes: []string{"interpretarPedido"},
		Descricao: "Pedido (jc-a2y.7.1): vagaId só da lista enviada; url https sem usuário, barra invertida, // a mais nem host fora do ASCII; texto puro, sem HTML, esquema de link perigoso, senha ou dado bancário; nenhum caractere invisível ou de direção.",
		aplicar: func(s, e Objeto) (Objeto, *ErroIA) {
			p := dominio.ObjetoDe(s["parametros"])
			for _, x := range p { // a mensagem é a mesma em qualquer ordem
				if v, ok := x.(string); ok && reInvisivel.MatchString(v) {
					return nil, bloquear("pedido-catalogo", "parâmetro com caractere invisível ou de direção")
				}
			}
			if id, tem := p["vagaId"]; tem && id != nil {
				achou := false
				for _, v := range dominio.ListaDe(e["vagas"]) {
					if outro, ok := dominio.ObjetoDe(v)["id"].(string); ok && outro == id {
						achou = true
					}
				}
				if !achou {
					return nil, bloquear("pedido-catalogo", "vagaId fora da lista de vagas enviada")
				}
			}
			if s["acao"] == "adicionar-vaga" && !URLSeguraPedido(p["url"]) {
				return nil, bloquear("pedido-catalogo", "url fora do formato seguro")
			}
			for _, k := range []string{"empresa", "titulo", "texto"} {
				v, ok := p[k].(string)
				if !ok {
					continue
				}
				if strings.ContainsAny(v, "<>") || reEsquemaPerigoso.MatchString(v) {
					return nil, bloquear("pedido-catalogo", "texto com HTML ou link perigoso")
				}
				if reSegredo.MatchString(v) {
					return nil, bloquear("pedido-catalogo", "texto com senha ou dado bancário")
				}
			}
			return s, nil
		}},
	{ID: "sem-documento", Funcoes: nil,
		Descricao: "Qualquer saída com CPF ou RG é bloqueada.",
		aplicar: func(s, e Objeto) (Objeto, *ErroIA) {
			if dominio.ContemDocumentoEm(s) {
				return nil, bloquear("sem-documento", "a saída da IA contém CPF/RG")
			}
			return s, nil
		}},
}

// lerJSON lê o texto do provedor como JSON (tolera cerca ```json), guardando a ordem das chaves.
func lerJSON(bruto string) (any, *ErroIA) {
	t := reCercaFim.ReplaceAllString(reCercaInicio.ReplaceAllString(strings.TrimSpace(bruto), ""), "")
	v, err := lerOrdenado([]byte(t))
	if err != nil {
		return nil, erroIA("json", "a IA não devolveu JSON válido")
	}
	return v, nil
}

// ValidarEntrada confere a entrada de uma função antes de chamar a IA.
func ValidarEntrada(funcao string, e Objeto) error {
	f := contratos.Funcoes[funcao]
	if f == nil {
		return erroIA("entrada", "função desconhecida: "+funcao)
	}
	var valor any = e
	if e == nil {
		valor = nil
	}
	erros := validarEsquema(valor, f.entrada, "$")
	if funcao == "responderPergunta" && e["tipoCampo"] == "opcao" && len(dominio.ListaDe(e["opcoes"])) == 0 {
		erros = append(erros, `$.opcoes: obrigatório para tipoCampo "opcao"`)
	}
	if len(erros) > 0 {
		return &ErroIA{Tipo: "entrada", Mensagem: fmt.Sprintf("entrada inválida para %s: %s", funcao, strings.Join(erros, "; ")), Erros: erros}
	}
	return nil
}

func extra(funcao string, s Objeto) []string {
	switch funcao {
	case "interpretarPedido":
		return conferirParametros(s)
	case "responderPergunta":
		if c, ok := s["confianca"].(float64); ok && (c < 0 || c > 1) {
			return []string{"confianca deve estar entre 0 e 1"}
		}
	case "avaliarEntrevista":
		if n, ok := s["nota"].(float64); ok && (n < 1 || n > 5) {
			return []string{"nota deve ser de 1 a 5"}
		}
	}
	return nil
}

// Guardar valida o esquema e aplica as regras. Devolve a saída e os ids das regras que mudaram algo.
func Guardar(funcao string, entrada Objeto, bruto string) (Objeto, []string, error) {
	f := contratos.Funcoes[funcao]
	lida, errJ := lerJSON(bruto)
	if errJ != nil {
		return nil, nil, errJ
	}
	erros := validarEsquema(lida, f.saida, "$")
	saida, ehObjeto := simples(lida).(map[string]any)
	if ehObjeto {
		erros = append(erros, extra(funcao, saida)...)
	}
	if len(erros) > 0 {
		return nil, nil, &ErroIA{Tipo: "esquema", Mensagem: fmt.Sprintf("saída fora do contrato de %s: %s", funcao, strings.Join(erros, "; ")), Erros: erros}
	}
	aplicadas := []string{}
	for _, r := range Regras {
		if r.Funcoes != nil && !contem(r.Funcoes, funcao) {
			continue
		}
		nova, err := r.aplicar(saida, entrada)
		if err != nil {
			return nil, nil, err
		}
		if !reflect.DeepEqual(nova, saida) {
			aplicadas = append(aplicadas, r.ID)
		}
		saida = nova
	}
	return saida, aplicadas, nil
}

func contem(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}
