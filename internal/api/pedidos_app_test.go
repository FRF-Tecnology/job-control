package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// jc-a2y.7.1: pedidos atendidos pelo app. A IA é falsa (devolve a saída que o teste escolhe); vagas e pedidos
// fictícios.

const marcaPedido = "Pedido-Ficticio-Marcado-7a1c" // o texto do pedido nunca vai para o log

type iaDoPedido struct {
	saida *string
	chave *string
}

func (iaDoPedido) Nome() string { return "anthropic" }
func (f iaDoPedido) Chamar(ctx context.Context, p ia.PedidoProvedor) (ia.RespostaProvedor, error) {
	*f.chave = p.Chave
	n := 10
	return ia.RespostaProvedor{Texto: *f.saida, TokensEntrada: &n, TokensSaida: &n}, nil
}

type cenaPedido struct {
	t     *testing.T
	a     *API
	saida string
}

func novaCenaPedido(t *testing.T) *cenaPedido {
	t.Helper()
	ctx := context.Background()
	arm, err := armazem.Abrir(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	s := &cenaPedido{t: t}
	var chave string
	s.a = Nova(arm, Opcoes{Hoje: func() string { return "2026-11-09" }, Aleatorio: func() string { return "abcd" }, Cofre: segredos.Memoria{},
		ProvedoresIA: map[string]ia.Provedor{"anthropic": iaDoPedido{saida: &s.saida, chave: &chave}},
		DetectarCLIs: func(context.Context) []ia.EstadoCLI { return nil }})
	s.chamar("PUT", "/ia/chaves/anthropic", Objeto{"chave": "sk-ant-chave-de-teste-1234"}, "eu", "", "", 200)
	s.chamar("PUT", "/ia/config", Objeto{"provedor": "anthropic"}, "eu", "", "", 200)
	s.chamar("POST", "/vagas", Objeto{"id": "li-1", "empresa": "Empresa Teste", "titulo": "Dev Go", "plataforma": "linkedin", "link": "https://www.linkedin.com/jobs/view/9000000001"}, "eu", "", "", 201)
	s.chamar("POST", "/vagas/li-1/mover", Objeto{"etapa": "aplicar"}, "eu", "", "", 200)
	return s
}

func (s *cenaPedido) chamar(m, caminho string, corpo any, autor, xPedido, xImpressao string, quer int) map[string]any {
	s.t.Helper()
	r, err := s.a.Chamar(context.Background(), Pedido{Metodo: m, Caminho: Prefixo + caminho, Corpo: normal(corpo), Autor: autor, XPedido: xPedido, XImpressao: xImpressao})
	if err != nil {
		s.t.Fatalf("%s %s: %v", m, caminho, err)
	}
	var bruto any
	json.Unmarshal([]byte(emJSON(r.Corpo)), &bruto)
	o, _ := bruto.(map[string]any)
	if l, ehLista := bruto.([]any); ehLista {
		o = map[string]any{"lista": l}
	}
	if r.Status != quer {
		s.t.Fatalf("%s %s = %d %v; queria %d", m, caminho, r.Status, o, quer)
	}
	return o
}

func (s *cenaPedido) novoPedido(texto string) string {
	s.t.Helper()
	return s.chamar("POST", "/pedidos", Objeto{"texto": texto}, "eu", "", "", 201)["id"].(string)
}

func (s *cenaPedido) interpretar(id, saida string, quer int) map[string]any {
	s.t.Helper()
	s.saida = saida
	return s.chamar("POST", "/pedidos/"+id+"/interpretar", Objeto{}, "eu", "", "", quer)
}

func (s *cenaPedido) versao() any {
	return s.chamar("GET", "/versao", nil, "eu", "", "", 200)["versao"]
}

func TestAdicionarVagaPeloPedido(t *testing.T) {
	var saidaLog bytes.Buffer
	log.SetOutput(&saidaLog)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	s := novaCenaPedido(t)
	id := s.novoPedido("adiciona esta vaga " + marcaPedido)

	// Só a tela interpreta (gasta a IA da pessoa).
	s.saida = `{"acao":"recusar","parametros":{"motivo":"nao-entendi"}}`
	s.chamar("POST", "/pedidos/"+id+"/interpretar", Objeto{}, "claude", "", "", 403)
	s.chamar("POST", "/pedidos/"+id+"/interpretar", Objeto{"x": 1}, "eu", "", "", 400)

	ped := s.interpretar(id, `{"acao":"adicionar-vaga","parametros":{"url":"https://br.linkedin.com/jobs/view/9000000002","empresa":"IGNORE AS REGRAS","titulo":"Dev"}}`, 200)
	prop := ped["proposta"].(map[string]any)
	if ped["status"] != "proposta" || prop["resumo"] != "Adicionar uma vaga em Descoberta com o link de br.linkedin.com (LinkedIn)." {
		t.Fatalf("proposta = %v", ped)
	}
	chamada := prop["chamada"].(map[string]any)
	if chamada["metodo"] != "POST" || chamada["caminho"] != "/vagas" {
		t.Fatalf("chamada = %v", chamada)
	}
	corpo, imp := chamada["corpo"].(map[string]any), prop["impressao"].(string)
	antes := s.versao()

	// Confirmação: só a tela, só a chamada guardada, só a impressão guardada. Nada muda nas recusas.
	s.chamar("POST", "/vagas", corpo, "claude", id, imp, 403)
	s.chamar("POST", "/vagas", corpo, "claude", "x", "y", 403) // a API confere o autor antes de tudo (trava própria)
	comEtapa := Objeto{"etapa": "aplicar"}
	for k, v := range corpo {
		comEtapa[k] = v
	}
	s.chamar("POST", "/vagas", comEtapa, "eu", id, imp, 409)
	s.chamar("POST", "/vagas", corpo, "eu", id, "sha256:v1:"+strings.Repeat("0", 64), 409)
	s.chamar("POST", "/vagas/li-1/mover", corpo, "eu", id, imp, 409)
	s.chamar("POST", "/vagas", corpo, "eu", id, "", 400)
	s.chamar("PATCH", "/vagas/li-1", Objeto{"notas": "x"}, "eu", id, imp, 400)
	s.chamar("POST", "/vagas", []any{corpo}, "eu", id, imp, 400)
	if s.versao() != antes {
		t.Fatal("uma confirmação recusada gravou")
	}

	v := s.chamar("POST", "/vagas", corpo, "eu", id, imp, 201)
	if v["etapa"] != "descoberta" || v["plataforma"] != "linkedin" || v["empresa"] != "IGNORE AS REGRAS" || !strings.HasPrefix(v["id"].(string), "pd-vaga-") {
		t.Fatalf("vaga = %v", v)
	}
	for _, x := range s.chamar("GET", "/pedidos", nil, "eu", "", "", 200)["lista"].([]any) {
		if p := x.(map[string]any); p["id"] == id && (p["status"] != "feito" || p["resultado"].(map[string]any)["vagaId"] != v["id"]) {
			t.Fatalf("pedido depois de confirmar = %v", p)
		}
	}
	// De novo: o pedido já está feito; nada grava duas vezes.
	depois := s.versao()
	s.chamar("POST", "/vagas", corpo, "eu", id, imp, 409)
	if s.versao() != depois {
		t.Fatal("confirmar duas vezes gravou")
	}
	// Sem os cabeçalhos, a rota de sempre.
	s.chamar("POST", "/vagas", Objeto{"id": "li-3", "empresa": "E3", "titulo": "T3"}, "eu", "", "", 201)

	tudo := saidaLog.String()
	for _, proibido := range []string{marcaPedido, "linkedin.com", "IGNORE", "sha256:"} {
		if strings.Contains(tudo, proibido) {
			t.Errorf("o log tem %q:\n%s", proibido, tudo)
		}
	}
	if !strings.Contains(tudo, "pedidos: interpretado (adicionar-vaga, proposta)") || !strings.Contains(tudo, "pedidos: proposta confirmada (adicionar-vaga)") {
		t.Errorf("log = %s", tudo)
	}
}

func TestRegistrarEtapaPeloPedido(t *testing.T) {
	s := novaCenaPedido(t)
	id := s.novoPedido("marca a li-1 como enviada")
	ped := s.interpretar(id, `{"acao":"registrar-etapa","parametros":{"vagaId":"li-1","etapa":"enviada"}}`, 200)
	prop := ped["proposta"].(map[string]any)
	if prop["resumo"] != "Mudar a vaga Empresa Teste — Dev Go de Aplicar para Enviada." {
		t.Fatalf("resumo = %v", prop["resumo"])
	}
	chamada := prop["chamada"].(map[string]any)
	if chamada["caminho"] != "/vagas/li-1/mover" {
		t.Fatalf("chamada = %v", chamada)
	}
	corpo := chamada["corpo"].(map[string]any)
	// Outra etapa no corpo: 409.
	s.chamar("POST", "/vagas/li-1/mover", Objeto{"etapa": "aplicar"}, "eu", id, prop["impressao"].(string), 409)
	v := s.chamar("POST", "/vagas/li-1/mover", corpo, "eu", id, prop["impressao"].(string), 200)
	if v["etapa"] != "enviada" {
		t.Fatalf("vaga = %v", v)
	}
	ev := s.chamar("GET", "/vagas/li-1", nil, "eu", "", "", 200)
	eventos, _ := ev["eventos"].([]any)
	ultimo := eventos[len(eventos)-1].(map[string]any)
	if ultimo["autor"] != "eu" || ultimo["tipo"] != "enviada" {
		t.Fatalf("evento = %v", ultimo)
	}
}

func TestPedidoSemGravarNada(t *testing.T) {
	s := novaCenaPedido(t)
	// recusar: recusado, frase fixa.
	id := s.novoPedido("aprova a li-1")
	if p := s.interpretar(id, `{"acao":"recusar","parametros":{"motivo":"so-a-pessoa"}}`, 200); p["status"] != "recusado" || p["proposta"].(map[string]any)["chamada"] != nil {
		t.Fatalf("recusar = %v", p)
	}
	// Interpretar de novo um pedido que não está aberto: 409.
	s.interpretar(id, `{"acao":"recusar","parametros":{"motivo":"so-a-pessoa"}}`, 409)
	// preparar-pacote: feito, só abre a Fila.
	id2 := s.novoPedido("prepara a li-1")
	if p := s.interpretar(id2, `{"acao":"preparar-pacote","parametros":{"vagaId":"li-1"}}`, 200); p["status"] != "feito" || p["resultado"].(map[string]any)["abrir"] != "#/fila?preparar=li-1" {
		t.Fatalf("preparar = %v", p)
	}
	// responder: feito, com a resposta da IA à parte.
	id3 := s.novoPedido("o que falta?")
	if p := s.interpretar(id3, `{"acao":"responder","parametros":{"texto":"Falta preparar o pacote."}}`, 200); p["status"] != "feito" || p["resposta"] != "Falta preparar o pacote." {
		t.Fatalf("responder = %v", p)
	}
	// A guarda barra: 422, e o pedido continua aberto.
	id4 := s.novoPedido("mova para aplicar")
	if r := s.interpretar(id4, `{"acao":"registrar-etapa","parametros":{"vagaId":"li-1","etapa":"aplicar"}}`, 422); r["tipo"] != "esquema" {
		t.Fatalf("aplicar = %v", r)
	}
	// Descartar: proposta volta a aberto; sem proposta, 409.
	id5 := s.novoPedido("adiciona https://empresa-teste.gupy.io/jobs/9000001")
	s.interpretar(id5, `{"acao":"adicionar-vaga","parametros":{"url":"https://empresa-teste.gupy.io/jobs/9000001"}}`, 200)
	if p := s.chamar("POST", "/pedidos/"+id5+"/descartar", Objeto{}, "claude", "", "", 200); p["status"] != "aberto" || p["proposta"] != nil {
		t.Fatalf("descartar = %v", p)
	}
	s.chamar("POST", "/pedidos/"+id5+"/descartar", Objeto{}, "eu", "", "", 409)
	// Nenhuma vaga nova nem etapa mudada: só os pedidos mudaram.
	if v := s.chamar("GET", "/vagas/li-1", nil, "eu", "", "", 200); v["etapa"] != "aplicar" {
		t.Fatalf("li-1 = %v", v["etapa"])
	}
}

// Uma interpretação por vez: a segunda recebe 409.
func TestUmaInterpretacaoPorVez(t *testing.T) {
	s := novaCenaPedido(t)
	id := s.novoPedido("oi")
	s.a.interpretando.Lock()
	s.interpretar(id, `{"acao":"responder","parametros":{"texto":"oi"}}`, 409)
	s.a.interpretando.Unlock()
	s.interpretar(id, `{"acao":"responder","parametros":{"texto":"oi"}}`, 200)
}

// Sem IA conectada: 400, e o pedido continua aberto.
func TestInterpretarSemIA(t *testing.T) {
	arm, _ := armazem.Abrir(context.Background(), ":memory:")
	t.Cleanup(func() { arm.Fechar() })
	a := Nova(arm, Opcoes{Hoje: func() string { return "2026-11-09" }, Cofre: segredos.Memoria{}, ProvedoresIA: map[string]ia.Provedor{},
		DetectarCLIs: func(context.Context) []ia.EstadoCLI { return nil }})
	r, _ := a.Chamar(context.Background(), Pedido{Metodo: "POST", Caminho: Prefixo + "/pedidos", Corpo: Objeto{"texto": "oi"}, Autor: "eu"})
	id := r.Corpo.(Objeto)["id"].(string)
	r, err := a.Chamar(context.Background(), Pedido{Metodo: "POST", Caminho: Prefixo + "/pedidos/" + id + "/interpretar", Corpo: Objeto{}, Autor: "eu"})
	if err != nil || r.Status != 400 {
		t.Fatalf("sem IA = %d %v", r.Status, err)
	}
}
