package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// jc-a2y.27: a chave salva fica presa ao endereço (host) para o qual foi colada. Trocar a URL do provedor não pode
// mandar a chave antiga para o host novo. Chave fictícia, cofre em memória e servidor TLS local.
const chaveFicticiaHost = "sk-ant-FICTICIA-host-0123456789" // gitleaks:allow

type servidorQueGuardaChaves struct {
	mu     sync.Mutex
	chaves []string
}

func (s *servidorQueGuardaChaves) handler(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	for _, h := range []string{"X-Api-Key", "Authorization", "X-Goog-Api-Key"} {
		if v := r.Header.Get(h); v != "" {
			s.chaves = append(s.chaves, v)
		}
	}
	s.mu.Unlock()
	w.WriteHeader(401)
}

func (s *servidorQueGuardaChaves) recebeu(chave string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.chaves {
		if strings.Contains(v, chave) {
			return true
		}
	}
	return false
}

func TestTrocarURLNaoMandaAChaveAntiga(t *testing.T) {
	outro := &servidorQueGuardaChaves{}
	srv := httptest.NewTLSServer(http.HandlerFunc(outro.handler))
	defer srv.Close()
	antes := http.DefaultTransport
	http.DefaultTransport = srv.Client().Transport // o cliente da IA passa a confiar no certificado do teste
	t.Cleanup(func() { http.DefaultTransport = antes })

	a := apiCom(t, nil)
	if st, c := chamarHTTP(t, a, "PUT", Prefixo+"/ia/chaves/anthropic", `{"chave":"`+chaveFicticiaHost+`"}`); st != 200 {
		t.Fatalf("salvar chave = %d %s", st, c)
	}
	if st, c := chamarHTTP(t, a, "PUT", Prefixo+"/ia/config", `{"provedor":"anthropic","url":"`+srv.URL+`"}`); st != 200 {
		t.Fatalf("trocar URL = %d %s", st, c)
	}
	chamarHTTP(t, a, "POST", Prefixo+"/ia/testar", "{}")
	if outro.recebeu(chaveFicticiaHost) {
		t.Fatal("trocar a URL mandou a chave salva para o host novo")
	}
	_, corpo := chamarHTTP(t, a, "GET", Prefixo+"/ia", "")
	var e map[string]any
	json.Unmarshal([]byte(corpo), &e)
	if e["url"] != srv.URL {
		t.Fatalf("a tela não vê o host personalizado: %v", e["url"])
	}
}

func confiarNoTLS(t *testing.T, srv *httptest.Server) {
	t.Helper()
	antes := http.DefaultTransport
	http.DefaultTransport = srv.Client().Transport
	t.Cleanup(func() { http.DefaultTransport = antes })
}

func decodificarJSON(t *testing.T, corpo string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(corpo), &m); err != nil {
		t.Fatalf("JSON: %v (%s)", err, corpo)
	}
	return m
}

