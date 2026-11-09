package executor

import (
	"regexp"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Detector dos motivos de bloqueio (jc-a2y.6.2.4). Trabalha sobre um retrato da página: o texto visível, os campos,
// os quadros (iframes) e os botões. O navegador do PR 5 tira o retrato da página de verdade; os testes usam
// retratos de páginas fictícias (testdata/paginas). Nada do retrato vai para o log nem muda a fila: só escolhe um
// motivo da lista fechada.

// Retrato da página num instante.
type Retrato struct {
	URL     string          `json:"url"`
	Texto   string          `json:"texto"`
	Campos  []CampoDaPagina `json:"campos"`
	Quadros []string        `json:"quadros"` // endereço (src) dos iframes
	Botoes  []string        `json:"botoes"`
}

// CampoDaPagina como o navegador vê: tipo do HTML (text, email, tel, number, password, file, checkbox, radio,
// select, textarea), rótulo, nome, placeholder e autocomplete.
type CampoDaPagina struct {
	ID           string `json:"id"`
	Tipo         string `json:"tipo"`
	Rotulo       string `json:"rotulo"`
	Nome         string `json:"nome"`
	Placeholder  string `json:"placeholder"`
	Autocomplete string `json:"autocomplete"`
	Obrigatorio  bool   `json:"obrigatorio"`
}

func (c CampoDaPagina) texto() string { return c.Rotulo + " " + c.Placeholder + " " + c.Nome }

func re(s string) *regexp.Regexp { return regexp.MustCompile(s) }

// Sinais da página inteira (texto já normalizado: sem acento, minúsculo).
var (
	quadroCaptcha = re(`recaptcha|hcaptcha|challenges\.cloudflare\.com|turnstile|arkoselabs|funcaptcha|geetest`)
	campoCaptcha  = re(`g-recaptcha-response|h-captcha-response|cf-turnstile-response|captcha`)
	textoCaptcha  = re(`nao sou (um )?robo|i'?m not a robot|verify (that )?you are (a )?human|verifique (que )?(voce )?(e )?humano|confirme que (voce )?(e )?humano|resolva o desafio`)

	textoCriarConta = re(`crie (sua|uma) conta|criar (sua |uma )?conta|cadastre-se|faca seu cadastro|create (an |your )?account|sign up|join now|registre-se`)
	textoEntrar     = re(`\b(faca|fazer) (o )?login\b|\b(entre|entrar) (na|com) (sua )?conta\b|\bentrar para continuar\b|\biniciar sessao\b|\bsign in\b|\blog in\b|\bsua sessao (expirou|terminou)\b|\bsession (has )?expired\b|\bpara continuar, entre\b`)
	textoCodigo     = re(`codigo de (verificacao|seguranca|acesso)|verification code|enviamos um codigo|we sent (you )?a code|autenticacao (em|de) (dois|duas) (fatores|etapas)|two-factor|2fa|one-time (code|password)|digite o codigo`)

	textoTeste  = re(`teste (comportamental|de perfil|de personalidade|cognitivo|de logica|psicologico|disc)|avaliacao (comportamental|de perfil|psicologica)|questionario (comportamental|de perfil)|entrevista (por|em) video|grave (um|seu) video|gravar (um )?video|video ?entrevista|entrevista (com|por) (ia|inteligencia artificial)|video interview|record (a|your) video|ai interview|personality (test|assessment)|cognitive (test|assessment)|psychometric|online assessment`)
	quadroTeste = re(`hirevue|pymetrics|harver|vervoe|codility|hackerrank|testgorilla|mindsight|gupy\.io/.*/(teste|test)`)

	botaoExterno = re(`candidatar-se no site|candidate-se no site|continuar no site|aplicar no site|ir para o site|apply on (the )?company (website|site)|apply on company|continue to (the )?(company )?(website|site)|apply externally`)

	rotuloCurriculo = re(`curriculo|\bcv\b|resume|curriculum`)
	rotuloNome      = re(`^(seu )?nome( completo)?$|^full name$|^name$|^first and last name$|^nome e sobrenome$`)
	rotuloEmail     = re(`e-?mail`)
	rotuloTelefone  = re(`telefone|celular|whatsapp|\bphone\b|\bmobile\b|\bfone\b`)
	rotuloCidade    = re(`^cidade|\bcity\b|localizacao|\blocation\b|onde (voce )?mora`)
	rotuloPretensao = re(`pretensao|salari|salary|remuneracao|compensation|expectativa (salarial|de remuneracao)`)
	rotuloMensagem  = re(`mensagem|carta de apresentacao|cover letter|apresente-se|message to`)
)

// DetectarBloqueio devolve o motivo da lista fechada quando a página pede algo que só a pessoa faz, ou "".
// Ordem: o que impede de continuar (captcha, sessão, conta, teste, site externo) e depois os campos.
func DetectarBloqueio(r Retrato) string {
	texto := dominio.Normalizar(r.Texto)
	temSenha := false
	for _, c := range r.Campos {
		if strings.EqualFold(c.Tipo, "password") || strings.Contains(c.Autocomplete, "password") {
			temSenha = true
		}
	}
	for _, q := range r.Quadros {
		if quadroCaptcha.MatchString(strings.ToLower(q)) {
			return "captcha"
		}
	}
	for _, c := range r.Campos {
		if campoCaptcha.MatchString(strings.ToLower(c.Nome + " " + c.ID)) {
			return "captcha"
		}
	}
	if textoCaptcha.MatchString(texto) {
		return "captcha"
	}
	for _, c := range r.Campos {
		if c.Autocomplete == "one-time-code" {
			return "login"
		}
	}
	if textoCodigo.MatchString(texto) {
		return "login"
	}
	if textoCriarConta.MatchString(texto) && (temSenha || !temCampoDeCandidatura(r)) {
		return "conta"
	}
	if textoEntrar.MatchString(texto) && (temSenha || !temCampoDeCandidatura(r)) {
		return "login"
	}
	for _, q := range r.Quadros {
		if quadroTeste.MatchString(strings.ToLower(q)) {
			return "teste-pessoal"
		}
	}
	if textoTeste.MatchString(texto) {
		return "teste-pessoal"
	}
	for _, b := range r.Botoes {
		if botaoExterno.MatchString(dominio.Normalizar(b)) {
			return "site-externo"
		}
	}
	for _, c := range ClassificarCampos(r) {
		if motivo := motivoDoCampo(c); motivo != "" {
			return motivo
		}
	}
	return ""
}

// temCampoDeCandidatura: a página tem formulário de candidatura (currículo, contato ou pergunta), não só login.
func temCampoDeCandidatura(r Retrato) bool {
	for _, c := range ClassificarCampos(r) {
		switch c.Tipo {
		case CampoCurriculo, CampoNome, CampoTelefone, CampoPretensao, CampoMensagem, CampoPergunta, CampoCidade:
			return true
		}
	}
	return false
}

// motivoDoCampo: campo que sempre bloqueia. Aceite de termos só bloqueia quando é obrigatório.
func motivoDoCampo(c Campo) string {
	if _, sensivel := dominio.PendenciaDoMotivo[c.Tipo]; !sensivel {
		return ""
	}
	if c.Tipo == "termos" && !c.Obrigatorio {
		return ""
	}
	return c.Tipo
}

// ClassificarCampos traduz os campos da página para os tipos que o executor preenche. Rótulo sensível vence o tipo
// do HTML (um campo "texto" com rótulo CPF é cpf); arquivo que não é currículo é documento.
func ClassificarCampos(r Retrato) []Campo {
	saida := make([]Campo, 0, len(r.Campos))
	for _, c := range r.Campos {
		saida = append(saida, Campo{ID: c.ID, Tipo: tipoDoCampo(c), Rotulo: c.Rotulo, Obrigatorio: c.Obrigatorio})
	}
	return saida
}

func tipoDoCampo(c CampoDaPagina) string {
	tipoHTML := strings.ToLower(c.Tipo)
	rotulo := strings.TrimSpace(strings.TrimRight(dominio.Normalizar(c.Rotulo), " *:"))
	if m := dominio.MotivoDoRotulo(c.texto()); m != "" {
		return m
	}
	switch {
	case tipoHTML == "password" || strings.Contains(c.Autocomplete, "password"):
		return "senha"
	case tipoHTML == "file":
		if rotuloCurriculo.MatchString(dominio.Normalizar(c.texto())) {
			return CampoCurriculo
		}
		return "documento"
	case tipoHTML == "email" || c.Autocomplete == "email" || rotuloEmail.MatchString(rotulo):
		return CampoEmail
	case tipoHTML == "tel" || strings.HasPrefix(c.Autocomplete, "tel") || rotuloTelefone.MatchString(rotulo):
		return CampoTelefone
	case c.Autocomplete == "name" || rotuloNome.MatchString(rotulo):
		return CampoNome
	case c.Autocomplete == "address-level2" || rotuloCidade.MatchString(rotulo):
		return CampoCidade
	case rotuloPretensao.MatchString(rotulo):
		return CampoPretensao
	case rotuloMensagem.MatchString(rotulo):
		return CampoMensagem
	case tipoHTML == "checkbox" || rotulo == "":
		return CampoOutro
	}
	return CampoPergunta
}
