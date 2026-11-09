package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// casaFalsa troca a pasta do usuário (e a de configuração, que fica dentro dela) por uma pasta de teste.
func casaFalsa(t *testing.T) string {
	t.Helper()
	casa := filepath.Join(t.TempDir(), "Usuario Ficticio")
	if err := os.MkdirAll(casa, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", casa)
	t.Setenv("USERPROFILE", casa)
	t.Setenv("APPDATA", filepath.Join(casa, "AppData", "Roaming"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(casa, ".config"))
	return casa
}

func TestOrigemPastaNaoTrazOCaminho(t *testing.T) {
	casa := casaFalsa(t)
	t.Setenv("APPLYER_PASTA", "")
	t.Setenv("CENTRAL_PASTA", "")
	if got := OrigemPasta(); got != "pasta: padrão" {
		t.Errorf("sem variável = %q", got)
	}
	t.Setenv("CENTRAL_PASTA", filepath.Join(casa, "central-teste"))
	if got := OrigemPasta(); got != "pasta: CENTRAL_PASTA (obsoleta)" { // jc-etv.7: aceita por duas versões
		t.Errorf("com CENTRAL_PASTA = %q", got)
	}
}

func TestOcultarCaminhosTiraAPastaEOUsuario(t *testing.T) {
	casa := casaFalsa(t)

	// CENTRAL_PASTA dentro da pasta do usuário (como no roteiro da .6.5): vira o marcador, não "~\...".
	teste := filepath.Join(casa, "AppData", "Local", "central-teste")
	t.Setenv("APPLYER_PASTA", "")
	t.Setenv("CENTRAL_PASTA", teste)
	got := OcultarCaminhos("abrir " + CaminhoBanco(teste) + ": acesso negado")
	if strings.Contains(got, casa) || !strings.Contains(got, "<CENTRAL_PASTA>") {
		t.Errorf("CENTRAL_PASTA: %q", got)
	}

	// Pasta padrão.
	t.Setenv("CENTRAL_PASTA", "")
	base, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	got = OcultarCaminhos("abrir " + CaminhoBanco(filepath.Join(base, NomePasta)) + ": disco cheio")
	if strings.Contains(got, casa) || !strings.Contains(got, "<pasta padrão>") {
		t.Errorf("padrão: %q", got)
	}

	// Erro de verdade do MkdirAll: o caminho do erro é o da pasta-mãe, que não é a pasta de dados.
	arquivo := filepath.Join(casa, "um-arquivo")
	if err := os.WriteFile(arquivo, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CENTRAL_PASTA", filepath.Join(arquivo, "dados"))
	_, err = PastaDados()
	if err == nil {
		t.Fatal("MkdirAll dentro de um arquivo deveria falhar")
	}
	if !strings.Contains(err.Error(), casa) {
		t.Fatalf("o teste supõe que o erro traz o caminho: %v", err)
	}
	got = OcultarCaminhos(err.Error())
	if strings.Contains(got, casa) || strings.Contains(got, "Usuario Ficticio") {
		t.Errorf("MkdirAll: %q", got)
	}

	// Barras normais também (Windows aceita as duas).
	got = OcultarCaminhos("falhou em " + filepath.ToSlash(filepath.Join(casa, "x")))
	if strings.Contains(got, "Usuario Ficticio") {
		t.Errorf("barras normais: %q", got)
	}
}

func TestOcultarCaminhosNaoEstragaTextoSemCaminho(t *testing.T) {
	casaFalsa(t)
	t.Setenv("CENTRAL_PASTA", "dados") // relativo: não traz o usuário, e trocaria a palavra no texto
	const texto = "aplicar migrações: dados do banco 3.2 corrompidos."
	if got := OcultarCaminhos(texto); got != texto {
		t.Errorf("relativo: %q", got)
	}

	// CENTRAL_PASTA na raiz do disco não troca todo caminho do texto.
	raiz := filepath.VolumeName(os.TempDir()) + string(filepath.Separator)
	t.Setenv("CENTRAL_PASTA", raiz)
	outro := "abrir " + filepath.Join(raiz, "outra", "coisa")
	if got := OcultarCaminhos(outro); got != outro {
		t.Errorf("raiz: %q", got)
	}
}
