//go:build windows || darwin

package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/config"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// jc-a2y.65 (achado do QA no ensaio da .6.5): "apagar meus dados" numa pasta de teste apagava as chaves de IA da
// pessoa, porque todas as pastas usavam o mesmo nome no cofre. Duas pastas fictícias, o cofre do sistema de verdade,
// cada uma com o seu nome (nunca o real): apagar uma não tira a chave da outra. Chaves fictícias.
func TestApagarSoApagaAsChavesDestaPasta(t *testing.T) {
	raiz := t.TempDir()
	pastas := []string{filepath.Join(raiz, "teste-a"), filepath.Join(raiz, "teste-b")}
	apis := make([]*API, 2)
	for i, p := range pastas {
		servico := segredos.ServicoDaPasta(p)
		if real, err := config.PastaPadrao(); err != nil || config.MesmaPasta(p, real) || servico == segredos.ServicoDaPasta(real) {
			t.Fatalf("a pasta de teste não pode usar o cofre da pasta padrão (%q)", servico)
		}
		t.Cleanup(func() {
			for _, prov := range comChave {
				keyring.Delete(servico, prov)
			}
		})
		if err := os.MkdirAll(p, 0o700); err != nil {
			t.Fatal(err)
		}
		arm, err := armazem.Abrir(context.Background(), config.CaminhoBanco(p))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { arm.Fechar() })
		apis[i] = Nova(arm, Opcoes{Hoje: func() string { return "2026-11-09" }, ProvedoresIA: map[string]ia.Provedor{},
			DetectarCLIs: func(context.Context) []ia.EstadoCLI { return nil }}) // Cofre nil: o do sistema, desta pasta
	}
	for i, a := range apis {
		r := chamarEnvio(t, a, "PUT", "/ia/chaves/anthropic", Objeto{"chave": "sk-ant-ficticia-pasta-" + string(rune('a'+i)) + "-1234"}, "eu")
		if r.Status != 200 {
			t.Fatalf("gravar a chave na pasta %d = %d %v", i, r.Status, r.Corpo)
		}
	}
	if r := chamarEnvio(t, apis[0], "POST", "/dados/apagar", Objeto{"confirmacao": FraseApagar}, "eu"); r.Status != 200 {
		t.Fatalf("apagar a pasta A = %d %v", r.Status, r.Corpo)
	}
	if c, _ := keyring.Get(segredos.ServicoDaPasta(pastas[0]), "anthropic"); c != "" {
		t.Error("a chave da pasta A ficou no cofre")
	}
	if c, err := keyring.Get(segredos.ServicoDaPasta(pastas[1]), "anthropic"); c != "sk-ant-ficticia-pasta-b-1234" {
		t.Errorf("apagar a pasta A tirou a chave da pasta B (%v)", err)
	}
}
