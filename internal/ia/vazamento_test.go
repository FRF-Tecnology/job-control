package ia

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Chave 100% fictícia, com "/", "+" e "=" para as formas escapada e codificada ficarem diferentes da exata.
const segredoFicticio = "FICTICIAqwerty0123456789/abcdefXYZ+fim=9876" // gitleaks:allow

// formasDoSegredo são os jeitos que um provedor pode ecoar a chave (reproduzidos na revisão do PR #14).
func formasDoSegredo(s string) map[string]string {
	return map[string]string{
		"exata":         s,
		"json-escapada": strings.ReplaceAll(s, "/", `\/`),
		"url-encoded":   url.QueryEscape(s),
		"prefixo+final": s[:16] + "..." + s[len(s)-4:],
		"ultimos-20":    s[len(s)-20:],
	}
}

// semPedacoDoSegredo falha se a mensagem tiver qualquer trecho de 4+ caracteres da chave, em qualquer forma.
func semPedacoDoSegredo(t *testing.T, msg, segredo string) {
	t.Helper()
	for _, forma := range formasDoSegredo(segredo) {
		for i := 0; i+4 <= len(forma); i++ {
			if strings.Contains(msg, forma[i:i+4]) {
				t.Fatalf("a mensagem traz um trecho da chave (%q): %q", forma[i:i+4], msg)
			}
		}
	}
}

func chamarProvedorFalso(t *testing.T, prov, endereco string) *ErroIA {
	t.Helper()
	p := Nova(Config{}, nil, nil).Provedores[prov]
	_, err := p.Chamar(context.Background(), PedidoProvedor{Modelo: "modelo-exemplo", Chave: segredoFicticio,
		Opcoes: OpcoesProvedor{URL: endereco}, Prompt: Prompt{Sistema: "s", Mensagem: "u"}})
	var e *ErroIA
	if !errors.As(err, &e) {
		t.Fatalf("%s: erro inesperado: %v", prov, err)
	}
	return e
}

// O corpo de erro do provedor nunca entra na mensagem: nas 5 formas, nos 3 provedores, em vários status.
func TestErroHTTPNaoRepeteAChave(t *testing.T) {
	for _, prov := range []string{"anthropic", "openai", "google"} {
		for nome, eco := range formasDoSegredo(segredoFicticio) {
			for _, status := range []int{400, 401, 403, 429, 500} {
				t.Run(fmt.Sprint(prov, "/", nome, "/", status), func(t *testing.T) {
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(status)
						fmt.Fprintf(w, `{"error":{"type":"authentication_error","message":"Incorrect API key provided: %s"}}`, eco)
					}))
					defer srv.Close()
					e := chamarProvedorFalso(t, prov, srv.URL)
					semPedacoDoSegredo(t, e.Mensagem, segredoFicticio)
					// Diagnóstico útil continua: tipo, status, reintentável e o código curto do provedor.
					if e.Tipo != "provedor" || e.Status != status || e.Reintentavel != (status == 429 || status >= 500) ||
						!strings.Contains(e.Mensagem, fmt.Sprintf("HTTP %d", status)) || !strings.HasSuffix(e.Mensagem, "[authentication_error]") {
						t.Fatalf("erro = %+v", e)
					}
				})
			}
		}
	}
}

// Código do provedor que é pedaço da chave, ou que não é só letras e "_", não passa.
func TestCodigoDoProvedorSoSeForSeguro(t *testing.T) {
	casos := map[string]string{
		`{"error":{"code":"model_not_found"}}`:                 "model_not_found",
		`{"error":{"code":404,"status":"NOT_FOUND"}}`:          "NOT_FOUND",
		`{"error":{"type":"FICTICIAqwerty"}}`:                  "", // pedaço da chave
		`{"error":{"code":"sk-ant-0123"}}`:                     "", // tem "-" e dígitos
		`{"error":{"code":"tem espaço"}}`:                      "",
		`{"error":"texto solto"}`:                              "",
		`não é json ` + segredoFicticio:                        "",
		`{"error":{"code":"` + strings.Repeat("a", 49) + `"}}`: "",
	}
	for corpo, quer := range casos {
		if got := codigoErro([]byte(corpo), segredoFicticio); got != quer {
			t.Errorf("codigoErro(%s) = %q; queria %q", corpo, got, quer)
		}
	}
}

// Falha de rede: o texto do erro do Go traz o endereço (que pode ter a chave); a mensagem não.
func TestFalhaDeRedeNaoRepeteEndereco(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	endereco := srv.URL + "/" + url.PathEscape(segredoFicticio)
	srv.Close() // ninguém escuta mais: conexão recusada
	for _, prov := range []string{"anthropic", "openai", "google"} {
		e := chamarProvedorFalso(t, prov, endereco)
		semPedacoDoSegredo(t, e.Mensagem, segredoFicticio)
		if !strings.HasPrefix(e.Mensagem, prov+": falha de rede (") || strings.Contains(e.Mensagem, "127.0.0.1") || !e.Reintentavel {
			t.Fatalf("%s: erro = %+v", prov, e)
		}
	}
}

func TestMotivoRedeTempoEsgotado(t *testing.T) {
	liberar := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select { // segura a resposta até o teste acabar
		case <-liberar:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(liberar) // roda antes do Close
	ctx, cancelar := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelar()
	_, err := postarJSON(ctx, http.DefaultClient, "anthropic", srv.URL, segredoFicticio, nil, map[string]any{})
	var e *ErroIA
	if !errors.As(err, &e) || e.Mensagem != "anthropic: falha de rede (tempo esgotado)" {
		t.Fatalf("erro = %v", err)
	}
}
