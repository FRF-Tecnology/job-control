package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/api"
	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/janela"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// O ícone da bandeja sai do ícone do app, reduzido na partida (janela.IconePequeno): trocou um, trocou o outro.
//
//go:embed build/appicon.png
var iconeDoApp []byte

// esperaParaGravar: a pessoa arrasta a borda e a janela muda de tamanho dezenas de vezes; grava-se uma vez, no fim.
const esperaParaGravar = 500 * time.Millisecond

// controleJanela cuida da janela do app: abre no tamanho que a pessoa deixou (ou no padrão), lembra o ajuste e,
// se a pessoa pediu, mantém o app aberto em segundo plano quando ela fecha a janela (ícone na bandeja).
// As regras ficam em internal/janela; aqui só se fala com o Wails.
type controleJanela struct {
	app   *application.App
	w     *application.WebviewWindow
	banco *armazem.Armazem

	mu      sync.Mutex // tamanho, gravado, espera e calado
	tamanho janela.Tamanho
	gravado janela.Tamanho
	espera  *time.Timer
	// calado: depois de "apagar meus dados" nada mais é gravado (o banco já foi apagado).
	calado bool

	muBandeja sync.Mutex
	bandeja   *application.SystemTray

	segundoPlano atomic.Bool
	// saindo: o app está fechando de verdade; fechar a janela não pode mais só escondê-la.
	saindo atomic.Bool
	coube  atomic.Bool
}

// lerJanela lê do banco o que a pessoa deixou: o tamanho da janela e a escolha do segundo plano. Sem registro ou com
// registro estragado, vale o padrão (janela.Inicial) e o segundo plano desligado.
func lerJanela(ctx context.Context, banco *armazem.Armazem) (janela.Tamanho, bool) {
	var guardado *janela.Tamanho
	var lido janela.Tamanho
	if tem, err := banco.LerMeta(ctx, janela.ChaveTamanho, &lido); err != nil {
		log.Printf("janela: não leu o tamanho guardado (%T)", err)
	} else if tem {
		guardado = &lido
	}
	var segundoPlano bool
	if _, err := banco.LerMeta(ctx, janela.ChaveSegundoPlano, &segundoPlano); err != nil {
		log.Printf("janela: não leu a escolha do segundo plano (%T)", err)
		segundoPlano = false
	}
	return janela.Inicial(guardado), segundoPlano
}

// abrirJanela cria a janela do app e liga o que a mantém: lembrar o tamanho, caber na tela e o segundo plano.
// Chamar antes de app.Run.
func abrirJanela(app *application.App, banco *armazem.Armazem, inicial janela.Tamanho, segundoPlano bool) *controleJanela {
	c := &controleJanela{app: app, banco: banco, tamanho: inicial, gravado: inicial}
	opcoes := application.WebviewWindowOptions{
		Title:           "Applyer",
		Width:           inicial.Largura,
		Height:          inicial.Altura,
		MinWidth:        janela.LarguraMinima,
		MinHeight:       janela.AlturaMinima,
		InitialPosition: application.WindowCentered,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		URL: "/",
	}
	if inicial.Maximizada {
		opcoes.StartState = application.WindowStateMaximised
	}
	c.w = app.Window.NewWithOptions(opcoes)

	// Fechar a janela com o segundo plano ligado só a esconde. O gancho roda antes de o Wails destruir a janela.
	c.w.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if c.segundoPlano.Load() && !c.saindo.Load() {
			c.w.Hide()
			e.Cancel()
		}
	})
	for _, evento := range []events.WindowEventType{
		events.Common.WindowDidResize, events.Common.WindowMaximise, events.Common.WindowUnMaximise, events.Common.WindowRestore,
	} {
		c.w.OnWindowEvent(evento, func(*application.WindowEvent) { c.registrar() })
	}
	// A tela pode ser menor que o tamanho guardado (outro monitor, notebook): encolhe uma vez, quando a janela existe.
	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) { c.caberNaTela() })
	c.w.OnWindowEvent(events.Common.WindowShow, func(*application.WindowEvent) { c.caberNaTela() })
	// Ao sair: o app fecha de verdade e o último tamanho vai para o banco (o banco só fecha depois do Run).
	app.OnShutdown(c.aoSair)

	if segundoPlano {
		c.definirSegundoPlano(true)
	}
	return c
}

// ganchos: o que a API chama quando a pessoa mexe em "Janela do app", no Perfil.
func (c *controleJanela) ganchos() *api.JanelaApp {
	return &api.JanelaApp{SegundoPlano: c.definirSegundoPlano, TamanhoPadrao: c.voltarAoPadrao}
}

// mostrar traz a janela de volta: da bandeja, minimizada ou atrás de outras. É o que acontece quando a pessoa abre
// o app de novo com ele já aberto.
func (c *controleJanela) mostrar() {
	if c.saindo.Load() {
		return
	}
	c.w.Show()
	c.w.UnMinimise()
	c.w.Focus()
}

// sair fecha o app de verdade, mesmo com o segundo plano ligado.
func (c *controleJanela) sair() {
	c.saindo.Store(true)
	c.app.Quit()
}

// calar: depois de "apagar meus dados", nada mais vai para o banco.
func (c *controleJanela) calar() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calado = true
	if c.espera != nil {
		c.espera.Stop()
	}
}

