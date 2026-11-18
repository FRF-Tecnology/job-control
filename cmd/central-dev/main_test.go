package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// O central-dev nunca traz o executor real, (revisão do CyberSec no #61): ele escuta numa
// porta TCP e trata como a pessoa o pedido sem Sec-Fetch-Site.
func TestCentralDevNuncaTrazOExecutor(t *testing.T) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	out, err := exec.Command(goBin, "list", "-deps", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	for _, proibido := range []string{"chromedp", "internal/navegador", "internal/executor", "internal/envio"} {
		if strings.Contains(string(out), proibido) {
			t.Errorf("central-dev depende de %s", proibido)
		}
	}
}
