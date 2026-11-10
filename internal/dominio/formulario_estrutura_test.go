package dominio

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// Estrutura lida do formulário (jc-a2y.7.2): só em preparada; respostas só por casamento exato com as
// respostasPadrao do perfil (e-mail e telefone pelo contato do pacote); o resto obrigatório vira pendência; a
// estrutura entra na impressão; aprovar com pendência ou sem estrutura dá conflito. Dados fictícios.

func perguntasFicticias() []PerguntaFormulario {
	return []PerguntaFormulario{
		{ID: "p1", Texto: "Qual o seu e-mail?", Tipo: "email", Obrigatoria: true},
		{ID: "p2", Texto: "Quantos anos de Go você tem?", Tipo: "texto-curto", Obrigatoria: true},
		{ID: "p3", Texto: "Por que você quer trabalhar aqui?", Tipo: "texto-longo", Obrigatoria: true},
		{ID: "p4", Texto: "Como soube da vaga?", Tipo: "escolha-unica", Opcoes: []string{"LinkedIn", "Amigo"}},
	}
}

const urlTypeform = "https://acme.typeform.com/to/AbC123"

func formularioPreparado(t *testing.T) (Estado, Candidatura, time.Time) {
	t.Helper()
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	e := comVaga(t, EstadoVazio(), "fm-1", "Site da empresa", urlTypeform, true)
	e.Perfil = Objeto{"respostasPadrao": []any{
		Objeto{"pergunta": "Quantos anos de Go você tem?", "resposta": "3"},
		Objeto{"pergunta": "Pretensão salarial", "resposta": "8500"},
	}}
	e, c := preparada(t, e, "fm-1", agora)
	return e, c, agora
}

func TestGravarEstruturaFormulario(t *testing.T) {
	e, c, agora := formularioPreparado(t)
	impressaoAntes := c.Impressao
	novo, o, err := GravarEstruturaFormulario(e, c.ID, perguntasFicticias(), urlTypeform, agora)
	if err != nil {
		t.Fatal(err)
	}
	g, _ := candidaturaDe(o)
	if g.Estrutura == nil || !strings.HasPrefix(g.Estrutura.Hash, "sha256:v1:") || len(g.Estrutura.Perguntas) != 4 {
		t.Fatalf("estrutura = %+v", g.Estrutura)
	}
	// p1 pelo contato (e-mail do pacote), p2 pela resposta padrão exata; p3 sem par: pendente; p4 opcional.
	if strings.Join(g.Pendentes, ",") != "p3" {
		t.Fatalf("pendentes = %v", g.Pendentes)
	}
	achou := false
	for _, r := range lista(g.Conteudo["respostas"]) {
		ro := ObjetoDe(r)
		if texto(ro["perguntaId"]) == "p2" && texto(ro["resposta"]) == "3" && texto(ro["origem"]) == "respostasPadrao" {
			achou = true
		}
		if texto(ro["perguntaId"]) == "p3" {
			t.Fatal("chutou resposta para pergunta sem par")
		}
	}
	if !achou {
		t.Fatalf("respostas = %v", g.Conteudo["respostas"])
	}
	if g.Impressao == impressaoAntes {
		t.Fatal("a estrutura não entrou na impressão")
	}
	// Aprovar com pendência: conflito; no lote, faltam-respostas.
	var ec *ErroConflito
	if _, _, err := AprovarCandidatura(novo, g.ID, g.Impressao, agora, nil, eu); !errors.As(err, &ec) {
		t.Fatalf("aprovar com pendência = %v", err)
	}
	if _, _, r, _ := AprovarLote(novo, []ItemLote{{g.ID, g.Impressao}}, agora, nil, eu); len(r) != 1 || r[0].Motivo != RecusaFaltamRespostas {
		t.Fatalf("lote com pendência = %v", r)
	}
	// A pessoa responde p3 na Fila: some das pendentes, a impressão muda, e aprovar passa.
	ct := clonarObj(g.Conteudo)
	ct["respostas"] = append(lista(ct["respostas"]), Objeto{"pergunta": "Por que você quer trabalhar aqui?", "resposta": "Gosto do produto.", "origem": "pessoa", "perguntaId": "p3"})
	novo, o, err = EditarConteudo(novo, g.ID, ct, g.Versao, agora, eu)
	if err != nil {
		t.Fatal(err)
	}
	g2, _ := candidaturaDe(o)
	if len(g2.Pendentes) != 0 || g2.Impressao == g.Impressao || g2.Estrutura == nil {
		t.Fatalf("depois de responder: pendentes %v", g2.Pendentes)
	}
	if _, _, err := AprovarCandidatura(novo, g2.ID, g2.Impressao, agora, nil, eu); err != nil {
		t.Fatalf("aprovar sem pendência = %v", err)
	}
}

