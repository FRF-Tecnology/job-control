package api

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// jc-a2y.28: só a pessoa (autor "eu") aprova uma vaga para aplicar, adiciona vaga fora de descoberta e liga o modo
// direto. O assistente (autor "claude": MCP e CLI na P6) recebe 400 e nada muda. Dados fictícios.
func TestSoAPessoaAprova(t *testing.T) {
	ctx := context.Background()
	arm, err := armazem.Abrir(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer arm.Fechar()
	a := Nova(arm, Opcoes{Hoje: func() string { return "2026-11-09" }, Aleatorio: func() string { return "abcd" }, Cofre: segredos.Memoria{}})
	chamar := func(m, p string, corpo Objeto, autor string) Resposta {
		t.Helper()
		r, err := a.Chamar(ctx, Pedido{Metodo: m, Caminho: Prefixo + p, Corpo: normal(corpo), Autor: autor})
		if err != nil {
			t.Fatalf("%s %s: %v", m, p, err)
		}
		return r
	}
	estado := func() string {
		e, err := arm.Carregar(ctx)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(e)
		return string(b)
	}
	vaga := Objeto{"id": "li-9001", "empresa": "Empresa Exemplo", "titulo": "Dev Backend", "plataforma": "linkedin", "mercado": "internacional", "aderencia": "A"}
	chamar("PUT", "/perfil", Objeto{"nome": "Pessoa Exemplo", "modo": "revisar"}, "eu")
	if r := chamar("POST", "/vagas", vaga, "claude"); r.Status != 201 {
		t.Fatalf("assistente adiciona em descoberta = %d", r.Status)
	}
	antes := estado()

	recusas := []struct {
		m, p  string
		corpo Objeto
		erro  string
	}{
		{"POST", "/vagas/li-9001/mover", Objeto{"etapa": "aplicar"}, dominio.ErroAplicarSoPessoa},
		{"POST", "/vagas", Objeto{"id": "li-9002", "empresa": "Outra Exemplo", "titulo": "Dev", "etapa": "aplicar"}, dominio.ErroAdicionarSoDescoberta},
		{"POST", "/vagas", Objeto{"id": "li-9003", "empresa": "Outra Exemplo", "titulo": "Dev 2", "etapa": "enviada", "envio": Objeto{"data": "2026-11-01"}}, dominio.ErroAdicionarSoDescoberta},
		{"PATCH", "/perfil", Objeto{"modo": "direto"}, ErroModoSoPessoa},
		{"PUT", "/perfil", Objeto{"nome": "Pessoa Exemplo", "modo": "direto"}, ErroModoSoPessoa},
	}
	for _, c := range recusas {
		r := chamar(c.m, c.p, c.corpo, "claude")
		if r.Status != 400 || !strings.Contains(emJSON(r.Corpo), c.erro) {
			t.Fatalf("%s %s pelo assistente = %d %s", c.m, c.p, r.Status, emJSON(r.Corpo))
		}
	}
	if depois := estado(); depois != antes {
		t.Fatalf("recusa mudou o estado:\nantes  %s\ndepois %s", antes, depois)
	}
	// Voltar para revisar o assistente pode (só reduz risco); o resto do perfil também.
	if r := chamar("PATCH", "/perfil", Objeto{"cidade": "Cidade Exemplo", "modo": "revisar"}, "claude"); r.Status != 200 {
		t.Fatalf("assistente ajusta o perfil sem ligar o direto = %d", r.Status)
	}

	// A pessoa continua aprovando, adicionando em qualquer etapa e escolhendo o modo.
	for _, c := range []struct {
		m, p  string
		corpo Objeto
		quer  int
	}{
		{"POST", "/vagas/li-9001/mover", Objeto{"etapa": "aplicar"}, 200},
		{"POST", "/vagas", Objeto{"id": "li-9004", "empresa": "Outra Exemplo", "titulo": "Dev 3", "etapa": "aplicar"}, 201},
		{"PATCH", "/perfil", Objeto{"modo": "direto"}, 200},
	} {
		if r := chamar(c.m, c.p, c.corpo, "eu"); r.Status != c.quer {
			t.Fatalf("%s %s pela pessoa = %d %s", c.m, c.p, r.Status, emJSON(r.Corpo))
		}
	}

	// Fila do /aplicar: as duas aprovadas pela pessoa; com o modo direto, nada a mais (não há grupo A em descoberta).
	e, _ := arm.Carregar(ctx)
	ids := []string{}
	for _, v := range dominio.FilaAplicar(e) {
		ids = append(ids, v["id"].(string))
	}
	if !reflect.DeepEqual(ids, []string{"li-9001", "li-9004"}) {
		t.Fatalf("fila = %v", ids)
	}
}
