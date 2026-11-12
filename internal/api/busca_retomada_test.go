package api

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/busca"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

type provedorDeRetomada struct{}

func (provedorDeRetomada) Nome() string { return "anthropic" }
func (provedorDeRetomada) Chamar(_ context.Context, pedido ia.PedidoProvedor) (ia.RespostaProvedor, error) {
	if strings.Contains(pedido.Prompt.Mensagem, "Parecer inválido") {
		return ia.RespostaProvedor{Texto: "{}"}, nil
	}
	return ia.RespostaProvedor{Texto: `{"decisao":"aplicar","aderencia":"A","motivo":"A função inclui design."}`}, nil
}

func apiDeRetomada(t *testing.T) *API {
	t.Helper()
	ctx := context.Background()
	arm, err := armazem.Abrir(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	a := Nova(arm, Opcoes{Cofre: segredos.Memoria{"anthropic": "ficticia"}, ProvedoresIA: map[string]ia.Provedor{"anthropic": provedorDeRetomada{}}})
	if err := arm.GravarMeta(ctx, chaveConfig, ia.Config{Rotas: map[string]ia.Rota{"padrao": {Provedor: "anthropic", Modelo: "teste"}}}); err != nil {
		t.Fatal(err)
	}
	if err := arm.Gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		e.Perfil = Objeto{"cargo": "Designer", "paisTrabalho": "BR"}
		return e, nil
	}); err != nil {
		t.Fatal(err)
	}
	return a
}

func rodadaDeRetomada(t *testing.T, a *API, vagas []Objeto) balancoBusca {
	t.Helper()
	a.consultarVagas = func(context.Context, Objeto) busca.Coleta { return busca.Coleta{Vagas: vagas} }
	r, err := a.buscarETriar(context.Background(), "original")
	if err != nil {
		t.Fatal(err)
	}
	return r.Corpo.(balancoBusca)
}

func TestRetomadaDaBuscaProssegueAposErrosIndividuais(t *testing.T) {
	a := apiDeRetomada(t)
	var vagas []Objeto
	agora := a.relogio()
	for i := 0; i <= limiteDaBusca; i++ {
		empresa := "Parecer inválido"
		if i == limiteDaBusca {
			empresa = "Parecer válido"
		}
		vagas = append(vagas, Objeto{"id": fmt.Sprintf("vaga-%d", i), "link": fmt.Sprintf("https://teste.example/vagas/%d", i), "titulo": "Designer", "empresa": empresa, "descricao": "Desenhar interfaces.", "modelo": "Remoto", "remotoMundial": true, "publicadaEm": agora.Add(-time.Duration(i+1) * time.Hour).Format(time.RFC3339)})
	}
	primeira := rodadaDeRetomada(t, a, vagas)
	if primeira.Pendentes != len(vagas) || primeira.Grupos["A"] != 0 {
		t.Fatalf("primeiro lote: %+v", primeira)
	}
	segunda := rodadaDeRetomada(t, a, vagas)
	if segunda.Grupos["A"] != 1 || segunda.Pendentes != limiteDaBusca {
		t.Fatalf("anúncio válido preso atrás dos erros persistentes: %+v", segunda)
	}
}

func TestRetomadaDaBuscaAtualizaEspelhoPelaFonteDireta(t *testing.T) {
	a := apiDeRetomada(t)
	const link = "https://exemplo.gupy.io/jobs/123"
	espelho := Objeto{"id": "himalayas-espelho", "titulo": "Designer", "empresa": "Estúdio", "link": link + "?utm_source=api", "fonteBusca": "Himalayas", "plataforma": "externo", "descricao": "Desenhar interfaces.", "modelo": "Remoto", "remotoMundial": true}
	if r := rodadaDeRetomada(t, a, []Objeto{espelho}); r.Novas != 1 || r.Grupos["B"] != 1 {
		t.Fatalf("espelho sem data: %+v", r)
	}
	data := a.relogio().Add(-time.Hour).Format(time.RFC3339)
	direta := Objeto{"id": "gupy-123", "titulo": "Designer", "empresa": "Estúdio", "link": link, "fonteBusca": "Gupy", "plataforma": "gupy", "descricao": "Desenhar interfaces para clínicas.", "modelo": "Remoto", "paisesElegiveis": []string{"BR"}, "publicadaEm": data}
	if r := rodadaDeRetomada(t, a, []Objeto{direta}); r.Novas != 0 || r.Grupos["A"] != 1 {
		t.Fatalf("fonte direta deve atualizar a vaga e reclassificá-la: %+v", r)
	}
	if r := rodadaDeRetomada(t, a, []Objeto{espelho}); r.Novas != 0 || r.Grupos["B"] != 0 {
		t.Fatalf("espelho não pode substituir a fonte direta: %+v", r)
	}
	e, err := a.arm.Carregar(context.Background())
	if err != nil || len(e.Vagas) != 1 {
		t.Fatalf("vagas: %d, erro: %v", len(e.Vagas), err)
	}
	v := e.Vagas[0]
	if v["id"] != espelho["id"] || v["anuncioId"] != direta["id"] || v["fonteBusca"] != "Gupy" || v["plataforma"] != "gupy" || v["publicadaEm"] != data || v["aderencia"] != "A" {
		t.Fatalf("a vaga precisa manter o ID local e os dados oficiais: %v", v)
	}
}
