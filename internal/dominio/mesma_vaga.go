package dominio

import (
	"net/url"
	"regexp"
	"strings"
)

// Mesma vaga (M5, jc-a2y.6.2.5): durante o envio, a página tem de estar num host da lista fechada da plataforma e
// na vaga aprovada (o id da vaga no endereço é o mesmo). Compara o host já analisado pela biblioteca de URL, inteiro,
// nunca por prefixo ou trecho. É mais estreito que URLDaPlataforma ("host da plataforma e subdomínios", o contrato):
// só aperta.
//
//   - LinkedIn: www.linkedin.com, linkedin.com ou o subdomínio de país (br.linkedin.com); vaga em /jobs/view/<n>.
//   - Gupy: só o host da empresa aprovado (<empresa>.gupy.io); vaga em /jobs/<n>.
//   - InHire: o host da empresa aprovado (<empresa>.inhire.app), vaga em /vagas/<n>; ou o formulário em
//     form-app.inhire.app, desde que o id aprovado esteja no endereço (segmento do caminho ou valor da query, inteiro).

var (
	hostLinkedIn    = regexp.MustCompile(`^(www\.|[a-z]{2}\.)?linkedin\.com$`)
	hostEmpresaGupy = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?\.gupy\.io$`)
	hostEmpresaIH   = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?\.inhire\.app$`)
	idLinkedIn      = regexp.MustCompile(`^/jobs/view/([0-9]+)(/.*)?$`)
	idGupy          = regexp.MustCompile(`^/jobs/([0-9]+)(/.*)?$`)
	idInHire        = regexp.MustCompile(`^/vagas/([0-9a-zA-Z-]+?)(/.*)?$`)
)

// hostsReservados de Gupy e InHire não são de empresa: login, portal, API e o formulário (tratado à parte).
var hostsReservados = map[string]bool{
	"www.gupy.io": true, "portal.gupy.io": true, "login.gupy.io": true, "api.gupy.io": true, "app.gupy.io": true,
	"www.inhire.app": true, "api.inhire.app": true, "app.inhire.app": true, "form-app.inhire.app": true, "login.inhire.app": true,
}

const hostFormInHire = "form-app.inhire.app"

// enderecoAnalisado: https, sem usuário, host em minúsculas e sem o ponto final.
func enderecoAnalisado(endereco string) (*url.URL, string, bool) {
	u, err := url.Parse(strings.TrimSpace(endereco))
	if err != nil || u.Scheme != "https" || u.User != nil || u.Opaque != "" {
		return nil, "", false
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" || u.Port() != "" {
		return nil, "", false
	}
	return u, host, true
}

// vagaDoEndereco devolve o id da vaga quando o endereço está num host da lista fechada da plataforma.
func vagaDoEndereco(plataforma string, u *url.URL, host string) (id string, ok bool) {
	caminho := u.EscapedPath()
	switch plataforma {
	case "linkedin":
		if g := idLinkedIn.FindStringSubmatch(caminho); hostLinkedIn.MatchString(host) && g != nil {
			return g[1], true
		}
	case "gupy":
		if g := idGupy.FindStringSubmatch(caminho); hostEmpresaGupy.MatchString(host) && !hostsReservados[host] && g != nil {
			return g[1], true
		}
	case "inhire":
		if g := idInHire.FindStringSubmatch(caminho); hostEmpresaIH.MatchString(host) && !hostsReservados[host] && g != nil {
			return g[1], true
		}
	}
	return "", false
}

// MesmaVaga diz se o endereço atual da página é a vaga aprovada, num host da lista fechada da plataforma.
func MesmaVaga(plataforma, aprovada, atual string) bool {
	ua, hostA, okA := enderecoAnalisado(aprovada)
	ub, hostB, okB := enderecoAnalisado(atual)
	if !okA || !okB {
		return false
	}
	idA, ok := vagaDoEndereco(plataforma, ua, hostA)
	if !ok {
		return false
	}
	if plataforma == "inhire" && hostB == hostFormInHire {
		return idNoEndereco(ub, idA)
	}
	idB, ok := vagaDoEndereco(plataforma, ub, hostB)
	if !ok || idB != idA {
		return false
	}
	// Gupy e InHire: a mesma empresa (o mesmo host). LinkedIn: qualquer host da lista (www, país).
	return plataforma == "linkedin" || hostA == hostB
}

// idNoEndereco: o id aparece inteiro como segmento do caminho ou como valor de um parâmetro da query.
func idNoEndereco(u *url.URL, id string) bool {
	for _, seg := range strings.Split(u.EscapedPath(), "/") {
		if seg == id {
			return true
		}
	}
	for _, valores := range u.Query() {
		for _, v := range valores {
			if v == id {
				return true
			}
		}
	}
	return false
}
