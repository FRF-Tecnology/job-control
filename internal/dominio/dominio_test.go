package dominio

import (
	"regexp"
	"testing"
)

// O golden fixa o sufixo aleatório dos ids. Aqui o sufixo de verdade.
func TestIdsDeEventoEPedidoTemSufixoAleatorio(t *testing.T) {
	ctx := Ctx{Autor: "eu", Agora: "2026-11-08"}
	e, _, err := AdicionarVaga(EstadoVazio(), Objeto{"id": "li-1", "empresa": "Acme", "titulo": "Dev React"}, ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	e, p, err := CriarPedido(e, "buscar vagas", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if id := e.Eventos[0]["id"].(string); !regexp.MustCompile(`^ev-2026-11-08-000001-[0-9a-z]{4}$`).MatchString(id) {
		t.Errorf("id do evento = %q", id)
	}
	if id := p["id"].(string); !regexp.MustCompile(`^pd-2026-11-08-0001-[0-9a-z]{4}$`).MatchString(id) {
		t.Errorf("id do pedido = %q", id)
	}
	if e.Vagas[0]["contaId"] != "local" {
		t.Errorf("contaId padrão = %v; queria local", e.Vagas[0]["contaId"])
	}
}

// A API responde 404 só para "não encontrado"; validação e duplicata são 400/409.
func TestTiposDeErro(t *testing.T) {
	ctx := Ctx{Agora: "2026-11-08"}
	_, _, err := MoverVaga(EstadoVazio(), "nao-existe", "aplicar", nil, ctx)
	if !EhNaoEncontrado(err) {
		t.Errorf("mover vaga inexistente: %v não é ErroNaoEncontrado", err)
	}
	_, _, err = MoverVaga(EstadoVazio(), "x", "voando", nil, ctx)
	if EhNaoEncontrado(err) {
		t.Errorf("etapa desconhecida não deveria ser 'não encontrado'")
	}
}

// Nada do estado novo pode apontar para o estado antigo (as operações são puras).
func TestEstadoNovoNaoCompartilhaDadosComOAntigo(t *testing.T) {
	ctx := Ctx{Agora: "2026-11-08"}
	e1, _, _ := AdicionarVaga(EstadoVazio(), Objeto{"id": "li-1", "empresa": "Acme", "titulo": "Dev", "stack": []any{"Go"}}, ctx, false)
	e2, _, _ := MoverVaga(e1, "li-1", "enviada", Objeto{"envio": Objeto{"rodada": float64(1)}}, ctx)
	e2.Vagas[0]["stack"].([]any)[0] = "mudou"
	obj(e2.Vagas[0]["envio"])["rodada"] = float64(9)
	if e1.Vagas[0]["stack"].([]any)[0] != "Go" || e1.Vagas[0]["envio"] != nil {
		t.Fatal("mexer no estado novo alterou o antigo")
	}
}
