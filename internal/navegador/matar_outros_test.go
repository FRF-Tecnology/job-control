//go:build !windows

package navegador

// fotoDoNavegador: fora do Windows não há o problema do PID reaproveitado no teste de foco (ele só roda no
// Windows); nada a encerrar.
func fotoDoNavegador(n *Navegador) func() int { return func() int { return 0 } }
