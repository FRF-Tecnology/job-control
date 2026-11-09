package dominio

import (
	"net/url"
	"regexp"
	"strings"
)

// Categorias do endereço onde a página do envio parou, quando ele não é a vaga aprovada (P0 da .6.5). Vão para o
// log no lugar do endereço (que pode trazer dado), para diagnosticar sem expor nada.
const (
	EnderecoVazio     = "about:blank" // a navegação não aconteceu (página em branco ou sem http)
	EnderecoLogin     = "login"       // a plataforma pediu login, authwall ou verificação
	EnderecoOutroHost = "outro-host"  // fora da plataforma (ATS externo, outro site)
	EnderecoOutraVaga = "outra-vaga"  // na plataforma, mas não na vaga aprovada
)

// caminhoLoginLinkedIn: telas de entrada e verificação do LinkedIn (sessão caída, authwall, checkpoint, 2FA).
var caminhoLoginLinkedIn = regexp.MustCompile(`^/(login|authwall|checkpoint|uas/|signup|m/login|ul/)`)

// EnderecoFinal devolve "" quando o endereço atual é a vaga aprovada, na plataforma; senão, a categoria (lista
// fechada acima). Login vem antes de outra-vaga: a tela de login do LinkedIn é no mesmo host.
func EnderecoFinal(plataforma, aprovada, atual string) string {
	u, err := url.Parse(strings.TrimSpace(atual))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return EnderecoVazio
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	switch plataforma {
	case "linkedin":
		if hostLinkedIn.MatchString(host) && caminhoLoginLinkedIn.MatchString(u.Path) {
			return EnderecoLogin
		}
	case "gupy":
		if host == "login.gupy.io" {
			return EnderecoLogin
		}
	case PlataformaFormulario: // jc-a2y.7.2: o Google pede login na conta (nunca enviamos a identidade dela)
		if host == "accounts.google.com" {
			return EnderecoLogin
		}
	}
	if !URLDaPlataforma(plataforma, atual) {
		return EnderecoOutroHost
	}
	if !MesmaVaga(plataforma, aprovada, atual) {
		return EnderecoOutraVaga
	}
	return ""
}
