package ia

import (
	"context"
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
	s, err := cli{qual: "gemini"}.Chamar(context.Background(), PedidoProvedor{Prompt: MontarPrompt("triarVaga", entradaTriagem)})
	if err != nil || s.Texto == "" {
		t.Fatalf("gemini = %+v, %v", s, err)
	}
}
