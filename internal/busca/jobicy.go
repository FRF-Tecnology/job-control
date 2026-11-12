package busca

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

const endpointJobicy = "https://jobicy.com/api/jobs"

func jobicy(ctx context.Context, c *http.Client, termos []string, pais string) ([]dominio.Objeto, error) {
	geo := ""
	if pais != "" {
		var resposta struct {
			Locations []struct{ GeoName, GeoSlug string }
		}
		if err := receber(ctx, c, endpointJobicy, url.Values{"get": {"locations"}}, &resposta); err != nil {
			return nil, err
		}
		for _, local := range resposta.Locations {
			if codigoPais(local.GeoName) == pais {
				geo = local.GeoSlug
				break
			}
		}
	}
	buscar := porTermo(func(ctx context.Context, c *http.Client, t string) ([]dominio.Objeto, error) {
		return jobicyPorTermo(ctx, c, t, geo)
	})
	// A API rejeita tag fora de 3–20 caracteres. Com um cargo longo sem
	// variante curta, pesquise o feed do país e confronte o título localmente.
	var curtos []string
	for _, t := range termos {
		if n := utf8.RuneCountInString(t); n >= 3 && n <= 20 {
			curtos = append(curtos, t)
		}
	}
	if len(curtos) == 0 {
		curtos = []string{""}
	}
	return buscar(ctx, c, curtos)
}

func jobicyPorTermo(ctx context.Context, c *http.Client, termo, geo string) ([]dominio.Objeto, error) {
	var vagas []dominio.Objeto
	params := url.Values{"count": {"10"}}
	if termo != "" {
		params.Set("tag", termo)
	}
	if geo != "" {
		params.Set("geo", geo)
	}
	for n := 0; n < paginasPorConsulta; n++ {
		var pagina struct {
			NextCursor string
			Jobs       []struct {
				ID                                                                                                  int64
				URL, JobTitle, CompanyName, JobGeo, JobLevel, JobDescription, PubDate, SalaryCurrency, SalaryPeriod string
				JobIndustry, JobType                                                                                []string
				SalaryMin, SalaryMax                                                                                float64
			}
		}
		if err := receber(ctx, c, endpointJobicy, params, &pagina); err != nil {
			return vagas, err
		}
		if pagina.Jobs == nil {
			return vagas, errors.New("formato inesperado")
		}
		for _, j := range pagina.Jobs {
			titulo, empresa := texto(j.JobTitle, 300), texto(j.CompanyName, 180)
			if j.ID <= 0 || titulo == "" || empresa == "" || !linkDaFonte(j.URL, "jobicy.com") {
				continue
			}
			v := dominio.Objeto{"id": "jobicy-" + strconv.FormatInt(j.ID, 10), "titulo": titulo, "empresa": empresa, "fonteBusca": "Jobicy", "link": j.URL, "fonteURL": j.URL, "plataforma": "externo", "mercado": "internacional", "modelo": "Remoto", "local": texto(j.JobGeo, 500), "descricao": texto(j.JobDescription, 6000), "contrato": strings.Join(j.JobType, ", "), "nivel": texto(j.JobLevel, 100), "categorias": j.JobIndustry}
			if pub, err := time.Parse(time.RFC3339, j.PubDate); err == nil {
				v["publicadaEm"] = pub.UTC().Format(time.RFC3339)
			}
			if strings.EqualFold(j.JobGeo, "Anywhere") || strings.EqualFold(j.JobGeo, "Worldwide") {
				v["remotoMundial"] = true
			} else if j.JobGeo != "" {
				var paises []string
				for _, s := range strings.Split(j.JobGeo, ",") {
					if nome := strings.TrimSpace(s); nome != "" {
						paises = append(paises, nome)
					}
				}
				v["paisesElegiveis"] = paises
			}
			salarioNativo(v, j.SalaryCurrency, j.SalaryPeriod, j.SalaryMin, j.SalaryMax)
			vagas = append(vagas, v)
		}
		if pagina.NextCursor == "" || params.Get("cursor") == pagina.NextCursor || len(pagina.Jobs) == 0 {
			break
		}
		params.Set("cursor", pagina.NextCursor)
	}
	return vagas, nil
}
