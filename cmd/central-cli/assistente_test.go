package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/assistente"
	"github.com/FRF-Tecnology/job-control/internal/config"
)

// jc-a2y.7.4 (PR 4): as ferramentas do assistente na CLI e o A7 do CyberSec. Dados fictícios.

// pastaDoApp: uma pasta fictícia como o app deixa (banco criado e migrado, pasta protegida).
func pastaDoApp(t *testing.T) string {
	t.Helper()
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
	return pasta
}

func cli(t *testing.T, nome string, json string) (string, error) {
	t.Helper()
	f, ok := assistente.Achar(nome)
	if !ok {
		t.Fatalf("sem %s", nome)
	}
	var saida bytes.Buffer
	err := rodarFerramenta(f, []string{"--json", json}, strings.NewReader(""), &saida)
	return saida.String(), err
}

// A7.1: importar-legado e avaliar são da pessoa e ficam fora da tabela do assistente.
func TestComandosDaPessoaForaDaTabela(t *testing.T) {
	for _, nome := range []string{"importar-legado", "avaliar", "ferramentas"} {
		if _, ok := assistente.Achar(nome); ok {
			t.Errorf("%s está na tabela do assistente", nome)
		}
	}
	var b bytes.Buffer
	listarFerramentas(&b)
	if strings.Contains(b.String(), "importar") || strings.Contains(b.String(), "avaliar") || !strings.Contains(b.String(), "vagas_listar") {
		t.Fatalf("lista = %s", b.String())
	}
}

// As ferramentas pela CLI, no canal cli: as mesmas regras do MCP.
func TestFerramentasPelaCLI(t *testing.T) {
	pastaDoApp(t)
	if _, err := cli(t, "vaga_adicionar", `{"vagas":[{"id":"v-1","empresa":"Empresa Teste","titulo":"Dev Go","link":"https://exemplo.com/1"}]}`); err != nil {
		t.Fatal(err)
	}
	saida, err := cli(t, "vagas_listar", `{}`)
	if err != nil || !strings.Contains(saida, `"v-1"`) {
		t.Fatalf("vagas_listar = %q %v", saida, err)
	}
	for nome, json := range map[string]string{
		"vaga_mover":   `{"id":"v-1","etapa":"aplicar"}`, // só a pessoa
		"vaga_detalhe": `{"id":"v-1","x":1}`,             // campo a mais
	} {
		if _, err := cli(t, nome, json); !errors.Is(err, errRecusado) {
			t.Errorf("%s = %v; queria recusado", nome, err)
		}
	}
	if _, err := cli(t, "vagas_listar", `não é json`); !errors.Is(err, errRecusado) {
		t.Fatalf("JSON inválido = %v", err)
	}
	// --json - lê da entrada padrão.
	f, _ := assistente.Achar("vaga_detalhe")
	var b bytes.Buffer
	if err := rodarFerramenta(f, []string{"--json", "-"}, strings.NewReader(`{"id":"v-1"}`), &b); err != nil || !strings.Contains(b.String(), "Empresa Teste") {
		t.Fatalf("--json - = %q %v", b.String(), err)
	}
	// Pausar pela CLI grava o canal cli.
	if _, err := cli(t, "envio_pausar", `{}`); err != nil {
		t.Fatal(err)
	}
	if saida, _ := cli(t, "envio_estado", `{}`); !strings.Contains(saida, `"canal": "cli"`) {
		t.Fatalf("envio_estado = %s", saida)
	}
}

// Sem a pasta do app: erro (não é recusa de regra), sem caminho, e nada é criado.
func TestCLISemPasta(t *testing.T) {
	pasta := filepath.Join(t.TempDir(), "Usuario Ficticio", "nao-existe")
	t.Setenv("APPLYER_PASTA", pasta)
	t.Setenv("CENTRAL_PASTA", "")
	_, err := cli(t, "resumo", `{}`)
	if err == nil || errors.Is(err, errRecusado) || strings.Contains(err.Error(), "Usuario Ficticio") {
		t.Fatalf("sem pasta = %v", err)
	}
	if _, e := os.Stat(pasta); e == nil {
		t.Fatal("a CLI criou a pasta")
	}
}

// A7.2: o --substituir pede a frase digitada; sem ela, nada é trocado.
func TestSubstituirPedeAFrase(t *testing.T) {
	t.Setenv("APPLYER_PASTA", filepath.Join(t.TempDir(), "applyer-teste"))
	t.Setenv("CENTRAL_PASTA", "")
	antiga := t.TempDir()
	os.WriteFile(filepath.Join(antiga, "perfil.json"), []byte(`{"nome":"Pessoa Ficticia"}`), 0o600)
	if _, err := capturar(t, func() error { return importarLegado([]string{"--dir", antiga}) }); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(antiga, "perfil.json"), []byte(`{"nome":"Pessoa Trocada"}`), 0o600)
	antes := entrada
	t.Cleanup(func() { entrada = antes })
	for _, digitado := range []string{"", "sim\n", "SUBSTITUIR MEUS DADOS\n", "substituir meus dado\n"} {
		entrada = strings.NewReader(digitado)
		if _, err := capturar(t, func() error { return importarLegado([]string{"--dir", antiga, "--substituir"}) }); err == nil {
			t.Errorf("--substituir com %q trocou os dados", digitado)
		}
	}
	entrada = strings.NewReader(fraseSubstituir + "\r\n")
	if _, err := capturar(t, func() error { return importarLegado([]string{"--dir", antiga, "--substituir"}) }); err != nil {
		t.Fatalf("--substituir com a frase = %v", err)
	}
}

// As ferramentas da CLI nunca criam nem migram o banco (achado do CyberSec no #139): só o AbrirExistente, no canal cli.
func TestFerramentasDaCLISoComAbrirExistente(t *testing.T) {
	b, err := os.ReadFile("assistente.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if strings.Contains(s, "armazem.Abrir(") || !strings.Contains(s, "armazem.AbrirExistente(") || !strings.Contains(s, "Canal: dominio.CanalCLI") {
		t.Fatal("as ferramentas da CLI abrem o banco fora do AbrirExistente, ou fora do canal cli")
	}
}
