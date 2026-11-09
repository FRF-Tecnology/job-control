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

// Sistema é o cofre do sistema operacional. Servico vazio é o nome de sempre; só os testes usam outro, nunca o
// real (jc-a2y.63).
type Sistema struct{ Servico string }

func (s Sistema) nome() string {
	if s.Servico != "" {
		return s.Servico
	}
	return servico
}

// ErroCofre: falha do cofre com texto fixo. O texto do sistema não aparece (poderia trazer o que foi gravado); o
// erro original fica no Unwrap, para o log registrar só o tipo (jc-a2y.63).
type ErroCofre struct {
	Acao string // ler, gravar ou apagar
	Err  error
}

func (e *ErroCofre) Error() string { return "cofre do sistema: não deu para " + e.Acao + " a chave" }
func (e *ErroCofre) Unwrap() error { return e.Err }

// Embrulhar põe um erro de cofre no texto fixo (o que já é ErroCofre fica como está).
func Embrulhar(acao string, err error) error {
	var ec *ErroCofre
	if err == nil || errors.As(err, &ec) {
		return err
	}
	return &ErroCofre{Acao: acao, Err: err}
}

func (s Sistema) Ler(p string) (string, error) {
	c, err := keyring.Get(s.nome(), p)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", Embrulhar("ler", err)
	}
	return c, nil
}

func (s Sistema) Gravar(p, chave string) error {
	return Embrulhar("gravar", keyring.Set(s.nome(), p, chave))
}

func (s Sistema) Apagar(p string) error {
	if err := keyring.Delete(s.nome(), p); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return Embrulhar("apagar", err)
	}
	return nil
}

// Seguro embrulha qualquer cofre: os erros dele saem com texto fixo (ErroCofre). A API usa este, para um cofre
// com defeito não vazar o que gravou por uma mensagem de erro.
type Seguro struct{ Cofre Cofre }

func (s Seguro) Ler(p string) (string, error) {
	c, err := s.Cofre.Ler(p)
	if err != nil {
		return "", Embrulhar("ler", err)
	}
	return c, nil
}
func (s Seguro) Gravar(p, chave string) error { return Embrulhar("gravar", s.Cofre.Gravar(p, chave)) }
func (s Seguro) Apagar(p string) error        { return Embrulhar("apagar", s.Cofre.Apagar(p)) }

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
