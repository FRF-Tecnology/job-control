package navegador

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	_ "image/jpeg" // só DecodeConfig do JPEG do screencast (largura e altura)
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// "Ao vivo" (jc-a2y.6.9, docs/propostas/ao-vivo.md): a aba do envio manda quadros pelo DevTools (CDP
// Page.startScreencast, pelo mesmo pipe, sem porta) só enquanto o observador quer. Só a aba do envio: AbrirLogin não
// passa por NovaPagina, então a janela de login nunca é observada. Só visualização: nada aqui clica nem digita.
// Os quadros passam direto para o observador, em memória; nada vai para disco, temp ou log.

// Observador recebe os quadros (internal/aovivo.AoVivo).
type Observador interface {
	// Querendo: o painel está aberto e há envio em andamento.
	Querendo() bool
	// Quadro: JPEG em base64, com largura e altura lidas do próprio JPEG.
	Quadro(base64 string, largura, altura int)
}

// Parâmetros do screencast (contrato): JPEG de qualidade baixa, até 1280×800, um quadro a cada 2 do navegador (o
// observador ainda descarta para no máximo 1 por segundo).
const (
	qualidadeQuadro = 40
	larguraQuadro   = 1280
	alturaQuadro    = 800
	cadaQuadros     = 2
)

// intervaloAoVivo: de quanto em quanto tempo a aba confere se o observador quer quadros (liga e desliga).
var intervaloAoVivo = time.Second

// assistir liga e desliga o screencast da aba conforme o observador, até a aba fechar.
func assistir(aba context.Context, obs Observador) {
	var ligado atomic.Bool
	quadros := chromedp.Events(aba, page.ScreencastFrame) // a inscrição começa aqui e acaba quando a aba fecha
	go func() {
		for f, err := range quadros {
			if err != nil {
				continue
			}
			_, _ = chromedp.Call(aba, page.ScreencastFrameAck, page.ScreencastFrameAckParams{SessionID: f.SessionID})
			if !ligado.Load() {
				continue // chegou depois de desligar: descartado
			}
			if largura, altura, ok := dimensoesJPEG(f.Data); ok {
				obs.Quadro(base64.StdEncoding.EncodeToString(f.Data), largura, altura)
			}
		}
	}()
	qualidade := int64(qualidadeQuadro)
	iniciar := page.StartScreencastParams{Format: page.StartScreencastFormatJpeg, Quality: &qualidade,
		MaxWidth: larguraQuadro, MaxHeight: alturaQuadro, EveryNthFrame: cadaQuadros}
	t := time.NewTicker(intervaloAoVivo)
	defer t.Stop()
	for {
		quer := obs.Querendo()
		switch {
		case quer && !ligado.Load():
			if _, err := chromedp.Call(aba, page.StartScreencast, iniciar); err == nil {
				ligado.Store(true)
			}
		case !quer && ligado.Load():
			ligado.Store(false)
			_, _ = chromedp.Call(aba, page.StopScreencast, cdp.Empty{})
		}
		select {
		case <-aba.Done():
			return
		case <-t.C:
		}
	}
}

// dimensoesJPEG lê largura e altura do cabeçalho do JPEG. Não é JPEG: false.
func dimensoesJPEG(b []byte) (int, int, bool) {
	cfg, formato, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || formato != "jpeg" {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}
