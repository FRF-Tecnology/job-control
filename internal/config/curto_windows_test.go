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

// O Windows não diferencia maiúsculas nem barras: a pasta vem no erro como a pessoa digitou (revisão do Fallen no
// #71). Nenhuma variante de caixa ou de barra pode mostrar o nome do usuário.
func TestOcultarCaminhosIgnoraCaixaEBarra(t *testing.T) {
	casa := casaFalsa(t)
	t.Setenv("CENTRAL_PASTA", "")
	arq := filepath.Join(casa, "x", "central.db")
	variantes := []string{
		strings.ToLower(arq),
		strings.ToUpper(arq),
		filepath.ToSlash(strings.ToLower(arq)),
		strings.Replace(arq, `\`, "/", 2), // barras misturadas
	}
	for _, v := range variantes {
		if got := OcultarCaminhos("abrir " + v + ": acesso negado"); strings.Contains(strings.ToLower(got), "usuario ficticio") {
			t.Errorf("%q → %q", v, got)
		}
	}

	// CENTRAL_PASTA digitada em minúsculas, e o MkdirAll falha na pasta-mãe (o caso do ponto 1 do #71).
	arquivo := filepath.Join(casa, "um-arquivo")
	if err := os.WriteFile(arquivo, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CENTRAL_PASTA", strings.ToLower(filepath.Join(arquivo, "dados")))
	_, err := PastaDados()
	if err == nil {
		t.Fatal("MkdirAll dentro de um arquivo deveria falhar")
	}
	if got := OcultarCaminhos(err.Error()); strings.Contains(strings.ToLower(got), "usuario ficticio") {
		t.Errorf("CENTRAL_PASTA em minúsculas: %q", got)
	}

	// Forma curta da pasta do usuário de verdade, em minúsculas.
	t.Setenv("CENTRAL_PASTA", "")
	t.Setenv("USERPROFILE", os.Getenv("SystemDrive")+`\Users\`+os.Getenv("USERNAME"))
	if real, err := os.UserHomeDir(); err == nil {
		if curta := formaCurta(real); curta != real && strings.Contains(curta, "~") {
			v := strings.ToLower(filepath.Join(curta, "AppData"))
			if got := OcultarCaminhos("abrir " + v); strings.Contains(got, strings.ToLower(curta)) {
				t.Errorf("forma curta em minúsculas: %q", got)
			}
		}
	}
}

// Só a pasta inteira: a pasta do usuário não casa dentro de outro nome que começa igual.
func TestOcultarCaminhosSoAPastaInteira(t *testing.T) {
	casa := casaFalsa(t)
	t.Setenv("CENTRAL_PASTA", "")
	outra := casa + "2" + `\x` // "Usuario Ficticio2": outra pasta
	if got := OcultarCaminhos("abrir " + outra); got != "abrir "+outra {
		t.Errorf("trocou dentro de outro nome: %q", got)
	}
	if got := OcultarCaminhos("abrir " + casa + ": negado"); got != "abrir ~: negado" {
		t.Errorf("pasta no fim do caminho = %q", got)
	}
}
