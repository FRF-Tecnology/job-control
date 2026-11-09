// central-cli: linha de comando do app novo. Por enquanto só importa os dados da Central antiga;
// os outros comandos (os mesmos de ferramentas/cli.mjs) chegam na fase P6.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/config"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/legado"
)

const ajuda = `Uso: central-cli <comando> [opções]

  importar-legado --dir <pasta dados/ da Central antiga> [--dry-run] [--substituir]
      Leva vagas, eventos, perfil, plataformas, rodadas, pedidos, emails e currículos (PDF)
      para o app novo. Os ids continuam os mesmos. Recusa dados com CPF/RG.
      --dry-run     só mostra o que seria importado
      --substituir  troca os dados que o app novo já tem (faz backup antes)

  avaliar [--provedor claude-cli] [--modelo m] [--limite n] [--saida arquivo]
      Roda o conjunto de avaliacao/ na IA escolhida e grava as respostas brutas; a pontuação
      é a do avaliador Node: node ferramentas/cli.mjs avaliar --respostas <arquivo> --modelo <nome>

A pasta de dados do app é a de sempre (ou CENTRAL_PASTA).`

func main() {
	if len(os.Args) < 2 {
		fmt.Println(ajuda)
		return
	}
	var err error
	switch os.Args[1] {
	case "importar-legado":
		err = importarLegado(os.Args[2:])
	case "avaliar":
		err = avaliar(os.Args[2:])
	default:
		fmt.Println(ajuda)
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", config.OcultarCaminhos(err.Error())) // sem o caminho da pasta nem o nome do usuário
		os.Exit(1)
	}
}

func importarLegado(args []string) error {
	fs := flag.NewFlagSet("importar-legado", flag.ExitOnError)
	dir := fs.String("dir", "", "pasta dados/ da Central antiga")
	simular := fs.Bool("dry-run", false, "só mostra o que seria importado")
	substituir := fs.Bool("substituir", false, "troca os dados que o app novo já tem")
	fs.Parse(args)
	if *dir == "" {
		return fmt.Errorf("informe --dir com a pasta dados/ da Central antiga")
	}
	estado, rel, err := legado.Ler(*dir)
	if err != nil {
		return err
	}
	colecoes := make([]string, 0, len(rel.Contagem))
	for c := range rel.Contagem {
		colecoes = append(colecoes, c)
	}
	sort.Strings(colecoes)
	fmt.Println("Lido de", config.OcultarCaminhos(*dir))
	for _, c := range colecoes {
		fmt.Printf("  %-12s %d\n", c, rel.Contagem[c])
	}
	fmt.Printf("  %-12s %d PDF(s)\n", "currículos", len(rel.Curriculos))
	for _, a := range rel.Avisos {
		fmt.Println("  aviso:", a)
	}
	if len(rel.Bloqueios) > 0 {
		for _, b := range rel.Bloqueios {
			fmt.Println("  BLOQUEIO:", b)
		}
		return fmt.Errorf("%d item(ns) parecem conter documentos (CPF/RG); a Central não guarda documentos. Corrija na Central antiga e rode de novo", len(rel.Bloqueios))
	}
	if *simular {
		fmt.Println("Simulação: nada foi gravado.")
		return nil
	}

	ctx := context.Background()
	pasta, err := config.PastaDados()
	if err != nil {
		return err
	}
	arm, err := armazem.Abrir(ctx, config.CaminhoBanco(pasta))
	if err != nil {
		return err
	}
	defer arm.Fechar()
	atual, err := arm.Carregar(ctx)
	if err != nil {
		return err
	}
	temDados := len(atual.Vagas)+len(atual.Eventos)+len(atual.Pedidos)+len(atual.Emails)+len(atual.Rodadas) > 0 || len(atual.Perfil) > 0
	if temDados && !*substituir {
		return fmt.Errorf("o app novo já tem dados (%s); use --substituir para trocar (um backup é feito antes)", config.OrigemPasta())
	}
	if temDados {
		if _, err := arm.BackupDiario(ctx, dominio.HojeISO(time.Now())+"-antes-da-importacao"); err != nil {
			return err
		}
	}
	if err := arm.Gravar(ctx, func(dominio.Estado) (dominio.Estado, error) { return estado, nil }); err != nil {
		return err
	}
	copiados, err := legado.CopiarCurriculos(*dir, filepath.Join(pasta, "curriculos"), rel.Curriculos)
	if err != nil {
		return err
	}
	fmt.Printf("Importado (%s; %d currículo(s) copiado(s)).\n", config.OrigemPasta(), copiados)
	return nil
}
