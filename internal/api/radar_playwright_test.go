//go:build e2e_radar

package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/busca"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
	modelodados "github.com/FRF-Tecnology/job-control/modelo-dados"
)

// O Playwright abre este servidor numa pasta temporária vazia. As duas modalidades rodam
// a API do app e recebem o mesmo anúncio fictício. Só a modalidade claude chama a assinatura.
func TestServidorRadarPlaywright(t *testing.T) {
	modo := os.Getenv("APPLYER_E2E_PROVEDOR")
	if modo != "controlado" && modo != "claude" {
		t.Skip("iniciar com playwright.radar.config.ts")
	}
	pasta := os.Getenv("CENTRAL_E2E_PASTA")
	if pasta == "" || pasta != os.Getenv("APPLYER_PASTA") {
		t.Fatal("a pasta de teste e a do app precisam coincidir")
	}
	entradas, err := os.ReadDir(pasta)
	if err != nil || len(entradas) != 0 {
		t.Fatal("use uma pasta de teste vazia")
	}
	ctx := context.Background()
	arm, err := armazem.Abrir(ctx, filepath.Join(pasta, "central.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer arm.Fechar()
	if _, err = arm.PrepararPrimeiraVez(ctx, modelodados.Plataformas); err != nil {
		t.Fatal(err)
	}
	op := Opcoes{Cofre: segredos.Memoria{}}
	if modo == "controlado" {
		op.DetectarCLIs = func(context.Context) []ia.EstadoCLI {
			ok := true
			return []ia.EstadoCLI{{InfoCLI: ia.CLIs[0], Instalada: true, Pronta: true, Logada: &ok}}
		}
		op.ProvedoresIA = map[string]ia.Provedor{"claude-cli": provedorRadarFicticio{}}
	}
	a := Nova(arm, op)
	a.consultarVagas = func(context.Context, Objeto) busca.Coleta {
		return busca.Coleta{Vagas: []Objeto{{
			"id": "radar-qa-react", "empresa": "Oficina de Software Fictícia", "titulo": "Desenvolvimento de software com React",
			"descricao": "Desenvolvimento de software com React e Java para pessoa de nível Pleno. Trabalho remoto no Brasil.",
			"nivel":     "Pleno", "modelo": "Remoto", "mercado": "nacional", "plataforma": "externo",
			"salarioMin": 4200, "salarioMax": 5500, "periodoSalario": "monthly", "moeda": "USD",
			"fonteBusca": "Fonte de teste", "link": "https://vaga.example.invalid/radar-qa-react",
		}}}
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", a.Handler())
	mux.Handle("/", http.FileServer(http.Dir(os.Getenv("CENTRAL_TELAS"))))
	endereco := "127.0.0.1:4511"
	if os.Getenv("APPLYER_E2E_SUITE") == "1" && modo == "controlado" {
		endereco = "127.0.0.1:4510"
	}
	serv := &http.Server{Addr: endereco, ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host != endereco && !(endereco == "127.0.0.1:4510" && r.Host == "localhost:4510") {
			http.Error(w, "host inválido", http.StatusForbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(w, r)
	})}
	t.Fatal(serv.ListenAndServe())
}

type provedorRadarFicticio struct{}

func (provedorRadarFicticio) Nome() string { return "claude-cli" }
func (provedorRadarFicticio) Chamar(_ context.Context, p ia.PedidoProvedor) (ia.RespostaProvedor, error) {
	if p.Prompt.NomeEsquema == "sugerirPerfilDoCV" {
		return ia.RespostaProvedor{Texto: `{"sugestoes":[{"campo":"cargo","valor":"Desenvolvimento de software","evidencia":"Desenvolvimento de software"},{"campo":"stackPrincipal","valor":"React","evidencia":"React e Java"},{"campo":"stackPrincipal","valor":"Java","evidencia":"React e Java"}]}`}, nil
	}
	return ia.RespostaProvedor{Texto: `{"decisao":"aplicar","aderencia":"A","motivo":"As ferramentas e o nível correspondem ao perfil."}`}, nil
}
