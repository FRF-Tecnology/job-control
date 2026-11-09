package navegador

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// PastaPerfil é o perfil próprio do app: <pasta de dados>/navegador. Nunca é o perfil pessoal do Chrome ou do Edge.
// Fica fora do git (a pasta de dados não está no repositório) e fora do backup (o backup diário copia só o banco).
// Guarda o cookie de sessão das plataformas: "apagar meus dados" tem de chamar ApagarPerfil.
func PastaPerfil(pastaDados string) string { return filepath.Join(pastaDados, "navegador") }

// preferencias desligam o que o app não usa: salvar senhas, preencher formulário sozinho, notificações,
// localização, câmera, microfone e downloads automáticos (2 = bloquear).
var preferencias = map[string]any{
	"credentials_enable_service":    false,
	"credentials_enable_autosignin": false,
	"profile": map[string]any{
		"password_manager_enabled": false,
		"default_content_setting_values": map[string]any{
			"notifications": 2, "geolocation": 2, "media_stream_camera": 2, "media_stream_mic": 2, "automatic_downloads": 2,
		},
	},
	"autofill":              map[string]any{"profile_enabled": false, "credit_card_enabled": false},
	"download":              map[string]any{"prompt_for_download": false, "directory_upgrade": false},
	"safebrowsing":          map[string]any{"enabled": true},
	"translate":             map[string]any{"enabled": false},
	"search":                map[string]any{"suggest_enabled": false},
	"alternate_error_pages": map[string]any{"enabled": false},
}

// prepararPerfil cria a pasta só para o usuário (0700) e grava as preferências do perfil (Default/Preferences),
// mantendo o que o navegador já tiver gravado e trocando só as chaves acima.
func prepararPerfil(pasta string) error {
	if err := os.MkdirAll(filepath.Join(pasta, "Default"), 0o700); err != nil {
		return err
	}
	if err := os.Chmod(pasta, 0o700); err != nil {
		return err
	}
	arquivo := filepath.Join(pasta, "Default", "Preferences")
	atual := map[string]any{}
	if b, err := os.ReadFile(arquivo); err == nil {
		json.Unmarshal(b, &atual) // arquivo estragado: recomeça
	}
	mesclar(atual, preferencias)
	b, err := json.Marshal(atual)
	if err != nil {
		return err
	}
	return os.WriteFile(arquivo, b, 0o600)
}

func mesclar(destino, origem map[string]any) {
	for k, v := range origem {
		if sub, ok := v.(map[string]any); ok {
			d, ok := destino[k].(map[string]any)
			if !ok {
				d = map[string]any{}
				destino[k] = d
			}
			mesclar(d, sub)
			continue
		}
		destino[k] = v
	}
}

// ApagarPerfil apaga o perfil próprio inteiro (cookies, sessão e cache das plataformas). O navegador tem de estar
// fechado.
func ApagarPerfil(pastaDados string) error { return os.RemoveAll(PastaPerfil(pastaDados)) }
