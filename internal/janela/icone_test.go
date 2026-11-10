package janela

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func pngDeTeste(t *testing.T, lado int, cor func(x, y int) color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, lado, lado))
	for y := 0; y < lado; y++ {
		for x := 0; x < lado; x++ {
			img.SetNRGBA(x, y, cor(x, y))
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestIconePequeno(t *testing.T) {
	preto, branco := color.NRGBA{A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	// Metade esquerda preta, metade direita branca; o canto de cima à esquerda transparente.
	origem := pngDeTeste(t, 256, func(x, y int) color.NRGBA {
		switch {
		case x < 64 && y < 64:
			return color.NRGBA{}
		case x < 128:
			return preto
		}
		return branco
	})
	pequeno, err := IconePequeno(origem, LadoIconeBandeja)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(pequeno))
	if err != nil {
		t.Fatalf("a saída não é PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != LadoIconeBandeja || b.Dy() != LadoIconeBandeja {
		t.Fatalf("tamanho %v", b)
	}
	em := func(x, y int) color.NRGBA { return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA) }
	if c := em(20, 40); c != preto {
		t.Errorf("esquerda: %v", c)
	}
	if c := em(50, 40); c != branco {
		t.Errorf("direita: %v", c)
	}
	if c := em(5, 5); c.A != 0 {
		t.Errorf("canto transparente ficou com alfa %d", c.A)
	}
}

func TestIconePequenoRecusaEntradaRuim(t *testing.T) {
	if _, err := IconePequeno([]byte("não é png"), 64); err == nil {
		t.Error("aceitou o que não é PNG")
	}
	menor := pngDeTeste(t, 32, func(int, int) color.NRGBA { return color.NRGBA{A: 255} })
	if _, err := IconePequeno(menor, 64); err == nil {
		t.Error("aceitou origem menor que o pedido")
	}
	if _, err := IconePequeno(menor, 0); err == nil {
		t.Error("aceitou lado zero")
	}
}
