package ia

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

// pastasDoSistemaNoTeste troca as pastas do Homebrew pelas do teste (nenhuma = nada instalado).
// Sem isso, um teste de "nada instalado" acharia o Claude de verdade de quem roda o teste no Mac.
func pastasDoSistemaNoTeste(t *testing.T, pastas ...string) {
	t.Helper()
	if len(pastas) == 0 {
		pastas = []string{t.TempDir()}
	}
	antes := pastasDoSistemaCLI
	pastasDoSistemaCLI = pastas
	t.Cleanup(func() { pastasDoSistemaCLI = antes })
}

// App aberto pelo Finder: o Claude instalado pelo Homebrew (/opt/homebrew/bin) fica fora do PATH.
func TestClaudeHomebrewForaDoPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("o Homebrew é só do Mac e do Linux")
	}
	t.Setenv("HOME", t.TempDir()) // sem ~/.local/bin/claude
	t.Setenv("PATH", t.TempDir())
	brew := t.TempDir()
	pastasDoSistemaNoTeste(t, filepath.Join(t.TempDir(), "nao-existe"), brew)

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
	pastasDoSistemaNoTeste(t, brew)

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

// Claude do npm do Homebrew: é um script "#!/usr/bin/env node" e o node mora na mesma pasta. Com o PATH do Finder,
// o env só acha o node se a pasta da CLI for para o PATH do processo (antes saía com código 127).
func TestClaudeDoNpmAchaONodeDaMesmaPasta(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("script com #! é só do Mac e do Linux")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", "/usr/bin:/bin") // o PATH de um app aberto pelo Finder, sem a pasta do Homebrew
	brew := t.TempDir()
	pastasDoSistemaNoTeste(t, brew)
	arquivos := map[string]string{
		"claude":    "#!/usr/bin/env nodefalso\n",        // como o cli.js do npm, mas com um "node" de teste
		"nodefalso": "#!/bin/sh\necho node-encontrado\n", // o "node" que mora na mesma pasta
	}
	for nome, conteudo := range arquivos {
		if err := os.WriteFile(filepath.Join(brew, nome), []byte(conteudo), 0700); err != nil {
			t.Fatal(err)
		}
	}
	out, stderr, err := rodar(context.Background(), "claude", nil, "")
	if err != nil || strings.TrimSpace(string(out)) != "node-encontrado" {
		t.Fatalf("saída %q, stderr %q, erro %v", out, stderr, err)
	}
}

func TestAmbienteDaCLIPoeAPastaNoPath(t *testing.T) {
	pasta := filepath.Join(t.TempDir(), "bin")
	t.Setenv("PATH", "/usr/bin")
	if !temNoPath(ambienteDaCLI(filepath.Join(pasta, "claude")), pasta) {
		t.Fatal("a pasta da CLI não foi para o PATH")
	}
	t.Setenv("PATH", pasta+string(os.PathListSeparator)+"/usr/bin")
	if env := ambienteDaCLI(filepath.Join(pasta, "claude")); strings.Count(strings.Join(env, "\n"), pasta) != 1 {
		t.Fatal("a pasta já estava no PATH e foi repetida")
	}
	t.Setenv("PATH", "/usr/bin")
	if temNoPath(ambienteDaCLI("claude"), pasta) {
		t.Fatal("comando sem caminho não deveria mudar o PATH")
	}
}

func temNoPath(env []string, pasta string) bool {
	for _, v := range env {
		if chave, valor, ok := strings.Cut(v, "="); ok && strings.EqualFold(chave, "PATH") {
			for _, p := range filepath.SplitList(valor) {
				if p == pasta {
					return true
				}
			}
		}
	}
	return false
}

// Sem nenhuma instalação conhecida, o erro continua sendo "não encontrado".
func TestClaudeNenhumaInstalacao(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	pastasDoSistemaNoTeste(t)

	if _, err := localizarCLI("claude"); !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("esperava exec.ErrNotFound, veio %v", err)
	}
}
