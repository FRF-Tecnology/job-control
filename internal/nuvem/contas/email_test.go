package contas

import (
	"strings"
	"testing"
)

func TestEmailNormalizado(t *testing.T) {
	got, err := NormalizarEmail(" Ana@Exemplo.com ")
	if err != nil || got != "ana@exemplo.com" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestEmailInvalido(t *testing.T) {
	longo := strings.Repeat("a", 250) + "@b.co"
	for _, s := range []string{"", "   ", "semarroba", "a b@c.com", "@c.com", "a@", "a@@b.com", longo} {
		if _, err := NormalizarEmail(s); err == nil {
			t.Errorf("%q deveria ser recusado", s)
		}
	}
}
