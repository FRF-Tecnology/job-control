package navegador

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// jc-a2y.7.2, parte 3a: leitura de formulários sem interagir. Páginas FICTÍCIAS, no formato público do
// FB_PUBLIC_LOAD_DATA_ (não conferido contra um formulário real, decisão do PO); nada contra o Google nem o Typeform
// de verdade. O Chrome roda sem janela (o padrão do NovaPagina).

const dadosUmaPagina = `[null,["Descrição fictícia",[[111,"Qual o seu e-mail?",null,0,[[1001,null,1]]],[113,"Nível de inglês",null,2,[[1003,[["Básico"],["Avançado"]],1]]]]],"/forms","Formulário fictício"]`
const dadosSecoes = `[null,["d",[[1,"Nome",null,0,[[10,null,1]]],[2,"Seção 2",null,8,null],[3,"Cidade",null,0,[[11,null,1]]]]]]`

func paginaGoogle(dados, corpo string) string {
	return `<!doctype html><html><head><meta charset="utf-8"><script>var FB_PUBLIC_LOAD_DATA_ = ` + dados + `;</script></head><body><h1>Formulário fictício</h1>` + corpo + `</body></html>`
}

// servidorDeFormularios: um servidor TLS só deste teste (os hosts vão todos para ele pelo host-resolver-rules do
// navegadorDeTeste), sem mexer no servidor fictício do envio.
func servidorDeFormularios(t *testing.T) *servidorFicticio {
	t.Helper()
	g := "/forms/d/e/1FAIpQLSf_"
	rotas := map[string]string{
		"docs.google.com " + g + "ok/viewform":       paginaGoogle(dadosUmaPagina, `<form><input name="entry.1001"></form>`),
		"docs.google.com " + g + "secoes/viewform":   paginaGoogle(dadosSecoes, ""),
		"docs.google.com " + g + "login/viewform":    `<!doctype html><html><body><p>Você precisa fazer login para continuar.</p></body></html>`,
		"docs.google.com " + g + "email/viewform":    paginaGoogle(dadosUmaPagina, `<p>O seu e-mail será registrado quando você enviar este formulário.</p>`),
		"docs.google.com " + g + "iframe/viewform":   paginaGoogle(dadosUmaPagina, `<iframe src="https://exemplo.com/pagamento"></iframe>`),
		"docs.google.com " + g + "arquivo/viewform":  paginaGoogle(dadosUmaPagina, `<input type="file" name="cv">`),
		"docs.google.com " + g + "semdados/viewform": `<!doctype html><html><body><p>Carregando...</p></body></html>`,
		// com dados que o leitor do Google aceitaria: o Typeform tem de bloquear antes de ler qualquer coisa
		"acme.typeform.com /to/AbC123": paginaGoogle(dadosUmaPagina, `<p>Typeform fictício</p>`),
	}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := strings.Split(r.Host, ":")[0]
		switch {
		case host == "forms.gle" && r.URL.Path == "/dentro":
			http.Redirect(w, r, "https://docs.google.com"+g+"ok/viewform?usp=sf_link", http.StatusFound)
			return
		case host == "forms.gle" && r.URL.Path == "/fora":
			http.Redirect(w, r, "https://exemplo.com/form", http.StatusFound)
			return
		case host == "exemplo.com":
			fmt.Fprint(w, `<!doctype html><html><body>outro site</body></html>`)
			return
		}
		if html, ok := rotas[host+" "+r.URL.Path]; ok {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, html)
			return
		}
		// Ler nunca envia: nenhum POST para o formulário (o Chrome faz chamadas próprias a outros caminhos, que o
		// host-resolver do teste também traz para cá; essas não contam).
		if r.Method == http.MethodPost && (strings.HasPrefix(r.URL.Path, "/forms/") || strings.HasPrefix(r.URL.Path, "/to/") || host == "forms.gle") {
			t.Errorf("a leitura enviou algo: %s %s", r.Method, r.URL.Path)
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return &servidorFicticio{Server: srv}
}

