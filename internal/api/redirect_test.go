package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// Ponta a ponta pelo HTTP: o provedor redireciona para outro host; o app não segue, responde 502 com o texto fixo,
// e o destino não recebe pedido nem chave (jc-a2y.22). Só servidores locais e chave fictícia.
func TestRedirecionamentoDoProvedorPeloHTTP(t *testing.T) {
	const chave = "sk-ant-api03-SENTINELAredirect-123xyz" // gitleaks:allow
	for _, prov := range []string{"anthropic", "openai", "google"} {
		t.Run(prov, func(t *testing.T) {
			var noDestino atomic.Int32
			destino := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				noDestino.Add(1)
				w.WriteHeader(200)
			}))
			defer destino.Close()
			origem := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, strings.Replace(destino.URL, "127.0.0.1", "localhost", 1)+r.URL.Path, http.StatusTemporaryRedirect)
			}))
			defer origem.Close()
			arm, err := armazem.Abrir(context.Background(), ":memory:")
			if err != nil {
				t.Fatal(err)
			}
			defer arm.Fechar()
			real := ia.Nova(ia.Config{}, nil, nil).Provedores[prov]
			a := Nova(arm, Opcoes{Hoje: func() string { return "2026-11-08" }, Aleatorio: func() string { return "abcd" },
				Cofre:        segredos.Memoria{},
				ProvedoresIA: map[string]ia.Provedor{prov: provedorRedirecionado{real, origem.URL}},
				DetectarCLIs: func(context.Context) []ia.EstadoCLI { return nil }})
			lerLog := capturarLog(t)
			if st, c := chamarHTTP(t, a, "PUT", Prefixo+"/ia/chaves/"+prov, `{"chave":"`+chave+`"}`); st != 200 {
				t.Fatalf("salvar chave = %d %s", st, c)
			}
			if st, c := chamarHTTP(t, a, "PUT", Prefixo+"/ia/config", `{"provedor":"`+prov+`","modelo":"modelo-exemplo"}`); st != 200 {
				t.Fatalf("config = %d %s", st, c)
			}
			st, corpo := chamarHTTP(t, a, "POST", Prefixo+"/ia/testar", "{}")
			var dec map[string]any
			json.Unmarshal([]byte(corpo), &dec)
			if st != 502 || dec["tipo"] != "provedor" || !strings.HasPrefix(dec["erro"].(string), prov+": HTTP 307 o provedor mandou para outro endereço") {
				t.Fatalf("POST /ia/testar = %d %s", st, corpo)
			}
			if noDestino.Load() != 0 {
				t.Fatalf("o destino do redirecionamento recebeu %d pedido(s)", noDestino.Load())
			}
			for _, texto := range []string{corpo, lerLog()} {
				if strings.Contains(texto, "SENTINELA") || strings.Contains(texto, "localhost") {
					t.Fatalf("vazou chave ou destino: %s", texto)
				}
			}
		})
	}
}
