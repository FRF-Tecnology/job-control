package limites

import (
	"fmt"
	"testing"
	"time"
)

func TestSeguidasBloqueiaNaDecima(t *testing.T) {
	agora := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	s := NovaSeguidas(10, 15*time.Minute, 100, func() time.Time { return agora })
	for i := 1; i <= 9; i++ {
		if s.Falhar("a") {
			t.Fatalf("bloqueou na %dª", i)
		}
	}
	if s.Bloqueada("a") || !s.Falhar("a") || !s.Bloqueada("a") {
		t.Fatal("deveria bloquear na 10ª")
	}
	if s.Bloqueada("b") {
		t.Fatal("outra chave bloqueada")
	}
	agora = agora.Add(15*time.Minute + time.Second)
	if s.Bloqueada("a") || s.Falhar("a") {
		t.Fatal("o bloqueio deveria ter acabado e o contador zerado")
	}
}

func TestSeguidasZerar(t *testing.T) {
	s := NovaSeguidas(2, time.Minute, 100, nil)
	s.Falhar("a")
	s.Zerar("a")
	if s.Falhar("a") {
		t.Fatal("Zerar não esqueceu")
	}
}

func TestSeguidasLimitada(t *testing.T) {
	agora := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	s := NovaSeguidas(10, time.Minute, 50, func() time.Time { return agora })
	for i := 0; i < 1000; i++ {
		s.Falhar(fmt.Sprintf("k%d", i))
	}
	if n := s.tamanho(); n > 50 {
		t.Fatalf("guardou %d chaves, máximo 50", n)
	}
}
