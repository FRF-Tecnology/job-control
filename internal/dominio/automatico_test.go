package dominio

import (
	"testing"
	"time"
)

// Dados fictícios. No modo automático, o app aprova sozinho só os pacotes das vagas nota A da Descoberta, dentro do
// limite do dia; a vaga B, escolhida pela pessoa, espera a aprovação dela. No modo Escolher, nada é aprovado.
func TestAprovarAutomaticas(t *testing.T) {
	agora := time.Date(2026, 11, 17, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	nota := func(e Estado, id, aderencia string) {
		for _, v := range e.Vagas {
			if v["id"] == id {
				v["aderencia"] = aderencia
			}
		}
	}
	montar := func(modo string) (Estado, []Candidatura) {
		e := Estado{Perfil: Objeto{"modo": "direto"}}
		e = comVaga(t, e, "li-1", "linkedin", linkLI("1"), false)
		e = comVaga(t, e, "li-2", "linkedin", linkLI("2"), false)
		e = comVaga(t, e, "li-3", "linkedin", linkLI("3"), true) // a pessoa escolheu na Triagem
		nota(e, "li-1", "A")
		nota(e, "li-2", "A")
		nota(e, "li-3", "B")
		var cs []Candidatura
		for _, id := range []string{"li-1", "li-2", "li-3"} {
			var c Candidatura
			e, c = preparada(t, e, id, agora)
			cs = append(cs, c)
		}
		e.Perfil["modo"] = modo
		return e, cs
	}

	e, cs := montar("direto")
	novo, ids := AprovarAutomaticas(e, agora, map[string]int{"linkedin": 1})
	if len(ids) != 1 || ids[0] != cs[0].ID {
		t.Fatalf("aprovadas = %v; queria só %s (limite 1)", ids, cs[0].ID)
	}
	c := estadoDe(t, novo, cs[0].ID)
	if c.Estado != CandAprovada || c.Aprovacao == nil || !c.Aprovacao.Automatica || c.Aprovacao.Impressao != c.Impressao {
		t.Fatalf("candidatura aprovada sozinha = %+v", c)
	}
	if s := estadoDe(t, novo, cs[1].ID).Estado; s != CandPreparada {
		t.Fatalf("passou do limite: %s", s)
	}
	if s := estadoDe(t, novo, cs[2].ID).Estado; s != CandPreparada {
		t.Fatalf("vaga B aprovada sozinha: %s", s)
	}
	if _, ids := AprovarAutomaticas(e, agora, map[string]int{"linkedin": 5}); len(ids) != 2 {
		t.Fatalf("sem limite apertado = %v; queria as duas nota A", ids)
	}

	e, _ = montar("revisar")
	if _, ids := AprovarAutomaticas(e, agora, map[string]int{"linkedin": 5}); len(ids) != 0 {
		t.Fatalf("modo Escolher aprovou %v", ids)
	}
}
