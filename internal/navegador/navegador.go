package navegador

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"

	"github.com/FRF-Tecnology/job-control/internal/executor"
)

// Opcoes do navegador.
type Opcoes struct {
	PastaDados string // o perfil fica em <PastaDados>/navegador
	Visivel    bool   // janela visível (desenvolvimento); o padrão é sem janela
}

// Navegador abre abas num Chrome (ou Edge) só do app. Conexão com o DevTools por pipe (padrão do chromedp v0.21:
// --remote-debugging-pipe), sem porta de rede aberta.
type Navegador struct {
	exec    string
	perfil  string
	visivel bool

	// extrasDeTeste só é preenchido pelos testes deste pacote (resolução de host para o servidor local e certificado
	// de teste). O caminho de produção não tem como preencher.
	extrasDeTeste []chromedp.ExecAllocatorOption

	mu          sync.Mutex
	navCtx      context.Context
	fecharAlloc context.CancelFunc
	fecharNav   context.CancelFunc
}

var _ executor.Navegador = (*Navegador)(nil)

// Novo acha o executável (lista fixa) e prepara o perfil próprio. Não abre o navegador ainda.
func Novo(o Opcoes) (*Navegador, error) {
	if o.PastaDados == "" {
		return nil, errors.New("navegador: pasta de dados vazia")
	}
	exec, err := Localizar()
	if err != nil {
		return nil, err
	}
	perfil := PastaPerfil(o.PastaDados)
	if err := prepararPerfil(perfil); err != nil {
		return nil, err
	}
	return &Navegador{exec: exec, perfil: perfil, visivel: o.Visivel}, nil
}

// flags além das padrão do chromedp (que já trazem --no-first-run, --disable-sync, --disable-extensions,
// --password-store=basic e --disable-background-networking).
func (n *Navegador) opcoes() []chromedp.ExecAllocatorOption {
	o := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	if n.visivel {
		o = append(o, chromedp.VisibleWindow)
	}
	o = append(o,
		chromedp.ExecPath(n.exec),
		chromedp.UserDataDir(n.perfil),
		chromedp.Flag("disable-features", "Translate,BlinkGenPropertyTrees,PasswordManagerOnboarding,AutofillServerCommunication,MediaRouter,OptimizationHints"),
		chromedp.Flag("disable-component-update", true),
		chromedp.Flag("disable-component-extensions-with-background-pages", true),
		chromedp.Flag("disable-domain-reliability", true),
		chromedp.Flag("disable-notifications", true),
		chromedp.Flag("deny-permission-prompts", true),
		chromedp.Flag("disable-save-password-bubble", true),
		chromedp.Flag("no-pings", true),
		chromedp.Flag("no-service-autorun", true),
		// O padrão do chromedp desliga o bloqueio de popups; aqui ele volta (jc-a2y.6.2.8).
		chromedp.Flag("disable-popup-blocking", false),
	)
	return append(o, n.extrasDeTeste...)
}

// semConteudo (M7): o chromedp registra mensagens do navegador com o conteúdo (até páginas inteiras). Aqui não vai
// nada para o log além de "houve erro".
func semConteudo(string, ...any) {}
func soErro(string, ...any)      { log.Printf("navegador: erro interno do chromedp") }

func (n *Navegador) iniciar() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.navCtx != nil {
		return nil
	}
	// O perfil pode ter sido apagado ("sair das plataformas"): recria com as preferências antes de abrir.
	if err := prepararPerfil(n.perfil); err != nil {
		return err
	}
	alloc, fecharAlloc := chromedp.NewExecAllocator(context.Background(), n.opcoes()...)
	navCtx, fecharNav := chromedp.NewContext(alloc, chromedp.WithLogf(semConteudo), chromedp.WithDebugf(semConteudo), chromedp.WithErrorf(soErro))
	// Download negado no navegador inteiro.
	if _, err := chromedp.CallBrowser(navCtx, browser.SetDownloadBehavior, browser.SetDownloadBehaviorParams{Behavior: browser.SetDownloadBehaviorBehaviorDeny}); err != nil {
		fecharNav()
		fecharAlloc()
		return err
	}
	n.navCtx, n.fecharAlloc, n.fecharNav = navCtx, fecharAlloc, fecharNav
	return nil
}

// NovaPagina abre uma aba nova.
func (n *Navegador) NovaPagina(ctx context.Context) (executor.Pagina, error) {
	if err := n.iniciar(); err != nil {
		return nil, err
	}
	aba, fechar := chromedp.NewContext(n.navCtx)
	if err := chromedp.Do(aba); err != nil { // cria a aba (sem prazo: a aba vive o tempo do contexto)
		fechar()
		return nil, err
	}
	return &pagina{aba: aba, fechar: fechar}, nil
}

// Fechar fecha o navegador (todas as abas).
func (n *Navegador) Fechar() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.navCtx == nil {
		return
	}
	n.fecharNav()
	n.fecharAlloc()
	n.navCtx = nil
}
