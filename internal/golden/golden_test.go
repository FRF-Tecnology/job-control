package golden

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// Ler e gravar sem mudar nada devolve o arquivo byte a byte: a regravação só mexe no que o teste trocou.
func TestLerEGravarDevolveOMesmoArquivo(t *testing.T) {
	for _, caminho := range []string{
		"../dominio/testdata/golden.json",
		"../ia/testdata/golden-ia.json",
		"../api/testdata/golden-api.json",
	} {
		t.Run(filepath.Base(caminho), func(t *testing.T) {
			antes, err := os.ReadFile(caminho)
			if err != nil {
				t.Fatal(err)
			}
			saida := filepath.Join(t.TempDir(), "g.json")
			Gravar(t, saida, Ler(t, caminho))
			depois, _ := os.ReadFile(saida)
			if !bytes.Equal(antes, depois) {
				i := 0
				for i < len(antes) && i < len(depois) && antes[i] == depois[i] {
					i++
				}
				ini := max(0, i-80)
				t.Fatalf("difere no byte %d\nantes:  %q\ndepois: %q", i, antes[ini:min(len(antes), i+80)], depois[ini:min(len(depois), i+80)])
			}
		})
	}
}

func TestSetMantemAOrdem(t *testing.T) {
	o := NovoObj("a", 1, "b", 2, "c", 3)
	o.Set("b", 9)
	o.Set("d", 4)
	o.Del("a")
	var buf bytes.Buffer
	escrever(&buf, Valor(map[string]any{"x": 1}), 0)
	if got := o.Chaves; len(got) != 3 || got[0] != "b" || got[1] != "c" || got[2] != "d" || o.Get("b") != 9 {
		t.Fatalf("chaves = %v", got)
	}
	if buf.String() != "{\n \"x\": 1\n}" {
		t.Fatalf("escrita = %q", buf.String())
	}
}
