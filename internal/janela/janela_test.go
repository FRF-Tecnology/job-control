package janela

import "testing"

func TestInicial(t *testing.T) {
	padrao := Tamanho{Largura: LarguraPadrao, Altura: AlturaPadrao}
	casos := []struct {
		nome     string
		guardado *Tamanho
		quer     Tamanho
	}{
		{"primeira vez", nil, padrao},
		{"o que a pessoa deixou", &Tamanho{Largura: 1000, Altura: 700}, Tamanho{Largura: 1000, Altura: 700}},
		{"maximizada lembra o tamanho solto", &Tamanho{Largura: 900, Altura: 640, Maximizada: true}, Tamanho{Largura: 900, Altura: 640, Maximizada: true}},
		{"no mínimo", &Tamanho{Largura: LarguraMinima, Altura: AlturaMinima}, Tamanho{Largura: LarguraMinima, Altura: AlturaMinima}},
		{"estreita demais volta ao padrão", &Tamanho{Largura: 100, Altura: 700}, padrao},
		{"baixa demais volta ao padrão", &Tamanho{Largura: 1000, Altura: 10}, padrao},
		{"zerada volta ao padrão", &Tamanho{}, padrao},
		{"negativa volta ao padrão", &Tamanho{Largura: -5, Altura: -5}, padrao},
		{"gigante volta ao padrão", &Tamanho{Largura: 99999, Altura: 700}, padrao},
		{"tamanho estragado, mas maximizada continua", &Tamanho{Largura: 1, Altura: 1, Maximizada: true}, Tamanho{Largura: LarguraPadrao, Altura: AlturaPadrao, Maximizada: true}},
	}
	for _, c := range casos {
		if got := Inicial(c.guardado); got != c.quer {
			t.Errorf("%s: %+v; queria %+v", c.nome, got, c.quer)
		}
	}
}

func TestCaber(t *testing.T) {
	casos := []struct {
		nome               string
		l, a, areaL, areaA int
		querL, querA       int
		querMudou          bool
	}{
		{"cabe", 1280, 800, 1920, 1040, 1280, 800, false},
		{"igual à área", 1280, 800, 1280, 800, 1280, 800, false},
		{"notebook 1366x768 com barra", 1280, 800, 1366, 728, 1280, 728, true},
		{"tela pequena nos dois lados", 1280, 800, 1024, 600, 1024, 600, true},
		{"nunca abaixo do mínimo", 1280, 800, 300, 400, LarguraMinima, AlturaMinima, true},
		{"área desconhecida", 1280, 800, 0, 0, 1280, 800, false},
		{"área negativa", 1280, 800, -1, -1, 1280, 800, false},
	}
	for _, c := range casos {
		l, a, mudou := Caber(c.l, c.a, c.areaL, c.areaA)
		if l != c.querL || a != c.querA || mudou != c.querMudou {
			t.Errorf("%s: %dx%d mudou=%v; queria %dx%d mudou=%v", c.nome, l, a, mudou, c.querL, c.querA, c.querMudou)
		}
	}
}

func TestLembrar(t *testing.T) {
	antes := Tamanho{Largura: 1000, Altura: 700}
	if got := Lembrar(antes, 1100, 760, false); got != (Tamanho{Largura: 1100, Altura: 760}) {
		t.Errorf("mudou de tamanho: %+v", got)
	}
	// Maximizada: o tamanho lido é o da tela inteira; fica o da janela solta.
	if got := Lembrar(antes, 1920, 1040, true); got != (Tamanho{Largura: 1000, Altura: 700, Maximizada: true}) {
		t.Errorf("maximizou: %+v", got)
	}
	if got := Lembrar(Tamanho{Largura: 1000, Altura: 700, Maximizada: true}, 1000, 700, false); got != antes {
		t.Errorf("desmaximizou: %+v", got)
	}
	// Leitura estranha (janela minimizada costuma dar 0x0 ou 160x28): não troca o que estava guardado.
	for _, lido := range [][2]int{{0, 0}, {160, 28}, {-32000, -32000}, {99999, 700}} {
		if got := Lembrar(antes, lido[0], lido[1], false); got != antes {
			t.Errorf("leitura %v trocou o guardado: %+v", lido, got)
		}
	}
}
