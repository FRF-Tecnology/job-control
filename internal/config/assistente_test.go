package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// jc-a2y.7.4 (A2 do CyberSec): a pasta do assistente só é resolvida e conferida; nada é criado, migrado nem renomeado.
func TestPastaDoAssistente(t *testing.T) {
	base := t.TempDir()
	t.Setenv("APPLYER_PASTA", "")
	t.Setenv("CENTRAL_PASTA", "")
	t.Setenv("APPDATA", base)         // Windows
	t.Setenv("XDG_CONFIG_HOME", base) // Linux
	t.Setenv("HOME", base)            // macOS (Library/Application Support)
	raiz, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	nova, antiga := filepath.Join(raiz, NomePasta), filepath.Join(raiz, NomePastaAntiga)

	// Nenhuma pasta: erro, e nada é criado.
	if _, err := PastaDoAssistente(); !errors.Is(err, ErrSemPastaDeDados) {
		t.Fatalf("sem pasta = %v", err)
	}
	if existeAlgo(nova) {
		t.Fatal("criou a pasta nova")
	}
	// Só a antiga: erro, e ela NÃO é renomeada (A2.1).
	if err := os.MkdirAll(antiga, 0o700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(CaminhoBanco(antiga), []byte("banco fictício"), 0o600)
	if _, err := PastaDoAssistente(); !errors.Is(err, ErrPastaAntigaSemMigrar) {
		t.Fatalf("só a antiga = %v", err)
	}
	if !ehPastaDeVerdade(antiga) || existeAlgo(nova) {
		t.Fatal("o assistente renomeou a pasta antiga")
	}
	// A nova, sem banco: erro.
	if err := os.MkdirAll(nova, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := PastaDoAssistente(); !errors.Is(err, ErrSemPastaDeDados) {
		t.Fatalf("sem banco = %v", err)
	}
	// Com banco, mas sem a proteção do app: erro, e a proteção não muda (A2.2).
	os.WriteFile(CaminhoBanco(nova), []byte("banco fictício"), 0o600)
	if runtime.GOOS != "windows" {
		os.Chmod(nova, 0o755)
	}
	if _, err := PastaDoAssistente(); !errors.Is(err, ErrPastaDesprotegida) {
		t.Fatalf("desprotegida = %v", err)
	}
	if ConferirProtecao(nova) == nil {
		t.Fatal("o assistente protegeu a pasta")
	}
	// Protegida pelo app: abre a nova (a antiga continua lá, como o app deixa).
	if err := ProtegerPasta(nova); err != nil {
		t.Fatal(err)
	}
	p, err := PastaDoAssistente()
	if err != nil || !MesmaPasta(p, nova) {
		t.Fatalf("protegida = %q %v", p, err)
	}
	// Pela variável: a pasta da variável, com as mesmas conferências.
	t.Setenv("APPLYER_PASTA", filepath.Join(base, "outra"))
	if _, err := PastaDoAssistente(); !errors.Is(err, ErrSemPastaDeDados) {
		t.Fatalf("variável sem pasta = %v", err)
	}
	if existeAlgo(filepath.Join(base, "outra")) {
		t.Fatal("criou a pasta da variável")
	}
}
