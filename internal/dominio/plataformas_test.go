package dominio

import (
	"testing"
	"time"
)

// jc-a2y.6.11: transições do estado por plataforma. Puras: o original não muda.
func TestEstadoDasPlataformas(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	vazio := Plataformas{}

	// Duas falhas iguais seguidas pausam; resultados diferentes no meio zeram a contagem.
	p := vazio.RegistrarResultado("linkedin", "erro-tecnico", agora)
	if len(vazio) != 0 {
		t.Fatal("mudou o original")
	}
	p = p.RegistrarResultado("linkedin", "enviada", agora).RegistrarResultado("linkedin", "erro-tecnico", agora)
	if !p.Ativa("linkedin", agora) {
		t.Fatal("pausou sem falhas seguidas")
	}
	p = p.RegistrarResultado("linkedin", "site-externo", agora).RegistrarResultado("linkedin", "site-externo", agora)
	if s := p.DaPlataforma("linkedin", agora); s.Estado != PlataformaPausada || s.Motivo != "falhas-seguidas" {
		t.Fatalf("2 site-externo seguidos = %+v", s)
	}
	if !p.Ativa("gupy", agora) {
		t.Fatal("pausou outra plataforma")
	}
	// Resultado bom não tira da pausa: só a pessoa retoma.
	if p.RegistrarResultado("linkedin", "enviada", agora).Ativa("linkedin", agora) {
		t.Fatal("saiu da pausa sozinha")
	}
	r, err := p.RetomarPlataforma("linkedin", agora)
	if err != nil || !r.Ativa("linkedin", agora) {
		t.Fatalf("retomar = %v", err)
	}
	if _, err := r.RetomarPlataforma("linkedin", agora); err == nil {
		t.Error("retomou plataforma ativa")
	}

	// Login: precisa-entrar; volta só depois do login.
	l := vazio.RegistrarResultado("gupy", "login", agora)
	if s := l.DaPlataforma("gupy", agora); s.Estado != PlataformaPrecisaEntrar || s.Motivo != "login" {
		t.Fatalf("login = %+v", s)
	}
	l2, liberadas := l.LiberarDepoisDoLogin(agora)
	if !l2.Ativa("gupy", agora) || len(liberadas) != 1 || liberadas[0] != "gupy" || l.Ativa("gupy", agora) {
		t.Fatalf("depois do login = %v, original ativa %v", liberadas, l.Ativa("gupy", agora))
	}

	// Captcha e aviso de conta: parada até 00:00 do dia seguinte; retomar e pausar não mudam isso.
	for _, motivo := range []string{"captcha", "aviso-conta"} {
		c := vazio.RegistrarResultado("inhire", motivo, agora)
		s := c.DaPlataforma("inhire", agora)
		if s.Estado != PlataformaParadaHoje || s.Motivo != motivo || s.Ate != "2026-11-10T00:00:00-03:00" {
			t.Fatalf("%s = %+v", motivo, s)
		}
		if _, err := c.RetomarPlataforma("inhire", agora); err == nil {
			t.Fatalf("%s: retomou no mesmo dia", motivo)
		}
		if pp, _ := c.PausarPlataforma("inhire", agora); pp.DaPlataforma("inhire", agora).Estado != PlataformaParadaHoje {
			t.Fatalf("%s: pausar trocou a parada-hoje", motivo)
		}
		if c2, _ := c.LiberarDepoisDoLogin(agora); c2.Ativa("inhire", agora) {
			t.Fatalf("%s: o login liberou a parada-hoje", motivo)
		}
		if !c.Ativa("inhire", time.Date(2026, 11, 10, 0, 0, 0, 0, agora.Location())) {
			t.Fatalf("%s: não renovou à meia-noite", motivo)
		}
	}

	// Plataforma sem envio automático: 404.
	if _, err := vazio.PausarPlataforma("outra", agora); err == nil {
		t.Error("pausou 'outra'")
	}
	// A lista para a tela: as três, com motivo e até só quando há.
	lista := vazio.RegistrarResultado("inhire", "captcha", agora).Lista(agora)
	if len(lista) != 3 || lista[0]["plataforma"] != "gupy" || lista[0]["motivo"] != nil || lista[1]["ate"] != "2026-11-10T00:00:00-03:00" {
		t.Fatalf("lista = %v", lista)
	}
	for _, o := range lista {
		if _, tem := o["seguidas"]; tem {
			t.Error("lista com contagem interna")
		}
	}
}

// jc-a2y.7.2 (pedido do Faker no #127): plataforma sem sessão (formulario) nunca ganha estado: sem rota de retomar,
// uma pausa ou um precisa-entrar a travariam de vez. O limite do dia e o teto de páginas continuam valendo.
func TestPlataformaSemSessaoNaoGanhaEstado(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	if TemSessao(PlataformaFormulario) || TemSessao("outra") || !TemSessao("linkedin") || !TemSessao("gupy") || !TemSessao("inhire") {
		t.Fatal("TemSessao fora da lista do envio")
	}
	p := Plataformas{}
	for _, codigo := range []string{"erro-tecnico", "erro-tecnico", "erro-tecnico", "site-externo", "site-externo", "login", "captcha", "aviso-conta"} {
		p = p.RegistrarResultado(PlataformaFormulario, codigo, agora)
	}
	if _, tem := p[PlataformaFormulario]; tem || len(p) != 0 {
		t.Fatalf("formulario ganhou estado: %+v", p)
	}
	if !p.Ativa(PlataformaFormulario, agora) {
		t.Fatal("formulario fora de ativa")
	}
}
