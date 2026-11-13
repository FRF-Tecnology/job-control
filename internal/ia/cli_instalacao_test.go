package ia

import (
	"errors"
	"os"
	"os/exec"
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

// App aberto pelo Finder: o Claude instalado pelo Homebrew (/opt/homebrew/bin) fica fora do PATH.
func TestClaudeHomebrewForaDoPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("o Homebrew é só do Mac e do Linux")
	}
	t.Setenv("HOME", t.TempDir()) // sem ~/.local/bin/claude
	t.Setenv("PATH", t.TempDir())
	brew := t.TempDir()
	antes := pastasDoSistemaCLI
	pastasDoSistemaCLI = []string{filepath.Join(t.TempDir(), "nao-existe"), brew}
	t.Cleanup(func() { pastasDoSistemaCLI = antes })

	alvo := filepath.Join(brew, "claude")
	if err := os.WriteFile(alvo, []byte("teste de localização, nunca executado"), 0700); err != nil {
		t.Fatal(err)
	}
	if p, err := localizarCLI("claude"); err != nil || p != alvo {
		t.Fatalf("não encontrou o Claude do Homebrew: %q, %v", p, err)
	}
}

// A instalação nativa vem antes da do Homebrew, como antes desta mudança.
func TestClaudeNativoAntesDoHomebrew(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("o Homebrew é só do Mac e do Linux")
	}
	casa := t.TempDir()
	t.Setenv("HOME", casa)
	t.Setenv("PATH", t.TempDir())
	brew := t.TempDir()
	antes := pastasDoSistemaCLI
	pastasDoSistemaCLI = []string{brew}
	t.Cleanup(func() { pastasDoSistemaCLI = antes })

	nativo := filepath.Join(casa, ".local", "bin")
	if err := os.MkdirAll(nativo, 0700); err != nil {
		t.Fatal(err)
	}
	for _, pasta := range []string{nativo, brew} {
		if err := os.WriteFile(filepath.Join(pasta, "claude"), []byte("teste de localização, nunca executado"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if p, err := localizarCLI("claude"); err != nil || p != filepath.Join(nativo, "claude") {
		t.Fatalf("esperava a instalação nativa primeiro: %q, %v", p, err)
	}
}

// Sem nenhuma instalação conhecida, o erro continua sendo "não encontrado".
func TestClaudeNenhumaInstalacao(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	antes := pastasDoSistemaCLI
	pastasDoSistemaCLI = []string{t.TempDir()}
	t.Cleanup(func() { pastasDoSistemaCLI = antes })

	if _, err := localizarCLI("claude"); !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("esperava exec.ErrNotFound, veio %v", err)
	}
}
