package dominio

import (
	"errors"
	"testing"
	"time"
)

// jc-a2y.6.11 (requisitos do CyberSec): a volta automática para aprovada só sai de enviando, sem marca gravada, com a
// mesma versão e impressão; devolve a aprovação sem uso, tira a tentativa do limite, marca a próxima; na 4ª desde a
// aprovação, falhou. Dados fictícios.

// enviando: vaga aprovada pela pessoa, candidatura aprovada e já em enviando (como o executor a deixa).
func enviando(t *testing.T, agora time.Time) (Estado, Candidatura) {
	t.Helper()
	e := comVaga(t, EstadoVazio(), "li-1", "linkedin", linkLI("1"), true)
	e, c := aprovada(t, e, "li-1", agora, nil)
	e, o, err := IniciarEnvio(e, c.ID, c.Versao, agora, nil)
	if err != nil {
		t.Fatal(err)
	}
	c, _ = candidaturaDe(o)
	return e, c
}

func TestDevolverAprovada(t *testing.T) {
	sp := fuso(t, "America/Sao_Paulo")
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, sp)
	e, c := enviando(t, agora)

	novo, o, err := DevolverAprovada(e, c.ID, c.Versao, "erro-tecnico", agora)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := candidaturaDe(o)
	if d.Estado != CandAprovada || d.Aprovacao.UsadaEm != "" || len(d.Tentativas) != 0 || len(d.Retentativas) != 1 || d.Versao != c.Versao+1 {
		t.Fatalf("devolvida = %+v", d)
	}
	if d.Aprovacao.Impressao != c.Aprovacao.Impressao || d.Aprovacao.ValidaAte != c.Aprovacao.ValidaAte || d.Aprovacao.Em != c.Aprovacao.Em {
		t.Errorf("a volta mudou a aprovação: %+v -> %+v", c.Aprovacao, d.Aprovacao)
	}
	if d.ProximaTentativaEm != agora.Add(5*time.Minute).Format(time.RFC3339) || ProntaParaTentar(d, agora) || !ProntaParaTentar(d, agora.Add(5*time.Minute)) {
		t.Fatalf("próxima tentativa = %q", d.ProximaTentativaEm)
	}
	if usadas, abertas := UsadasNoDia(novo, "linkedin", agora), AberturasNoDia(novo, "linkedin", agora); usadas != 0 || abertas != 1 {
		t.Errorf("usadas = %d, abertas = %d; queria 0 e 1", usadas, abertas)
	}
	// Antes da hora marcada, o executor não pega de novo (vale também com o app reaberto: está gravado).
	if _, _, err := IniciarEnvio(novo, d.ID, d.Versao, agora.Add(4*time.Minute), nil); err == nil {
		t.Fatal("iniciou antes da próxima tentativa")
	}

	// 2ª e 3ª voltas: 30 min e 2 h. Na 4ª, falhou/erro-tecnico.
	e, quando := novo, agora.Add(5*time.Minute)
	for n, espera := range []time.Duration{30 * time.Minute, 2 * time.Hour} {
		e, o, err = IniciarEnvio(e, d.ID, d.Versao, quando, nil)
		if err != nil {
			t.Fatalf("volta %d: %v", n+2, err)
		}
		d, _ = candidaturaDe(o)
		e, o, err = DevolverAprovada(e, d.ID, d.Versao, "login", quando)
		if err != nil {
			t.Fatal(err)
		}
		d, _ = candidaturaDe(o)
		if d.ProximaTentativaEm != quando.Add(espera).Format(time.RFC3339) || d.Retentativas[n+1].Codigo != "login" {
			t.Errorf("volta %d: %+v", n+2, d)
		}
		quando = quando.Add(espera)
	}
	e, o, _ = IniciarEnvio(e, d.ID, d.Versao, quando, nil)
	d, _ = candidaturaDe(o)
	if _, o, err := DevolverAprovada(e, d.ID, d.Versao, "erro-tecnico", quando); err != nil || o["estado"] != CandFalhou {
		t.Fatalf("depois de 3 voltas = %v, %v", o["estado"], err)
	}
}

