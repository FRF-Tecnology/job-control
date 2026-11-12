package busca

import (
	"net/url"
	"slices"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// MesmoAnuncio identifica a requisição pelo ID da fonte ou pelo endereço
// individual do anúncio. Duas requisições podem usar empresa e título idênticos.
func MesmoAnuncio(a, b dominio.Objeto) bool {
	uma := chavesAnuncio(a)
	for _, outra := range chavesAnuncio(b) {
		if slices.Contains(uma, outra) {
			return true
		}
	}
	return false
}

func chavesAnuncio(v dominio.Objeto) []string {
	var chaves []string
	id := dominio.Texto(v["anuncioId"])
	if id == "" {
		id = dominio.Texto(v["id"])
	}
	if id != "" {
		chaves = append(chaves, "id:"+id)
	}
	for _, campo := range []string{"link", "fonteURL"} {
		u, err := url.Parse(dominio.Texto(v[campo]))
		if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
			continue
		}
		u.Fragment = ""
		u.Host = strings.ToLower(u.Host)
		q := u.Query()
		for k := range q {
			if strings.HasPrefix(strings.ToLower(k), "utm_") {
				q.Del(k)
			}
		}
		u.RawQuery = q.Encode()
		// A página inicial de um portal nunca identifica uma requisição.
		path := strings.Trim(u.Path, "/")
		if u.RawQuery == "" && (path == "" || path == "jobs" || path == "careers" || path == "vagas") {
			continue
		}
		chaves = append(chaves, "url:"+u.String())
	}
	return chaves
}
