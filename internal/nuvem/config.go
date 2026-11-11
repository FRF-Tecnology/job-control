// Package nuvem é o servidor de contas, licença e assinatura do Applyer (cmd/nuvem).
package nuvem

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// Config reúne as variáveis de ambiente do servidor. Só as obrigatórias de cada fase são exigidas;
// as das integrações (Stripe, Google, notas) passam a ser exigidas pelas tasks que as usam.
type Config struct {
	DatabaseURL         string // DATABASE_URL
	ChavePrivada        string // APPLYER_CHAVE_PRIVADA (Ed25519 em base64)
	SegredoCodigos      string // APPLYER_SEGREDO_CODIGOS
	StripeSecretKey     string // STRIPE_SECRET_KEY
	StripeWebhookSecret string // STRIPE_WEBHOOK_SECRET
	StripePriceID       string // STRIPE_PRICE_ID
	GoogleClientID      string // GOOGLE_CLIENT_ID
	GoogleClientSecret  string // GOOGLE_CLIENT_SECRET
	SMTPHost            string // SMTP_HOST
	SMTPPorta           string // SMTP_PORTA
	SMTPUsuario         string // SMTP_USUARIO
	SMTPSenha           string // SMTP_SENHA
	SMTPRemetente       string // SMTP_REMETENTE
	NotasProvedor       string // NOTAS_PROVEDOR: nfeio | falso
	NotasChave          string // NOTAS_CHAVE
	NotasEmpresa        string // NOTAS_EMPRESA
	NotasServico        string // NOTAS_SERVICO
	TermosVersao        string // APPLYER_TERMOS_VERSAO
	Endereco            string // APPLYER_ENDERECO (padrão ":8080")
	EnderecoPublico     string // APPLYER_ENDERECO_PUBLICO
}

// LerConfig lê a configuração por env (os.Getenv em produção). O erro cita só o nome da variável que falta.
func LerConfig(env func(string) string) (Config, error) {
	v := func(k string) string { return strings.TrimSpace(env(k)) }
	cfg := Config{
		DatabaseURL:         v("DATABASE_URL"),
		ChavePrivada:        v("APPLYER_CHAVE_PRIVADA"),
		SegredoCodigos:      v("APPLYER_SEGREDO_CODIGOS"),
		StripeSecretKey:     v("STRIPE_SECRET_KEY"),
		StripeWebhookSecret: v("STRIPE_WEBHOOK_SECRET"),
		StripePriceID:       v("STRIPE_PRICE_ID"),
		GoogleClientID:      v("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:  v("GOOGLE_CLIENT_SECRET"),
		SMTPHost:            v("SMTP_HOST"),
		SMTPPorta:           v("SMTP_PORTA"),
		SMTPUsuario:         v("SMTP_USUARIO"),
		SMTPSenha:           v("SMTP_SENHA"),
		SMTPRemetente:       v("SMTP_REMETENTE"),
		NotasProvedor:       v("NOTAS_PROVEDOR"),
		NotasChave:          v("NOTAS_CHAVE"),
		NotasEmpresa:        v("NOTAS_EMPRESA"),
		NotasServico:        v("NOTAS_SERVICO"),
		TermosVersao:        v("APPLYER_TERMOS_VERSAO"),
		Endereco:            v("APPLYER_ENDERECO"),
		EnderecoPublico:     strings.TrimRight(v("APPLYER_ENDERECO_PUBLICO"), "/"), // sem barra final: recebe caminhos
	}
	if cfg.Endereco == "" {
		cfg.Endereco = ":8080"
	}
	var faltam []string
	for _, o := range []struct{ nome, valor string }{
		{"DATABASE_URL", cfg.DatabaseURL},
		{"APPLYER_CHAVE_PRIVADA", cfg.ChavePrivada},
		{"APPLYER_SEGREDO_CODIGOS", cfg.SegredoCodigos},
		{"APPLYER_TERMOS_VERSAO", cfg.TermosVersao},
		{"SMTP_HOST", cfg.SMTPHost},
		{"SMTP_PORTA", cfg.SMTPPorta},
		{"SMTP_USUARIO", cfg.SMTPUsuario},
		{"SMTP_SENHA", cfg.SMTPSenha},
		{"SMTP_REMETENTE", cfg.SMTPRemetente},
		{"GOOGLE_CLIENT_ID", cfg.GoogleClientID},
		{"GOOGLE_CLIENT_SECRET", cfg.GoogleClientSecret},
		{"STRIPE_SECRET_KEY", cfg.StripeSecretKey},
		{"STRIPE_WEBHOOK_SECRET", cfg.StripeWebhookSecret},
		{"STRIPE_PRICE_ID", cfg.StripePriceID},
		{"APPLYER_ENDERECO_PUBLICO", cfg.EnderecoPublico},
		{"NOTAS_PROVEDOR", cfg.NotasProvedor},
	} {
		if o.valor == "" {
			faltam = append(faltam, o.nome)
		}
	}
	if len(faltam) > 0 {
		return Config{}, fmt.Errorf("variáveis de ambiente que faltam: %s", strings.Join(faltam, ", "))
	}
	switch cfg.NotasProvedor {
	case "falso":
	case "nfeio":
		for _, o := range []struct{ nome, valor string }{
			{"NOTAS_CHAVE", cfg.NotasChave}, {"NOTAS_EMPRESA", cfg.NotasEmpresa}, {"NOTAS_SERVICO", cfg.NotasServico},
		} {
			if o.valor == "" {
				faltam = append(faltam, o.nome)
			}
		}
		if len(faltam) > 0 {
			return Config{}, fmt.Errorf("variáveis de ambiente que faltam: %s", strings.Join(faltam, ", "))
		}
	default:
		return Config{}, errors.New("NOTAS_PROVEDOR inválido: use nfeio ou falso")
	}
	if _, err := cfg.Chave(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

var errChave = errors.New("APPLYER_CHAVE_PRIVADA inválida: esperava Ed25519 em base64 (semente de 32 ou chave de 64 bytes)")

// Chave decodifica APPLYER_CHAVE_PRIVADA. O erro nunca traz o valor.
func (c Config) Chave() (ed25519.PrivateKey, error) {
	b, err := base64.StdEncoding.DecodeString(c.ChavePrivada)
	if err != nil {
		return nil, errChave
	}
	switch len(b) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(b), nil
	case ed25519.PrivateKeySize:
		// A metade pública tem de bater com a semente; senão a chave está corrompida.
		k := ed25519.NewKeyFromSeed(b[:ed25519.SeedSize])
		if !k.Equal(ed25519.PrivateKey(b)) {
			return nil, errChave
		}
		return k, nil
	}
	return nil, errChave
}
