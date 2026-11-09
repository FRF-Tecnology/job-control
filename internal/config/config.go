// Package config resolve onde o app guarda os dados e outras opções vindas do ambiente.
package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	p, err := PastaPadrao()
	if err != nil {
		return "", err
	}
	return p, criarProtegida(p)
}

// PastaPadrao é a pasta de dados de sempre (sem CENTRAL_PASTA). Não cria nada.
func PastaPadrao() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, NomePasta), nil
}

// MesmaPasta diz se dois caminhos são a mesma pasta (limpos e absolutos; no Windows, sem diferenciar maiúsculas).
func MesmaPasta(a, b string) bool {
	limpar := func(p string) string {
		p = filepath.Clean(p)
		if abs, err := filepath.Abs(p); err == nil {
			p = abs
		}
		return p
	}
	a, b = limpar(a), limpar(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func criarProtegida(p string) error {
	if err := os.MkdirAll(p, 0o700); err != nil {
		return err
	}
	return ProtegerPasta(p)
}

// OrigemPasta diz de onde veio a pasta de dados, sem o caminho (que traz o nome do usuário): é o que os logs e
// as mensagens de partida mostram.
func OrigemPasta() string {
	if os.Getenv("CENTRAL_PASTA") != "" {
		return "pasta: CENTRAL_PASTA"
	}
	return "pasta: padrão"
}

// OcultarCaminhos troca, num texto de erro ou de log, a pasta de dados e a pasta do usuário por marcadores
// (<CENTRAL_PASTA>, <pasta padrão>, ~), para a saída de partida não mostrar o nome do usuário. Troca também a
// forma curta do Windows (C:\Users\NOME~1). No Windows, sem diferenciar maiúsculas e com \ ou / (substituir, em
// curto_windows.go). Caminhos relativos ficam como estão.
func OcultarCaminhos(texto string) string {
	var pares [][2]string // forma do caminho e marcador, do mais específico para o mais geral
	trocar := func(caminho, marcador string) {
		if caminho == "" || !filepath.IsAbs(caminho) {
			return
		}
		caminho = filepath.Clean(caminho)
		if len(caminho) <= len(filepath.VolumeName(caminho))+1 {
			return // a raiz do disco (C:\ ou /) não traz nome e trocaria demais
		}
		formas := []string{caminho}
		if c := formaCurta(caminho); c != caminho {
			formas = append(formas, c)
		}
		for _, f := range formas {
			pares = append(pares, [2]string{f, marcador})
		}
	}
	// Do mais específico para o mais geral: a CENTRAL_PASTA e a pasta padrão costumam ficar dentro da do usuário.
	trocar(os.Getenv("CENTRAL_PASTA"), "<CENTRAL_PASTA>")
	if base, err := os.UserConfigDir(); err == nil {
		trocar(filepath.Join(base, NomePasta), "<pasta padrão>")
	}
	if casa, err := os.UserHomeDir(); err == nil {
		trocar(casa, "~")
	}
	if len(pares) == 0 {
		return texto
	}
	return substituir(texto, pares)
}

// CaminhoBanco é o arquivo SQLite dentro da pasta de dados.
func CaminhoBanco(pasta string) string { return filepath.Join(pasta, "central.db") }

// PastaNavegador é o perfil próprio do navegador do app dentro da pasta de dados. Fica aqui (e não só em
// internal/navegador) para a API apagar o perfil sem trazer o navegador para o binário padrão.
func PastaNavegador(pasta string) string { return filepath.Join(pasta, "navegador") }
