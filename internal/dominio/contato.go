package dominio

import (
	"net/url"
	"strings"
)

// Dados de contato da pessoa: os mesmos em todos os perfis de busca (vão no pacote de cada candidatura e no
// "Preencher no site da empresa"). Trocar de perfil de busca nunca troca o contato.
var CamposDeContato = []string{"nome", "email", "telefone", "cidade", "linkedin", "github"}

// LinkDePerfil devolve o endereço do perfil público da pessoa (LinkedIn, GitHub) em https, ou "" se não for um
// endereço desse site. Aceita sem o "https://" ("linkedin.com/in/fulana"). O host tem de ser o site ou um subdomínio
// dele: "linkedin.com.golpe.example" e "golpe.example/linkedin.com" não passam.
func LinkDePerfil(s, site string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, " \t\n<>\"'") {
		return ""
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.User != nil || u.Port() != "" {
		return ""
	}
	if u.Scheme == "http" {
		u.Scheme = "https"
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if u.Scheme != "https" || (host != site && !strings.HasSuffix(host, "."+site)) || strings.Trim(u.Path, "/") == "" {
		return ""
	}
	u.Host = host
	return u.String()
}
