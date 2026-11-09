package ia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Provedor falso: devolve as respostas na ordem (a última se repete); erro no lugar de resposta simula queda.
type falso struct {
	nome     string
	passos   []any
	chamadas int
}

func (f *falso) Nome() string { return f.nome }
func (f *falso) Chamar(ctx context.Context, p PedidoProvedor) (RespostaProvedor, error) {
	passo := f.passos[min(f.chamadas, len(f.passos)-1)]
	f.chamadas++
	if err, ok := passo.(error); ok {
		return RespostaProvedor{}, err
	}
	b, _ := json.Marshal(passo)
	n := 10
	return RespostaProvedor{Texto: string(b), TokensEntrada: &n, TokensSaida: &n}, nil
}

var triagemOk = map[string]any{"decisao": "aplicar", "aderencia": "A", "motivo": "bate"}
var entradaTriagem = Objeto{"vaga": Objeto{"id": "li-1", "empresa": "Acme", "titulo": "Dev"}, "perfil": Objeto{}}

func iaCom(principal, reserva *falso, usos *[]Uso) *IA {
	rota := Rota{Provedor: principal.nome, Modelo: "claude-haiku-5-5"}
	if reserva != nil {
		rota.Reserva = &Rota{Provedor: reserva.nome, Modelo: "m2"}
	}
	a := Nova(Config{Rotas: map[string]Rota{"padrao": rota}}, nil, func(u Uso) { *usos = append(*usos, u) })
	a.Provedores = map[string]Provedor{principal.nome: principal}
	if reserva != nil {
		a.Provedores[reserva.nome] = reserva
	}
	return a
}

func TestReservaEntraQuandoOProvedorCai(t *testing.T) {
	var usos []Uso
	caiu := &ErroIA{Tipo: "provedor", Mensagem: "a: HTTP 503", Reintentavel: true}
	a := iaCom(&falso{nome: "a", passos: []any{caiu}}, &falso{nome: "b", passos: []any{triagemOk}}, &usos)
	s, err := a.Executar(context.Background(), "triarVaga", entradaTriagem)
	if err != nil || s["decisao"] != "aplicar" {
		t.Fatalf("saída %v, erro %v", s, err)
	}
	if len(usos) != 2 || usos[0].OK || usos[0].Erro != "provedor" || !usos[1].OK || usos[1].Provedor != "b" || usos[0].VagaID != "li-1" {
		t.Fatalf("usos = %+v", usos)
	}
	if usos[0].CustoUSD != nil || usos[1].CustoUSD != nil { // m2 fora da tabela; a falha não tem tokens
		t.Errorf("custos = %v %v", usos[0].CustoUSD, usos[1].CustoUSD)
	}
}

func TestErroQueNaoEReintentavelNaoTrocaDeProvedor(t *testing.T) {
	var usos []Uso
	reserva := &falso{nome: "b", passos: []any{triagemOk}}
	a := iaCom(&falso{nome: "a", passos: []any{map[string]any{"decisao": "x"}}}, reserva, &usos)
	_, err := a.Executar(context.Background(), "triarVaga", entradaTriagem)
	var e *ErroIA
	if !errors.As(err, &e) || e.Tipo != "esquema" || reserva.chamadas != 0 {
		t.Fatalf("erro %v; reserva chamada %d vezes", err, reserva.chamadas)
	}
}

func TestSemIAConectada(t *testing.T) {
	a := Nova(Config{}, nil, nil)
	_, err := a.Executar(context.Background(), "triarVaga", entradaTriagem)
	if err == nil || !strings.Contains(err.Error(), "Conectar IA") || a.Configurada() {
		t.Fatalf("erro = %v", err)
	}
}

func TestCustoMedidoPeloModelo(t *testing.T) {
	var usos []Uso
	a := iaCom(&falso{nome: "a", passos: []any{triagemOk}}, nil, &usos)
	if _, err := a.Executar(context.Background(), "triarVaga", entradaTriagem); err != nil {
		t.Fatal(err)
	}
	if usos[0].CustoUSD == nil || *usos[0].CustoUSD != 0.000006 {
		t.Fatalf("custo = %v", usos[0].CustoUSD)
	}
}

// ---------- CLI falsa: o próprio binário de teste faz o papel do "claude" ----------

