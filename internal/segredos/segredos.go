// Package segredos guarda as chaves de API da pessoa no cofre do sistema
// (Gerenciador de Credenciais no Windows, Keychain no Mac, Secret Service no Linux).
// A chave nunca vai para o banco, para arquivo nem volta inteira para a tela.
package segredos

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const servico = "Central de Candidaturas"

// Cofre lê e grava chaves por provedor.
type Cofre interface {
	Ler(provedor string) (string, error)
	Gravar(provedor, chave string) error
	Apagar(provedor string) error
}

// Sistema é o cofre do sistema operacional.
type Sistema struct{}

func (Sistema) Ler(p string) (string, error) {
	c, err := keyring.Get(servico, p)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return c, err
}

func (Sistema) Gravar(p, chave string) error { return keyring.Set(servico, p, chave) }

func (Sistema) Apagar(p string) error {
	if err := keyring.Delete(servico, p); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	return nil
}

// Memoria é um cofre só para testes.
type Memoria map[string]string

func (m Memoria) Ler(p string) (string, error) { return m[p], nil }
func (m Memoria) Gravar(p, chave string) error { m[p] = chave; return nil }
func (m Memoria) Apagar(p string) error        { delete(m, p); return nil }

// Final mostra só os 4 últimos caracteres (para a pessoa reconhecer qual chave está salva).
func Final(chave string) string {
	r := []rune(chave)
	if len(r) <= 4 {
		return "…"
	}
	return "…" + string(r[len(r)-4:])
}
