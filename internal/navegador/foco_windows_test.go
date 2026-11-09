//go:build windows

package navegador

import (
	"os"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/chromedp/chromedp"
	"golang.org/x/sys/windows"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// jc-a2y.6.7: um envio fictício, com o Chrome (ou Edge) de verdade SEM JANELA, não muda a janela em primeiro plano
// nem a posição do mouse, e o navegador não cria janela visível. Também mede o consumo (CPU e memória) do navegador.
//
// A janela em primeiro plano e o mouse só são conferidos no CI (CI=true): no PC de quem está usando, a própria pessoa
// troca de janela e mexe no mouse. A ausência de janela do navegador é conferida sempre.

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	procGetCursorPos = user32.NewProc("GetCursorPos")
	psapi            = windows.NewLazySystemDLL("psapi.dll")
	procMemoria      = psapi.NewProc("GetProcessMemoryInfo")
)

type ponto struct{ X, Y int32 }

func posicaoDoMouse() (ponto, bool) {
	var p ponto
	r, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	return p, r != 0
}

// processMemoryCounters é o PROCESS_MEMORY_COUNTERS do psapi.
type processMemoryCounters struct {
	Cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

// arvore devolve o processo e todos os descendentes dele.
//
// O Windows reaproveita PID: um processo órfão (o pai morreu) guarda o ParentProcessID antigo, e esse número pode ser
// o do navegador novo. Foi o que fez o CI falhar no #98 (run 42681823547: 138 processos, CPU 203 s e janela visível
// num envio sem janela de 3,5 s), com restos do Chrome com janela de um teste anterior do mesmo job. Por isso, só
// conta como filho o processo criado DEPOIS do pai: um filho de verdade nunca é mais velho que o pai.
func arvore(raiz uint32) map[uint32]bool {
	pais := map[uint32]uint32{}
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return map[uint32]bool{raiz: true}
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err := windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		pais[e.ProcessID] = e.ParentProcessID
	}
	return descendentes(raiz, pais, criacao)
}

// descendentes: a raiz e os descendentes dela pelo mapa filho → pai, sem os mais velhos que o pai (PID
// reaproveitado). criadoEm devolve o instante de criação; sem ele, o processo fica de fora quando o pai tem instante.
func descendentes(raiz uint32, pais map[uint32]uint32, criadoEm func(uint32) (int64, bool)) map[uint32]bool {
	type inst struct {
		c  int64
		ok bool
	}
	cache := map[uint32]inst{}
	criado := func(pid uint32) (int64, bool) {
		if v, tem := cache[pid]; tem {
			return v.c, v.ok
		}
		c, ok := criadoEm(pid)
		cache[pid] = inst{c, ok}
		return c, ok
	}
	dentro := map[uint32]bool{raiz: true}
	fora := map[uint32]bool{}
	for mudou := true; mudou; {
		mudou = false
		for pid, pai := range pais {
			if dentro[pid] || fora[pid] || !dentro[pai] {
				continue
			}
			cPai, okPai := criado(pai)
			cFilho, okFilho := criado(pid)
			if okPai && (!okFilho || cFilho < cPai) {
				fora[pid] = true // órfão com o PID antigo do pai reaproveitado: não é deste navegador
				continue
			}
			dentro[pid], mudou = true, true
		}
	}
	return dentro
}

// criacao: o instante de criação do processo (FILETIME em unidades de 100 ns).
func criacao(pid uint32) (int64, bool) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return 0, false
	}
	defer windows.CloseHandle(h)
	var criado, saiu, kernel, usuario windows.Filetime
	if err := windows.GetProcessTimes(h, &criado, &saiu, &kernel, &usuario); err != nil {
		return 0, false
	}
	return int64(criado.HighDateTime)<<32 | int64(criado.LowDateTime), true
}

// consumo: tempo de CPU (kernel + usuário) e pico de memória (working set) de um processo.
func consumo(pid uint32) (cpu time.Duration, pico uintptr, ok bool) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return 0, 0, false
	}
	defer windows.CloseHandle(h)
	var criado, saiu, kernel, usuario windows.Filetime
	if err := windows.GetProcessTimes(h, &criado, &saiu, &kernel, &usuario); err != nil {
		return 0, 0, false
	}
	cem := func(f windows.Filetime) int64 { return int64(f.HighDateTime)<<32 | int64(f.LowDateTime) }
	cpu = time.Duration((cem(kernel) + cem(usuario)) * 100)
	var m processMemoryCounters
	m.Cb = uint32(unsafe.Sizeof(m))
	if r, _, _ := procMemoria.Call(uintptr(h), uintptr(unsafe.Pointer(&m)), uintptr(m.Cb)); r != 0 {
		pico = m.PeakWorkingSetSize
	}
	return cpu, pico, true
}

