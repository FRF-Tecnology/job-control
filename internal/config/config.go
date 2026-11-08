// Package config resolve onde o app guarda os dados e outras opções vindas do ambiente.
package config

import (
	"os"
	"path/filepath"
)

// NomePasta é a pasta do app dentro da pasta de configuração do usuário.
const NomePasta = "Central de Candidaturas"

// PastaDados devolve a pasta dos dados locais (banco, currículos, backups).
// CENTRAL_PASTA troca o local (testes, uma segunda conta, simulação de usuário novo).
func PastaDados() (string, error) {
	if p := os.Getenv("CENTRAL_PASTA"); p != "" {
		return p, os.MkdirAll(p, 0o700)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(base, NomePasta)
	return p, os.MkdirAll(p, 0o700)
}

// CaminhoBanco é o arquivo SQLite dentro da pasta de dados.
func CaminhoBanco(pasta string) string { return filepath.Join(pasta, "central.db") }
