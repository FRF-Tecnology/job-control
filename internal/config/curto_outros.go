//go:build !windows

package config

// formaCurta só existe no Windows; nos outros sistemas, o caminho é um só.
func formaCurta(caminho string) string { return caminho }
