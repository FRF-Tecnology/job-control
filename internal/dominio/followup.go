package dominio

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Follow-ups (porte de nucleo/followup.mjs).

// PendentesFollowUp: só vagas ainda "enviada" (sem retorno) com a data vencida, da mais antiga para a mais nova.
func PendentesFollowUp(vagas []Objeto, hoje string) []Objeto {
	saida := []Objeto{}
	for _, v := range vagas {
		p := obj(v["followUp"])["proximo"]
		if igual(v["etapa"], "enviada") && verdadeiro(p) && texto(p) <= hoje {
			saida = append(saida, v)
		}
	}
	sort.SliceStable(saida, func(i, j int) bool {
		return texto(obj(saida[i]["followUp"])["proximo"]) < texto(obj(saida[j]["followUp"])["proximo"])
	})
	return saida
}

func juntarItens(itens []string, e string) string {
	switch len(itens) {
	case 0:
		return ""
	case 1:
		return itens[0]
	}
	return strings.Join(itens[:len(itens)-1], ", ") + " " + e + " " + itens[len(itens)-1]
}

// A apresentação vem só do perfil da pessoa. Sem perfil, a frase sai — nunca um texto pronto com experiência de outra pessoa.
func apresentacao(perfil Objeto, ingles bool) string {
	var stack []string
	for _, s := range lista(perfil["stackPrincipal"]) {
		if verdadeiro(s) && len(stack) < 3 {
			stack = append(stack, jsString(s))
		}
	}
	if ingles {
		if len(stack) > 0 {
			return "I work with " + juntarItens(stack, "and") + " and would be glad to talk about how I can help the team. "
		}
		return ""
	}
	if cargo := aparar(texto(perfil["cargo"])); cargo != "" {
		r, n := utf8.DecodeRuneInString(cargo)
		return "Atuo como " + string(unicode.ToLower(r)) + cargo[n:] + " e ficaria feliz em conversar sobre como posso contribuir com o time. "
	}
	if len(stack) > 0 {
		return "Trabalho com " + juntarItens(stack, "e") + " e ficaria feliz em conversar sobre como posso contribuir com o time. "
	}
	return ""
}

// SugerirMensagem monta o rascunho de follow-up (em inglês quando a vaga é em inglês).
func SugerirMensagem(vaga, perfil Objeto) string {
	nome := texto(ouV(perfil["nome"], ""))
	if igual(vaga["idioma"], "en") {
		fim := ""
		if nome != "" {
			fim = "\n\nThanks,\n" + nome
		}
		return "Hi! I applied for the " + campo(vaga, "titulo") + " position at " + campo(vaga, "empresa") + " and wanted to reinforce my interest. " +
			apresentacao(perfil, true) + "Is there any update on the process?" + fim
	}
	fim := ""
	if nome != "" {
		fim = "\n\nUm abraço,\n" + nome
	}
	return "Olá! Tudo bem? Me candidatei para a vaga de " + campo(vaga, "titulo") + " na " + campo(vaga, "empresa") + " e queria reforçar meu interesse. " +
		apresentacao(perfil, false) + "Há alguma novidade sobre o processo?" + fim
}
