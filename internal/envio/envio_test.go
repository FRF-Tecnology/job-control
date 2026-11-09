//go:build !envio_dev

package envio

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// No build padrão o envio está desligado: Ligar não faz nada e o binário não traz o gatilho, nem o executor, nem o
// navegador. Prova com um binário mínimo (testdata/sonda) compilado das duas formas e lido com go tool nm.
func TestBuildPadraoSemGatilho(t *testing.T) {
	if Ligado {
		t.Fatal("build padrão com o envio ligado")
	}
	c := Ligar(context.Background(), nil, "") // não faz nada, nem com armazém nil
	c.Suspender()()
	c.FecharNavegador()
	c.Parar()

	if testing.Short() {
		t.Skip("compilar a sonda: fora do -short")
	}
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	dir := t.TempDir()
	simbolos := func(tags ...string) string {
		t.Helper()
		bin := filepath.Join(dir, "sonda"+strings.Join(tags, "-")+".exe")
		args := append([]string{"build"}, tags...)
		args = append(args, "-o", bin, "./testdata/sonda")
		if out, err := exec.Command(goBin, args...).CombinedOutput(); err != nil {
			t.Fatalf("go build %v: %v\n%s", tags, err, out)
		}
		out, err := exec.Command(goBin, "tool", "nm", bin).CombinedOutput()
		if err != nil {
			t.Fatalf("go tool nm: %v\n%s", err, out)
		}
		return string(out)
	}
	marcas := []string{"ligarExecutorDeDesenvolvimento", "internal/executor.", "internal/navegador.", "chromedp"}

	padrao := simbolos()
	for _, m := range marcas {
		if strings.Contains(padrao, m) {
			t.Errorf("o binário padrão traz %q", m)
		}
	}
	// Controle: com a tag, o gatilho está lá (o teste sabe achar o símbolo).
	dev := simbolos("-tags", "envio_dev")
	for _, m := range marcas {
		if !strings.Contains(dev, m) {
			t.Errorf("o binário envio_dev não traz %q: o teste não está vendo o símbolo", m)
		}
	}
}
