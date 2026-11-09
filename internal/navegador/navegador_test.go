package navegador

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/curriculos"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/executor"
)

// Ponta a ponta com o Chrome (ou Edge) de verdade contra um servidor local que imita as plataformas. Nenhuma página
// real: TODO host é resolvido para o servidor local (127.0.0.1) por uma flag que só existe neste arquivo de teste,
// e o certificado é o do httptest. Dados fictícios. Pula onde não houver navegador.

const marca = "Resposta-Ficticia-Marcada-4d2b" // não pode aparecer no log

var pdf = []byte("%PDF-1.4\n% curriculo ficticio de teste\n")

// envioRecebido é o que o servidor fictício recebeu num POST de candidatura.
type envioRecebido struct {
	host, caminho string
	campos        url.Values
	arquivo       []byte
}

type servidorFicticio struct {
	*httptest.Server
	mu       sync.Mutex
	recebido []envioRecebido
}

func (s *servidorFicticio) envios() []envioRecebido {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]envioRecebido(nil), s.recebido...)
}

func novoServidor(t *testing.T) *servidorFicticio {
	t.Helper()
	s := &servidorFicticio{}
	pagina := func(nome string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			b, err := os.ReadFile(filepath.Join("testdata", "html", nome))
			if err != nil {
				http.Error(w, "sem página", 500)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(b)
		}
	}
	receber := func(confirma bool) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			r.ParseMultipartForm(1 << 20)
			e := envioRecebido{host: r.Host, caminho: r.URL.Path}
			if r.MultipartForm != nil {
				e.campos = r.MultipartForm.Value
				if fs := r.MultipartForm.File["cv"]; len(fs) == 1 {
					f, _ := fs[0].Open()
					e.arquivo, _ = io.ReadAll(f)
					f.Close()
				}
			}
			s.mu.Lock()
			s.recebido = append(s.recebido, e)
			s.mu.Unlock()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if confirma {
				fmt.Fprint(w, "<!doctype html><html><body><h1>Candidatura enviada!</h1><p>Recebemos sua candidatura.</p></body></html>")
			} else {
				fmt.Fprint(w, "<!doctype html><html><body><h1>Algo deu errado</h1></body></html>")
			}
		}
	}
	rotas := map[string]http.HandlerFunc{
		"www.linkedin.com GET /jobs/view/9000000001":        pagina("linkedin-vaga.html"),
		"www.linkedin.com POST /jobs/view/9000000001/apply": receber(true),
		"www.linkedin.com GET /jobs/view/9000000004":        pagina("linkedin-sem-confirmacao.html"),
		"www.linkedin.com POST /jobs/view/9000000004/apply": receber(false),
		"empresa-teste.gupy.io GET /jobs/9000001":           pagina("gupy-cpf.html"),
		"empresa-teste.gupy.io POST /jobs/9000001/apply":    receber(true),
		"ats.exemplo.io GET /form/1":                        pagina("ats-externo.html"),
		"ats.exemplo.io POST /form/1/enviar":                receber(true),
		"www.linkedin.com GET /jobs/view/9000000003": func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "https://ats.exemplo.io/form/1", http.StatusFound)
		},
		"www.linkedin.com GET /baixar": func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `<!doctype html><a id="b" href="/arquivo.pdf">baixar</a>`)
		},
		"www.linkedin.com GET /arquivo.pdf": func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Disposition", `attachment; filename="arquivo.pdf"`)
			w.Write(pdf)
		},
	}
	s.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := strings.Split(r.Host, ":")[0]
		if h, ok := rotas[host+" "+r.Method+" "+r.URL.Path]; ok {
			h(w, r)
			return
		}
		http.NotFound(w, r) // qualquer outro host (serviços do navegador) fica aqui, sem rede de verdade
	}))
	t.Cleanup(s.Close)
	return s
}

// navegadorDeTeste: perfil numa pasta temporária, todo host no servidor local e certificado de teste aceito.
func navegadorDeTeste(t *testing.T, s *servidorFicticio, dados string) *Navegador {
	t.Helper()
	if testing.Short() {
		t.Skip("ponta a ponta com navegador: fora do -short")
	}
	if _, err := Localizar(); err != nil {
		t.Skipf("sem Chrome nem Edge nesta máquina: %v", err)
	}
	n, err := Novo(Opcoes{PastaDados: dados})
	if err != nil {
		t.Fatal(err)
	}
	// Só no teste: os downloads iriam para uma pasta temporária, para o teste ver se algo foi baixado.
	pref := filepath.Join(PastaPerfil(dados), "Default", "Preferences")
	atual := map[string]any{}
	if b, err := os.ReadFile(pref); err == nil {
		json.Unmarshal(b, &atual)
	}
	mesclar(atual, map[string]any{"download": map[string]any{"default_directory": filepath.Join(dados, "baixados")}, "savefile": map[string]any{"default_directory": filepath.Join(dados, "baixados")}})
	b, _ := json.Marshal(atual)
	os.WriteFile(pref, b, 0o600)
	porta := s.Listener.Addr().(*net.TCPAddr).Port
	n.extrasDeTeste = []chromedp.ExecAllocatorOption{
		chromedp.Flag("host-resolver-rules", fmt.Sprintf("MAP * 127.0.0.1:%d", porta)),
		chromedp.Flag("ignore-certificate-errors", true),
	}
	t.Cleanup(n.Fechar)
	return n
}