var (
	contarJanelas sync.Mutex
	janelasAlvo   map[uint32]bool
	janelasAchou  int
	enumJanelas   = windows.NewCallback(func(h windows.HWND, _ uintptr) uintptr {
		var pid uint32
		if _, err := windows.GetWindowThreadProcessId(h, &pid); err == nil && janelasAlvo[pid] && windows.IsWindowVisible(h) {
			janelasAchou++
		}
		return 1 // continua
	})
)

// janelasVisiveis conta as janelas de nível superior visíveis dos processos dados.
func janelasVisiveis(pids map[uint32]bool) int {
	contarJanelas.Lock()
	defer contarJanelas.Unlock()
	janelasAlvo, janelasAchou = pids, 0
	windows.EnumWindows(enumJanelas, nil)
	return janelasAchou
}

func pidDoNavegador(n *Navegador) uint32 {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.navCtx == nil {
		return 0
	}
	c := chromedp.FromContext(n.navCtx)
	if c == nil || c.Browser == nil || c.Browser.Process() == nil {
		return 0
	}
	return uint32(c.Browser.Process().Pid)
}

func TestEnvioNaoPegaFocoNemMouse(t *testing.T) {
	srv := novoServidor(t)
	dados := t.TempDir()
	nav := navegadorDeTeste(t, srv, dados) // sem janela: o padrão
	s := novoCenario(t, nav, dados)
	noCI := os.Getenv("CI") == "true"

	frente := windows.GetForegroundWindow()
	mouse, temMouse := posicaoDoMouse()
	// No log do CI: com primeiro plano 0 e sem mouse, o runner não tem área de trabalho e a conferência abaixo não
	// prova nada; fica registrado.
	t.Logf("início: janela em primeiro plano 0x%x, mouse legível %v (%d,%d), CI %v", frente, temMouse, mouse.X, mouse.Y, noCI)
	// No CI, sem área de trabalho a conferência passaria sem provar nada (sugestão do Kauai no #76): falha.
	if noCI && (frente == 0 || !temMouse) {
		t.Fatalf("CI sem área de trabalho: primeiro plano 0x%x, mouse legível %v; a conferência não provaria nada", frente, temMouse)
	}

	parar := make(chan struct{})
	var mu sync.Mutex
	var mudouFrente, mudouMouse, comJanela, amostras int
	cpu := map[uint32]time.Duration{}
	pico := map[uint32]uintptr{}
	var raiz uint32
	var feito sync.WaitGroup
	feito.Add(1)
	go func() {
		defer feito.Done()
		tique := time.NewTicker(50 * time.Millisecond)
		defer tique.Stop()
		for {
			select {
			case <-parar:
				return
			case <-tique.C:
			}
			mu.Lock()
			amostras++
			if windows.GetForegroundWindow() != frente {
				mudouFrente++
			}
			if p, ok := posicaoDoMouse(); temMouse && ok && p != mouse {
				mudouMouse++
			}
			if pid := pidDoNavegador(nav); pid != 0 {
				raiz = pid
				pids := arvore(pid)
				if janelasVisiveis(pids) > 0 {
					comJanela++
				}
				for p := range pids {
					if c, m, ok := consumo(p); ok {
						cpu[p], pico[p] = c, m
					}
				}
			}
			mu.Unlock()
		}
	}()

	d := s.enviar(s.aprovada("linkedin", "https://www.linkedin.com/jobs/view/9000000001"))
	close(parar)
	feito.Wait()

	if d.Estado != dominio.CandEnviada {
		t.Fatalf("desfecho = %+v", d)
	}
	if raiz == 0 || amostras == 0 {
		t.Fatalf("o navegador não foi observado (raiz %d, %d amostras)", raiz, amostras)
	}
	if comJanela > 0 {
		t.Errorf("o navegador do envio teve janela visível em %d de %d amostras", comJanela, amostras)
	}
	if c, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, raiz); err == nil {
		if classe, err := windows.GetPriorityClass(c); err == nil && classe != windows.BELOW_NORMAL_PRIORITY_CLASS {
			t.Errorf("prioridade do navegador = 0x%x; queria abaixo do normal (0x4000)", classe)
		}
		windows.CloseHandle(c)
	}
	if noCI {
		if mudouFrente > 0 {
			t.Errorf("a janela em primeiro plano mudou em %d de %d amostras", mudouFrente, amostras)
		}
		if mudouMouse > 0 {
			t.Errorf("o mouse mudou de lugar em %d de %d amostras", mudouMouse, amostras)
		}
	} else {
		t.Logf("fora do CI: primeiro plano e mouse não conferidos (quem usa o PC mexe neles); mudaram em %d e %d de %d amostras", mudouFrente, mudouMouse, amostras)
	}
	var totalCPU time.Duration
	var totalPico uintptr
	for p := range cpu {
		totalCPU += cpu[p]
		totalPico += pico[p]
	}
	t.Logf("consumo do navegador num envio fictício: %d processos, CPU %.2f s, soma dos picos de memória %.0f MB, %d amostras",
		len(cpu), totalCPU.Seconds(), float64(totalPico)/(1<<20), amostras)
}
