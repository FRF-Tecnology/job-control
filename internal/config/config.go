// Package config resolve onde o app guarda os dados e outras opções vindas do ambiente.
package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// NomePasta é a pasta do app dentro da pasta de configuração do usuário. NomePastaAntiga é a de antes do Applyer
// (jc-etv.7): ao abrir, ela é renomeada para NomePasta, inteira e de uma vez.
const (
	NomePasta       = "Applyer"
	NomePastaAntiga = "Central de Candidaturas"
)

// Avisos da abertura da pasta, para a tela (sem caminho).
const (
	AvisoAntigaFicou    = "pasta-antiga-ficou" // as duas existem: usa a nova, e a antiga continua lá para a pessoa decidir
	AvisoMigracaoAdiada = "migracao-adiada"    // o rename falhou (arquivo em uso): usa a antiga nesta vez e tenta na próxima
)

// ErrDuasPastas: APPLYER_PASTA e CENTRAL_PASTA apontam para pastas diferentes. Falha fechada: abrir a pasta errada
// seria pior que não abrir.
var ErrDuasPastas = errors.New("defina só APPLYER_PASTA (APPLYER_PASTA e CENTRAL_PASTA apontam para pastas diferentes)")

// renomear é o os.Rename; só os testes trocam, para simular a pasta em uso.
var renomear = os.Rename

// Abertura: a pasta de dados aberta e o que aconteceu, sem caminho fora de Caminho.
type Abertura struct {
	Caminho string
	Migrou  bool   // a pasta antiga acabou de virar a nova
	Aviso   string // AvisoAntigaFicou, AvisoMigracaoAdiada ou vazio
}

// PastaDados devolve a pasta dos dados locais (banco, currículos, backups). Ver AbrirPastaDados.
func PastaDados() (string, error) {
	a, err := AbrirPastaDados()
	return a.Caminho, err
}

// AbrirPastaDados resolve e cria a pasta de dados.
//   - APPLYER_PASTA troca o local (testes, uma segunda conta, simulação de usuário novo). CENTRAL_PASTA ainda vale,
//     por duas versões; as duas definidas e diferentes: ErrDuasPastas.
//   - Sem variável, a pasta padrão: Applyer. Se só a antiga existe, ela é renomeada (os.Rename, na mesma pasta-mãe:
//     no mesmo disco e de uma vez; nunca copia e apaga). Se o rename falha, usa a antiga nesta vez. Se as duas
//     existem, usa a nova e não junta nada.
//
// A pasta fica só para o usuário (ProtegerPasta), também quando já existia ou acabou de ser renomeada: guarda o
// banco, os currículos e o perfil do navegador (jc-a2y.58).
func AbrirPastaDados() (Abertura, error) {
	p, _, err := pastaDoAmbiente()
	if err != nil {
		return Abertura{}, err
	}
	if p != "" {
		return Abertura{Caminho: p}, criarProtegida(p)
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return Abertura{}, err
	}
	nova, antiga := filepath.Join(base, NomePasta), filepath.Join(base, NomePastaAntiga)
	a := Abertura{Caminho: nova}
	switch {
	case existeAlgo(nova):
		if ehPastaDeVerdade(antiga) {
			a.Aviso = AvisoAntigaFicou
		}
	case ehPastaDeVerdade(antiga):
		if err := renomear(antiga, nova); err != nil {
			a.Caminho, a.Aviso = antiga, AvisoMigracaoAdiada
		} else {
			a.Migrou = true
		}
	}
	return a, criarProtegida(a.Caminho)
}

// pastaDoAmbiente: a pasta das variáveis e a origem para o log (sem caminho). Nenhuma definida: "".
func pastaDoAmbiente() (pasta, origem string, err error) {
	nova, antiga := os.Getenv("APPLYER_PASTA"), os.Getenv("CENTRAL_PASTA")
	switch {
	case nova != "" && antiga != "" && !MesmaPasta(nova, antiga):
		return "", "pasta: APPLYER_PASTA e CENTRAL_PASTA diferentes", ErrDuasPastas
	case nova != "":
		return nova, "pasta: APPLYER_PASTA", nil
	case antiga != "":
		return antiga, "pasta: CENTRAL_PASTA (obsoleta)", nil
	}
	return "", "pasta: padrão", nil
}

// existeAlgo: há qualquer coisa no caminho (pasta, arquivo ou atalho).
func existeAlgo(p string) bool { _, err := os.Lstat(p); return err == nil }

// ehPastaDeVerdade: uma pasta, e não um arquivo nem um atalho (um atalho nunca é renomeado sozinho).
func ehPastaDeVerdade(p string) bool {
	fi, err := os.Lstat(p)
	return err == nil && fi.IsDir() && fi.Mode()&os.ModeSymlink == 0
}

// PastaPadrao é a pasta de dados de sempre (sem variável): a nova, Applyer. Não cria nada.
func PastaPadrao() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, NomePasta), nil
}

// EhPastaPadrao: o caminho é a pasta padrão nova ou a antiga (antes do rename, ou com o rename adiado). O cofre usa
// isto para dar o mesmo nome às chaves nos dois casos.
func EhPastaPadrao(p string) bool {
	base, err := os.UserConfigDir()
	if err != nil {
		return false
	}
	return MesmaPasta(p, filepath.Join(base, NomePasta)) || MesmaPasta(p, filepath.Join(base, NomePastaAntiga))
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
	_, origem, _ := pastaDoAmbiente()
	return origem
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
	// Do mais específico para o mais geral: as variáveis e a pasta padrão costumam ficar dentro da do usuário.
	trocar(os.Getenv("APPLYER_PASTA"), "<APPLYER_PASTA>")
	trocar(os.Getenv("CENTRAL_PASTA"), "<CENTRAL_PASTA>")
	if base, err := os.UserConfigDir(); err == nil {
		trocar(filepath.Join(base, NomePasta), "<pasta padrão>")
		trocar(filepath.Join(base, NomePastaAntiga), "<pasta padrão>")
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
