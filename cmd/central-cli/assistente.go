package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/api"
	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/assistente"
	"github.com/FRF-Tecnology/job-control/internal/config"
	"github.com/FRF-Tecnology/job-control/internal/conta"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// As ferramentas do assistente na CLI (jc-a2y.7.4, PR 4): a mesma tabela do MCP (internal/assistente), pelo canal
// cli. A pasta só é lida e conferida, e o banco nunca é criado nem migrado (A2 e o achado do CyberSec no #139).
//
// Limite (A7.3 do CyberSec): um assistente com acesso ao shell pode rodar qualquer comando ou apagar arquivos. A
// barreira da regra M4 é o MCP, o canal recomendado para o assistente. A CLI segue as mesmas regras, mas não protege
// contra um shell aberto.

// errRecusado: a ferramenta recusou (regra do domínio, lista do assistente ou argumentos). Sai com código 2.
var errRecusado = errors.New("recusado")

// listarFerramentas: o nome e o título de cada ferramenta, e se só lê.
func listarFerramentas(saida io.Writer) {
	for _, f := range assistente.Ferramentas {
		marca := ""
		if f.SomenteLeitura {
			marca = " (só lê)"
		}
		fmt.Fprintf(saida, "  %-22s %s%s\n", f.Nome, f.Titulo, marca)
	}
}

// rodarFerramenta: central-cli <ferramenta> [--json '{...}' | --json -]. A resposta vai para a saída padrão, em JSON
// (com o teto de 64 KB); a recusa, para a saída de erro.
func rodarFerramenta(f assistente.Ferramenta, args []string, entrada io.Reader, saida io.Writer) error {
	fs := flag.NewFlagSet(f.Nome, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	bruto := fs.String("json", "", "argumentos em JSON ('-' lê da entrada padrão)")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		return fmt.Errorf("%w: use central-cli %s --json '{...}'", errRecusado, f.Nome)
	}
	argumentos := map[string]any{}
	texto := *bruto
	if texto == "-" {
		b, err := io.ReadAll(io.LimitReader(entrada, 1<<20))
		if err != nil {
			return err
		}
		texto = string(b)
	}
	if strings.TrimSpace(texto) != "" {
		if err := json.Unmarshal([]byte(texto), &argumentos); err != nil {
			return fmt.Errorf("%w: os argumentos têm de ser um objeto JSON", errRecusado)
		}
	}
	ctx := context.Background()
	a, fechar, err := abrirDoAssistente(ctx)
	if err != nil {
		return err
	}
	defer fechar()
	r, err := assistente.Rodar(ctx, a, f, argumentos)
	var ef *assistente.ErroFerramenta
	if errors.As(err, &ef) {
		return fmt.Errorf("%w: %s", errRecusado, ef.Msg)
	}
	if err != nil {
		return err
	}
	fmt.Fprintln(saida, assistente.Texto(r))
	return nil
}

// abrirDoAssistente: a API do assistente no canal cli, sobre o banco que o app criou.
func abrirDoAssistente(ctx context.Context) (*api.API, func(), error) {
	pasta, err := config.PastaDoAssistente()
	if err != nil {
		return nil, nil, err
	}
	arm, err := armazem.AbrirExistente(ctx, config.CaminhoBanco(pasta)) // nunca cria nem migra
	if err != nil {
		return nil, nil, err
	}
	// A trava lê licenca.json a cada chamada, sem rede e sem renovar (só o app renova). Build sem nuvem: sem trava.
	trava, err := conta.TravaDoBuild(pasta)
	if err != nil {
		arm.Fechar()
		return nil, nil, err
	}
	if trava == nil {
		log.Print(conta.AvisoDesligada) // ruling A11: sem trava, como antes (uma vez: um comando por processo)
	}
	return api.Nova(arm, api.Opcoes{Canal: dominio.CanalCLI, PodeAgir: trava}), func() { arm.Fechar() }, nil
}

// entrada: de onde a confirmação do --substituir é lida (o teste troca).
var entrada io.Reader = os.Stdin

// fraseSubstituir: a frase que a pessoa digita para o importar-legado --substituir (A7.2 do CyberSec).
const fraseSubstituir = "substituir meus dados"

func confirmarSubstituir() error {
	fmt.Fprintf(os.Stderr, "Isso troca TODOS os dados do app novo pelos da Central antiga (um backup é feito antes).\nPara confirmar, digite: %s\n> ", fraseSubstituir)
	b := make([]byte, 0, 64)
	um := make([]byte, 1)
	for len(b) < 200 {
		n, err := entrada.Read(um)
		if n == 0 || err != nil || um[0] == '\n' {
			break
		}
		b = append(b, um[0])
	}
	if strings.TrimSpace(strings.TrimSuffix(string(b), "\r")) != fraseSubstituir {
		return errors.New("confirmação diferente da frase: nada foi trocado")
	}
	return nil
}
