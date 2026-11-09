package ia

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
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
					// Diagnóstico útil continua: tipo, status e reintentável; a mensagem é só o texto fixo do status.
					if e.Tipo != "provedor" || e.Status != status || e.Reintentavel != (status == 429 || status >= 500) ||
						e.Mensagem != fmt.Sprintf("%s: HTTP %d %s", prov, status, mensagemHTTP(status)) {
						t.Fatalf("erro = %+v", e)
					}
				})
			}
		}
	}
}

// Sentinela fictícia com trechos só de letras e "_", como numa chave base64url (revisão CyberSec do PR #15, v2).
const sentinelaFicticia = "sk-ant-api03-SENTINELAvazaKEYfragmento_ABC-123xyz" // gitleaks:allow

// ecosAlterados são trechos da sentinela com caixa trocada, prefixo, sufixo ou "_" no lugar de "-" (CyberSec e QA).
func ecosAlterados() map[string]string {
	run := "SENTINELAvazaKEYfragmento_ABC"
	return map[string]string{
		"exato":        run,
		"caixa-baixa":  strings.ToLower(run),
		"caixa-alta":   strings.ToUpper(run),
		"prefixado":    "x" + run,
		"err_prefixo":  "err_" + run,
		"sufixado":     run + "_err",
		"trecho+caixa": strings.ToLower(run[:12]) + "_code",
		"inicio":       "sk_ant_api",
	}
}

// semLetrasDaSentinela falha se a mensagem tiver 4+ letras seguidas da sentinela, em qualquer caixa,
// ou o começo "sk_ant"/"sk-ant".
func semLetrasDaSentinela(t *testing.T, msg string) {
	t.Helper()
	baixa := strings.ToLower(msg)
	for _, trecho := range regexp.MustCompile(`[A-Za-z]{4,}`).FindAllString(sentinelaFicticia, -1) {
		trecho = strings.ToLower(trecho)
		for i := 0; i+4 <= len(trecho); i++ {
			if strings.Contains(baixa, trecho[i:i+4]) {
				t.Fatalf("a mensagem traz letras da chave (%q): %q", trecho[i:i+4], msg)
			}
		}
	}
	if strings.Contains(baixa, "sk_ant") || strings.Contains(baixa, "sk-ant") {
		t.Fatalf("a mensagem traz o começo da chave: %q", msg)
	}
}

// Nenhum valor do corpo de erro entra na mensagem: 8 ecos × 3 campos (code, type, status) × 3 provedores.
func TestNenhumValorDoCorpoNaMensagem(t *testing.T) {
	for _, prov := range []string{"anthropic", "openai", "google"} {
		for _, campo := range []string{"code", "type", "status"} {
			for nome, eco := range ecosAlterados() {
				t.Run(prov+"/"+campo+"/"+nome, func(t *testing.T) {
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(401)
						fmt.Fprintf(w, `{"error":{%q:%q,"message":"x"}}`, campo, eco)
					}))
					defer srv.Close()
					p := Nova(Config{}, nil, nil).Provedores[prov]
					_, err := p.Chamar(context.Background(), PedidoProvedor{Modelo: "modelo-exemplo", Chave: sentinelaFicticia,
						Opcoes: OpcoesProvedor{URL: srv.URL}, Prompt: Prompt{Sistema: "s", Mensagem: "u"}})
					var e *ErroIA
					if !errors.As(err, &e) {
						t.Fatalf("erro inesperado: %v", err)
					}
					semLetrasDaSentinela(t, e.Mensagem)
					if e.Mensagem != prov+": HTTP 401 "+mensagemHTTP(401) {
						t.Fatalf("mensagem = %q", e.Mensagem)
					}
				})
			}
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
