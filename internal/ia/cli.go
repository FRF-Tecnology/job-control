package ia

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Modo "assinatura": roda a CLI oficial (Claude Code, Codex do ChatGPT, Gemini CLI) com o login da pessoa.
// Sempre com argumentos fixos (sem shell), sem ferramentas, numa pasta temporária vazia, com tempo limite
// e saída limitada. A Central só manda texto; nunca comandos.

// CLIs conhecidas. Verificada = testada de verdade com a versão indicada.
var CLIs = []InfoCLI{
	{Provedor: "claude-cli", Comando: "claude", Nome: "Claude", Plano: "Claude Pro ou Max", Verificada: true,
		Instalar: "https://docs.claude.com/claude-code"},
	{Provedor: "codex-cli", Comando: "codex", Nome: "ChatGPT", Plano: "ChatGPT Plus ou Pro",
		Instalar: "https://github.com/openai/codex"},
	{Provedor: "gemini-cli", Comando: "gemini", Nome: "Gemini", Plano: "Google AI Pro ou Ultra",
		Instalar: "https://github.com/google-gemini/gemini-cli"},
}

// InfoCLI descreve uma CLI de assinatura.
type InfoCLI struct {
	Provedor   string `json:"provedor"`
	Comando    string `json:"comando"`
	Nome       string `json:"nome"`
	Plano      string `json:"plano"`
	Instalar   string `json:"instalar"`
	Verificada bool   `json:"verificada"`
}

// EstadoCLI é o que a tela Conectar IA mostra de cada CLI.
type EstadoCLI struct {
	InfoCLI
	Instalada bool   `json:"instalada"`
	Logada    *bool  `json:"logada"` // nil = não dá para saber sem chamar a IA
	Versao    string `json:"versao,omitempty"`
}

const limiteSaida = 4 << 20

type cli struct{ qual string }

func (c cli) Nome() string { return c.qual + "-cli" }

// rodar executa a CLI com tempo limite numa pasta vazia. A chave de API do ambiente é removida:
// no modo assinatura a CLI usa o login da pessoa, nunca uma chave de API cobrada à parte.
func rodar(ctx context.Context, comando string, args []string, entrada string) ([]byte, string, error) {
	ctx, cancelar := context.WithTimeout(ctx, tempoLimite)
	defer cancelar()
	pasta, err := os.MkdirTemp("", "central-ia-")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(pasta)
	cmd := exec.CommandContext(ctx, comando, args...)
	cmd.Dir = pasta
	cmd.Stdin = strings.NewReader(entrada)
	var saida, erro limitado
	saida.max, erro.max = limiteSaida, 64<<10
	cmd.Stdout, cmd.Stderr = &saida, &erro
	cmd.Env = semChaves(os.Environ())
	err = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, "", &ErroIA{Tipo: "provedor", Mensagem: comando + ": passou do tempo limite", Provedor: comando + "-cli", Reintentavel: true}
	}
	return saida.Bytes(), strings.TrimSpace(erro.String()), err
}

func semChaves(env []string) []string {
	saida := env[:0:0]
	for _, v := range env {
		if strings.HasPrefix(v, "ANTHROPIC_API_KEY=") || strings.HasPrefix(v, "OPENAI_API_KEY=") || strings.HasPrefix(v, "GEMINI_API_KEY=") || strings.HasPrefix(v, "GOOGLE_API_KEY=") {
			continue
		}
		saida = append(saida, v)
	}
	return saida
}

type limitado struct {
	bytes.Buffer
	max int
}

func (l *limitado) Write(p []byte) (int, error) {
	if l.Len()+len(p) > l.max {
		return 0, errors.New("saída grande demais")
	}
	return l.Buffer.Write(p)
}

func erroCLI(provedor, msg string) error {
	return &ErroIA{Tipo: "provedor", Mensagem: provedor + ": " + msg, Provedor: provedor}
}

func (c cli) Chamar(ctx context.Context, p PedidoProvedor) (RespostaProvedor, error) {
	nome := c.Nome()
	if _, err := exec.LookPath(c.qual); err != nil {
		return RespostaProvedor{}, &ErroIA{Tipo: "config", Mensagem: nome + ": a CLI não está instalada neste computador", Provedor: nome}
	}
	switch c.qual {
	case "claude":
		return chamarClaude(ctx, p)
	case "codex":
		return chamarCodex(ctx, p)
	case "gemini":
		return chamarGemini(ctx, p)
	}
	return RespostaProvedor{}, erroCLI(nome, "CLI desconhecida")
}

