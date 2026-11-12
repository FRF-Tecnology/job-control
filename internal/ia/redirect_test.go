package ia

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// O provedor responde com redirecionamento: o app não segue, nenhum outro endereço recebe pedido (e muito menos a chave).
// Só servidores locais (httptest) e chave fictícia (revisão CyberSec, jc-a2y.22).
const chaveRedirect = "sk-ant-api03-SENTINELAredirect-123xyz" // gitleaks:allow

func TestRedirecionamentoNaoESeguido(t *testing.T) {
	for _, prov := range []string{"anthropic", "openai", "google", "xai"} {
		for _, status := range []int{301, 302, 303, 307, 308} {
			for _, destinoNome := range []string{"outro-host", "mesmo-host"} {
				t.Run(fmt.Sprint(prov, "/", status, "/", destinoNome), func(t *testing.T) {
					var noDestino, comChave atomic.Int32
					destino := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						noDestino.Add(1)
						for _, v := range r.Header {
							if strings.Contains(strings.Join(v, " "), "SENTINELA") || strings.Contains(r.URL.RawQuery, "SENTINELA") {
								comChave.Add(1)
							}
						}
						w.WriteHeader(200)
						fmt.Fprint(w, `{}`)
					}))
					defer destino.Close()
					var naOrigem atomic.Int32
					var origem *httptest.Server
					origem = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						naOrigem.Add(1)
						alvo := strings.Replace(destino.URL, "127.0.0.1", "localhost", 1) + r.URL.Path // outro host para o Go
						if destinoNome == "mesmo-host" {
							alvo = origem.URL + "/outro-caminho"
						}
						http.Redirect(w, r, alvo, status)
					}))
					defer origem.Close()

					_, err := Nova(Config{}, nil, nil).Provedores[prov].Chamar(context.Background(), PedidoProvedor{Modelo: "modelo-exemplo",
						Chave: chaveRedirect, Opcoes: OpcoesProvedor{URL: origem.URL}, Prompt: Prompt{Sistema: "s", Mensagem: "u"}})
					var e *ErroIA
					if !errors.As(err, &e) {
						t.Fatalf("erro inesperado: %v", err)
					}
					if noDestino.Load() != 0 || comChave.Load() != 0 || naOrigem.Load() != 1 {
						t.Fatalf("redirecionamento seguido: origem %d, destino %d (com chave %d)", naOrigem.Load(), noDestino.Load(), comChave.Load())
					}
					if e.Tipo != "provedor" || e.Status != status || e.Reintentavel ||
						e.Mensagem != fmt.Sprintf("%s: HTTP %d %s", prov, status, mensagemHTTP(status)) || strings.Contains(e.Mensagem, "localhost") {
						t.Fatalf("erro = %+v", e)
					}
				})
			}
		}
	}
}
