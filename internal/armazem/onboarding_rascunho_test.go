package armazem

import (
	"bytes"
	"context"
	"encoding/base64"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/curriculos"
)

func TestRascunhoRollbackRecuperacaoEBytesForaDoSQLite(t *testing.T) {
	ctx := context.Background()
	pasta := t.TempDir()
	a, err := Abrir(ctx, filepath.Join(pasta, "central.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Fechar()
	if _, err := a.PrepararPrimeiraVez(ctx, []byte("[]")); err != nil {
		t.Fatal(err)
	}
	pdf := []byte("%PDF-1.4\nIDENTIFICADOR-SOMENTE-DENTRO-DO-PDF-DE-TESTE\n%%EOF")
	d, err := a.TrocarPDFRascunho(ctx, 0, "Curriculo.pdf", pdf)
	if err != nil {
		t.Fatal(err)
	}
	var raw string
	if err = a.db.QueryRowContext(ctx, "SELECT valor FROM meta WHERE chave=?", chaveRascunho).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "IDENTIFICADOR-SOMENTE") || strings.Contains(raw, base64.StdEncoding.EncodeToString(pdf)) {
		t.Fatal("conteúdo do documento no SQLite")
	}
	if _, err = a.db.Exec("CREATE TRIGGER rejeitarRascunho BEFORE UPDATE ON meta WHEN OLD.chave='rascunhoPrimeiroAcesso' BEGIN SELECT RAISE(ABORT,'falha ao gravar'); END"); err != nil {
		t.Fatal(err)
	}
	if _, err = a.TrocarPDFRascunho(ctx, 1, "Outro.pdf", []byte("%PDF-1.4\nOutro\n%%EOF")); err == nil {
		t.Fatal("esperava rollback")
	}
	ref := ReferenciaRascunho{ArquivoID: d.Curriculo.ArquivoID, Revisao: 1}
	original, err := a.LerPDFRascunho(ctx, ref)
	if err != nil || !bytes.Equal(original, pdf) {
		t.Fatal("perdeu PDF anterior", err)
	}
	raiz, err := abrirPastaRascunho(pasta, false)
	if err != nil {
		t.Fatal(err)
	}
	defer raiz.Close()
	if itens, err := fs.ReadDir(raiz.FS(), "."); err != nil || len(itens) != 1 {
		t.Fatalf("sobrou arquivo após rollback: %v %v", itens, err)
	}
	// A cópia terminou, mas o processo foi encerrado antes do COMMIT que a vincularia.
	if _, _, err = curriculos.Importar(raiz, []byte("interrompida")); err != nil {
		t.Fatal(err)
	}
	if _, err = a.PrepararPrimeiraVez(ctx, []byte("[]")); err != nil {
		t.Fatal(err)
	}
	if itens, err := fs.ReadDir(raiz.FS(), "."); err != nil || len(itens) != 1 || itens[0].Name() != d.Curriculo.nomeLocal() {
		t.Fatalf("recuperação: %v %v", itens, err)
	}
	if _, err = a.db.Exec("DROP TRIGGER rejeitarRascunho"); err != nil {
		t.Fatal(err)
	}
	if _, err = a.TrocarPDFRascunho(ctx, 1, "", nil); err != nil {
		t.Fatal(err)
	}
	itens, err := os.ReadDir(filepath.Join(pasta, "curriculos", ".rascunho-onboarding"))
	if err != nil || len(itens) != 0 {
		t.Fatalf("exclusão: %v %v", itens, err)
	}
}
