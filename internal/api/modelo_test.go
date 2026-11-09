package api

import (
	"encoding/json"
	"strings"
	"testing"
)

// jc-a2y.43: o modelo configurado vira argumento da CLI (--model). Só passa nome com os caracteres seguros para o
// cmd.exe; outro caractere dá 400 com texto fixo e nada é gravado. Vale para todo provedor (nome de modelo real cabe).
func TestModeloSoComCaracteresSeguros(t *testing.T) {
	a := apiCom(t, nil)
	if st, c := chamarHTTP(t, a, "PUT", Prefixo+"/ia/config", `{"provedor":"claude-cli","modelo":"haiku"}`); st != 200 {
		t.Fatalf("modelo válido = %d %s", st, c)
	}
	for _, modelo := range []string{`haiku\" & calc & rem \"`, "haiku\nopus", "haiku|x", "haiku^x", "%PATH%", "haiku>x", "a(b)", "haiku!", "haʺiku", "--dangerously-skip-permissions", "-m", "--x"} {
		corpo, _ := json.Marshal(map[string]string{"provedor": "claude-cli", "modelo": modelo})
		st, c := chamarHTTP(t, a, "PUT", Prefixo+"/ia/config", string(corpo))
		if st != 400 || !strings.Contains(c, "o nome do modelo não pode começar com -") {
			t.Fatalf("modelo %q = %d %s", modelo, st, c)
		}
	}
	_, c := chamarHTTP(t, a, "GET", Prefixo+"/ia/config", "")
	if !strings.Contains(c, `"modelo":"haiku"`) {
		t.Fatalf("recusa mudou a configuração: %s", c)
	}
	for _, valido := range []string{"claude-haiku-5-5", "anthropic/claude-3.5-sonnet:beta", "gpt-5.1", "gemini-2.5-flash", "meta-llama/llama-3.1-70b-instruct"} {
		corpo, _ := json.Marshal(map[string]string{"provedor": "openai", "modelo": valido})
		if st, c := chamarHTTP(t, a, "PUT", Prefixo+"/ia/config", string(corpo)); st != 200 {
			t.Fatalf("modelo real %q recusado: %d %s", valido, st, c)
		}
	}
}
