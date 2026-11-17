package main

import (
	"log"

	"github.com/FRF-Tecnology/job-control/internal/api"
	"github.com/FRF-Tecnology/job-control/internal/conta"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// ligarConta: a conta do app sobre a nuvem do build (ldflags -X em internal/conta), com o mesmo cofre da API, e a
// trava das ações (Conta.PodeAgir). Ruling A11: build sem a nuvem, nada (conta e trava nil, o app como antes) e uma
// linha de aviso no log. Chave inválida no build: erro (o app não abre com a nuvem pela metade).
func ligarConta(cofre segredos.Cofre, pasta string) (*conta.Conta, func() (bool, string), error) {
	c, err := conta.DoBuild(cofre, pasta)
	if err != nil {
		return nil, nil, err
	}
	if c == nil {
		log.Print(conta.AvisoDesligada)
		return nil, nil, nil
	}
	return c, c.PodeAgir, nil
}

// opcoesDaConta põe a conta, a trava e o abrir do navegador nas Opcoes da API local.
func opcoesDaConta(o api.Opcoes, c *conta.Conta, podeAgir func() (bool, string), abrir func(url string) error) api.Opcoes {
	o.Conta, o.PodeAgir, o.AbrirNavegador = c, podeAgir, abrir
	return o
}
