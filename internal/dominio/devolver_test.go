package dominio

import (
	"testing"
	"time"
)

// jc-a2y.6.11: a volta automática para aprovada só sai de enviando, devolve a aprovação sem uso, tira a tentativa do
// limite e marca a próxima; na 4ª, falhou. Dados fictícios.
func TestDevolverAprovada(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600))
	cand := func(estado string, retentativas int) Estado {
		c := Candidatura{
			ID: "cand-1", VagaID: "v-1", Plataforma: "linkedin", URL: "https://www.linkedin.com/jobs/view/9000000001",
			Estado: estado, Impressao: "sha256:v1:x", Versao: 3,
			Aprovacao:  &Aprovacao{Em: agora.Format(time.RFC3339), Impressao: "sha256:v1:x", ValidaAte: FimDoDia(agora).Format(time.RFC3339), UsadaEm: agora.Format(time.RFC3339)},
			Tentativas: []string{agora.Add(-time.Hour).Format(time.RFC3339), agora.Format(time.RFC3339)},
		}
		for i := 0; i < retentativas; i++ {
			c.Retentativas = append(c.Retentativas, Retentativa{Em: agora.Format(time.RFC3339), Codigo: "erro-tecnico"})
		}
		e := EstadoVazio()
		e.Candidaturas = []Objeto{c.objeto()}
		return e
	}

	novo, o, err := DevolverAprovada(cand(CandEnviando, 0), "cand-1", "erro-tecnico", agora)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := candidaturaDe(o)
	if c.Estado != CandAprovada || c.Aprovacao.UsadaEm != "" || len(c.Tentativas) != 1 || len(c.Retentativas) != 1 || c.Versao != 4 {
		t.Fatalf("devolvida = %+v", c)
	}
	if c.ProximaTentativa != agora.Add(5*time.Minute).Format(time.RFC3339) || ProntaParaTentar(c, agora) || !ProntaParaTentar(c, agora.Add(5*time.Minute)) {
		t.Fatalf("próxima tentativa = %q", c.ProximaTentativa)
	}
	if usadas := UsadasNoDia(novo, "linkedin", agora); usadas != 1 {
		t.Errorf("a tentativa devolvida contou no limite: usadas = %d", usadas)
	}

	// 2ª e 3ª voltas: 30 min e 2 h.
	for n, espera := range map[int]time.Duration{1: 30 * time.Minute, 2: 2 * time.Hour} {
		_, o, err := DevolverAprovada(cand(CandEnviando, n), "cand-1", "login", agora)
		if err != nil {
			t.Fatal(err)
		}
		c, _ := candidaturaDe(o)
		if c.ProximaTentativa != agora.Add(espera).Format(time.RFC3339) || c.Retentativas[n].Codigo != "login" {
			t.Errorf("volta %d: %+v", n+1, c)
		}
	}

	// 4ª: falhou, como antes.
	if _, o, err := DevolverAprovada(cand(CandEnviando, MaxRetentativas), "cand-1", "erro-tecnico", agora); err != nil || o["estado"] != CandFalhou {
		t.Fatalf("depois de 3 voltas = %v, %v", o["estado"], err)
	}

	// Só de enviando; código fora da lista.
	for _, estado := range []string{CandAprovada, CandEnviada, CandIncerta, CandBloqueada} {
		if _, _, err := DevolverAprovada(cand(estado, 0), "cand-1", "erro-tecnico", agora); err == nil {
			t.Errorf("devolveu de %s", estado)
		}
	}
	if _, _, err := DevolverAprovada(cand(CandEnviando, 0), "cand-1", "site-externo", agora); err == nil {
		t.Error("devolveu com código fora da lista")
	}
}