func TestEstruturaFormularioRecusas(t *testing.T) {
	e, c, agora := formularioPreparado(t)
	var ec *ErroConflito
	// Sem estrutura lida: aprovar dá conflito; no lote, faltam-respostas.
	if _, _, err := AprovarCandidatura(e, c.ID, c.Impressao, agora, nil, eu); !errors.As(err, &ec) {
		t.Fatalf("aprovar sem estrutura = %v", err)
	}
	if _, _, r, _ := AprovarLote(e, []ItemLote{{c.ID, c.Impressao}}, agora, nil, eu); len(r) != 1 || r[0].Motivo != RecusaFaltamRespostas {
		t.Fatalf("lote sem estrutura = %v", r)
	}
	ruins := map[string]struct {
		perguntas []PerguntaFormulario
		url       string
	}{
		"nenhuma pergunta":   {nil, urlTypeform},
		"id repetido":        {[]PerguntaFormulario{{ID: "a", Texto: "x", Tipo: "texto-curto"}, {ID: "a", Texto: "y", Tipo: "texto-curto"}}, urlTypeform},
		"id fora do formato": {[]PerguntaFormulario{{ID: "a b", Texto: "x", Tipo: "texto-curto"}}, urlTypeform},
		"tipo fora da lista": {[]PerguntaFormulario{{ID: "a", Texto: "x", Tipo: "script"}}, urlTypeform},
		"texto vazio":        {[]PerguntaFormulario{{ID: "a", Texto: "", Tipo: "texto-curto"}}, urlTypeform},
		"texto grande":       {[]PerguntaFormulario{{ID: "a", Texto: strings.Repeat("x", 501), Tipo: "texto-curto"}}, urlTypeform},
		"opção grande":       {[]PerguntaFormulario{{ID: "a", Texto: "x", Tipo: "lista", Opcoes: []string{strings.Repeat("o", 201)}}}, urlTypeform},
		"destino de outro":   {perguntasFicticias(), "https://outra.typeform.com/to/Zz9"},
		"destino fora":       {perguntasFicticias(), "https://exemplo.com/form"},
		"destino forms.gle":  {perguntasFicticias(), "https://forms.gle/AbC"},
	}
	for nome, r := range ruins {
		if _, _, err := GravarEstruturaFormulario(e, c.ID, r.perguntas, r.url, agora); err == nil {
			t.Errorf("%s: aceitou", nome)
		}
	}
	// Só em preparada.
	novo, o, err := GravarEstruturaFormulario(e, c.ID, []PerguntaFormulario{{ID: "a", Texto: "Opcional?", Tipo: "texto-curto"}}, urlTypeform, agora)
	if err != nil {
		t.Fatal(err)
	}
	g, _ := candidaturaDe(o)
	if novo, _, err = AprovarCandidatura(novo, g.ID, g.Impressao, agora, nil, eu); err != nil {
		t.Fatal(err)
	}
	if _, _, err := GravarEstruturaFormulario(novo, c.ID, perguntasFicticias(), urlTypeform, agora); !errors.As(err, &ec) {
		t.Fatalf("em aprovada = %v", err)
	}
	// Outra plataforma não tem estrutura.
	e2 := comVaga(t, EstadoVazio(), "li-1", "linkedin", linkLI("1"), true)
	e2, c2 := preparada(t, e2, "li-1", agora)
	if _, _, err := GravarEstruturaFormulario(e2, c2.ID, perguntasFicticias(), urlTypeform, agora); !errors.As(err, &ec) {
		t.Fatalf("linkedin = %v", err)
	}
}

// forms.gle: o destino canônico (resolvido no navegador) tem de ser um formulário da lista, e não outro forms.gle.
func TestEstruturaDoFormsGle(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	e := comVaga(t, EstadoVazio(), "fm-2", "Site da empresa", "https://forms.gle/AbC123xyz", true)
	e, c := preparada(t, e, "fm-2", agora)
	_, o, err := GravarEstruturaFormulario(e, c.ID, []PerguntaFormulario{{ID: "a", Texto: "Opcional?", Tipo: "texto-curto"}}, "https://docs.google.com/forms/d/e/1FAIpQLSf_x/viewform?usp=sf_link", agora)
	if err != nil {
		t.Fatal(err)
	}
	g, _ := candidaturaDe(o)
	if g.Estrutura.URLFinal != "https://docs.google.com/forms/d/e/1FAIpQLSf_x/viewform" {
		t.Fatalf("urlFinal = %s", g.Estrutura.URLFinal)
	}
}

// O pacote aceita perguntaId (só o formato fechado).
func TestConteudoComPerguntaId(t *testing.T) {
	ct := conteudoOK()
	ct["respostas"] = []any{Objeto{"pergunta": "x", "resposta": "y", "origem": "pessoa", "perguntaId": "p_1-a"}}
	if erros := ValidarConteudo(ct); len(erros) != 0 {
		t.Fatalf("perguntaId válido: %v", erros)
	}
	ct["respostas"] = []any{Objeto{"pergunta": "x", "resposta": "y", "origem": "pessoa", "perguntaId": "../x"}}
	if erros := ValidarConteudo(ct); len(erros) == 0 {
		t.Fatal("perguntaId fora do formato passou")
	}
}

