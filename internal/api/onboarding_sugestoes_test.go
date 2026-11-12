package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/ia"
)

// PDF real e mínimo: índice de objetos, uma página, fonte e texto selecionável.
func cvComTexto(linhas ...string) []byte {
	var pagina, b bytes.Buffer
	for i, linha := range linhas {
		fmt.Fprintf(&pagina, "BT /F1 12 Tf 50 %d Td (%s) Tj ET\n", 700-18*i, linha)
	}
	objetos := []string{
		"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", pagina.Len(), pagina.String()),
	}
	b.WriteString("%PDF-1.4\n")
	var offsets []int
	for i, obj := range objetos {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objetos)+1)
	for _, n := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", n)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objetos)+1, xref)
	return b.Bytes()
}

func onboardingJSON(t *testing.T, a *API, rota string, corpo Objeto) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(corpo)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", Prefixo+rota, bytes.NewReader(b))
	req.Header.Set("X-Applyer-Onboarding", "1")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, req)
	return w
}

type provedorParaCV struct {
	t        *testing.T
	chamadas int
}

func (p *provedorParaCV) Nome() string { return "teste" }
func (p *provedorParaCV) Chamar(_ context.Context, pedido ia.PedidoProvedor) (ia.RespostaProvedor, error) {
	p.chamadas++
	msg := pedido.Prompt.Mensagem
	if strings.Contains(msg, "AB123456") {
		p.t.Fatal("número de passaporte no texto enviado ao provedor")
	}
	if pedido.Prompt.NomeEsquema != "sugerirPerfilDoCV" || !strings.Contains(msg, "Senior Java Developer") ||
		strings.Contains(msg, "@") || strings.Contains(msg, "987654") || strings.Contains(msg, "%PDF") {
		p.t.Fatal("conteúdo inesperado no pedido")
	}
	return ia.RespostaProvedor{Texto: `{"sugestoes":[{"campo":"cargo","valor":"Java Developer","evidencia":"Senior Java Developer"},{"campo":"niveis","valor":"Sênior","evidencia":"Senior Java Developer"},{"campo":"ingles","valor":"Fluente","evidencia":"Fluent English"},{"campo":"stackPrincipal","valor":"Java","evidencia":"Senior Java Developer"},{"campo":"ingles","valor":"Básico","evidencia":"Senior Java Developer"},{"campo":"stackSecundaria","valor":"Docker","evidencia":"Inventada"},{"campo":"stackSecundaria","valor":"Scala","evidencia":"Senior Java Developer"}]}`}, nil
}

func TestOnboardingSugereCVComEvidenciaSemSalvar(t *testing.T) {
	ctx := context.Background()
	pasta := t.TempDir()
	a := apiPrimeiroAcesso(t, pasta)
	arquivo := cvComTexto("Senior Java Developer", "Fluent English", "teste@example.test", "CPF 98765432100", "Passaporte: AB123456")
	corpo := Objeto{"curriculoBase64": arquivo}
	if w := onboardingJSON(t, a, "/onboarding/sugerir", corpo); w.Code != 409 {
		t.Fatalf("sem IA: %d", w.Code)
	}
	a.arm.GravarMeta(ctx, chaveConfig, ia.Config{Rotas: map[string]ia.Rota{"padrao": {Provedor: "teste"}}})
	prov := &provedorParaCV{t: t}
	a.provedoresExtras = map[string]ia.Provedor{"teste": prov}
	w := onboardingJSON(t, a, "/onboarding/sugerir", corpo)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	var res struct{ Sugestoes []Objeto }
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil || len(res.Sugestoes) != 4 || prov.chamadas != 1 {
		t.Fatalf("sugestões %v", res.Sugestoes)
	}
	if e := estadoInicial(t, a); e["onboardingConcluido"] != false || len(e["perfil"].(Objeto)) != 0 || len(e["curriculos"].([]Curriculo)) != 0 {
		t.Fatal("sugestão persistida sem a pessoa salvar")
	}
	if w := onboardingJSON(t, a, "/onboarding/sugerir", Objeto{"curriculoBase64": cvComTexto()}); w.Code != 422 || prov.chamadas != 1 {
		t.Fatalf("arquivo sem camada de texto: %d", w.Code)
	}
	var perfil Objeto
	json.Unmarshal([]byte(perfilInicialJSON), &perfil)
	if w := onboardingJSON(t, a, "/onboarding", Objeto{"perfil": perfil, "curriculoBase64": arquivo}); w.Code != 200 {
		t.Fatalf("salvou %d %s", w.Code, w.Body.String())
	}
	cv := estadoInicial(t, a)["perfil"].(Objeto)["curriculos"].(Objeto)["PT"].(string)
	salvo, err := os.ReadFile(filepath.Join(pasta, filepath.FromSlash(cv)))
	if err != nil || !bytes.Equal(salvo, arquivo) {
		t.Fatal("bytes diferentes após o transporte JSON")
	}
}

func TestOnboardingJSONTrataPDFQueNaoChegou(t *testing.T) {
	a := apiPrimeiroAcesso(t, t.TempDir())
	for _, caso := range []struct {
		conteudo any
		mensagem string
	}{
		{"", "não chegou"}, {"-invalido-", "transferir"}, {[]byte("x"), "cabeçalho"}, {[]byte("%PDF-1.4"), "incompleto"},
	} {
		w := onboardingJSON(t, a, "/onboarding", Objeto{"curriculoBase64": caso.conteudo, "perfil": Objeto{}})
		if w.Code != 400 || !strings.Contains(w.Body.String(), caso.mensagem) {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
	}
}

func TestOnboardingSugestaoDependeDaHabilidadeEDoIdiomaCitados(t *testing.T) {
	for _, c := range []struct {
		campo, valor, trecho string
		quer                 bool
	}{
		{"stackPrincipal", "Go", "Django", false},
		{"stackPrincipal", "C++", "Trabalho com C++ e Go.", true},
		{"stackPrincipal", "Java", "JavaScript", false},
		{"ingles", "Fluente", "Português fluente; inglês básico", false},
		{"ingles", "Básico", "Português fluente; inglês básico", true},
		{"ingles", "Avançado", "English: C1", true},
		{"ingles", "Fluente", "not fluent in English", false},
		{"niveis", "Sênior", "10 years in development", false},
	} {
		if got := valorExplicito(c.campo, c.valor, c.trecho); got != c.quer {
			t.Errorf("%s %q: %v", c.campo, c.valor, got)
		}
	}
}
