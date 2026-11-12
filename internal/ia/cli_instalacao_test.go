package ia

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestClaudeInstalacaoNativaForaDoPath(t *testing.T) {
	casa := t.TempDir()
	t.Setenv("HOME", casa)
	t.Setenv("USERPROFILE", casa)
	t.Setenv("PATH", t.TempDir())
	dir := filepath.Join(casa, ".local", "bin")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	n := "claude"
	if runtime.GOOS == "windows" {
		n += ".exe"
	}
	alvo := filepath.Join(dir, n)
	if err := os.WriteFile(alvo, []byte("teste de localização, nunca executado"), 0700); err != nil {
		t.Fatal(err)
	}
	if p, err := localizarCLI("claude"); err != nil || p != alvo {
		t.Fatalf("não encontrou instalação nativa: %v", err)
	}
}
