package envio

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// jc-a2y.6.7: o envio usa só o DevTools (CDP). Nenhum código do app mexe no mouse, no teclado ou no foco do
// sistema, nem traz biblioteca que faça isso. Varre os .go de produção do repositório e o go.mod/go.sum.
var proibidosNoCodigo = []string{
	"SendInput", "keybd_event", "mouse_event", "SetCursorPos", "SetForegroundWindow", "BlockInput",
	"AttachThreadInput", "SetActiveWindow", "BringWindowToTop", "SwitchToThisWindow", "robotgo",
}

var proibidosNosModulos = []string{"robotgo", "go-vgo", "keybd_event", "micmonay", "go-hook", "gohook"}

func TestNadaUsaMouseTecladoOuFoco(t *testing.T) {
	raiz, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(raiz, "go.mod")); err != nil {
		t.Fatalf("raiz do repositório não achada: %v", err)
	}
	vistos := 0
	err = filepath.WalkDir(raiz, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "node_modules", "graphify-out", ".git", "dados", "frontend":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		vistos++
		for _, x := range proibidosNoCodigo {
			if strings.Contains(string(b), x) {
				t.Errorf("%s usa %s", strings.TrimPrefix(p, raiz), x)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if vistos < 50 {
		t.Fatalf("só %d arquivos .go vistos: a varredura não está olhando o repositório", vistos)
	}
	for _, arq := range []string{"go.mod", "go.sum"} {
		b, err := os.ReadFile(filepath.Join(raiz, arq))
		if err != nil {
			t.Fatal(err)
		}
		for _, x := range proibidosNosModulos {
			if strings.Contains(string(b), x) {
				t.Errorf("%s traz %s", arq, x)
			}
		}
	}
}
