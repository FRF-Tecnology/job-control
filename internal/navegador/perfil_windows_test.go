//go:build windows

package navegador

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/config"
	"golang.org/x/sys/windows"
)

// jc-a2y.58: o perfil que já existia numa pasta com herança aberta (como C:\Users\Public) fica só do usuário,
// SISTEMA e Administradores, com o Preferences e o cookie dentro. O teste confere pela DACL.
func TestPerfilExistenteFicaSoDoUsuario(t *testing.T) {
	mae := t.TempDir()
	// Herança aberta: Modificar para INTERATIVO e Usuários, herdável.
	var ea []windows.EXPLICIT_ACCESS
	for _, tipo := range []windows.WELL_KNOWN_SID_TYPE{windows.WinInteractiveSid, windows.WinBuiltinUsersSid} {
		sid, err := windows.CreateWellKnownSid(tipo)
		if err != nil {
			t.Fatal(err)
		}
		ea = append(ea, windows.EXPLICIT_ACCESS{AccessPermissions: 0x1301BF, AccessMode: windows.GRANT_ACCESS,
			Inheritance: windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
			Trustee:     windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_WELL_KNOWN_GROUP, TrusteeValue: windows.TrusteeValueFromSID(sid)}})
	}
	acl, err := windows.ACLFromEntries(ea, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(mae, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}

	perfil := filepath.Join(mae, "navegador")
	cookie := filepath.Join(perfil, "Default", "Network", "Cookies")
	if err := os.MkdirAll(filepath.Dir(cookie), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cookie, []byte("sessao-ficticia"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !temInterativo(t, cookie) {
		t.Fatal("o teste supõe que o cookie herdou INTERATIVO")
	}

	if err := prepararPerfil(perfil); err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{perfil, filepath.Join(perfil, "Default", "Preferences"), cookie} {
		protegida, entradas, err := config.LerDACL(c)
		if err != nil {
			t.Fatal(err)
		}
		if c == perfil && !protegida {
			t.Errorf("perfil: DACL não está protegida")
		}
		for _, e := range entradas {
			switch e.SID {
			case "S-1-5-18", "S-1-5-32-544":
			default:
				if !ehUsuario(t, e.SID) {
					t.Errorf("%s: SID %s tem acesso", filepath.Base(c), e.SID)
				}
			}
		}
	}
}

func temInterativo(t *testing.T, caminho string) bool {
	t.Helper()
	_, entradas, err := config.LerDACL(caminho)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entradas {
		if e.SID == "S-1-5-4" {
			return true
		}
	}
	return false
}

func ehUsuario(t *testing.T, sid string) bool {
	t.Helper()
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	return u.User.Sid.String() == sid
}