// Buracos achados pelas mutações: pergunta parecida não casa; a estrutura sozinha muda a impressão; o destino do
// forms.gle nunca é outro forms.gle.
func TestFormularioSemChuteEImpressaoDaEstrutura(t *testing.T) {
	e, c, agora := formularioPreparado(t)
	// Resposta padrão de uma pergunta parecida, mas diferente: fica pendente.
	parecida := []PerguntaFormulario{{ID: "p9", Texto: "Quantos anos de Go você tem de experiência profissional?", Tipo: "texto-curto", Obrigatoria: true}}
	_, o, err := GravarEstruturaFormulario(e, c.ID, parecida, urlTypeform, agora)
	if err != nil {
		t.Fatal(err)
	}
	if g, _ := candidaturaDe(o); strings.Join(g.Pendentes, ",") != "p9" {
		t.Fatalf("pergunta parecida casou: pendentes %v", g.Pendentes)
	}
	// Duas estruturas que só diferem no texto de uma pergunta opcional, com as mesmas respostas: impressões diferentes.
	a := []PerguntaFormulario{{ID: "o1", Texto: "Comentário (opcional)", Tipo: "texto-longo"}}
	b := []PerguntaFormulario{{ID: "o1", Texto: "Observações (opcional)", Tipo: "texto-longo"}}
	_, oa, err := GravarEstruturaFormulario(e, c.ID, a, urlTypeform, agora)
	if err != nil {
		t.Fatal(err)
	}
	_, ob, err := GravarEstruturaFormulario(e, c.ID, b, urlTypeform, agora)
	if err != nil {
		t.Fatal(err)
	}
	ga, _ := candidaturaDe(oa)
	gb, _ := candidaturaDe(ob)
	if ga.Impressao == gb.Impressao || ga.Estrutura.Hash == gb.Estrutura.Hash {
		t.Fatal("a estrutura não entrou na impressão")
	}
	// forms.gle: o destino tem de ser o formulário de verdade, nunca outro forms.gle.
	e2 := comVaga(t, EstadoVazio(), "fm-3", "Site da empresa", "https://forms.gle/AbC123xyz", true)
	e2, c2 := preparada(t, e2, "fm-3", agora)
	if _, _, err := GravarEstruturaFormulario(e2, c2.ID, a, "https://forms.gle/AbC123xyz", agora); err == nil {
		t.Fatal("aceitou forms.gle como destino")
	}
}

// Achado médio do CyberSec no #120: pergunta sensível (CPF, RG, senha, banco, documento, dado sensível, termos) nunca
// é preenchida, nem por uma respostasPadrao que a pessoa (ou uma IA pelo /perfil) tenha gravado. A estrutura com uma
// delas é recusada com o motivo do detector, para a leitura bloquear a candidatura (parte 2). Dados fictícios.
func TestEstruturaComPerguntaSensivel(t *testing.T) {
	e, c, agora := formularioPreparado(t)
	e.Perfil = Objeto{"respostasPadrao": []any{
		Objeto{"pergunta": "Qual o seu CPF?", "resposta": "000.000.000-00"},
		Objeto{"pergunta": "Senha de acesso", "resposta": "senha-ficticia"},
	}}
	for texto, motivo := range map[string]string{
		"Qual o seu CPF?":              "cpf",
		"Senha de acesso":              "senha",
		"Agência e conta bancária":     "dado-bancario",
		"Data de nascimento":           "dado-sensivel",
		"Li e aceito os termos de uso": "termos",
	} {
		perguntas := []PerguntaFormulario{{ID: "ok", Texto: "Opcional?", Tipo: "texto-curto"}, {ID: "s", Texto: texto, Tipo: "texto-curto", Obrigatoria: true}}
		novo, _, err := GravarEstruturaFormulario(e, c.ID, perguntas, urlTypeform, agora)
		var eb *ErroBloqueioLeitura
		if !errors.As(err, &eb) || eb.Motivo != motivo {
			t.Errorf("%q: erro %v; queria bloqueio %s", texto, err, motivo)
			continue
		}
		if emJSONTeste(novo) != emJSONTeste(e) || strings.Contains(emJSONTeste(Estado{Candidaturas: novo.Candidaturas}), "000.000.000-00") {
			t.Errorf("%q: gravou ou preencheu", texto)
		}
	}
	// Defesa a mais: o casamento exato nunca devolve resposta para pergunta sensível.
	rs := respostasDoFormulario(e.Perfil, Objeto{}, []PerguntaFormulario{{ID: "s", Texto: "Qual o seu CPF?", Tipo: "texto-curto"}})
	if len(rs) != 0 {
		t.Fatalf("respostas = %v", rs)
	}
}
