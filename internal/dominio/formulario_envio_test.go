package dominio

import (
	"strings"
	"testing"
)

// jc-a2y.7.2, parte 3b: o plano do envio. A estrutura relida tem de ser a aprovada, e cada valor sai só do pacote.
// Dados fictícios.

// candidaturaComEstrutura: uma candidatura de formulário com esta estrutura (hash calculado) e estas respostas.
func candidaturaComEstrutura(t *testing.T, perguntas []PerguntaFormulario, respostas []any) Objeto {
	t.Helper()
	e, c, agora := formularioPreparado(t)
	_, o, err := GravarEstruturaFormulario(e, c.ID, perguntasFicticias(), urlTypeform, agora)
	if err != nil {
		t.Fatal(err)
	}
	h, err := hashDasPerguntas(perguntas)
	if err != nil {
		t.Fatal(err)
	}
	o["estrutura"] = EstruturaFormulario{Hash: h, URLFinal: urlTypeform, Perguntas: perguntas}
	conteudo := clonarObj(ObjetoDe(o["conteudo"]))
	conteudo["respostas"] = respostas
	o["conteudo"] = conteudo
	return o
}

func resposta(id, valor string) Objeto {
	return Objeto{"pergunta": "x", "resposta": valor, "origem": "pessoa", "perguntaId": id}
}

func TestPlanoDoFormulario(t *testing.T) {
	perguntas := []PerguntaFormulario{
		{ID: "p1", Texto: "Qual o seu e-mail?", Tipo: "email", Obrigatoria: true},
		{ID: "p2", Texto: "Anos de Go", Tipo: "texto-curto", Obrigatoria: true},
		{ID: "p3", Texto: "Inglês", Tipo: "escolha-unica", Obrigatoria: true, Opcoes: []string{"Básico", "Avançado"}},
		{ID: "p4", Texto: "Disponível a partir de", Tipo: "data"},
		{ID: "p5", Texto: "Comentários", Tipo: "texto-longo"},
	}
	o := candidaturaComEstrutura(t, perguntas, []any{resposta("p2", "3"), resposta("p3", "Avançado"), resposta("p4", "2026-12-01"), resposta("p5", "  ")})
	plano, motivo := PlanoDoFormulario(o, perguntas, urlTypeform)
	if motivo != "" {
		t.Fatalf("motivo = %s", motivo)
	}
	var got []string
	for _, r := range plano {
		got = append(got, r.Pergunta.ID+"="+r.Valor)
	}
	// p1 pelo contato do pacote; p5 só com espaços: opcional em branco.
	quer := "p1=" + texto(ObjetoDe(ObjetoDe(o["conteudo"])["contato"])["email"]) + ",p2=3,p3=Avançado,p4=2026-12-01"
	if strings.Join(got, ",") != quer {
		t.Fatalf("plano = %v; queria %s", got, quer)
	}
}

