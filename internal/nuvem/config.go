// Package nuvem é o servidor de contas, licença e assinatura do Applyer (cmd/nuvem).
package nuvem

import (
	"fmt"
	"strings"
)

// Config reúne as variáveis de ambiente do servidor. Só as obrigatórias de cada fase são exigidas;
// as das integrações (Stripe, Google, SMTP, notas) passam a ser exigidas pelas tasks que as usam.
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
		EnderecoPublico:     v("APPLYER_ENDERECO_PUBLICO"),
	}
	if cfg.Endereco == "" {
		cfg.Endereco = ":8080"
	}
	var faltam []string
	for _, o := range []struct{ nome, valor string }{
		{"DATABASE_URL", cfg.DatabaseURL},
		{"APPLYER_CHAVE_PRIVADA", cfg.ChavePrivada},
		{"APPLYER_SEGREDO_CODIGOS", cfg.SegredoCodigos},
	} {
		if o.valor == "" {
			faltam = append(faltam, o.nome)
		}
	}
	if len(faltam) > 0 {
		return Config{}, fmt.Errorf("variáveis de ambiente que faltam: %s", strings.Join(faltam, ", "))
	}
	return cfg, nil
}
