//go:build windows

package navegador

import (
	"sort"
	"testing"
)

// A árvore de processos do teste de foco ignora o órfão cujo pai morreu e cujo PID foi reaproveitado pelo navegador
// novo (run 42681823547 do #98). PIDs e instantes fictícios.
func TestDescendentesIgnoraPIDReaproveitado(t *testing.T) {
	pais := map[uint32]uint32{
		11: 10, // órfão de um Chrome com janela de outro teste: o pai antigo tinha o PID 10, hoje do navegador novo
		13: 11, // filho do órfão
		12: 10, // filho de verdade (renderizador)
		14: 12, // neto de verdade (GPU)
		20: 99, // outro processo qualquer
	}
	criado := map[uint32]int64{10: 1000, 11: 500, 13: 600, 12: 1010, 14: 1020, 20: 10}
	got := descendentes(10, pais, func(pid uint32) (int64, bool) { c, ok := criado[pid]; return c, ok })
	var pids []int
	for p := range got {
		pids = append(pids, int(p))
	}
	sort.Ints(pids)
	if len(pids) != 3 || pids[0] != 10 || pids[1] != 12 || pids[2] != 14 {
		t.Fatalf("descendentes = %v; queria [10 12 14]", pids)
	}
	// Sem o instante do filho (processo que já saiu), fica de fora; sem o do pai, conta (não dá para comparar).
	got = descendentes(10, map[uint32]uint32{15: 10}, func(pid uint32) (int64, bool) { return 1000, pid == 10 })
	if got[15] {
		t.Error("contou filho sem instante de criação")
	}
	got = descendentes(10, map[uint32]uint32{15: 10}, func(pid uint32) (int64, bool) { return 0, false })
	if !got[15] {
		t.Error("sem o instante do pai, o filho deveria contar")
	}
}
