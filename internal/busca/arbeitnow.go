package busca

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

var termosProfissionais = regexp.MustCompile(`[\pL\pN.+#]+`)
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
	var saida []string
	for _, p := range termosProfissionais.FindAllString(dominio.Normalizar(s), -1) {
		if traduzida, ok := sinonimos[p]; ok {
			p = traduzida
		}
		saida = append(saida, strings.Fields(p)...)
	}
	return saida
}

// A API fornece um feed, sem busca por texto. Exigimos um termo completo do
// perfil no título ou nas tags; Java e JavaScript não são o mesmo requisito.
func combinaArbeitnow(termos [][]string, titulo string, tags []string) bool {
	disponiveis := make(map[string]bool)
	for _, p := range palavras(titulo + " " + strings.Join(tags, " ")) {
		disponiveis[p] = true
	}
	for _, grupo := range termos {
		completo := len(grupo) > 0
		for _, p := range grupo {
			if !disponiveis[p] {
				completo = false
				break
			}
		}
		if completo {
			return true
		}
	}
	return false
}

func arbeitnow(ctx context.Context, c *http.Client, termos []string, aceitaRemoto bool) ([]dominio.Objeto, error) {
	var grupos [][]string
	for _, t := range termos {
		if p := palavras(t); len(p) > 0 {
			grupos = append(grupos, p)
		}
	}
	if len(grupos) == 0 {
		return nil, nil
	}
	var vagas []dominio.Objeto
	const endpoint = "https://www.arbeitnow.com/api/job-board-api"
	const limite = 30
	for n := 1; n <= 2; n++ {
		var pagina struct {
			Data []struct {
				Slug, Title, Description, URL, Location string
				CompanyName                             string `json:"company_name"`
				Tags                                    []string
				JobTypes                                []string `json:"job_types"`
				Remote                                  bool
			}
			Links struct{ Next string }
		}
		if err := receber(ctx, c, endpoint, url.Values{"page": {strconv.Itoa(n)}}, &pagina); err != nil {
			return vagas, err
		}
		if pagina.Data == nil {
			return vagas, errors.New("formato inesperado")
		}
		for _, v := range pagina.Data {
			titulo, empresa := texto(v.Title, 300), texto(v.CompanyName, 180)
			if v.Slug == "" || titulo == "" || empresa == "" || !linkDaFonte(v.URL, "arbeitnow.com") || v.Remote && !aceitaRemoto {
				continue
			}
			if !combinaArbeitnow(grupos, titulo, v.Tags) {
				continue
			}
			id := sha256.Sum256([]byte(v.Slug))
			vaga := dominio.Objeto{"id": fmt.Sprintf("arbeitnow-%x", id[:12]), "titulo": titulo, "empresa": empresa, "link": v.URL, "plataforma": "externo", "fonteBusca": "Arbeitnow", "fonteURL": v.URL, "mercado": "internacional", "local": texto(v.Location, 250), "descricao": texto(v.Description, 6000), "categorias": v.Tags, "contrato": strings.Join(v.JobTypes, ", ")}
			if v.Remote {
				vaga["modelo"] = "Remoto"
			} else {
				for _, t := range append(append([]string{}, v.Tags...), v.JobTypes...) {
					switch dominio.Normalizar(t) {
					case "hybrid", "hybrid work", "hybrides arbeiten":
						vaga["modelo"] = "Híbrido"
					case "onsite", "on-site", "on site", "vor ort":
						vaga["modelo"] = "Presencial"
					}
				}
			}
			vagas = append(vagas, vaga)
			if len(vagas) == limite {
				return vagas, nil
			}
		}
		if pagina.Links.Next == "" || len(pagina.Data) == 0 {
			break
		}
	}
	return vagas, nil
}
