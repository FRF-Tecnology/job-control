package conta

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/licenca"
)

var emitidaTeste = time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)

func docDeTeste(t *testing.T, estado licenca.Estado) (licenca.Documento, ed25519.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	d, err := licenca.Assinar(priv, licenca.Licenca{Usuario: "u1", Email: "ana@exemplo.com", Estado: estado,
		EmitidaEm: emitidaTeste, ValidaAte: emitidaTeste.Add(30 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	return d, pub
}

func TestGravarELerLicenca(t *testing.T) {
	pasta := t.TempDir()
	d, pub := docDeTeste(t, licenca.Teste)
	if err := GravarLicenca(pasta, d); err != nil {
		t.Fatal(err)
	}
	l, err := LerLicencaDoDisco(pasta, pub, emitidaTeste.Add(10*time.Minute))
	if err != nil || l.Estado != licenca.Teste || l.Email != "ana@exemplo.com" {
		t.Fatalf("leitura: %+v, %v", l, err)
	}
	if _, err := LerLicencaDoDisco(pasta, pub, emitidaTeste.Add(30*time.Minute+time.Second)); !errors.Is(err, licenca.ErrVencida) {
		t.Fatalf("queria vencida, veio %v", err)
	}
}

func TestGravarLicencaAtomicaSemSobras(t *testing.T) {
	pasta := t.TempDir()
	d, _ := docDeTeste(t, licenca.Teste)
	for i := 0; i < 3; i++ {
		if err := GravarLicenca(pasta, d); err != nil {
			t.Fatal(err)
		}
	}
	itens, err := os.ReadDir(pasta)
	if err != nil {
		t.Fatal(err)
	}
	if len(itens) != 1 || itens[0].Name() != arquivoLicencaNome {
		t.Fatalf("sobrou arquivo temporário: %v", itens)
	}
	if runtime.GOOS != "windows" { // no Windows as permissões Unix não valem
		info, err := os.Stat(filepath.Join(pasta, arquivoLicencaNome))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("permissão %v, queria 0600", info.Mode().Perm())
		}
	}
}

func TestLerLicencaDoDiscoUsaDesvioGuardado(t *testing.T) {
	pasta := t.TempDir()
	d, pub := docDeTeste(t, licenca.Assinante)
	// relógio local 3 h atrasado: desvio = emitida − local = +3 h
	if err := gravarLicenca(pasta, d, 3*time.Hour); err != nil {
		t.Fatal(err)
	}
	local := emitidaTeste.Add(-3 * time.Hour)
	if _, err := LerLicencaDoDisco(pasta, pub, local.Add(30*time.Minute)); err != nil {
		t.Fatalf("aos 30 min ainda vale: %v", err)
	}
	if _, err := LerLicencaDoDisco(pasta, pub, local.Add(30*time.Minute+time.Second)); !errors.Is(err, licenca.ErrVencida) {
		t.Fatalf("aos 30m1s pelo relógio local devia vencer, veio %v", err)
	}
}

func TestDesvioEditadoNoDiscoNaoEstica(t *testing.T) {
	pasta := t.TempDir()
	d, pub := docDeTeste(t, licenca.Assinante)
	// alguém troca o desvio para −72 h tentando esticar a licença depois de cortar a internet
	if err := gravarLicenca(pasta, d, -72*time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := LerLicencaDoDisco(pasta, pub, emitidaTeste.Add(40*time.Minute)); !errors.Is(err, licenca.ErrVencida) {
		t.Fatalf("desvio editado não pode esticar a validade, veio %v", err)
	}
	if limitarDesvio(-72*time.Hour) != -desvioMaximo || limitarDesvio(72*time.Hour) != desvioMaximo {
		t.Fatal("desvio deve ficar em ±24 h")
	}
}

func TestLicencaArquivoAusenteOuCorrompido(t *testing.T) {
	pasta := t.TempDir()
	_, pub := docDeTeste(t, licenca.Teste)
	if _, err := LerLicencaDoDisco(pasta, pub, emitidaTeste); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("sem arquivo: queria ErrNotExist, veio %v", err)
	}
	if err := os.WriteFile(filepath.Join(pasta, arquivoLicencaNome), []byte("{não é json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LerLicencaDoDisco(pasta, pub, emitidaTeste); !errors.Is(err, licenca.ErrAssinatura) {
		t.Fatalf("arquivo corrompido: queria ErrAssinatura, veio %v", err)
	}
}
