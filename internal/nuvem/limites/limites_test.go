package limites

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestJanela(t *testing.T) {
	agora := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	j := Nova(3, time.Minute, func() time.Time { return agora })
	for i := 0; i < 3; i++ {
		if !j.Permitir("ana") {
			t.Fatalf("tentativa %d deveria passar", i+1)
		}
	}
	if j.Permitir("ana") {
		t.Fatal("4ª tentativa deveria ser negada")
	}
	if !j.Permitir("bia") {
		t.Fatal("outra chave não deve ser afetada")
	}
	agora = agora.Add(61 * time.Second)
	if !j.Permitir("ana") {
		t.Fatal("após 61 s deveria permitir de novo")
	}
}

func TestJanelaLimpaChavesAntigas(t *testing.T) {
	agora := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	j := Nova(3, time.Minute, func() time.Time { return agora })
	for i := 0; i < 2000; i++ {
		j.Permitir(fmt.Sprintf("k%d", i))
	}
	agora = agora.Add(2 * time.Minute)
	j.Permitir("nova")
	if n := j.tamanho(); n > 1 {
		t.Fatalf("chaves antigas não foram limpas: %d", n)
	}
}

func TestJanelaConcorrente(t *testing.T) {
	j := Nova(100, time.Minute, time.Now)
	var wg sync.WaitGroup
	var mu sync.Mutex
	passou := 0
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if j.Permitir("x") {
				mu.Lock()
				passou++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if passou != 100 {
		t.Fatalf("passaram %d, esperado 100", passou)
	}
}
