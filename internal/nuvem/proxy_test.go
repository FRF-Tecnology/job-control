package nuvem

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func prefixos(t *testing.T, cidrs ...string) []netip.Prefix {
	t.Helper()
	var out []netip.Prefix
	for _, c := range cidrs {
		out = append(out, netip.MustParsePrefix(c))
	}
	return out
}

func TestIPDoCliente(t *testing.T) {
	proxy := prefixos(t, "10.0.0.0/8", "2001:db8:aaaa::/48")
	casos := []struct {
		nome       string
		remoto     string
		cf         string
		xff        []string
		confiaveis []netip.Prefix
		quer       string
	}{
		{"sem proxy configurado", "198.51.100.7:4000", "", nil, nil, "198.51.100.7"},
		{"sem proxy configurado ignora cabeçalhos", "198.51.100.7:4000", "203.0.113.9", []string{"203.0.113.10"}, nil, "198.51.100.7"},
		{"remoto sem porta", "198.51.100.7", "", nil, nil, "198.51.100.7"},
		{"remoto IPv6", "[2001:db8:1::5]:4000", "", nil, proxy, "2001:db8:1::5"},
		{"peer não confiável ignora CF", "198.51.100.7:4000", "203.0.113.9", nil, proxy, "198.51.100.7"},
		{"peer não confiável ignora XFF", "198.51.100.7:4000", "", []string{"203.0.113.9"}, proxy, "198.51.100.7"},
		{"peer não confiável, tudo forjado", "198.51.100.7:4000", "203.0.113.9", []string{"203.0.113.10, 10.1.1.1"}, proxy, "198.51.100.7"},
		{"proxy confiável com CF", "10.1.2.3:4000", "203.0.113.9", nil, proxy, "203.0.113.9"},
		{"CF tem prioridade sobre XFF", "10.1.2.3:4000", "203.0.113.9", []string{"203.0.113.77"}, proxy, "203.0.113.9"},
		{"CF IPv6 com espaços", "10.1.2.3:4000", "  2001:db8:9::1 ", nil, proxy, "2001:db8:9::1"},
		{"CF inválido cai no XFF", "10.1.2.3:4000", "lixo", []string{"203.0.113.77"}, proxy, "203.0.113.77"},
		{"CF inválido e sem XFF: IP do proxy", "10.1.2.3:4000", "lixo", nil, proxy, "10.1.2.3"},
		{"proxy sem cabeçalhos: IP do proxy", "10.1.2.3:4000", "", nil, proxy, "10.1.2.3"},
		{"XFF simples", "10.1.2.3:4000", "", []string{"203.0.113.77"}, proxy, "203.0.113.77"},
		{"XFF: o mais à direita não confiável", "10.1.2.3:4000", "", []string{"1.1.1.1, 203.0.113.77, 10.9.9.9"}, proxy, "203.0.113.77"},
		{"XFF: forjado à esquerda não vale", "10.1.2.3:4000", "", []string{"9.9.9.9, 203.0.113.77"}, proxy, "203.0.113.77"},
		{"XFF em várias linhas", "10.1.2.3:4000", "", []string{"9.9.9.9", "203.0.113.77, 10.4.4.4"}, proxy, "203.0.113.77"},
		{"XFF todo confiável: IP do proxy", "10.1.2.3:4000", "", []string{"10.5.5.5, 10.6.6.6"}, proxy, "10.1.2.3"},
		{"XFF com entrada inválida à direita: IP do proxy", "10.1.2.3:4000", "", []string{"203.0.113.77, lixo"}, proxy, "10.1.2.3"},
		{"XFF vazio", "10.1.2.3:4000", "", []string{" , "}, proxy, "10.1.2.3"},
		{"XFF IPv6 confiável pula", "10.1.2.3:4000", "", []string{"203.0.113.77, 2001:db8:aaaa::7"}, proxy, "203.0.113.77"},
		{"proxy IPv4 mapeado em IPv6", "[::ffff:10.1.2.3]:4000", "203.0.113.9", nil, proxy, "203.0.113.9"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/v1/saude", nil)
			r.RemoteAddr = c.remoto
			if c.cf != "" {
				r.Header.Set("CF-Connecting-IP", c.cf)
			}
			for _, x := range c.xff {
				r.Header.Add("X-Forwarded-For", x)
			}
			if got := ipDoCliente(r, c.confiaveis); got != c.quer {
				t.Fatalf("ipDoCliente = %q, quer %q", got, c.quer)
			}
		})
	}
}

func TestLerConfigProxyConfiavel(t *testing.T) {
	env := envCompleto(t)
	cfg, err := LerConfig(envDe(env))
	if err != nil || len(cfg.ProxyConfiavel) != 0 {
		t.Fatalf("sem a variável: %v %v", cfg.ProxyConfiavel, err)
	}
	env["APPLYER_PROXY_CONFIAVEL"] = " 10.0.0.0/8 , 2001:db8::/32,172.16.0.5 "
	cfg, err = LerConfig(envDe(env))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.ProxyConfiavel) != 3 || cfg.ProxyConfiavel[2].String() != "172.16.0.5/32" {
		t.Fatalf("prefixos = %v", cfg.ProxyConfiavel)
	}
	env["APPLYER_PROXY_CONFIAVEL"] = "10.0.0.0/8,nao-e-cidr"
	if _, err = LerConfig(envDe(env)); err == nil || !strings.Contains(err.Error(), "APPLYER_PROXY_CONFIAVEL") {
		t.Fatalf("CIDR inválido: veio %v", err)
	}
}

// Atrás de um proxy confiável, cada CF-Connecting-IP tem o seu próprio limite por IP.
func TestLimitePorIPAtrasDeProxyConfiavel(t *testing.T) {
	a := novoAmb(t)
	a.s.cfg.ProxyConfiavel = prefixos(t, "10.0.0.0/8")
	de := func(remoto, cf string) int {
		req := httptest.NewRequest("POST", "/v1/sessao/sair", strings.NewReader(`{}`))
		req.RemoteAddr = remoto
		if cf != "" {
			req.Header.Set("CF-Connecting-IP", cf)
		}
		w := httptest.NewRecorder()
		a.s.Handler().ServeHTTP(w, req)
		return w.Code
	}
	for i := 0; i < limitePorIP; i++ {
		if c := de("10.0.0.1:5000", "203.0.113.1"); c != http.StatusNoContent {
			t.Fatalf("pedido %d do cliente A: %d", i+1, c)
		}
	}
	if c := de("10.0.0.1:5000", "203.0.113.1"); c != http.StatusTooManyRequests {
		t.Fatalf("cliente A acima do limite: %d", c)
	}
	if c := de("10.0.0.1:5000", "203.0.113.2"); c != http.StatusNoContent {
		t.Fatalf("cliente B, mesmo proxy, tem outro limite: %d", c)
	}
	// Peer não confiável não escolhe o próprio IP pelo cabeçalho.
	for i := 0; i < limitePorIP; i++ {
		de("198.51.100.9:5000", "203.0.113."+string(rune('1'+i%9)))
	}
	if c := de("198.51.100.9:5000", "203.0.113.99"); c != http.StatusTooManyRequests {
		t.Fatalf("peer não confiável trocando CF escapou do limite: %d", c)
	}
}
