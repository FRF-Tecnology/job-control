package dominio

import (
	"errors"
	"testing"
	"time"
)

// jc-a2y.6.13: aprovar a lista do dia em lote. Dados fictícios.
func TestAprovarLote(t *testing.T) {
	agora := time.Date(2026, 11, 9, 9, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	limites := map[string]int{"linkedin": 3}
	e := EstadoVazio()
	for _, n := range []string{"1", "2", "3", "4", "5"} {
		e = comVaga(t, e, "li-"+n, "linkedin", linkLI(n), true)
	}
	e = comVaga(t, e, "gp-1", "gupy", "https://acme.gupy.io/jobs/1", true)
	var c [6]Candidatura
	for i, v := range []string{"li-1", "li-2", "li-3", "li-4", "li-5", "gp-1"} {
		e, c[i] = preparada(t, e, v, agora)
	}
	// li-1 já aprovada antes (conta no limite); li-2 editada depois que a tela mostrou (impressão velha).
	e, _, err := AprovarCandidatura(e, c[0].ID, c[0].Impressao, agora, limites, eu)
	if err != nil {
		t.Fatal(err)
	}
	velha := c[1].Impressao
	novoConteudo := conteudoOK()
	novoConteudo["idioma"] = "en"
	e, _, err = EditarConteudo(e, c[1].ID, novoConteudo, c[1].Versao, agora, eu)
	if err != nil {
		t.Fatal(err)
	}
	antes := emJSONTeste(e)
	itens := []ItemLote{
		{c[0].ID, c[0].Impressao},                          // já aprovada: estado
		{c[1].ID, velha},                                   // impressão velha
		{c[2].ID, c[2].Impressao},                          // 2ª do linkedin (1 pendente + esta)
		{c[3].ID, c[3].Impressao},                          // 3ª: completa o limite
		{c[4].ID, c[4].Impressao},                          // passou do limite
		{c[5].ID, c[5].Impressao},                          // gupy: limite próprio
		{"cand-2026-11-09-9999-zzzz", "sha256:v1:" + "ab"}, // não existe
	}
	novo, aprovadas, recusadas, err := AprovarLote(e, itens, agora, limites, eu)
	if err != nil {
		t.Fatal(err)
	}
	if emJSONTeste(e) != antes {
		t.Fatal("mudou o estado original")
	}
	quer := []string{c[2].ID, c[3].ID, c[5].ID}
	if len(aprovadas) != 3 || aprovadas[0] != quer[0] || aprovadas[1] != quer[1] || aprovadas[2] != quer[2] {
		t.Fatalf("aprovadas = %v; queria %v", aprovadas, quer)
	}
	motivos := map[string]string{}
	for _, r := range recusadas {
		motivos[r.ID] = r.Motivo
	}
	for id, m := range map[string]string{c[0].ID: RecusaEstado, c[1].ID: RecusaImpressaoMudou, c[4].ID: RecusaLimite, "cand-2026-11-09-9999-zzzz": RecusaNaoEncontrada} {
		if motivos[id] != m {
			t.Errorf("%s: motivo %q; queria %q", id, motivos[id], m)
		}
	}
	for _, id := range quer {
		a := estadoDe(t, novo, id)
		if a.Estado != CandAprovada || a.Aprovacao == nil || a.Aprovacao.ValidaAte != FimDoDia(agora).Format(time.RFC3339) {
			t.Errorf("%s = %+v", id, a)
		}
	}
	if estadoDe(t, novo, c[1].ID).Estado != CandPreparada || estadoDe(t, novo, c[4].ID).Estado != CandPreparada {
		t.Error("recusada mudou de estado")
	}
	if n := AprovadasPendentes(novo, "linkedin", agora); n != 3 {
		t.Errorf("pendentes = %d", n)
	}
	// Vencida não conta.
	if n := AprovadasPendentes(novo, "linkedin", agora.Add(24*time.Hour)); n != 0 {
		t.Errorf("pendentes amanhã = %d", n)
	}
}

func TestAprovarLoteRecusaTudoAntes(t *testing.T) {
	agora := time.Date(2026, 11, 9, 9, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	e := comVaga(t, EstadoVazio(), "li-1", "linkedin", linkLI("1"), true)
	e, c := preparada(t, e, "li-1", agora)
	it := []ItemLote{{c.ID, c.Impressao}}
	// Só a pessoa: nada aprovado.
	if _, a, _, err := AprovarLote(e, it, agora, nil, claude); err == nil || len(a) != 0 {
		t.Fatalf("assistente aprovou: %v %v", a, err)
	} else {
		var sp *ErroSoPessoa
		if !errors.As(err, &sp) {
			t.Fatalf("erro = %T", err)
		}
	}
	// O domínio confere o autor antes dos itens (trava própria): mesmo com itens que seriam todos recusados.
	if _, _, r, err := AprovarLote(e, []ItemLote{{"cand-2026-11-09-9999-zzzz", c.Impressao}}, agora, nil, claude); err == nil {
		t.Fatalf("assistente: sem erro, recusadas %v", r)
	}
	// id repetido, vazio, grande demais: validação e nada muda.
	grande := make([]ItemLote, MaxItensLote+1)
	for i := range grande {
		grande[i] = ItemLote{ID: "cand-" + string(rune('a'+i%26)) + string(rune('a'+i/26)), Impressao: c.Impressao}
	}
	for nome, itens := range map[string][]ItemLote{"repetido": {it[0], it[0]}, "vazio": {}, "grande": grande} {
		novo, a, _, err := AprovarLote(e, itens, agora, nil, eu)
		var ev *ErroValidacao
		if !errors.As(err, &ev) || len(a) != 0 || estadoDe(t, novo, c.ID).Estado != CandPreparada {
			t.Errorf("%s: %v %v", nome, a, err)
		}
	}
	// Vaga saiu de aplicar: vaga-fora.
	e2, _, err := MoverVaga(e, "li-1", "pulada", Objeto{"motivo": "Outro"}, eu)
	if err != nil {
		t.Fatal(err)
	}
	_, a, r, err := AprovarLote(e2, it, agora, nil, eu)
	if err != nil || len(a) != 0 || len(r) != 1 || r[0].Motivo != RecusaVagaFora {
		t.Fatalf("vaga fora = %v %v %v", a, r, err)
	}
}

// Decisão do CyberSec no #102: o aprovar de uma também conta as aprovadas que ainda não saíram.
func TestAprovarDeUmaContaAsPendentes(t *testing.T) {
	agora := time.Date(2026, 11, 9, 9, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	limites := map[string]int{"linkedin": 2}
	e := EstadoVazio()
	for _, n := range []string{"1", "2", "3"} {
		e = comVaga(t, e, "li-"+n, "linkedin", linkLI(n), true)
	}
	e, _ = aprovada(t, e, "li-1", agora, limites)
	e, _ = aprovada(t, e, "li-2", agora, limites) // 2 aprovadas que não saíram: o dia está cheio
	e, c3 := preparada(t, e, "li-3", agora)
	_, _, err := AprovarCandidatura(e, c3.ID, c3.Impressao, agora, limites, eu)
	var el *ErroLimite
	if !errors.As(err, &el) || el.Limite != 2 {
		t.Fatalf("3ª aprovação com 2 pendentes = %v", err)
	}
	// Amanhã as pendentes venceram: aprova de novo.
	amanha := agora.Add(24 * time.Hour)
	e, _ = VencerAprovacoes(e, amanha)
	if _, _, err := AprovarCandidatura(e, c3.ID, c3.Impressao, amanha, limites, eu); err != nil {
		t.Fatalf("no dia seguinte = %v", err)
	}
}
