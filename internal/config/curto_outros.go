//go:build !windows

package config

import "strings"

// formaCurta só existe no Windows; nos outros sistemas, o caminho é um só.
func formaCurta(caminho string) string { return caminho }

// substituir: fora do Windows, o caminho diferencia maiúsculas e só usa /. O Replacer tenta os pares na ordem
// quando dois começam no mesmo ponto (o mais específico vem antes).
func substituir(texto string, pares [][2]string) string {
	velhos := make([]string, 0, 2*len(pares))
	for _, p := range pares {
		velhos = append(velhos, p[0], p[1])
	}
	return strings.NewReplacer(velhos...).Replace(texto)
}
