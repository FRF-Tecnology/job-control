//go:build windows

package ia

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// jc-a2y.25: no Windows, a CLI instalada pelo npm é um .cmd, e o .cmd roda dentro do cmd.exe. O Go não escapa os
// argumentos para o cmd.exe, então texto no argv pode virar comando, e uma quebra de linha corta o resto dos
// argumentos. As CLIs falsas aqui são .cmd que gravam o argv (o redirecionamento vem antes, para não ser cortado) e
// o stdin recebidos. A vaga traz o payload da auditoria. Só dados fictícios.
const payloadCmd = `x" & type nul > %TEMP%\marcador-jc25 & rem "`

func comCLIFalsaCmd(t *testing.T, nome string) (pasta string) {
	t.Helper()
	pasta = t.TempDir()
	script := "@echo off\r\n" +
		">\"" + filepath.Join(pasta, "argv.txt") + "\" echo(%*\r\n" +
		"more > \"" + filepath.Join(pasta, "stdin.txt") + "\"\r\n" +
		"echo {\"response\":\"{}\"}\r\n"
	if err := os.WriteFile(filepath.Join(pasta, nome+".cmd"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", pasta+string(os.PathListSeparator)+filepath.Join(os.Getenv("SystemRoot"), "System32")) // more.com
	t.Setenv("TEMP", pasta)                                                                                 // o payload grava em %TEMP%: fica dentro da pasta do teste
	t.Setenv("TMP", pasta)
	return pasta
}

func promptComPayload() Prompt {
	return MontarPrompt("triarVaga", Objeto{
		"vaga":   Objeto{"id": "li-1", "empresa": "Empresa Exemplo", "titulo": "Dev " + payloadCmd, "notas": "Requisitos: " + payloadCmd},
		"perfil": Objeto{"nome": "Pessoa Exemplo"},
	})
}

func existe(caminho string) bool { _, err := os.Stat(caminho); return err == nil }

// Gemini e Codex via .cmd: o texto da vaga vai pelo stdin, o argv só tem argumentos fixos e nada roda.
func TestCLIViaCmdTextoDaVagaSoPeloStdin(t *testing.T) {
	for _, qual := range []string{"gemini", "codex"} {
		t.Run(qual, func(t *testing.T) {
			pasta := comCLIFalsaCmd(t, qual)
			chamarSemPortao(qual, PedidoProvedor{Modelo: "modelo-exemplo", Prompt: promptComPayload()})
			if existe(filepath.Join(pasta, "marcador-jc25")) {
				t.Fatal("o payload da vaga rodou no cmd.exe")
			}
			argv, err := os.ReadFile(filepath.Join(pasta, "argv.txt"))
			if err != nil {
				t.Fatalf("a CLI falsa não rodou: %v", err)
			}
			if strings.Contains(string(argv), "Empresa Exemplo") || strings.Contains(string(argv), "marcador") {
				t.Fatalf("o argv leva texto da vaga: %s", argv)
			}
			stdin, _ := os.ReadFile(filepath.Join(pasta, "stdin.txt"))
			if !strings.Contains(string(stdin), "Empresa Exemplo") {
				t.Fatalf("o pedido não chegou pelo stdin: %.200s", stdin)
			}
		})
	}
}

// Claude via .cmd: o prompt de sistema (várias linhas) e o esquema (com aspas) não são seguros para o cmd.exe, e a
// quebra de linha cortaria --tools "" e as outras travas. O app recusa antes de rodar, com erro de configuração.
func TestClaudeViaCmdERecusado(t *testing.T) {
	pasta := comCLIFalsaCmd(t, "claude")
	_, err := cli{qual: "claude"}.Chamar(context.Background(), PedidoProvedor{Modelo: "haiku", Prompt: promptComPayload()})
	var e *ErroIA
	if !errors.As(err, &e) || e.Tipo != "config" || e.Mensagem != "claude-cli: "+msgCmdInseguro {
		t.Fatalf("erro = %v", err)
	}
	if existe(filepath.Join(pasta, "argv.txt")) || existe(filepath.Join(pasta, "marcador-jc25")) {
		t.Fatal("a CLI .cmd rodou mesmo assim")
	}
}

// O mecanismo de fundo: um argumento de uma linha com o payload rodava no cmd.exe. Agora rodar recusa.
func TestArgumentoInseguroNaoChegaAoCmdExe(t *testing.T) {
	pasta := comCLIFalsaCmd(t, "gemini")
	_, _, err := rodar(context.Background(), "gemini", []string{"-p", payloadCmd}, "")
	var e *ErroIA
	if !errors.As(err, &e) || e.Tipo != "config" {
		t.Fatalf("erro = %v", err)
	}
	if existe(filepath.Join(pasta, "marcador-jc25")) || existe(filepath.Join(pasta, "argv.txt")) {
		t.Fatal("o payload chegou ao cmd.exe")
	}
}

// A tela Conectar IA vê o Claude .cmd como instalado, mas não pronto, com o aviso fixo; Gemini e Codex .cmd ficam prontos.
func TestDetectarCLIsMarcaClaudeCmdComoNaoPronto(t *testing.T) {
	pasta := t.TempDir()
	for _, qual := range []string{"claude", "codex", "gemini"} {
		if err := os.WriteFile(filepath.Join(pasta, qual+".cmd"), []byte("@echo off\r\necho 1.0.0\r\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", pasta+string(os.PathListSeparator)+filepath.Join(os.Getenv("SystemRoot"), "System32"))
	for _, e := range DetectarCLIs(context.Background()) {
		// Codex e Gemini ficam fora do app (jc-a2y.26); o Claude .cmd fica fora por causa do cmd.exe (jc-a2y.25).
		aviso := map[string]string{"claude": msgCmdInseguro, "codex": msgSemTrava, "gemini": msgSemTrava}[e.Comando]
		if !e.Instalada || e.Pronta || e.Aviso != aviso {
			t.Errorf("%s: instalada=%v pronta=%v aviso=%q", e.Comando, e.Instalada, e.Pronta, e.Aviso)
		}
	}
}
