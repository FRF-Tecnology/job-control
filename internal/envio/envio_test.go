package envio

import (
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Sem navegador, o Controle não faz nada e os erros de Entrar trazem o motivo que a tela usa.
func TestDesligadoNaoFazNada(t *testing.T) {
	c := desligado()
	c.Suspender()()
	c.FecharNavegador()
	c.MostrarEnvio(true)
	if v, l := c.Janela(); v || l {
		t.Fatalf("sem navegador com janela: visível %v, login %v", v, l)
	}
	if err := c.Entrar("https://www.linkedin.com/login"); err != ErrSemNavegador {
		t.Fatalf("Entrar sem navegador = %v", err)
	}
	for _, e := range []struct {
		err    error
		motivo string
	}{{ErrSemNavegador, "sem-navegador"}, {ErrLoginAberto, "login-aberto"}, {ErrEnviando, "enviando"}} {
		if c, ok := e.err.(*dominio.ErroConflito); !ok || c.Motivo != e.motivo {
			t.Errorf("motivo de %v = %+v; queria %s", e.err, e.err, e.motivo)
		}
	}
	c.Parar()
}
