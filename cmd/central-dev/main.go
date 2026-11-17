// central-dev: serve a API local e as telas (app/frontend/dist) num navegador comum, só para desenvolvimento
// e testes de ponta a ponta. Escuta apenas em 127.0.0.1 e recusa outro Host (proteção contra DNS rebinding).
//
//	CENTRAL_PASTA=<pasta de teste> go run ./cmd/central-dev        (PORT muda a porta; padrão 4500)
//
// O central-dev NUNCA roda o executor real (revisão do CyberSec no #61): ele escuta numa porta TCP, e o Handler
// trata como a pessoa todo pedido sem Sec-Fetch-Site, então qualquer processo local poderia aprovar e retomar. Com
// -tags envio_dev, só o painel do envio aparece ligado (para testar a tela), sempre começando pausado; nada é
// enviado. O envio de verdade só existe no app compilado com a tag (a janela do Wails, sem porta).
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/api"
	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/config"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
	modelodados "github.com/FRF-Tecnology/job-control/modelo-dados"
)

func main() {
	pastaTeste := os.Getenv("CENTRAL_E2E_PASTA")
	if pastaTeste != "" && pastaTeste != os.Getenv("APPLYER_PASTA") && pastaTeste != os.Getenv("CENTRAL_PASTA") {
		falhar("teste", fmt.Errorf("a pasta do servidor deve ser a pasta de teste"))
	}
	pasta, err := config.PastaDados()
	if err != nil {
		falhar("pasta de dados", err)
	}
	arm, err := armazem.Abrir(context.Background(), config.CaminhoBanco(pasta))
	if err != nil {
		falhar("banco", err)
	}
	defer arm.Fechar()
	if _, err := arm.PrepararPrimeiraVez(context.Background(), modelodados.Plataformas); err != nil {
		falhar("primeira vez", err)
	}
	telas := os.Getenv("CENTRAL_TELAS")
	if telas == "" {
		telas = "app/frontend/dist"
	}
	// Painel do envio sem executor: no build envio_dev, aparece ligado e abre pausado (como o app); no padrão, desligado.
	if painelDoEnvio {
		if err := arm.GravarEnvioPausado(context.Background(), true); err != nil {
			falhar("envio", err)
		}
	}
	// Depois de "apagar meus dados", o processo sai (o banco já foi fechado e apagado).
	o := api.Opcoes{AoApagar: func() { os.Exit(0) }, EnvioLigado: painelDoEnvio}
	if pastaTeste != "" {
		o.Cofre = segredos.Memoria{}
	}
	// Conta: só com -tags contafalsa (nuvem falsa e a rota de teste do relógio); sem a tag, sem conta (ruling A11).
	minhaConta, envolver := contaDoDev(pasta)
	if minhaConta != nil {
		minhaConta.Iniciar(context.Background())
		o.Conta, o.PodeAgir = minhaConta, minhaConta.PodeAgir
		// Nada abre de verdade: o central-dev não abre o navegador do PC (Checkout e login Google da nuvem falsa).
		o.AbrirNavegador = func(string) error { log.Print("conta: o app abriria o navegador aqui"); return nil }
	}
	apiLocal := envolver(api.Nova(arm, o).Handler())
	arquivos := http.FileServer(http.Dir(telas))
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			host = r.Host
		}
		if host != "localhost" && host != "127.0.0.1" && host != "[::1]" && host != "::1" {
			http.Error(w, "acesso só pelo próprio computador", http.StatusForbidden)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiLocal.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		arquivos.ServeHTTP(w, r)
	})
	porta := os.Getenv("PORT")
	if porta == "" {
		porta = "4500"
	}
	log.Print(mensagemDePartida(porta))
	log.Fatal(novoServidor("127.0.0.1:"+porta, h).ListenAndServe()) // com tempos limite (jc-a2y.14)
}

// mensagemDePartida diz onde a Central abriu e de onde veio a pasta, sem o caminho (que traz o nome do usuário).
func mensagemDePartida(porta string) string {
	return fmt.Sprintf("Central (desenvolvimento) em http://localhost:%s — %s", porta, config.OrigemPasta())
}

// falhar encerra com o erro de partida sem o caminho da pasta de dados nem o da pasta do usuário.
func falhar(etapa string, err error) {
	log.Fatalf("%s: %s", etapa, config.OcultarCaminhos(err.Error()))
}
