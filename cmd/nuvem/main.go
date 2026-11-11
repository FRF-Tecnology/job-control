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
	"github.com/FRF-Tecnology/job-control/internal/nuvem/notas"
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
	stripe := pagamento.NovoReal(cfg.StripeSecretKey, cfg.StripeWebhookSecret, cfg.StripePriceID, cfg.EnderecoPublico)
	var emissor notas.Emissor
	if cfg.NotasProvedor == "nfeio" {
		emissor = &notas.NFEio{Chave: cfg.NotasChave, Empresa: cfg.NotasEmpresa, Servico: cfg.NotasServico}
	} else {
		slog.Warn("NOTAS_PROVEDOR=falso: nenhuma nota fiscal de verdade será emitida; use só em teste ou rodada local")
		emissor = &notas.Falso{}
	}
	fila := notas.NovaFila(pool, stripe, emissor, slog.Default())
	srv := nuvem.Novo(cfg, nuvem.Dependencias{Banco: pool, Agora: time.Now, Aleatorio: rand.Reader, Emails: envio,
		Google: &google.Real{ClientID: cfg.GoogleClientID, ClientSecret: cfg.GoogleClientSecret},
		Stripe: stripe, Notas: fila})
	// A fila roda até o desligamento; o main espera a passada em andamento terminar.
	ctxFila, pararFila := context.WithCancel(ctx)
	filaFim := make(chan struct{})
	go func() { fila.Rodar(ctxFila); close(filaFim) }()
	defer func() { pararFila(); <-filaFim }()
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
