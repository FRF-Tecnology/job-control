// Navegador falso dos testes (jc-a2y.6.7): anota os argumentos e a prioridade com que foi aberto num arquivo ao
// lado do executável e sai. Não abre janela nem rede.
package main

import (
	"os"
	"path/filepath"
	"strings"
)

func main() {
	exe, err := os.Executable()
	if err != nil {
		os.Exit(2)
	}
	linha := prioridade() + "\t" + strings.Join(os.Args[1:], " ") + "\n"
	f, err := os.OpenFile(filepath.Join(filepath.Dir(exe), "chamadas.txt"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		os.Exit(2)
	}
	f.WriteString(linha)
	f.Close()
	os.Exit(1) // não é um navegador: o chromedp desiste
}
