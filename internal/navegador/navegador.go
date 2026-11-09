package navegador

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"

	"github.com/FRF-Tecnology/job-control/internal/executor"
)

// Opcoes do navegador.
type Opcoes struct {
	PastaDados string // o perfil fica em <PastaDados>/navegador
	// AoVivo (jc-a2y.6.9): recebe os quadros da aba do envio enquanto quer. nil: nenhuma aba manda quadros.
	AoVivo Observador
}

// Erros de AbrirLogin.
var (
	ErrLoginAberto = errors.New("navegador: já há uma janela de login aberta")
	ErrEmUso       = errors.New("navegador: há uma página do envio aberta")
)

// Navegador abre abas num Chrome (ou Edge) só do app. Conexão com o DevTools por pipe (padrão do chromedp v0.21:
// --remote-debugging-pipe), sem porta de rede aberta.
//
// Segundo plano (jc-a2y.6.7): por padrão o navegador abre SEM JANELA (--headless=new), com prioridade baixa de
// CPU no Windows, e tudo vai pelo DevTools: nada usa o mouse, o teclado nem o foco do sistema. Janela visível só por
// ação da pessoa na tela: MostrarEnvio(true) ("Ver o envio", a partir da próxima página) e AbrirLogin ("Entrar nas
// plataformas", até a pessoa fechar a janela).
type Navegador struct {
	exec   string
	perfil string

	// extrasDeTeste só é preenchido pelos testes deste pacote (resolução de host para o servidor local e certificado
	// de teste). O caminho de produção não tem como preencher.
	extrasDeTeste []chromedp.ExecAllocatorOption

	mu          sync.Mutex
	navCtx      context.Context
	fecharAlloc context.CancelFunc
	fecharNav   context.CancelFunc

	querVisivel  bool            // "Ver o envio" ligado pela pessoa (começa false a cada abertura do app)
	visivelAgora bool            // o navegador aberto agora tem janela
	loginCtx     context.Context // navegador da janela de login da pessoa; enquanto vivo, nenhuma página do envio abre
	abertas      int             // páginas do envio abertas agora

	aoVivo Observador // "Ao vivo" (jc-a2y.6.9): só as abas do envio, nunca a janela de login
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
	return &Navegador{exec: exec, perfil: perfil, aoVivo: o.AoVivo}, nil
}

