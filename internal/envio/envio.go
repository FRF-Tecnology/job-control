// Package envio liga o executor de candidaturas no app (P5, jc-a2y.6.2.7).
//
// Até a jc-a2y.6.5 (primeiro envio de verdade, com o Vinicius), o envio fica DESLIGADO: no build padrão, Ligar não
// faz nada e o binário nem traz o executor nem o navegador (envio_padrao.go). O gatilho só existe num build de
// desenvolvimento com a tag envio_dev (envio_dev.go): go build -tags envio_dev ./app. Não há variável de ambiente
// nem botão que ligue o envio.
package envio
