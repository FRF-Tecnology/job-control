//go:build !contafalsa

package main

import (
	"log"
	"net/http"

	"github.com/FRF-Tecnology/job-control/internal/conta"
)

// contaDoDev: sem a tag contafalsa, o central-dev fica sem conta e sem trava (ruling A11) e sem a rota de teste.
func contaDoDev(string) (*conta.Conta, func(http.Handler) http.Handler) {
	log.Print(conta.AvisoDesligada)
	return nil, func(h http.Handler) http.Handler { return h }
}
