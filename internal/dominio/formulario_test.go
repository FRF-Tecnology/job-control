package dominio

import (
	"strings"
	"testing"
	"time"
)

// jc-a2y.7.2 (docs/propostas/formularios.md): Google Forms e Typeform pela lista exata de hosts e caminhos, nunca
// endsWith; a URL guardada é a canônica, sem query nem fragmento, e o pré-preenchimento (entry.*, campos ocultos) é
// ignorado com aviso. Endereços fictícios.
func TestFormularioDoEndereco(t *testing.T) {
	for _, c := range []struct {
		entrada    string
		canonica   string
		preenchido bool
	}{
		{"https://docs.google.com/forms/d/e/1FAIpQLSf_exemplo-123/viewform", "https://docs.google.com/forms/d/e/1FAIpQLSf_exemplo-123/viewform", false},
		{"https://docs.google.com/forms/d/1AbC_exemplo-9/viewform", "https://docs.google.com/forms/d/1AbC_exemplo-9/viewform", false},
		{"https://docs.google.com/forms/d/e/1FAIpQLSf_x/viewform?usp=sf_link", "https://docs.google.com/forms/d/e/1FAIpQLSf_x/viewform", false},
		{"https://docs.google.com/forms/d/e/1FAIpQLSf_x/viewform?entry.123=Pessoa&usp=pp_url", "https://docs.google.com/forms/d/e/1FAIpQLSf_x/viewform", true},
		{"https://forms.gle/AbC123xyz", "https://forms.gle/AbC123xyz", false},
		{"https://acme.typeform.com/to/AbC123", "https://acme.typeform.com/to/AbC123", false},
		{"https://form.typeform.com/to/AbC123", "https://form.typeform.com/to/AbC123", false},
		{"https://acme.typeform.com/to/AbC123#email=pessoa@exemplo.com", "https://acme.typeform.com/to/AbC123", true},
		{"https://acme.typeform.com/to/AbC123?utm_source=x&nome=Pessoa", "https://acme.typeform.com/to/AbC123", true},
		{"HTTPS://Docs.Google.com/forms/d/e/1FAIpQLSf_x/viewform", "https://docs.google.com/forms/d/e/1FAIpQLSf_x/viewform", false},
	} {
		canonica, preenchido, ok := FormularioDoEndereco(c.entrada)
		if !ok || canonica != c.canonica || preenchido != c.preenchido {
			t.Errorf("%s = %q %v %v; queria %q %v", c.entrada, canonica, preenchido, ok, c.canonica, c.preenchido)
		}
	}
	for _, ruim := range []string{
		"http://docs.google.com/forms/d/e/1FAIpQLSf_x/viewform",              // sem https
		"https://docs.google.com.exemplo.com/forms/d/e/1FAIpQLSf_x/viewform", // host parecido
		"https://docs.google.com/document/d/1AbC/edit",                       // fora de /forms/
		"https://docs.google.com/forms/d/e/1FAIpQLSf_x/formResponse",         // não é o viewform
		"https://docs.google.com/forms/d/e/1FAIpQLSf_x/viewform/extra",       // caminho a mais
		"https://forms.gle.exemplo.com/AbC",                                  // host parecido
		"https://forms.gle/AbC/outra",                                        // caminho a mais
		"https://typeform.com/to/AbC",                                        // sem conta
		"https://acme.typeform.com/AbC",                                      // sem /to/
		"https://acme.typeform.com.exemplo.com/to/AbC",                       // host parecido
		"https://a.b.typeform.com/to/AbC",                                    // conta com ponto
		"https://xn--typform-6ya.com/to/AbC",                                 // punycode
		"https://acme.typefоrm.com/to/AbC",                                   // homoglifo (o cirílico)
		"https://pessoa@docs.google.com/forms/d/e/1FAIpQLSf_x/viewform",      // usuário na URL
		"https://docs.google.com:8443/forms/d/e/1FAIpQLSf_x/viewform",        // porta
		"https://docs.google.com/forms/d/e/../../x/viewform",                 // ..
		"https://docs.google.com/forms/d/e/1FAIpQLSf_%2F/viewform",           // escape no id
		"https://www.linkedin.com/jobs/view/1",                               // outra plataforma
		"javascript:alert(1)",
	} {
		if c, _, ok := FormularioDoEndereco(ruim); ok {
			t.Errorf("%s passou como formulário (%q)", ruim, c)
		}
	}
}

// A vaga com link de formulário vira "formulario"; a candidatura guarda a URL canônica e só o fato do
// pré-preenchimento. Até o executor dos formulários, a aprovada segue para site-externo, como hoje.
func TestCandidaturaDeFormulario(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	e := comVaga(t, EstadoVazio(), "fm-1", "Site da empresa", "https://docs.google.com/forms/d/e/1FAIpQLSf_x/viewform?entry.123=Pessoa+Teste", true)
	if p := PlataformaDaVaga(vagaDe(e, "fm-1")); p != PlataformaFormulario {
		t.Fatalf("plataforma = %s", p)
	}
	e, c := preparada(t, e, "fm-1", agora)
	if c.Plataforma != PlataformaFormulario || c.URL != "https://docs.google.com/forms/d/e/1FAIpQLSf_x/viewform" || !c.PreenchimentoIgnorado {
		t.Fatalf("candidatura = %s %s %v", c.Plataforma, c.URL, c.PreenchimentoIgnorado)
	}
	if strings.Contains(emJSONTeste(e), "Pessoa+Teste") && strings.Contains(texto(e.Candidaturas[0]["url"]), "entry") {
		t.Fatal("o pré-preenchimento ficou na candidatura")
	}
	// Sem envio automático por enquanto: LimiteDe diz que não há executor para formulario.
	if _, automatico := LimiteDe(nil, PlataformaFormulario); automatico {
		t.Fatal("formulario com envio automático antes do executor")
	}
	// Link que não é de formulário continua "outra".
	e2 := comVaga(t, EstadoVazio(), "ou-1", "Site da empresa", "https://carreiras.exemplo.com/1", true)
	if p := PlataformaDaVaga(vagaDe(e2, "ou-1")); p != "outra" {
		t.Fatalf("site da empresa = %s", p)
	}
}
