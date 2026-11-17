package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/config"
	"github.com/FRF-Tecnology/job-control/internal/conta"
	"github.com/FRF-Tecnology/job-control/internal/licenca"
	"github.com/FRF-Tecnology/job-control/internal/mcp"
)

// semRede troca o transporte padrão por um que falha o teste: o MCP nunca fala com a nuvem.
type semRede struct{ t *testing.T }

func (s semRede) RoundTrip(r *http.Request) (*http.Response, error) {
	s.t.Errorf("o MCP chamou a rede: %s", r.URL.Host)
	return nil, errors.New("sem rede no teste")
}

// Task 6: o MCP só lê licenca.json (sem rede, sem renovar). Válida: escreve; vencida: a frase de abrir o app; sem
// arquivo: sem-conta. Ler continua liberado.
func TestMCPSoLeLicenca(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	e, c, tr := conta.EnderecoNuvem, conta.ChavePublica, http.DefaultTransport
	conta.EnderecoNuvem, conta.ChavePublica = "https://nuvem.exemplo.com", base64.StdEncoding.EncodeToString(pub)
	http.DefaultTransport = semRede{t}
	t.Cleanup(func() { conta.EnderecoNuvem, conta.ChavePublica, http.DefaultTransport = e, c, tr })

	pasta := filepath.Join(t.TempDir(), "applyer-teste")
	if err := os.MkdirAll(pasta, 0o700); err != nil {
		t.Fatal(err)
	}
	arm, err := armazem.Abrir(context.Background(), config.CaminhoBanco(pasta))
	if err != nil {
		t.Fatal(err)
	}
	arm.Fechar()
	if err := config.ProtegerPasta(pasta); err != nil {
		t.Fatal(err)
	}
	t.Setenv("APPLYER_PASTA", pasta)
	t.Setenv("CENTRAL_PASTA", "")

	s := &mcp.Servidor{Abrir: abrir, Log: func(string, ...any) {}}
	chamar := func(nome string, args map[string]any) string {
		t.Helper()
		pedido, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": nome, "arguments": args}})
		b, _ := json.Marshal(s.Receber(context.Background(), pedido))
		return string(b)
	}
	adicionar := map[string]any{"vagas": []any{map[string]any{"id": "v-trava", "empresa": "Empresa Teste", "titulo": "Dev Go", "link": "https://exemplo.com/trava"}}}
	gravar := func(emitida time.Time) {
		t.Helper()
		d, err := licenca.Assinar(priv, licenca.Licenca{Usuario: "u1", Email: "ana@exemplo.com", Estado: licenca.Assinante,
			EmitidaEm: emitida, ValidaAte: emitida.Add(30 * time.Minute)})
		if err != nil {
			t.Fatal(err)
		}
		if err := conta.GravarLicenca(pasta, d); err != nil {
			t.Fatal(err)
		}
	}

	if r := chamar("vaga_adicionar", adicionar); !strings.Contains(r, `"isError":true`) || !strings.Contains(r, "Entre na sua conta") {
		t.Fatalf("sem licenca.json = %s", r)
	}
	if r := chamar("vagas_listar", map[string]any{}); strings.Contains(r, `"isError":true`) {
		t.Fatalf("ler sem licença = %s", r)
	}
	gravar(time.Now().Add(-time.Hour))
	if r := chamar("vaga_adicionar", adicionar); !strings.Contains(r, `"isError":true`) || !strings.Contains(r, "Abra o Applyer para renovar o acesso.") {
		t.Fatalf("vencida = %s", r)
	}
	gravar(time.Now())
	if r := chamar("vaga_adicionar", adicionar); strings.Contains(r, `"isError":true`) || !strings.Contains(r, "v-trava") {
		t.Fatalf("válida = %s", r)
	}

	// Ruling A15 (falha fechada): build sem a nuvem com licenca.json na pasta: trava a escrita; ler continua.
	conta.EnderecoNuvem, conta.ChavePublica = "", ""
	if r := chamar("vaga_adicionar", adicionar); !strings.Contains(r, `"isError":true`) || !strings.Contains(r, "Abra o Applyer para renovar o acesso.") {
		t.Fatalf("sem nuvem com licenca.json = %s", r)
	}
	if r := chamar("vagas_listar", map[string]any{}); strings.Contains(r, `"isError":true`) {
		t.Fatalf("ler sem nuvem = %s", r)
	}
	// Build pela metade: erro em cada chamada.
	conta.ChavePublica = base64.StdEncoding.EncodeToString(pub)
	if r := chamar("vagas_listar", map[string]any{}); !strings.Contains(r, `"isError":true`) && !strings.Contains(r, `"error"`) {
		t.Fatalf("build pela metade = %s", r)
	}
}
