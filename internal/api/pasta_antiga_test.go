package api

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/config"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// jc-etv.7, achado médio do CyberSec no #113 (LGPD): quando as duas pastas padrão existem (pasta-antiga-ficou), a
// "Central de Candidaturas" continua com o banco, os currículos, os backups e o perfil do navegador. "Apagar meus
// dados" e "sair das plataformas" limpam também a antiga, pelas mesmas regras, e a resposta diz (pasta-antiga).
// Casa fictícia: nunca a pasta de verdade.

func casaComAsDuasPastas(t *testing.T) (nova, antiga string) {
	t.Helper()
	casa := filepath.Join(t.TempDir(), "Usuario Ficticio")
	t.Setenv("HOME", casa)
	t.Setenv("USERPROFILE", casa)
	t.Setenv("APPDATA", filepath.Join(casa, "AppData", "Roaming"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(casa, ".config"))
	t.Setenv("APPLYER_PASTA", "")
	t.Setenv("CENTRAL_PASTA", "")
	base, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	nova, antiga = filepath.Join(base, config.NomePasta), filepath.Join(base, config.NomePastaAntiga)
	for _, arq := range []string{"central.db", "central.db-wal", "curriculos/cv-antigo.pdf", "backups/central-2026-11-01.db", "navegador/Default/Cookies"} {
		os.MkdirAll(filepath.Dir(filepath.Join(antiga, arq)), 0o700)
		if err := os.WriteFile(filepath.Join(antiga, arq), []byte("ficticio"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return nova, antiga
}

func TestApagarLimpaTambemAPastaAntiga(t *testing.T) {
	nova, antiga := casaComAsDuasPastas(t)
	f := novaPastaFicticiaEm(t, segredos.Memoria{}, nova)
	res := f.chamar("POST", "/dados/apagar", Objeto{"confirmacao": FraseApagar}, 200)
	if strings.Join(lista(res["apagado"]), ",") != "backups,banco,chaves,curriculos,navegador,pasta-antiga" || len(lista(res["faltou"])) != 0 || res["completo"] != true {
		t.Fatalf("resposta = %v", res)
	}
	for _, p := range []string{nova, antiga} {
		if _, err := os.Lstat(p); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s ainda existe", filepath.Base(p))
		}
	}
}

func TestSairLimpaTambemONavegadorDaPastaAntiga(t *testing.T) {
	nova, antiga := casaComAsDuasPastas(t)
	f := novaPastaFicticiaEm(t, segredos.Memoria{}, nova)
	res := f.chamar("POST", "/navegador/sair", Objeto{}, 200)
	if strings.Join(lista(res["apagado"]), ",") != "navegador,pasta-antiga" || res["completo"] != true {
		t.Fatalf("resposta = %v", res)
	}
	if _, err := os.Lstat(filepath.Join(antiga, "navegador")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("o perfil do navegador ficou na pasta antiga")
	}
	// Sair só tira a sessão: o resto da antiga fica, como na pasta em uso.
	if _, err := os.Stat(filepath.Join(antiga, "central.db")); err != nil {
		t.Fatal("sair apagou o banco da pasta antiga")
	}
}

// Sem a antiga, ou com a pasta em uso fora da padrão (APPLYER_PASTA), a antiga não entra.
func TestPastaAntigaSoComAPadraoEmUso(t *testing.T) {
	_, antiga := casaComAsDuasPastas(t)
	outra := filepath.Join(t.TempDir(), "teste")
	t.Setenv("APPLYER_PASTA", outra)
	f := novaPastaFicticiaEm(t, segredos.Memoria{}, outra)
	res := f.chamar("POST", "/navegador/sair", Objeto{}, 200)
	if strings.Join(lista(res["apagado"]), ",") != "navegador" {
		t.Fatalf("resposta = %v", res)
	}
	if _, err := os.Stat(filepath.Join(antiga, "navegador", "Default", "Cookies")); err != nil {
		t.Fatal("mexeu na pasta antiga com a pasta em uso fora da padrão")
	}
}
