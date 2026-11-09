package segredos

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// jc-a2y.65: cada pasta de dados tem o seu nome no cofre; a padrão continua com o de sempre.
func TestServicoDaPasta(t *testing.T) {
	casa := t.TempDir()
	t.Setenv("APPDATA", filepath.Join(casa, "Roaming"))         // Windows
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(casa, ".config")) // Linux
	t.Setenv("HOME", casa)                                      // macOS
	padrao := map[string]string{"windows": filepath.Join(casa, "Roaming"), "darwin": filepath.Join(casa, "Library", "Application Support")}[runtime.GOOS]
	if padrao == "" {
		padrao = filepath.Join(casa, ".config")
	}
	// jc-etv.7: a pasta padrão nova (Applyer) e a antiga (antes do rename, ou com o rename adiado) dão o mesmo nome,
	// para as chaves de IA não sumirem na migração. O serviço do cofre só muda no PR A2.
	for _, nome := range []string{"Applyer", "Central de Candidaturas"} {
		if got := ServicoDaPasta(filepath.Join(padrao, nome)); got != servico {
			t.Errorf("pasta padrão %s = %q; queria o nome de sempre", nome, got)
		}
	}
	a, b := filepath.Join(casa, "teste-a"), filepath.Join(casa, "teste-b")
	sa, sb := ServicoDaPasta(a), ServicoDaPasta(b)
	if sa == servico || sb == servico || sa == sb {
		t.Errorf("pastas de teste: %q e %q; queria nomes próprios e diferentes", sa, sb)
	}
	if !strings.HasPrefix(sa, servico+" - pasta ") || len(sa) != len(servico+" - pasta ")+12 {
		t.Errorf("formato = %q", sa)
	}
	if strings.Contains(sa, "teste-a") || strings.Contains(sa, casa) {
		t.Errorf("o caminho foi para o nome do cofre: %q", sa)
	}
	if ServicoDaPasta(a) != sa || ServicoDaPasta(a+string(filepath.Separator)) != sa {
		t.Error("a mesma pasta deu nomes diferentes")
	}
	if runtime.GOOS == "windows" && ServicoDaPasta(strings.ToUpper(a)) != sa {
		t.Error("no Windows, a mesma pasta em maiúsculas deu outro nome")
	}
	if got := ServicoDaPasta(""); got == servico || !strings.HasPrefix(got, servico) {
		t.Errorf("banco em memória = %q; queria um nome próprio", got)
	}
}
