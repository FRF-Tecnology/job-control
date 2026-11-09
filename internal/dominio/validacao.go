package dominio

import (
	"fmt"
	"regexp"
	"strings"
)

// Validação de vagas e perfil e detecção de CPF/RG (porte de nucleo/validacao.mjs).
// Documentos nunca são guardados na Central.

func cpfValido(d string) bool {
	if len(d) != 11 {
		return false
	}
	todosIguais := true
	for i := 1; i < 11; i++ {
		if d[i] != d[0] {
			todosIguais = false
		}
	}
	if todosIguais {
		return false
	}
	dv := func(n int) int {
		soma := 0
		for i := 0; i < n; i++ {
			soma += int(d[i]-'0') * (n + 1 - i)
		}
		r := (soma * 10) % 11
		if r == 10 {
			return 0
		}
		return r
	}
	return dv(9) == int(d[9]-'0') && dv(10) == int(d[10]-'0')
}

var (
	reCPFFormatado = regexp.MustCompile(`\b\d{3}\.\d{3}\.\d{3}-\d{2}\b`)
	reCPFouRG      = regexp.MustCompile(`(?i)\b(cpf|rg)\b[^\d\n]{0,15}\d[\d.\-\s]{5,}`)
)

// ContemDocumento procura CPF ou RG num texto. Telefones (DDD + 9 dígitos) e ids de vaga não batem
// porque exigimos o dígito verificador do CPF ou a palavra "RG"/"CPF".
func ContemDocumento(v any) bool {
	s := texto(v)
	if reCPFFormatado.MatchString(s) || reCPFouRG.MatchString(s) {
		return true
	}
	// 11 dígitos soltos (sem dígito antes nem depois): só conta se o dígito verificador de CPF bater.
	inicio := -1
	for i := 0; i <= len(s); i++ {
		digito := i < len(s) && s[i] >= '0' && s[i] <= '9'
		if digito && inicio < 0 {
			inicio = i
		}
		if !digito && inicio >= 0 {
			if i-inicio == 11 && cpfValido(s[inicio:i]) {
				return true
			}
			inicio = -1
		}
	}
	return false
}

// textos junta todos os textos de um valor JSON, em qualquer profundidade.
func textos(v any, acc []string) []string {
	switch x := v.(type) {
	case string:
		acc = append(acc, x)
	case map[string]any:
		for _, e := range x {
			acc = textos(e, acc)
		}
	case []any:
		for _, e := range x {
			acc = textos(e, acc)
		}
	}
	return acc
}

func algumDocumento(v any) bool {
	for _, t := range textos(v, nil) {
		if ContemDocumento(t) {
			return true
		}
	}
	return false
}

var camposTextoVaga = []string{"id", "empresa", "titulo", "link", "plataforma", "nivel", "contrato", "modelo", "ingles", "idioma", "aderencia", "motivoClaude", "notas", "motivo", "pendencia", "local"}

var reHTTP = regexp.MustCompile(`(?i)^https?://`)

// ValidarVaga devolve a lista de problemas da vaga (vazia = válida).
func ValidarVaga(x any) []string {
	v, ok := x.(map[string]any)
	if !ok {
		if _, ehLista := x.([]any); !ehLista {
			return []string{"vaga vazia"}
		}
		v = Objeto{}
	}
	erros := []string{}
	for _, c := range []string{"id", "empresa", "titulo"} {
		if aparar(texto(v[c])) == "" {
			erros = append(erros, "campo obrigatório: "+c)
		}
	}
	if !contem(Etapas, v["etapa"]) {
		erros = append(erros, "etapa desconhecida: "+campo(v, "etapa"))
	}
	if contem(PedeMotivo, v["etapa"]) && aparar(texto(v["motivo"])) == "" {
		erros = append(erros, "informe o motivo")
	}
	if igual(v["etapa"], "bloqueada") && aparar(texto(v["pendencia"])) == "" {
		erros = append(erros, "informe a pendência (o que falta você fazer)")
	}
	for _, c := range camposTextoVaga {
		if x := v[c]; x != nil && !ehTextoOuNumero(x) {
			erros = append(erros, c+" deve ser texto")
		}
	}
	if v["stack"] != nil && !listaDeTextos(v["stack"]) {
		erros = append(erros, "stack deve ser uma lista de textos")
	}
	if v["subEtapa"] != nil && !contem(SubEtapas, v["subEtapa"]) {
		erros = append(erros, "subetapa desconhecida: "+campo(v, "subEtapa"))
	}
	if v["mercado"] != nil && !contem(Mercados, v["mercado"]) {
		erros = append(erros, "mercado deve ser "+strings.Join(Mercados, " ou "))
	}
	if d := obj(v["envio"])["data"]; verdadeiro(d) && !EhDataISO(d) {
		erros = append(erros, "data de envio deve ser AAAA-MM-DD")
	}
	if p := obj(v["followUp"])["proximo"]; verdadeiro(p) && !EhDataISO(p) {
		erros = append(erros, "data de follow-up deve ser AAAA-MM-DD")
	}
	if verdadeiro(v["link"]) && !reHTTP.MatchString(texto(v["link"])) {
		erros = append(erros, "link deve começar com http:// ou https://")
	}
	if algumDocumento(v) {
		erros = append(erros, "parece conter CPF/RG — documentos não são guardados na central")
	}
	return erros
}

