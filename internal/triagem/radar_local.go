package triagem

import (
	"regexp"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

var ligacoesDePais = strings.NewReplacer(" and ", " & ", " e ", " & ")

func nomeDeRegiao(exibicao string) string {
	return ligacoesDePais.Replace(dominio.Normalizar(exibicao))
}

var regioesRadar = func() map[string]language.Region {
	nomes := map[string]language.Region{}
	for _, r := range language.Supported.Regions() {
		if !r.IsCountry() && !r.IsGroup() {
			continue
		}
		for _, nome := range []string{r.String(), r.ISO3(), display.English.Regions().Name(r), display.Portuguese.Regions().Name(r)} {
			if nome != "" {
				nomes[nomeDeRegiao(nome)] = r.Canonicalize()
			}
		}
	}
	for nome, codigo := range map[string]string{"eua": "US", "u.s.": "US", "u.s.a.": "US", "uk": "GB", "latam": "419", "america latina": "419", "latin america": "419"} {
		nomes[nome] = language.MustParseRegion(codigo)
	}
	return nomes
}()

var listaDeRegioes = regexp.MustCompile(`\s*(?:[,;/|]|\s+(?:and|or|e|ou)\s+)\s*`)
var excecaoLocal = regexp.MustCompile(`\b(?:except(?: for)?|excluding|exceto|excluindo)\s+`)
var rotulosDeLocal = regexp.MustCompile(`\b(?:remote|remot[oa]|only|apenas|somente|based in|residents of|residentes (?:em|no|na))\b`)
var localMundial = regexp.MustCompile(`^(?:worldwide|world wide|mundial|global|anywhere(?: in the world)?|qualquer (?:lugar do mundo|pais)|todo o mundo)$`)

// Apenas declarações sobre o posto ou a residência exigida. A localização da
// empresa, de colegas e de clientes não estabelece onde a pessoa pode trabalhar.
var localNoAnuncio = regexp.MustCompile(`\b(?:(?:candidates?|applicants?|you) must (?:be based|reside|live) in (?:the )?|(?:the job|this role|this position) is (?:fully )?remote (?:from )?|(?:candidatos?|pessoas candidatas?) (?:devem|precisam) (?:morar|residir) (?:em|no|na) |(?:trabalho|vaga|posicao) remot[oa] (?:em todo o |em toda a |em |no |na |a partir d[eo] |para ))([^.!?;]{2,100})`)
var regiaoContratacao = regexp.MustCompile(`\b(?:we (?:can|only) hire (?:candidates|applicants) (?:based|residing) in|contratamos (?:apenas |somente )?(?:pessoas|candidatos) residentes (?:em|no|na))\s+([^.!?;]{2,220})`)
var fimDaLocalizacao = regexp.MustCompile(`\b(?:about the team|about the role|about us|what you|job description|sobre (?:a vaga|a equipe|nos))\b`)
var regiaoNoTitulo = regexp.MustCompile(`(?i)\s+[-–—|]\s*(EU|UE|Europe|Europa|LATAM)\s*$`)

// Uma lista parcialmente desconhecida ainda pode incluir o país da pessoa.
func incluiPaisRadar(locais []string, pais language.Region) (inclui, conclusivo bool) {
	conclusivo = len(locais) != 0
	for _, local := range locais {
		if r, ok := regioesRadar[nomeDeRegiao(local)]; ok {
			if r == pais || r.Contains(pais) {
				return true, true
			}
			continue
		}
		for _, nome := range listaDeRegioes.Split(dominio.Normalizar(local), -1) {
			r, ok := regioesRadar[nomeDeRegiao(strings.TrimPrefix(nome, "the "))]
			if ok && (r == pais || r.Contains(pais)) {
				return true, true
			}
			if !ok {
				conclusivo = false
			}
		}
	}
	return false, conclusivo
}

// Cada declaração se aplica ao mesmo anúncio. Uma restrição expressa prevalece
// sobre um rótulo mundial. A lista de países dentro de cada declaração é alternativa.
func localRemotoRadar(vaga dominio.Objeto, nome string, pais language.Region) (string, string) {
	aceito, _ := vaga["remotoMundial"].(bool)
	incerto := false
	var rotulos []string
	if local := dominio.Normalizar(vaga["local"]); local != "" {
		semRotulos := strings.Trim(rotulosDeLocal.ReplaceAllString(local, ""), " ,()/:-")
		if local != semRotulos || localMundial.MatchString(strings.TrimSpace(excecaoLocal.Split(local, 2)[0])) {
			rotulos = append(rotulos, semRotulos)
		}
	}
	for _, m := range localNoAnuncio.FindAllStringSubmatch(dominio.Normalizar(vaga["descricao"]), -1) {
		rotulos = append(rotulos, strings.TrimSpace(m[1]))
	}
	areasDeclaradas := regiaoContratacao.FindAllStringSubmatch(dominio.Normalizar(vaga["descricao"]), -1)
	for _, m := range areasDeclaradas {
		rotulos = append(rotulos, strings.TrimSpace(fimDaLocalizacao.Split(m[1], 2)[0]))
	}
	if len(areasDeclaradas) == 0 {
		if m := regiaoNoTitulo.FindStringSubmatch(texto(vaga["titulo"])); m != nil {
			area := dominio.Normalizar(m[1])
			if area == "ue" {
				area = "eu"
			}
			rotulos = append(rotulos, area)
		}
	}
	if elegiveis := lista(vaga["paisesElegiveis"]); len(elegiveis) > 0 {
		// Preserve países cujo nome contém vírgulas ou conjunções.
		var areas []string
		for _, local := range elegiveis {
			if regiao, ok := regioesRadar[nomeDeRegiao(local)]; ok {
				local = regiao.String()
			}
			areas = append(areas, local)
		}
		rotulos = append(rotulos, strings.Join(areas, ", "))
	}
	for _, rotulo := range rotulos {
		rotulo = dominio.Normalizar(rotulo)
		if rotulo == "" {
			continue
		}
		partes := excecaoLocal.Split(rotulo, 2)
		if len(partes) > 1 {
			vedado, conhecida := incluiPaisRadar([]string{partes[1]}, pais)
			if vedado {
				return "excluir", "A restrição geográfica do anúncio exclui " + nome + "."
			}
			if !conhecida {
				incerto = true
			}
		}
		area := strings.TrimSpace(partes[0])
		if localMundial.MatchString(area) {
			aceito = true
			continue
		}
		inclui, conclusivo := incluiPaisRadar([]string{area}, pais)
		if conclusivo && !inclui {
			return "excluir", "O local permitido pelo anúncio não inclui " + nome + "."
		}
		if inclui {
			aceito = true
		} else {
			incerto = true
		}
	}
	if aceito && !incerto {
		return "forte", ""
	}
	return "conferir", "O anúncio não confirma se aceita trabalho a partir de " + nome + "."
}

func geografiaRadar(perfil, vaga dominio.Objeto) (string, string) {
	nome := texto(perfil["paisTrabalho"])
	pais, ok := regioesRadar[nomeDeRegiao(nome)]
	if !ok || !pais.IsCountry() {
		return "conferir", "Confirme o país de trabalho do perfil."
	}
	modelo := modelos[dominio.Normalizar(vaga["modelo"])]
	if modelo == "" {
		return "conferir", "O anúncio não esclarece a modalidade de trabalho."
	}
	if modelo == "remoto" {
		return localRemotoRadar(vaga, nome, pais)
	}
	local := dominio.Normalizar(vaga["local"])
	elegiveis := lista(vaga["paisesElegiveis"])

	partes := strings.Split(local, ",")
	cidadeAnuncio := strings.TrimSpace(partes[0])
	if len(elegiveis) == 0 && len(partes) > 1 {
		// Uma UF de duas letras também pode ser um código de país (por exemplo, ES).
		ultimo := strings.TrimSpace(partes[len(partes)-1])
		if len(ultimo) > 2 {
			elegiveis = []string{ultimo}
		}
	}
	aceito, conhecido := incluiPaisRadar(elegiveis, pais)
	if !conhecido {
		return "conferir", "O anúncio não confirma se aceita trabalho a partir de " + nome + "."
	}
	if !aceito {
		return "excluir", "O local permitido pelo anúncio não inclui " + nome + "."
	}
	cidade := dominio.Normalizar(perfil["cidadeTrabalho"])
	if cidade == "" || cidadeAnuncio == "" {
		return "conferir", "Confirme a cidade de trabalho para a vaga " + texto(vaga["modelo"]) + "."
	}
	if regiao, ok := regioesRadar[nomeDeRegiao(cidadeAnuncio)]; ok && (regiao == pais || regiao.Contains(pais)) {
		return "conferir", "O anúncio informa o país, mas falta a cidade de trabalho."
	}
	if cidade != cidadeAnuncio {
		return "excluir", "O local “" + texto(vaga["local"]) + "” é diferente da cidade de trabalho do perfil."
	}
	return "forte", ""
}
