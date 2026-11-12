// Package busca consulta anúncios públicos. As fontes recebem termos profissionais;
// currículo, contato, pretensão e regras pessoais ficam fora das requisições.
package busca

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

type Coleta struct {
	Vagas  []dominio.Objeto
	Avisos []string
}

var cliente = &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
var tag = regexp.MustCompile(`<[^>]*>`)

type respostaHTTP int

func (codigo respostaHTTP) Error() string { return fmt.Sprintf("HTTP %d", codigo) }

func texto(s string, limite int) string {
	s = strings.Join(strings.Fields(html.UnescapeString(tag.ReplaceAllString(s, " "))), " ")
	r := []rune(s)
	if len(r) > limite {
		return string(r[:limite])
	}
	return s
}

func receber(ctx context.Context, c *http.Client, endpoint string, parametros url.Values, resultado any) error {
	if len(parametros) > 0 {
		endpoint += "?" + parametros.Encode()
	}
	dados, err := baixar(ctx, c, endpoint, "application/json")
	if err != nil {
		return err
	}
	return json.Unmarshal(dados, resultado)
}

func baixar(ctx context.Context, c *http.Client, endpoint, aceita string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", aceita)
	req.Header.Set("User-Agent", "Applyer-Desktop/1.0")
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, respostaHTTP(resp.StatusCode)
	}
	const max = 8 << 20
	dados, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return nil, err
	}
	if len(dados) > max {
		return nil, errors.New("resposta grande demais")
	}
	return dados, nil
}

func linkDaFonte(raw, host string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.User == nil && u.Port() == "" &&
		(u.Hostname() == host || strings.HasSuffix(u.Hostname(), "."+host))
}

const paginasPorConsulta = 3

func gupy(ctx context.Context, c *http.Client, termo string) ([]dominio.Objeto, error) {
	var vagas []dominio.Objeto
	const tamanho = 10
	for n := 0; n < paginasPorConsulta; n++ {
		var pagina struct {
			Data []struct {
				ID                                                                                                                 json.Number
				Name, CareerPageName, Description, WorkplaceType, City, State, Country, JobURL, ApplicationDeadline, PublishedDate string
			}
		}
		err := receber(ctx, c, "https://portal.gupy.io/api/job-search/jobs", url.Values{"jobName": {termo}, "limit": {strconv.Itoa(tamanho)}, "offset": {strconv.Itoa(n * tamanho)}}, &pagina)
		if err != nil {
			return vagas, err
		}
		if pagina.Data == nil {
			return vagas, errors.New("formato inesperado")
		}
		for _, v := range pagina.Data {
			titulo, empresa := texto(v.Name, 300), texto(v.CareerPageName, 180)
			if v.ID.String() == "" || !linkDaFonte(v.JobURL, "gupy.io") || titulo == "" || empresa == "" {
				continue
			}
			modelo := map[string]string{"remote": "Remoto", "hybrid": "Híbrido", "on-site": "Presencial", "on_site": "Presencial", "onsite": "Presencial"}[strings.ToLower(v.WorkplaceType)]
			local := ""
			if modelo != "Remoto" {
				local = strings.Join(slices.DeleteFunc([]string{v.City, v.State, v.Country}, func(s string) bool { return s == "" }), ", ")
			}
			vaga := dominio.Objeto{"id": "gupy-" + v.ID.String(), "titulo": titulo, "empresa": empresa, "link": v.JobURL, "plataforma": "gupy", "fonteBusca": "Gupy", "fonteURL": v.JobURL, "mercado": "nacional", "modelo": modelo, "local": local, "descricao": texto(v.Description, 6000)}
			if modelo != "Remoto" && v.Country != "" {
				vaga["paisesElegiveis"] = []string{v.Country}
			}
			if pub, err := time.Parse(time.RFC3339, v.PublishedDate); err == nil {
				vaga["publicadaEm"] = pub.UTC().Format(time.RFC3339)
			}
			if fim, err := time.Parse(time.RFC3339, v.ApplicationDeadline); err == nil {
				if !fim.After(time.Now()) {
					continue
				}
				vaga["expiraEm"] = fim.UTC().Format(time.RFC3339)
			}
			vagas = append(vagas, vaga)
		}
		if len(pagina.Data) < tamanho {
			break
		}
	}
	return vagas, nil
}

