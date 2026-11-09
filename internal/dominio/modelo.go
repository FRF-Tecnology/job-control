package dominio

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Vocabulário comum da Central (porte de nucleo/modelo.mjs).
var (
	Etapas    = []string{"descoberta", "aplicar", "enviada", "bloqueada", "em-processo", "oferta", "encerrada", "pulada"}
	NomeEtapa = map[string]string{
		"descoberta": "Descoberta", "aplicar": "Aplicar", "enviada": "Enviada", "bloqueada": "Precisa de você",
		"em-processo": "Em processo", "oferta": "Oferta", "encerrada": "Encerrada", "pulada": "Pulada",
	}
	SubEtapas           = []string{"teste", "entrevista-rh", "entrevista-tecnica", "proposta"}
	MotivosPulo         = []string{"Sênior", "Inglês", "Diploma", "Stack", "Presencial/Local", "Documento", "Duplicada", "Vaga fechada", "Outro"}
	MotivosEncerramento = []string{"Reprovada", "Sem retorno", "Vaga fechada", "Desisti", "Outro"}
	// Mercado da vaga e foco da busca (seletor Internacional / Nacional / Os dois).
	Mercados = []string{"internacional", "nacional"}
	Focos    = []string{"internacional", "nacional", "ambos"}
	// Etapas que exigem um texto explicando o porquê.
	PedeMotivo = []string{"pulada", "encerrada"}
)

// Normalizar tira acentos e caixa, e apara as pontas.
func Normalizar(v any) string {
	s := norm.NFD.String(texto(v))
	s = strings.Map(func(r rune) rune {
		if r >= 0x300 && r <= 0x36F {
			return -1
		}
		return r
	}, s)
	return aparar(strings.ToLower(s))
}

// HojeISO devolve a data local como AAAA-MM-DD.
func HojeISO(t time.Time) string { return t.In(time.Local).Format("2006-01-02") }

// SomarDias soma dias a uma data AAAA-MM-DD (atravessa mês e ano).
func SomarDias(iso string, dias int) string {
	p := strings.Split(iso, "-")
	if len(p) != 3 {
		return "NaN-NaN-NaN"
	}
	a, e1 := strconv.Atoi(p[0])
	m, e2 := strconv.Atoi(p[1])
	d, e3 := strconv.Atoi(p[2])
	if e1 != nil || e2 != nil || e3 != nil {
		return "NaN-NaN-NaN"
	}
	return HojeISO(time.Date(a, time.Month(m), d+dias, 0, 0, 0, 0, time.Local))
}

var reDataISO = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// EhDataISO diz se v é texto no formato AAAA-MM-DD.
func EhDataISO(v any) bool {
	s, ok := v.(string)
	return ok && reDataISO.MatchString(s)
}

var (
	reNacional      = regexp.MustCompile(`\bclt\b|nacional|\bbrl\b|reais`)
	reInternacional = regexp.MustCompile(`\busd\b|contractor|overseas|dolar`)
)

// MercadoDe: o mercado anotado na vaga ou um palpite pelas anotações (CLT/nacional pesa mais) e pelo idioma.
func MercadoDe(v Objeto) string {
	if contem(Mercados, v["mercado"]) {
		return v["mercado"].(string)
	}
	t := Normalizar(texto(v["motivoClaude"]) + " " + texto(v["notas"]) + " " + texto(v["contrato"]))
	if reNacional.MatchString(t) {
		return "nacional"
	}
	if reInternacional.MatchString(t) || igual(v["idioma"], "en") {
		return "internacional"
	}
	return "nacional"
}

// NoFoco diz se a vaga entra no foco escolhido (vazio ou "ambos" = todas).
func NoFoco(v Objeto, foco any) bool {
	return !verdadeiro(foco) || igual(foco, "ambos") || igual(MercadoDe(v), foco)
}

// FoiEnviada diz se a vaga tem data de envio.
func FoiEnviada(v Objeto) bool { return verdadeiro(obj(v["envio"])["data"]) }

var (
	reMilhar  = regexp.MustCompile(`^\d{1,3}(\.\d{3})+(,\d+)?$`)
	reVirgula = regexp.MustCompile(`^\d+(,\d+)?$`)
	rePonto   = regexp.MustCompile(`^\d+(\.\d+)?$`)
)

// LerValor lê dinheiro digitado à brasileira: "8.500", "8.500,50", "R$ 8.500" ou "8500.5".
// ok=false quando não é um valor.
func LerValor(v any) (float64, bool) {
	s := strings.Map(func(r rune) rune {
		if r == 'R' || r == '$' || unicode.IsSpace(r) || r == 0xFEFF {
			return -1
		}
		return r
	}, texto(v))
	switch {
	case reMilhar.MatchString(s):
		s = strings.Replace(strings.ReplaceAll(s, ".", ""), ",", ".", 1)
	case reVirgula.MatchString(s):
		s = strings.Replace(s, ",", ".", 1)
	case rePonto.MatchString(s):
	default:
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil
}
