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

// Decisão do PO e do Fallen (#102): o aprovar de uma conta, no limite do dia, as enviadas hoje e as aprovadas que
// ainda não saíram, como o lote. O executor (IniciarEnvio) continua contando só as tentativas: a candidatura não
// conta contra ela mesma. Dados fictícios.
func TestAprovarDeUmaContaAsAprovadas(t *testing.T) {
	agora := time.Date(2026, 11, 9, 9, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	limites := map[string]int{"linkedin": 2}
	e := EstadoVazio()
	var c [3]Candidatura
	for i, n := range []string{"1", "2", "3"} {
		e = comVaga(t, e, "li-"+n, "linkedin", linkLI(n), true)
		e, c[i] = preparada(t, e, "li-"+n, agora)
	}
	var err error
	for _, x := range c[:2] {
		if e, _, err = AprovarCandidatura(e, x.ID, x.Impressao, agora, limites, eu); err != nil {
			t.Fatal(err)
		}
	}
	// 0 enviadas + 2 aprovadas pendentes = limite 2: a 3ª dá limite (429 na API), e nada muda.
	novo, _, err := AprovarCandidatura(e, c[2].ID, c[2].Impressao, agora, limites, eu)
	var el *ErroLimite
	if !errors.As(err, &el) || estadoDe(t, novo, c[2].ID).Estado != CandPreparada {
		t.Fatalf("3ª aprovada com 2 pendentes e limite 2: %v", err)
	}
	// O executor envia as duas aprovadas: nenhuma conta contra ela mesma.
	for _, x := range c[:2] {
		atual := estadoDe(t, e, x.ID)
		if e, _, err = IniciarEnvio(e, x.ID, atual.Versao, agora, limites); err != nil {
			t.Fatalf("executor recusou %s dentro do limite: %v", x.ID, err)
		}
	}
	// 2 enviadas + 0 pendentes: continua no limite.
	if _, _, err = AprovarCandidatura(e, c[2].ID, c[2].Impressao, agora, limites, eu); !errors.As(err, &el) {
		t.Fatalf("3ª aprovada com 2 enviadas: %v", err)
	}
}

// jc-a2y.6.11: a volta automática devolve a candidatura para aprovada e tira a tentativa de tentativas. Ela conta
// uma vez só, como aprovada pendente, mesmo com a aprovação já marcada como usada.
func TestVoltaAutomaticaContaUmaVez(t *testing.T) {
	agora := time.Date(2026, 11, 9, 9, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	limites := map[string]int{"linkedin": 2}
	e := EstadoVazio()
	var c [3]Candidatura
	for i, n := range []string{"1", "2", "3"} {
		e = comVaga(t, e, "li-"+n, "linkedin", linkLI(n), true)
		e, c[i] = preparada(t, e, "li-"+n, agora)
	}
	var err error
	for _, x := range c[:2] {
		if e, _, err = AprovarCandidatura(e, x.ID, x.Impressao, agora, limites, eu); err != nil {
			t.Fatal(err)
		}
	}
	if e, _, err = IniciarEnvio(e, c[0].ID, estadoDe(t, e, c[0].ID).Versao, agora, limites); err != nil {
		t.Fatal(err)
	}
	// Em enviando: 1 tentativa + 1 pendente.
	if u, p := UsadasNoDia(e, "linkedin", agora), AprovadasPendentes(e, "linkedin", agora); u+p != 2 {
		t.Fatalf("enviando: usadas %d + pendentes %d; queria 2", u, p)
	}
	// A volta, como o .6.11 descreve (a tentativa sai de tentativas; a marca de uso pode ficar).
	i, v, err := acharCandidatura(e, c[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	v.Estado, v.Tentativas = CandAprovada, nil
	e, _ = salvarCandidatura(e, i, v, agora)
	if u, p := UsadasNoDia(e, "linkedin", agora), AprovadasPendentes(e, "linkedin", agora); u != 0 || p != 2 {
		t.Fatalf("depois da volta: usadas %d, pendentes %d; queria 0 e 2", u, p)
	}
	var el *ErroLimite
	if _, _, err = AprovarCandidatura(e, c[2].ID, c[2].Impressao, agora, limites, eu); !errors.As(err, &el) {
		t.Fatalf("3ª aprovada com uma volta pendente: %v", err)
	}
	if _, _, r, _ := AprovarLote(e, []ItemLote{{c[2].ID, c[2].Impressao}}, agora, limites, eu); len(r) != 1 || r[0].Motivo != RecusaLimite {
		t.Fatalf("lote com uma volta pendente: %v", r)
	}
}

// "outra" não tem limite (só envio manual): no lote ela é aprovada como no aprovar de uma, nunca recusada por
// "limite". Dados fictícios.
func TestAprovarLoteOutraSemLimite(t *testing.T) {
	agora := time.Date(2026, 11, 9, 9, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	e := comVaga(t, EstadoVazio(), "ou-1", "outra", "https://vagas.exemplo.com/1", true)
	e, c := preparada(t, e, "ou-1", agora)
	if _, _, err := AprovarCandidatura(e, c.ID, c.Impressao, agora, nil, eu); err != nil {
		t.Fatalf("aprovar de uma: %v", err)
	}
	_, a, r, err := AprovarLote(e, []ItemLote{{c.ID, c.Impressao}}, agora, nil, eu)
	if err != nil || len(a) != 1 || len(r) != 0 {
		t.Fatalf("lote com outra: aprovadas %v, recusadas %v, %v", a, r, err)
	}
}

// jc-a2y.6.11 + jc-a2y.6.13 (combinado, como o PO pediu): a volta automática de verdade (DevolverAprovada) tira a
// tentativa de tentativas e devolve a candidatura para aprovada. No limite do dia ela conta uma vez só, como
// aprovada pendente, no aprovar de uma e no lote. Dados fictícios.
func TestVoltaDeVerdadeNoLimiteDoDia(t *testing.T) {
	agora := time.Date(2026, 11, 9, 9, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	limites := map[string]int{"linkedin": 2}
	e := EstadoVazio()
	var c [3]Candidatura
	for i, n := range []string{"1", "2", "3"} {
		e = comVaga(t, e, "li-"+n, "linkedin", linkLI(n), true)
		e, c[i] = preparada(t, e, "li-"+n, agora)
	}
	var err error
	for _, x := range c[:2] {
		if e, _, err = AprovarCandidatura(e, x.ID, x.Impressao, agora, limites, eu); err != nil {
			t.Fatal(err)
		}
	}
	if e, _, err = IniciarEnvio(e, c[0].ID, estadoDe(t, e, c[0].ID).Versao, agora, limites); err != nil {
		t.Fatal(err)
	}
	if e, _, err = DevolverAprovada(e, c[0].ID, estadoDe(t, e, c[0].ID).Versao, "erro-tecnico", agora); err != nil {
		t.Fatal(err)
	}
	if v := estadoDe(t, e, c[0].ID); v.Estado != CandAprovada {
		t.Fatalf("devolvida = %s", v.Estado)
	}
	if u, p := UsadasNoDia(e, "linkedin", agora), AprovadasPendentes(e, "linkedin", agora); u != 0 || p != 2 {
		t.Fatalf("depois da volta: usadas %d, pendentes %d; queria 0 e 2", u, p)
	}
	var el *ErroLimite
	if _, _, err = AprovarCandidatura(e, c[2].ID, c[2].Impressao, agora, limites, eu); !errors.As(err, &el) {
		t.Fatalf("3ª aprovada com uma devolvida pendente: %v", err)
	}
	if _, _, r, _ := AprovarLote(e, []ItemLote{{c[2].ID, c[2].Impressao}}, agora, limites, eu); len(r) != 1 || r[0].Motivo != RecusaLimite {
		t.Fatalf("lote com uma devolvida pendente: %v", r)
	}
	// A devolvida sai de novo pelo executor sem contar contra ela mesma.
	if _, _, err = IniciarEnvio(e, c[0].ID, estadoDe(t, e, c[0].ID).Versao, agora.Add(10*time.Minute), limites); err != nil {
		t.Fatalf("a devolvida não saiu de novo: %v", err)
	}
}
