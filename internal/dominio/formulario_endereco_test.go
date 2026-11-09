package dominio

import "testing"

// jc-a2y.7.2, parte 3: no envio, a página do formulário tem de ficar no formulário aprovado (o destino canônico
// guardado na leitura). O login do Google é "login"; outro formulário, "outra-vaga"; outro site, "outro-host".
// Endereços fictícios.
func TestEnderecoDoFormulario(t *testing.T) {
	aprovada := "https://docs.google.com/forms/d/e/1FAIpQLSf_x/viewform"
	for atual, quer := range map[string]string{
		"https://docs.google.com/forms/d/e/1FAIpQLSf_x/viewform":                 "",
		"https://docs.google.com/forms/d/e/1FAIpQLSf_x/viewform?usp=sf_link":     "",
		"https://docs.google.com/forms/d/e/1FAIpQLSf_x/viewform?entry.1=Pessoa":  "",
		"https://docs.google.com/forms/d/e/1FAIpQLSf_OUTRO/viewform":             EnderecoOutraVaga,
		"https://accounts.google.com/v3/signin/identifier?continue=https://docs": EnderecoLogin,
		"https://exemplo.com/forms/d/e/1FAIpQLSf_x/viewform":                     EnderecoOutroHost,
		"https://docs.google.com.exemplo.com/forms/d/e/1FAIpQLSf_x/viewform":     EnderecoOutroHost,
		"about:blank": EnderecoVazio,
	} {
		if got := EnderecoFinal(PlataformaFormulario, aprovada, atual); got != quer {
			t.Errorf("%s = %q; queria %q", atual, got, quer)
		}
	}
	typeform := "https://acme.typeform.com/to/AbC123"
	if !MesmaVaga(PlataformaFormulario, typeform, typeform+"#nome=x") {
		t.Error("Typeform com fragmento não é o mesmo formulário")
	}
	if MesmaVaga(PlataformaFormulario, typeform, "https://outra.typeform.com/to/AbC123") {
		t.Error("outra conta do Typeform passou como o mesmo formulário")
	}
}
