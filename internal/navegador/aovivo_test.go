package navegador

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// "Ao vivo" (jc-a2y.6.9). Só dados fictícios; o Chrome roda sem janela.

func TestDimensoesJPEG(t *testing.T) {
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 320, 200)), nil); err != nil {
		t.Fatal(err)
	}
	if l, a, ok := dimensoesJPEG(b.Bytes()); !ok || l != 320 || a != 200 {
		t.Fatalf("jpeg = %d×%d %v", l, a, ok)
	}
	for _, ruim := range []string{"", "<html>", base64.StdEncoding.EncodeToString(b.Bytes())} {
		if _, _, ok := dimensoesJPEG([]byte(ruim)); ok {
			t.Errorf("%q passou como JPEG", ruim[:min(len(ruim), 10)])
		}
	}
}

// observadorTeste: liga e desliga por fora, e guarda quantos quadros chegaram.
type observadorTeste struct {
	quer    atomic.Bool
	mu      sync.Mutex
	quadros int
	maior   [2]int
}

func (o *observadorTeste) Querendo() bool { return o.quer.Load() }
func (o *observadorTeste) Quadro(_ string, l, a int) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.quadros++
	if l > o.maior[0] {
		o.maior[0] = l
	}
	if a > o.maior[1] {
		o.maior[1] = a
	}
}
func (o *observadorTeste) contar() int { o.mu.Lock(); defer o.mu.Unlock(); return o.quadros }

func esperarAte(t *testing.T, limite time.Duration, cond func() bool) bool {
	t.Helper()
	fim := time.Now().Add(limite)
	for time.Now().Before(fim) {
		if cond() {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return cond()
}

// Com o Chrome de verdade, sem janela: a aba do envio só manda quadros enquanto o observador quer, para de mandar
// ao desligar, e nenhum quadro vai para a pasta de dados nem para o temp.
func TestAoVivoComChrome(t *testing.T) {
	temp := pastaQueEsperaOChrome(t)
	for _, v := range []string{"TMP", "TEMP", "TMPDIR"} {
		t.Setenv(v, temp) // o Chrome herda: tudo o que ele puser no temp fica aqui, para conferir
	}
	antes := intervaloAoVivo
	intervaloAoVivo = 200 * time.Millisecond
	t.Cleanup(func() { intervaloAoVivo = antes })

	srv := novoServidor(t)
	dados := pastaQueEsperaOChrome(t)
	n := navegadorDeTeste(t, srv, dados)
	obs := &observadorTeste{}
	n.aoVivo = obs
	inicio := time.Now()

	pag, err := n.NovaPagina(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer pag.Fechar()
	if _, err := pag.Abrir(context.Background(), "https://www.linkedin.com/jobs/view/9000000001"); err != nil {
		t.Fatal(err)
	}
	// Sem querer: nenhum quadro.
	time.Sleep(time.Second)
	if q := obs.contar(); q != 0 {
		t.Fatalf("%d quadros sem o painel", q)
	}
	// Ligado: chegam quadros, dentro do tamanho máximo. Mexer na página força quadros novos.
	obs.quer.Store(true)
	cor := 0
	mexer := func() { // troca a cor de fundo: o navegador desenha de novo e manda quadro
		cor = (cor + 0x123457) % 0xFFFFFF
		rodar(context.Background(), pag.(*pagina), tempoAcao, chromedp.Evaluate[int](fmt.Sprintf(`document.body.style.background='#%06x';1`, cor)))
	}
	if !esperarAte(t, 15*time.Second, func() bool { mexer(); return obs.contar() > 0 }) {
		t.Fatal("nenhum quadro com o painel aberto")
	}
	obs.mu.Lock()
	if obs.maior[0] > larguraQuadro || obs.maior[1] > alturaQuadro {
		t.Errorf("quadro %v passou de %d×%d", obs.maior, larguraQuadro, alturaQuadro)
	}
	obs.mu.Unlock()
	// Desligado: para de mandar (o laço confere a cada 200 ms aqui).
	obs.quer.Store(false)
	time.Sleep(time.Second)
	parou := obs.contar()
	for i := 0; i < 10; i++ {
		mexer()
		time.Sleep(200 * time.Millisecond)
	}
	if q := obs.contar(); q != parou {
		t.Fatalf("depois de desligar chegaram %d quadros", q-parou)
	}
	// Nenhum JPEG novo na pasta de dados nem no temp.
	for _, pasta := range []string{dados, temp} {
		filepath.WalkDir(pasta, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if info, err := d.Info(); err != nil || info.ModTime().Before(inicio) {
				return nil
			}
			f, err := os.Open(p)
			if err != nil {
				return nil
			}
			defer f.Close()
			cab := make([]byte, 3)
			if k, _ := f.Read(cab); k == 3 && cab[0] == 0xFF && cab[1] == 0xD8 && cab[2] == 0xFF {
				t.Errorf("JPEG gravado em disco: %s", filepath.Base(p))
			}
			return nil
		})
	}
}

// pastaQueEsperaOChrome: pasta temporária que, no fim do teste, espera até 15 s o Chrome soltar os arquivos. No
// Windows, um processo filho do Chrome (cache, GPU) às vezes ainda segura um arquivo logo depois de o navegador
// fechar, e o RemoveAll do t.TempDir falharia o teste por isso. O que o teste confere não muda.
func pastaQueEsperaOChrome(t *testing.T) string {
	t.Helper()
	p, err := os.MkdirTemp("", "aovivo-teste-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		fim := time.Now().Add(15 * time.Second)
		for {
			err := os.RemoveAll(p)
			if err == nil {
				return
			}
			if time.Now().After(fim) {
				t.Errorf("a pasta temporária do teste não saiu: %v", err)
				return
			}
			time.Sleep(300 * time.Millisecond)
		}
	})
	return p
}
