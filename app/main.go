// Applyer: app de PC (Wails v3). A janela mostra as telas React de
// frontend/dist; os pedidos para /api/ vão para a API local em Go (internal/api).
package main

import (
	"context"
	"embed"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

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
	// jc-etv.7: a pasta "Central de Candidaturas" vira "Applyer" ao abrir (rename inteiro, ou adiado se estiver em uso).
	abertura, err := config.AbrirPastaDados()
	if err != nil {
		falhar("pasta de dados", err)
	}
	pasta := abertura.Caminho
	log.Printf("partida: %s", config.OrigemPasta()) // sem o caminho (jc-a2y.57)

	// O app nasce antes do banco: se já há um Applyer aberto nesta pasta de dados, esta segunda abertura só traz a
	// janela dele de volta e sai aqui dentro (application.New), sem tocar no banco nem no envio.
	var aberta atomic.Pointer[controleJanela]
	apiLocal := &apiTardia{}
	app := application.New(application.Options{
		Name:        "Applyer",
		Description: "Busca, triagem e acompanhamento de candidaturas",
		Assets: application.AssetOptions{
			Handler: rotear(apiLocal, application.AssetFileServerFS(telas)),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: idDaInstancia(pasta),
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if c := aberta.Load(); c != nil {
					c.mostrar()
				}
			},
		},
	})

	banco, err := armazem.Abrir(ctx, config.CaminhoBanco(pasta))
	if err != nil {
		falhar("banco", err)
	}
	defer banco.Fechar()
	registrarMigracao(ctx, banco, abertura)
	if _, err := banco.PrepararPrimeiraVez(context.Background(), modelodados.Plataformas); err != nil {
		falhar("primeira vez", err)
	}
	// Envio de candidaturas: desligado no build padrão (não faz nada); só o build -tags envio_dev liga o executor.
	controleEnvio := envio.Ligar(ctx, banco, pasta, nil) // a trava real (Conta.PodeAgir) entra na Task 6
	defer controleEnvio.Parar()

	// A janela abre no tamanho que a pessoa deixou (ou no padrão) e, se ela pediu, o app fica em segundo plano.
	tamanho, segundoPlano := lerJanela(ctx, banco)
	controle := abrirJanela(app, banco, tamanho, segundoPlano)
	aberta.Store(controle)

	local := api.Nova(banco, api.Opcoes{
		PararEnvio:      controleEnvio.Parar,
		SuspenderEnvio:  controleEnvio.Suspender,
		FecharNavegador: controleEnvio.FecharNavegador,
		// Depois de "apagar meus dados", o app fecha (o banco já foi fechado e apagado): nada mais é gravado.
		AoApagar: func() {
			controle.calar()
			controle.sair()
		},
		EnvioLigado: envio.Ligado,
		// Envio sem janela; só a tela mostra o envio ou abre o login (jc-a2y.6.7).
		MostrarEnvio: controleEnvio.MostrarEnvio,
		Entrar:       controleEnvio.Entrar,
		Janela:       controleEnvio.Janela,
		// "Ao vivo" (jc-a2y.6.9): nil no build padrão (a rota dá 404).
		AoVivo: controleEnvio.AoVivo,
		// Aviso da pasta de dados nesta abertura, para a tela (GET /dados/pasta, jc-etv.14), sem caminho.
		AvisoPasta: abertura.Aviso,
		// Leitura de formulários (jc-a2y.7.2): nil no build padrão (a rota dá 404).
		LerFormulario: controleEnvio.LerFormulario,
		// Janela do app (Perfil): segundo plano e voltar ao tamanho padrão.
		JanelaApp: controle.ganchos(),
	})
	apiLocal.definir(local.Handler())

	if err := app.Run(); err != nil {
		falhar("janela", err)
	}
}

// registrarMigracao: o fato e o aviso da migração da pasta no log, sem caminho. No banco (meta "pasta") fica só
// quando a pasta foi renomeada (migradaEm), e uma vez: o aviso é desta abertura e vai para a API por Opcoes.AvisoPasta
// (GET /dados/pasta, jc-etv.14), para não ficar velho no banco depois que a pessoa resolver.
func registrarMigracao(ctx context.Context, banco *armazem.Armazem, a config.Abertura) {
	if a.Aviso != "" {
		log.Printf("pasta: %s", a.Aviso)
	}
	if !a.Migrou {
		return
	}
	log.Printf("pasta: migrada para %s", config.NomePasta)
	if err := banco.GravarMeta(ctx, "pasta", map[string]any{"migradaEm": time.Now().Format(time.RFC3339)}); err != nil {
		log.Printf("pasta: não gravou o registro da migração (%T)", err)
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
