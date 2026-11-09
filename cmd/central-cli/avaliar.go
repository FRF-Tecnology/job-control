package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/config"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// avaliar roda o conjunto de avaliacao/ num provedor e grava as respostas brutas ({id: texto}).
// A pontuação é a do avaliador Node, com a mesma guarda:
//
//	node ferramentas/cli.mjs avaliar --respostas <arquivo> --modelo <nome>
func avaliar(args []string) error {
	fs := flag.NewFlagSet("avaliar", flag.ExitOnError)
	dir := fs.String("dir", "avaliacao", "pasta do conjunto de avaliação")
	provedor := fs.String("provedor", "claude-cli", "claude-cli | codex-cli | gemini-cli | anthropic | openai | google")
	modelo := fs.String("modelo", "", "modelo (padrão do provedor quando vazio)")
	limite := fs.Int("limite", 0, "no máximo N casos de cada função (0 = todos)")
	saida := fs.String("saida", "respostas-avaliacao.json", "arquivo com as respostas brutas")
	fs.Parse(args)

	ler := func(nome string, destino any) error {
		b, err := os.ReadFile(filepath.Join(*dir, nome))
		if err != nil {
			return err
		}
		return json.Unmarshal(b, destino)
	}
	var perfil dominio.Objeto
	var vagas []any
	if err := ler("perfil.json", &perfil); err != nil {
		return err
	}
	if err := ler("vagas-andamento.json", &vagas); err != nil {
		return err
	}
	curriculo, err := os.ReadFile(filepath.Join(*dir, "curriculo.md"))
	if err != nil {
		return err
	}
	arquivos, _ := filepath.Glob(filepath.Join(*dir, "casos", "*.json"))
	sort.Strings(arquivos)

	pasta, err := config.PastaDados()
	if err != nil {
		return err
	}
	cofre := segredos.Sistema{Servico: segredos.ServicoDaPasta(pasta)} // as chaves desta pasta (jc-a2y.65)
	x := ia.Nova(ia.Config{}, func(p string) string { c, _ := cofre.Ler(p); return c }, nil)
	prov := x.Provedores[*provedor]
	if prov == nil {
		return fmt.Errorf("provedor desconhecido: %s", *provedor)
	}
	respostas := map[string]string{}
	falhas := 0
	for _, arq := range arquivos {
		funcao := strings.TrimSuffix(filepath.Base(arq), ".json")
		var casos []struct {
			ID      string         `json:"id"`
			Entrada dominio.Objeto `json:"entrada"`
		}
		b, _ := os.ReadFile(arq)
		if err := json.Unmarshal(b, &casos); err != nil {
			return fmt.Errorf("%s: %w", arq, err)
		}
		for i, c := range casos {
			if *limite > 0 && i >= *limite {
				break
			}
			// Completa a entrada com o perfil, o currículo e as vagas do conjunto (como montarEntrada no Node).
			e := c.Entrada
			if contemTexto([]string{"triarVaga", "responderPergunta", "revisarAntesDeEnviar", "escreverMensagem"}, funcao) && e["perfil"] == nil {
				e["perfil"] = perfil
			}
			if contemTexto([]string{"responderPergunta", "revisarAntesDeEnviar"}, funcao) && e["curriculo"] == nil {
				e["curriculo"] = string(curriculo)
			}
			if funcao == "classificarEmail" && e["vagas"] == nil {
				e["vagas"] = vagas
			}
			if err := ia.ValidarEntrada(funcao, e); err != nil {
				return fmt.Errorf("%s: %w", c.ID, err)
			}
			inicio := time.Now()
			r, err := prov.Chamar(context.Background(), ia.PedidoProvedor{Modelo: *modelo, Prompt: ia.MontarPrompt(funcao, e)})
			if err != nil {
				falhas++
				fmt.Printf("  %-45s erro: %v\n", c.ID, err)
				continue
			}
			respostas[c.ID] = r.Texto
			fmt.Printf("  %-45s %5.1f s\n", c.ID, time.Since(inicio).Seconds())
		}
	}
	b, _ := json.MarshalIndent(respostas, "", " ")
	if err := os.WriteFile(*saida, b, 0o600); err != nil {
		return err
	}
	fmt.Printf("%d respostas em %s (%d falhas). Pontue com: node ferramentas/cli.mjs avaliar --respostas %s --modelo %s\n", len(respostas), *saida, falhas, *saida, *provedor)
	return nil
}

func contemTexto(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}
