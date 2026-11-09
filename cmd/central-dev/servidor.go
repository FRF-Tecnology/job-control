package main

import (
	"net/http"
	"time"
)

// Tempos limite do servidor do central-dev (jc-a2y.14, gosec G114). Ele só ouve em 127.0.0.1, mas um cliente
// lento (cabeçalho pela metade) não pode prender conexões para sempre.
var (
	lerCabecalho = 5 * time.Second
	lerTudo      = 30 * time.Second
	// escrever fica acima do tempo limite das chamadas de IA (120 s em internal/ia), senão a resposta é cortada.
	escrever = 3 * time.Minute
	ocioso   = 60 * time.Second
)

// novoServidor monta o http.Server com os tempos limite (no lugar do http.ListenAndServe sem tempos).
func novoServidor(endereco string, h http.Handler) *http.Server {
	return &http.Server{
		Addr:              endereco,
		Handler:           h,
		ReadHeaderTimeout: lerCabecalho,
		ReadTimeout:       lerTudo,
		WriteTimeout:      escrever,
		IdleTimeout:       ocioso,
		MaxHeaderBytes:    1 << 20,
	}
}
