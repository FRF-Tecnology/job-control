//go:build windows

package config

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// SIDs que nunca podem aparecer na pasta de dados nem no perfil (jc-a2y.58).
var sidsProibidos = map[string]string{
	"S-1-5-4":      "INTERATIVO",
	"S-1-5-6":      "SERVICE",
	"S-1-5-3":      "LOTE",
	"S-1-5-32-545": "Usuários",
	"S-1-1-0":      "Todos",
	"S-1-5-11":     "Usuários autenticados",
	"S-1-3-0":      "PROPRIETÁRIO CRIADOR",
}

// abrirHeranca reproduz o FAIL do QA (C:\Users\Public): a pasta-mãe dá Modificar, herdável, a INTERATIVO,
// Usuários e Todos. Tudo o que for criado dentro herda.
func abrirHeranca(t *testing.T, mae string, manterProtecao bool) {
	t.Helper()
	var ea []windows.EXPLICIT_ACCESS
	for _, tipo := range []windows.WELL_KNOWN_SID_TYPE{windows.WinInteractiveSid, windows.WinBuiltinUsersSid, windows.WinWorldSid} {
		sid, err := windows.CreateWellKnownSid(tipo)
		if err != nil {
			t.Fatal(err)
		}
		ea = append(ea, windows.EXPLICIT_ACCESS{
			AccessPermissions: 0x1301BF, // Modificar (M)
			AccessMode:        windows.GRANT_ACCESS,
			Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
			Trustee:           windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_WELL_KNOWN_GROUP, TrusteeValue: windows.TrusteeValueFromSID(sid)},
		})
	}
	sd, err := windows.GetNamedSecurityInfo(mae, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	atual, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	acl, err := windows.ACLFromEntries(ea, atual)
	if err != nil {
		t.Fatal(err)
	}
	info := windows.SECURITY_INFORMATION(windows.DACL_SECURITY_INFORMATION)
	if manterProtecao {
		info |= windows.PROTECTED_DACL_SECURITY_INFORMATION
	}
	if err := windows.SetNamedSecurityInfo(mae, windows.SE_FILE_OBJECT, info, nil, nil, acl, nil); err != nil {
		t.Fatal(err)
	}
}

// conferirSoDoUsuario: só usuário, SISTEMA e Administradores, com acesso total para o usuário.
func conferirSoDoUsuario(t *testing.T, caminho string, raiz bool) {
	t.Helper()
	protegida, entradas, err := LerDACL(caminho)
	if err != nil {
		t.Fatal(err)
	}
	if raiz && !protegida {
		t.Errorf("%s: DACL não está protegida (ainda herda da pasta-mãe)", filepath.Base(caminho))
	}
	u, err := sidDoUsuario()
	if err != nil {
		t.Fatal(err)
	}
	permitidos := map[string]bool{u.String(): true, "S-1-5-18": true, "S-1-5-32-544": true}
	usuarioTotal := false
	for _, e := range entradas {
		if nome, ruim := sidsProibidos[e.SID]; ruim {
			t.Errorf("%s: %s tem acesso", filepath.Base(caminho), nome)
		}
		if !permitidos[e.SID] {
			t.Errorf("%s: SID inesperado %s", filepath.Base(caminho), e.SID)
		}
		if e.SID == u.String() && e.Permitir && e.Mascara == acessoTotal {
			usuarioTotal = true
		}
	}
	if !usuarioTotal {
		t.Errorf("%s: o usuário não tem acesso total: %+v", filepath.Base(caminho), entradas)
	}
}

func temSID(t *testing.T, caminho, sid string) bool {
	t.Helper()
	_, entradas, err := LerDACL(caminho)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entradas {
		if e.SID == sid {
			return true
		}
	}
	return false
}

func TestProtegerPastaComHerancaAberta(t *testing.T) {
	mae := t.TempDir()
	abrirHeranca(t, mae, false)
	// Perfil que já existia antes da correção: pastas e arquivo criados com a herança aberta.
	pasta := filepath.Join(mae, "dados")
	pref := filepath.Join(pasta, "navegador", "Default", "Preferences")
	if err := os.MkdirAll(filepath.Dir(pref), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pref, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !temSID(t, pref, "S-1-5-4") {
		t.Fatal("o teste supõe que o arquivo herdou INTERATIVO da pasta-mãe")
	}

	if err := ProtegerPasta(pasta); err != nil {
		t.Fatal(err)
	}
	conferirSoDoUsuario(t, pasta, true)
	conferirSoDoUsuario(t, filepath.Join(pasta, "navegador"), false)
	conferirSoDoUsuario(t, pref, false)

	// Arquivo novo, criado depois, herda a DACL protegida.
	novo := filepath.Join(pasta, "navegador", "Cookies")
	if err := os.WriteFile(novo, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	conferirSoDoUsuario(t, novo, false)

	// De novo (a cada partida): continua igual.
	if err := ProtegerPasta(pasta); err != nil {
		t.Fatal(err)
	}
	conferirSoDoUsuario(t, pasta, true)
}

// Se alguém abriu a pasta depois (icacls /grant Todos), a próxima partida refaz: não basta estar protegida.
func TestProtegerPastaRefazSeAlguemAbriu(t *testing.T) {
	pasta := filepath.Join(t.TempDir(), "dados")
	if err := os.MkdirAll(pasta, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := ProtegerPasta(pasta); err != nil {
		t.Fatal(err)
	}
	abrirHeranca(t, pasta, true) // acrescenta Todos/Usuários/INTERATIVO e mantém a proteção
	if protegida, _, _ := LerDACL(pasta); !protegida || !temSID(t, pasta, "S-1-1-0") {
		t.Fatal("o teste supõe que Todos entrou na DACL e que ela continua protegida")
	}
	if err := ProtegerPasta(pasta); err != nil {
		t.Fatal(err)
	}
	conferirSoDoUsuario(t, pasta, true)
}

// PastaDados protege a CENTRAL_PASTA nova e a que já existia, fora do perfil do usuário.
func TestPastaDadosProtegeAPasta(t *testing.T) {
	mae := t.TempDir()
	abrirHeranca(t, mae, false)
	for _, nome := range []string{"nova", "existente"} {
		p := filepath.Join(mae, nome)
		if nome == "existente" {
			if err := os.MkdirAll(filepath.Join(p, "curriculos"), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		t.Setenv("CENTRAL_PASTA", p)
		got, err := PastaDados()
		if err != nil || got != p {
			t.Fatalf("%s: PastaDados = %q, %v", nome, got, err)
		}
		conferirSoDoUsuario(t, p, true)
		if nome == "existente" {
			conferirSoDoUsuario(t, filepath.Join(p, "curriculos"), false)
		}
	}
}
