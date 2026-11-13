package api

import (
	"context"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/conta"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Trava das ações (Task 5): sem licença válida, toda escrita dá 402 com a frase fixa do motivo, para a tela, a CLI e
// o MCP. Ver continua liberado, e também a conta, a assinatura, apagar os dados, a janela do app e as chaves de IA.

func apiTravada(t *testing.T, canal, motivo string) *API {
	t.Helper()
	arm, err := armazem.Abrir(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	return Nova(arm, Opcoes{Hoje: func() string { return "2026-11-11" }, Aleatorio: func() string { return "trva" }, Canal: canal,
		PodeAgir: func() (bool, string) { return motivo == "", motivo }})
}

func chamarTela(t *testing.T, a *API, m, caminho string, corpo any) Resposta {
	t.Helper()
	if corpo == nil {
		corpo = Objeto{}
	}
	r, err := a.Chamar(context.Background(), Pedido{Metodo: m, Caminho: Prefixo + caminho, Corpo: corpo, Autor: "eu"})
	if err != nil {
		t.Fatalf("%s %s: %v", m, caminho, err)
	}
	return r
}

func TestTravaPorRota(t *testing.T) {
	a := apiTravada(t, "", conta.MotivoTesteAcabou)
	bloqueadas := []struct{ m, caminho string }{
		{"POST", "/vagas"},
		{"POST", "/vagas/x/mover"},
		{"PUT", "/perfil"},
		{"POST", "/candidaturas/x/aprovar"},
		{"PUT", "/ia/config"},
		{"post", "/vagas?forcar=1"},
		{"DELETE", "/vagas/x"},
		{"PATCH", "/vagas/x"},
		{"PUT", "/ia/chaves"},         // fora do formato de /ia/chaves/{provedor}
		{"PUT", "/ia/chaves/A"},       // idem
		{"POST", "/contas"},           // /conta só com a barra
		{"POST", "/dados/apagar/x"},   // só o caminho exato
		{"POST", "/app/janela/outra"}, // só /app/janela e /app/janela/padrao
		// Ruling A9: pausar sim, retomar não; entrar e pedidos continuam travados.
		{"POST", "/envio/retomar"},
		{"POST", "/envio/plataformas/linkedin/retomar"},
		{"POST", "/envio/plataformas/linkedin/pausar/x"},
		{"POST", "/navegador/entrar"},
		{"POST", "/pedidos/x/cancelar"},
		{"PUT", "/backup"},
		{"POST", "/dados/pasta"},
	}
	for _, b := range bloqueadas {
		r := chamarTela(t, a, b.m, b.caminho, nil)
		if r.Status != 402 {
			t.Errorf("%s %s travado = %d; queria 402", b.m, b.caminho, r.Status)
		}
	}
	liberadas := []struct{ m, caminho string }{
		{"GET", "/estado"},
		{"HEAD", "/estado"},
		{"OPTIONS", "/vagas"},
		{"POST", "/dados/apagar"},
		{"PUT", "/ia/chaves/anthropic"},
		{"DELETE", "/ia/chaves/openai?x=1"},
		{"POST", "/conta/sair"},
		{"POST", "/assinatura/assinar"},
		{"POST", "/app/janela"},
		{"POST", "/app/janela/padrao"},
		// Ruling A9: escritas de proteção ou de levar os dados embora.
		{"POST", "/envio/pausar"},
		{"POST", "/envio/plataformas/linkedin/pausar"},
		{"POST", "/navegador/sair"},
		{"POST", "/backup"},
		{"PUT", "/dados/pasta"},
	}
	for _, l := range liberadas {
		r := chamarTela(t, a, l.m, l.caminho, nil)
		if r.Status == 402 {
			t.Errorf("%s %s travado = 402; queria liberado", l.m, l.caminho)
		}
	}
	// Ver o estado mostra tudo: a trava não esconde dado nenhum.
	if r := chamarTela(t, a, "GET", "/estado", nil); r.Status != 200 {
		t.Fatalf("GET /estado travado = %d", r.Status)
	}
}

func TestTravaMotivos(t *testing.T) {
	frases := map[string]string{
		conta.MotivoTesteAcabou:    "Seu teste acabou. Você ainda vê tudo; para buscar e se candidatar, assine.",
		conta.MotivoSemAssinatura:  "Sua assinatura não está ativa. Você ainda vê tudo; para buscar e se candidatar, assine.",
		conta.MotivoSemInternet:    "Sem conexão: dá para ver, mas não para buscar ou se candidatar.",
		conta.MotivoSemConta:       "Entre na sua conta para buscar e se candidatar.",
		conta.MotivoLicencaVencida: "Abra o Applyer para renovar o acesso.",
	}
	for motivo, frase := range frases {
		r := chamarTela(t, apiTravada(t, "", motivo), "POST", "/vagas", Objeto{"id": "v-trava", "empresa": "Empresa Teste", "titulo": "Dev"})
		c := dominio.ObjetoDe(r.Corpo)
		if r.Status != 402 || c["erro"] != frase || c["motivo"] != motivo {
			t.Errorf("motivo %s = %d %v; queria 402 %q", motivo, r.Status, r.Corpo, frase)
		}
		if got := respostaTrava(motivo); dominio.ObjetoDe(got.Corpo)["erro"] != frase || got.Status != 402 {
			t.Errorf("respostaTrava(%s) = %+v", motivo, got)
		}
	}
	// Motivo desconhecido (falha fechada): ainda 402, com a frase e o motivo de quem não tem assinatura (a tela só
	// recebe motivos conhecidos).
	if r := respostaTrava("outro"); r.Status != 402 || dominio.ObjetoDe(r.Corpo)["erro"] != frases[conta.MotivoSemAssinatura] ||
		dominio.ObjetoDe(r.Corpo)["motivo"] != conta.MotivoSemAssinatura {
		t.Errorf("respostaTrava(outro) = %+v", r)
	}
	if r := respostaTrava(""); r.Status != 402 || dominio.ObjetoDe(r.Corpo)["motivo"] != conta.MotivoSemAssinatura {
		t.Errorf("respostaTrava(vazio) = %+v", r)
	}
	// Liberado: a mesma escrita passa.
	if r := chamarTela(t, apiTravada(t, "", ""), "POST", "/vagas", Objeto{"id": "v-trava", "empresa": "Empresa Teste", "titulo": "Dev"}); r.Status != 201 {
		t.Errorf("POST /vagas liberado = %d %v", r.Status, r.Corpo)
	}
}

// Sem PodeAgir (testes antigos, central-dev sem conta), tudo liberado.
func TestTravaSemPodeAgirLibera(t *testing.T) {
	hoje := "2026-11-11"
	a := novaAPI(t, &hoje, "livr")
	if r := chamarTela(t, a, "POST", "/vagas", Objeto{"id": "v-trava", "empresa": "Empresa Teste", "titulo": "Dev"}); r.Status != 201 {
		t.Fatalf("POST /vagas sem PodeAgir = %d %v", r.Status, r.Corpo)
	}
}

func TestTravaValeParaAssistente(t *testing.T) {
	for _, canal := range []string{dominio.CanalCLI, dominio.CanalMCP} {
		a := apiTravada(t, canal, conta.MotivoTesteAcabou)
		r := chamarAssistente(t, a, "POST", "/vagas/x/mover", Objeto{"etapa": "triagem"})
		if r.Status != 402 {
			t.Errorf("%s: POST /vagas/x/mover travado = %d %v; queria 402", canal, r.Status, r.Corpo)
		}
		// Pelo Chamar direto (o desvio para o ChamarAssistente) também.
		if r := chamarTela(t, a, "POST", "/vagas/x/mover", Objeto{"etapa": "triagem"}); r.Status != 402 {
			t.Errorf("%s: Chamar POST /vagas/x/mover travado = %d; queria 402", canal, r.Status)
		}
		// Ler continua liberado.
		if r := chamarAssistente(t, a, "GET", "/vagas", nil); r.Status != 200 {
			t.Errorf("%s: GET /vagas travado = %d %v", canal, r.Status, r.Corpo)
		}
	}
}

func TestLiberadoSemLicenca(t *testing.T) {
	casos := []struct {
		m, caminho string
		quer       bool
	}{
		{"GET", Prefixo + "/vagas", true},
		{"get", Prefixo + "/vagas", true},
		{"POST", Prefixo + "/conta", true},
		{"POST", Prefixo + "/conta/entrar?x=1", true},
		{"POST", Prefixo + "/conta/atualizar", true}, // "Tentar de novo" da faixa
		{"POST", Prefixo + "/assinatura", true},
		{"POST", Prefixo + "/ia/chaves/gemini", true},
		{"POST", Prefixo + "/vagas", false},
		{"POST", "/conta/sair", false}, // sem o prefixo não é rota da API
		{"POST", Prefixo + "/contabil", false},
		{"POST", Prefixo + "/ia/chaves/", false},
	}
	for _, c := range casos {
		if got := liberadoSemLicenca(c.m, c.caminho); got != c.quer {
			t.Errorf("liberadoSemLicenca(%s, %s) = %v; queria %v", c.m, c.caminho, got, c.quer)
		}
	}
}

// Ruling A11 (Task 6): build sem a nuvem, sem Conta. GET /conta responde {"ativa": false} para a tela pular a
// entrada; o resto da conta continua 503, e o assistente continua 403.
func TestContaDesligadaNoBuild(t *testing.T) {
	a := apiTravada(t, "", "")
	a.podeAgir = nil
	r := chamarTela(t, a, "GET", "/conta", nil)
	if c := dominio.ObjetoDe(r.Corpo); r.Status != 200 || c["ativa"] != false || len(c) != 1 {
		t.Fatalf("GET /conta sem Conta = %d %v", r.Status, r.Corpo)
	}
	if r := chamarTela(t, a, "POST", "/conta/entrar", Objeto{}); r.Status != 503 {
		t.Fatalf("POST /conta/entrar sem Conta = %d", r.Status)
	}
	if r, _ := a.Chamar(context.Background(), Pedido{Metodo: "GET", Caminho: Prefixo + "/conta", Autor: "claude"}); r.Status != 403 {
		t.Fatalf("GET /conta do assistente = %d", r.Status)
	}
	m := apiTravada(t, "mcp", "")
	if r, _ := m.Chamar(context.Background(), Pedido{Metodo: "GET", Caminho: Prefixo + "/conta", Autor: "eu"}); r.Status != 403 {
		t.Fatalf("GET /conta no canal mcp = %d", r.Status)
	}
}
