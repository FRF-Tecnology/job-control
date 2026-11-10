//go:build windows

package ia

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/windows"
)

// Toda CLI roda sem janela de console: o app é um programa de janela, e sem isso o Windows abre um terminal a cada
// chamada à IA. O processo filho aqui é o próprio binário de teste, que diz se tem uma janela de console visível.
const ajudanteJanela = "JC_AJUDANTE_JANELA"

func TestAjudanteJanela(t *testing.T) {
	if os.Getenv(ajudanteJanela) != "1" {
		t.Skip("só roda como processo filho de TestCLISemJanela")
	}
	visivel := uintptr(0)
	if janela, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetConsoleWindow").Call(); janela != 0 {
		visivel, _, _ = syscall.NewLazyDLL("user32.dll").NewProc("IsWindowVisible").Call(janela)
	}
	fmt.Printf("janela-visivel=%d\n", visivel)
}

func TestCLISemJanela(t *testing.T) {
	cmd := comandoCLI(context.Background(), os.Args[0], "-test.run=^TestAjudanteJanela$", "-test.v")
	if a := cmd.SysProcAttr; a == nil || !a.HideWindow || a.CreationFlags&windows.CREATE_NO_WINDOW == 0 {
		t.Fatalf("a CLI tem de rodar com HideWindow e CREATE_NO_WINDOW; veio %+v", cmd.SysProcAttr)
	}
	cmd.Env = append(os.Environ(), ajudanteJanela+"=1")
	saida, err := cmd.Output()
	if err != nil {
		t.Fatalf("o processo filho não rodou: %v", err)
	}
	if !strings.Contains(string(saida), "janela-visivel=0") {
		t.Fatalf("a CLI rodou com janela de console visível:\n%s", saida)
	}
}

// Nenhuma chamada a CLI em cli.go escapa de comandoCLI.
func TestTodaCLIPassaPorComandoCLI(t *testing.T) {
	fonte, err := os.ReadFile("cli.go")
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(fonte), "exec.Command"); n != 1 {
		t.Fatalf("cli.go tem %d usos de exec.Command; só comandoCLI pode criar o processo de uma CLI", n)
	}
}
