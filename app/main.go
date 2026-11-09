// Central de Candidaturas: app de PC (Wails v3). A janela mostra as telas React de
// frontend/dist; os pedidos para /api/ vão para a API local em Go (internal/api).
package main

import (
	"context"
	"embed"
	"log"
	"net/http"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/api"
	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/config"
	"github.com/FRF-Tecnology/job-control/internal/envio"
	modelodados "github.com/FRF-Tecnology/job-control/modelo-dados"
	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var telas embed.FS

func main() {
	ctx := context.Background()
	pasta, err := config.PastaDados()
	if err != nil {
		falhar("pasta de dados", err)
	}
	banco, err := armazem.Abrir(ctx, config.CaminhoBanco(pasta))
	if err != nil {
		falhar("banco", err)
	}
	defer banco.Fechar()
	if _, err := banco.PrepararPrimeiraVez(context.Background(), modelodados.Plataformas); err != nil {
		falhar("primeira vez", err)
	}
	// Envio de candidaturas: desligado no build padrão (não faz nada); só o build -tags envio_dev liga o executor.
	controleEnvio := envio.Ligar(ctx, banco, pasta)
	defer controleEnvio.Parar()

	// Depois de "apagar meus dados", o app fecha (o banco já foi fechado e apagado).
	var app *application.App
	local := api.Nova(banco, api.Opcoes{
		PararEnvio:      controleEnvio.Parar,
		SuspenderEnvio:  controleEnvio.Suspender,
		FecharNavegador: controleEnvio.FecharNavegador,
		AoApagar:        func() { app.Quit() },
		EnvioLigado:     envio.Ligado,
	})

	app = application.New(application.Options{
		Name:        "Central de Candidaturas",
		Description: "Busca, triagem e acompanhamento de candidaturas",
		Assets: application.AssetOptions{
			Handler: rotear(local.Handler(), application.AssetFileServerFS(telas)),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Central de Candidaturas",
		Width:     1280,
		Height:    820,
		MinWidth:  380,
		MinHeight: 560,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		URL: "/",
	})

	if err := app.Run(); err != nil {
		falhar("janela", err)
	}
}

// falhar encerra com o erro de partida sem o caminho da pasta de dados nem o da pasta do usuário.
func falhar(etapa string, err error) {
	log.Fatalf("%s: %s", etapa, config.OcultarCaminhos(err.Error()))
}

// rotear manda /api/ para a API e todo o resto para as telas.
func rotear(apiLocal, telas http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			apiLocal.ServeHTTP(w, r)
			return
		}
		telas.ServeHTTP(w, r)
	})
}
