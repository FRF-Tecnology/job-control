package ia

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Falhas das CLIs com email e caminho fictícios no stderr ou no JSON de erro: a mensagem é só a classe fixa (jc-a2y.21).
func TestErroDaCLINaoRepeteTextoDeFora(t *testing.T) {
	casos := []struct {
		modo, quer   string
		reintentavel bool
		so           string // "" = todas as CLIs
	}{
		{"stderr-pessoal", "a CLI saiu com código 3 (rode a CLI no terminal para ver o detalhe)", true, ""},
		{"stderr-login", msgSemLogin, false, ""},
		{"mudo", msgSemResposta, true, "codex"},
		{"mudo", msgSemResposta, true, "gemini"},
		{"mudo", "resposta não é JSON", true, "claude"}, // texto fixo do Claude Code sem saída
		{"json-erro-pessoal", msgRespondeuErro, true, "claude"},
		{"erro", msgSemLogin, false, "claude"}, // JSON do Claude Code: "Not logged in · Please run /login"
	}
	for _, qual := range []string{"claude", "codex", "gemini"} {
		for _, c := range casos {
			if c.so != "" && c.so != qual {
				continue
			}
			t.Run(qual+"/"+c.modo, func(t *testing.T) {
				comCLIFalsaNome(t, qual, c.modo)
				_, err := cli{qual: qual}.Chamar(context.Background(), PedidoProvedor{Prompt: MontarPrompt("triarVaga", entradaTriagem)})
				var e *ErroIA
				if !errors.As(err, &e) {
					t.Fatalf("erro inesperado: %v", err)
				}
				if e.Mensagem != qual+"-cli: "+c.quer || e.Tipo != "provedor" || e.Reintentavel != c.reintentavel {
					t.Fatalf("erro = %+v", e)
				}
				semTextoPessoal(t, e.Mensagem)
			})
		}
	}
}

// semTextoPessoal falha se a mensagem tiver email, caminho, nome do usuário ou letras da sentinela, em qualquer caixa.
func semTextoPessoal(t *testing.T, msg string) {
	t.Helper()
	baixa := strings.ToLower(msg)
	for _, proibido := range []string{"fulano", "ficticio", "exemplo.com", "@", `c:\`, "users", "usuario", "settings", ".config", "quota", "sentinela", "sk_ant", "error:", "segunda linha", "not logged"} {
		if strings.Contains(baixa, proibido) {
			t.Fatalf("a mensagem traz %q: %q", proibido, msg)
		}
	}
}

// Recusa do Google: o motivo passa por lista fechada; qualquer outro valor vira "motivo não informado".
func TestMotivoDeBloqueioDoGoogleListaFechada(t *testing.T) {
	casos := map[any]string{
		"SAFETY": "segurança", "BLOCKLIST": "termos proibidos", "PROHIBITED_CONTENT": "conteúdo proibido", "IMAGE_SAFETY": "segurança de imagem",
		"OTHER": "motivo não informado", "BLOCK_REASON_UNSPECIFIED": "motivo não informado", "safety": "motivo não informado",
		"SAFETY_": "motivo não informado", "SK_ANT_SENTINELA": "motivo não informado", "sk_ant_api": "motivo não informado",
		"fulano.ficticio@exemplo.com": "motivo não informado", `C:\Users\Usuario Exemplo`: "motivo não informado",
		"toString": "motivo não informado", "constructor": "motivo não informado", 3.0: "motivo não informado", true: "motivo não informado",
	}
	for motivo, quer := range casos {
		if got := motivoBloqueioGoogle(motivo); got != quer {
			t.Errorf("motivoBloqueioGoogle(%v) = %q; queria %q", motivo, got, quer)
		}
		semTextoPessoal(t, motivoBloqueioGoogle(motivo))
	}
}
