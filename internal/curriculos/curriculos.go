// Package curriculos lê um PDF da pasta de currículos com segurança. A API usa para calcular o sha256 ao criar a
// candidatura, e o executor usa para conferir de novo antes de anexar (M2).
package curriculos

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// MaxBytes: currículo maior que 20 MB não é PDF de currículo.
const MaxBytes = int64(20 << 20)

var (
	ErrFora   = errors.New("o currículo não está na pasta de currículos")
	ErrGrande = errors.New("o currículo tem mais de 20 MB")
)

// entreLstatEOpen só existe para o teste trocar o arquivo nessa janela.
var entreLstatEOpen = func() {}

// Ler devolve o conteúdo do PDF nome dentro de pasta. Só arquivo comum, bem dentro da pasta (sem barra, drive, ..
// nem :), sem atalho nem link simbólico, de até 20 MB. Depois de abrir, confere que o arquivo aberto é o mesmo que
// foi conferido (os.SameFile) e que continua comum: troca do arquivo entre a conferência e a abertura é recusada.
func Ler(pasta, nome string) ([]byte, error) {
	if pasta == "" || !dominio.NomeDeCurriculoValido(nome) {
		return nil, ErrFora
	}
	caminho := filepath.Join(pasta, nome)
	if filepath.Dir(caminho) != filepath.Clean(pasta) {
		return nil, ErrFora
	}
	antes, err := os.Lstat(caminho)
	if err != nil || !antes.Mode().IsRegular() {
		return nil, ErrFora
	}
	if antes.Size() > MaxBytes {
		return nil, ErrGrande
	}
	// No Windows, o FileInfo do Lstat só lê a identidade do arquivo (volume e índice) na primeira comparação, pelo
	// caminho. Comparar agora fixa a identidade do arquivo conferido, antes de qualquer troca.
	os.SameFile(antes, antes)
	entreLstatEOpen()
	f, err := os.Open(caminho)
	if err != nil {
		return nil, ErrFora
	}
	defer f.Close()
	depois, err := f.Stat()
	if err != nil || !depois.Mode().IsRegular() || !os.SameFile(antes, depois) {
		return nil, ErrFora
	}
	b, err := io.ReadAll(io.LimitReader(f, MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > MaxBytes {
		return nil, ErrGrande
	}
	return b, nil
}

// SHA256 em hexadecimal minúsculo, como fica em conteudo.curriculo.sha256.
func SHA256(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
