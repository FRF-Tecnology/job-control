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

// Achado médio do CyberSec no #105: a confirmação confere de novo as regras da ação, porque a proposta guardada pode
// ter sido alterada (bug, migração, banco mexido) com a impressão recalculada. Qualquer coisa fora das listas
// fechadas, ou uma chamada que não sai dos parâmetros, dá conflito e nada muda. Dados fictícios.
func adulterar(t *testing.T, e Estado, id, acao string, params, chamada Objeto) (Estado, string) {
	t.Helper()
	novo := e.Clonar()
	i, err := acharPedido(novo, id)
	if err != nil {
		t.Fatal(err)
	}
	impressao, err := ImpressaoProposta(id, acao, params)
	if err != nil {
		t.Fatal(err)
	}
	prop := clonarObj(ObjetoDe(novo.Pedidos[i]["proposta"]))
	prop["acao"], prop["parametros"], prop["chamada"], prop["impressao"] = acao, params, chamada, impressao
	novo.Pedidos[i]["proposta"] = prop
	return novo, impressao
}

func TestConfirmarConfereDeNovoAsRegrasDaAcao(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	base := comVaga(t, EstadoVazio(), "li-2", "linkedin", linkLI("2"), false) // em descoberta
	base, p, err := CriarPedido(base, "pula a li-2", eu)
	if err != nil {
		t.Fatal(err)
	}
	id := texto(p["id"])
	base, p, err = InterpretarPedido(base, id, Objeto{"acao": "registrar-etapa", "parametros": Objeto{"vagaId": "li-2", "etapa": "pulada", "motivo": "Outro"}}, agora, eu)
	if err != nil || p["status"] != PedidoProposta {
		t.Fatalf("proposta válida = %v %v", p, err)
	}
	mover := func(corpo Objeto) Objeto {
		return Objeto{"metodo": "POST", "caminho": "/vagas/li-2/mover", "corpo": corpo}
	}
	vagas := func(corpo Objeto) Objeto { return Objeto{"metodo": "POST", "caminho": "/vagas", "corpo": corpo} }
	casos := map[string]struct {
		acao    string
		params  Objeto
		chamada Objeto
	}{
		// O cenário do CyberSec: a proposta guardada vira "aplicar", com a impressão recalculada.
		"etapa aplicar":         {"registrar-etapa", Objeto{"vagaId": "li-2", "etapa": "aplicar"}, mover(Objeto{"etapa": "aplicar"})},
		"etapa descoberta":      {"registrar-etapa", Objeto{"vagaId": "li-2", "etapa": "descoberta"}, mover(Objeto{"etapa": "descoberta"})},
		"etapa bloqueada":       {"registrar-etapa", Objeto{"vagaId": "li-2", "etapa": "bloqueada"}, mover(Objeto{"etapa": "bloqueada"})},
		"motivo fora da lista":  {"registrar-etapa", Objeto{"vagaId": "li-2", "etapa": "pulada", "motivo": "Qualquer"}, mover(Objeto{"etapa": "pulada", "motivo": "Qualquer"})},
		"motivo de encerrada":   {"registrar-etapa", Objeto{"vagaId": "li-2", "etapa": "pulada", "motivo": "Reprovada"}, mover(Objeto{"etapa": "pulada", "motivo": "Reprovada"})},
		"pulada sem motivo":     {"registrar-etapa", Objeto{"vagaId": "li-2", "etapa": "pulada"}, mover(Objeto{"etapa": "pulada"})},
		"sub fora de processo":  {"registrar-etapa", Objeto{"vagaId": "li-2", "etapa": "pulada", "motivo": "Outro", "sub": "teste"}, mover(Objeto{"etapa": "pulada", "motivo": "Outro", "subEtapa": "teste"})},
		"campo a mais":          {"registrar-etapa", Objeto{"vagaId": "li-2", "etapa": "pulada", "motivo": "Outro", "pendencia": "x"}, mover(Objeto{"etapa": "pulada", "motivo": "Outro"})},
		"corpo com pendência":   {"registrar-etapa", Objeto{"vagaId": "li-2", "etapa": "pulada", "motivo": "Outro"}, mover(Objeto{"etapa": "pulada", "motivo": "Outro", "pendencia": "x"})},
		"corpo de outra etapa":  {"registrar-etapa", Objeto{"vagaId": "li-2", "etapa": "pulada", "motivo": "Outro"}, mover(Objeto{"etapa": "aplicar"})},
		"caminho de outra vaga": {"registrar-etapa", Objeto{"vagaId": "li-2", "etapa": "pulada", "motivo": "Outro"}, Objeto{"metodo": "POST", "caminho": "/vagas/li-9/mover", "corpo": Objeto{"etapa": "pulada", "motivo": "Outro"}}},
		"vagaId fora do padrão": {"registrar-etapa", Objeto{"vagaId": "li-2/../x", "etapa": "pulada", "motivo": "Outro"}, mover(Objeto{"etapa": "pulada", "motivo": "Outro"})},
		"url http":              {"adicionar-vaga", Objeto{"url": "http://www.linkedin.com/jobs/view/1"}, vagas(Objeto{"link": "http://www.linkedin.com/jobs/view/1"})},
		"url com @":             {"adicionar-vaga", Objeto{"url": "https://www.linkedin.com@exemplo.com/1"}, vagas(Objeto{"link": "https://www.linkedin.com@exemplo.com/1"})},
		"url javascript":        {"adicionar-vaga", Objeto{"url": "javascript:alert(1)"}, vagas(Objeto{"link": "javascript:alert(1)"})},
		"url com IDN":           {"adicionar-vaga", Objeto{"url": "https://lіnkedin.com/jobs/1"}, vagas(Objeto{"link": "https://lіnkedin.com/jobs/1"})},
		"url com //":            {"adicionar-vaga", Objeto{"url": "https://www.linkedin.com//exemplo.com"}, vagas(Objeto{"link": "https://www.linkedin.com//exemplo.com"})},
		"link diferente da url": {"adicionar-vaga", Objeto{"url": "https://www.linkedin.com/jobs/view/1"}, vagas(Objeto{"link": "https://exemplo.com/1"})},
		"vaga com etapa":        {"adicionar-vaga", Objeto{"url": "https://www.linkedin.com/jobs/view/1"}, vagas(Objeto{"link": "https://www.linkedin.com/jobs/view/1", "etapa": "aplicar"})},
		"empresa grande":        {"adicionar-vaga", Objeto{"url": "https://www.linkedin.com/jobs/view/1", "empresa": string(make([]byte, 121))}, vagas(Objeto{"link": "https://www.linkedin.com/jobs/view/1", "empresa": string(make([]byte, 121))})},
		"ação aprovar":          {"aprovar", Objeto{"vagaId": "li-2"}, mover(Objeto{"etapa": "aplicar"})},
	}
	for nome, c := range casos {
		e, impressao := adulterar(t, base, id, c.acao, c.params, c.chamada)
		antes := emJSONTeste(e)
		ch := c.chamada
		novo, _, err := ConfirmarProposta(e, id, impressao, texto(ch["metodo"]), texto(ch["caminho"]), ch["corpo"], eu)
		var ec *ErroConflito
		if !errors.As(err, &ec) {
			t.Errorf("%s: confirmou (%v)", nome, err)
			continue
		}
		if emJSONTeste(novo) != antes || len(novo.Vagas) != 1 || !igual(novo.Vagas[0]["etapa"], "descoberta") {
			t.Errorf("%s: mudou o estado", nome)
		}
	}
	// No cenário do CyberSec, quem barra é a conferência das regras (e não só a comparação da chamada guardada).
	c := casos["etapa aplicar"]
	e, impressao := adulterar(t, base, id, c.acao, c.params, c.chamada)
	_, _, err = ConfirmarProposta(e, id, impressao, "POST", "/vagas/li-2/mover", Objeto{"etapa": "aplicar"}, eu)
	if err == nil || err.Error() != "a proposta não passa nas regras da ação" {
		t.Fatalf("etapa aplicar: %v", err)
	}
	// A proposta válida continua passando.
	prop := ObjetoDe(base.Pedidos[0]["proposta"])
	ch := ObjetoDe(prop["chamada"])
	novo, v, err := ConfirmarProposta(base, id, texto(prop["impressao"]), "POST", texto(ch["caminho"]), ch["corpo"], eu)
	if err != nil || !igual(v["etapa"], "pulada") || !igual(novo.Pedidos[0]["status"], PedidoFeito) {
		t.Fatalf("válida = %v %v", v, err)
	}
}
