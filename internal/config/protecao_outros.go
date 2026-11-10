//go:build !windows

package config

import (
	"errors"
	"os"
	"syscall"
)

// ProtegerPasta deixa a pasta só para o usuário (0700), também quando ela já existia. No Windows, a versão de
// protecao_windows.go aplica uma DACL explícita e protegida (jc-a2y.58).
func ProtegerPasta(pasta string) error { return os.Chmod(pasta, 0o700) }

// ErrPastaDeOutroUsuario: a pasta de dados pertence a outro usuário deste computador. Sem caminho na mensagem.
var ErrPastaDeOutroUsuario = errors.New("a pasta de dados pertence a outro usuário deste computador: use outra pasta (APPLYER_PASTA) criada por você")

// ConferirProtecao só lê (jc-a2y.7.4, A2.2): o dono é o usuário e ninguém mais lê, grava nem entra (sem bits de
// grupo e de outros). Nunca muda nada: proteger é do app, ao abrir.
func ConferirProtecao(pasta string) error {
	fi, err := os.Stat(pasta)
	if err != nil {
		return err
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok && int(st.Uid) != os.Getuid() {
		return ErrPastaDeOutroUsuario
	}
	if fi.Mode().Perm()&0o077 != 0 {
		return ErrPastaDesprotegida
	}
	return nil
}
