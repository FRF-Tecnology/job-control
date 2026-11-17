package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"os"
)

// chaves gera o par Ed25519 da licença: imprime a privada (APPLYER_CHAVE_PRIVADA=<base64>) na saída, para
// ir direto ao cofre de segredos do servidor, e grava a pública (base64) em -pub, para embutir no app.
// A privada nunca vai para arquivo. Não sobrescreve uma pública que já existe: trocar o par invalida as
// licenças de todos os apps já instalados.
func chaves(args []string, saida io.Writer) error {
	fs := flag.NewFlagSet("chaves", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	pub := fs.String("pub", "licenca.pub", "arquivo onde gravar a chave pública")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("uso: nuvem chaves [-pub caminho]: %w", err)
	}
	publica, privada, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	// Grava a pública antes de imprimir a privada: se o arquivo não puder ser criado, nada vaza à toa.
	f, err := os.OpenFile(*pub, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("não consegui criar %s (apague o arquivo antes, se for trocar o par): %w", *pub, err)
	}
	if _, err := fmt.Fprintln(f, base64.StdEncoding.EncodeToString(publica)); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	_, err = fmt.Fprintf(saida, "APPLYER_CHAVE_PRIVADA=%s\n", base64.StdEncoding.EncodeToString(privada))
	return err
}
