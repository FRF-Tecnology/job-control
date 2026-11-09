package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// Chave 100% fictícia; "/", "+" e "=" deixam as formas escapada e codificada diferentes da exata.
const segredoFicticioAPI = "FICTICIAqwerty0123456789/abcdefXYZ+fim=9876" // gitleaks:allow

// O provedor de verdade do app, mas apontado para um servidor falso local (a URL do app exige https).
type provedorRedirecionado struct {
	real ia.Provedor
	url  string
}

func (p provedorRedirecionado) Nome() string { return p.real.Nome() }
func (p provedorRedirecionado) Chamar(ctx context.Context, r ia.PedidoProvedor) (ia.RespostaProvedor, error) {
	r.Opcoes.URL = p.url
	return p.real.Chamar(ctx, r)
}

// Ponta a ponta pelo HTTP: chave no cofre (em memória), provedor ecoa a chave no erro, a resposta 502 não traz nenhum trecho dela.
func TestErro502NaoDevolveAChave(t *testing.T) {
	s := segredoFicticioAPI
	formas := map[string]string{
		"exata":         s,
		"json-escapada": strings.ReplaceAll(s, "/", `\/`),
		"url-encoded":   url.QueryEscape(s),
		"prefixo+final": s[:16] + "..." + s[len(s)-4:],
		"ultimos-20":    s[len(s)-20:],
	}
	// O log do app (pacote log) vai para um buffer durante o teste; no fim, nenhum trecho da chave pode estar lá.
	var registro bytes.Buffer
	saidaAntes, flagsAntes := log.Writer(), log.Flags()
	log.SetOutput(&registro)
	t.Cleanup(func() { log.SetOutput(saidaAntes); log.SetFlags(flagsAntes) })
	defer func() {
		log.Print("marcador-do-teste")
		texto := registro.String()
		if !strings.Contains(texto, "marcador-do-teste") {
			t.Fatalf("a captura do log não funcionou: %q", texto)
		}
		for _, f := range formas {
			for i := 0; i+4 <= len(f); i++ {
				if strings.Contains(texto, f[i:i+4]) {
					t.Fatalf("o log traz um trecho da chave (%q): %s", f[i:i+4], texto)
				}
			}
		}
	}()
	for _, prov := range []string{"anthropic", "openai", "google"} {
		for nome, eco := range formas {
			t.Run(prov+"/"+nome, func(t *testing.T) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(401)
					fmt.Fprintf(w, `{"error":{"type":"authentication_error","message":"Incorrect API key provided: %s"}}`, eco)
				}))
				defer srv.Close()
				arm, err := armazem.Abrir(context.Background(), ":memory:")
				if err != nil {
					t.Fatal(err)
				}
				defer arm.Fechar()
				real := ia.Nova(ia.Config{}, nil, nil).Provedores[prov]
				h := Nova(arm, Opcoes{Hoje: func() string { return "2026-11-08" }, Aleatorio: func() string { return "abcd" },
					Cofre:        segredos.Memoria{},
					ProvedoresIA: map[string]ia.Provedor{prov: provedorRedirecionado{real, srv.URL}},
					DetectarCLIs: func(context.Context) []ia.EstadoCLI { return nil }}).Handler()
				chamar := func(m, p string, corpo any) (int, string) {
					t.Helper()
					b, _ := json.Marshal(corpo)
					req := httptest.NewRequest(m, "http://127.0.0.1"+Prefixo+p, strings.NewReader(string(b)))
					req.Header.Set("Content-Type", "application/json")
					w := httptest.NewRecorder()
					h.ServeHTTP(w, req)
					bruto, _ := io.ReadAll(w.Result().Body)
					return w.Code, string(bruto)
				}
				if st, c := chamar("PUT", "/ia/chaves/"+prov, Objeto{"chave": s}); st != 200 || strings.Contains(c, s) {
					t.Fatalf("salvar chave = %d %s", st, c)
				}
				if st, c := chamar("PUT", "/ia/config", Objeto{"provedor": prov, "modelo": "modelo-exemplo"}); st != 200 {
					t.Fatalf("config = %d %s", st, c)
				}
				chamar("POST", "/vagas", Objeto{"id": "vaga-exemplo", "empresa": "Empresa Exemplo", "titulo": "Vaga de teste"})
				for _, rota := range []string{"/ia/testar", "/ia/triar/vaga-exemplo", "/ia/mensagem/vaga-exemplo"} {
					st, corpo := chamar("POST", rota, Objeto{})
					// Confere o corpo cru (bytes da resposta) e o texto já decodificado, trecho a trecho.
					var dec map[string]any
					json.Unmarshal([]byte(corpo), &dec)
					erro, _ := dec["erro"].(string)
					for _, texto := range []string{corpo, erro} {
						for _, f := range formas {
							for i := 0; i+4 <= len(f); i++ {
								if strings.Contains(texto, f[i:i+4]) {
									t.Fatalf("%s: a resposta traz um trecho da chave (%q): %s", rota, f[i:i+4], corpo)
								}
							}
						}
					}
					if st != 502 || dec["tipo"] != "provedor" || !strings.Contains(erro, "HTTP 401 chave recusada") {
						t.Fatalf("%s = %d %s", rota, st, corpo)
					}
				}
			})
		}
	}
}
