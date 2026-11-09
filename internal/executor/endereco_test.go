package executor

import (
	"context"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// P0 da .6.5: a página do envio parou fora da vaga. Cada caso tem o desfecho certo, nenhum envia, e o log diz só a
// categoria (nunca o endereço). Com o jc-a2y.6.11, about:blank e login ANTES de interagir voltam para aprovada (com o
// código), e o login deixa a plataforma em precisa-entrar.
func TestEnderecoFinalDecideODesfecho(t *testing.T) {
	casos := []struct {
		final, estado, motivo, categoria string
	}{
		{"about:blank", dominio.CandAprovada, "erro-tecnico", "about:blank"},
		{"https://www.linkedin.com/authwall?trk=ficticio", dominio.CandAprovada, "login", "login"},
		{"https://www.linkedin.com/checkpoint/challenge/ficticio", dominio.CandAprovada, "login", "login"},
		{"https://ats.exemplo.io/form/1", dominio.CandBloqueada, "site-externo", "outro-host"},
		{"https://www.linkedin.com/jobs/view/9000000099/", dominio.CandBloqueada, "site-externo", "outra-vaga"},
	}
	for _, c := range casos {
		p := novaPagina()
		p.final = c.final
		s := novoCenario(t, p)
		d, err := s.x.Enviar(context.Background(), s.aprovada("linkedin", linkLI))
		if err != nil {
			t.Fatal(err)
		}
		if d.Estado != c.estado || p.enviou != 0 {
			t.Errorf("%s: desfecho = %+v, envios %d; queria %s sem envio", c.final, d, p.enviou, c.estado)
		}
		if d.Motivo != c.motivo {
			t.Errorf("%s: motivo = %q; queria %q", c.final, d.Motivo, c.motivo)
		}
		log := strings.Join(s.logs, "\n")
		if !strings.Contains(log, "endereço final: "+c.categoria) || strings.Contains(log, "exemplo.io") || strings.Contains(log, "authwall") || strings.Contains(log, "9000000099") {
			t.Errorf("%s: log = %q", c.final, log)
		}
	}
}
