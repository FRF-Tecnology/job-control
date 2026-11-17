package ia

import (
	"context"
	"errors"
	"testing"
)

// jc-a2y.25: o que o cmd.exe interpreta não passa; o que as CLIs usam (opções, modelo, caminho temporário) passa.
func TestArgsSegurosParaCmd(t *testing.T) {
	seguros := [][]string{
		{"-p", "Answer the request given on standard input.", "--output-format", "json", "--model", "gemini-2.5-flash"},
		{"exec", "--skip-git-repo-check", "--sandbox", "read-only", "--output-schema", `C:\Users\Pessoa Exemplo\AppData\Local\Temp\central-codex-1\esquema.json`, "-"},
		{"--tools", ""},
		{"--model", "claude-haiku-5-5", "--effort", "low"},
	}
	for _, a := range seguros {
		if !argsSegurosParaCmd(a) {
			t.Errorf("deveria passar: %q", a)
		}
	}
	inseguros := []string{
		`x" & type nul > %TEMP%\marcador & rem "`, "linha 1\nlinha 2", "a\rb", `{"type":"object"}`, "a&b", "a|b", "a<b", "a>b",
		"a^b", "%PATH%", "!x!", "(a)", "a`b", "tab\there",
		"aʺb", "aˆb", // ʺ (Lm) e ˆ (Sk): a conversão best-fit para ANSI pode virar " e ^
	}
	for _, s := range inseguros {
		if argsSegurosParaCmd([]string{"-p", s}) {
			t.Errorf("deveria recusar: %q", s)
		}
	}
}

// Gemini (qualquer sistema): o pedido vai pelo stdin e o argv não leva o texto da vaga. A CLI falsa confere os dois.
func TestGeminiRecebePedidoPeloStdin(t *testing.T) {
	comCLIFalsaNome(t, "gemini", "gemini-stdin")
	s, err := chamarGemini(context.Background(), PedidoProvedor{Prompt: MontarPrompt("triarVaga", entradaTriagem)})
	if err != nil || s.Texto == "" {
		t.Fatalf("gemini = %+v, %v", s, err)
	}
}

// CLI que não está no PATH: nem instalada, nem pronta, sem aviso.
func TestDetectarCLIsSemNadaInstalado(t *testing.T) {
	casa := t.TempDir()
	t.Setenv("HOME", casa)
	t.Setenv("USERPROFILE", casa)
	t.Setenv("PATH", t.TempDir())
	pastasDoSistemaNoTeste(t) // nem o Claude do Homebrew de quem roda o teste
	for _, e := range DetectarCLIs(context.Background()) {
		if e.Instalada || e.Pronta || e.Aviso != "" || e.Motivo != "" {
			t.Errorf("%s: %+v", e.Comando, e)
		}
	}
}

// chamarSemPortao roda a CLI sem a trava do jc-a2y.26, para os testes continuarem cobrindo o código do Codex e do
// Gemini (argv, stdin e mensagens fixas) enquanto eles estão fora do app.
func chamarSemPortao(qual string, p PedidoProvedor) (RespostaProvedor, error) {
	switch qual {
	case "codex":
		return chamarCodex(context.Background(), p)
	case "gemini":
		return chamarGemini(context.Background(), p)
	}
	return cli{qual: qual}.Chamar(context.Background(), p)
}

// jc-a2y.26: Codex e Gemini ficam fora do app até haver travas de ferramentas conferidas. A trava vem antes de procurar
// a CLI e de rodar qualquer coisa; o Claude continua.
func TestCodexEGeminiForaDoApp(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // nenhuma CLI instalada: a mensagem tem de ser a da trava, não "não instalada"
	for _, qual := range []string{"codex", "gemini"} {
		_, err := cli{qual: qual}.Chamar(context.Background(), PedidoProvedor{Prompt: MontarPrompt("triarVaga", entradaTriagem)})
		var e *ErroIA
		if !errors.As(err, &e) || e.Tipo != "config" || e.Mensagem != qual+"-cli: "+msgSemTrava {
			t.Fatalf("%s: erro = %v", qual, err)
		}
		if msg, fora := CLIIndisponivel(qual + "-cli"); !fora || msg != msgSemTrava {
			t.Fatalf("%s: CLIIndisponivel = %q %v", qual, msg, fora)
		}
	}
	for _, p := range []string{"claude-cli", "anthropic", "openai", "google", "codex", "gemini", ""} {
		if _, fora := CLIIndisponivel(p); fora {
			t.Fatalf("%q não deveria ficar fora", p)
		}
	}
}

// Com Codex e Gemini instalados, a tela Conectar IA os vê como instalados e não prontos, com o aviso fixo.
func TestDetectarCLIsMarcaCodexEGeminiNaoProntos(t *testing.T) {
	comCLIFalsaNome(t, "codex", "mudo")
	comCLIFalsaNome(t, "gemini", "mudo")
	for _, e := range DetectarCLIs(context.Background()) {
		if e.Comando == "claude" {
			continue
		}
		if !e.Instalada || e.Pronta || e.Aviso != msgSemTrava || e.Motivo != MotivoPolitica {
			t.Errorf("%s: instalada=%v pronta=%v aviso=%q", e.Comando, e.Instalada, e.Pronta, e.Aviso)
		}
	}
}

// O Claude nativo (não .cmd) instalado fica pronto, sem aviso e sem motivo.
func TestDetectarCLIsClaudeNativoPronto(t *testing.T) {
	comCLIFalsaNome(t, "claude", "mudo")
	for _, e := range DetectarCLIs(context.Background()) {
		if e.Comando == "claude" && (!e.Instalada || !e.Pronta || e.Aviso != "" || e.Motivo != "") {
			t.Errorf("claude nativo: %+v", e)
		}
	}
}

// jc-a2y.43: modelo que começa com "-" (viraria opção da CLI) ou com caractere inseguro é recusado na hora de rodar,
// mesmo com a configuração gravada por fora da API; a CLI nem é procurada.
func TestCLIRecusaModeloInseguroAntesDeRodar(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	for _, m := range []string{"--dangerously-skip-permissions", "-m", `haiku" & calc`} {
		_, err := cli{qual: "claude"}.Chamar(context.Background(), PedidoProvedor{Modelo: m, Prompt: MontarPrompt("triarVaga", entradaTriagem)})
		var e *ErroIA
		if !errors.As(err, &e) || e.Tipo != "config" || e.Mensagem != "claude-cli: "+MsgModeloInseguro {
			t.Fatalf("modelo %q: erro = %v", m, err)
		}
	}
	for _, m := range []string{"haiku", "claude-haiku-5-5", "a-b"} {
		if !ModeloSeguro(m) {
			t.Errorf("%q deveria passar", m)
		}
	}
}
