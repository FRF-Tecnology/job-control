//go:build windows

package config

import (
	"regexp"
	"strings"

	"golang.org/x/sys/windows"
)

// sep: \ ou / entre as partes do caminho (o Windows aceita as duas, e até misturadas).
const sep = `[\\/]+`

// substituir: no Windows, C:\Users\Ana, c:/users/ana e C:\USERS\ANA são a mesma pasta, e o erro traz o caminho como
// foi digitado (revisão do Fallen no #71). Cada forma vira uma expressão sem diferenciar maiúsculas, com \ ou / entre
// as partes, que só casa quando o nome da pasta termina ali (C:\Users\Ana não pega C:\Users\Anabela). Os pares vão em
// ordem, do mais específico para o mais geral; depois de trocado, o marcador não casa com os seguintes.
func substituir(texto string, pares [][2]string) string {
	for _, p := range pares {
		partes := strings.FieldsFunc(p[0], func(r rune) bool { return r == '\\' || r == '/' })
		if len(partes) == 0 {
			continue
		}
		for i := range partes {
			partes[i] = regexp.QuoteMeta(partes[i])
		}
		inicio := ""
		if strings.HasPrefix(p[0], `\`) || strings.HasPrefix(p[0], "/") {
			inicio = sep // caminho de rede (\\servidor\pasta)
		}
		re := regexp.MustCompile(`(?i)` + inicio + strings.Join(partes, sep) + `([^\pL\pN._-]|$)`)
		texto = re.ReplaceAllString(texto, strings.ReplaceAll(p[1], "$", "$$")+"${1}")
	}
	return texto
}

// formaCurta devolve o nome 8.3 do Windows (C:\Users\NOME~1\...), que o TEMP e alguns erros usam. Se a pasta não
// existe ou o volume não tem nomes curtos, devolve o próprio caminho.
func formaCurta(caminho string) string {
	p, err := windows.UTF16PtrFromString(caminho)
	if err != nil {
		return caminho
	}
	tam := uint32(windows.MAX_PATH)
	for range 3 { // não coube: a resposta é o tamanho necessário; tenta de novo com ele
		buf := make([]uint16, tam)
		n, err := windows.GetShortPathName(p, &buf[0], tam)
		if err != nil || n == 0 {
			return caminho
		}
		if n < tam {
			return windows.UTF16ToString(buf[:n])
		}
		tam = n
	}
	return caminho
}
