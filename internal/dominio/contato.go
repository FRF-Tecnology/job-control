package dominio

import (
	"net/url"
	"regexp"
	"strings"
)

// Dados de contato da pessoa: os mesmos em todos os perfis de busca (vão no pacote de cada candidatura e no
// "Preencher no site da empresa"). Trocar de perfil de busca nunca troca o contato.
// nome é o nome completo (a tela junta primeiroNome e sobrenome ao salvar); cidade é só a cidade, e estado é a sigla
// (no Brasil) ou o nome do estado/província (fora dele); paisResidencia é o código de duas letras.
var CamposDeContato = []string{"nome", "primeiroNome", "sobrenome", "email", "ddi", "telefone", "cidade", "estado", "paisResidencia", "linkedin", "github"}

var reDDI = regexp.MustCompile(`^\+[1-9][0-9]{0,3}$`)

// DDIValido: o código de discagem do país, como "+55" ou "+351".
func DDIValido(s string) bool { return reDDI.MatchString(s) }

// TelefoneCompleto junta o código do país ao número ("+55 34 99999-0000"). O perfil guarda os dois separados
// (ddi e telefone); perfis antigos têm só o telefone, às vezes já com o "+".
func TelefoneCompleto(perfil Objeto) string {
	tel, ddi := strings.TrimSpace(texto(perfil["telefone"])), strings.TrimSpace(texto(perfil["ddi"]))
	if tel == "" || strings.HasPrefix(tel, "+") || !DDIValido(ddi) {
		return tel
	}
	return ddi + " " + tel
}

// LocalDoPerfil junta cidade e estado para um campo só ("Recife, PE"). Perfis antigos guardavam tudo em cidade.
func LocalDoPerfil(perfil Objeto) string {
	cidade, estado := strings.TrimSpace(texto(perfil["cidade"])), strings.TrimSpace(texto(perfil["estado"]))
	switch {
	case cidade == "":
		return estado
	case estado == "" || strings.HasSuffix(Normalizar(cidade), Normalizar(estado)):
		return cidade
	}
	return cidade + ", " + estado
}

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
