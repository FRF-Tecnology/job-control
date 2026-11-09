// Package navegador é o navegador real do executor (P5, jc-a2y.6.2.7): Chrome ou Edge, com perfil próprio na pasta
// de dados, conduzido pelo chromedp. Implementa executor.Navegador e executor.Pagina. Não envia nada sozinho: quem
// decide é o executor, com as regras M1 a M7.
package navegador

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// ErrSemNavegador: nenhum Chrome nem Edge nos locais conhecidos.
var ErrSemNavegador = errors.New("Chrome ou Edge não encontrado nos locais de instalação")

// locaisDoNavegador é a lista fixa de onde procurar, na ordem: Chrome e depois Edge. Não usa PATH nem configuração
// da pessoa; no Windows, só as pastas de programas do sistema e a instalação por usuário (pasta local do usuário).
func locaisDoNavegador(goos string, env func(string) string, home, local string) []string {
	var l []string
	switch goos {
	case "windows":
		pf, pf86 := env("ProgramFiles"), env("ProgramFiles(x86)")
		for _, base := range []string{pf, pf86, local} {
			if base != "" {
				l = append(l, filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"))
			}
		}
		for _, base := range []string{pf86, pf, local} {
			if base != "" {
				l = append(l, filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe"))
			}
		}
	case "darwin":
		apps := []string{"/Applications"}
		if home != "" {
			apps = append(apps, filepath.Join(home, "Applications"))
		}
		for _, a := range apps {
			l = append(l, filepath.Join(a, "Google Chrome.app", "Contents", "MacOS", "Google Chrome"))
		}
		for _, a := range apps {
			l = append(l, filepath.Join(a, "Microsoft Edge.app", "Contents", "MacOS", "Microsoft Edge"))
		}
	default: // Linux: só para desenvolvimento e CI
		l = append(l, "/usr/bin/google-chrome", "/usr/bin/google-chrome-stable", "/opt/google/chrome/chrome",
			"/usr/bin/microsoft-edge", "/usr/bin/microsoft-edge-stable", "/usr/bin/chromium", "/usr/bin/chromium-browser")
	}
	return l
}

// Localizar devolve o primeiro executável que existe na lista fixa.
func Localizar() (string, error) {
	home, _ := os.UserHomeDir()
	local := ""
	if runtime.GOOS == "windows" {
		local, _ = os.UserCacheDir() // %LocalAppData%: Chrome instalado só para o usuário
	}
	for _, c := range locaisDoNavegador(runtime.GOOS, os.Getenv, home, local) {
		if info, err := os.Stat(c); err == nil && info.Mode().IsRegular() {
			return c, nil
		}
	}
	return "", ErrSemNavegador
}
