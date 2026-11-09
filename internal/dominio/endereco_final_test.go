package dominio

import "testing"

func TestEnderecoFinal(t *testing.T) {
	const vaga = "https://www.linkedin.com/jobs/view/9000000001/"
	casos := []struct{ plataforma, atual, quer string }{
		{"linkedin", "https://www.linkedin.com/jobs/view/9000000001/", ""},
		{"linkedin", "https://www.linkedin.com/jobs/view/9000000001/apply/?openSDUIApplyFlow=true", ""},
		{"linkedin", "https://br.linkedin.com/jobs/view/9000000001", ""},
		{"linkedin", "about:blank", EnderecoVazio},
		{"linkedin", "", EnderecoVazio},
		{"linkedin", "chrome-error://chromewebdata/", EnderecoVazio},
		{"linkedin", "https://www.linkedin.com/authwall?trk=x&sessionRedirect=y", EnderecoLogin},
		{"linkedin", "https://www.linkedin.com/login?session_redirect=z", EnderecoLogin},
		{"linkedin", "https://www.linkedin.com/checkpoint/challenge/abc", EnderecoLogin},
		{"linkedin", "https://www.linkedin.com/uas/login", EnderecoLogin},
		{"linkedin", "https://ats.exemplo.io/form/1", EnderecoOutroHost},
		{"linkedin", "https://linkedin.com.exemplo.com/login", EnderecoOutroHost},
		{"linkedin", "https://www.linkedin.com/jobs/view/9000000002/", EnderecoOutraVaga},
		{"linkedin", "https://www.linkedin.com/feed/", EnderecoOutraVaga},
		{"gupy", "https://login.gupy.io/candidates/signin", EnderecoLogin},
	}
	for _, c := range casos {
		aprovada := vaga
		if c.plataforma == "gupy" {
			aprovada = "https://empresa-teste.gupy.io/jobs/9000001"
		}
		if got := EnderecoFinal(c.plataforma, aprovada, c.atual); got != c.quer {
			t.Errorf("EnderecoFinal(%s, %q) = %q; queria %q", c.plataforma, c.atual, got, c.quer)
		}
	}
}
