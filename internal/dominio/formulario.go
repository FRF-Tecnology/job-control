package dominio

import (
	"net/url"
	"regexp"
	"strings"
)

// Formulários externos (jc-a2y.7.2, docs/propostas/formularios.md): Google Forms e Typeform viram a plataforma
// "formulario". Lista exata de hosts e caminhos, nunca endsWith; o servidor nunca busca a URL (o forms.gle é
// resolvido só no navegador do envio). A URL guardada é a canônica, sem query nem fragmento: o pré-preenchimento
// (entry.* do Google, campos ocultos do Typeform) iria junto no envio sem estar no pacote, então é tirado, e a
// candidatura guarda só o fato (preenchimentoIgnorado), nunca os valores.

// PlataformaFormulario: a plataforma dos formulários externos.
const PlataformaFormulario = "formulario"

var (
	idFormulario      = `[A-Za-z0-9_-]{1,120}`
	caminhoGoogleForm = regexp.MustCompile(`^/forms/d/(e/)?` + idFormulario + `/viewform$`)
	caminhoFormsGle   = regexp.MustCompile(`^/` + idFormulario + `$`)
	caminhoTypeform   = regexp.MustCompile(`^/to/` + idFormulario + `$`)
	hostContaTypeform = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?\.typeform\.com$`)
)

// FormularioDoEndereco: a URL canônica (https, host em minúsculas, sem query nem fragmento) quando o endereço é de um
// formulário da lista exata, e se ele trazia pré-preenchimento. Fora da lista: ok false.
func FormularioDoEndereco(endereco string) (canonica string, preenchido bool, ok bool) {
	u, host, valido := enderecoAnalisado(endereco)
	if !valido || u.RawPath != "" || strings.Contains(u.Path, "..") {
		return "", false, false
	}
	caminho := u.Path
	switch {
	case host == "docs.google.com" && caminhoGoogleForm.MatchString(caminho):
		for chave := range u.Query() {
			if strings.HasPrefix(chave, "entry.") {
				preenchido = true
			}
		}
	case host == "forms.gle" && caminhoFormsGle.MatchString(caminho):
		preenchido = temCampoDeFora(u)
	case (host == "form.typeform.com" || hostContaTypeform.MatchString(host)) && caminhoTypeform.MatchString(caminho):
		preenchido = temCampoDeFora(u)
	default:
		return "", false, false
	}
	return "https://" + host + caminho, preenchido, true
}

// temCampoDeFora: o link traz valores que iriam para o formulário (query que não é de campanha, ou fragmento, onde o
// Typeform lê os campos ocultos).
func temCampoDeFora(u *url.URL) bool {
	if u.Fragment != "" {
		return true
	}
	for chave := range u.Query() {
		if !strings.HasPrefix(strings.ToLower(chave), "utm_") {
			return true
		}
	}
	return false
}
