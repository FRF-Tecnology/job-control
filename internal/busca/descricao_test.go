package busca

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTextoDaDescricao(t *testing.T) {
	h := `<h2>About us</h2><p>We build   tools.</p><p>Requirements:</p><ul><li>Go</li><li>SQL &amp; APIs</li></ul>Line<br/>break &lt;script&gt;x&lt;/script&gt;`
	quer := "About us\nWe build tools.\nRequirements:\n• Go\n• SQL & APIs\nLine\nbreak x"
	if got := textoDaDescricao(h); got != quer {
		t.Errorf("parágrafos e listas:\n%q\nquer\n%q", got, quer)
	}
	longa := strings.Repeat("palavra ", 1000)
	got := textoDaDescricao(longa)
	if utf8.RuneCountInString(got) > tetoDescricao || !strings.HasSuffix(got, "palavra…") {
		t.Errorf("teto: %d caracteres, fim %q", utf8.RuneCountInString(got), got[len(got)-20:])
	}
}
