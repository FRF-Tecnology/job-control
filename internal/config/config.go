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
// A pasta fica só para o usuário (ProtegerPasta), também quando já existia: guarda o banco, os currículos e o
// perfil do navegador (jc-a2y.58).
func PastaDados() (string, error) {
	if p := os.Getenv("CENTRAL_PASTA"); p != "" {
		return p, criarProtegida(p)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(base, NomePasta)
	return p, criarProtegida(p)
}

func criarProtegida(p string) error {
	if err := os.MkdirAll(p, 0o700); err != nil {
		return err
	}
	return ProtegerPasta(p)
}

// CaminhoBanco é o arquivo SQLite dentro da pasta de dados.
func CaminhoBanco(pasta string) string { return filepath.Join(pasta, "central.db") }

// PastaNavegador é o perfil próprio do navegador do app dentro da pasta de dados. Fica aqui (e não só em
// internal/navegador) para a API apagar o perfil sem trazer o navegador para o binário padrão.
func PastaNavegador(pasta string) string { return filepath.Join(pasta, "navegador") }
