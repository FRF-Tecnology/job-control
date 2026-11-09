package curriculos

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

var pdf = []byte("%PDF-1.4\n% curriculo ficticio de teste\n")

func pasta(t *testing.T) (string, string) {
	t.Helper()
	raiz := t.TempDir()
	p := filepath.Join(raiz, "curriculos")
	if err := os.Mkdir(p, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "cv.pdf"), pdf, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(raiz, "fora.pdf"), []byte("%PDF fora"), 0o600); err != nil {
		t.Fatal(err)
	}
	return raiz, p
}

func TestLerCurriculo(t *testing.T) {
	raiz, p := pasta(t)
	b, err := Ler(p, "cv.pdf")
	if err != nil || !bytes.Equal(b, pdf) {
		t.Fatalf("Ler = %q, %v", b, err)
	}
	if soma := sha256.Sum256(pdf); SHA256(b) != hex.EncodeToString(soma[:]) {
		t.Fatalf("sha256 = %s", SHA256(b))
	}
	os.Mkdir(filepath.Join(p, "pasta.pdf"), 0o700)
	grande, _ := os.Create(filepath.Join(p, "grande.pdf"))
	grande.Truncate(MaxBytes + 1)
	grande.Close()
	casos := map[string]error{
		"nao-existe.pdf": ErrFora, "../fora.pdf": ErrFora, `..\fora.pdf`: ErrFora, "pasta.pdf": ErrFora,
		"C:fora.pdf": ErrFora, "cv.pdf:fluxo": ErrFora, "cv.PDF.exe": ErrFora, "": ErrFora, "grande.pdf": ErrGrande,
	}
	for nome, quer := range casos {
		if _, err := Ler(p, nome); !errors.Is(err, quer) {
			t.Errorf("Ler(%q) = %v; queria %v", nome, err, quer)
		}
	}
	if _, err := Ler("", "cv.pdf"); !errors.Is(err, ErrFora) {
		t.Errorf("sem pasta = %v", err)
	}
	if err := os.Symlink(filepath.Join(raiz, "fora.pdf"), filepath.Join(p, "atalho.pdf")); err == nil {
		if _, err := Ler(p, "atalho.pdf"); !errors.Is(err, ErrFora) {
			t.Errorf("link simbólico = %v", err)
		}
	} else {
		t.Logf("sem link simbólico nesta máquina (%v); o caso roda no CI", err)
	}
}

// Troca entre a conferência (Lstat) e a abertura: o arquivo aberto não é o conferido, então é recusado.
func TestLerRecusaTrocaDepoisDaConferencia(t *testing.T) {
	raiz, p := pasta(t)
	defer func() { entreLstatEOpen = func() {} }()

	// Outro arquivo comum no lugar (renomeado por cima).
	entreLstatEOpen = func() {
		os.WriteFile(filepath.Join(p, "novo.tmp"), []byte("%PDF trocado"), 0o600)
		if err := os.Rename(filepath.Join(p, "novo.tmp"), filepath.Join(p, "cv.pdf")); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Ler(p, "cv.pdf"); !errors.Is(err, ErrFora) {
		t.Fatalf("arquivo trocado = %v; queria recusa", err)
	}

	// Link simbólico no lugar (onde a máquina permite criar).
	os.WriteFile(filepath.Join(p, "cv.pdf"), pdf, 0o600)
	entreLstatEOpen = func() {
		os.Remove(filepath.Join(p, "cv.pdf"))
		if err := os.Symlink(filepath.Join(raiz, "fora.pdf"), filepath.Join(p, "cv.pdf")); err != nil {
			t.Skipf("sem link simbólico nesta máquina (%v)", err)
		}
	}
	if _, err := Ler(p, "cv.pdf"); !errors.Is(err, ErrFora) {
		t.Fatalf("link no lugar = %v; queria recusa", err)
	}
}