// Formato do perfil: a tela de Perfil e a IA dependem destes tipos.
var (
	camposTextoPerfil = []string{"nome", "email", "telefone", "cidade", "linkedin", "github", "cargo", "ingles", "formacao", "inicio", "salarioAtual", "modo"}
	listasPerfil      = []string{"stackPrincipal", "stackSecundaria"}
	listasFiltro      = []string{"niveis", "modelo", "pularSe"}
	numerosPerfil     = []string{"pretensaoCLT", "pretensaoPJ", "pretensaoUSD", "experienciaAnos", "diasFollowUp", "limiteLinkedinDia"}
)

// ValidarPerfil devolve a lista de problemas do perfil (vazia = válido).
func ValidarPerfil(x any) []string {
	p, ok := x.(map[string]any)
	if !ok {
		return []string{"perfil deve ser um objeto"}
	}
	erros := []string{}
	for _, k := range camposTextoPerfil {
		if p[k] != nil && !ehTextoOuNumero(p[k]) {
			erros = append(erros, k+" deve ser texto")
		}
	}
	for _, k := range listasPerfil {
		if p[k] != nil && !listaDeTextos(p[k]) {
			erros = append(erros, k+" deve ser uma lista de textos")
		}
	}
	if p["filtros"] != nil {
		f, ok := p["filtros"].(map[string]any)
		if !ok {
			erros = append(erros, "filtros deve ser um objeto")
		} else {
			for _, k := range listasFiltro {
				if f[k] != nil && !listaDeTextos(f[k]) {
					erros = append(erros, "filtros."+k+" deve ser uma lista de textos")
				}
			}
		}
	}
	if p["respostasPadrao"] != nil && !respostasValidas(p["respostasPadrao"]) {
		erros = append(erros, "respostasPadrao deve ser uma lista de {pergunta, resposta}")
	}
	if p["foco"] != nil && !contem(Focos, p["foco"]) {
		erros = append(erros, "foco deve ser "+strings.Join(Focos, ", "))
	}
	for _, k := range numerosPerfil {
		if p[k] != nil && !igual(p[k], "") && !finito(p[k]) {
			erros = append(erros, k+" deve ser número")
		}
	}
	return erros
}

func respostasValidas(x any) bool {
	l, ok := x.([]any)
	if !ok {
		return false
	}
	for _, e := range l {
		r, ok := e.(map[string]any)
		if !ok {
			return false
		}
		for _, k := range []string{"pergunta", "resposta"} {
			if v, tem := r[k]; tem {
				if _, ehTexto := v.(string); !ehTexto {
					return false
				}
			}
		}
	}
	return true
}

var (
	rePalavrasProibidas = regexp.MustCompile(`(?i)\b(cpf|rg|senha|password|passwd)\b`)
	reNao               = regexp.MustCompile(`(?i)^n[aã]o\b`)
)

// PerfilSensivel: além de CPF/RG nos textos, recusa respostas padrão sobre documentos/senha e campos de senha.
func PerfilSensivel(perfil any) bool {
	var chaves []string
	var varrer func(any)
	varrer = func(o any) {
		switch x := o.(type) {
		case map[string]any:
			for k, v := range x {
				chaves = append(chaves, k)
				varrer(v)
			}
		case []any:
			for i, v := range x {
				chaves = append(chaves, fmt.Sprint(i))
				varrer(v)
			}
		}
	}
	varrer(perfil)
	for _, k := range chaves {
		if rePalavrasProibidas.MatchString(k) {
			return true
		}
	}
	for _, e := range lista(obj(perfil)["respostasPadrao"]) {
		r := obj(e)
		resp := aparar(texto(r["resposta"]))
		if rePalavrasProibidas.MatchString(texto(r["pergunta"])) && resp != "" && !reNao.MatchString(resp) {
			return true
		}
	}
	return algumDocumento(perfil)
}

// ContemDocumentoEm checa CPF/RG nos textos de qualquer objeto livre (plataforma, rodada, evento).
func ContemDocumentoEm(o any) bool { return algumDocumento(o) }
