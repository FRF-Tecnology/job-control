//go:build !windows

package ia

import "os/exec"

// semJanela: fora do Windows, rodar uma CLI não abre janela.
func semJanela(*exec.Cmd) {}
