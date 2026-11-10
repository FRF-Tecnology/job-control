// Package mcp é o servidor MCP local do Applyer (jc-a2y.7.4): JSON-RPC 2.0 sobre stdio, uma mensagem por linha, sem
// dependências, como o da Node (mcp/servidor.mjs). O stdout é só do protocolo (A8 do CyberSec); o log vai para o
// stderr, só com a ferramenta e o resultado (A6). Cada tools/call abre a pasta e o banco e fecha no fim (A2.3).
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/api"
	"github.com/FRF-Tecnology/job-control/internal/assistente"
)

// versoesDoProtocolo: as que o servidor fala, da mais nova para a mais antiga.
var versoesDoProtocolo = []string{"2025-11-25", "2025-06-18", "2025-03-26", "2024-11-05"}

// maxLinha: o maior pedido aceito (bytes). Acima disso, a linha é descartada com erro de JSON.
const maxLinha = 1 << 20

// Servidor do MCP.
type Servidor struct {
	// Abrir abre a pasta de dados e o banco para UMA chamada e devolve a API do assistente e quem fecha.
	Abrir  func(ctx context.Context) (*api.API, func(), error)
	Versao string
	Log    func(formato string, args ...any) // padrão: log.Printf (stderr)
}

type mensagem struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  *string         `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func resposta(id json.RawMessage, resultado any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "result": resultado}
}

func erro(id json.RawMessage, codigo int, msg string) map[string]any {
	if id == nil {
		id = json.RawMessage("null")
	}
	return map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": codigo, "message": msg}}
}

func (s *Servidor) log(f string, a ...any) {
	if s.Log != nil {
		s.Log(f, a...)
		return
	}
	log.Printf(f, a...)
}

// Servir lê do in, uma mensagem por linha, e escreve as respostas no out, na ordem.
func (s *Servidor) Servir(ctx context.Context, in io.Reader, out io.Writer) error {
	r := bufio.NewReaderSize(in, 64*1024)
	w := bufio.NewWriter(out)
	for {
		linha, longa, err := lerLinha(r)
		if len(linha) > 0 || longa {
			var saidas []any
			if longa {
				saidas = []any{erro(nil, -32700, "pedido grande demais")}
			} else {
				saidas = s.Receber(ctx, linha)
			}
			for _, x := range saidas {
				b, _ := json.Marshal(x)
				w.Write(b)
				w.WriteByte('\n')
			}
			if err := w.Flush(); err != nil {
				return err
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// lerLinha devolve a próxima linha (sem o \n e sem espaços) e se ela passou de maxLinha (descartada).
func lerLinha(r *bufio.Reader) ([]byte, bool, error) {
	var b []byte
	longa := false
	for {
		pedaco, prefixo, err := r.ReadLine()
		if !longa {
			b = append(b, pedaco...)
			if len(b) > maxLinha {
				longa, b = true, nil
			}
		}
		if err != nil || !prefixo {
			return trim(b), longa, err
		}
	}
}

func trim(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\t' || b[0] == '\r') {
		b = b[1:]
	}
	for len(b) > 0 && (b[len(b)-1] == ' ' || b[len(b)-1] == '\t' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}

// Receber trata uma linha (uma mensagem ou uma lista) e devolve as respostas a escrever.
func (s *Servidor) Receber(ctx context.Context, linha []byte) []any {
	var bruto json.RawMessage
	if json.Unmarshal(linha, &bruto) != nil {
		return []any{erro(nil, -32700, "JSON inválido")}
	}
	var lista []json.RawMessage
	if len(bruto) > 0 && bruto[0] == '[' {
		if json.Unmarshal(bruto, &lista) != nil || len(lista) == 0 {
			return []any{erro(nil, -32600, "pedido inválido")}
		}
	} else {
		lista = []json.RawMessage{bruto}
	}
	var saidas []any
	for _, m := range lista {
		if r := s.tratar(ctx, m); r != nil {
			saidas = append(saidas, r)
		}
	}
	return saidas
}

func (s *Servidor) tratar(ctx context.Context, bruto json.RawMessage) any {
	var m mensagem
	if json.Unmarshal(bruto, &m) != nil || m.JSONRPC != "2.0" || m.Method == nil {
		if m.ID != nil {
			return erro(m.ID, -32600, "pedido inválido")
		}
		return nil
	}
	notificacao := m.ID == nil
	switch *m.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		json.Unmarshal(m.Params, &p)
		versao := versoesDoProtocolo[1]
		for _, v := range versoesDoProtocolo {
			if v == p.ProtocolVersion {
				versao = v
			}
		}
		return resposta(m.ID, map[string]any{
			"protocolVersion": versao,
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "applyer", "title": "Applyer", "version": s.Versao},
			"instructions":    assistente.Instrucoes,
		})
	case "notifications/initialized", "notifications/cancelled":
		return nil
	case "ping":
		return resposta(m.ID, map[string]any{})
	case "tools/list":
		ferramentas := []any{}
		for _, f := range assistente.Ferramentas {
			ferramentas = append(ferramentas, map[string]any{
				"name": f.Nome, "title": f.Titulo, "description": f.Descricao, "inputSchema": f.Entrada,
				"annotations": map[string]any{"title": f.Titulo, "readOnlyHint": f.SomenteLeitura, "destructiveHint": false, "openWorldHint": false},
			})
		}
		return resposta(m.ID, map[string]any{"tools": ferramentas})
	case "tools/call":
		return s.chamarFerramenta(ctx, m)
	}
	if notificacao {
		return nil
	}
	return erro(m.ID, -32601, "método não suportado")
}

func textoDeResultado(t string, falhou bool) map[string]any {
	o := map[string]any{"content": []any{map[string]any{"type": "text", "text": t}}}
	if falhou {
		o["isError"] = true
	}
	return o
}

func (s *Servidor) chamarFerramenta(ctx context.Context, m mensagem) any {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if json.Unmarshal(m.Params, &p) != nil {
		return erro(m.ID, -32602, "parâmetros inválidos")
	}
	f, ok := assistente.Achar(p.Name)
	if !ok {
		return erro(m.ID, -32602, "ferramenta desconhecida") // sem ecoar o nome pedido
	}
	args := map[string]any{}
	if len(p.Arguments) > 0 && string(p.Arguments) != "null" {
		if json.Unmarshal(p.Arguments, &args) != nil {
			s.log("mcp: %s recusado (argumentos não são um objeto)", f.Nome)
			return resposta(m.ID, textoDeResultado("Recusado: os argumentos têm de ser um objeto", true))
		}
	}
	// O schema antes do banco: entrada errada nem abre a pasta (A8).
	if problemas := assistente.Conferir(f.Entrada, args); len(problemas) > 0 {
		s.log("mcp: %s recusado (argumentos)", f.Nome)
		return resposta(m.ID, textoDeResultado("Recusado: argumentos inválidos: "+strings.Join(problemas, "; "), true))
	}
	a, fechar, err := s.Abrir(ctx)
	if err != nil {
		s.log("mcp: %s sem dados (%T)", f.Nome, err)
		return resposta(m.ID, textoDeResultado("Recusado: "+err.Error(), true)) // as mensagens da pasta não têm caminho
	}
	defer fechar()
	r, err := assistente.Rodar(ctx, a, f, args)
	var ef *assistente.ErroFerramenta
	switch {
	case errors.As(err, &ef):
		s.log("mcp: %s recusado", f.Nome)
		return resposta(m.ID, textoDeResultado("Recusado: "+ef.Msg, true))
	case err != nil:
		s.log("mcp: %s erro (%T)", f.Nome, err)
		return resposta(m.ID, textoDeResultado("Erro interno do Applyer; tente de novo", true))
	}
	s.log("mcp: %s ok", f.Nome)
	return resposta(m.ID, textoDeResultado(assistente.Texto(r), false))
}
