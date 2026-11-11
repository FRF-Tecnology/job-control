package nuvem

import (
	"strings"
	"testing"
)

func envDe(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestLerConfigFaltando(t *testing.T) {
	_, err := LerConfig(envDe(map[string]string{
		"APPLYER_CHAVE_PRIVADA": "x", "APPLYER_SEGREDO_CODIGOS": "y",
	}))
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("esperava erro citando DATABASE_URL, veio %v", err)
	}
}

func TestLerConfigNaoVazaValor(t *testing.T) {
	_, err := LerConfig(envDe(map[string]string{"APPLYER_SEGREDO_CODIGOS": "segredo-fictício-123"}))
	if err == nil || strings.Contains(err.Error(), "segredo-fictício-123") {
		t.Fatalf("erro nulo ou vazou valor: %v", err)
	}
}

func TestLerConfigPadroes(t *testing.T) {
	cfg, err := LerConfig(envDe(map[string]string{
		"DATABASE_URL": "postgres://x", "APPLYER_CHAVE_PRIVADA": "k", "APPLYER_SEGREDO_CODIGOS": "s",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Endereco != ":8080" {
		t.Errorf("endereço padrão = %q", cfg.Endereco)
	}
	if cfg.DatabaseURL != "postgres://x" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
}
