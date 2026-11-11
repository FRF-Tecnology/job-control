package conta

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"
)

func TestHTTPEntrar(t *testing.T) {
	var metodo, caminho string
	var corpo map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metodo, caminho, corpo = r.Method, r.URL.Path, nil
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &corpo)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"acesso":"a1","renovacao":"r1","acesso_ate":"2026-11-11T12:00:00Z"}`))
	}))
	defer srv.Close()
	n, err := NovaHTTP(srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	p, err := n.Entrar(context.Background(), "ana@exemplo.com", "senha-forte-1", "d", "2026-11")
	if err != nil {
		t.Fatal(err)
	}
	if metodo != http.MethodPost || caminho != "/v1/contas/entrar" {
		t.Fatalf("pedido errado: %s %s", metodo, caminho)
	}
	if corpo["email"] != "ana@exemplo.com" || corpo["senha"] != "senha-forte-1" ||
		corpo["dispositivo"] != "d" || corpo["termos_versao"] != "2026-11" {
		t.Fatalf("corpo errado: %v", corpo)
	}
	if p.Acesso != "a1" || p.Renovacao != "r1" || !p.AcessoAte.Equal(time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("par errado: %+v", p)
	}
	// sem termos, o campo não vai
	_, _ = n.Entrar(context.Background(), "ana@exemplo.com", "x", "d", "")
	if _, tem := corpo["termos_versao"]; tem {
		t.Fatal("termos_versao não devia ir vazio")
	}
}

func TestHTTPBearerEPaths(t *testing.T) {
	var auth, caminho, query, metodo string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, caminho, query, metodo = r.Header.Get("Authorization"), r.URL.Path, r.URL.RawQuery, r.Method
		switch r.URL.Path {
		case "/v1/licenca":
			_, _ = w.Write([]byte(`{"dados":"ZA==","assinatura":"cw=="}`))
		case "/v1/notas":
			_, _ = w.Write([]byte(`[{"numero":"1","link_pdf":"https://x/1.pdf","estado":"emitida","criada_em":"2026-11-01T10:00:00Z"}]`))
		case "/v1/assinar":
			_, _ = w.Write([]byte(`{"url":"https://stripe.exemplo.com/c"}`))
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()
	n, _ := NovaHTTP(srv.URL, nil)
	ctx := context.Background()
	d, err := n.Licenca(ctx, "tok", "abc")
	if err != nil || d.Dados != "ZA==" || auth != "Bearer tok" || caminho != "/v1/licenca" || query != "dispositivo=abc" || metodo != "GET" {
		t.Fatalf("licenca: %v %+v %s %s %s", err, d, auth, caminho, query)
	}
	notas, err := n.Notas(ctx, "tok")
	if err != nil || len(notas) != 1 || notas[0].LinkPDF != "https://x/1.pdf" || notas[0].Estado != "emitida" {
		t.Fatalf("notas: %v %+v", err, notas)
	}
	if u, err := n.Assinar(ctx, "tok"); err != nil || u != "https://stripe.exemplo.com/c" {
		t.Fatalf("assinar: %v %s", err, u)
	}
	if err := n.Excluir(ctx, "tok", "ana@exemplo.com"); err != nil || metodo != "DELETE" || caminho != "/v1/contas/eu" {
		t.Fatalf("excluir: %v %s %s", err, metodo, caminho)
	}
}

func TestHTTPErroDaNuvem(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"erro":"Email ou senha incorretos"}`))
	}))
	defer srv.Close()
	n, _ := NovaHTTP(srv.URL, nil)
	_, err := n.Entrar(context.Background(), "a@b.c", "x", "d", "")
	var e *ErroNuvem
	if !errors.As(err, &e) || e.Status != 401 || e.Msg != "Email ou senha incorretos" {
		t.Fatalf("erro errado: %#v", err)
	}
}

func TestHTTPErroTermosEMotivo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"erro":"Os termos mudaram","motivo":"termos","versao":"2026-12"}`))
	}))
	defer srv.Close()
	n, _ := NovaHTTP(srv.URL, nil)
	_, err := n.Confirmar(context.Background(), "a@b.c", "123456", "s", "d", "2026-11")
	var e *ErroNuvem
	if !errors.As(err, &e) || e.Status != 409 || e.Motivo != "termos" || e.Versao != "2026-12" {
		t.Fatalf("erro errado: %#v", err)
	}
}

func TestHTTP503ContinuaErroNuvem(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	n, _ := NovaHTTP(srv.URL, nil)
	err := n.Recuperar(context.Background(), "a@b.c")
	var e *ErroNuvem
	if !errors.As(err, &e) || e.Status != 503 || errors.Is(err, ErrSemInternet) {
		t.Fatalf("erro errado: %#v", err)
	}
}

func TestHTTPSemInternet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()
	n, _ := NovaHTTP(url, nil)
	_, err := n.Renovar(context.Background(), "r")
	if !errors.Is(err, ErrSemInternet) {
		t.Fatalf("queria ErrSemInternet, veio %v", err)
	}
}

func TestHTTPRecusaHTTPForaDoLoopback(t *testing.T) {
	for _, end := range []string{"http://nuvem.exemplo.com", "ftp://127.0.0.1", "", "nuvem.exemplo.com"} {
		if _, err := NovaHTTP(end, nil); err == nil {
			t.Errorf("devia recusar %q", end)
		}
	}
	for _, end := range []string{"https://nuvem.exemplo.com", "http://127.0.0.1:8080", "http://localhost:9"} {
		if _, err := NovaHTTP(end, nil); err != nil {
			t.Errorf("devia aceitar %q: %v", end, err)
		}
	}
}

func TestHTTPCorpoGrandeDemais(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 2<<20)
		for i := range b {
			b[i] = ' '
		}
		_, _ = w.Write(b)
	}))
	defer srv.Close()
	n, _ := NovaHTTP(srv.URL, nil)
	if _, err := n.Notas(context.Background(), "t"); err == nil {
		t.Fatal("devia falhar com corpo grande")
	}
}

func TestDispositivoHex64(t *testing.T) {
	d, err := Dispositivo()
	if err != nil {
		t.Skipf("sem id de máquina neste ambiente: %v", err)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(d) {
		t.Fatalf("não é hex de 64: %q", d)
	}
}

func TestHTTPContextoCancelado(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()
	n, _ := NovaHTTP(srv.URL, nil)
	ctx, cancela := context.WithCancel(context.Background())
	cancela()
	if _, err := n.Renovar(ctx, "r"); !errors.Is(err, context.Canceled) {
		t.Fatalf("queria context.Canceled, veio %v", err)
	}
}
