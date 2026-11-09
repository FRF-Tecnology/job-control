package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/config"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// Apagar meus dados e sair das plataformas (jc-a2y.49). Pasta de dados fictícia num diretório temporário; cofre
// em memória (nunca o cofre do sistema).

type pastaFicticia struct {
	t      *testing.T
	pasta  string
	arm    *armazem.Armazem
	cofre  segredos.Cofre
	api    *API
	srv    *httptest.Server
	parou  atomic.Int32 // vezes que PararEnvio foi chamado
	fechou atomic.Int32 // vezes que FecharNavegador foi chamado
	// Suspender: quantas vezes foi chamado e quantas suspensões ainda não foram soltas (tem de voltar a 0).
	suspendeu atomic.Int32
	suspenso  atomic.Int32
	saiu      chan struct{}
}

func novaPastaFicticia(t *testing.T, cofre segredos.Cofre) *pastaFicticia {
	t.Helper()
	f := &pastaFicticia{t: t, pasta: filepath.Join(t.TempDir(), "Central de Candidaturas"), cofre: cofre, saiu: make(chan struct{}, 1)}
	os.MkdirAll(f.pasta, 0o700)
	arm, err := armazem.Abrir(context.Background(), config.CaminhoBanco(f.pasta))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	f.arm = arm
	for _, arq := range []string{"curriculos/cv-pt.pdf", "backups/central-2026-11-08.db", "navegador/Default/Preferences", "navegador/Default/Cookies"} {
		os.MkdirAll(filepath.Dir(filepath.Join(f.pasta, arq)), 0o700)
		os.WriteFile(filepath.Join(f.pasta, arq), []byte("ficticio"), 0o600)
	}
	for _, p := range comChave {
		cofre.Gravar(p, "chave-ficticia-"+p+"-1234")
	}
	// Grava algo no banco (vaga fictícia) para o banco ter dados de verdade.
	f.api = Nova(arm, Opcoes{Hoje: func() string { return "2026-11-09" }, Cofre: cofre, ProvedoresIA: map[string]ia.Provedor{},
		DetectarCLIs: func(context.Context) []ia.EstadoCLI { return nil },
		PararEnvio:   func() { f.parou.Add(1) }, FecharNavegador: func() { f.fechou.Add(1) },
		SuspenderEnvio: func() func() { f.suspendeu.Add(1); f.suspenso.Add(1); return func() { f.suspenso.Add(-1) } },
		AoApagar:       func() { f.saiu <- struct{}{} }})
	f.srv = httptest.NewServer(f.api.Handler())
	t.Cleanup(f.srv.Close)
	f.chamar("POST", "/vagas", Objeto{"id": "li-1", "empresa": "Empresa Teste", "titulo": "Dev Go"}, 201)
	return f
}

func (f *pastaFicticia) chamar(m, caminho string, corpo any, quer int) Objeto {
	f.t.Helper()
	b, _ := json.Marshal(corpo)
	r, _ := http.NewRequest(m, f.srv.URL+Prefixo+caminho, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	var o Objeto
	json.NewDecoder(resp.Body).Decode(&o)
	if resp.StatusCode != quer {
		f.t.Fatalf("%s %s = %d %v; queria %d", m, caminho, resp.StatusCode, o, quer)
	}
	return o
}

// arquivos lista o que existe na pasta de dados (caminhos relativos), para comparar antes e depois.
func (f *pastaFicticia) arquivos() []string {
	var l []string
	filepath.WalkDir(f.pasta, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			r, _ := filepath.Rel(f.pasta, p)
			l = append(l, filepath.ToSlash(r))
		}
		return nil
	})
	sort.Strings(l)
	return l
}

func (f *pastaFicticia) chaves() int {
	n := 0
	for _, p := range comChave {
		if c, _ := f.cofre.Ler(p); c != "" {
			n++
		}
	}
	return n
}

func lista(v any) []string {
	var l []string
	for _, x := range dominio.ListaDe(v) {
		l = append(l, dominio.Texto(x))
	}
	sort.Strings(l)
	return l
}

