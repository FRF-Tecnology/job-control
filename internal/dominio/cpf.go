package dominio

import "regexp"

// CPF da pessoa (decisão da PO, contra a regra anterior de nunca preencher CPF; ver docs/propostas/cpf-no-preencher.md):
//   - fica só no cofre do sistema (Keychain), nunca no banco, no pacote, no log, na IA ou no assistente;
//   - a tela vê só a máscara (***.456.789-**);
//   - vai só para o "Preencher no site da empresa", em site brasileiro, num campo cujo rótulo é CPF, e só depois de a
//     pessoa confirmar naquele domínio. O envio automático continua sem CPF.

// CPFValido devolve os 11 dígitos de um CPF com os dígitos verificadores certos (com ou sem pontuação).
func CPFValido(s string) (string, bool) {
	d := make([]byte, 0, 11)
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			d = append(d, c)
		case c == '.' || c == '-' || c == ' ':
		default:
			return "", false
		}
	}
	if len(d) != 11 {
		return "", false
	}
	iguais := true
	for _, c := range d[1:] {
		iguais = iguais && c == d[0]
	}
	if iguais { // 000.000.000-00, 111.111.111-11...: passam na conta, mas não existem
		return "", false
	}
	for _, n := range []int{9, 10} {
		soma := 0
		for i := 0; i < n; i++ {
			soma += int(d[i]-'0') * (n + 1 - i)
		}
		dv := soma * 10 % 11 % 10
		if int(d[n]-'0') != dv {
			return "", false
		}
	}
	return string(d), true
}

// FormatarCPF: "12345678909" vira "123.456.789-09" (o jeito que os formulários pedem).
func FormatarCPF(d string) string {
	if len(d) != 11 {
		return ""
	}
	return d[:3] + "." + d[3:6] + "." + d[6:9] + "-" + d[9:]
}

// MascararCPF: só o miolo aparece ("***.456.789-**"), para a pessoa reconhecer qual CPF está salvo.
func MascararCPF(d string) string {
	if len(d) != 11 {
		return ""
	}
	return "***." + d[3:6] + "." + d[6:9] + "-**"
}

var (
	rotuloCPF  = regexp.MustCompile(`\bcpf\b|cadastro de pessoas? fisicas?`)
	rotuloCNPJ = regexp.MustCompile(`\bcnpj\b`)
)

// RotuloDeCPF: o campo pede o CPF (e não CNPJ, SSN, NIF ou outro documento, que o app nunca preenche).
func RotuloDeCPF(rotulo string) bool {
	s := Normalizar(rotulo)
	return rotuloCPF.MatchString(s) && !rotuloCNPJ.MatchString(s)
}
