package dominio

import (
	"errors"
	"testing"
	"time"
)

// jc-a2y.7.1: proposta e confirmação dos pedidos atendidos pelo app. Dados fictícios.

func TestPlataformaDoHostListaExata(t *testing.T) {
	for host, quer := range map[string]string{
		"www.linkedin.com": "linkedin", "linkedin.com": "linkedin", "br.linkedin.com": "linkedin",
		"linkedin.com.exemplo.com": "outra", "evil-linkedin.com": "outra", "www.linkedin.com.br": "outra", "xn--lnkedin-xxx.com": "outra",
		"acme.gupy.io": "gupy", "login.gupy.io": "outra", "portal.gupy.io": "outra", "gupy.io": "outra",
		"acme.inhire.app": "inhire", "form-app.inhire.app": "outra", "a.b.inhire.app": "outra",
	} {
		if got := PlataformaDoHost(host); got != quer {
			t.Errorf("%s = %s; queria %s", host, got, quer)
		}
	}
	if HostDoLink("https://BR.LinkedIn.com./jobs/view/1") != "br.linkedin.com" {
		t.Error("host não normalizado")
	}
}

func estadoComPedido(t *testing.T) (Estado, string) {
	t.Helper()
	e := comVaga(t, EstadoVazio(), "li-1", "linkedin", linkLI("1"), true)
	e, p, err := CriarPedido(e, "marca a li-1 como enviada", eu)
	if err != nil {
		t.Fatal(err)
	}
	return e, texto(p["id"])
}

func TestEnviadaComCandidaturaNaFila(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	e, id := estadoComPedido(t)
	saida := Objeto{"acao": "registrar-etapa", "parametros": Objeto{"vagaId": "li-1", "etapa": "enviada"}}
	// Sem candidatura: proposta; a confirmação passa.
	e2, p, err := InterpretarPedido(e, id, saida, agora, eu)
	if err != nil || p["status"] != PedidoProposta {
		t.Fatalf("proposta = %v %v", p, err)
	}
	// A pessoa prepara um pacote na Fila depois da proposta: a confirmação dá conflito, nada muda.
	e3, _ := preparada(t, e2, "li-1", agora)
	prop := ObjetoDe(e3.Pedidos[0]["proposta"])
	chamada := ObjetoDe(prop["chamada"])
	_, _, err = ConfirmarProposta(e3, id, texto(prop["impressao"]), "POST", texto(chamada["caminho"]), chamada["corpo"], eu)
	var ec *ErroConflito
	if !errors.As(err, &ec) {
		t.Fatalf("confirmar com candidatura na Fila = %v", err)
	}
	// Interpretar com candidatura já na Fila: recusado.
	e4, id2 := e3, ""
	e4, p2, _ := CriarPedido(e4, "marca a li-1 como enviada", eu)
	id2 = texto(p2["id"])
	if _, p, _ := InterpretarPedido(e4, id2, saida, agora, eu); p["status"] != PedidoRecusado || ObjetoDe(p["proposta"])["chamada"] != nil {
		t.Fatalf("recusado = %v", p)
	}
}

func TestConfirmarConfereAImpressaoGuardada(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	e, id := estadoComPedido(t)
	e, _, _ = InterpretarPedido(e, id, Objeto{"acao": "registrar-etapa", "parametros": Objeto{"vagaId": "li-1", "etapa": "pulada", "motivo": "Stack"}}, agora, eu)
	prop := ObjetoDe(e.Pedidos[0]["proposta"])
	chamada := ObjetoDe(prop["chamada"])
	imp := texto(prop["impressao"])
	// Alguém troca os parâmetros guardados direto no banco: a impressão recalculada não bate.
	mexido := e.Clonar()
	ObjetoDe(ObjetoDe(mexido.Pedidos[0]["proposta"])["parametros"])["motivo"] = "Outro"
	if _, _, err := ConfirmarProposta(mexido, id, imp, "POST", texto(chamada["caminho"]), chamada["corpo"], eu); err == nil {
		t.Fatal("confirmou com os parâmetros trocados")
	}
	// O assistente não confirma.
	if _, _, err := ConfirmarProposta(e, id, imp, "POST", texto(chamada["caminho"]), chamada["corpo"], claude); err == nil {
		t.Fatal("o assistente confirmou")
	}
	// A pessoa confirma: a vaga muda e o pedido fica feito, no mesmo estado novo.
	novo, v, err := ConfirmarProposta(e, id, imp, "POST", texto(chamada["caminho"]), chamada["corpo"], eu)
	if err != nil || v["etapa"] != "pulada" || v["motivo"] != "Stack" || novo.Pedidos[0]["status"] != PedidoFeito {
		t.Fatalf("confirmado = %v %v %v", v, novo.Pedidos[0], err)
	}
	if e.Pedidos[0]["status"] != PedidoProposta || vagaPorID(e, "li-1")["etapa"] != "aplicar" {
		t.Fatal("mudou o estado original")
	}
}

// Confirmar duas vezes não grava duas vezes (o pedido já está feito).
func TestConfirmarDuasVezes(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	e := comVaga(t, EstadoVazio(), "gp-2", "gupy", "https://acme.gupy.io/jobs/2", true)
	e, _, _ = MoverVaga(e, "gp-2", "enviada", nil, eu)
	e, p, _ := CriarPedido(e, "entrevista com RH na gp-2", eu)
	id := texto(p["id"])
	e, _, _ = InterpretarPedido(e, id, Objeto{"acao": "registrar-etapa", "parametros": Objeto{"vagaId": "gp-2", "etapa": "em-processo", "sub": "entrevista-rh"}}, agora, eu)
	prop := ObjetoDe(e.Pedidos[0]["proposta"])
	ch := ObjetoDe(prop["chamada"])
	e, _, err := ConfirmarProposta(e, id, texto(prop["impressao"]), "POST", texto(ch["caminho"]), ch["corpo"], eu)
	if err != nil {
		t.Fatal(err)
	}
	eventos := len(e.Eventos)
	if _, _, err := ConfirmarProposta(e, id, texto(prop["impressao"]), "POST", texto(ch["caminho"]), ch["corpo"], eu); err == nil {
		t.Fatal("confirmou duas vezes")
	}
	if len(e.Eventos) != eventos {
		t.Fatal("gravou outro evento")
	}
}

// enviada só a partir de aplicar.
func TestEnviadaSoDeAplicar(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	e := comVaga(t, EstadoVazio(), "li-1", "linkedin", linkLI("1"), false) // em descoberta
	e, p, _ := CriarPedido(e, "marca como enviada", eu)
	if _, p, _ := InterpretarPedido(e, texto(p["id"]), Objeto{"acao": "registrar-etapa", "parametros": Objeto{"vagaId": "li-1", "etapa": "enviada"}}, agora, eu); p["status"] != PedidoRecusado {
		t.Fatalf("enviada de descoberta = %v", p)
	}
}

func TestAdicionarVagaJaNaLista(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	e, id := estadoComPedido(t)
	_, p, err := InterpretarPedido(e, id, Objeto{"acao": "adicionar-vaga", "parametros": Objeto{"url": linkLI("1")}}, agora, eu)
	if err != nil || p["status"] != PedidoRecusado {
		t.Fatalf("duplicada = %v %v", p, err)
	}
}
