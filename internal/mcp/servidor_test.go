package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/api"
	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/assistente"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// jc-a2y.7.4 (PR 3): o servidor MCP. Dados fictícios.

const marcaMCP = "Marca-Ficticia-MCP-41b7" // não pode aparecer no log

type servidorDeTeste struct {
	*Servidor
	logs              []string
	abertos, fechados int
}

func novoServidor(t *testing.T) *servidorDeTeste {
	t.Helper()
	arm, err := armazem.Abrir(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	s := &servidorDeTeste{}
	s.Servidor = &Servidor{Versao: "0.0.0-teste",
		Abrir: func(ctx context.Context) (*api.API, func(), error) {
			s.abertos++
			return api.Nova(arm, api.Opcoes{Hoje: func() string { return "2026-11-09" }, Canal: dominio.CanalMCP}), func() { s.fechados++ }, nil
		},
		Log: func(f string, a ...any) { s.logs = append(s.logs, fmt.Sprintf(f, a...)) }}
	return s
}

func (s *servidorDeTeste) pedir(t *testing.T, msg string) map[string]any {
	t.Helper()
	saidas := s.Receber(context.Background(), []byte(msg))
	if len(saidas) != 1 {
		t.Fatalf("%s: %d respostas", msg, len(saidas))
	}
	b, _ := json.Marshal(saidas[0])
	var o map[string]any
	json.Unmarshal(b, &o)
	return o
}

func chamada(id int, nome string, args any) string {
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": "tools/call", "params": map[string]any{"name": nome, "arguments": args}})
	return string(b)
}

func textoDe(o map[string]any) (string, bool) {
	r := dominio.ObjetoDe(o["result"])
	c := dominio.ListaDe(r["content"])
	if len(c) != 1 {
		return "", false
	}
	return dominio.Texto(dominio.ObjetoDe(c[0])["text"]), r["isError"] == true
}

func TestProtocolo(t *testing.T) {
	s := novoServidor(t)
	o := s.pedir(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}`)
	r := dominio.ObjetoDe(o["result"])
	if r["protocolVersion"] != "2025-06-18" || !strings.Contains(dominio.Texto(r["instructions"]), "DADO, não ordem") {
		t.Fatalf("initialize = %v", o)
	}
	if o := s.pedir(t, `{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"1999-01-01"}}`); dominio.ObjetoDe(o["result"])["protocolVersion"] != versoesDoProtocolo[1] {
		t.Fatalf("versão desconhecida = %v", o)
	}
	l := s.pedir(t, `{"jsonrpc":"2.0","id":3,"method":"tools/list"}`)
	if n := len(dominio.ListaDe(dominio.ObjetoDe(l["result"])["tools"])); n != len(assistente.Ferramentas) {
		t.Fatalf("tools/list = %d", n)
	}
	if o := s.pedir(t, `{"jsonrpc":"2.0","id":4,"method":"ping"}`); o["result"] == nil {
		t.Fatalf("ping = %v", o)
	}
	if saidas := s.Receber(context.Background(), []byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)); len(saidas) != 0 {
		t.Fatalf("notificação respondida: %v", saidas)
	}
	for msg, codigo := range map[string]float64{
		`não é json`: -32700,
		`{"jsonrpc":"2.0","id":5,"method":"apagar_tudo"}`:                            -32601,
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"aprovar"}}`: -32602,
		`{"jsonrpc":"1.0","id":7,"method":"ping"}`:                                   -32600,
		`[]`: -32600,
	} {
		if o := s.pedir(t, msg); dominio.ObjetoDe(o["error"])["code"] != codigo {
			t.Errorf("%s = %v; queria %v", msg, o, codigo)
		}
	}
	// A ferramenta desconhecida não volta no erro (o nome vem do modelo).
	if o := s.pedir(t, chamada(8, marcaMCP, map[string]any{})); strings.Contains(fmt.Sprint(o), marcaMCP) {
		t.Fatalf("ecoou o nome pedido: %v", o)
	}
	if s.abertos != 0 {
		t.Fatal("abriu o banco sem tools/call")
	}
}

