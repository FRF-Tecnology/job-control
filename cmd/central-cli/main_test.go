package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// capturar devolve o que f escreveu na saída padrão.
func capturar(t *testing.T, f func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	antes := os.Stdout
	os.Stdout = w
	errF := f()
	os.Stdout = antes
	w.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(b), errF
}

// A importação não mostra a pasta de dados do app (que traz o nome do usuário): nem ao terminar, nem ao recusar
// porque já há dados. Diz só "pasta: CENTRAL_PASTA" ou "pasta: padrão".
func TestImportarNaoMostraAPastaDeDados(t *testing.T) {
	pasta := filepath.Join(t.TempDir(), "Usuario Ficticio", "central-teste")
	t.Setenv("CENTRAL_PASTA", pasta)
	antiga := t.TempDir()
	if err := os.WriteFile(filepath.Join(antiga, "perfil.json"), []byte(`{"nome":"Pessoa Ficticia"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	saida, err := capturar(t, func() error { return importarLegado([]string{"--dir", antiga}) })
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(saida, pasta) || strings.Contains(saida, "Usuario Ficticio") || !strings.Contains(saida, "pasta: CENTRAL_PASTA") {
		t.Errorf("saída da importação: %q", saida)
	}

	_, err = capturar(t, func() error { return importarLegado([]string{"--dir", antiga}) })
	if err == nil {
		t.Fatal("a segunda importação sem --substituir deveria recusar")
	}
	if strings.Contains(err.Error(), pasta) || strings.Contains(err.Error(), "Usuario Ficticio") || !strings.Contains(err.Error(), "pasta: CENTRAL_PASTA") {
		t.Errorf("recusa: %q", err)
	}
}
