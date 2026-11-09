package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/config"
)

// jc-a2y.61: o nome visível do produto é Applyer nos metadados do build (executável, instaladores, Mac e Linux).
// Os nomes internos (pasta de dados, cofre, identidade do app, executável) só mudam com o plano do jc-etv.6
// aprovado e a migração de cada um: mudar sem migração faria o app abrir vazio ou perder as chaves de IA.
//   - PR A1 (jc-etv.7): a pasta de dados já é Applyer, com a antiga renomeada ao abrir (config.AbrirPastaDados).
//   - O resto continua travado: o serviço do cofre até o PR A2; identidade, MSIX e executável até o PR B (jc-etv.8).
func TestMarcaApplyerSemMudarNomesInternos(t *testing.T) {
	ler := func(caminho string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.FromSlash(caminho))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	var info struct {
		Info map[string]map[string]string `json:"info"`
	}
	if err := json.Unmarshal([]byte(ler("build/windows/info.json")), &info); err != nil {
		t.Fatal(err)
	}
	for _, campo := range []string{"ProductName", "FileDescription"} {
		if got := info.Info["0409"][campo]; got != "Applyer" {
			t.Errorf("info.json %s = %q; queria Applyer", campo, got)
		}
	}

	visivel := map[string][]string{
		"build/config.yml":                    {`productName: "Applyer"`},
		"build/darwin/Info.plist":             {"<string>Applyer</string>"},
		"build/linux/desktop":                 {"Name=Applyer"},
		"build/windows/nsis/wails_tools.nsh":  {`INFO_PRODUCTNAME "Applyer"`},
		"build/windows/msix/app_manifest.xml": {"<DisplayName>Applyer</DisplayName>"},
		"main.go":                             {`Name:        "Applyer"`},
	}
	for arq, trechos := range visivel {
		texto := ler(arq)
		for _, tr := range trechos {
			if !strings.Contains(texto, tr) {
				t.Errorf("%s não tem %q", arq, tr)
			}
		}
	}

	// Internos: iguais até o jc-etv.6.
	internos := map[string][]string{
		"build/config.yml":                    {`productIdentifier: "com.frftecnology.central"`},
		"build/darwin/Info.plist":             {"<string>com.frftecnology.central</string>", "<string>central.exe</string>"},
		"build/windows/msix/app_manifest.xml": {`Name="com.frftecnology.central"`, `Executable="central.exe"`},
		"Taskfile.yml":                        {`APP_NAME: "central"`},
		"../internal/segredos/segredos.go":    {`servico = "Central de Candidaturas"`},
	}
	for arq, trechos := range internos {
		texto := ler(arq)
		for _, tr := range trechos {
			if !strings.Contains(texto, tr) {
				t.Errorf("%s: nome interno mudou (%q); só com o plano do jc-etv.6", arq, tr)
			}
		}
	}
	// Pasta de dados (jc-etv.7, PR A1): a nova é Applyer, e a antiga, que a migração renomeia, continua com o nome de
	// antes. Trocar qualquer uma das duas sem nova migração deixaria a pasta de quem já usa para trás.
	if config.NomePasta != "Applyer" {
		t.Errorf("pasta de dados = %q; queria Applyer (jc-etv.7)", config.NomePasta)
	}
	if config.NomePastaAntiga != "Central de Candidaturas" {
		t.Errorf("pasta antiga = %q; a migração do jc-etv.7 renomeia a Central de Candidaturas", config.NomePastaAntiga)
	}
}