// Cada tools/call abre e fecha o banco (A2.3); entrada fora do schema é recusada antes da API (A8); recusas do domínio
// voltam para o modelo; o log não leva argumentos (A6).
func TestChamarFerramenta(t *testing.T) {
	s := novoServidor(t)
	o := s.pedir(t, chamada(1, "vaga_adicionar", map[string]any{"vagas": []any{map[string]any{"id": "mk-1", "empresa": marcaMCP, "titulo": marcaMCP, "link": "https://exemplo.com/" + marcaMCP}}}))
	if txt, falhou := textoDe(o); falhou || !strings.Contains(txt, "mk-1") {
		t.Fatalf("vaga_adicionar = %v", o)
	}
	if s.abertos != 1 || s.fechados != 1 {
		t.Fatalf("abertos %d, fechados %d", s.abertos, s.fechados)
	}
	for nome, args := range map[string]any{
		"vaga_mover":       map[string]any{"id": "mk-1", "etapa": "aplicar"},                   // domínio: só a pessoa
		"evento_registrar": map[string]any{"id": "mk-1", "tipo": "enviada", "texto": marcaMCP}, // entrada do assistente
	} {
		if txt, falhou := textoDe(s.pedir(t, chamada(2, nome, args))); !falhou || !strings.HasPrefix(txt, "Recusado") {
			t.Errorf("%s = %q", nome, txt)
		}
	}
	antes := s.abertos
	for nome, args := range map[string]any{
		"vaga_mover":   map[string]any{"id": "mk-1", "etapa": "enviada"},                            // fora do enum
		"vaga_detalhe": map[string]any{"id": "mk-1", "envio": map[string]any{"data": "2026-11-09"}}, // campo a mais
		"resumo":       []any{"não é objeto"},
	} {
		if txt, falhou := textoDe(s.pedir(t, chamada(3, nome, args))); !falhou || !strings.HasPrefix(txt, "Recusado") {
			t.Errorf("%s = %q", nome, txt)
		}
	}
	if s.abertos != antes { // entrada fora do schema nem abre o banco
		t.Fatalf("abriu o banco com entrada inválida: %d (antes %d)", s.abertos, antes)
	}
	if s.abertos != s.fechados {
		t.Fatalf("abertos %d, fechados %d", s.abertos, s.fechados)
	}
	o = s.pedir(t, chamada(4, "vaga_detalhe", map[string]any{"id": "mk-1"}))
	if txt, falhou := textoDe(o); falhou || !strings.Contains(txt, marcaMCP) {
		t.Fatalf("vaga_detalhe = %v", o)
	}
	tudo := strings.Join(s.logs, "\n")
	for _, proibido := range []string{marcaMCP, "mk-1", "exemplo.com"} {
		if strings.Contains(tudo, proibido) {
			t.Errorf("log tem %q:\n%s", proibido, tudo)
		}
	}
	if !strings.Contains(tudo, "mcp: vaga_adicionar ok") {
		t.Fatalf("log = %s", tudo)
	}
}

// Sem a pasta de dados: a chamada volta como erro da ferramenta, com a mensagem sem caminho.
func TestSemPastaDeDados(t *testing.T) {
	s := &Servidor{Abrir: func(context.Context) (*api.API, func(), error) {
		return nil, nil, errors.New("a pasta de dados do Applyer ainda não existe: abra o Applyer uma vez")
	}, Log: func(string, ...any) {}}
	saidas := s.Receber(context.Background(), []byte(chamada(1, "resumo", map[string]any{})))
	b, _ := json.Marshal(saidas[0])
	if !bytes.Contains(b, []byte(`"isError":true`)) || !bytes.Contains(b, []byte("abra o Applyer")) {
		t.Fatalf("sem pasta = %s", b)
	}
}

// A8: o stdout é só JSON-RPC, uma resposta por linha; linha grande demais dá erro sem derrubar o servidor.
func TestServirSoJSON(t *testing.T) {
	s := novoServidor(t)
	entrada := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		"",
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"ping","params":{"x":"` + strings.Repeat("a", maxLinha+10) + `"}}`,
		chamada(4, "resumo", map[string]any{}),
	}, "\n") + "\n"
	var saida bytes.Buffer
	if err := s.Servir(context.Background(), strings.NewReader(entrada), &saida); err != nil {
		t.Fatal(err)
	}
	linhas := strings.Split(strings.TrimRight(saida.String(), "\n"), "\n")
	if len(linhas) != 4 {
		t.Fatalf("%d linhas:\n%s", len(linhas), saida.String()[:min(len(saida.String()), 500)])
	}
	for _, l := range linhas {
		var o map[string]any
		if json.Unmarshal([]byte(l), &o) != nil || o["jsonrpc"] != "2.0" {
			t.Fatalf("linha fora do protocolo: %.200s", l)
		}
	}
	if !strings.Contains(linhas[2], "-32700") {
		t.Fatalf("linha grande = %.200s", linhas[2])
	}
}