func TestApagarMeusDados(t *testing.T) {
	var saida bytes.Buffer
	log.SetOutput(&saida)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	f := novaPastaFicticia(t, segredos.Memoria{})
	res := f.chamar("POST", "/dados/apagar", Objeto{"confirmacao": FraseApagar}, 200)
	if strings.Join(lista(res["apagado"]), ",") != "backups,banco,chaves,curriculos,navegador" || len(lista(res["faltou"])) != 0 || res["completo"] != true || res["fechar"] != true {
		t.Fatalf("resposta = %v", res)
	}
	if _, err := os.Stat(f.pasta); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a pasta de dados ainda existe com %v", f.arquivos())
	}
	if f.chaves() != 0 {
		t.Fatalf("ficaram %d chaves no cofre", f.chaves())
	}
	if f.parou.Load() != 1 {
		t.Fatalf("PararEnvio chamado %d vezes", f.parou.Load())
	}
	select {
	case <-f.saiu:
	case <-time.After(5 * time.Second):
		t.Fatal("o app não fechou depois de apagar")
	}
	// M7: só o resultado por item, sem caminho nem chave.
	for _, proibido := range []string{f.pasta, "chave-ficticia", "Empresa Teste", "cv-pt.pdf"} {
		if strings.Contains(saida.String(), proibido) {
			t.Errorf("log tem %q: %s", proibido, saida.String())
		}
	}
	if !strings.Contains(saida.String(), "apagar: banco ok") {
		t.Errorf("log sem o resultado por item: %s", saida.String())
	}
}

// Frase errada, campo a mais, assistente e candidatura em enviando: nada muda (pasta e cofre iguais).
func TestApagarRecusaSemMudarNada(t *testing.T) {
	f := novaPastaFicticia(t, segredos.Memoria{})
	antes := f.arquivos()
	for _, corpo := range []Objeto{
		{}, {"confirmacao": "apagar meus dados"}, {"confirmacao": "APAGAR MEUS DADOS "}, {"confirmacao": FraseApagar, "tudo": true}, {"confirmacao": 1.0},
	} {
		f.chamar("POST", "/dados/apagar", corpo, 400)
	}
	f.chamar("POST", "/navegador/sair", Objeto{"perfil": "Default"}, 400)
	for _, p := range []string{"/dados/apagar", "/navegador/sair"} {
		corpo := Objeto{}
		if p == "/dados/apagar" {
			corpo = Objeto{"confirmacao": FraseApagar}
		}
		if r, err := f.api.Chamar(context.Background(), Pedido{Metodo: "POST", Caminho: Prefixo + p, Corpo: corpo, Autor: "claude"}); err != nil || r.Status != 403 {
			t.Errorf("%s como assistente = %d %v", p, r.Status, err)
		}
	}
	if f.parou.Load() != 0 || f.suspendeu.Load() != 0 || f.fechou.Load() != 0 {
		t.Fatalf("400/403 mexeram no executor: parou %d, suspendeu %d, fechou %d", f.parou.Load(), f.suspendeu.Load(), f.fechou.Load())
	}
	// Candidatura em enviando: 409 nas duas rotas. O executor para (apagar) ou é suspenso e solto (sair) antes de
	// conferir, para não haver corrida; nada é apagado.
	if err := f.arm.Gravar(context.Background(), func(e dominio.Estado) (dominio.Estado, error) {
		e.Candidaturas = append(e.Candidaturas, Objeto{"id": "cand-1", "vagaId": "li-1", "estado": dominio.CandEnviando})
		return e, nil
	}); err != nil {
		t.Fatal(err)
	}
	if r := f.chamar("POST", "/dados/apagar", Objeto{"confirmacao": FraseApagar}, 409); r["estado"] != "enviando" {
		t.Fatalf("409 = %v", r)
	}
	f.chamar("POST", "/navegador/sair", Objeto{}, 409)
	if depois := f.arquivos(); strings.Join(depois, ",") != strings.Join(antes, ",") {
		t.Fatalf("a pasta mudou:\nantes  %v\ndepois %v", antes, depois)
	}
	if f.chaves() != len(comChave) || f.parou.Load() != 1 || f.suspendeu.Load() != 1 || f.suspenso.Load() != 0 || f.fechou.Load() != 0 {
		t.Fatalf("chaves %d, parou %d, suspendeu %d (preso %d), fechou %d", f.chaves(), f.parou.Load(), f.suspendeu.Load(), f.suspenso.Load(), f.fechou.Load())
	}
	select {
	case <-f.saiu:
		t.Fatal("o app fechou sem apagar")
	case <-time.After(700 * time.Millisecond):
	}
}