// Item 1: com a marca gravada, nunca volta (mesmo com versão e impressão certas).
func TestDevolverRecusaComMarca(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	for _, marca := range []string{MarcaInteragiu, MarcaClicouEnviar} {
		e, c := enviando(t, agora)
		e, o, err := MarcarTentativa(e, c.ID, marca, agora)
		if err != nil {
			t.Fatal(err)
		}
		m, _ := candidaturaDe(o)
		if m.InteragiuEm == "" || (marca == MarcaClicouEnviar) != (m.ClicouEnviarEm != "") {
			t.Fatalf("%s: marcas = %q, %q", marca, m.InteragiuEm, m.ClicouEnviarEm)
		}
		if _, _, err := DevolverAprovada(e, m.ID, m.Versao, "erro-tecnico", agora); err == nil {
			t.Fatalf("%s: voltou com a marca gravada", marca)
		} else {
			conflito(t, err)
		}
	}
}

func TestMarcarTentativa(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	e, c := enviando(t, agora)
	e, _, _ = MarcarTentativa(e, c.ID, MarcaInteragiu, agora)
	e, o, _ := MarcarTentativa(e, c.ID, MarcaInteragiu, agora.Add(time.Minute))
	m, _ := candidaturaDe(o)
	if m.InteragiuEm != agora.Format(time.RFC3339) {
		t.Errorf("repetir mudou o instante: %q", m.InteragiuEm)
	}
	if _, _, err := MarcarTentativa(e, c.ID, "outra", agora); err == nil {
		t.Error("aceitou marca desconhecida")
	}
	// Só em enviando.
	e2 := comVaga(t, EstadoVazio(), "li-1", "linkedin", linkLI("1"), true)
	e2, a := aprovada(t, e2, "li-1", agora, nil)
	if _, _, err := MarcarTentativa(e2, a.ID, MarcaInteragiu, agora); err == nil {
		t.Error("marcou fora de enviando")
	}
	// Tentativa nova começa sem marcas.
	_, o, _ = FalharCandidatura(e, c.ID, "erro-tecnico", agora)
	f, _ := candidaturaDe(o)
	if f.InteragiuEm == "" {
		t.Fatal("falhar apagou a marca (histórico da tentativa)")
	}
}

func TestDevolverConfereVersaoImpressaoEEstado(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	e, c := enviando(t, agora)
	if _, _, err := DevolverAprovada(e, c.ID, c.Versao-1, "erro-tecnico", agora); err == nil {
		t.Error("voltou com versão velha")
	}
	// Pacote trocado direto no banco: a impressão recalculada não bate.
	mexido := e.Clonar()
	ObjetoDe(mexido.Candidaturas[0]["conteudo"])["idioma"] = "en"
	if _, _, err := DevolverAprovada(mexido, c.ID, c.Versao, "erro-tecnico", agora); err == nil {
		t.Error("voltou com o pacote trocado")
	} else if conflito(t, err).ImpressaoAtual == "" {
		t.Error("sem a impressão atual no conflito")
	}
	if _, _, err := DevolverAprovada(e, c.ID, c.Versao, "site-externo", agora); err == nil {
		t.Error("voltou com código fora da lista")
	}
	for _, fim := range []func(Estado) (Estado, Objeto, error){
		func(e Estado) (Estado, Objeto, error) { return FalharCandidatura(e, c.ID, "erro-tecnico", agora) },
		func(e Estado) (Estado, Objeto, error) { return TornarIncerta(e, c.ID, agora) },
	} {
		e2, o, err := fim(e)
		if err != nil {
			t.Fatal(err)
		}
		v, _ := candidaturaDe(o)
		if _, _, err := DevolverAprovada(e2, c.ID, v.Versao, "erro-tecnico", agora); err == nil {
			t.Errorf("voltou de %s (incerta e falhou nunca voltam sozinhas)", v.Estado)
		}
	}
}

