package dominio

import (
	"errors"
	"testing"
	"time"
)

// jc-a2y.7.2, parte 2: cada leitura do formulário abre uma página no navegador do envio e conta no teto de 40 páginas
// abertas por plataforma no dia (não no limite do dia); um bloqueio achado na leitura leva a candidatura de preparada
// para bloqueada, com a pendência fixa. Dados fictícios.

func TestRegistrarLeituraContaNoTeto(t *testing.T) {
	e, c, agora := formularioPreparado(t)
	novo, _, err := RegistrarLeitura(e, c.ID, agora)
	if err != nil {
		t.Fatal(err)
	}
	if n := AberturasNoDia(novo, PlataformaFormulario, agora); n != 1 {
		t.Fatalf("aberturas = %d", n)
	}
	if n := UsadasNoDia(novo, PlataformaFormulario, agora); n != 0 {
		t.Fatalf("a leitura contou no limite do dia: %d", n)
	}
	// No teto: a 41ª leitura do dia é recusada (limite), e nada muda.
	for i := 1; i < LimiteMaximo; i++ {
		if novo, _, err = RegistrarLeitura(novo, c.ID, agora); err != nil {
			t.Fatalf("leitura %d: %v", i+1, err)
		}
	}
	var el *ErroLimite
	if _, _, err := RegistrarLeitura(novo, c.ID, agora); !errors.As(err, &el) {
		t.Fatalf("41ª leitura = %v", err)
	}
	// Amanhã, renova.
	if _, _, err := RegistrarLeitura(novo, c.ID, agora.Add(24*time.Hour)); err != nil {
		t.Fatalf("no dia seguinte = %v", err)
	}
}

func TestRegistrarLeituraSoEmPreparadaDeFormulario(t *testing.T) {
	e, c, agora := formularioPreparado(t)
	var ec *ErroConflito
	e2 := comVaga(t, EstadoVazio(), "li-1", "linkedin", linkLI("1"), true)
	e2, c2 := preparada(t, e2, "li-1", agora)
	if _, _, err := RegistrarLeitura(e2, c2.ID, agora); !errors.As(err, &ec) {
		t.Fatalf("linkedin = %v", err)
	}
	novo, o, err := GravarEstruturaFormulario(e, c.ID, []PerguntaFormulario{{ID: "a", Texto: "Opcional?", Tipo: "texto-curto"}}, urlTypeform, agora)
	if err != nil {
		t.Fatal(err)
	}
	g, _ := candidaturaDe(o)
	if novo, _, err = AprovarCandidatura(novo, g.ID, g.Impressao, agora, nil, eu); err != nil {
		t.Fatal(err)
	}
	if _, _, err := RegistrarLeitura(novo, c.ID, agora); !errors.As(err, &ec) {
		t.Fatalf("aprovada = %v", err)
	}
}

func TestBloquearLeitura(t *testing.T) {
	e, c, agora := formularioPreparado(t)
	novo, o, err := BloquearLeitura(e, c.ID, "varias-etapas", agora, eu)
	if err != nil {
		t.Fatal(err)
	}
	g, _ := candidaturaDe(o)
	if g.Estado != CandBloqueada || g.Bloqueio == nil || g.Bloqueio.Motivo != "varias-etapas" || g.Bloqueio.Pendencia != "Formulário em várias etapas: envie à mão" {
		t.Fatalf("bloqueada = %+v", g)
	}
	if v := vagaDe(novo, c.VagaID); texto(v["etapa"]) != "bloqueada" {
		t.Fatalf("vaga = %v", v["etapa"])
	}
	// Motivo fora da lista e candidatura fora de preparada: recusados.
	if _, _, err := BloquearLeitura(e, c.ID, "texto da página", agora, eu); err == nil {
		t.Fatal("motivo fora da lista passou")
	}
	var ec *ErroConflito
	if _, _, err := BloquearLeitura(novo, c.ID, "login", agora, eu); !errors.As(err, &ec) {
		t.Fatalf("já bloqueada = %v", err)
	}
}
