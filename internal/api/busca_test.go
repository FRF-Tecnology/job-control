package api

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/busca"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
	"github.com/FRF-Tecnology/job-control/internal/triagem"
)

type provedorDaRodada struct {
	chamadas int
	falhar   bool
}

func TestTrocarCriteriosInvalidaTodaRodadaERespeitaEdicaoConcorrente(t *testing.T) {
	ctx := context.Background()
	arm, err := armazem.Abrir(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer arm.Fechar()
	a := Nova(arm, Opcoes{Cofre: segredos.Memoria{}})
	err = arm.Gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		e.Perfil = Objeto{"cargo": "Designer", "modo": "direto"}
		for i := 0; i < limiteDaBusca+2; i++ {
			e.Vagas = append(e.Vagas, Objeto{"id": fmt.Sprint(i), "titulo": "Design", "empresa": "Exemplo", "etapa": "descoberta", "aderencia": "A", "triagemBusca": "classificada", "triagemPerfil": assinaturaDeBusca(e.Perfil)})
		}
		e.Vagas = append(e.Vagas, Objeto{"id": "manual", "empresa": "Outra", "titulo": "Analista", "etapa": "descoberta", "notas": "nota particular"})
		return e, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	r, err := a.Chamar(ctx, Pedido{Metodo: "PATCH", Caminho: Prefixo + "/perfil", Autor: "eu", Corpo: Objeto{"cargo": "Analista"}})
	if err != nil || r.Status != 200 {
		t.Fatalf("%v %v", err, r)
	}
	e, err := arm.Carregar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(dominio.FilaAplicar(e)) != 0 {
		t.Fatal("nota do perfil anterior ainda habilita a Fila")
	}
	var balanco balancoBusca
	lote, err := a.prepararBusca(ctx, nil, assinaturaDeBusca(e.Perfil), &balanco)
	if err != nil || len(lote) != limiteDaBusca || balanco.Pendentes != 2 {
		t.Fatalf("lote %d, pendentes %d, erro %v", len(lote), balanco.Pendentes, err)
	}
	id := lote[0]["id"]
	if err = arm.Gravar(ctx, func(atual dominio.Estado) (dominio.Estado, error) {
		novo, _, err := dominio.MoverVaga(atual, id, "pulada", Objeto{"motivo": "Escolha da pessoa"}, a.ctx(atual, "eu"))
		return novo, err
	}); err != nil {
		t.Fatal(err)
	}
	gravado, err := a.salvarParecer(ctx, lote[0], assinaturaDeBusca(e.Perfil), triagem.Resultado{Estado: "classificada", Aderencia: "A", Decisao: "aplicar", Motivo: "Compatível"})
	if err != nil || gravado {
		t.Fatalf("parecer sobrescreveu decisão: %v %v", gravado, err)
	}
}

func (*provedorDaRodada) Nome() string { return "anthropic" }
func (p *provedorDaRodada) Chamar(_ context.Context, req ia.PedidoProvedor) (ia.RespostaProvedor, error) {
	p.chamadas++
	if p.falhar {
		return ia.RespostaProvedor{}, errors.New("provedor indisponível")
	}
	grupo := "B"
	if strings.Contains(req.Prompt.Mensagem, "Produto Aurora") {
		grupo = "A"
	}
	if strings.Contains(req.Prompt.Mensagem, "Produto Cedro") {
		grupo = "C"
	}
	return ia.RespostaProvedor{Texto: `{"decisao":"perguntar","aderencia":"` + grupo + `","motivo":"Verifique o escopo do cargo com a equipe."}`}, nil
}

func TestPrimeiroAcessoBuscaTriagemRetomada(t *testing.T) {
	ctx := context.Background()
	caminho := filepath.Join(t.TempDir(), "dados.sqlite")
	provedor := &provedorDaRodada{falhar: true}
	abrir := func() (*armazem.Armazem, *API) {
		arm, err := armazem.Abrir(ctx, caminho)
		if err != nil {
			t.Fatal(err)
		}
		a := Nova(arm, Opcoes{Cofre: segredos.Memoria{"anthropic": "chave-ficticia"}, ProvedoresIA: map[string]ia.Provedor{"anthropic": provedor}})
		return arm, a
	}
	arm, a := abrir()
	rr := httptest.NewRecorder()
	perfilComLocal := strings.Replace(perfilInicialJSON, `"cargo":"Designer"`, `"cargo":"Designer","paisTrabalho":"BR"`, 1)
	a.Handler().ServeHTTP(rr, pedidoOnboarding(t, perfilComLocal, "curriculo.pdf", []byte("%PDF-1.4\nCurrículo fictício\n%%EOF\n")))
	if rr.Code != 200 {
		t.Fatalf("onboarding: %d %s", rr.Code, rr.Body.String())
	}
	if err := arm.GravarMeta(ctx, chaveConfig, ia.Config{Rotas: map[string]ia.Rota{"padrao": {Provedor: "anthropic", Modelo: "teste"}}}); err != nil {
		t.Fatal(err)
	}
	arm.Fechar()
	arm, a = abrir()
	defer arm.Fechar()
	a.consultarVagas = func(_ context.Context, perfil Objeto) busca.Coleta {
		if perfil["cargo"] != "Designer" {
			t.Fatalf("perfil após reiniciar: %v", perfil["cargo"])
		}
		var vagas []Objeto
		for i, empresa := range []string{"Produto Cedro", "Produto Aurora", "Produto Ipê", "Estúdio Sênior"} {
			nivel := "Pleno"
			if i == 3 {
				nivel = "Sênior"
			}
			vagas = append(vagas, Objeto{"id": empresa, "empresa": empresa, "titulo": "Designer de produto", "nivel": nivel, "modelo": "Remoto", "mercado": "nacional", "descricao": "Projetar interfaces e protótipos usando Figma.", "categorias": []string{"Produto"}, "publicadaEm": time.Now().UTC().Add(-24 * time.Hour).Truncate(24 * time.Hour).Format(time.RFC3339), "paisesElegiveis": []string{"BR"}})
		}
		return busca.Coleta{Vagas: vagas}
	}
	rodar := func() balancoBusca {
		r, err := a.Chamar(ctx, Pedido{Metodo: "POST", Caminho: Prefixo + "/busca", Corpo: Objeto{}, Autor: "eu"})
		if err != nil || r.Status != 200 {
			t.Fatalf("busca %d: %v %v", r.Status, r.Corpo, err)
		}
		return r.Corpo.(balancoBusca)
	}
	if r := rodar(); r.Novas != 4 || r.Pendentes != 3 || r.Puladas != 1 {
		t.Fatalf("rodada com falha: %+v", r)
	}
	e, err := arm.Carregar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range e.Vagas {
		if v["aderencia"] != nil {
			t.Fatal("nota inventada antes de resposta do provedor")
		}
	}
	provedor.falhar = false
	r := rodar()
	if r.Novas != 0 || r.Grupos["A"] != 1 || r.Grupos["B"] != 1 || r.Grupos["C"] != 1 || r.Puladas != 0 || r.Pendentes != 0 {
		t.Fatalf("retomada: %+v", r)
	}
	antes := provedor.chamadas
	if r := rodar(); r.Novas != 0 || provedor.chamadas != antes {
		t.Fatalf("duplicação: %+v", r)
	}
	e, err = arm.Carregar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Vagas) != 4 || len(e.Candidaturas) != 0 {
		t.Fatalf("vagas %d candidaturas %d", len(e.Vagas), len(e.Candidaturas))
	}
	for _, v := range e.Vagas {
		if v["nivel"] == "Sênior" && (v["etapa"] != "pulada" || v["motivo"] == "") {
			t.Fatal("exclusão não persistida")
		}
	}
	// Mudou o nível aceito: o pulo automático anterior pode ser reavaliado.
	_, err = a.Chamar(ctx, Pedido{Metodo: "PATCH", Caminho: Prefixo + "/perfil", Corpo: Objeto{"filtros": Objeto{"niveis": []any{"Sênior"}, "modelo": []any{"Remoto"}}}, Autor: "eu"})
	if err != nil {
		t.Fatal(err)
	}
	if r := rodar(); r.Puladas != 3 || r.Grupos["B"] != 1 {
		t.Fatalf("mudança do perfil: %+v", r)
	}
}
