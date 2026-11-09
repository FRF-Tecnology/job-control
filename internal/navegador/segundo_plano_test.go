package navegador

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// jc-a2y.6.7: como o navegador é aberto, com um navegador falso (testdata/falso) que anota os argumentos e a
// prioridade e sai. Nenhuma janela abre, nem com CHROMEDP_VISIBLEWINDOW no ambiente.

func navegadorFalso(t *testing.T) (*Navegador, func() []string) {
	t.Helper()
	if testing.Short() {
		t.Skip("compilar o navegador falso: fora do -short")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "falso.exe")
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	if out, err := exec.Command(goBin, "build", "-o", exe, "./testdata/falso").CombinedOutput(); err != nil {
		t.Fatalf("go build do navegador falso: %v\n%s", err, out)
	}
	n := &Navegador{exec: exe, perfil: filepath.Join(dir, "perfil")}
	chamadas := func() []string {
		b, err := os.ReadFile(filepath.Join(dir, "chamadas.txt"))
		if err != nil {
			t.Fatalf("o navegador falso não foi chamado: %v", err)
		}
		return strings.Split(strings.TrimSpace(string(b)), "\n")
	}
	return n, chamadas
}

func TestNavegadorAbreSemJanela(t *testing.T) {
	t.Setenv("CHROMEDP_VISIBLEWINDOW", "1") // o chromedp só olha isso sem alocador próprio; aqui não pode mudar nada
	n, chamadas := navegadorFalso(t)

	// Envio (padrão): sem janela.
	n.mu.Lock()
	err := n.iniciarSemTrava(n.querVisivel)
	n.mu.Unlock()
	if err == nil {
		t.Fatal("o navegador falso não é um navegador: iniciar deveria falhar")
	}
	c := chamadas()
	prio, args, _ := strings.Cut(c[len(c)-1], "\t")
	if !strings.Contains(args, "--headless=new") || strings.Contains(args, "--start-maximized") {
		t.Errorf("envio padrão abriu com janela: %s", args)
	}
	if !strings.Contains(args, "--remote-debugging-pipe") || strings.Contains(args, "--remote-debugging-port") {
		t.Errorf("DevTools fora do pipe: %s", args)
	}
	if runtime.GOOS == "windows" && prio != "0x4000" { // BELOW_NORMAL_PRIORITY_CLASS
		t.Errorf("prioridade do navegador = %s; queria 0x4000 (abaixo do normal)", prio)
	}

	// "Ver o envio" / login: com janela (só por ação da tela; aqui, com o falso, nada aparece).
	n.mu.Lock()
	err = n.iniciarSemTrava(true)
	n.mu.Unlock()
	if err == nil {
		t.Fatal("iniciar visível deveria falhar com o falso")
	}
	c = chamadas()
	_, args, _ = strings.Cut(c[len(c)-1], "\t")
	if strings.Contains(args, "--headless") {
		t.Errorf("modo visível abriu sem janela: %s", args)
	}
}

// O modo só troca quando a pessoa pede, e a página do envio que está aberta termina no modo em que começou.
func TestModoDaJanelaSoPorPedido(t *testing.T) {
	n := &Navegador{}
	if v, l := n.Estado(); v || l {
		t.Fatalf("ao abrir: visível %v, login %v", v, l)
	}
	n.MostrarEnvio(true)
	if v, _ := n.Estado(); !v {
		t.Fatal("MostrarEnvio(true) não ligou")
	}
	n.MostrarEnvio(false)
	if v, _ := n.Estado(); v {
		t.Fatal("MostrarEnvio(false) não desligou")
	}
	if n.Ocupado() {
		t.Fatal("ocupado sem login")
	}
}

// Login com o navegador de verdade e janela visível: SÓ NO CI. Neste PC, ninguém abre janela sem horário combinado
// com quem usa o computador (pedido do PO no jc-a2y.6.7).
func TestLoginAbreJanelaESegura(t *testing.T) {
	if os.Getenv("CI") != "true" {
		t.Skip("abre uma janela visível: só no CI")
	}
	srv := novoServidor(t)
	dados := t.TempDir()
	nav := navegadorDeTeste(t, srv, dados)

	if err := nav.AbrirLogin("https://www.linkedin.com/login"); err != nil {
		t.Fatal(err)
	}
	if !nav.Ocupado() {
		t.Fatal("com a janela de login aberta, o navegador não está ocupado")
	}
	if err := nav.AbrirLogin("https://www.linkedin.com/login"); err != ErrLoginAberto {
		t.Fatalf("segundo login = %v", err)
	}
	if _, err := nav.NovaPagina(context.Background()); err != ErrLoginAberto {
		t.Fatalf("página do envio com login aberto = %v", err)
	}
	// A pessoa fecha a janela (o navegador sai): o envio volta, sem janela. Fecha pelo próprio Chrome (Browser.close),
	// como fechar a janela, e não matando só o processo principal: matar deixava os filhos órfãos vivos, e eles
	// atrapalhavam o teste de foco do mesmo job do CI (run 42681823547 do #98).
	nav.mu.Lock()
	navCtx := nav.navCtx
	nav.mu.Unlock()
	if _, err := chromedp.CallBrowser(navCtx, browser.Close, cdp.Empty{}); err != nil && navCtx.Err() == nil {
		t.Fatal(err)
	}
	prazo := time.Now().Add(20 * time.Second)
	for nav.Ocupado() && time.Now().Before(prazo) {
		time.Sleep(100 * time.Millisecond)
	}
	if nav.Ocupado() {
		t.Fatal("depois de fechar a janela, o login continua aberto")
	}
	pag, err := nav.NovaPagina(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer pag.Fechar()
	nav.mu.Lock()
	visivel := nav.visivelAgora
	nav.mu.Unlock()
	if visivel {
		t.Fatal("depois do login, o envio abriu com janela")
	}
}

// P0 da .6.5: com "Ver o envio" ligado (janela visível), a candidatura abre a vaga e envia como no modo sem janela.
// Abre janela: SÓ NO CI.
func TestEnvioVisivelNavegaNaVaga(t *testing.T) {
	if os.Getenv("CI") != "true" {
		t.Skip("abre uma janela visível: só no CI")
	}
	srv := novoServidor(t)
	dados := t.TempDir()
	nav := navegadorDeTeste(t, srv, dados)
	nav.MostrarEnvio(true)
	s := novoCenario(t, nav, dados)
	d := s.enviar(s.aprovada("linkedin", "https://www.linkedin.com/jobs/view/9000000001"))
	if d.Estado != dominio.CandEnviada {
		t.Fatalf("visível: desfecho = %+v", d)
	}
	nav.mu.Lock()
	visivel := nav.visivelAgora
	nav.mu.Unlock()
	if !visivel {
		t.Fatal("o envio não abriu no modo visível")
	}
}
