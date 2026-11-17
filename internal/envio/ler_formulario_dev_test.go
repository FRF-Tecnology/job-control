//go:build envio_dev

package envio

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
)

// Ler um formulário com um ciclo do executor em andamento recusa na hora (a rota dá 409), sem esperar sem limite
// (baixa do Fallen no #124). Nenhuma página abre: o ciclo está ocupado.
func TestLerFormularioComCicloOcupadoRecusa(t *testing.T) {
	dados := t.TempDir()
	arm, err := armazem.Abrir(context.Background(), filepath.Join(dados, "central.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	c := Ligar(context.Background(), arm, dados, nil)
	t.Cleanup(c.Parar)
	if c.LerFormulario == nil {
		if os.Getenv("CI") == "true" {
			t.Fatal("sem navegador no CI")
		}
		t.Skip("sem Chrome nem Edge nesta máquina")
	}
	soltar := c.Suspender() // segura o ciclo, como um envio em andamento
	defer soltar()
	feito := make(chan error, 1)
	go func() {
		_, err := c.LerFormulario(context.Background(), "https://docs.google.com/forms/d/e/1FAIpQLSf_x/viewform")
		feito <- err
	}()
	select {
	case err := <-feito:
		if !errors.Is(err, ErrEnviando) {
			t.Fatalf("com o ciclo ocupado = %v; queria ErrEnviando", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a leitura esperou o ciclo em vez de recusar")
	}
}
