// Comando nuvem sobe o servidor de contas, licença e assinatura do Applyer.
package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/nuvem"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/banco"
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

	srv := nuvem.Novo(cfg, nuvem.Dependencias{Banco: pool, Agora: time.Now, Aleatorio: rand.Reader})
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
	if err := sh.Shutdown(fim); err != nil {
		return err
	}
	return nil
}