func init() {
	if os.Getenv("CENTRAL_CLI_FALSA") == "" {
		return
	}
	// Modo CLI falsa: confere os argumentos e responde como o Claude Code.
	args := strings.Join(os.Args[1:], " ")
	entrada, _ := io.ReadAll(os.Stdin)
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		fmt.Print(`{"is_error":true,"result":"usou chave de API"}`)
		os.Exit(0)
	}
	if !strings.Contains(args, "--tools  --no-session-persistence") && !strings.Contains(args, `--tools "" --no-session-persistence`) {
		fmt.Print(`{"is_error":true,"result":"ferramentas não foram desligadas: ` + strings.ReplaceAll(args, `"`, `'`) + `"}`)
		os.Exit(0)
	}
	switch os.Getenv("CENTRAL_CLI_FALSA") {
	case "ok":
		if !strings.Contains(string(entrada), "Acme") {
			fmt.Print(`{"is_error":true,"result":"mensagem não chegou pela entrada"}`)
			os.Exit(0)
		}
		fmt.Print(`{"type":"result","subtype":"success","is_error":false,"result":"","structured_output":{"decisao":"pular","aderencia":"C","motivo":"sênior"},"usage":{"input_tokens":3,"cache_read_input_tokens":100,"output_tokens":20}}`)
	case "erro":
		fmt.Print(`{"type":"result","subtype":"error","is_error":true,"result":"Not logged in · Please run /login"}`)
	case "lixo":
		fmt.Fprint(os.Stderr, "falhou feio\nmais linhas")
		os.Exit(3)
	}
	os.Exit(0)
}

func comCLIFalsa(t *testing.T, modo string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	pasta := t.TempDir()
	nome := "claude"
	if runtime.GOOS == "windows" {
		nome += ".exe"
	}
	b, _ := os.ReadFile(exe)
	if err := os.WriteFile(filepath.Join(pasta, nome), b, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", pasta+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CENTRAL_CLI_FALSA", modo)
	t.Setenv("ANTHROPIC_API_KEY", "sk-nao-pode-chegar-na-cli")
}

func TestCLIClaudeUsaSaidaEstruturadaEPassaPelaGuarda(t *testing.T) {
	comCLIFalsa(t, "ok")
	var usos []Uso
	a := Nova(Config{Rotas: map[string]Rota{"padrao": {Provedor: "claude-cli", Modelo: "haiku"}}}, nil, func(u Uso) { usos = append(usos, u) })
	s, err := a.Executar(context.Background(), "triarVaga", entradaTriagem)
	if err != nil {
		t.Fatal(err)
	}
	if s["decisao"] != "pular" || s["motivoPulo"] != "Outro" { // guarda: pular sem motivoPulo vira "Outro"
		t.Fatalf("saída = %v", s)
	}
	if *usos[0].TokensEntrada != 103 || *usos[0].TokensSaida != 20 || usos[0].Regras[0] != "pulo-com-motivo" {
		t.Fatalf("uso = %+v", usos[0])
	}
}

func TestCLIClaudeSemLogin(t *testing.T) {
	comCLIFalsa(t, "erro")
	_, err := cli{qual: "claude"}.Chamar(context.Background(), PedidoProvedor{Prompt: MontarPrompt("triarVaga", entradaTriagem)})
	var e *ErroIA
	if !errors.As(err, &e) || e.Tipo != "provedor" || !strings.Contains(e.Mensagem, "Not logged in") || e.Reintentavel {
		t.Fatalf("erro = %+v", err)
	}
}

func TestCLIClaudeQuebrada(t *testing.T) {
	comCLIFalsa(t, "lixo")
	_, err := cli{qual: "claude"}.Chamar(context.Background(), PedidoProvedor{Prompt: MontarPrompt("triarVaga", entradaTriagem)})
	if err == nil || !strings.Contains(err.Error(), "claude-cli: falhou feio") {
		t.Fatalf("erro = %v", err)
	}
}

func TestCLINaoInstalada(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := cli{qual: "gemini"}.Chamar(context.Background(), PedidoProvedor{})
	var e *ErroIA
	if !errors.As(err, &e) || e.Tipo != "config" {
		t.Fatalf("erro = %v", err)
	}
}