// Claude Code: -p sem interação, saída JSON com structured_output no esquema pedido, sem ferramentas nem MCP.
func chamarClaude(ctx context.Context, p PedidoProvedor) (RespostaProvedor, error) {
	args := []string{"-p", "--output-format", "json", "--json-schema", string(p.Prompt.Esquema), "--system-prompt", p.Prompt.Sistema,
		"--tools", "", "--no-session-persistence", "--strict-mcp-config",
		// Tarefas curtas e objetivas: esforço baixo responde mais rápido. Só as configurações do projeto (a pasta
		// temporária não tem nenhuma): ganchos e plugins da instalação da pessoa não rodam a cada chamada.
		"--effort", "low", "--setting-sources", "project"}
	if p.Modelo != "" {
		args = append(args, "--model", p.Modelo)
	}
	out, stderr, err := rodar(ctx, "claude", args, p.Prompt.Mensagem)
	var r map[string]any
	if jerr := json.Unmarshal(out, &r); jerr != nil {
		if err != nil {
			return RespostaProvedor{}, &ErroIA{Tipo: "provedor", Mensagem: "claude-cli: " + primeiraLinha(stderr, err), Provedor: "claude-cli", Reintentavel: true}
		}
		return RespostaProvedor{}, &ErroIA{Tipo: "provedor", Mensagem: "claude-cli: resposta não é JSON", Provedor: "claude-cli", Reintentavel: true}
	}
	if r["is_error"] == true {
		msg := dominio.Texto(r["result"])
		if msg == "" {
			msg = dominio.Texto(r["subtype"])
		}
		return RespostaProvedor{}, &ErroIA{Tipo: "provedor", Mensagem: "claude-cli: " + msg, Provedor: "claude-cli", Reintentavel: !strings.Contains(strings.ToLower(msg), "login")}
	}
	texto := dominio.Texto(r["result"])
	if s, ok := r["structured_output"]; ok && s != nil {
		b, _ := json.Marshal(s)
		texto = string(b)
	}
	u := dominio.ObjetoDe(r["usage"])
	entrada := inteiro(u["input_tokens"])
	if entrada != nil { // o cache também é entrada
		for _, k := range []string{"cache_creation_input_tokens", "cache_read_input_tokens"} {
			if n := inteiro(u[k]); n != nil {
				*entrada += *n
			}
		}
	}
	return RespostaProvedor{Texto: texto, TokensEntrada: entrada, TokensSaida: inteiro(u["output_tokens"])}, nil
}

// Codex (ChatGPT) — não testado nesta versão: exec sem interação, só leitura, última mensagem em arquivo.
func chamarCodex(ctx context.Context, p PedidoProvedor) (RespostaProvedor, error) {
	pasta, err := os.MkdirTemp("", "central-codex-")
	if err != nil {
		return RespostaProvedor{}, err
	}
	defer os.RemoveAll(pasta)
	esquema, ultima := filepath.Join(pasta, "esquema.json"), filepath.Join(pasta, "resposta.txt")
	os.WriteFile(esquema, p.Prompt.Esquema, 0o600)
	args := []string{"exec", "--skip-git-repo-check", "--sandbox", "read-only", "--color", "never", "--output-schema", esquema, "--output-last-message", ultima}
	if p.Modelo != "" {
		args = append(args, "--model", p.Modelo)
	}
	args = append(args, "-")
	_, stderr, err := rodar(ctx, "codex", args, p.Prompt.Sistema+"\n\n"+p.Prompt.Mensagem)
	texto, lerr := os.ReadFile(ultima)
	if lerr != nil || len(bytes.TrimSpace(texto)) == 0 {
		return RespostaProvedor{}, &ErroIA{Tipo: "provedor", Mensagem: "codex-cli: " + primeiraLinha(stderr, err), Provedor: "codex-cli", Reintentavel: true}
	}
	return RespostaProvedor{Texto: string(texto)}, nil
}

// Gemini CLI — não testado nesta versão: -p sem interação, saída JSON com o campo "response".
func chamarGemini(ctx context.Context, p PedidoProvedor) (RespostaProvedor, error) {
	args := []string{"-p", p.Prompt.Sistema + "\n\n" + p.Prompt.Mensagem, "--output-format", "json"}
	if p.Modelo != "" {
		args = append(args, "--model", p.Modelo)
	}
	out, stderr, err := rodar(ctx, "gemini", args, "")
	var r map[string]any
	if json.Unmarshal(out, &r) != nil || r["response"] == nil {
		return RespostaProvedor{}, &ErroIA{Tipo: "provedor", Mensagem: "gemini-cli: " + primeiraLinha(stderr, err), Provedor: "gemini-cli", Reintentavel: true}
	}
	return RespostaProvedor{Texto: dominio.Texto(r["response"])}, nil
}

func primeiraLinha(stderr string, err error) string {
	if stderr != "" {
		l, _, _ := strings.Cut(stderr, "\n")
		return cortarUTF16(l, 200)
	}
	if err != nil {
		return err.Error()
	}
	return "sem resposta"
}

// DetectarCLIs diz quais CLIs estão instaladas e, quando dá para saber sem gastar cota, se há login.
func DetectarCLIs(ctx context.Context) []EstadoCLI {
	saida := make([]EstadoCLI, 0, len(CLIs))
	for _, info := range CLIs {
		e := EstadoCLI{InfoCLI: info}
		if _, err := exec.LookPath(info.Comando); err == nil {
			e.Instalada = true
			c, cancelar := context.WithTimeout(ctx, 15*time.Second)
			if out, err := exec.CommandContext(c, info.Comando, "--version").Output(); err == nil {
				e.Versao = strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
			}
			switch info.Comando {
			case "claude":
				if out, err := exec.CommandContext(c, "claude", "auth", "status", "--json").Output(); err == nil {
					var s struct {
						LoggedIn bool `json:"loggedIn"`
					}
					if json.Unmarshal(out, &s) == nil {
						e.Logada = &s.LoggedIn
					}
				}
			case "codex":
				ok := exec.CommandContext(c, "codex", "login", "status").Run() == nil
				e.Logada = &ok
			}
			cancelar()
		}
		saida = append(saida, e)
	}
	return saida
}
