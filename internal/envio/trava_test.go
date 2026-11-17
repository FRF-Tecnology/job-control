package envio

import "testing"

// Trava das ações (Task 5): o executor só começa um ciclo quando podeAgir libera; nil libera (testes e central-dev).
func TestEnvioNaoComecaBloqueado(t *testing.T) {
	if !liberado(nil)() {
		t.Fatal("podeAgir nil devia liberar")
	}
	pode, motivo := false, "teste-acabou"
	f := liberado(func() (bool, string) { return pode, motivo })
	if f() {
		t.Fatal("podeAgir false liberou o ciclo")
	}
	pode, motivo = true, ""
	if !f() {
		t.Fatal("podeAgir true não liberou o ciclo")
	}
}
