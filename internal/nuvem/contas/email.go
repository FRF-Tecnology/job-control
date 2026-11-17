// Package contas reúne as regras de contas da nuvem.
package contas

import (
	"errors"
	"strings"
)

// ErrEmailInvalido indica um email vazio, sem @, com espaço interno ou longo demais.
var ErrEmailInvalido = errors.New("email inválido")

// NormalizarEmail tira espaços das pontas e põe em minúsculas; é a forma gravada no banco.
func NormalizarEmail(s string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(s))
	if e == "" || len(e) > 254 || strings.ContainsAny(e, " \t\r\n") {
		return "", ErrEmailInvalido
	}
	i := strings.Index(e, "@")
	if i <= 0 || i == len(e)-1 || strings.Count(e, "@") != 1 {
		return "", ErrEmailInvalido
	}
	return e, nil
}