type cenario struct {
	t     *testing.T
	arm   *armazem.Armazem
	cv    string
	agora time.Time
	x     *executor.Executor
	logs  []string
	n     int
}

func novoCenario(t *testing.T, nav executor.Navegador, dados string) *cenario {
	t.Helper()
	arm, err := armazem.Abrir(context.Background(), filepath.Join(dados, "central.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	s := &cenario{t: t, arm: arm, cv: filepath.Join(dados, "curriculos"), agora: time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local)}
	os.MkdirAll(s.cv, 0o700)
	os.WriteFile(filepath.Join(s.cv, "cv-pt.pdf"), pdf, 0o600)
	s.x = executor.Novo(arm, executor.Opcoes{
		Navegador: nav, PastaCurriculos: s.cv,
		Relogio: func() time.Time { return s.agora },
		Hoje:    func() string { return s.agora.Format("2006-01-02") },
		Esperar: func(ctx context.Context, d time.Duration) error { return ctx.Err() },
		Log:     func(f string, a ...any) { s.logs = append(s.logs, fmt.Sprintf(f, a...)) },
	})
	return s
}

func (s *cenario) aprovada(plataforma, link string) string {
	s.t.Helper()
	s.n++
	vaga := fmt.Sprintf("v-%d", s.n)
	eu := dominio.Ctx{Autor: "eu", Agora: s.agora.Format("2006-01-02")}
	var id string
	err := s.arm.Gravar(context.Background(), func(e dominio.Estado) (dominio.Estado, error) {
		e, _, err := dominio.AdicionarVaga(e, dominio.Objeto{"id": vaga, "empresa": "Empresa Teste " + vaga, "titulo": "Dev Go " + vaga, "plataforma": plataforma, "link": link}, eu, false)
		if err != nil {
			return e, err
		}
		if e, _, err = dominio.MoverVaga(e, vaga, "aplicar", nil, eu); err != nil {
			return e, err
		}
		conteudo := dominio.Objeto{
			"curriculo": dominio.Objeto{"arquivo": "cv-pt.pdf", "sha256": curriculos.SHA256(pdf)},
			"idioma":    "pt",
			"contato":   dominio.Objeto{"nome": "Pessoa Teste", "email": "pessoa@exemplo.com", "telefone": "11900000000"},
			"pretensao": nil,
			"respostas": []any{
				dominio.Objeto{"pergunta": "Anos de experiência com Go", "resposta": "3", "origem": "pessoa"},
				dominio.Objeto{"pergunta": "Experiência com banco de dados?", "resposta": "Sim", "origem": "respostasPadrao"},
				dominio.Objeto{"pergunta": "Mensagem marcada", "resposta": marca, "origem": "pessoa"},
			},
			"mensagem": nil,
		}
		e, o, err := dominio.CriarCandidatura(e, vaga, conteudo, s.agora, eu)
		if err != nil {
			return e, err
		}
		id = o["id"].(string)
		e, _, err = dominio.AprovarCandidatura(e, id, o["impressao"].(string), s.agora, nil, eu)
		return e, err
	})
	if err != nil {
		s.t.Fatal(err)
	}
	return id
}

func (s *cenario) enviar(id string) executor.Desfecho {
	s.t.Helper()
	ctx, cancelar := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelar()
	d, err := s.x.Enviar(ctx, id)
	if err != nil {
		s.t.Fatalf("Enviar = %v", err)
	}
	return d
}

// Uma sessão do navegador para todos os casos (abrir o Chrome é o que mais demora).
func TestEnvioComNavegadorDeVerdade(t *testing.T) {
	var saida bytes.Buffer
	log.SetOutput(&saida)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	srv := novoServidor(t)
	dados := t.TempDir()
	nav := navegadorDeTeste(t, srv, dados)
	s := novoCenario(t, nav, dados)

	t.Run("LinkedIn em duas etapas: envia só o pacote, com o currículo conferido", func(t *testing.T) {
		d := s.enviar(s.aprovada("linkedin", "https://www.linkedin.com/jobs/view/9000000001"))
		if d.Estado != dominio.CandEnviada {
			t.Fatalf("desfecho = %+v", d)
		}
		var e *envioRecebido
		for _, x := range srv.envios() {
			if x.caminho == "/jobs/view/9000000001/apply" {
				x := x
				e = &x
			}
		}
		if e == nil {
			t.Fatal("o servidor fictício não recebeu a candidatura")
		}
		quer := map[string]string{"nome": "Pessoa Teste", "email": "pessoa@exemplo.com", "tel": "11900000000", "anos": "3", "bd": "s"}
		for k, v := range quer {
			if e.campos.Get(k) != v {
				t.Errorf("campo %s = %q; queria %q", k, e.campos.Get(k), v)
			}
		}
		if e.campos.Get("remoto") != "" {
			t.Errorf("respondeu pergunta fora do pacote: remoto = %q", e.campos.Get("remoto"))
		}
		if !bytes.Equal(e.arquivo, pdf) {
			t.Errorf("arquivo recebido = %q", e.arquivo)
		}
	})

	t.Run("Gupy pede CPF: bloqueia sem enviar", func(t *testing.T) {
		antes := len(srv.envios())
		d := s.enviar(s.aprovada("gupy", "https://empresa-teste.gupy.io/jobs/9000001"))
		if d.Estado != dominio.CandBloqueada || d.Motivo != "cpf" || len(srv.envios()) != antes {
			t.Fatalf("desfecho = %+v, envios %d → %d", d, antes, len(srv.envios()))
		}
	})

	t.Run("Vaga redireciona para ATS externo: site-externo sem enviar", func(t *testing.T) {
		antes := len(srv.envios())
		d := s.enviar(s.aprovada("linkedin", "https://www.linkedin.com/jobs/view/9000000003"))
		if d.Estado != dominio.CandBloqueada || d.Motivo != "site-externo" || len(srv.envios()) != antes {
			t.Fatalf("desfecho = %+v, envios %d → %d", d, antes, len(srv.envios()))
		}
	})

	t.Run("Envio sem confirmação: incerta, um envio só", func(t *testing.T) {
		antes := len(srv.envios())
		id := s.aprovada("linkedin", "https://www.linkedin.com/jobs/view/9000000004")
		if d := s.enviar(id); d.Estado != dominio.CandIncerta || len(srv.envios()) != antes+1 {
			t.Fatalf("desfecho = %+v, envios %d → %d", d, antes, len(srv.envios()))
		}
		if _, err := s.x.Enviar(context.Background(), id); err == nil || len(srv.envios()) != antes+1 {
			t.Fatal("incerta foi enviada de novo")
		}
	})

	t.Run("O retrato só lê: o DOM não muda", func(t *testing.T) {
		pag, err := nav.NovaPagina(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer pag.Fechar()
		p := pag.(*pagina)
		ctx := context.Background()
		if err := p.fazer(ctx, tempoAbrir, chromedp.Navigate("https://www.linkedin.com/jobs/view/9000000001")); err != nil {
			t.Fatal(err)
		}
		dom := func() string {
			v, err := rodar(ctx, p, tempoAcao, chromedp.Evaluate[string](`document.documentElement.outerHTML`))
			if err != nil {
				t.Fatal(err)
			}
			return v
		}
		antes := dom()
		r, err := p.retrato(ctx)
		if err != nil || len(r.Botoes) == 0 || !strings.Contains(r.Texto, "Empresa Teste") {
			t.Fatalf("retrato = %+v (%v)", r, err)
		}
		if depois := dom(); depois != antes {
			t.Fatal("o script de retrato mudou a página")
		}
	})

	t.Run("Download negado", func(t *testing.T) {
		pag, err := nav.NovaPagina(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer pag.Fechar()
		p := pag.(*pagina)
		ctx := context.Background()
		p.fazer(ctx, tempoAbrir, chromedp.Navigate("https://www.linkedin.com/baixar"), chromedp.Click(chromedp.CSS("#b")), chromedp.Sleep(2*time.Second))
		for _, dir := range []string{filepath.Join(dados, "baixados"), filepath.Join(PastaPerfil(dados), "Downloads"), dados} {
			itens, _ := os.ReadDir(dir)
			for _, i := range itens {
				if strings.HasSuffix(i.Name(), ".pdf") || strings.HasSuffix(i.Name(), ".crdownload") {
					t.Fatalf("baixou %s em %s", i.Name(), dir)
				}
			}
		}
	})

	t.Run("Sair das plataformas: o perfil volta com as preferências na próxima página", func(t *testing.T) {
		nav.Fechar()
		if err := ApagarPerfil(dados); err != nil {
			t.Fatal(err)
		}
		pag, err := nav.NovaPagina(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer pag.Fechar()
		b, err := os.ReadFile(filepath.Join(PastaPerfil(dados), "Default", "Preferences"))
		if err != nil || !strings.Contains(string(b), "password_manager_enabled") {
			t.Fatalf("perfil recriado sem as preferências: %v", err)
		}
	})

	// M7: nada da página nem do pacote no log (do executor ou do chromedp).
	tudo := strings.Join(s.logs, "\n") + "\n" + saida.String()
	for _, proibido := range []string{marca, "Pessoa Teste", "pessoa@exemplo.com", "Empresa Teste", "linkedin.com", "ats.exemplo.io", "cv-pt.pdf", dados} {
		if strings.Contains(tudo, proibido) {
			t.Errorf("log tem %q:\n%s", proibido, tudo)
		}
	}
}
