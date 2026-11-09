package api

import (
	"context"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// Erro inesperado (500): a resposta é a mensagem fixa e o log só tem método, caminho sem query e tipo do erro.
// Dados fictícios: "Usuario Exemplo" faz o papel do nome do usuário do sistema na pasta de dados.

func chamarHTTP(t *testing.T, a *API, metodo, alvo, corpo string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(metodo, "http://127.0.0.1"+alvo, strings.NewReader(corpo))
	if metodo != "GET" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, req)
	b, _ := io.ReadAll(w.Result().Body)
	return w.Code, string(b)
}

// Backup com a pasta de backups bloqueada: o erro do Go traz o caminho inteiro da pasta de dados.
func TestErro500NaoMostraCaminhoDaPasta(t *testing.T) {
	pasta := filepath.Join(t.TempDir(), "Usuario Exemplo", "Central de Candidaturas")
	if err := os.MkdirAll(pasta, 0o700); err != nil {
		t.Fatal(err)
	}
	// Um arquivo no lugar da pasta de backups faz o MkdirAll do backup falhar.
	if err := os.WriteFile(filepath.Join(pasta, "backups"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	arm, err := armazem.Abrir(context.Background(), filepath.Join(pasta, "central.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer arm.Fechar()
	a := Nova(arm, Opcoes{Hoje: func() string { return "2026-11-08" }, Aleatorio: func() string { return "abcd" }, Cofre: segredos.Memoria{}})
	lerLog := capturarLog(t)

	st, corpo := chamarHTTP(t, a, "POST", Prefixo+"/backup", "{}")
	if st != 500 || strings.TrimSpace(corpo) != `{"erro":"`+ErroInterno+`"}` {
		t.Fatalf("POST /backup = %d %s", st, corpo)
	}
	texto := lerLog()
	for _, proibido := range []string{"Usuario Exemplo", "Central de Candidaturas", t.TempDir(), filepath.ToSlash(pasta), "backups"} {
		if strings.Contains(corpo, proibido) || strings.Contains(texto, proibido) {
			t.Fatalf("%q apareceu; corpo = %s; log = %s", proibido, corpo, texto)
		}
	}
	if !strings.Contains(texto, "api: erro 500 em POST "+Prefixo+"/backup (") || !strings.Contains(texto, "*fs.PathError mkdir") {
		t.Fatalf("o log perdeu o diagnóstico seguro: %s", texto)
	}
}

// Leitura com query string (empresa e título da vaga) e banco fechado: nada da query entra no log nem na resposta.
func TestErro500LogSemQuery(t *testing.T) {
	arm, err := armazem.Abrir(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	a := Nova(arm, Opcoes{Hoje: func() string { return "2026-11-08" }, Aleatorio: func() string { return "abcd" }, Cofre: segredos.Memoria{}})
	arm.Fechar() // toda leitura passa a falhar com erro inesperado
	lerLog := capturarLog(t)

	st, corpo := chamarHTTP(t, a, "GET", Prefixo+"/duplicatas?empresa=Empresa%20Ficticia%20Secreta&titulo=Vaga%20Secreta", "")
	if st != 500 || strings.TrimSpace(corpo) != `{"erro":"`+ErroInterno+`"}` {
		t.Fatalf("GET /duplicatas = %d %s", st, corpo)
	}
	texto := lerLog()
	for _, proibido := range []string{"Empresa", "Ficticia", "Secreta", "empresa=", "titulo", "?"} {
		if strings.Contains(texto, proibido) {
			t.Fatalf("o log traz %q: %s", proibido, texto)
		}
	}
	if !strings.Contains(texto, "api: erro 500 em GET "+Prefixo+"/duplicatas (") {
		t.Fatalf("o log perdeu método e caminho: %s", texto)
	}
}

func TestTipoDoErroSemTexto(t *testing.T) {
	_, err := os.Open(filepath.Join(t.TempDir(), "Usuario Exemplo", "nao-existe.json"))
	got := tipoDoErro(err)
	if strings.Contains(got, "Usuario") || strings.Contains(got, "nao-existe") || !strings.HasPrefix(got, "*fs.PathError open") {
		t.Fatalf("tipoDoErro = %q", got)
	}
}
