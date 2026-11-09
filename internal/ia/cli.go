package ia

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Modo "assinatura": roda a CLI oficial (Claude Code, Codex do ChatGPT, Gemini CLI) com o login da pessoa.
// Sempre com argumentos fixos (sem shell), sem ferramentas, numa pasta temporária vazia, com tempo limite
// e saída limitada. A Central só manda texto; nunca comandos. O texto da vaga vai pela entrada padrão, nunca
// pela linha de comando (jc-a2y.25). No Windows, uma CLI instalada como .cmd/.bat roda dentro do cmd.exe, que
// não respeita o escape do Go: ela só roda se todos os argumentos forem seguros para o cmd.exe.

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
	Pronta    bool   `json:"pronta"`          // instalada e num formato que o app usa com segurança
	Aviso     string `json:"aviso,omitempty"` // texto fixo quando não está pronta
	// Motivo, só quando instalada e não pronta: MotivoFormato (instalar a versão nativa resolve) ou MotivoPolitica
	// (fora do app por regra de segurança; o caminho é a chave de API). A tela escolhe a ação por ele (jc-a2y.42).
	Motivo string `json:"motivo,omitempty"`
}

// Motivos de uma CLI instalada que não está pronta (EstadoCLI.Motivo).
const (
	MotivoFormato  = "formato"
	MotivoPolitica = "politica"
)

// aceitaCmd: CLIs cujos argumentos são sempre seguros para o cmd.exe (o pedido vai pelo stdin). O Claude leva o
// prompt de sistema (várias linhas) e o esquema (com aspas) no argv, então instalado como .cmd não roda (jc-a2y.25).
func aceitaCmd(comando string) bool { return comando != "claude" }

const limiteSaida = 4 << 20

type cli struct{ qual string }

func (c cli) Nome() string { return c.qual + "-cli" }

// rodar executa a CLI com tempo limite numa pasta vazia. A chave de API do ambiente é removida:
// no modo assinatura a CLI usa o login da pessoa, nunca uma chave de API cobrada à parte.
func rodar(ctx context.Context, comando string, args []string, entrada string) ([]byte, string, error) {
	// O arquivo conferido é o mesmo que roda: o caminho resolvido pelo LookPath vai para o CommandContext.
	alvo, err := exec.LookPath(comando)
	if err != nil {
		alvo = comando // o exec devolve o erro de "não achei" na hora de rodar
	}
	if viaCmdExe(alvo) && !argsSegurosParaCmd(args) {
		return nil, "", &ErroIA{Tipo: "config", Mensagem: comando + "-cli: " + msgCmdInseguro, Provedor: comando + "-cli"}
	}
	ctx, cancelar := context.WithTimeout(ctx, tempoLimite)
	defer cancelar()
	pasta, err := os.MkdirTemp("", "central-ia-")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(pasta)
	cmd := exec.CommandContext(ctx, alvo, args...)
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

const msgCmdInseguro = "esta CLI está instalada como .cmd, e o app não passa o pedido pelo cmd.exe; instale a versão nativa (.exe)"

// viaCmdExe: no Windows, .cmd e .bat rodam dentro do cmd.exe.
func viaCmdExe(caminho string) bool {
	ext := strings.ToLower(filepath.Ext(caminho))
	return ext == ".cmd" || ext == ".bat"
}

// argsSegurosParaCmd: só letras, dígitos, espaço e . , _ : / \ = + @ ~ -. Nada de aspas, quebra de linha,
// & | < > ^ % ! ( ) nem crase, que o cmd.exe interpreta. Argumento vazio pode.
func argsSegurosParaCmd(args []string) bool {
	for _, a := range args {
		for _, r := range a {
			// Letra modificadora (Lm) e símbolo modificador (Sk), como ʺ e ˆ, podem virar " e ^ na conversão para ANSI.
			if unicode.Is(unicode.Lm, r) || unicode.Is(unicode.Sk, r) {
				return false
			}
			if !(unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(` .,_:/\=+@~-`, r)) {
				return false
			}
		}
	}
	return true
}

// ModeloSeguro diz se o nome do modelo pode ir como argumento de CLI (--model), com a mesma lista segura do cmd.exe
// (jc-a2y.43). A API recusa o resto ao configurar, para o modelo nunca chegar à linha de comando com outro caractere.
// Também não pode começar com "-": vindo logo depois de --model, a CLI poderia ler o nome como opção.
func ModeloSeguro(modelo string) bool {
	return !strings.HasPrefix(modelo, "-") && argsSegurosParaCmd([]string{modelo})
}

// MsgModeloInseguro é o texto fixo da recusa do nome do modelo.
const MsgModeloInseguro = "o nome do modelo não pode começar com - e só pode ter letras, números, espaço e . , _ : / \\ = + @ ~ -"

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

// semTravaDeFerramentas: CLIs que o app ainda não sabe rodar sem ferramentas, MCP e configuração da pessoa
// (jc-a2y.26). O Claude roda com --tools "", --strict-mcp-config e --setting-sources project. Codex e Gemini ficam
// indisponíveis até as travas equivalentes serem conferidas no --help das versões instaladas e testadas com um
// arquivo canário: lendo texto de vaga, uma IA com ferramentas poderia ler arquivo da pessoa ou chamar MCP.
var semTravaDeFerramentas = map[string]bool{"codex": true, "gemini": true}

const msgSemTrava = "esta CLI ainda não roda no app sem as ferramentas dela; use o Claude ou uma chave de API"

// CLIIndisponivel diz se o provedor (ex.: "gemini-cli") está fora do app e por quê (texto fixo).
func CLIIndisponivel(provedor string) (string, bool) {
	if semTravaDeFerramentas[strings.TrimSuffix(provedor, "-cli")] && strings.HasSuffix(provedor, "-cli") {
		return msgSemTrava, true
	}
	return "", false
}

func (c cli) Chamar(ctx context.Context, p PedidoProvedor) (RespostaProvedor, error) {
	nome := c.Nome()
	if msg, fora := CLIIndisponivel(nome); fora {
		return RespostaProvedor{}, &ErroIA{Tipo: "config", Mensagem: nome + ": " + msg, Provedor: nome}
	}
	// Confere de novo na hora de rodar: a configuração pode ter sido gravada por fora do PUT /ia/config (jc-a2y.43).
	if p.Modelo != "" && !ModeloSeguro(p.Modelo) {
		return RespostaProvedor{}, &ErroIA{Tipo: "config", Mensagem: nome + ": " + MsgModeloInseguro, Provedor: nome}
	}
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
			return RespostaProvedor{}, falhaDaCLI("claude-cli", stderr, err)
		}
		return RespostaProvedor{}, &ErroIA{Tipo: "provedor", Mensagem: "claude-cli: resposta não é JSON", Provedor: "claude-cli", Reintentavel: true}
	}
	if r["is_error"] == true {
		// O texto do erro só escolhe a classe; ele não vai para a mensagem.
		if pedeLogin(dominio.Texto(r["result"]) + " " + dominio.Texto(r["subtype"])) {
			return RespostaProvedor{}, &ErroIA{Tipo: "provedor", Mensagem: "claude-cli: " + msgSemLogin, Provedor: "claude-cli"}
		}
		return RespostaProvedor{}, &ErroIA{Tipo: "provedor", Mensagem: "claude-cli: " + msgRespondeuErro, Provedor: "claude-cli", Reintentavel: true}
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
		return RespostaProvedor{}, falhaDaCLI("codex-cli", stderr, err)
	}
	return RespostaProvedor{Texto: string(texto)}, nil
}