// flags além das padrão do chromedp (que já trazem --no-first-run, --disable-sync, --disable-extensions,
// --password-store=basic e --disable-background-networking).
func (n *Navegador) opcoes(visivel bool) []chromedp.ExecAllocatorOption {
	o := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	if visivel {
		o = append(o, chromedp.VisibleWindow)
	} else {
		o = append(o, chromedp.Flag("headless", "new")) // sem janela: nada aparece nem pega o foco
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
	o = append(o, opcoesDoSistema()...) // prioridade baixa de CPU (Windows)
	return append(o, n.extrasDeTeste...)
}

// semConteudo (M7): o chromedp registra mensagens do navegador com o conteúdo (até páginas inteiras). Aqui não vai
// nada para o log além de "houve erro".
func semConteudo(string, ...any) {}
func soErro(string, ...any)      { log.Printf("navegador: erro interno do chromedp") }

// iniciarSemTrava abre o navegador no modo pedido, se ainda não está aberto (ou se a pessoa fechou a janela).
// Chame com n.mu.
func (n *Navegador) iniciarSemTrava(visivel bool) error {
	if n.navCtx != nil && n.navCtx.Err() == nil {
		return nil
	}
	n.fecharSemTrava() // fechado por fora (a pessoa fechou a janela, o processo caiu): limpa antes de abrir de novo
	// O perfil pode ter sido apagado ("sair das plataformas"): recria com as preferências antes de abrir.
	if err := prepararPerfil(n.perfil); err != nil {
		return err
	}
	alloc, fecharAlloc := chromedp.NewExecAllocator(context.Background(), n.opcoes(visivel)...)
	navCtx, fecharNav := chromedp.NewContext(alloc, chromedp.WithLogf(semConteudo), chromedp.WithDebugf(semConteudo), chromedp.WithErrorf(soErro))
	// Download negado no navegador inteiro.
	if _, err := chromedp.CallBrowser(navCtx, browser.SetDownloadBehavior, browser.SetDownloadBehaviorParams{Behavior: browser.SetDownloadBehaviorBehaviorDeny}); err != nil {
		fecharNav()
		fecharAlloc()
		return err
	}
	n.navCtx, n.fecharAlloc, n.fecharNav, n.visivelAgora = navCtx, fecharAlloc, fecharNav, visivel
	return nil
}

// NovaPagina abre uma aba nova para o envio, no modo escolhido pela pessoa (sem janela, a não ser com "Ver o envio").
func (n *Navegador) NovaPagina(ctx context.Context) (executor.Pagina, error) {
	n.mu.Lock()
	if n.emLogin() {
		n.mu.Unlock()
		return nil, ErrLoginAberto
	}
	if n.navCtx != nil && n.navCtx.Err() == nil && n.abertas == 0 && n.visivelAgora != n.querVisivel {
		n.fecharSemTrava() // a pessoa trocou o modo: a próxima candidatura já abre no novo
	}
	if err := n.iniciarSemTrava(n.querVisivel); err != nil {
		n.mu.Unlock()
		return nil, err
	}
	navCtx := n.navCtx
	n.abertas++
	n.mu.Unlock()

	aba, fechar := chromedp.NewContext(navCtx)
	if err := chromedp.Do(aba); err != nil { // cria a aba (sem prazo: a aba vive o tempo do contexto)
		fechar()
		n.paginaFechada()
		return nil, err
	}
	if obs := n.aoVivo; obs != nil {
		go assistir(aba, obs) // termina quando a aba fecha
	}
	var uma sync.Once
	return &pagina{aba: aba, fechar: func() { fechar(); uma.Do(n.paginaFechada) }}, nil
}

// paginaFechada: a última página do envio fechou; se a pessoa trocou o modo enquanto isso, fecha para a próxima
// abrir no modo novo.
func (n *Navegador) paginaFechada() {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.abertas > 0 {
		n.abertas--
	}
	if n.abertas == 0 && !n.emLogin() && n.navCtx != nil && n.visivelAgora != n.querVisivel {
		n.fecharSemTrava()
	}
}

// MostrarEnvio liga ou desliga a janela do envio ("Ver o envio"). Vale a partir da próxima página: a que está
// aberta termina no modo em que começou.
func (n *Navegador) MostrarEnvio(visivel bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.querVisivel = visivel
	if n.navCtx != nil && n.abertas == 0 && !n.emLogin() && n.visivelAgora != visivel {
		n.fecharSemTrava()
	}
}

// Estado devolve se "Ver o envio" está ligado e se há uma janela de login aberta.
func (n *Navegador) Estado() (visivel, login bool) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.querVisivel, n.emLogin()
}

// emLogin: a janela de login está aberta (o navegador dela ainda vive). Chame com n.mu.
func (n *Navegador) emLogin() bool { return n.loginCtx != nil && n.loginCtx.Err() == nil }

// Ocupado: a pessoa está com a janela de login aberta. O executor não pega candidatura enquanto isso.
func (n *Navegador) Ocupado() bool {
	_, login := n.Estado()
	return login
}

// AbrirLogin abre o navegador numa janela visível, no endereço dado ("Entrar nas plataformas"). Fecha antes o
// navegador sem janela, se estiver aberto e sem página do envio. Até a pessoa fechar a janela, nenhuma página do
// envio abre; depois, o envio volta ao modo escolhido (sem janela, por padrão). Não espera a página carregar.
func (n *Navegador) AbrirLogin(endereco string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.emLogin() {
		return ErrLoginAberto
	}
	if n.abertas > 0 {
		return ErrEmUso
	}
	n.fecharSemTrava()
	if err := n.iniciarSemTrava(true); err != nil {
		return err
	}
	navCtx := n.navCtx
	n.loginCtx = navCtx
	go func() {
		// Usa a aba que o navegador já abriu (sem uma segunda aba em branco). Erro de carga não fecha a janela.
		ctx, cancelar := context.WithTimeout(navCtx, 2*time.Minute)
		defer cancelar()
		_, _ = chromedp.Run(ctx, chromedp.Navigate(endereco))
	}()
	go func() {
		<-navCtx.Done() // a pessoa fechou a janela (o navegador saiu) ou o app fechou o navegador
		n.mu.Lock()
		defer n.mu.Unlock()
		if n.navCtx == navCtx {
			n.fecharSemTrava()
		}
		if n.loginCtx == navCtx { // um login novo, aberto depois, não é desfeito
			n.loginCtx = nil
		}
	}()
	return nil
}

// Fechar fecha o navegador (todas as abas, também a janela de login).
func (n *Navegador) Fechar() {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.fecharSemTrava()
	n.loginCtx = nil
}

func (n *Navegador) fecharSemTrava() {
	if n.navCtx == nil {
		return
	}
	n.fecharNav()
	n.fecharAlloc()
	n.navCtx = nil
}
