//go:build windows

package config

import "golang.org/x/sys/windows"

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
