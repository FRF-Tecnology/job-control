//go:build comparacao

// Compara a API Go com a Central Node rodando, sobre os mesmos dados reais. Opcional (não roda no CI):
//
//	central-cli importar-legado --dir dados        (com CENTRAL_PASTA apontando para uma pasta de teste)
//	CENTRAL_PASTA=<pasta> CENTRAL_NODE=http://localhost:4321 go test -tags comparacao -run CompararComNode ./internal/api/
//
// Nenhum dado é gravado nem impresso: só os caminhos que diferem.
package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/config"
)

func TestCompararComNode(t *testing.T) {
	pasta, node := os.Getenv("CENTRAL_PASTA"), os.Getenv("CENTRAL_NODE")
	if pasta == "" || node == "" {
		t.Skip("defina CENTRAL_PASTA e CENTRAL_NODE")
	}
	ctx := context.Background()
	arm, err := armazem.Abrir(ctx, config.CaminhoBanco(pasta))
	if err != nil {
		t.Fatal(err)
	}
	defer arm.Fechar()
	a := Nova(arm, Opcoes{})
	e, err := arm.Carregar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rotas := []string{"/resumo", "/validar", "/vagas", "/vagas?etapa=enviada", "/vagas?plataforma=gupy", "/followups", "/perfil",
		"/plataformas", "/rodadas", "/pedidos", "/emails", "/emails?status=sem-vaga", "/historico.md", "/exportar.csv",
		"/duplicatas?empresa=CI%26T&titulo=Desenvolvedor"}
	for i, v := range e.Vagas {
		if i%25 == 0 { // uma amostra das vagas, com os eventos de cada uma
			rotas = append(rotas, "/vagas/"+url.PathEscape(v["id"].(string)))
		}
	}
	for _, rota := range rotas {
		r, err := a.Chamar(ctx, Pedido{Metodo: "GET", Caminho: Prefixo + rota, Corpo: Objeto{}})
		if err != nil {
			t.Fatalf("%s: %v", rota, err)
		}
		resp, err := http.Get(node + Prefixo + rota)
		if err != nil {
			t.Fatalf("Node não respondeu: %v", err)
		}
		bruto, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != r.Status {
			t.Errorf("%s: status Go %d, Node %d", rota, r.Status, resp.StatusCode)
			continue
		}
		if r.Tipo != "" {
			if r.Corpo.(string) != string(bruto) {
				t.Errorf("%s: texto diferente (Go %d bytes, Node %d bytes)", rota, len(r.Corpo.(string)), len(bruto))
			}
			continue
		}
		var nodeJSON any
		json.Unmarshal(bruto, &nodeJSON)
		if !reflect.DeepEqual(normal(r.Corpo), nodeJSON) {
			t.Errorf("%s: JSON diferente", rota)
		}
	}
	t.Logf("%d rotas comparadas", len(rotas))
}