func TestLerFormularioComChrome(t *testing.T) {
	srv := servidorDeFormularios(t)
	n := navegadorDeTeste(t, srv, t.TempDir())
	g := "https://docs.google.com/forms/d/e/1FAIpQLSf_"
	l, err := n.LerFormulario(context.Background(), g+"ok/viewform")
	if err != nil || l.Bloqueio != "" || len(l.Perguntas) != 2 || l.URLFinal != g+"ok/viewform" {
		t.Fatalf("uma página = %+v %v", l, err)
	}
	if l.Perguntas[0].ID != "entry.1001" || l.Perguntas[1].Tipo != "escolha-unica" || strings.Join(l.Perguntas[1].Opcoes, ",") != "Básico,Avançado" {
		t.Fatalf("perguntas = %+v", l.Perguntas)
	}
	for endereco, quer := range map[string]string{
		g + "secoes/viewform":                 "varias-etapas", // decisão do PO
		g + "login/viewform":                  "login",
		g + "email/viewform":                  "login", // e-mail registrado: nunca enviamos a identidade da conta
		g + "iframe/viewform":                 "site-externo",
		g + "arquivo/viewform":                "outro",
		g + "semdados/viewform":               "outro",
		"https://acme.typeform.com/to/AbC123": "outro", // sem amostra real do formato: a pessoa envia
		"https://forms.gle/fora":              "site-externo",
	} {
		l, err := n.LerFormulario(context.Background(), endereco)
		if err != nil || l.Bloqueio != quer || len(l.Perguntas) != 0 {
			t.Errorf("%s = %+v %v; queria bloqueio %s", endereco, l, err, quer)
		}
	}
	// forms.gle que leva ao formulário da lista: o destino canônico, sem a query.
	l, err = n.LerFormulario(context.Background(), "https://forms.gle/dentro")
	if err != nil || l.Bloqueio != "" || l.URLFinal != g+"ok/viewform" {
		t.Fatalf("forms.gle = %+v %v", l, err)
	}
}

// A decisão só com o retrato da página (sem Chrome): bloqueios antes da estrutura, e o que vier fora do esperado
// fica com a pessoa.
func TestLeituraDoRetrato(t *testing.T) {
	retrato := func(google *string, texto string, iframes []string, arquivo bool) string {
		b, _ := json.Marshal(retratoFormulario{Google: google, Texto: texto, Iframes: iframes, Arquivo: arquivo})
		return string(b)
	}
	ok := dadosUmaPagina
	canon := "https://docs.google.com/forms/d/e/1FAIpQLSf_ok/viewform"
	if l := leituraDoRetrato(retrato(&ok, "Formulário", []string{"docs.google.com", ""}, false), canon); l.Bloqueio != "" || len(l.Perguntas) != 2 {
		t.Fatalf("normal = %+v", l)
	}
	ruim := "não é json"
	for nome, c := range map[string]struct {
		bruto, quer string
	}{
		"Sign in to continue":       {retrato(&ok, "Sign in to continue", nil, false), "login"},
		"will be recorded":          {retrato(&ok, "Your email will be recorded when you submit", nil, false), "login"},
		"iframe de outro domínio":   {retrato(&ok, "", []string{"pagamentos.exemplo.com"}, false), "site-externo"},
		"upload":                    {retrato(&ok, "", nil, true), "outro"},
		"estrutura fora do formato": {retrato(&ruim, "", nil, false), "outro"},
		"retrato que não é JSON":    {"<script>", "outro"},
		"retrato grande demais":     {retrato(&ok, strings.Repeat("a", maxRetratoFormulario), nil, false), "outro"}, // JSON bom: só o teto bloqueia
	} {
		if l := leituraDoRetrato(c.bruto, canon); l.Bloqueio != c.quer || len(l.Perguntas) != 0 {
			t.Errorf("%s = %+v; queria %s", nome, l, c.quer)
		}
	}
}