func himalayas(ctx context.Context, c *http.Client, termo, pais string) ([]dominio.Objeto, error) {
	var vagas []dominio.Objeto
	for n := 1; n <= paginasPorConsulta; n++ {
		var pagina struct {
			Jobs []struct {
				Title, CompanyName, Description, ApplicationLink, GUID, Currency, SalaryPeriod, EmploymentType string
				LocationRestrictions, Seniority, Categories                                                    []string
				MinSalary, MaxSalary                                                                           float64
				ExpiryDate                                                                                     int64
			}
		}
		params := url.Values{"q": {termo}, "page": {strconv.Itoa(n)}, "sort": {"recent"}}
		if pais != "" {
			params.Set("country", pais)
		}
		err := receber(ctx, c, "https://himalayas.app/jobs/api/search", params, &pagina)
		if err != nil {
			return vagas, err
		}
		if pagina.Jobs == nil {
			return vagas, errors.New("formato inesperado")
		}
		for _, v := range pagina.Jobs {
			titulo, empresa := texto(v.Title, 300), texto(v.CompanyName, 180)
			if (!linkDaFonte(v.ApplicationLink, "himalayas.app") && !linkDaFonte(v.ApplicationLink, "gupy.io")) || v.GUID == "" || titulo == "" || empresa == "" || (v.ExpiryDate > 0 && v.ExpiryDate <= time.Now().Unix()) {
				continue
			}
			id := sha256.Sum256([]byte(v.GUID))
			vaga := dominio.Objeto{"id": fmt.Sprintf("himalayas-%x", id[:12]), "titulo": titulo, "empresa": empresa, "link": v.ApplicationLink, "plataforma": "externo", "fonteBusca": "Himalayas", "fonteURL": "https://himalayas.app", "mercado": "internacional", "modelo": "Remoto", "local": strings.Join(v.LocationRestrictions, ", "), "contrato": v.EmploymentType, "descricao": texto(v.Description, 6000)}
			if linkDaFonte(v.GUID, "himalayas.app") {
				vaga["fonteURL"] = v.GUID
			}
			if len(v.LocationRestrictions) > 0 {
				vaga["paisesElegiveis"] = v.LocationRestrictions
			}
			if v.ExpiryDate > 0 {
				vaga["expiraEm"] = time.Unix(v.ExpiryDate, 0).UTC().Format(time.RFC3339)
			}
			if len(v.Seniority) > 0 {
				vaga["nivel"] = strings.Join(v.Seniority, " / ")
			}
			if len(v.Categories) > 0 {
				vaga["categorias"] = v.Categories
			}
			salarioNativo(vaga, v.Currency, v.SalaryPeriod, v.MinSalary, v.MaxSalary)
			vagas = append(vagas, vaga)
		}
		// A API de pesquisa retorna 20 por página e não aceita um limite customizado.
		if len(pagina.Jobs) < 20 {
			break
		}
	}
	return vagas, nil
}

func salarioNativo(vaga dominio.Objeto, moeda, periodo string, minimo, maximo float64) {
	if moeda != "" {
		vaga["moeda"] = moeda
	}
	if periodo != "" {
		vaga["periodoSalario"] = periodo
	}
	if minimo > 0 {
		vaga["salarioMin"] = minimo
	}
	if maximo > 0 {
		vaga["salarioMax"] = maximo
	}
	// Uma taxa horária não implica uma carga horária mensal.
	if strings.EqualFold(moeda, "USD") {
		divisor := map[string]float64{"monthly": 1, "annual": 12, "yearly": 12}[strings.ToLower(periodo)]
		if divisor > 0 && minimo > 0 {
			vaga["salarioUSDMin"] = minimo / divisor
		}
		if divisor > 0 && maximo > 0 {
			vaga["salarioUSDMax"] = maximo / divisor
		}
	}
}

type consultaFonte func(context.Context, *http.Client, []string) ([]dominio.Objeto, error)
type fonte struct {
	nome      string
	consultar consultaFonte
}

func porTermo(buscar func(context.Context, *http.Client, string) ([]dominio.Objeto, error)) consultaFonte {
	return func(ctx context.Context, c *http.Client, termos []string) ([]dominio.Objeto, error) {
		var vagas []dominio.Objeto
		for _, t := range termos {
			novas, err := buscar(ctx, c, t)
			vagas = append(vagas, novas...)
			if err != nil {
				return vagas, err
			}
		}
		return vagas, nil
	}
}

func listaTextos(v any) []string {
	if l, ok := v.([]string); ok {
		return l
	}
	var res []string
	for _, v := range dominio.ListaDe(v) {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			res = append(res, s)
		}
	}
	return res
}

// Consultar intercala os mercados para que uma fonte volumosa não ocupe toda a rodada.
func Consultar(ctx context.Context, perfil dominio.Objeto) Coleta {
	return consultar(ctx, cliente, perfil)
}

