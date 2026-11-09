package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// A linha de partida diz só de onde veio a pasta (CENTRAL_PASTA ou padrão), nunca o caminho, que traz o nome
// do usuário (nota do CyberSec no #68).
func TestMensagemDePartidaSemOCaminho(t *testing.T) {
	pasta := filepath.Join(t.TempDir(), "Usuario Ficticio", "central-teste")
	t.Setenv("CENTRAL_PASTA", pasta)
	got := mensagemDePartida("4500")
	if strings.Contains(got, pasta) || strings.Contains(got, "Usuario Ficticio") {
		t.Errorf("mostra o caminho: %q", got)
	}
	if !strings.Contains(got, "http://localhost:4500") || !strings.Contains(got, "pasta: CENTRAL_PASTA") {
		t.Errorf("= %q", got)
	}
	t.Setenv("CENTRAL_PASTA", "")
	if got := mensagemDePartida("4500"); !strings.Contains(got, "pasta: padrão") {
		t.Errorf("padrão = %q", got)
	}
}
