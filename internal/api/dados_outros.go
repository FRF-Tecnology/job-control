//go:build !windows

package api

// emUso: fora do Windows, apagar arquivo aberto não falha.
func emUso(err error) bool { return false }
