package dominio

import "regexp"

// Pedidos que só a pessoa responde, reconhecidos pelo texto do campo ou da pergunta (jc-a2y.6.2.4 e .6.2.6), em
// português, espanhol e inglês (o público é BR e LATAM). O executor bloqueia com o motivo, seja qual for o tipo do
// campo na página, e o pacote não pode trazer resposta para essas perguntas. Os padrões olham o texto já
// normalizado (sem acento e em minúsculas: "contraseña" vira "contrasena"). Em caso de dúvida, bloqueia: a pessoa
// faz o passo à mão.
var padroesSensiveis = []struct {
	motivo string
	re     *regexp.Regexp
}{
	// Número fiscal ou de seguridade social: CPF/CNPJ (BR), RFC (MX), CUIT/CUIL (AR), RUT (CL/UY), RUC (PE/EC/PY),
	// NIT (CO/BO), NIF (ES), SSN, ITIN e tax id (EUA e outros).
	{"cpf", regexp.MustCompile(`\bcpf\b|\bcnpj\b|cadastro de pessoa (fisica|juridica)|\brfc\b|\bcuit\b|\bcuil\b|\brut\b|\bruc\b|\bnit\b|\bnif\b|\bssn\b|social security|seguro social|seguridad social|\bitin\b|tax (id|identification|number)|taxpayer|identificacion (fiscal|tributaria)|numero (de identificacion )?(fiscal|tributario)|registro federal de contribuyentes`)},
	// Documento de identidade: RG, DNI (AR/PE/ES), CURP e INE (MX), cédula (CO/EC/UY/VE...), carnet, national id.
	{"rg", regexp.MustCompile(`\brg\b|registro geral|carteira de identidade|(numero|documento|cedula) de identidade|\borgao emissor\b|\bdni\b|documento nacional de identidad|\bcurp\b|clave unica de registro|\bcedula\b|documento de identidad|carnet de identidad|\bcredencial (para votar|de elector)\b|numero de documento|national (id|identity|identification)|identity (card|document|number)|\bid (card|number)\b|government[- ]issued id`)},
	{"senha", regexp.MustCompile(`\bsenha\b|\bpassword\b|\bpasscode\b|\bpin\b|\bcontrasena\b|\bclave (de acceso|secreta|personal)\b|^clave\b[\s*:]*$`)},
	{"dado-bancario", regexp.MustCompile(`\bagencia\b|\bconta (corrente|bancaria|poupanca|salario)\b|numero da conta|dados bancarios|\bchave pix\b|\bpix\b|\biban\b|\bswift\b|\bbic\b|routing number|\baba number\b|sort code|bank account|account number|bank details|\bbanco (para|do) (deposito|pagamento)\b|cuenta (bancaria|corriente|de ahorros?)|numero de cuenta|datos bancarios|\bclabe\b|\bcbu\b|\bcvu\b|\bsucursal bancaria\b`)},
	{"documento", regexp.MustCompile(`\bcnh\b|carteira (de motorista|nacional de habilitacao|de trabalho)|\bctps\b|comprovante|comprobante|certidao|certificado de antecedentes|antecedentes (penales|criminais)|titulo de eleitor|\bpis\b|\bpasep\b|\bnis\b|reservista|passaporte|pasaporte|\bpassport\b|driver'?s licen[cs]e|licencia de (conducir|manejo)|proof of (address|residence)`)},
	{"dado-sensivel", regexp.MustCompile(`\braca\b|\braza\b|\betnia\b|\betnic|cor ou raca|cor/raca|raca/cor|deficiencia|discapacidad|\bpcd\b|\bpne\b|genero|\bsexo\b|orientacao sexual|orientacion sexual|identidade de genero|identidad de genero|\bsaude\b|\bsalud\b|\bdoenca|enfermedad|embaraz|gravidez|gestante|pregnan|religiao|religion|\breligios|\bcrenca\b|\bcreencia|\bgender\b|\brace\b|ethnicity|disabilit|sexual orientation|\bveteran|pronome|pronoun|\btrans(genero)?\b|lgbt|data de nascimento|fecha de nacimiento|date of birth|birth ?date|\bdob\b|\bidade\b|\bedad\b|how old are you|\byour age\b|^age\b[\s*:]*$|estado civil|marital status`)},
	{"termos", regexp.MustCompile(`\btermos?\b|\bterminos\b|politica de privacidade|politica de privacidad|consentimento|consentimiento|\bconsinto\b|\baceito\b|\bacepto\b|\bconcordo\b|estoy de acuerdo|\blgpd\b|\bterms\b|privacy policy|\bi agree\b|\bconsent\b|\bautorizo\b`)},
	{"captcha", regexp.MustCompile(`captcha|nao sou (um )?robo|no soy (un )?robot|i'?m not a robot|verify (that )?you are (a )?human|verifique (que )?(voce )?(e )?humano|verifica que eres humano`)},
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