func consultar(ctx context.Context, c *http.Client, perfil dominio.Objeto) Coleta {
	termos := consultasDoPerfil(perfil)
	if len(termos) == 0 {
		return Coleta{Avisos: []string{"Informe o cargo que você procura para buscar vagas."}}
	}
	var fontes []fonte
	if perfil["foco"] != "internacional" {
		fontes = append(fontes, fonte{"Gupy", porTermo(gupy)})
	}
	if perfil["foco"] != "nacional" {
		modelos := listaTextos(dominio.ObjetoDe(perfil["filtros"])["modelo"])
		remoto := len(modelos) == 0 || slices.ContainsFunc(modelos, func(m string) bool { return dominio.Normalizar(m) == "remoto" })
		outros := len(modelos) == 0 || slices.ContainsFunc(modelos, func(m string) bool { return dominio.Normalizar(m) != "remoto" })
		if remoto {
			pais := texto(dominio.Texto(perfil["paisTrabalho"]), 100)
			if pais == "" {
				pais = texto(dominio.Texto(perfil["pais"]), 100)
			}
			pais = codigoPais(pais)
			fontes = append(fontes, fonte{"Himalayas", porTermo(func(ctx context.Context, c *http.Client, t string) ([]dominio.Objeto, error) {
				return himalayas(ctx, c, t, pais)
			})}, fonte{"Jobicy", func(ctx context.Context, c *http.Client, termos []string) ([]dominio.Objeto, error) {
				return jobicy(ctx, c, termos, pais)
			}}, fonte{"We Work Remotely", weWorkRemotely})
		}
		if outros {
			fontes = append(fontes, fonte{"Arbeitnow", func(ctx context.Context, c *http.Client, termos []string) ([]dominio.Objeto, error) {
				return arbeitnow(ctx, c, termos, remoto)
			}})
		}
	}
	type lote struct {
		indice int
		vagas  []dominio.Objeto
		aviso  string
	}
	canal := make(chan lote, len(fontes))
	for i, f := range fontes {
		go func() {
			l := lote{indice: i}
			v, err := f.consultar(ctx, c, termos)
			l.vagas = v
			if err != nil {
				l.aviso = f.nome + " indisponível. Tente novamente mais tarde."
				if len(v) > 0 {
					l.aviso = "A coleta de " + f.nome + " foi parcial. Tente novamente mais tarde."
				}
				var codigo respostaHTTP
				if errors.As(err, &codigo) {
					if codigo == 429 {
						l.aviso = f.nome + " limitou as consultas (HTTP 429). Tente novamente mais tarde."
					} else {
						l.aviso += " (" + codigo.Error() + ")"
					}
				}
			}
			canal <- l
		}()
	}
	lotes := make([]lote, len(fontes))
	var saida Coleta
	for range fontes {
		l := <-canal
		lotes[l.indice] = l
	}
	maior := 0
	for _, l := range lotes {
		if l.aviso != "" {
			saida.Avisos = append(saida.Avisos, l.aviso)
		}
		if len(l.vagas) > maior {
			maior = len(l.vagas)
		}
	}
	vistas := map[string]int{}
	agora := time.Now()
	const limiteResultados = 60
	for i := 0; i < maior; i++ {
		for _, l := range lotes {
			if i >= len(l.vagas) {
				continue
			}
			v := l.vagas[i]
			if pub, err := time.Parse(time.RFC3339, dominio.Texto(v["publicadaEm"])); err == nil && (pub.Before(agora.AddDate(0, 0, -30)) || pub.After(agora)) {
				continue
			}
			titulo, descricao := dominio.Texto(v["titulo"]), dominio.Texto(v["descricao"])
			if !cargoNasAtribuicoes(termos, titulo, descricao) || !especialidadeNoAnuncio(perfil, titulo, descricao) {
				continue
			}
			chaves := chavesAnuncio(v)
			pos := len(saida.Vagas)
			for _, k := range chaves {
				if ix, ok := vistas[k]; ok {
					pos = ix
					break
				}
			}
			if pos == len(saida.Vagas) {
				saida.Vagas = append(saida.Vagas, v)
			} else if v["fonteBusca"] == "Gupy" && saida.Vagas[pos]["fonteBusca"] != "Gupy" {
				saida.Vagas[pos] = v
			}
			for _, k := range chaves {
				vistas[k] = pos
			}
		}
	}
	// Quem já tem publicação comprovada pode chegar à triagem forte. Os
	// anúncios sem data continuam disponíveis para conferência.
	slices.SortStableFunc(saida.Vagas, func(a, b dominio.Objeto) int {
		return strings.Compare(dominio.Texto(b["publicadaEm"]), dominio.Texto(a["publicadaEm"]))
	})
	if len(saida.Vagas) > limiteResultados {
		saida.Vagas = saida.Vagas[:limiteResultados]
	}
	return saida
}
