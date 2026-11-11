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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Applyer-Desktop/1.0")
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	const max = 8 << 20
	dados, err := io.ReadAll(io.LimitReader(resp.Body, max+1))
	if err != nil {
		return err
	}
	if len(dados) > max {
		return errors.New("resposta grande demais")
	}
	return json.Unmarshal(dados, resultado)
}

func linkDaFonte(raw, host string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.User == nil && u.Port() == "" &&
		(u.Hostname() == host || strings.HasSuffix(u.Hostname(), "."+host))
}

func gupy(ctx context.Context, c *http.Client, termo string) ([]dominio.Objeto, error) {
	var pagina struct {
		Data []struct {
			ID                                                                                         json.Number
			Name, CareerPageName, Description, WorkplaceType, City, State, JobURL, ApplicationDeadline string
		}
	}
	err := receber(ctx, c, "https://portal.gupy.io/api/job-search/jobs", url.Values{"jobName": {termo}, "limit": {"30"}, "offset": {"0"}}, &pagina)
	if err != nil {
		return nil, err
	}
	if pagina.Data == nil {
		return nil, errors.New("formato inesperado")
	}
	var vagas []dominio.Objeto
	for _, v := range pagina.Data {
		titulo, empresa := texto(v.Name, 300), texto(v.CareerPageName, 180)
		if v.ID.String() == "" || !linkDaFonte(v.JobURL, "gupy.io") || titulo == "" || empresa == "" {
			continue
		}
		modelo := map[string]string{"remote": "Remoto", "hybrid": "Híbrido", "on-site": "Presencial", "on_site": "Presencial", "onsite": "Presencial"}[strings.ToLower(v.WorkplaceType)]
		vaga := dominio.Objeto{"id": "gupy-" + v.ID.String(), "titulo": titulo, "empresa": empresa, "link": v.JobURL, "plataforma": "gupy", "fonteBusca": "Gupy", "fonteURL": v.JobURL, "mercado": "nacional", "modelo": modelo, "local": strings.Trim(strings.Join([]string{v.City, v.State}, ", "), ", "), "descricao": texto(v.Description, 6000)}
		if fim, err := time.Parse(time.RFC3339, v.ApplicationDeadline); err == nil {
			if !fim.After(time.Now()) {
				continue
			}
			vaga["expiraEm"] = fim.UTC().Format(time.RFC3339)
		}
		vagas = append(vagas, vaga)
	}
	return vagas, nil
}

func himalayas(ctx context.Context, c *http.Client, termo string) ([]dominio.Objeto, error) {
	var pagina struct {
		Jobs []struct {
			Title, CompanyName, Description, ApplicationLink, GUID, Currency, SalaryPeriod, EmploymentType string
			LocationRestrictions, Seniority, Categories                                                    []string
			MinSalary, MaxSalary                                                                           float64
			ExpiryDate                                                                                     int64
		}
	}
	err := receber(ctx, c, "https://himalayas.app/jobs/api/search", url.Values{"q": {termo}, "page": {"1"}}, &pagina)
	if err != nil {
		return nil, err
	}
	if pagina.Jobs == nil {
		return nil, errors.New("formato inesperado")
	}
	var vagas []dominio.Objeto
	for _, v := range pagina.Jobs {
		titulo, empresa := texto(v.Title, 300), texto(v.CompanyName, 180)
		if !linkDaFonte(v.ApplicationLink, "himalayas.app") || v.GUID == "" || titulo == "" || empresa == "" || (v.ExpiryDate > 0 && v.ExpiryDate <= time.Now().Unix()) {
			continue
		}
		id := sha256.Sum256([]byte(v.GUID))
		vaga := dominio.Objeto{"id": fmt.Sprintf("himalayas-%x", id[:12]), "titulo": titulo, "empresa": empresa, "link": v.ApplicationLink, "plataforma": "externo", "fonteBusca": "Himalayas", "fonteURL": "https://himalayas.app", "mercado": "internacional", "modelo": "Remoto", "local": strings.Join(v.LocationRestrictions, ", "), "contrato": v.EmploymentType, "descricao": texto(v.Description, 6000)}
		if linkDaFonte(v.GUID, "himalayas.app") {
			vaga["fonteURL"] = v.GUID
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
		if v.Currency != "" {
			vaga["moeda"] = v.Currency
		}
		if v.SalaryPeriod != "" {
			vaga["periodoSalario"] = v.SalaryPeriod
		}
		if v.MinSalary > 0 {
			vaga["salarioMin"] = v.MinSalary
		}
		if v.MaxSalary > 0 {
			vaga["salarioMax"] = v.MaxSalary
		}
		// Uma taxa horária não implica uma carga horária mensal.
		if strings.EqualFold(v.Currency, "USD") {
			divisor := map[string]float64{"monthly": 1, "annual": 12, "yearly": 12}[strings.ToLower(v.SalaryPeriod)]
			if divisor > 0 && v.MinSalary > 0 {
				vaga["salarioUSDMin"] = v.MinSalary / divisor
			}
			if divisor > 0 && v.MaxSalary > 0 {
				vaga["salarioUSDMax"] = v.MaxSalary / divisor
			}
		}
		vagas = append(vagas, vaga)
	}
	return vagas, nil
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
			if err != nil {
				return vagas, err
			}
			vagas = append(vagas, novas...)
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
	termos := []string{}
	adicionar := func(v any) {
		raw, _ := v.(string)
		s := texto(raw, 120)
		if s == "" || len(termos) >= 2 {
			return
		}
		for _, t := range termos {
			if strings.EqualFold(t, s) {
				return
			}
		}
		termos = append(termos, s)
	}
	adicionar(perfil["cargo"])
	for _, t := range listaTextos(perfil["stackPrincipal"]) {
		adicionar(t)
	}
	if len(termos) == 0 {
		return Coleta{Avisos: []string{"Informe um cargo ou uma habilidade principal para buscar."}}
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
			fontes = append(fontes, fonte{"Himalayas", porTermo(himalayas)})
		}
		if outros {
			locais := append([]string{dominio.Texto(perfil["cargo"])}, listaTextos(perfil["stackPrincipal"])...)
			fontes = append(fontes, fonte{"Arbeitnow", func(ctx context.Context, c *http.Client, _ []string) ([]dominio.Objeto, error) {
				return arbeitnow(ctx, c, locais, remoto)
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
	vistas := map[string]bool{}
	const limiteResultados = 60
	for i := 0; i < maior && len(saida.Vagas) < limiteResultados; i++ {
		for _, l := range lotes {
			if i >= len(l.vagas) {
				continue
			}
			v := l.vagas[i]
			id := dominio.Texto(v["id"])
			link := dominio.Texto(v["link"])
			chave := dominio.ChaveVaga(v["empresa"], v["titulo"])
			if !vistas[id] && !vistas[link] && !vistas[chave] {
				vistas[id] = true
				vistas[link] = true
				vistas[chave] = true
				saida.Vagas = append(saida.Vagas, v)
			}
			if len(saida.Vagas) == limiteResultados {
				break
			}
		}
	}
	return saida
}
