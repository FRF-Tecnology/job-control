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
	// Só no CI (revisão do CyberSec no #93): a API deriva o nome do cofre sozinha, e um erro nela (voltar a
	// Sistema{}) gravaria e apagaria a chave real deste PC antes de qualquer conferência do teste.
	if os.Getenv("CI") != "true" {
		t.Skip("cofre do sistema de verdade: só no CI")
	}
	// Sentinela: a chave do nome real (a pasta padrão) não pode mudar em nenhum passo deste teste.
	padrao, err := config.PastaPadrao()
	if err != nil {
		t.Fatal(err)
	}
	servicoReal := segredos.ServicoDaPasta(padrao)
	antes, errAntes := keyring.Get(servicoReal, "anthropic")
	if errAntes != nil {
		t.Cleanup(func() { keyring.Delete(servicoReal, "anthropic") }) // não existia: se o teste criou, tira (só no CI)
	}
	sentinela := func(passo string) {
		t.Helper()
		depois, errDepois := keyring.Get(servicoReal, "anthropic")
		if depois != antes || (errAntes == nil) != (errDepois == nil) {
			t.Fatalf("%s mexeu na chave do cofre real (a API não usou o nome da pasta)", passo)
		}
	}
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
		sentinela("gravar a chave na pasta de teste")
	}
	if r := chamarEnvio(t, apis[0], "POST", "/dados/apagar", Objeto{"confirmacao": FraseApagar}, "eu"); r.Status != 200 {
		t.Fatalf("apagar a pasta A = %d %v", r.Status, r.Corpo)
	}
	sentinela("apagar os dados da pasta de teste")
	if c, _ := keyring.Get(segredos.ServicoDaPasta(pastas[0]), "anthropic"); c != "" {
		t.Error("a chave da pasta A ficou no cofre")
	}
	if c, err := keyring.Get(segredos.ServicoDaPasta(pastas[1]), "anthropic"); c != "sk-ant-ficticia-pasta-b-1234" {
		t.Errorf("apagar a pasta A tirou a chave da pasta B (%v)", err)
	}
}
