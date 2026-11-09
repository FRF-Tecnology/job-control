package navegador

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLocaisDoNavegador(t *testing.T) {
	env := map[string]string{"ProgramFiles": `C:\Program Files`, "ProgramFiles(x86)": `C:\Program Files (x86)`, "PATH": `C:\malicioso`}
	l := locaisDoNavegador("windows", func(k string) string { return env[k] }, `C:\Users\teste`, `C:\Users\teste\AppData\Local`)
	if len(l) != 6 || !strings.HasSuffix(l[0], filepath.Join("Google", "Chrome", "Application", "chrome.exe")) || !strings.HasSuffix(l[5], "msedge.exe") {
		t.Fatalf("windows = %v", l)
	}
	mac := locaisDoNavegador("darwin", func(string) string { return "" }, "/Users/teste", "")
	if len(mac) != 4 || mac[0] != filepath.Join("/Applications", "Google Chrome.app", "Contents", "MacOS", "Google Chrome") || !strings.Contains(mac[3], "Microsoft Edge") {
		t.Fatalf("macOS = %v", mac)
	}
	for _, goos := range []string{"windows", "darwin", "linux"} {
		for _, c := range locaisDoNavegador(goos, func(k string) string { return env[k] }, "/home/teste", `C:\local`) {
			if strings.Contains(c, "malicioso") {
				t.Fatalf("%s: usou o PATH: %s", goos, c)
			}
		}
	}
}

// O PATH não conta: um "chrome" falso no PATH não é escolhido.
func TestLocalizarIgnoraPATH(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"chrome", "chrome.exe", "google-chrome", "msedge.exe"} {
		os.WriteFile(filepath.Join(dir, n), []byte("falso"), 0o755)
	}
	t.Setenv("PATH", dir)
	if c, err := Localizar(); err == nil && strings.HasPrefix(c, dir) {
		t.Fatalf("escolheu o executável do PATH: %s", c)
	}
}

func TestPerfilProprio(t *testing.T) {
	dados := t.TempDir()
	p := PastaPerfil(dados)
	if p != filepath.Join(dados, "navegador") {
		t.Fatalf("pasta = %s", p)
	}
	os.MkdirAll(filepath.Join(p, "Default"), 0o755)
	os.WriteFile(filepath.Join(p, "Default", "Preferences"), []byte(`{"outra":1,"profile":{"name":"Pessoa"}}`), 0o644)
	if err := prepararPerfil(p); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(p); info.Mode().Perm() != 0o700 {
			t.Fatalf("permissão = %v", info.Mode().Perm())
		}
	}
	b, _ := os.ReadFile(filepath.Join(p, "Default", "Preferences"))
	var pref map[string]any
	json.Unmarshal(b, &pref)
	perfil := pref["profile"].(map[string]any)
	permissoes := perfil["default_content_setting_values"].(map[string]any)
	if pref["credentials_enable_service"] != false || perfil["password_manager_enabled"] != false || pref["outra"] != 1.0 || perfil["name"] != "Pessoa" ||
		permissoes["notifications"] != 2.0 || permissoes["geolocation"] != 2.0 || permissoes["automatic_downloads"] != 2.0 ||
		pref["autofill"].(map[string]any)["profile_enabled"] != false {
		t.Fatalf("preferências = %s", b)
	}
	if err := ApagarPerfil(dados); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("perfil não foi apagado: %v", err)
	}
	if _, err := os.Stat(dados); err != nil {
		t.Fatal("apagou a pasta de dados junto")
	}
}

// A resolução de host para o servidor local e o certificado de teste só existem em arquivo de teste: o caminho de
// produção não consegue apontar o navegador para outro servidor nem aceitar certificado falso.
func TestFlagsDeTesteSoNosTestes(t *testing.T) {
	var arquivos []string
	for _, padrao := range []string{"*.go", "../envio/*.go", "../executor/*.go", "../../app/*.go", "../../cmd/*/*.go"} {
		m, _ := filepath.Glob(padrao)
		arquivos = append(arquivos, m...)
	}
	vistos := 0
	for _, a := range arquivos {
		if strings.HasSuffix(a, "_test.go") {
			continue
		}
		b, err := os.ReadFile(a)
		if err != nil {
			t.Fatal(err)
		}
		vistos++
		for _, proibido := range []string{"host-resolver-rules", "ignore-certificate-errors", "remote-debugging-port", "remote-debugging-address", "extrasDeTeste ="} {
			if strings.Contains(string(b), proibido) {
				t.Errorf("%s tem %q fora de teste", a, proibido)
			}
		}
	}
	if vistos < 5 {
		t.Fatalf("só %d arquivos conferidos", vistos)
	}
}
