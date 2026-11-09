package dominio

import "regexp"

// Pedidos que só a pessoa responde, reconhecidos pelo texto do campo ou da pergunta (jc-a2y.6.2.4). O executor
// bloqueia com o motivo, seja qual for o tipo do campo na página, e o pacote não pode trazer resposta para essas
// perguntas. Os padrões olham o texto já normalizado (sem acento e em minúsculas). Em caso de dúvida, bloqueia: a
// pessoa faz o passo à mão.
var padroesSensiveis = []struct {
	motivo string
	re     *regexp.Regexp
}{
	{"cpf", regexp.MustCompile(`\bcpf\b|cadastro de pessoa fisica`)},
	{"rg", regexp.MustCompile(`\brg\b|registro geral|carteira de identidade|(numero|documento|cedula) de identidade|\borgao emissor\b`)},
	{"senha", regexp.MustCompile(`\bsenha\b|\bpassword\b|\bpasscode\b|\bpin\b`)},
	{"dado-bancario", regexp.MustCompile(`\bagencia\b|\bconta (corrente|bancaria|poupanca|salario)\b|numero da conta|dados bancarios|\bchave pix\b|\bpix\b|\biban\b|\bswift\b|routing number|bank account|account number|\bbanco (para|do) (deposito|pagamento)\b`)},
	{"documento", regexp.MustCompile(`\bcnh\b|carteira (de motorista|nacional de habilitacao|de trabalho)|\bctps\b|comprovante|certidao|titulo de eleitor|\bpis\b|\bpasep\b|\bnis\b|reservista|passaporte|\bpassport\b|driver'?s license|proof of (address|residence)`)},
	{"dado-sensivel", regexp.MustCompile(`\braca\b|\betnia\b|\betnic|cor ou raca|cor/raca|raca/cor|deficiencia|\bpcd\b|\bpne\b|genero|\bsexo\b|orientacao sexual|identidade de genero|\bsaude\b|\bdoenca|religiao|\breligios|\bcrenca\b|\bgender\b|\brace\b|ethnicity|disabilit|sexual orientation|\bveteran|religion|pronome|pronoun|\btrans(genero)?\b|lgbt`)},
	{"termos", regexp.MustCompile(`\btermos?\b|politica de privacidade|consentimento|\bconsinto\b|\baceito\b|\bconcordo\b|\blgpd\b|\bterms\b|privacy policy|\bi agree\b|\bconsent\b|\bautorizo\b`)},
	{"captcha", regexp.MustCompile(`captcha|nao sou (um )?robo|i'?m not a robot|verify (that )?you are (a )?human|verifique (que )?(voce )?(e )?humano`)},
}

// MotivoDoRotulo devolve o motivo de bloqueio quando o texto pede algo que só a pessoa responde, ou "".
func MotivoDoRotulo(rotulo string) string {
	s := Normalizar(rotulo)
	if s == "" {
		return ""
	}
	for _, p := range padroesSensiveis {
		if p.re.MatchString(s) {
			return p.motivo
		}
	}
	return ""
}

// perguntaSensivel: a resposta a esta pergunta não pode estar no pacote (vale para qualquer origem).
func perguntaSensivel(pergunta string) string {
	m := MotivoDoRotulo(pergunta)
	if m == "termos" || m == "captcha" {
		return "" // aceite e captcha não são respostas de texto; a página bloqueia na hora
	}
	return m
}
