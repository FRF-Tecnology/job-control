package api

import (
	"context"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/ia"
)

// Simula um provedor ocupado. Os dois destinos e as credenciais só existem em memória.
type provedorEmAndamento struct {
	iniciou   chan struct{}
	continuar chan struct{}
}

func (*provedorEmAndamento) Nome() string { return "anthropic" }
func (p *provedorEmAndamento) Chamar(ctx context.Context, _ ia.PedidoProvedor) (ia.RespostaProvedor, error) {
	close(p.iniciou)
	select {
	case <-ctx.Done():
		return ia.RespostaProvedor{}, ctx.Err()
	case <-p.continuar:
		return ia.RespostaProvedor{Texto: `{"decisao":"perguntar","aderencia":"B","motivo":"Confirmar o nível."}`}, nil
	}
}

func TestRodadaRevalidaDestinoDaChaveSemTravarAConfiguracao(t *testing.T) {
	p := &provedorEmAndamento{iniciou: make(chan struct{}), continuar: make(chan struct{})}
	a := apiCom(t, map[string]ia.Provedor{"anthropic": p})
	salvar := func(rota, corpo string) {
		t.Helper()
		if status, _ := chamarHTTP(t, a, "PUT", Prefixo+rota, corpo); status != 200 {
			t.Fatalf("PUT %s: %d", rota, status)
		}
	}
	salvar("/ia/config", `{"provedor":"anthropic","url":"https://primeiro.example"}`)
	salvar("/ia/chaves/anthropic", `{"chave":"primeira-chave-de-teste"}`)
	ctx, cancelar := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancelar()
	anterior, err := a.novaIA(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if anterior.Chave("anthropic") != "primeira-chave-de-teste" {
		t.Fatal("a primeira configuração não encontrou sua chave")
	}
	terminou := make(chan error, 1)
	go func() {
		_, err := anterior.Executar(ctx, "triarVaga", Objeto{"vaga": Objeto{"titulo": "Vaga fictícia"}, "perfil": Objeto{}})
		terminou <- err
	}()
	select {
	case <-p.iniciou:
	case <-ctx.Done():
		t.Fatal("provedor fictício não iniciou")
	}
	salvar("/ia/config", `{"provedor":"anthropic","url":"https://segundo.example"}`)
	salvar("/ia/chaves/anthropic", `{"chave":"segunda-chave-de-teste"}`)
	close(p.continuar)
	if err := <-terminou; err != nil {
		t.Fatalf("a troca ficou bloqueada pela chamada ao provedor: %v", err)
	}
	novo, err := a.novaIA(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if anterior.Chave("anthropic") != "" || novo.Chave("anthropic") != "segunda-chave-de-teste" {
		t.Fatal("credencial incoerente com a configuração da rodada")
	}
	salvar("/ia/config", `{"provedor":"nenhum"}`)
	if novo.Chave("anthropic") != "" {
		t.Fatal("uma rodada desconectada ainda obteve credencial")
	}
}
