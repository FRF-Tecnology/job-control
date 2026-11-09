//go:build !windows

package config

import "os"

// ProtegerPasta deixa a pasta só para o usuário (0700), também quando ela já existia. No Windows, a versão de
// protecao_windows.go aplica uma DACL explícita e protegida (jc-a2y.58).
func ProtegerPasta(pasta string) error { return os.Chmod(pasta, 0o700) }
