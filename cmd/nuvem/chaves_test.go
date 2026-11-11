package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/licenca"
	"github.com/FRF-Tecnology/job-control/internal/nuvem"
)

func TestSubcomandoChaves(t *testing.T) {
	pub := filepath.Join(t.TempDir(), "licenca.pub")
	var saida bytes.Buffer
	if err := chaves([]string{"-pub", pub}, &saida); err != nil {
		t.Fatal(err)
	}
	linha := strings.TrimSpace(saida.String())
	valor, ok := strings.CutPrefix(linha, "APPLYER_CHAVE_PRIVADA=")
	if !ok || strings.Contains(linha, "\n") {
		t.Fatalf("saída esperada em uma linha APPLYER_CHAVE_PRIVADA=<base64>, veio %q", linha)
	}
	// A mesma leitura que o servidor faz da variável de ambiente.
	priv, err := nuvem.Config{ChavePrivada: valor}.Chave()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(pub)
	if err != nil {
		t.Fatal(err)
	}
	pubBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(b)))
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		t.Fatalf("pública gravada inválida: %v", err)
	}
	agora := time.Now()
	doc, err := licenca.Assinar(priv, licenca.Licenca{Usuario: "u", Estado: licenca.Teste, EmitidaEm: agora, ValidaAte: agora.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := licenca.Conferir(ed25519.PublicKey(pubBytes), doc, agora); err != nil {
		t.Fatalf("a pública gravada não confere com a privada impressa: %v", err)
	}
	// Não sobrescreve uma pública que já existe.
	if err := chaves([]string{"-pub", pub}, &bytes.Buffer{}); err == nil {
		t.Fatal("esperava recusar sobrescrever a pública existente")
	}
}

func TestSubcomandoChavesPadrao(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := chaves(nil, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("licenca.pub"); err != nil {
		t.Fatalf("o padrão de -pub é licenca.pub: %v", err)
	}
}
