package dominio

import (
	"testing"
	"time"
)

// jc-a2y.7.4 (PR 2): as regras do assistente no domínio. Dados fictícios.

func TestCanalNuncaEhAPessoa(t *testing.T) {
	for _, c := range []Ctx{{Autor: "eu", Canal: CanalMCP}, {Autor: "", Canal: CanalCLI}, {Autor: "claude", Canal: CanalMCP}, {Autor: "claude"}} {
		if c.EhPessoa() {
			t.Errorf("%+v é a pessoa", c)
		}
	}
	if !(Ctx{Autor: "eu"}).EhPessoa() || !(Ctx{}).EhPessoa() {
		t.Fatal("a tela deixou de ser a pessoa")
	}
	for _, c := range []struct {
		ctx  Ctx
		quer *Quem
	}{
		{Ctx{Autor: "eu"}, &Quem{"eu", CanalTela}},
		{Ctx{Autor: "eu", Canal: CanalMCP}, &Quem{"claude", CanalMCP}},
		{Ctx{Autor: "claude", Canal: CanalCLI}, &Quem{"claude", CanalCLI}},
		{Ctx{Autor: "claude"}, nil},
		{Ctx{Autor: "claude", Canal: "tela"}, nil},
	} {
		q := QuemDe(c.ctx)
		if (q == nil) != (c.quer == nil) || (q != nil && *q != *c.quer) {
			t.Errorf("QuemDe(%+v) = %+v; queria %+v", c.ctx, q, c.quer)
		}
	}
	for _, q := range []Quem{{"eu", "mcp"}, {"eu", "cli"}, {"claude", "tela"}, {"claude", ""}, {"", "mcp"}, {"executor", "mcp"}} {
		if QuemValido(&q) {
			t.Errorf("%+v válido", q)
		}
	}
	if QuemValido(nil) {
		t.Fatal("nil válido")
	}
}

func TestEventoComCanal(t *testing.T) {
	e := comVaga(t, EstadoVazio(), "v-1", "linkedin", "https://www.linkedin.com/jobs/view/9000000001", false)
	novo, _, err := RegistrarEvento(e, "v-1", "nota", "Nota fictícia", Ctx{Autor: "claude", Agora: "2026-11-09", Canal: CanalMCP})
	if err != nil {
		t.Fatal(err)
	}
	ev := novo.Eventos[len(novo.Eventos)-1]
	if ev["autor"] != "claude" || ev["canal"] != "mcp" {
		t.Fatalf("evento = %v", ev)
	}
	novo, _, _ = RegistrarEvento(e, "v-1", "nota", "x", Ctx{Autor: "eu", Agora: "2026-11-09"})
	if _, tem := novo.Eventos[len(novo.Eventos)-1]["canal"]; tem {
		t.Fatal("evento da tela com canal")
	}
}

func TestMoverPeloAssistente(t *testing.T) {
	ctx := Ctx{Autor: "claude", Agora: "2026-11-09", Canal: CanalMCP}
	e := comVaga(t, EstadoVazio(), "v-1", "linkedin", "https://www.linkedin.com/jobs/view/9000000001", false)
	for nome, c := range map[string]struct {
		etapa  string
		extras Objeto
	}{
		"enviada":               {"enviada", nil},
		"envio junto":           {"bloqueada", Objeto{"pendencia": "Login", "envio": Objeto{"data": "2026-11-09"}}},
		"em-processo sem envio": {"em-processo", nil},
		"oferta sem envio":      {"oferta", nil},
	} {
		if _, _, err := MoverVaga(e, "v-1", c.etapa, c.extras, ctx); err == nil {
			t.Errorf("%s: aceitou", nome)
		}
	}
	if _, _, err := MoverVaga(e, "v-1", "pulada", Objeto{"motivo": "Outro"}, ctx); err != nil {
		t.Fatalf("pulada pelo assistente = %v", err)
	}
	// Pela tela continua igual (a Node e o golden).
	if _, _, err := MoverVaga(e, "v-1", "em-processo", nil, Ctx{Autor: "eu", Agora: "2026-11-09"}); err != nil {
		t.Fatalf("em-processo pela tela = %v", err)
	}
	// Assistente sem canal (caminho antigo, golden): a regra nova não vale.
	if _, _, err := MoverVaga(e, "v-1", "em-processo", nil, Ctx{Autor: "claude", Agora: "2026-11-09"}); err != nil {
		t.Fatalf("assistente sem canal = %v", err)
	}
}

func TestPausadaPor(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	p, err := Plataformas{}.PausarPlataformaPor("gupy", agora, &Quem{"claude", CanalMCP})
	if err != nil {
		t.Fatal(err)
	}
	q, _ := Plataformas{}.PausarPlataformaPor("inhire", agora, &Quem{"eu", CanalMCP}) // inválido: sem quem
	if s := q.DaPlataforma("inhire", agora); s.PausadaPor != nil || s.Estado != PlataformaPausada {
		t.Fatalf("inhire = %+v", s)
	}
	for _, o := range p.Lista(agora) {
		switch o["plataforma"] {
		case "gupy":
			if q := ObjetoDe(o["pausadaPor"]); q["autor"] != "claude" || q["canal"] != "mcp" {
				t.Fatalf("gupy = %v", o)
			}
		default:
			if _, tem := o["pausadaPor"]; tem {
				t.Fatalf("%v com pausadaPor", o)
			}
		}
	}
	// Falhas seguidas (com motivo) não mostram quem pausou.
	f := p.RegistrarResultado("linkedin", "erro-tecnico", agora).RegistrarResultado("linkedin", "erro-tecnico", agora)
	for _, o := range f.Lista(agora) {
		if o["plataforma"] == "linkedin" && o["pausadaPor"] != nil {
			t.Fatalf("falhas-seguidas com pausadaPor: %v", o)
		}
	}
}
