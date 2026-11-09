package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// Texto pessoal fictício que uma CLI ou um servidor compatível poderia mandar (jc-a2y.21).
const textoPessoalAPI = `Error: quota for fulano.ficticio@exemplo.com exceeded; see C:\Users\Usuario Exemplo\.config\cli\settings.json sk_ant_SENTINELAcli`

// CLI falsa: com CENTRAL_CLI_FALSA_API ligada, o próprio binário de teste faz o papel da CLI e falha com texto pessoal.
func init() {
	if os.Getenv("CENTRAL_CLI_FALSA_API") == "" {
		return
	}
	fmt.Fprint(os.Stderr, textoPessoalAPI)
	os.Exit(3)
}

func semTextoPessoalAPI(t *testing.T, onde, texto string) {
	t.Helper()
	baixa := strings.ToLower(texto)
	for _, proibido := range []string{"fulano", "ficticio", "exemplo.com", "usuario", "settings", ".config", "quota", "sentinela", "sk_ant"} {
		if strings.Contains(baixa, proibido) {
			t.Fatalf("%s traz %q: %s", onde, proibido, texto)
		}
	}
}

func apiCom(t *testing.T, provedores map[string]ia.Provedor) *API {
	t.Helper()
	arm, err := armazem.Abrir(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	return Nova(arm, Opcoes{Hoje: func() string { return "2026-11-08" }, Aleatorio: func() string { return "abcd" },
		Cofre: segredos.Memoria{}, ProvedoresIA: provedores,
		DetectarCLIs: func(context.Context) []ia.EstadoCLI { return nil }})
}

func erroDoTestar(t *testing.T, a *API) (int, string, map[string]any) {
	t.Helper()
	st, corpo := chamarHTTP(t, a, "POST", Prefixo+"/ia/testar", "{}")
	var dec map[string]any
	json.Unmarshal([]byte(corpo), &dec)
	return st, corpo, dec
}

// Ponta a ponta pelo HTTP com as CLIs de verdade do app rodando uma CLI falsa que escreve email e caminho no stderr.
func TestErroDaCLIPeloHTTPSemTextoDeFora(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	pasta := t.TempDir()
	for _, qual := range []string{"claude", "codex", "gemini"} {
		nome := qual
		if runtime.GOOS == "windows" {
			nome += ".exe"
		}
		if err := os.WriteFile(filepath.Join(pasta, nome), b, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", pasta+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CENTRAL_CLI_FALSA_API", "1")
	lerLog := capturarLog(t)
	for _, qual := range []string{"claude", "codex", "gemini"} {
		t.Run(qual, func(t *testing.T) {
			a := apiCom(t, nil)
			if st, c := chamarHTTP(t, a, "PUT", Prefixo+"/ia/config", `{"provedor":"`+qual+`-cli"}`); st != 200 {
				t.Fatalf("config = %d %s", st, c)
			}
			st, corpo, dec := erroDoTestar(t, a)
			semTextoPessoalAPI(t, "a resposta", corpo)
			if st != 502 || dec["tipo"] != "provedor" || dec["erro"] != qual+"-cli: a CLI saiu com código 3 (rode a CLI no terminal para ver o detalhe)" {
				t.Fatalf("POST /ia/testar = %d %s", st, corpo)
			}
		})
	}
	semTextoPessoalAPI(t, "o log", lerLog())
}

// Ponta a ponta pelo HTTP: servidor compatível do Google devolve um motivo de bloqueio com texto pessoal.
func TestBloqueioDoGooglePeloHTTPSemTextoDeFora(t *testing.T) {
	for _, motivo := range []string{textoPessoalAPI, "SK_ANT_SENTINELA", "toString", "SAFETY"} {
		t.Run(motivo[:min(len(motivo), 12)], func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, _ := json.Marshal(map[string]any{"promptFeedback": map[string]any{"blockReason": motivo}})
				w.Write(b)
			}))
			defer srv.Close()
			real := ia.Nova(ia.Config{}, nil, nil).Provedores["google"]
			a := apiCom(t, map[string]ia.Provedor{"google": provedorRedirecionado{real, srv.URL}})
			lerLog := capturarLog(t)
			chamarHTTP(t, a, "PUT", Prefixo+"/ia/chaves/google", `{"chave":"chave-ficticia-google-1234"}`)
			chamarHTTP(t, a, "PUT", Prefixo+"/ia/config", `{"provedor":"google","modelo":"modelo-exemplo"}`)
			st, corpo, dec := erroDoTestar(t, a)
			quer := "google: bloqueado (motivo não informado)"
			if motivo == "SAFETY" {
				quer = "google: bloqueado (segurança)"
			}
			semTextoPessoalAPI(t, "a resposta", corpo)
			semTextoPessoalAPI(t, "o log", lerLog())
			if st != 502 || dec["erro"] != quer {
				t.Fatalf("POST /ia/testar = %d %s", st, corpo)
			}
		})
	}
}
