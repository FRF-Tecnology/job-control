package config

import (
	"errors"
	"os"
	"path/filepath"
)

// A pasta de dados do assistente (jc-a2y.7.4, A2 do CyberSec). O MCP e a CLI resolvem a pasta como o app, mas nunca
// criam, migram nem renomeiam nada, e só leem a proteção: quem cria, renomeia e protege é o app, ao abrir.

var (
	// ErrSemPastaDeDados: a pasta (ou o banco) ainda não existe. Sem caminho na mensagem.
	ErrSemPastaDeDados = errors.New("a pasta de dados do Applyer ainda não existe: abra o Applyer uma vez")
	// ErrPastaAntigaSemMigrar: só a pasta com o nome antigo existe; renomear é só do app.
	ErrPastaAntigaSemMigrar = errors.New("a pasta de dados ainda tem o nome antigo: abra o Applyer uma vez para ele renomear")
	// ErrPastaDesprotegida: a pasta não está só para o usuário (o app protege ao abrir).
	ErrPastaDesprotegida = errors.New("a pasta de dados não está protegida só para você: abra o Applyer uma vez")
)

// PastaDoAssistente devolve a pasta de dados para o MCP e a CLI: APPLYER_PASTA (ou CENTRAL_PASTA), ou a pasta padrão
// Applyer. Só a pasta antiga presente: erro, sem renomear. A pasta e o banco têm de existir; o dono tem de ser o
// usuário e a proteção tem de estar como o app deixa.
func PastaDoAssistente() (string, error) {
	p, _, err := pastaDoAmbiente()
	if err != nil {
		return "", err
	}
	if p == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		nova, antiga := filepath.Join(base, NomePasta), filepath.Join(base, NomePastaAntiga)
		switch {
		case ehPastaDeVerdade(nova):
			p = nova
		case ehPastaDeVerdade(antiga):
			return "", ErrPastaAntigaSemMigrar
		default:
			return "", ErrSemPastaDeDados
		}
	}
	if !ehPastaDeVerdade(p) {
		return "", ErrSemPastaDeDados
	}
	if fi, err := os.Lstat(CaminhoBanco(p)); err != nil || !fi.Mode().IsRegular() {
		return "", ErrSemPastaDeDados
	}
	if err := ConferirProtecao(p); err != nil {
		return "", err
	}
	return p, nil
}