// Os caminhos que importam: trocar o host apaga a chave (e avisa); colar de novo prende a chave ao host novo; voltar
// por outro provedor não contorna; mandar a mesma URL não apaga; e a conferência na hora da chamada segura mesmo se
// a configuração mudar por fora.
func TestChavePresaAoHost(t *testing.T) {
	outro := &servidorQueGuardaChaves{}
	srv := httptest.NewTLSServer(http.HandlerFunc(outro.handler))
	defer srv.Close()
	confiarNoTLS(t, srv)
	a := apiCom(t, nil)
	chave := func(c string) {
		t.Helper()
		if st, b := chamarHTTP(t, a, "PUT", Prefixo+"/ia/chaves/anthropic", `{"chave":"`+c+`"}`); st != 200 {
			t.Fatalf("salvar chave = %d %s", st, b)
		}
	}
	config := func(corpo string) map[string]any {
		t.Helper()
		st, b := chamarHTTP(t, a, "PUT", Prefixo+"/ia/config", corpo)
		if st != 200 {
			t.Fatalf("config %s = %d %s", corpo, st, b)
		}
		return decodificarJSON(t, b)
	}
	finalSalvo := func() any {
		_, b := chamarHTTP(t, a, "GET", Prefixo+"/ia", "")
		return decodificarJSON(t, b)["chaves"].(map[string]any)["anthropic"]
	}

	// 1. Chave do endereço padrão; trocar para o host novo apaga e avisa; a tela vê a chave sumir.
	chave(chaveFicticiaHost)
	if r := config(`{"provedor":"anthropic"}`); r["chaveApagada"] != false {
		t.Fatalf("mesmo host apagou a chave: %v", r)
	}
	if r := config(`{"provedor":"anthropic","url":"` + srv.URL + `"}`); r["chaveApagada"] != true {
		t.Fatalf("trocar o host não apagou a chave: %v", r)
	}
	if finalSalvo() != nil {
		t.Fatal("a chave antiga continua no cofre")
	}
	// 2. A pessoa cola de novo, já vendo o host novo: aí a chave vai para ele (escolha dela).
	nova := "sk-ant-FICTICIA-nova-9876543210" // gitleaks:allow
	chave(nova)
	chamarHTTP(t, a, "POST", Prefixo+"/ia/testar", "{}")
	if !outro.recebeu(nova) {
		t.Fatal("a chave colada para o host novo não foi usada")
	}
	// 3. Mesmo host de novo (com ponto final e outro caminho) não apaga; caixa diferente está no TestHostDe.
	mesmo := strings.Replace(srv.URL, "127.0.0.1", "127.0.0.1.", 1)
	if r := config(`{"provedor":"anthropic","url":"` + mesmo + `/v1"}`); r["chaveApagada"] != false {
		t.Fatalf("mesmo host normalizado apagou a chave: %v", r)
	}
	// 4. Voltar ao endereço padrão apaga a chave do host personalizado (ela não vai para outro lugar).
	if r := config(`{"provedor":"anthropic"}`); r["chaveApagada"] != true {
		t.Fatalf("voltar ao padrão não apagou: %v", r)
	}
	// 5. Desvio por outro provedor: chave do padrão, troca para openai e volta para anthropic com o host novo.
	chave(chaveFicticiaHost)
	config(`{"provedor":"openai","modelo":"modelo-exemplo"}`)
	if r := config(`{"provedor":"anthropic","url":"` + srv.URL + `"}`); r["chaveApagada"] != true {
		t.Fatalf("o desvio por outro provedor manteve a chave: %v", r)
	}
	outro.mu.Lock()
	outro.chaves = nil
	outro.mu.Unlock()
	// 6. Conferência na hora da chamada: chave do padrão e configuração trocada por fora (sem passar pela rota).
	config(`{"provedor":"anthropic"}`)
	chave(chaveFicticiaHost)
	if err := a.arm.GravarMeta(t.Context(), chaveConfig, map[string]any{
		"rotas":      map[string]any{"padrao": map[string]any{"provedor": "anthropic", "modelo": "modelo-exemplo"}},
		"provedores": map[string]any{"anthropic": map[string]any{"url": srv.URL}},
	}); err != nil {
		t.Fatal(err)
	}
	st, b := chamarHTTP(t, a, "POST", Prefixo+"/ia/testar", "{}")
	if outro.recebeu(chaveFicticiaHost) {
		t.Fatal("a chave do endereço padrão saiu para o host novo")
	}
	if st != 400 || !strings.Contains(b, "sem chave") {
		t.Fatalf("sem chave para o host novo = %d %s", st, b)
	}
}

func TestHostDe(t *testing.T) {
	casos := map[string]string{
		"":                               "",
		"  ":                             "",
		"https://API.Exemplo.com/v1":     "api.exemplo.com",
		"https://api.exemplo.com./outro": "api.exemplo.com",
		"https://api.exemplo.com:8443/x": "api.exemplo.com:8443",
		"https://openrouter.ai/api/v1":   "openrouter.ai",
	}
	for u, quer := range casos {
		if got := hostDe(u); got != quer {
			t.Errorf("hostDe(%q) = %q; queria %q", u, got, quer)
		}
	}
	if hostDe("https://") == "" || hostDe("https://") == hostDe("") {
		t.Error("URL sem host não pode valer como o endereço padrão")
	}
}
