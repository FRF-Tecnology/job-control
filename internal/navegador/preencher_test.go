package navegador

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// "Preencher no site da empresa" com o navegador de verdade, sem janela: uma vaga fictícia do LinkedIn cujo
// "Candidatar-se" abre o site fictício da empresa numa aba nova. O app acha a aba, preenche depois do domínio
// conferido e nunca envia. Dados fictícios.

const vagaLinkedInExterna = `<!doctype html><html><body><h1>Vaga fictícia</h1>
<button class="jobs-apply-button" onclick="window.open('https://carreiras.empresa-ficticia.test/vaga/1', '_blank')">Candidatar-se</button>
</body></html>`

const formularioDaEmpresa = `<!doctype html><html><body><h1>Trabalhe conosco</h1>
<form method="post" action="/vaga/1/enviar">
<label for="fn">First name *</label><input id="fn" name="fn" required>
<label for="ln">Last name *</label><input id="ln" name="ln" required>
<label for="em">Email *</label><input id="em" type="email" name="em" required>
<label for="ph">Phone</label><input id="ph" type="tel" name="ph">
<label for="li">LinkedIn Profile</label><input id="li" name="li">
<label for="uf">State</label><select id="uf" name="uf"><option value="">Select</option><option>PE</option><option>SP</option></select>
<label for="cpf">CPF *</label><input id="cpf" name="cpf" required>
<label for="cv">Resume/CV *</label><input id="cv" type="file" name="cv" required>
<label for="sal">Pretensão salarial *</label><input id="sal" name="sal" required>
<label><input id="termos" type="checkbox" name="termos" required> Aceito os termos</label>
<button type="submit">Enviar candidatura</button>
</form></body></html>`

func TestPreencherSiteDaEmpresa(t *testing.T) {
	var envios atomic.Int32
	srv := &servidorFicticio{}
	srv.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := strings.Split(r.Host, ":")[0]
		switch {
		case host == "www.linkedin.com" && strings.TrimSuffix(r.URL.Path, "/") == "/jobs/view/9100000001":
			fmt.Fprint(w, vagaLinkedInExterna)
		case host == "carreiras.empresa-ficticia.test" && r.Method == "GET" && r.URL.Path == "/vaga/1":
			fmt.Fprint(w, formularioDaEmpresa)
		case host == "carreiras.empresa-ficticia.test" && r.Method == "POST":
			envios.Add(1)
			fmt.Fprint(w, "recebido")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	n := navegadorDeTeste(t, srv, t.TempDir())
	n.extrasDeTeste = append(n.extrasDeTeste, chromedp.Flag("headless", "new")) // a janela "visível" sem aparecer
	ctx := context.Background()

	if _, err := n.SiteParaPreencher(ctx); !errors.Is(err, ErrSemJanela) {
		t.Fatalf("sem janela: %v", err)
	}
	if err := n.AbrirParaPreencher("https://www.linkedin.com/jobs/view/9100000001/"); err != nil {
		t.Fatal(err)
	}
	if _, login := n.Estado(); !login {
		t.Fatal("a janela de preenchimento tem de travar o envio, como a de login")
	}
	if _, err := n.NovaPagina(ctx); !errors.Is(err, ErrLoginAberto) {
		t.Fatalf("o envio abriu página com a janela de preenchimento aberta: %v", err)
	}
	// O app clica em "Candidatar-se" e a aba da empresa abre.
	var host string
	for fim := time.Now().Add(30 * time.Second); time.Now().Before(fim); time.Sleep(300 * time.Millisecond) {
		h, err := n.SiteParaPreencher(ctx)
		if err == nil {
			host = h
			break
		}
		if !errors.Is(err, ErrSemSite) {
			t.Fatal(err)
		}
	}
	if host != "carreiras.empresa-ficticia.test" {
		t.Fatalf("a aba da empresa não abriu: %q", host)
	}

	perfil := dominio.Objeto{"nome": "Fulana de Tal", "email": "fulana@example.com", "telefone": "+55 11 90000-0000", "linkedin": "https://www.linkedin.com/in/fulana-ficticia",
		"estado": "PE", "paisResidencia": "BR"}
	conteudo := dominio.ConteudoAssistido(perfil, nil)
	pdf := []byte("%PDF-1.4\nCurrículo fictício\n%%EOF\n")

	// Domínio diferente do conferido: nada é preenchido.
	if _, err := n.PreencherSite(ctx, "outro.example", conteudo, perfil, "cv.pdf", pdf); !errors.Is(err, ErrSiteMudou) {
		t.Fatalf("domínio trocado: %v", err)
	}
	r, err := n.PreencherSite(ctx, host, conteudo, perfil, "cv.pdf", pdf)
	if err != nil {
		t.Fatal(err)
	}
	if r.Preenchidos != 6 || !r.Anexou || r.Faltando != 3 {
		t.Fatalf("resultado %+v; quer 6 campos (o estado pela sigla: a lista não tem Pernambuco), currículo e 3 em branco (CPF, pretensão, termos)", r)
	}

	// Confere a página: o que foi, o que ficou em branco, e que nada foi enviado.
	n.mu.Lock()
	aba := n.abaPreencher // o site da empresa abriu na própria aba da vaga
	n.mu.Unlock()
	valores, err := chromedp.Run(aba, chromedp.Evaluate[string](`[fn.value, ln.value, em.value, ph.value, li.value, uf.value, cpf.value, sal.value, String(termos.checked), String(cv.files.length)].join('|')`))
	if err != nil {
		t.Fatal(err)
	}
	if valores != "Fulana|de Tal|fulana@example.com|+55 11 90000-0000|https://www.linkedin.com/in/fulana-ficticia|PE|||false|1" {
		t.Errorf("página: %s", valores)
	}
	time.Sleep(500 * time.Millisecond)
	if envios.Load() != 0 {
		t.Fatal("o app enviou o formulário")
	}

	// A cópia do currículo some quando a janela fecha.
	n.mu.Lock()
	pasta := n.pastaPreencher
	n.mu.Unlock()
	if _, err := os.Stat(pasta); err != nil {
		t.Fatalf("a cópia devia existir com a janela aberta: %v", err)
	}
	n.Fechar()
	for fim := time.Now().Add(5 * time.Second); time.Now().Before(fim); time.Sleep(100 * time.Millisecond) {
		if _, err := os.Stat(pasta); os.IsNotExist(err) {
			return
		}
	}
	t.Fatal("a cópia do currículo ficou depois de fechar a janela")
}
