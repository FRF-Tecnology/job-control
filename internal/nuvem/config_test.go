package nuvem

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"
)

func envDe(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// envCompleto traz todas as obrigatórias, com valores fictícios.
func envCompleto(t *testing.T) map[string]string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{
		"DATABASE_URL": "postgres://x", "APPLYER_CHAVE_PRIVADA": base64.StdEncoding.EncodeToString(priv),
		"APPLYER_SEGREDO_CODIGOS": "s", "APPLYER_TERMOS_VERSAO": "2026-11",
		"SMTP_HOST": "smtp.exemplo.com", "SMTP_PORTA": "587", "SMTP_USUARIO": "u",
		"SMTP_SENHA": "senha-ficticia", "SMTP_REMETENTE": "app@exemplo.com",
		"GOOGLE_CLIENT_ID": "cliente-ficticio.apps.googleusercontent.com", "GOOGLE_CLIENT_SECRET": "segredo-ficticio",
		"STRIPE_SECRET_KEY": "sk_test_ficticia", "STRIPE_WEBHOOK_SECRET": "whsec_ficticio", "STRIPE_PRICE_ID": "price_ficticio",
		"APPLYER_ENDERECO_PUBLICO": "https://nuvem.exemplo.com/", "NOTAS_PROVEDOR": "falso",
	}
}

func TestLerConfigFaltando(t *testing.T) {
	env := envCompleto(t)
	delete(env, "DATABASE_URL")
	_, err := LerConfig(envDe(env))
	if err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("esperava erro citando DATABASE_URL, veio %v", err)
	}
}

func TestLerConfigExigeTermosESMTP(t *testing.T) {
	for _, k := range []string{"APPLYER_TERMOS_VERSAO", "SMTP_HOST", "SMTP_PORTA", "SMTP_USUARIO", "SMTP_SENHA", "SMTP_REMETENTE", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET",
		"STRIPE_SECRET_KEY", "STRIPE_WEBHOOK_SECRET", "STRIPE_PRICE_ID", "APPLYER_ENDERECO_PUBLICO", "NOTAS_PROVEDOR"} {
		env := envCompleto(t)
		delete(env, k)
		if _, err := LerConfig(envDe(env)); err == nil || !strings.Contains(err.Error(), k) {
			t.Errorf("sem %s: veio %v", k, err)
		}
	}
}

func TestLerConfigChaveInvalida(t *testing.T) {
	for _, v := range []string{"k", base64.StdEncoding.EncodeToString([]byte("curta-demais"))} {
		env := envCompleto(t)
		env["APPLYER_CHAVE_PRIVADA"] = v
		_, err := LerConfig(envDe(env))
		if err == nil || !strings.Contains(err.Error(), "APPLYER_CHAVE_PRIVADA") || strings.Contains(err.Error(), v) {
			t.Errorf("chave %q: veio %v", v, err)
		}
	}
	// A semente de 32 bytes também vale.
	env := envCompleto(t)
	seed := make([]byte, ed25519.SeedSize)
	env["APPLYER_CHAVE_PRIVADA"] = base64.StdEncoding.EncodeToString(seed)
	if _, err := LerConfig(envDe(env)); err != nil {
		t.Errorf("semente: %v", err)
	}
}

func TestLerConfigNaoVazaValor(t *testing.T) {
	_, err := LerConfig(envDe(map[string]string{"APPLYER_SEGREDO_CODIGOS": "segredo-fictício-123"}))
	if err == nil || strings.Contains(err.Error(), "segredo-fictício-123") {
		t.Fatalf("erro nulo ou vazou valor: %v", err)
	}
}

func TestLerConfigPadroes(t *testing.T) {
	cfg, err := LerConfig(envDe(envCompleto(t)))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Endereco != ":8080" {
		t.Errorf("endereço padrão = %q", cfg.Endereco)
	}
	if cfg.EnderecoPublico != "https://nuvem.exemplo.com" {
		t.Errorf("endereço público sem a barra final = %q", cfg.EnderecoPublico)
	}
	if cfg.DatabaseURL != "postgres://x" || cfg.TermosVersao != "2026-11" {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestLerConfigNotas(t *testing.T) {
	env := envCompleto(t)
	env["NOTAS_PROVEDOR"] = "outro"
	if _, err := LerConfig(envDe(env)); err == nil || !strings.Contains(err.Error(), "NOTAS_PROVEDOR") {
		t.Errorf("provedor desconhecido: veio %v", err)
	}
	env["NOTAS_PROVEDOR"] = "nfeio"
	_, err := LerConfig(envDe(env))
	for _, k := range []string{"NOTAS_CHAVE", "NOTAS_EMPRESA", "NOTAS_SERVICO"} {
		if err == nil || !strings.Contains(err.Error(), k) {
			t.Errorf("nfeio sem %s: veio %v", k, err)
		}
	}
	env["NOTAS_CHAVE"], env["NOTAS_EMPRESA"], env["NOTAS_SERVICO"] = "chave-ficticia", "emp1", "1.05"
	if _, err := LerConfig(envDe(env)); err != nil {
		t.Errorf("nfeio completo: %v", err)
	}
}
