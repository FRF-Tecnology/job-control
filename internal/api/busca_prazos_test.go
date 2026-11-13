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

// provedorLento responde depois de `atraso`. A vaga com "Trava" na empresa só responde quando o prazo vence,
// como a CLI presa numa resposta longa.
type provedorLento struct{ atraso time.Duration }

func (provedorLento) Nome() string { return "anthropic" }
func (p provedorLento) Chamar(ctx context.Context, pedido ia.PedidoProvedor) (ia.RespostaProvedor, error) {
	espera := p.atraso
	if strings.Contains(pedido.Prompt.Mensagem, "Trava") {
		espera = time.Hour
	}
	select {
	case <-time.After(espera):
		return ia.RespostaProvedor{Texto: `{"decisao":"aplicar","aderencia":"A","motivo":"A função inclui design."}`}, nil
	case <-ctx.Done():
		return ia.RespostaProvedor{}, &ia.ErroIA{Tipo: "provedor", Mensagem: "anthropic: passou do tempo limite", Provedor: "anthropic", Reintentavel: true}
	}
}

func apiLenta(t *testing.T, atraso time.Duration) *API {
	t.Helper()
	ctx := context.Background()
	arm, err := armazem.Abrir(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	a := Nova(arm, Opcoes{Cofre: segredos.Memoria{"anthropic": "ficticia"}, ProvedoresIA: map[string]ia.Provedor{"anthropic": provedorLento{atraso}}})
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

// prazosNoTeste encurta os prazos da busca e devolve os originais no fim do teste.
func prazosNoTeste(t *testing.T, coleta, porVaga, triagem time.Duration) {
	t.Helper()
	antes := [3]time.Duration{prazoDaColeta, prazoPorVaga, prazoDaTriagem}
	prazoDaColeta, prazoPorVaga, prazoDaTriagem = coleta, porVaga, triagem
	t.Cleanup(func() { prazoDaColeta, prazoPorVaga, prazoDaTriagem = antes[0], antes[1], antes[2] })
}

func vagasDeDesign(a *API, empresas ...string) []Objeto {
	agora := a.relogio()
	var vagas []Objeto
	for i, empresa := range empresas {
		vagas = append(vagas, Objeto{"id": fmt.Sprintf("vaga-%d", i), "link": fmt.Sprintf("https://teste.example/vagas/%d", i), "titulo": "Designer", "empresa": empresa, "descricao": "Desenhar interfaces.", "modelo": "Remoto", "remotoMundial": true, "publicadaEm": agora.Add(-time.Duration(i+1) * time.Hour).Format(time.RFC3339)})
	}
	return vagas
}

func buscarComVagas(t *testing.T, a *API, vagas []Objeto) balancoBusca {
	t.Helper()
	a.consultarVagas = func(context.Context, Objeto) busca.Coleta { return busca.Coleta{Vagas: vagas} }
	r, err := a.buscarETriar(context.Background(), "original")
	if err != nil {
		t.Fatal(err)
	}
	return r.Corpo.(balancoBusca)
}

func temAviso(r balancoBusca, trecho string) bool {
	for _, aviso := range r.Avisos {
		if strings.Contains(aviso, trecho) {
			return true
		}
	}
	return false
}

// Antes, a coleta e as triagens dividiam um prazo só: a soma das triagens passava dele e a rodada parava
// pedindo para conferir a conexão. Agora o prazo da coleta não corta a triagem.
func TestBuscaTriagemNaoDivideOPrazoDaColeta(t *testing.T) {
	prazosNoTeste(t, 100*time.Millisecond, time.Second, 10*time.Second)
	a := apiLenta(t, 60*time.Millisecond) // 4 vagas x 60 ms passam dos 100 ms da coleta
	r := buscarComVagas(t, a, vagasDeDesign(a, "Um", "Dois", "Tres", "Quatro"))
	if r.Pendentes != 0 || r.Grupos["A"]+r.Grupos["B"]+r.Grupos["C"] != 4 {
		t.Fatalf("todas deviam ser avaliadas: %+v", r)
	}
	if temAviso(r, "Confira a conexão") {
		t.Fatalf("culpou a conexão: %+v", r.Avisos)
	}
}

// Uma vaga que passa do prazo dela fica pendente, e a rodada segue com as outras.
func TestBuscaVagaLentaFicaPendenteEARodadaSegue(t *testing.T) {
	prazosNoTeste(t, time.Second, 150*time.Millisecond, 10*time.Second)
	a := apiLenta(t, 10*time.Millisecond)
	r := buscarComVagas(t, a, vagasDeDesign(a, "Um", "Trava", "Tres"))
	if r.Pendentes != 1 || r.Grupos["A"]+r.Grupos["B"]+r.Grupos["C"] != 2 {
		t.Fatalf("esperava 1 pendente e 2 avaliadas: %+v", r)
	}
	if !temAviso(r, msgVagaLenta) || temAviso(r, "Confira a conexão") {
		t.Fatalf("avisos: %+v", r.Avisos)
	}
	e, err := a.arm.Carregar(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range e.Vagas {
		if v["empresa"] == "Trava" && v["triagemBusca"] != "pendente" {
			t.Fatalf("a vaga lenta devia ficar pendente para o Retomar: %v", v["triagemBusca"])
		}
	}
}

// O teto da rodada para a triagem com um aviso de Retomar, sem culpar a conexão.
func TestBuscaTetoDaRodadaPedeRetomar(t *testing.T) {
	prazosNoTeste(t, time.Second, time.Second, 150*time.Millisecond)
	a := apiLenta(t, 100*time.Millisecond)
	r := buscarComVagas(t, a, vagasDeDesign(a, "Um", "Dois", "Tres", "Quatro"))
	avaliadas := r.Grupos["A"] + r.Grupos["B"] + r.Grupos["C"]
	if avaliadas == 0 || avaliadas == 4 || r.Pendentes != 4-avaliadas {
		t.Fatalf("esperava parar no meio, com o resto pendente: %+v", r)
	}
	if !temAviso(r, msgRodadaSemTempo) || temAviso(r, "Confira a conexão") {
		t.Fatalf("avisos: %+v", r.Avisos)
	}
}
