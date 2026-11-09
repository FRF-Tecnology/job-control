//go:build windows

package navegador

import (
	"os/exec"
	"syscall"

	"github.com/chromedp/chromedp"
	"golang.org/x/sys/windows"
)

// opcoesDoSistema: no Windows, o navegador do envio roda com prioridade abaixo do normal, para não disputar a CPU
// com quem usa o PC (jc-a2y.6.7). Os processos filhos do Chrome herdam a classe. O chromedp já pôs em SysProcAttr
// os handles do pipe do DevTools: só acrescenta a flag.
func opcoesDoSistema() []chromedp.ExecAllocatorOption {
	return []chromedp.ExecAllocatorOption{chromedp.ModifyCmdFunc(prioridadeBaixa)}
}

func prioridadeBaixa(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windows.BELOW_NORMAL_PRIORITY_CLASS
}
