package busca

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

func arbeitnow(ctx context.Context, c *http.Client, termos []string, aceitaRemoto bool) ([]dominio.Objeto, error) {
	var vagas []dominio.Objeto
	const endpoint = "https://www.arbeitnow.com/api/job-board-api"
	for n := 1; n <= paginasPorConsulta; n++ {
		var pagina struct {
			Data []struct {
				Slug, Title, Description, URL, Location string
				CompanyName                             string `json:"company_name"`
				Tags                                    []string
				JobTypes                                []string `json:"job_types"`
				Remote                                  bool
				CreatedAt                               int64 `json:"created_at"`
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
			descricao := texto(v.Description, 6000)
			if v.Slug == "" || titulo == "" || empresa == "" || !linkDaFonte(v.URL, "arbeitnow.com") || v.Remote && !aceitaRemoto || !cargoNasAtribuicoes(termos, titulo, descricao) {
				continue
			}
			id := sha256.Sum256([]byte(v.Slug))
			vaga := dominio.Objeto{"id": fmt.Sprintf("arbeitnow-%x", id[:12]), "titulo": titulo, "empresa": empresa, "link": v.URL, "plataforma": "externo", "fonteBusca": "Arbeitnow", "fonteURL": v.URL, "mercado": "internacional", "descricao": descricao, "categorias": v.Tags, "contrato": strings.Join(v.JobTypes, ", ")}
			if v.CreatedAt > 0 {
				vaga["publicadaEm"] = time.Unix(v.CreatedAt, 0).UTC().Format(time.RFC3339)
			}
			if v.Remote {
				vaga["modelo"] = "Remoto"
			} else {
				vaga["local"] = texto(v.Location, 250)
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
		}
		if pagina.Links.Next == "" || len(pagina.Data) == 0 {
			break
		}
	}
	return vagas, nil
}
