package api

import (
	"context"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// jc-etv.14 (contrato do #113): GET /dados/pasta diz à tela o que aconteceu com a pasta de dados ao abrir o app, sem
// caminho. O aviso é o desta abertura (vem do app); migradaEm fica no banco desde o rename. Dados fictícios.

func apiComAviso(t *testing.T, aviso string) (*API, *armazem.Armazem) {
	t.Helper()
	arm, err := armazem.Abrir(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	a := Nova(arm, Opcoes{Hoje: func() string { return "2026-11-09" }, Cofre: segredos.Memoria{}, ProvedoresIA: map[string]ia.Provedor{},
		DetectarCLIs: func(context.Context) []ia.EstadoCLI { return nil }, AvisoPasta: aviso})
	return a, arm
}

func TestDadosPasta(t *testing.T) {
	for _, c := range []struct {
		nome, aviso string
		quer        any
	}{
		{"sem aviso", "", nil},
		{"as duas pastas", "pasta-antiga-ficou", "pasta-antiga-ficou"},
		{"rename adiado", "migracao-adiada", "migracao-adiada"},
		{"aviso desconhecido", `C:\Users\Pessoa`, nil}, // nada fora da lista fechada, nunca caminho
	} {
		a, _ := apiComAviso(t, c.aviso)
		r := chamarEnvio(t, a, "GET", "/dados/pasta", nil, "claude") // só leitura, sem caminho: qualquer autor
		o, _ := r.Corpo.(map[string]any)
		if r.Status != 200 || o["aviso"] != c.quer || o["migradaEm"] != nil || len(o) != 2 {
			t.Errorf("%s: %d %v", c.nome, r.Status, o)
		}
	}
	// migradaEm vem do banco (gravado pelo app no rename) e fica nas aberturas seguintes, com ou sem aviso.
	a, arm := apiComAviso(t, "pasta-antiga-ficou")
	if err := arm.GravarMeta(context.Background(), "pasta", map[string]any{"migradaEm": "2026-11-09T10:00:00-03:00"}); err != nil {
		t.Fatal(err)
	}
	o, _ := chamarEnvio(t, a, "GET", "/dados/pasta", nil, "eu").Corpo.(map[string]any)
	if o["migradaEm"] != "2026-11-09T10:00:00-03:00" || o["aviso"] != "pasta-antiga-ficou" {
		t.Fatalf("com migradaEm = %v", o)
	}
	// Meta antiga ou estranha: só migradaEm em texto passa; nada de caminho.
	arm.GravarMeta(context.Background(), "pasta", map[string]any{"migradaEm": 12, "caminho": `C:\Users\Pessoa\AppData`})
	o, _ = chamarEnvio(t, a, "GET", "/dados/pasta", nil, "eu").Corpo.(map[string]any)
	if o["migradaEm"] != nil || strings.Contains(strings.ToLower(strings.Join(chaves(o), ",")), "caminho") {
		t.Fatalf("meta estranha = %v", o)
	}
	// Só GET.
	if r := chamarEnvio(t, a, "POST", "/dados/pasta", Objeto{}, "eu"); r.Status != 404 {
		t.Fatalf("POST = %d", r.Status)
	}
}

func chaves(o map[string]any) []string {
	l := []string{}
	for k := range o {
		l = append(l, k)
	}
	return l
}