// registrar lê o estado da janela e marca para gravar. Minimizada, em tela cheia ou fechando, não registra nada.
func (c *controleJanela) registrar() {
	if c.saindo.Load() || c.w.IsMinimised() || c.w.IsFullscreen() {
		return
	}
	maximizada := c.w.IsMaximised()
	largura, altura := c.w.Size()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tamanho = janela.Lembrar(c.tamanho, largura, altura, maximizada)
	if c.calado || c.tamanho == c.gravado {
		return
	}
	if c.espera != nil {
		c.espera.Stop()
	}
	c.espera = time.AfterFunc(esperaParaGravar, c.gravar)
}

// gravar leva o tamanho lembrado para o banco, se mudou.
func (c *controleJanela) gravar() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.calado || c.tamanho == c.gravado {
		return
	}
	if err := c.banco.GravarMeta(context.Background(), janela.ChaveTamanho, c.tamanho); err != nil {
		log.Printf("janela: não gravou o tamanho (%T)", err)
		return
	}
	c.gravado = c.tamanho
}

// aoSair roda no encerramento do app, antes de as janelas fecharem.
func (c *controleJanela) aoSair() {
	c.saindo.Store(true)
	c.mu.Lock()
	if c.espera != nil {
		c.espera.Stop()
	}
	c.mu.Unlock()
	c.gravar()
}

// caberNaTela encolhe a janela para a área útil da tela, uma vez só. Maximizada já ocupa a tela.
func (c *controleJanela) caberNaTela() {
	if c.coube.Load() {
		return
	}
	tela, err := c.w.GetScreen()
	if err != nil || tela == nil {
		return // a janela ainda não existe; o próximo evento tenta de novo
	}
	if !c.coube.CompareAndSwap(false, true) || c.w.IsMaximised() {
		return
	}
	largura, altura := c.w.Size()
	if l, a, mudou := janela.Caber(largura, altura, tela.WorkArea.Width, tela.WorkArea.Height); mudou {
		c.w.SetSize(l, a)
		c.w.Center()
	}
}

// voltarAoPadrao: o botão "Voltar ao tamanho padrão". Sai do maximizado, aplica o padrão (ou o que couber na tela)
// e centraliza.
func (c *controleJanela) voltarAoPadrao() {
	c.w.UnMaximise()
	largura, altura := janela.LarguraPadrao, janela.AlturaPadrao
	if tela, err := c.w.GetScreen(); err == nil && tela != nil {
		largura, altura, _ = janela.Caber(largura, altura, tela.WorkArea.Width, tela.WorkArea.Height)
	}
	c.w.SetSize(largura, altura)
	c.w.Center()
	c.registrar()
}

// definirSegundoPlano liga ou desliga "continuar aberto em segundo plano". Ligado, o app ganha um ícone na bandeja
// (perto do relógio no Windows, na barra de menus no Mac); desligado, o ícone some e fechar a janela fecha o app.
// Devolve se ficou como pedido.
func (c *controleJanela) definirSegundoPlano(ligar bool) bool {
	c.muBandeja.Lock()
	defer c.muBandeja.Unlock()
	if !ligar {
		c.segundoPlano.Store(false)
		if c.bandeja != nil {
			c.bandeja.Hide()
		}
		return true
	}
	if c.bandeja != nil {
		c.bandeja.Show()
	} else if c.bandeja = c.criarBandeja(); c.bandeja == nil {
		// Sem ícone, a janela escondida ficaria sem caminho de volta: o segundo plano não liga.
		return false
	}
	c.segundoPlano.Store(true)
	return true
}

// criarBandeja monta o ícone da bandeja: um clique abre o app; o menu tem "Abrir o Applyer" e "Sair".
func (c *controleJanela) criarBandeja() *application.SystemTray {
	icone, err := janela.IconePequeno(iconeDoApp, janela.LadoIconeBandeja)
	if err != nil {
		log.Printf("janela: não montou o ícone da bandeja (%v)", err)
		return nil
	}
	menu := c.app.NewMenu()
	menu.Add("Abrir o Applyer").OnClick(func(*application.Context) { c.mostrar() })
	menu.AddSeparator()
	menu.Add("Sair").OnClick(func(*application.Context) { c.sair() })
	bandeja := c.app.SystemTray.New()
	bandeja.SetIcon(icone)
	bandeja.SetTooltip("Applyer")
	bandeja.SetMenu(menu)
	bandeja.OnClick(c.mostrar)
	return bandeja
}

// idDaInstancia identifica o app aberto sobre uma pasta de dados: abrir o app de novo na mesma pasta só traz a
// janela de volta; outra pasta (APPLYER_PASTA) é outro app. Vai só o resumo do caminho, nunca o caminho.
func idDaInstancia(pasta string) string {
	soma := sha256.Sum256([]byte(pasta))
	return "dev.applyer.app." + hex.EncodeToString(soma[:8])
}

// apiTardia entrega os pedidos à API local, que só fica pronta depois de o app existir: o app é criado primeiro
// (para a segunda abertura sair antes de tocar no banco) e a API, que depende do banco, depois.
type apiTardia struct{ h atomic.Pointer[http.Handler] }

func (a *apiTardia) definir(h http.Handler) { a.h.Store(&h) }

func (a *apiTardia) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h := a.h.Load(); h != nil {
		(*h).ServeHTTP(w, r)
		return
	}
	http.Error(w, `{"erro":"o app ainda está abrindo"}`, http.StatusServiceUnavailable)
}
