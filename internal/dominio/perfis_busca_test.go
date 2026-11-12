package dominio

import "testing"

func TestModoDiretoRespeitaDevolucaoAoRadar(t *testing.T) {
	e := EstadoVazio()
	e.Perfil["modo"] = "direto"
	e.Vagas = []Objeto{
		{"id": "devolvida", "etapa": "descoberta", "aderencia": "A", "devolvidaAoRadar": true},
		{"id": "recomendada", "etapa": "descoberta", "aderencia": "A"},
	}
	fila := FilaAplicar(e)
	if len(fila) != 1 || fila[0]["id"] != "recomendada" {
		t.Fatal(fila)
	}
}
