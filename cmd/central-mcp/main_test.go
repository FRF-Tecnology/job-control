package main

import (
	"context"
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/config"
	"github.com/FRF-Tecnology/job-control/internal/mcp"
)

// jc-a2y.7.4 (PR 3). A2.4 (c) do CyberSec: o pacote main do MCP não chega ao cofre nem à IA por fora da API do
// assistente (que recusa o cofre e não tem IA): não importa internal/segredos nem internal/ia, nem cita
// segredos.Sistema.
func TestMainSemCofreNemIA(t *testing.T) {
	arquivos, _ := filepath.Glob("*.go")
	vistos := 0
	for _, a := range arquivos {
		if strings.HasSuffix(a, "_test.go") {
			continue
		}
		vistos++
		f, err := parser.ParseFile(token.NewFileSet(), a, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			if p := strings.Trim(imp.Path.Value, `"`); strings.HasSuffix(p, "/internal/segredos") || strings.HasSuffix(p, "/internal/ia") {
				t.Errorf("%s importa %s", a, p)
			}
		}
		b, _ := os.ReadFile(a)
		if strings.Contains(string(b), "segredos.") || strings.Contains(string(b), "Cofre:") {
			t.Errorf("%s fala do cofre", a)
		}
	}
	if vistos == 0 {
		t.Fatal("nenhum arquivo do main")
	}
}

// A2.3: entre as chamadas, o MCP não segura o banco: logo depois de uma tools/call, a pasta pode ser renomeada (o
// app renomeia a pasta antiga e apaga os dados com ela livre). Pasta fictícia, criada e protegida como o app faz.
func TestBancoLivreEntreAsChamadas(t *testing.T) {
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
	pedido, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "vaga_adicionar",
		"arguments": map[string]any{"vagas": []any{map[string]any{"id": "v-1", "empresa": "Empresa Teste", "titulo": "Dev Go", "link": "https://exemplo.com/1"}}}}})
	saidas := s.Receber(context.Background(), pedido)
	b, _ := json.Marshal(saidas)
	if strings.Contains(string(b), `"isError":true`) || !strings.Contains(string(b), "v-1") {
		t.Fatalf("vaga_adicionar = %s", b)
	}
	if err := os.Rename(pasta, pasta+"-renomeada"); err != nil {
		t.Fatalf("o MCP segurou a pasta depois da chamada: %v", err)
	}
	// Sem a pasta: a chamada volta como erro da ferramenta, sem caminho na mensagem, e nada é criado.
	saidas = s.Receber(context.Background(), pedido)
	b, _ = json.Marshal(saidas)
	if !strings.Contains(string(b), `"isError":true`) || strings.Contains(string(b), pasta) {
		t.Fatalf("sem pasta = %s", b)
	}
	if _, err := os.Stat(pasta); err == nil {
		t.Fatal("o MCP criou a pasta")
	}
}
