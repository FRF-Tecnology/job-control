package dominio

import (
	"slices"
	"testing"
)

func TestLinkDePerfil(t *testing.T) {
	for _, c := range []struct{ entrada, site, quer string }{
		{"https://www.linkedin.com/in/fulana/", "linkedin.com", "https://www.linkedin.com/in/fulana/"},
		{"linkedin.com/in/fulana", "linkedin.com", "https://linkedin.com/in/fulana"},
		{"http://br.linkedin.com/in/fulana", "linkedin.com", "https://br.linkedin.com/in/fulana"},
		{"github.com/fulana", "github.com", "https://github.com/fulana"},
		{"https://GitHub.com/fulana", "github.com", "https://github.com/fulana"},
		{"https://golpe.example/linkedin.com", "linkedin.com", ""},
		{"https://linkedin.com.golpe.example/in/x", "linkedin.com", ""},
		{"https://user@linkedin.com/in/x", "linkedin.com", ""},
		{"https://linkedin.com", "linkedin.com", ""}, // sem o perfil
		{"https://github.com/fulana", "linkedin.com", ""},
		{"javascript:alert(1)", "linkedin.com", ""},
		{"fulana", "github.com", ""},
		{"", "github.com", ""},
	} {
		if got := LinkDePerfil(c.entrada, c.site); got != c.quer {
			t.Errorf("LinkDePerfil(%q, %q) = %q, quer %q", c.entrada, c.site, got, c.quer)
		}
	}
}

func TestValidarLinksDoPerfil(t *testing.T) {
	if e := ValidarPerfil(map[string]any{"linkedin": "linkedin.com/in/fulana", "github": ""}); len(e) != 0 {
		t.Errorf("links certos: %v", e)
	}
	e := ValidarPerfil(map[string]any{"linkedin": "https://golpe.example/in/x", "github": "fulana"})
	if len(e) != 2 || !slices.Contains(e, "informe o endereço do seu perfil no LinkedIn, como https://linkedin.com/…") {
		t.Errorf("links errados: %v", e)
	}
}

func TestContatoIgualEmTodosOsPerfis(t *testing.T) {
	e := Estado{Perfil: Objeto{"nome": "Fulana", "cargo": "Dev Go", "cidade": "Recife"}}
	e, err := CriarPerfilBusca(e, "ia", "IA", "ai")
	if err != nil {
		t.Fatal(err)
	}
	// O contato muda no perfil ativo (IA)...
	e.Perfil["nome"], e.Perfil["cidade"], e.Perfil["linkedin"] = "Fulana de Tal", "Olinda", "https://linkedin.com/in/fulana"
	// ...e um perfil guardado antes, com nome e cidade nos critérios, não traz o contato velho de volta.
	for _, p := range e.PerfisBusca {
		if p["id"] == PerfilOriginal {
			p["criterios"] = Objeto{"cargo": "Dev Go", "nome": "Nome Velho", "cidade": "Cidade Velha"}
		}
	}
	e, err = AtivarPerfilBusca(e, PerfilOriginal)
	if err != nil {
		t.Fatal(err)
	}
	if e.Perfil["cargo"] != "Dev Go" || e.Perfil["nome"] != "Fulana de Tal" || e.Perfil["cidade"] != "Olinda" || e.Perfil["linkedin"] != "https://linkedin.com/in/fulana" {
		t.Errorf("perfil original: %v", e.Perfil)
	}
	if ia := e.PerfilDeBusca("ia"); ia["cargo"] != "AI Engineer" || ia["nome"] != "Fulana de Tal" {
		t.Errorf("perfil IA: %v", ia)
	}
}