// Gemini CLI — não testado nesta versão: -p sem interação, saída JSON com o campo "response".
func chamarGemini(ctx context.Context, p PedidoProvedor) (RespostaProvedor, error) {
	// O pedido (com o texto da vaga) vai pela entrada padrão; o -p só leva um texto fixo, que o Gemini junta ao fim dela.
	args := []string{"-p", "Answer the request given on standard input.", "--output-format", "json"}
	if p.Modelo != "" {
		args = append(args, "--model", p.Modelo)
	}
	out, stderr, err := rodar(ctx, "gemini", args, p.Prompt.Sistema+"\n\n"+p.Prompt.Mensagem)
	var r map[string]any
	if json.Unmarshal(out, &r) != nil || r["response"] == nil {
		return RespostaProvedor{}, falhaDaCLI("gemini-cli", stderr, err)
	}
	return RespostaProvedor{Texto: dominio.Texto(r["response"])}, nil
}

// Mensagens fixas das falhas das CLIs. O stderr e o erro do processo podem ter email da conta, caminho de pasta
// com o nome do usuário ou outra mensagem de fora: eles só escolhem a classe e nunca entram na mensagem (jc-a2y.21).
const (
	msgSemLogin      = "sem login na CLI (abra a CLI no terminal e entre na sua conta)"
	msgRespondeuErro = "a CLI respondeu com erro (rode a CLI no terminal para ver o detalhe)"
	msgNaoRodou      = "não foi possível rodar a CLI"
	msgSemResposta   = "a CLI não respondeu"
)

// pedeLogin diz se o texto da CLI fala de login. Serve só para escolher a mensagem fixa.
func pedeLogin(texto string) bool {
	t := strings.ToLower(texto)
	for _, p := range []string{"login", "log in", "logged in", "sign in", "authenticat", "unauthorized"} {
		if strings.Contains(t, p) {
			return true
		}
	}
	return false
}

// falhaDaCLI transforma a falha do processo numa mensagem fixa: sem login, código de saída, não rodou ou sem resposta.
// Erro que o próprio rodar já classificou (tempo esgotado, .cmd inseguro) passa como está.
func falhaDaCLI(nome, stderr string, err error) *ErroIA {
	var saiu *exec.ExitError
	var ja *ErroIA
	switch {
	case errors.As(err, &ja):
		return ja
	case pedeLogin(stderr):
		return &ErroIA{Tipo: "provedor", Mensagem: nome + ": " + msgSemLogin, Provedor: nome}
	case errors.As(err, &saiu):
		return &ErroIA{Tipo: "provedor", Mensagem: fmt.Sprintf("%s: a CLI saiu com código %d (rode a CLI no terminal para ver o detalhe)", nome, saiu.ExitCode()), Provedor: nome, Reintentavel: true}
	case err != nil:
		return &ErroIA{Tipo: "provedor", Mensagem: nome + ": " + msgNaoRodou, Provedor: nome, Reintentavel: true}
	}
	return &ErroIA{Tipo: "provedor", Mensagem: nome + ": " + msgSemResposta, Provedor: nome, Reintentavel: true}
}

// DetectarCLIs diz quais CLIs estão instaladas e, quando dá para saber sem gastar cota, se há login.
func DetectarCLIs(ctx context.Context) []EstadoCLI {
	saida := make([]EstadoCLI, 0, len(CLIs))
	for _, info := range CLIs {
		e := EstadoCLI{InfoCLI: info}
		if caminho, err := exec.LookPath(info.Comando); err == nil {
			e.Instalada, e.Pronta = true, true
			if msg, fora := CLIIndisponivel(info.Provedor); fora {
				e.Pronta, e.Aviso, e.Motivo = false, msg, MotivoPolitica
			} else if viaCmdExe(caminho) && !aceitaCmd(info.Comando) {
				e.Pronta, e.Aviso, e.Motivo = false, msgCmdInseguro, MotivoFormato
			}
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
