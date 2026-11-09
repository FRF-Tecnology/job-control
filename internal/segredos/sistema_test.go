//go:build windows || darwin

package segredos

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"
)

// jc-a2y.63: o cofre do sistema de verdade (Gerenciador de Credenciais no Windows, Keychain no macOS), cada um no
// seu sistema. Usa um serviço de TESTE com nome aleatório, nunca o real, e apaga o que gravou. Chaves fictícias.
func TestCofreDoSistema(t *testing.T) {
	// O cofre de verdade só no CI: neste PC, um erro no código poderia mexer nas chaves da pessoa (revisão do
	// CyberSec no #93, depois do incidente do #85). O CI exige que este teste passe, sem pular.
	if os.Getenv("CI") != "true" {
		t.Skip("cofre do sistema de verdade: só no CI")
	}
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	teste := "Applyer-teste-" + hex.EncodeToString(b)
	if teste == servico || strings.Contains(teste, "Central de Candidaturas") {
		t.Fatal("o teste nunca usa o serviço real")
	}
	s := Sistema{Servico: teste}
	// Antes de gravar qualquer coisa: o nome em uso tem de ser o de teste. Se o código ignorar o Servico, o teste para
	// aqui e não toca no serviço real (onde estão as chaves de verdade da pessoa).
	if s.nome() != teste || s.nome() == servico {
		t.Fatalf("o cofre de teste não usa o serviço de teste (%q)", s.nome())
	}
	// Provedor fictício: mesmo num erro, não troca a chave de um provedor de verdade.
	prov := "provedor-teste-" + hex.EncodeToString(b)
	// Apaga o provedor fictício nos dois serviços: se algum método ignorasse o Servico, o que ele gravou no real sai
	// também (revisão do Fallen no #85). O provedor é fictício, então nunca apaga uma chave de verdade.
	t.Cleanup(func() { keyring.Delete(teste, prov); keyring.Delete(servico, prov) })

	if c, err := s.Ler(prov); c != "" || err != nil {
		t.Fatalf("serviço novo: ler = %q, %v", c, err)
	}
	chave := "sk-ficticia-" + hex.EncodeToString(b) + "-WXYZ"
	if err := s.Gravar(prov, chave); err != nil {
		t.Fatalf("gravar: %v", err)
	}
	// Gravou no serviço de teste, não no real: lá o provedor fictício continua não existindo.
	if _, err := keyring.Get(servico, prov); !errors.Is(err, keyring.ErrNotFound) {
		t.Fatalf("o Gravar foi para o serviço real (%v)", err)
	}
	if _, err := keyring.Get(teste, prov); err != nil {
		t.Fatalf("o Gravar não foi para o serviço de teste (%v)", err)
	}
	if c, err := s.Ler(prov); c != chave || err != nil {
		t.Fatalf("ler = %q, %v", c, err)
	}
	// Gravar de novo troca a chave (não acumula).
	outra := chave + "-2"
	if err := s.Gravar(prov, outra); err != nil {
		t.Fatal(err)
	}
	if c, _ := s.Ler(prov); c != outra {
		t.Fatalf("depois de trocar = %q", c)
	}
	// O serviço real não foi tocado por este teste: o provedor fictício não existe lá.
	if c, _ := (Sistema{}).Ler("provedor-ficticio-" + hex.EncodeToString(b)); c != "" {
		t.Fatal("achou o provedor fictício no serviço real")
	}
	if err := s.Apagar(prov); err != nil {
		t.Fatal(err)
	}
	if c, err := s.Ler(prov); c != "" || err != nil {
		t.Fatalf("depois de apagar = %q, %v", c, err)
	}
	if err := s.Apagar(prov); err != nil { // apagar o que não existe não é erro
		t.Fatalf("apagar de novo = %v", err)
	}

	// Chave grande demais para o cofre: o erro tem texto fixo, sem a chave, e o motivo continua no Unwrap.
	grande := "sk-ficticia-MARCADOR-" + strings.Repeat("x", 6000)
	err := s.Gravar(prov, grande)
	if err == nil {
		s.Apagar(prov)
		t.Fatal("o cofre aceitou 6000 caracteres; o teste supõe o limite")
	}
	var ec *ErroCofre
	if !errors.As(err, &ec) || strings.Contains(err.Error(), "MARCADOR") || !errors.Is(err, keyring.ErrSetDataTooBig) {
		t.Fatalf("erro de chave grande = %v", err)
	}
}
