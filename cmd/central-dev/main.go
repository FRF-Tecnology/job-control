// central-dev: serve a API local e as telas (app/frontend/dist) num navegador comum, só para desenvolvimento
// e testes de ponta a ponta. Escuta apenas em 127.0.0.1 e recusa outro Host (proteção contra DNS rebinding).
//
//	CENTRAL_PASTA=<pasta de teste> go run ./cmd/central-dev        (PORT muda a porta; padrão 4500)
package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/api"
	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/config"
	modelodados "github.com/FRF-Tecnology/job-control/modelo-dados"
)

func main() {
	pasta, err := config.PastaDados()
	if err != nil {
		log.Fatal(err)
	}
	arm, err := armazem.Abrir(context.Background(), config.CaminhoBanco(pasta))
	if err != nil {
		log.Fatal(err)
	}
	defer arm.Fechar()
	if _, err := arm.PrepararPrimeiraVez(context.Background(), modelodados.Plataformas); err != nil {
		log.Fatalf("primeira vez: %v", err)
	}
	telas := os.Getenv("CENTRAL_TELAS")
	if telas == "" {
		telas = "app/frontend/dist"
	}
	// Depois de "apagar meus dados", o processo sai (o banco já foi fechado e apagado).
	apiLocal := api.Nova(arm, api.Opcoes{AoApagar: func() { os.Exit(0) }}).Handler()
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
	log.Printf("Central (desenvolvimento) em http://localhost:%s — dados em %s", porta, pasta)
	log.Fatal(http.ListenAndServe("127.0.0.1:"+porta, h))
}
