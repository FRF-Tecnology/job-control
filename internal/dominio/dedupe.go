package dominio

import (
	"regexp"
	"strings"
)

// Detecção de vagas repetidas por empresa + título (porte de nucleo/dedupe.mjs).

// Palavras que variam entre plataformas e não distinguem uma vaga da outra.
var ruido = map[string]bool{"pessoa": true, "desenvolvedora": true, "desenvolvedor": true, "desenvolvedor a": true, "dev": true, "a": true, "de": true, "da": true, "do": true, "remoto": true, "remota": true, "vaga": true}

var sufixosEmpresa = map[string]bool{"group": true, "grupo": true, "global": true, "software": true, "tecnologia": true, "technology": true, "tech": true, "ltda": true, "sa": true, "s": true, "brasil": true, "brazil": true, "inc": true, "consultoria": true, "solucoes": true, "digital": true}

var (
	reA         = regexp.MustCompile(`\(a\)`)
	reBackEnd   = regexp.MustCompile(`back[\s-]+end`)
	reFrontEnd  = regexp.MustCompile(`front[\s-]+end`)
	reFullStack = regexp.MustCompile(`full[\s-]+stack`)
	reNaoChave  = regexp.MustCompile(`[^a-z0-9+#]+`)
)

func limparChave(v any) string {
	s := Normalizar(v)
	s = reA.ReplaceAllString(s, " ")
	s = reBackEnd.ReplaceAllString(s, "backend")
	s = reFrontEnd.ReplaceAllString(s, "frontend")
	s = reFullStack.ReplaceAllString(s, "fullstack")
	return aparar(reNaoChave.ReplaceAllString(s, " "))
}

// ChaveEmpresa tira sufixos como "Group", "Tecnologia" e "Ltda" (menos a primeira palavra).
func ChaveEmpresa(empresa any) string {
	var nucleo []string
	for i, p := range strings.Fields(limparChave(empresa)) {
		if i == 0 || !sufixosEmpresa[p] {
			nucleo = append(nucleo, p)
		}
	}
	return strings.Join(nucleo, "")
}

// ChaveVaga é empresa|título sem palavras de ruído.
func ChaveVaga(empresa, titulo any) string {
	var palavras []string
	for _, p := range strings.Fields(limparChave(titulo)) {
		if !ruido[p] {
			palavras = append(palavras, p)
		}
	}
	return ChaveEmpresa(empresa) + "|" + strings.Join(palavras, " ")
}

// DuplicatasPorTitulo devolve as vagas com a mesma empresa+título (empresa desconhecida nunca casa).
// excluirID (opcional) deixa a própria vaga de fora.
func DuplicatasPorTitulo(vagas []Objeto, nova Objeto, excluirID any) []Objeto {
	saida := []Objeto{}
	if ChaveEmpresa(nova["empresa"]) == "" {
		return saida
	}
	chave := ChaveVaga(nova["empresa"], nova["titulo"])
	for _, v := range vagas {
		if !igual(v["id"], excluirID) && ChaveVaga(v["empresa"], v["titulo"]) == chave {
			saida = append(saida, v)
		}
	}
	return saida
}

// AcharDuplicata procura pelo mesmo id e depois pela mesma empresa+título.
func AcharDuplicata(vagas []Objeto, nova Objeto) Objeto {
	for _, v := range vagas {
		if igual(v["id"], nova["id"]) {
			return v
		}
	}
	if d := DuplicatasPorTitulo(vagas, nova, nil); len(d) > 0 {
		return d[0]
	}
	return nil
}
