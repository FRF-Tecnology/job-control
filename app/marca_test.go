package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/config"
)

// jc-a2y.61: o nome visível do produto é Applyer nos metadados do build (executável, instaladores, Mac e Linux).
// Os nomes internos (pasta de dados, cofre, identidade do app, executável) só mudam com o plano do jc-etv.6
// aprovado: mudar sem migração faria o app abrir vazio ou perder as chaves de IA.
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
		"build/darwin/Info.plist":             {"<string>com.frftecnology.central</string>"},
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
	// O executável do app tem o nome que cada sistema gera (jc-etv.10): central.exe no Windows (MSIX), central no Mac,
	// porque o Taskfile do Mac copia o binário sem .exe. Com central.exe no plist, o .app não abria.
	for _, plist := range []string{"build/darwin/Info.plist", "build/darwin/Info.dev.plist"} {
		if !regexp.MustCompile(`<key>CFBundleExecutable</key>\s*<string>central</string>`).MatchString(ler(plist)) {
			t.Errorf("%s: CFBundleExecutable tem de ser central (o binário do Mac)", plist)
		}
	}
	if config.NomePasta != "Central de Candidaturas" {
		t.Errorf("pasta de dados = %q; só muda com a migração do jc-etv.6", config.NomePasta)
	}
}
