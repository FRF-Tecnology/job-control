package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// O central-dev nunca traz o executor real, nem com -tags envio_dev (revisão do CyberSec no #61): ele escuta numa
// porta TCP e trata como a pessoa o pedido sem Sec-Fetch-Site.
func TestCentralDevNuncaTrazOExecutor(t *testing.T) {
	goBin := filepath.Join(runtime.GOROOT(), "bin", "go")
	for _, tags := range [][]string{nil, {"-tags", "envio_dev"}} {
		args := append([]string{"list"}, tags...)
		args = append(args, "-deps", ".")
		out, err := exec.Command(goBin, args...).CombinedOutput()
		if err != nil {
			t.Fatalf("go list %v: %v\n%s", tags, err, out)
		}
		for _, proibido := range []string{"chromedp", "internal/navegador", "internal/executor", "internal/envio"} {
			if strings.Contains(string(out), proibido) {
				t.Errorf("central-dev %v depende de %s", tags, proibido)
			}
		}
	}
}
