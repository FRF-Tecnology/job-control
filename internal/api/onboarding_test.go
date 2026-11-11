package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/curriculos"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

const perfilInicialJSON = `{"cargo":"Designer","stackPrincipal":["Figma"],"filtros":{"niveis":["Pleno"],"modelo":["Remoto"],"pularSe":[]},"foco":"ambos","ingles":"Avançado","pretensaoUSD":3500}`

func pedidoOnboarding(t *testing.T, perfil, arquivo string, pdf []byte) *http.Request {
	t.Helper()
	var b bytes.Buffer
	form := multipart.NewWriter(&b)
	if err := form.WriteField("perfil", perfil); err != nil {
		t.Fatal(err)
	}
	if arquivo != "" {
		p, err := form.CreateFormFile("curriculo", arquivo)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := p.Write(pdf); err != nil {
			t.Fatal(err)
		}
	}
	if err := form.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", Prefixo+"/onboarding", &b)
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("X-Applyer-Onboarding", "1")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	return req
}

func apiPrimeiroAcesso(t *testing.T, pasta string) *API {
	t.Helper()
	a, err := armazem.Abrir(context.Background(), filepath.Join(pasta, "teste.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Fechar() })
	if _, err := a.PrepararPrimeiraVez(context.Background(), []byte("[]")); err != nil {
		t.Fatal(err)
	}
	return Nova(a, Opcoes{Cofre: &segredos.Memoria{}})
}

func estadoInicial(t *testing.T, a *API) Objeto {
	t.Helper()
	r, err := a.Chamar(context.Background(), Pedido{Metodo: "GET", Caminho: Prefixo + "/estado"})
	if err != nil || r.Status != 200 {
		t.Fatalf("estado: %#v %v", r, err)
	}
	return r.Corpo.(Objeto)
}

func TestOnboardingHTTPConcorrenciaEReabertura(t *testing.T) {
	pasta := t.TempDir()
	api := apiPrimeiroAcesso(t, pasta)
	if estadoInicial(t, api)["onboardingConcluido"] != false {
		t.Fatal("conta nova já concluída")
	}
	pdf := []byte("%PDF-1.4\n1 0 obj\n%%EOF\n")
	resultados := make(chan int, 2)
	iniciar := make(chan struct{})
	outra := apiPrimeiroAcesso(t, pasta)
	for i, arquivo := range []string{"../../aux.PDF", "CON.pdf"} {
		req := pedidoOnboarding(t, perfilInicialJSON, arquivo, pdf)
		janela := []*API{api, outra}[i]
		go func() {
			<-iniciar
			w := httptest.NewRecorder()
			janela.Handler().ServeHTTP(w, req)
			resultados <- w.Code
		}()
	}
	close(iniciar)
	status := []int{<-resultados, <-resultados}
	sort.Ints(status)
	if status[0] != 200 || status[1] != 409 {
		t.Fatalf("respostas simultâneas: %v", status)
	}

	e := estadoInicial(t, apiPrimeiroAcesso(t, pasta))
	p := e["perfil"].(Objeto)
	if e["onboardingConcluido"] != true || p["cargo"] != "Designer" || p["pretensaoUSD"] != 3500.0 {
		t.Fatalf("estado reaberto: %v", p)
	}
	arquivo := p["curriculos"].(map[string]any)["PT"].(string)
	if !strings.HasPrefix(arquivo, "curriculos/curriculo-") || !strings.HasSuffix(arquivo, ".pdf") {
		t.Fatalf("nome não portátil: %s", arquivo)
	}
	dados, err := os.ReadFile(filepath.Join(pasta, filepath.FromSlash(arquivo)))
	if err != nil || !bytes.Equal(dados, pdf) {
		t.Fatal("os bytes do PDF mudaram", err)
	}
	if len(e["curriculos"].([]Curriculo)) != 1 {
		t.Fatalf("currículos duplicados: %v", e["curriculos"])
	}
}

func TestOnboardingRecusaCamposInvalidosSemGravar(t *testing.T) {
	api := apiPrimeiroAcesso(t, t.TempDir())
	pdf := []byte("%PDF-1.4\n%%EOF")
	for _, caso := range []struct {
		nome, perfil, arquivo string
		pdf                   []byte
		quer                  int
	}{
		{"perfil inválido", "null", "f.pdf", pdf, 400},
		{"regras incompletas", "{}", "f.pdf", pdf, 400},
		{"modo de envio na entrada", strings.Replace(perfilInicialJSON, `"cargo"`, `"modo":"direto","cargo"`, 1), "f.pdf", pdf, 400},
		{"dado sensível", strings.Replace(perfilInicialJSON, "Designer", "123.456.789-00", 1), "f.pdf", pdf, 400},
		{"sem arquivo", perfilInicialJSON, "", nil, 400},
		{"extensão", perfilInicialJSON, "f.txt", pdf, 400},
		{"arquivo falso", perfilInicialJSON, "f.pdf", []byte("texto qualquer"), 400},
		{"arquivo incompleto", perfilInicialJSON, "f.pdf", []byte("%PDF-1.7"), 400},
		{"PDF grande", perfilInicialJSON, "f.pdf", make([]byte, curriculos.MaxBytes+1), 413},
	} {
		t.Run(caso.nome, func(t *testing.T) {
			w := httptest.NewRecorder()
			api.Handler().ServeHTTP(w, pedidoOnboarding(t, caso.perfil, caso.arquivo, caso.pdf))
			if w.Code != caso.quer {
				t.Fatalf("HTTP %d, corpo=%s", w.Code, w.Body)
			}
		})
	}
	for _, alteracao := range []struct {
		nome  string
		mudar func(*http.Request)
		quer  int
	}{
		{"sem header", func(r *http.Request) { r.Header.Del("X-Applyer-Onboarding") }, 403},
		{"cross site", func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") }, 403},
		{"json", func(r *http.Request) {
			r.Header.Set("Content-Type", "application/json")
			r.Body = io.NopCloser(strings.NewReader("{}"))
		}, 415},
	} {
		t.Run(alteracao.nome, func(t *testing.T) {
			r := pedidoOnboarding(t, perfilInicialJSON, "f.pdf", pdf)
			alteracao.mudar(r)
			w := httptest.NewRecorder()
			api.Handler().ServeHTTP(w, r)
			if w.Code != alteracao.quer {
				t.Fatalf("HTTP %d", w.Code)
			}
		})
	}
	e := estadoInicial(t, api)
	if e["onboardingConcluido"] != false || len(e["perfil"].(Objeto)) > 0 || len(e["curriculos"].([]Curriculo)) > 0 {
		t.Fatal("entrada inválida alterou a conta")
	}
	var perfil Objeto
	if err := json.Unmarshal([]byte(perfilInicialJSON), &perfil); err != nil {
		t.Fatal(err)
	}
	pedido := Pedido{Metodo: "POST", Caminho: Prefixo + "/onboarding", Autor: "claude", Corpo: Objeto{"perfil": perfil, "curriculo": pdf}}
	if r, err := api.Chamar(context.Background(), pedido); err != nil || r.Status != 403 {
		t.Fatalf("autor assistente: %v %v", r, err)
	}
	mcp := Nova(api.arm, Opcoes{Canal: "mcp"})
	if r, err := mcp.Chamar(context.Background(), pedido); err != nil || r.Status != 403 {
		t.Fatalf("canal mcp: %v %v", r, err)
	}
}
