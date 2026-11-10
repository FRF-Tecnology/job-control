package api

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// jc-a2y.7.5: PUT e PATCH /perfil recusam (400) resposta padrão para pergunta de número fiscal, documento, senha ou
// banco, seja quem for o autor, e nada é gravado. Sim/não passa, e o PATCH de outro campo não é travado pelo que já
// estava guardado. Dados fictícios.
func TestPerfilRecusaRespostaSensivel(t *testing.T) {
	ctx := context.Background()
	arm, err := armazem.Abrir(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer arm.Fechar()
	a := Nova(arm, Opcoes{Hoje: func() string { return "2026-11-10" }, Aleatorio: func() string { return "abcd" }, Cofre: segredos.Memoria{}})
	chamar := func(m string, corpo Objeto, autor string) Resposta {
		t.Helper()
		r, err := a.Chamar(ctx, Pedido{Metodo: m, Caminho: Prefixo + "/perfil", Corpo: normal(corpo), Autor: autor})
		if err != nil {
			t.Fatalf("%s /perfil: %v", m, err)
		}
		return r
	}
	perfil := func() string {
		e, err := arm.Carregar(ctx)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(e.Perfil)
		return string(b)
	}
	resposta := func(pergunta, valor string) Objeto {
		return Objeto{"respostasPadrao": []any{Objeto{"pergunta": pergunta, "resposta": valor}}}
	}
	base := Objeto{"nome": "Pessoa Exemplo", "respostasPadrao": []any{
		Objeto{"pergunta": "Anos com Go?", "resposta": "5"},
		Objeto{"pergunta": "Possui CNPJ para emitir nota?", "resposta": "Sim"},
	}}
	if r := chamar("PUT", base, "eu"); r.Status != 200 {
		t.Fatalf("perfil comum = %d %s", r.Status, emJSON(r.Corpo))
	}
	antes := perfil()

	for _, autor := range []string{"eu", "claude"} {
		for pergunta, valor := range map[string]string{
			"Agência e conta para depósito": "0001 / 12345-6",
			"Chave PIX":                     "pessoa.teste@exemplo.com",
			"Contraseña del portal":         "ficticia",
			"Número de documento":           "A000000",
		} {
			for _, m := range []string{"PUT", "PATCH"} {
				corpo := resposta(pergunta, valor)
				corpo["nome"] = "Pessoa Exemplo"
				r := chamar(m, corpo, autor)
				if r.Status != 400 || !strings.Contains(emJSON(r.Corpo), ErroRespostaSensivel) || !strings.Contains(emJSON(r.Corpo), pergunta) {
					t.Errorf("%s por %s com %q = %d %s", m, autor, pergunta, r.Status, emJSON(r.Corpo))
				}
			}
		}
	}
	if depois := perfil(); depois != antes {
		t.Fatalf("a recusa mudou o perfil:\nantes  %s\ndepois %s", antes, depois)
	}

	// Sim/não não é o dado; e o PATCH de outro campo segue passando.
	if r := chamar("PATCH", resposta("Tem conta bancária no exterior?", "Não"), "eu"); r.Status != 200 {
		t.Errorf("sim/não = %d %s", r.Status, emJSON(r.Corpo))
	}
	if r := chamar("PATCH", Objeto{"cidade": "Cidade Exemplo"}, "claude"); r.Status != 200 {
		t.Errorf("PATCH de outro campo = %d %s", r.Status, emJSON(r.Corpo))
	}
	// A pergunta longa volta cortada, numa linha só.
	longa := "Chave PIX " + strings.Repeat("muito longa ", 20)
	if r := chamar("PATCH", resposta(longa, "ficticia"), "eu"); r.Status != 400 || strings.Contains(emJSON(r.Corpo), longa) || !strings.Contains(emJSON(r.Corpo), "…") {
		t.Errorf("pergunta longa = %d %s", r.Status, emJSON(r.Corpo))
	}
}
