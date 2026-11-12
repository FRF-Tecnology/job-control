package api

import "testing"

func TestJornadaPelaAPI(t *testing.T) {
	f := novaFila(t)
	pacote := f.chamar("POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": conteudoTeste()}, 201)
	f.chamar("POST", "/vagas/li-1/devolver-ao-radar", Objeto{}, 200)
	if c := f.chamar("GET", "/candidaturas/"+pacote["id"].(string), nil, 200); c["estado"] != "cancelada" {
		t.Fatal(c)
	}
	f.chamar("POST", "/vagas/li-1/mover", Objeto{"etapa": "enviada"}, 200)
	dados := Objeto{"etapaAntes": "Enviada", "notasAntes": "", "etapa": "Proposta", "notas": "Analisar contrato"}
	f.chamar("POST", "/vagas/li-1/acompanhamento", dados, 200)
	dados["etapa"] = "Entrevista"
	f.chamar("POST", "/vagas/li-1/acompanhamento", dados, 409)
	if r := f.comoAssistente("POST", "/vagas/li-1/acompanhamento", dados); r.Status != 403 {
		t.Fatalf("assistente: %v", r)
	}
}
