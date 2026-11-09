//go:build windows

package config

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// acessoTotal é FILE_ALL_ACCESS, o (F) do icacls.
const acessoTotal = 0x1F01FF

// EntradaDACL é uma entrada da DACL de um arquivo ou pasta, para conferir quem tem acesso.
type EntradaDACL struct {
	SID        string
	Permitir   bool   // ACCESS_ALLOWED (false: negar ou outro tipo)
	Mascara    uint32 // direitos
	Herdada    bool   // veio da pasta-mãe
	ParaFilhos bool   // vale para subpastas e arquivos (OI)(CI)
}

// ProtegerPasta deixa a pasta só para o usuário atual, SISTEMA e Administradores (jc-a2y.58). No Windows, o 0700
// do MkdirAll não muda a ACL: a pasta herda da mãe, e fora do perfil (C:\Users\Public, outro disco) isso dá acesso
// a INTERATIVO, Usuários ou Todos, que leriam o banco e o cookie de sessão do perfil do navegador. Aqui a DACL fica
// explícita e protegida (sem herança), com (OI)(CI), e o Windows a propaga para o que já existe dentro. Roda a
// cada partida: se já está assim, não mexe (não percorre o perfil do navegador toda vez).
func ProtegerPasta(pasta string) error {
	sids, err := donos()
	if err != nil {
		return fmt.Errorf("proteger a pasta: %w", err)
	}
	if protegida, entradas, err := LerDACL(pasta); err == nil && protegida && soEstes(entradas, sids) {
		return nil
	}
	ea := make([]windows.EXPLICIT_ACCESS, 0, len(sids))
	for _, s := range sids {
		ea = append(ea, windows.EXPLICIT_ACCESS{
			AccessPermissions: acessoTotal,
			AccessMode:        windows.SET_ACCESS,
			Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
			Trustee:           windows.TRUSTEE{TrusteeForm: windows.TRUSTEE_IS_SID, TrusteeType: windows.TRUSTEE_IS_UNKNOWN, TrusteeValue: windows.TrusteeValueFromSID(s)},
		})
	}
	acl, err := windows.ACLFromEntries(ea, nil)
	if err != nil {
		return fmt.Errorf("proteger a pasta: %w", err)
	}
	// Sem caminho na mensagem (o caminho traz o nome do usuário): só o erro do Windows.
	if err := windows.SetNamedSecurityInfo(pasta, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
		return fmt.Errorf("proteger a pasta: %w", err)
	}
	return nil
}

// sidDoUsuario é o SID de quem roda o processo.
func sidDoUsuario() (*windows.SID, error) {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	return u.User.Sid.Copy()
}

// donos: usuário atual, SISTEMA e Administradores, sem repetir (um serviço roda como SISTEMA).
func donos() ([]*windows.SID, error) {
	u, err := sidDoUsuario()
	if err != nil {
		return nil, err
	}
	sids := []*windows.SID{u}
	for _, tipo := range []windows.WELL_KNOWN_SID_TYPE{windows.WinLocalSystemSid, windows.WinBuiltinAdministratorsSid} {
		s, err := windows.CreateWellKnownSid(tipo)
		if err != nil {
			return nil, err
		}
		if !s.Equals(u) {
			sids = append(sids, s)
		}
	}
	return sids, nil
}

// soEstes: a DACL tem exatamente uma entrada explícita de acesso total, para filhos, por SID pedido.
func soEstes(entradas []EntradaDACL, sids []*windows.SID) bool {
	if len(entradas) != len(sids) {
		return false
	}
	falta := map[string]bool{}
	for _, s := range sids {
		falta[s.String()] = true
	}
	for _, e := range entradas {
		if !e.Permitir || e.Herdada || !e.ParaFilhos || e.Mascara != acessoTotal || !falta[e.SID] {
			return false
		}
		delete(falta, e.SID)
	}
	return len(falta) == 0
}

// LerDACL devolve se a DACL está protegida (não herda) e as entradas dela. DACL nula (todos com acesso total)
// vira uma entrada de Todos.
func LerDACL(caminho string) (protegida bool, entradas []EntradaDACL, err error) {
	sd, err := windows.GetNamedSecurityInfo(caminho, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return false, nil, err
	}
	controle, _, err := sd.Control()
	if err != nil {
		return false, nil, err
	}
	protegida = controle&windows.SE_DACL_PROTECTED != 0
	dacl, _, err := sd.DACL()
	if err != nil {
		return false, nil, err
	}
	if dacl == nil {
		return protegida, []EntradaDACL{{SID: "S-1-1-0", Permitir: true, Mascara: acessoTotal, ParaFilhos: true}}, nil
	}
	for i := range dacl.AceCount {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, uint32(i), &ace); err != nil {
			return false, nil, err
		}
		e := EntradaDACL{
			Mascara:    uint32(ace.Mask),
			Herdada:    ace.Header.AceFlags&windows.INHERITED_ACE != 0,
			ParaFilhos: ace.Header.AceFlags&(windows.OBJECT_INHERIT_ACE|windows.CONTAINER_INHERIT_ACE) == windows.OBJECT_INHERIT_ACE|windows.CONTAINER_INHERIT_ACE,
		}
		// Permitir e negar (tipos 0 e 1) têm o SID logo depois da máscara; outros tipos não são lidos.
		if t := ace.Header.AceType; t == windows.ACCESS_ALLOWED_ACE_TYPE || t == windows.ACCESS_DENIED_ACE_TYPE {
			e.Permitir = t == windows.ACCESS_ALLOWED_ACE_TYPE
			// O SID começa em SidStart (layout de ACCESS_ALLOWED_ACE/ACCESS_DENIED_ACE do Windows); o ACE vem de GetAce,
			// dentro da DACL que o Windows devolveu, e só é lido. Mesmo padrão dos testes do x/sys/windows.
			e.SID = (*windows.SID)(unsafe.Pointer(&ace.SidStart)).String() // #nosec G103 -- leitura do SID dentro do ACE, layout fixo do Windows
		}
		entradas = append(entradas, e)
	}
	return protegida, entradas, nil
}
