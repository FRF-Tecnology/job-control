package janela

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
)

// LadoIconeBandeja é o lado, em pixels, do ícone que vai para perto do relógio (Windows) ou para a barra de menus
// (Mac). O sistema ainda ajusta ao tamanho dele; 64 sobra para telas de alta densidade.
const LadoIconeBandeja = 64

// IconePequeno reduz um PNG (o ícone do app) para lado x lado pixels, pela média de cada bloco. Assim o ícone da
// bandeja sai do mesmo arquivo do ícone do app e muda junto com ele. O PNG grande não serve direto: o Windows recusa
// ícone acima de 256 pixels.
func IconePequeno(origem []byte, lado int) ([]byte, error) {
	if lado < 1 {
		return nil, errors.New("lado do ícone inválido")
	}
	img, err := png.Decode(bytes.NewReader(origem))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	if b.Dx() < lado || b.Dy() < lado {
		return nil, errors.New("ícone de origem menor que o pedido")
	}
	saida := image.NewNRGBA(image.Rect(0, 0, lado, lado))
	for y := 0; y < lado; y++ {
		y0, y1 := b.Min.Y+y*b.Dy()/lado, b.Min.Y+(y+1)*b.Dy()/lado
		for x := 0; x < lado; x++ {
			x0, x1 := b.Min.X+x*b.Dx()/lado, b.Min.X+(x+1)*b.Dx()/lado
			saida.SetNRGBA(x, y, mediaDoBloco(img, x0, y0, x1, y1))
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, saida); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// mediaDoBloco: a cor média, com o alfa como peso (ponto transparente não escurece a borda).
func mediaDoBloco(img image.Image, x0, y0, x1, y1 int) color.NRGBA {
	var r, g, b, a, n uint64
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			c := color.NRGBA64Model.Convert(img.At(x, y)).(color.NRGBA64)
			r += uint64(c.R) * uint64(c.A)
			g += uint64(c.G) * uint64(c.A)
			b += uint64(c.B) * uint64(c.A)
			a += uint64(c.A)
			n++
		}
	}
	if n == 0 || a == 0 {
		return color.NRGBA{}
	}
	return color.NRGBA{R: uint8((r / a) >> 8), G: uint8((g / a) >> 8), B: uint8((b / a) >> 8), A: uint8((a / n) >> 8)}
}
