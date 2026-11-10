//go:build windows

package ia

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// semJanela: o app é um programa de janela, sem console. Sem esta flag, o Windows abre um console novo (uma janela de
// terminal) para cada CLI chamada. CREATE_NO_WINDOW dá à CLI um console sem janela, que os processos filhos dela
// herdam; DETACHED_PROCESS não serve, porque cada filho de console abriria a própria janela.
func semJanela(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NO_WINDOW
}
