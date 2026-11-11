package triagem

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

var pisoUSD = regexp.MustCompile(`^(?:(?:salario|pagamento|oferta|remuneracao)\s+)?(?:abaixo de|menor que|menos de|inferior a)\s+(?:us\$|usd)\s*([0-9]+(?:\.[0-9]{3})*(?:,[0-9]{1,2})?)\s*(?:/\s*mes|por mes|mensais|mensal)$`)

func pisoDeExclusao(regras []string, pretensao float64) float64 {
	var maior float64
	for _, regra := range regras {
		s := dominio.Normalizar(inicioDaRegra.ReplaceAllString(strings.ToLower(strings.TrimSpace(regra)), ""))
		switch s {
		case "salario abaixo da pretensao", "salario abaixo da minha pretensao", "salario abaixo da pretensao em usd", "abaixo da minha pretensao em usd":
			maior = max(maior, pretensao)
		default:
			if m := pisoUSD.FindStringSubmatch(s); m != nil {
				n, err := strconv.ParseFloat(strings.ReplaceAll(strings.ReplaceAll(m[1], ".", ""), ",", "."), 64)
				if err == nil {
					maior = max(maior, n)
				}
			}
		}
	}
	return maior
}

var (
	inicioDaRegra = regexp.MustCompile(`^(?:(?:eu )?não (?:quero|aceito)|pular(?: quando| se)?|evitar|excluir)\s+(?:vagas? (?:com|de)\s+)?`)
	palavrasRegra = regexp.MustCompile(`[\pL\pN+#]+(?:\.[\pL\pN]+)*|\.[\pL\pN]+`)
	separaFrases  = regexp.MustCompile(`[;!?\n]+|\.\s+`)
	incerteza     = regexp.MustCompile(`\b(nao|sem|dispensa(?:do|da)?|opciona(?:l|is)|facultativ[oa]s?|eventua(?:l|is)|possivel|pode(?:m)?|desejavel|diferencial|preferencial|negociavel|caso|somente se|apenas se|not|never|without|optional|preferred|may|might|kein|keine)\b|\bno (?:on[- ]call|mandatory|required)\b`)
	condicao      = regexp.MustCompile(`\b(obrigatori[oa]s?|exigid[oa]s?|mandatori[oa]s?|mandatory|required)\b`)
	afirmaAntes   = regexp.MustCompile(`\b(?:exige(?:m)?|requer(?:em)?|inclui|incluem|exigimos|obrigatori[oa]|necessari[oa]|requisito|requires?|mandatory|must include|somos uma?)\s*(?::\s*)?(?:a |o |de )?$`)
)

func frasesDeclarativas(s string) []string {
	var res []string
	inicio := 0
	for _, i := range separaFrases.FindAllStringIndex(s, -1) {
		if !strings.Contains(s[i[0]:i[1]], "?") {
			res = append(res, s[inicio:i[0]])
		}
		inicio = i[1]
	}
	return append(res, s[inicio:])
}

// Retorna uma evidência literal curta. Título, descrição e campos estruturados
// são as únicas fontes. Uma mera menção na descrição não prova uma exigência.
func exclusaoPersonalizada(regras []string, vaga dominio.Objeto) string {
	for _, original := range regras {
		regra := strings.TrimSpace(inicioDaRegra.ReplaceAllString(strings.ToLower(strings.TrimSpace(original)), ""))
		if regra == "" || len([]rune(regra)) > 100 || dominio.ContemDocumento(regra) || incerteza.MatchString(dominio.Normalizar(regra)) {
			continue
		}
		// Os campos de nível e modelo já trataram as alternativas, inclusive vagas
		// que aceitam mais de um nível. Não transforme menções secundárias em veto.
		if _, ok := opcoes([]string{regra}, niveis); ok {
			continue
		}
		if _, ok := opcoes([]string{regra}, modelos); ok {
			continue
		}
		alvo := palavrasRegra.FindAllString(dominio.Normalizar(regra), -1)
		if len(alvo) == 0 {
			continue
		}
		for _, campo := range []string{"empresa", "contrato", "stack", "categorias"} {
			itens := lista(vaga[campo])
			if s := texto(vaga[campo]); s != "" {
				itens = []string{s}
			}
			for _, item := range itens {
				if slices.Equal(alvo, palavrasRegra.FindAllString(dominio.Normalizar(item), -1)) {
					return "A regra de exclusão coincide com o campo " + campo + " do anúncio: “" + item + "”."
				}
			}
		}
		descricao := texto(vaga["descricao"])
		for _, fonte := range []struct {
			conteudo, nome string
			titulo         bool
		}{{texto(vaga["titulo"]), "título", true}, {descricao, "anúncio", false}} {
			for _, frase := range frasesDeclarativas(fonte.conteudo) {
				normal := dominio.Normalizar(frase)
				if incerteza.MatchString(normal) {
					continue
				}
				pos := palavrasRegra.FindAllStringIndex(frase, -1)
				tokens := make([]string, len(pos))
				for i, p := range pos {
					tokens[i] = dominio.Normalizar(frase[p[0]:p[1]])
				}
				for i := 0; i+len(alvo) <= len(tokens); i++ {
					if !slices.Equal(tokens[i:i+len(alvo)], alvo) {
						continue
					}
					inicio, fim := pos[i][0], pos[i+len(alvo)-1][1]
					citacao := frase[inicio:fim]
					depois := strings.TrimLeft(dominio.Normalizar(frase[fim:]), " :–-")
					depois = strings.TrimPrefix(strings.TrimPrefix(depois, "e "), "sao ")
					requisitoDepois := condicao.FindStringIndex(depois)
					categoriaExplicita := fonte.titulo || len(tokens) == len(alvo)
					obrigatoria := condicao.MatchString(dominio.Normalizar(citacao)) || afirmaAntes.MatchString(dominio.Normalizar(frase[:inicio])+" ") || requisitoDepois != nil && requisitoDepois[0] == 0
					if categoriaExplicita || obrigatoria {
						return "Pela regra “" + original + "”, o " + fonte.nome + " informa “" + citacao + "”."
					}
				}
			}
		}
	}
	return ""
}
