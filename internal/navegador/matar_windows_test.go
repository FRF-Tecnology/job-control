//go:build windows

package navegador

import "golang.org/x/sys/windows"

// fotoDoNavegador guarda os processos do navegador agora (PID e instante de criação) e devolve a função que encerra
// os que ainda estiverem vivos. Só encerra o mesmo processo (PID e instante iguais): um PID reaproveitado por outro
// programa do runner nunca é tocado. Para os testes com janela não deixarem órfãos para os passos seguintes do CI
// (jc-a2y.6.14).
func fotoDoNavegador(n *Navegador) func() int {
	raiz := pidDoNavegador(n)
	if raiz == 0 {
		return func() int { return 0 }
	}
	type vivo struct {
		pid    uint32
		criado int64
	}
	var foto []vivo
	pids, _ := arvore(raiz)
	for p := range pids {
		if c, ok := criacao(p); ok {
			foto = append(foto, vivo{p, c})
		}
	}
	return func() int {
		encerrados := 0
		for _, v := range foto {
			if c, ok := criacao(v.pid); !ok || c != v.criado {
				continue // já saiu, ou o PID é de outro processo agora
			}
			h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, v.pid)
			if err != nil {
				continue
			}
			if windows.TerminateProcess(h, 1) == nil {
				encerrados++
			}
			windows.CloseHandle(h)
		}
		return encerrados
	}
}