// Sair das plataformas: só o perfil do navegador sai; banco, currículos, backups e chaves ficam; o app continua.
func TestSairDasPlataformas(t *testing.T) {
	f := novaPastaFicticia(t, segredos.Memoria{})
	res := f.chamar("POST", "/navegador/sair", Objeto{}, 200)
	if strings.Join(lista(res["apagado"]), ",") != "navegador" || res["completo"] != true || res["fechar"] != false || f.fechou.Load() != 1 {
		t.Fatalf("resposta = %v, fechou %d", res, f.fechou.Load())
	}
	if f.suspendeu.Load() != 1 || f.suspenso.Load() != 0 || f.parou.Load() != 0 {
		t.Fatalf("executor: suspendeu %d, preso %d, parou %d", f.suspendeu.Load(), f.suspenso.Load(), f.parou.Load())
	}
	for _, a := range f.arquivos() {
		if strings.HasPrefix(a, "navegador/") {
			t.Fatalf("o perfil não saiu: %s", a)
		}
	}
	if got := strings.Join(f.arquivos(), ","); !strings.Contains(got, "curriculos/cv-pt.pdf") || !strings.Contains(got, "backups/") || !strings.Contains(got, "central.db") {
		t.Fatalf("saiu demais: %v", got)
	}
	if f.chaves() != len(comChave) {
		t.Fatal("as chaves saíram")
	}
	f.chamar("GET", "/vagas/li-1", nil, 200) // o app continua com os dados
}

// Falha parcial: o cofre recusa; o resto sai, e a resposta diz o que faltou, sem texto do sistema.
func TestApagarFalhaParcial(t *testing.T) {
	f := novaPastaFicticia(t, segredos.Memoria{})
	f.api.cofre = cofreQuebrado{}
	res := f.chamar("POST", "/dados/apagar", Objeto{"confirmacao": FraseApagar}, 200)
	if strings.Join(lista(res["faltou"]), ",") != "chaves" || res["completo"] != false || res["fechar"] != true || strings.Join(lista(res["apagado"]), ",") != "backups,banco,curriculos,navegador" {
		t.Fatalf("resposta = %v", res)
	}
	b, _ := json.Marshal(res)
	if strings.Contains(string(b), "cofre indisponível") || strings.Contains(string(b), f.pasta) {
		t.Fatalf("resposta com texto do sistema: %s", b)
	}
}

// Só os caminhos do app: um arquivo da pessoa na mesma pasta (CENTRAL_PASTA em Documentos) fica, e a raiz também.
// Uma junção (Windows) ou link simbólico dentro de curriculos/ apontando para fora sai como link; o destino fica.
func TestApagarSoOsCaminhosDoApp(t *testing.T) {
	f := novaPastaFicticia(t, segredos.Memoria{})
	os.WriteFile(filepath.Join(f.pasta, "minha-planilha.xlsx"), []byte("da pessoa"), 0o600)
	fora := t.TempDir()
	os.WriteFile(filepath.Join(fora, "importante.txt"), []byte("fora do app"), 0o600)
	link := filepath.Join(f.pasta, "curriculos", "link-para-fora")
	var err error
	if runtime.GOOS == "windows" {
		var out []byte
		out, err = exec.Command("cmd", "/c", "mklink", "/J", link, fora).CombinedOutput()
		if err != nil {
			t.Fatalf("mklink /J: %v %s", err, out)
		}
	} else if err = os.Symlink(fora, link); err != nil {
		t.Fatal(err)
	}
	res := f.chamar("POST", "/dados/apagar", Objeto{"confirmacao": FraseApagar}, 200)
	if res["completo"] != true {
		t.Fatalf("resposta = %v", res)
	}
	if b, err := os.ReadFile(filepath.Join(fora, "importante.txt")); err != nil || string(b) != "fora do app" {
		t.Fatalf("o destino do link foi apagado: %v", err)
	}
	if got := strings.Join(f.arquivos(), ","); got != "minha-planilha.xlsx" {
		t.Fatalf("restou na pasta %q; queria só o arquivo da pessoa", got)
	}
}

// Arquivo preso (no Windows, aberto por outro processo não sai): o item vai para faltou.
func TestApagarArquivoPreso(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("arquivo aberto só fica preso no Windows")
	}
	f := novaPastaFicticia(t, segredos.Memoria{})
	preso, err := os.Open(filepath.Join(f.pasta, "curriculos", "cv-pt.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer preso.Close()
	res := f.chamar("POST", "/dados/apagar", Objeto{"confirmacao": FraseApagar}, 200)
	if strings.Join(lista(res["faltou"]), ",") != "curriculos" || res["completo"] != false {
		t.Fatalf("resposta = %v", res)
	}
}

// O binário padrão não traz o navegador por causa desta rota: a API só usa config.PastaNavegador.
func TestAPISemNavegadorNoBinarioPadrao(t *testing.T) {
	out, err := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "list", "-deps", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	for _, proibido := range []string{"chromedp", "internal/navegador", "internal/executor", "internal/envio"} {
		if strings.Contains(string(out), proibido) {
			t.Errorf("internal/api depende de %s", proibido)
		}
	}
}
