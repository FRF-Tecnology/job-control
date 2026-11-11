// central-mcp: o servidor MCP local do Applyer (jc-a2y.7.4, docs/propostas/mcp-cli.md). Um cliente MCP (Claude Code,
// Claude Desktop) o abre por stdio. Ele fala com o banco da pasta do app pela entrada do assistente (canal mcp, a
// regra M4): só lê, triagem e registros, revogar e pausar. Nunca cria, migra nem renomeia a pasta, e nunca usa o
// cofre nem uma IA. O stdout é só do protocolo; o log vai para o stderr.
package main

import (
	"context"
	"log"
	"os"

	"github.com/FRF-Tecnology/job-control/internal/api"
	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/config"
	"github.com/FRF-Tecnology/job-control/internal/conta"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/mcp"
)

func main() {
	log.SetOutput(os.Stderr)
	if _, ligada, err := conta.ChaveDoBuild(); err == nil && !ligada {
		log.Print(conta.AvisoDesligada) // ruling A11: sem trava, como antes
	}
	s := &mcp.Servidor{Abrir: abrir, Versao: api.VersaoApp}
	if err := s.Servir(context.Background(), os.Stdin, os.Stdout); err != nil {
		log.Printf("mcp: parou (%T)", err)
		os.Exit(1)
	}
}

// abrir: a pasta e o banco para UMA chamada (A2.3): entre as chamadas, o MCP não segura o arquivo, e o app pode
// apagar os dados ou renomear a pasta.
func abrir(ctx context.Context) (*api.API, func(), error) {
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
	return api.Nova(arm, api.Opcoes{Canal: dominio.CanalMCP, PodeAgir: trava}), func() { arm.Fechar() }, nil
}
