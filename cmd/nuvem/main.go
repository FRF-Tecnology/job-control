// Comando nuvem sobe o servidor de contas, licença e assinatura do Applyer.
package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/nuvem"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/banco"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/emails"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/google"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/pagamento"
)

func main() {
	if err := executar(); err != nil {
		fmt.Fprintln(os.Stderr, "nuvem:", err)
		os.Exit(1)
	}
}

func executar() error {
	cfg, err := nuvem.LerConfig(os.Getenv)
	if err != nil {
		return err
	}
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	pool, err := banco.Abrir(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	envio := emails.SMTP{Host: cfg.SMTPHost, Porta: cfg.SMTPPorta, Usuario: cfg.SMTPUsuario, Senha: cfg.SMTPSenha, Remetente: cfg.SMTPRemetente}
	srv := nuvem.Novo(cfg, nuvem.Dependencias{Banco: pool, Agora: time.Now, Aleatorio: rand.Reader, Emails: envio,
		Google: &google.Real{ClientID: cfg.GoogleClientID, ClientSecret: cfg.GoogleClientSecret},
		Stripe: pagamento.NovoReal(cfg.StripeSecretKey, cfg.StripeWebhookSecret, cfg.StripePriceID, cfg.EnderecoPublico),
		Notas:  notasDepois{}})
	sh := &http.Server{
		Addr:              cfg.Endereco,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	falha := make(chan error, 1)
	go func() { falha <- sh.ListenAndServe() }()
	fmt.Println("nuvem: ouvindo em", cfg.Endereco)

	select {
	case err := <-falha:
		return err
	case <-ctx.Done():
	}
	fim, cancelar := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelar()
	errDesligar := sh.Shutdown(fim)
	srv.Esperar(fim) // emails em andamento (códigos) terminam dentro do mesmo prazo
	return errDesligar
}

// notasDepois é o agendador provisório até a emissão de notas (Task 10): só registra os ids.
type notasDepois struct{}

func (notasDepois) Agendar(_ context.Context, invoiceID, usuarioID string) error {
	slog.Info("nota fiscal pendente", "fatura", invoiceID, "usuario", usuarioID)
	return nil
}
