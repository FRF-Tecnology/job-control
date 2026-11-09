//go:build windows

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// No Windows, o TEMP e alguns erros usam o nome curto (C:\Users\NOME~1). Ele também é ocultado. Usa a pasta
// do usuário de verdade: as pastas novas não ganham nome curto quando o volume desliga o 8.3, mas a do usuário,
// criada com o Windows, costuma ter.
func TestOcultarCaminhosFormaCurtaDoWindows(t *testing.T) {
	t.Setenv("CENTRAL_PASTA", "")
	casa, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	curta := formaCurta(casa)
	if curta == casa || !strings.Contains(curta, "~") {
		t.Skipf("a pasta do usuário não tem nome curto neste volume")
	}
	got := OcultarCaminhos("abrir " + filepath.Join(curta, "AppData", "central.db") + ": acesso negado")
	if strings.Contains(got, curta) || !strings.Contains(got, "~"+string(filepath.Separator)+"AppData") {
		t.Errorf("= %q", got)
	}
}