func TestPlanoDoFormularioBloqueia(t *testing.T) {
	base := []PerguntaFormulario{{ID: "p1", Texto: "Anos de Go", Tipo: "texto-curto", Obrigatoria: true}}
	com := func(p PerguntaFormulario) []PerguntaFormulario {
		return append(append([]PerguntaFormulario(nil), base...), p)
	}
	ok := []any{resposta("p1", "3")}
	for nome, c := range map[string]struct {
		perguntas []PerguntaFormulario
		respostas []any
		quer      string
	}{
		"obrigatória sem resposta":       {base, nil, "pergunta-nova"},
		"opção que não existe":           {com(PerguntaFormulario{ID: "p2", Texto: "Inglês", Tipo: "escolha-unica", Opcoes: []string{"Básico"}}), append(ok, resposta("p2", "básico")), "pergunta-nova"},
		"múltipla fora das opções":       {com(PerguntaFormulario{ID: "p2", Texto: "Stack", Tipo: "multipla-escolha", Opcoes: []string{"Go"}}), append(ok, resposta("p2", "Go, React")), "pergunta-nova"},
		"lista fora das opções":          {com(PerguntaFormulario{ID: "p2", Texto: "Estado", Tipo: "lista", Opcoes: []string{"SP"}}), append(ok, resposta("p2", "RJ")), "pergunta-nova"},
		"escala fora das opções":         {com(PerguntaFormulario{ID: "p2", Texto: "Nota", Tipo: "escala", Opcoes: []string{"1", "5"}}), append(ok, resposta("p2", "3")), "pergunta-nova"},
		"data fora do formato":           {com(PerguntaFormulario{ID: "p2", Texto: "Início", Tipo: "data"}), append(ok, resposta("p2", "01/12/2026")), "pergunta-nova"},
		"tipo que o app não preenche":    {com(PerguntaFormulario{ID: "p2", Texto: "Hora", Tipo: "outro"}), append(ok, resposta("p2", "10h")), "outro"},
		"pergunta sensível":              {com(PerguntaFormulario{ID: "p2", Texto: "Qual o seu CPF?", Tipo: "texto-curto"}), ok, "cpf"},
		"aceite só na opção (Fallen)":    {com(PerguntaFormulario{ID: "p2", Texto: "Confirmação", Tipo: "multipla-escolha", Opcoes: []string{"Li e aceito os termos de uso"}}), ok, "termos"},
		"obrigatória do contato sem ele": {com(PerguntaFormulario{ID: "p2", Texto: "Telefone", Tipo: "telefone", Obrigatoria: true}), ok, "pergunta-nova"}, // o contato fictício não tem telefone
	} {
		o := candidaturaComEstrutura(t, c.perguntas, c.respostas)
		_, motivo := PlanoDoFormulario(o, c.perguntas, urlTypeform)
		if motivo != c.quer {
			t.Errorf("%s: motivo %q; queria %q", nome, motivo, c.quer)
		}
	}
	// A estrutura relida diferente da aprovada, outro destino, perguntas fora dos limites ou outra plataforma.
	o := candidaturaComEstrutura(t, base, ok)
	mudou := []PerguntaFormulario{{ID: "p1", Texto: "Anos de Go e Rust", Tipo: "texto-curto", Obrigatoria: true}}
	opcional := []PerguntaFormulario{{ID: "p1", Texto: "Anos de Go", Tipo: "texto-curto"}}
	for nome, c := range map[string]struct {
		lidas []PerguntaFormulario
		url   string
		quer  string
	}{
		"texto mudou":         {mudou, urlTypeform, "pergunta-nova"},
		"obrigatória mudou":   {opcional, urlTypeform, "pergunta-nova"},
		"pergunta a mais":     {com(PerguntaFormulario{ID: "p9", Texto: "Nova", Tipo: "texto-curto"}), urlTypeform, "pergunta-nova"},
		"outro destino":       {base, "https://acme.typeform.com/to/Outro1", "site-externo"},
		"leitura sem nada":    {nil, urlTypeform, "outro"},
		"id fora do formato":  {[]PerguntaFormulario{{ID: "p 1", Texto: "Anos de Go", Tipo: "texto-curto", Obrigatoria: true}}, urlTypeform, "outro"},
		"mesma, mas sem hash": {base, urlTypeform, ""},
	} {
		_, motivo := PlanoDoFormulario(o, c.lidas, c.url)
		if motivo != c.quer {
			t.Errorf("%s: motivo %q; queria %q", nome, motivo, c.quer)
		}
	}
	semEstrutura := clonarObj(o)
	delete(semEstrutura, "estrutura")
	outra := clonarObj(o)
	outra["plataforma"] = "gupy"
	for nome, x := range map[string]Objeto{"sem estrutura": semEstrutura, "outra plataforma": outra} {
		if _, motivo := PlanoDoFormulario(x, base, urlTypeform); motivo != "outro" {
			t.Errorf("%s: motivo %q", nome, motivo)
		}
	}
}

// O hash do envio é o mesmo da leitura (GravarEstruturaFormulario).
func TestHashDoEnvioIgualAoDaLeitura(t *testing.T) {
	e, c, agora := formularioPreparado(t)
	_, o, err := GravarEstruturaFormulario(e, c.ID, perguntasFicticias(), urlTypeform, agora)
	if err != nil {
		t.Fatal(err)
	}
	g, _ := candidaturaDe(o)
	if h, _ := hashDasPerguntas(perguntasFicticias()); h != g.Estrutura.Hash {
		t.Fatalf("hash = %s; leitura %s", h, g.Estrutura.Hash)
	}
}