// Passou da meia-noite: a aprovação era do dia anterior; a candidatura volta a preparada, sem aprovação.
func TestDevolverDepoisDaMeiaNoite(t *testing.T) {
	sp := fuso(t, "America/Sao_Paulo")
	antes := time.Date(2026, 11, 9, 23, 59, 0, 0, sp)
	e, c := enviando(t, antes)
	_, o, err := DevolverAprovada(e, c.ID, c.Versao, "erro-tecnico", antes.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	d, _ := candidaturaDe(o)
	if d.Estado != CandPreparada || d.Aprovacao != nil || d.ProximaTentativaEm != "" || len(d.Retentativas) != 1 || len(d.Tentativas) != 0 {
		t.Fatalf("depois da meia-noite = %+v", d)
	}
}

// Aprovação nova: as voltas contam de novo do zero; o histórico fica.
func TestVoltasPorAprovacao(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	e, c := enviando(t, agora)
	for n := 0; n < MaxRetentativas; n++ {
		e, _, _ = DevolverAprovada(e, c.ID, c.Versao, "erro-tecnico", agora)
		c = estadoDe(t, e, c.ID)
		if n < MaxRetentativas-1 {
			e, _, _ = IniciarEnvio(e, c.ID, c.Versao, agora.Add(3*time.Hour), nil)
			c = estadoDe(t, e, c.ID)
		}
	}
	if RetentativasDaAprovacao(c) != MaxRetentativas {
		t.Fatalf("voltas = %d", RetentativasDaAprovacao(c))
	}
	depois := agora.Add(4 * time.Hour)
	e, _, err := RevogarCandidatura(e, c.ID, depois, eu)
	if err != nil {
		t.Fatal(err)
	}
	c = estadoDe(t, e, c.ID)
	e, o, err := AprovarCandidatura(e, c.ID, c.Impressao, depois.Add(time.Second), nil, eu)
	if err != nil {
		t.Fatal(err)
	}
	c, _ = candidaturaDe(o)
	if RetentativasDaAprovacao(c) != 0 || len(c.Retentativas) != MaxRetentativas {
		t.Fatalf("aprovação nova: voltas %d, histórico %d", RetentativasDaAprovacao(c), len(c.Retentativas))
	}
}

// Item 3: as voltas não contam no limite, mas contadas + devolvidas no dia nunca passam de LimiteMaximo.
func TestTetoDePaginasNoDia(t *testing.T) {
	sp := fuso(t, "America/Sao_Paulo")
	agora := time.Date(2026, 11, 9, 8, 0, 0, 0, sp)
	limites := map[string]int{"linkedin": LimiteMaximo}
	e := comVaga(t, EstadoVazio(), "li-0", "linkedin", linkLI("0"), true)
	// Histórico fictício: 39 páginas devolvidas hoje noutra candidatura.
	e, c := aprovada(t, e, "li-0", agora, limites)
	cand, _ := candidaturaDe(e.Candidaturas[0])
	for n := 0; n < LimiteMaximo-1; n++ {
		cand.Retentativas = append(cand.Retentativas, Retentativa{Em: agora.Add(-time.Duration(n) * time.Minute).Format(time.RFC3339), Codigo: "erro-tecnico"})
	}
	e.Candidaturas[0] = cand.objeto()
	if AberturasNoDia(e, "linkedin", agora) != LimiteMaximo-1 || UsadasNoDia(e, "linkedin", agora) != 0 {
		t.Fatalf("abertas = %d", AberturasNoDia(e, "linkedin", agora))
	}
	e, o, err := IniciarEnvio(e, c.ID, c.Versao, agora, limites) // a 40ª página
	if err != nil {
		t.Fatal(err)
	}
	c, _ = candidaturaDe(o)
	e, _, _ = DevolverAprovada(e, c.ID, c.Versao, "erro-tecnico", agora)
	c = estadoDe(t, e, c.ID)
	_, _, err = IniciarEnvio(e, c.ID, c.Versao, agora.Add(time.Hour), limites)
	var el *ErroLimite
	if !errors.As(err, &el) {
		t.Fatalf("passou do teto de %d páginas: %v", LimiteMaximo, err)
	}
	// Dia seguinte: renova.
	amanha := time.Date(2026, 11, 10, 8, 0, 0, 0, sp)
	if AberturasNoDia(e, "linkedin", amanha) != 0 {
		t.Error("o teto não renovou à meia-noite")
	}
}

// Revisão do CyberSec no #99: ao abrir o app, a queda ANTES da marca gravada volta (conta como volta); DEPOIS da
// marca (interagiu ou clicou em enviar), incerta. Na 4ª queda antes da marca, falhou.
func TestRecuperarAoAbrir(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	depois := agora.Add(time.Minute)

	e, c := enviando(t, agora) // caiu antes da marca
	novo, incertas, devolvidas := RecuperarAoAbrir(e, depois)
	d := estadoDe(t, novo, c.ID)
	if incertas != 0 || devolvidas != 1 || d.Estado != CandAprovada || len(d.Retentativas) != 1 || VoltasNoDia(novo, "linkedin", depois) != 1 {
		t.Fatalf("queda antes da marca = %d %d %+v", incertas, devolvidas, d)
	}

	for _, marca := range []string{MarcaInteragiu, MarcaClicouEnviar} { // caiu depois da marca
		e, c := enviando(t, agora)
		e, _, _ = MarcarTentativa(e, c.ID, marca, agora)
		novo, incertas, devolvidas := RecuperarAoAbrir(e, depois)
		if incertas != 1 || devolvidas != 0 || estadoDe(t, novo, c.ID).Estado != CandIncerta {
			t.Fatalf("%s: queda depois da marca = %d %d %v", marca, incertas, devolvidas, estadoDe(t, novo, c.ID).Estado)
		}
	}

	// Já com 3 voltas desde a aprovação: a 4ª queda antes da marca é falhou.
	e, c = enviando(t, agora)
	for n := 0; n < MaxRetentativas; n++ {
		e, _, _ = DevolverAprovada(e, c.ID, c.Versao, "erro-tecnico", agora)
		c = estadoDe(t, e, c.ID)
		e, _, _ = IniciarEnvio(e, c.ID, c.Versao, agora.Add(3*time.Hour), nil)
		c = estadoDe(t, e, c.ID)
	}
	if novo, _, _ := RecuperarAoAbrir(e, agora.Add(4*time.Hour)); estadoDe(t, novo, c.ID).Estado != CandFalhou {
		t.Fatalf("4ª queda = %v", estadoDe(t, novo, c.ID).Estado)
	}
}

// PO (jc-a2y.6.13 com o .6.11): a volta automática não conta duas vezes no limite do aprovar. Ela sai das tentativas
// (não é enviada) e conta uma vez só, como aprovada que ainda não saiu.
func TestVoltaNaoContaDuasVezesNoLimite(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	e, c := enviando(t, agora)
	if n := UsadasNoDia(e, "linkedin", agora) + AprovadasPendentes(e, "linkedin", agora); n != 1 {
		t.Fatalf("em enviando, conta %d; queria 1", n)
	}
	e, _, err := DevolverAprovada(e, c.ID, c.Versao, "erro-tecnico", agora)
	if err != nil {
		t.Fatal(err)
	}
	if u, p := UsadasNoDia(e, "linkedin", agora), AprovadasPendentes(e, "linkedin", agora); u != 0 || p != 1 {
		t.Fatalf("depois da volta: usadas %d, pendentes %d; queria 0 e 1", u, p)
	}
}
