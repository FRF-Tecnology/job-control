package dominio

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTextoLongo(t *testing.T) {
	for _, c := range []struct{ nome, entrada, quer string }{
		{"parágrafos", "Sobre nós\n\n\n\n  Somos   uma empresa.  \nRequisitos:\n- Go", "Sobre nós\n\nSomos uma empresa.\nRequisitos:\n- Go"},
		{"sem marcação", "<b>Vaga</b> boa", "bVaga/b boa"},
		{"brancos nas pontas", "\n\n  texto  \n\n", "texto"},
		{"windows", "a\r\nb", "a\nb"},
	} {
		if got := TextoLongo(c.entrada, 100); got != c.quer {
			t.Errorf("%s: %q, quer %q", c.nome, got, c.quer)
		}
	}
	got := TextoLongo("applicable laws and regulations, protected by law", 30)
	if got != "applicable laws and…" || utf8.RuneCountInString(got) > 30 {
		t.Errorf("corta na palavra inteira com reticências: %q", got)
	}
	longa := strings.Repeat("x", 80)
	if got := TextoLongo(longa, 30); utf8.RuneCountInString(got) != 30 || !strings.HasSuffix(got, "…") {
		t.Errorf("palavra única maior que o teto: %q", got)
	}
	if got := TextoLongo("Ação é ótima", 100); got != "Ação é ótima" {
		t.Errorf("acentos: %q", got)
	}
}

func TestCargoSemLocal(t *testing.T) {
	for entrada, quer := range map[string]string{
		"[Job-32191] Senior Backend IA Developer, Brazil": "Senior Backend IA Developer",
		"Senior IA Developer Java / Python, Brazil":       "Senior IA Developer Java / Python",
		"Dev Go - Remote":                  "Dev Go",
		"Engenheira iOS | Brasil (Remoto)": "Engenheira iOS",
		"Analista de Dados":                "Analista de Dados",
		"Pessoa Dev (Pleno)":               "Pessoa Dev (Pleno)",
		"":                                 "",
	} {
		if got := CargoSemLocal(entrada); got != quer {
			t.Errorf("CargoSemLocal(%q) = %q, quer %q", entrada, got, quer)
		}
	}
}
