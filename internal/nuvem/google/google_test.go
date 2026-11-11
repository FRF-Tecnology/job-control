package google

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/go-oidc/v3/oidc/oidctest"
)

const (
	idTeste      = "cliente-ficticio.apps.googleusercontent.com"
	segredoTeste = "segredo-ficticio"
	retornoTeste = "http://127.0.0.1:53682/retorno"
	verifierOK   = "verificador-ficticio-com-mais-de-quarenta-e-tres-caracteres-0123"
)

func TestRedirectSoLoopback(t *testing.T) {
	for _, u := range []string{
		"http://evil.com/retorno", "https://127.0.0.1:5000/retorno", "http://127.0.0.1/retorno",
		"http://127.0.0.1:5000/retorno?x=1", "http://127.0.0.1:5000/outra", "http://localhost:5000/retorno",
		"http://127.0.0.1:123456/retorno", "http://127.0.0.1:5000/retorno@evil.com",
	} {
		if RedirectValido(u) {
			t.Errorf("%q aceito", u)
		}
		r := &Real{ClientID: idTeste, ClientSecret: segredoTeste, emissor: "http://127.0.0.1:1"}
		if _, err := r.Trocar(context.Background(), "c", verifierOK, u); !errors.Is(err, ErrRedirect) {
			t.Errorf("%q: erro = %v", u, err)
		}
	}
	if !RedirectValido(retornoTeste) {
		t.Error("loopback recusado")
	}
}

// emissorTeste é um servidor OIDC de mentira: descoberta e chaves do oidctest mais um /token que
// confere o segredo e o verifier e devolve o id_token assinado com a chave do teste.
type emissorTeste struct {
	srv    *httptest.Server
	priv   *rsa.PrivateKey
	claims map[string]map[string]any // code → claims
	// descobertas conta os GET da descoberta (o Real deve fazer só um).
	descobertas int
}

func novoEmissor(t *testing.T) *emissorTeste {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	e := &emissorTeste{priv: priv, claims: map[string]map[string]any{}}
	o := &oidctest.Server{PublicKeys: []oidctest.PublicKey{{PublicKey: priv.Public(), KeyID: "k1", Algorithm: oidc.RS256}}}
	mux := http.NewServeMux()
	mux.Handle("/", o)
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		e.descobertas++
		o.ServeHTTP(w, r)
	})
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "form", http.StatusBadRequest)
			return
		}
		id, seg, ok := r.BasicAuth()
		if !ok {
			id, seg = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
		}
		c, achou := e.claims[r.PostForm.Get("code")]
		if id != idTeste || seg != segredoTeste || r.PostForm.Get("code_verifier") != verifierOK ||
			r.PostForm.Get("redirect_uri") != retornoTeste || !achou {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"texto do Google segredo-ficticio"}`))
			return
		}
		b, _ := json.Marshal(c)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "acesso-ficticio", "token_type": "Bearer", "expires_in": 3600,
			"id_token": oidctest.SignIDToken(priv, "k1", oidc.RS256, string(b)),
		})
	})
	e.srv = httptest.NewServer(mux)
	o.SetIssuer(e.srv.URL)
	t.Cleanup(e.srv.Close)
	return e
}

func (e *emissorTeste) claimsBase() map[string]any {
	return map[string]any{
		"iss": e.srv.URL, "aud": idTeste, "sub": "1000001",
		"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
		"email": " Ana@Exemplo.com ", "email_verified": true, "name": "Ana Teste",
	}
}

func TestRealConfereIDToken(t *testing.T) {
	e := novoEmissor(t)
	casos := map[string]func(c map[string]any){
		"ok":             func(c map[string]any) {},
		"aud-errado":     func(c map[string]any) { c["aud"] = "outro-cliente" },
		"nao-verificado": func(c map[string]any) { c["email_verified"] = false },
		"sem-verificado": func(c map[string]any) { delete(c, "email_verified") },
		"vencido":        func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() },
		"emissor":        func(c map[string]any) { c["iss"] = "https://accounts.google.com" },
		"sem-sub":        func(c map[string]any) { c["sub"] = "" },
		"sem-email":      func(c map[string]any) { delete(c, "email") },
		"verif-texto":    func(c map[string]any) { c["email_verified"] = "true" },
	}
	for code, mudar := range casos {
		c := e.claimsBase()
		mudar(c)
		e.claims[code] = c
	}
	r := &Real{ClientID: idTeste, ClientSecret: segredoTeste, emissor: e.srv.URL}
	ctx := context.Background()

	for _, code := range []string{"ok", "verif-texto"} {
		id, err := r.Trocar(ctx, code, verifierOK, retornoTeste)
		if err != nil {
			t.Fatalf("%s: %v", code, err)
		}
		if id != (Identidade{Sub: "1000001", Email: "ana@exemplo.com", Nome: "Ana Teste"}) {
			t.Fatalf("%s: identidade = %+v", code, id)
		}
	}
	for code := range casos {
		if code == "ok" || code == "verif-texto" {
			continue
		}
		if _, err := r.Trocar(ctx, code, verifierOK, retornoTeste); !errors.Is(err, ErrGoogle) {
			t.Errorf("%s: erro = %v", code, err)
		}
	}
	// Verifier errado e code desconhecido: o Google recusa; o erro não traz o texto dele.
	for _, v := range [][2]string{{"ok", "verificador-errado"}, {"desconhecido", verifierOK}} {
		_, err := r.Trocar(ctx, v[0], v[1], retornoTeste)
		if !errors.Is(err, ErrGoogle) || strings.Contains(err.Error(), "texto do Google") {
			t.Errorf("%v: erro = %v", v, err)
		}
	}
	// A descoberta acontece uma vez só (o provedor fica guardado).
	if e.descobertas != 1 {
		t.Errorf("descobertas = %d", e.descobertas)
	}
}

func TestRealTentaDescobrirDeNovo(t *testing.T) {
	e := novoEmissor(t)
	c := e.claimsBase()
	e.claims["ok"] = c
	url := e.srv.URL
	r := &Real{ClientID: idTeste, ClientSecret: segredoTeste, emissor: "http://127.0.0.1:1"}
	if _, err := r.Trocar(context.Background(), "ok", verifierOK, retornoTeste); !errors.Is(err, ErrGoogle) {
		t.Fatalf("erro = %v", err)
	}
	r.emissor = url // a falha não ficou guardada
	if _, err := r.Trocar(context.Background(), "ok", verifierOK, retornoTeste); err != nil {
		t.Fatal(err)
	}
}

func TestFalso(t *testing.T) {
	f := Falso{Identidades: map[string]Identidade{"c1": {Sub: "1", Email: "ana@exemplo.com"}}}
	if id, err := f.Trocar(context.Background(), "c1", "v", retornoTeste); err != nil || id.Sub != "1" {
		t.Fatalf("%+v %v", id, err)
	}
	if _, err := f.Trocar(context.Background(), "c2", "v", retornoTeste); !errors.Is(err, ErrGoogle) {
		t.Fatal(err)
	}
	if _, err := f.Trocar(context.Background(), "c1", "v", "http://evil.com/retorno"); !errors.Is(err, ErrRedirect) {
		t.Fatal(err)
	}
}
