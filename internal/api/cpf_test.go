package api

import (
	"fmt"
	"strings"
	"testing"
)

// CPF da pessoa: só no cofre, só da tela, só mascarado; no preencher, só depois de a pessoa confirmar. O CPF é
// gerado só para teste (dígitos verificadores calculados), de ninguém.
const cpfDeTeste = "529.982.247-25"

func TestCPFNoCofre(t *testing.T) {
	a := apiDePreencher(t, nil)
	if r := chamar(t, a, "PUT", "/perfil/cpf", Objeto{"cpf": cpfDeTeste}, "claude"); r.Status != 403 {
		t.Fatalf("assistente: %d", r.Status)
	}
	if r := chamar(t, a, "GET", "/perfil/cpf", nil, "eu"); r.Status != 200 || r.Corpo.(Objeto)["salvo"] != false {
		t.Fatalf("sem CPF: %d %v", r.Status, r.Corpo)
	}
	for _, ruim := range []any{"529.982.247-24", "111.111.111-11", "123", 52998224725} {
		if r := chamar(t, a, "PUT", "/perfil/cpf", Objeto{"cpf": ruim}, "eu"); r.Status != 400 {
			t.Fatalf("CPF inválido %v: %d", ruim, r.Status)
		}
	}
	r := chamar(t, a, "PUT", "/perfil/cpf", Objeto{"cpf": cpfDeTeste}, "eu")
	if r.Status != 200 || r.Corpo.(Objeto)["mascarado"] != "***.982.247-**" {
		t.Fatalf("gravar: %d %v", r.Status, r.Corpo)
	}
	// Só os dígitos no cofre; nunca no banco nem em resposta nenhuma.
	if v, _ := a.cofre.Ler(chaveCofreCPF); v != "52998224725" {
		t.Fatalf("cofre: %q", v)
	}
	if r := chamar(t, a, "GET", "/estado", nil, "eu"); strings.Contains(fmt.Sprint(r.Corpo), "52998224725") || strings.Contains(fmt.Sprint(r.Corpo), cpfDeTeste) {
		t.Fatal("o CPF apareceu no estado")
	}
	if r := chamar(t, a, "GET", "/perfil/cpf", nil, "eu"); r.Corpo.(Objeto)["mascarado"] != "***.982.247-**" {
		t.Fatalf("ler: %v", r.Corpo)
	}
	if r := chamar(t, a, "DELETE", "/perfil/cpf", nil, "eu"); r.Status != 200 || r.Corpo.(Objeto)["salvo"] != false {
		t.Fatalf("apagar: %d %v", r.Status, r.Corpo)
	}
	if v, _ := a.cofre.Ler(chaveCofreCPF); v != "" {
		t.Fatal("o CPF ficou no cofre")
	}
}

func TestPreencherCPFSoDepoisDeConfirmar(t *testing.T) {
	j := &janelaDePreencher{host: "carreiras.acme.com.br", pedeCPF: 1}
	a := apiDePreencher(t, j)
	chamar(t, a, "POST", "/vagas/linkedin-1/abrir-para-preencher", Objeto{}, "eu")

	// Sem CPF salvo: a resposta avisa que o site pede, e o CPF não vai.
	r := chamar(t, a, "POST", "/vagas/linkedin-1/preencher", Objeto{"dominio": "carreiras.acme.com.br"}, "eu")
	if res := r.Corpo.(Objeto); r.Status != 200 || res["pedeCPF"] != 1 || res["cpfSalvo"] != false || j.conteudo["cpf"] != nil {
		t.Fatalf("sem CPF salvo: %d %v %v", r.Status, r.Corpo, j.conteudo["cpf"])
	}
	if r := chamar(t, a, "POST", "/vagas/linkedin-1/preencher", Objeto{"dominio": "carreiras.acme.com.br", "cpf": true}, "eu"); r.Status != 409 || !strings.Contains(fmt.Sprint(r.Corpo), "sem-cpf") {
		t.Fatalf("cpf sem salvar: %d %v", r.Status, r.Corpo)
	}

	// Com o CPF salvo: o primeiro preencher continua sem CPF; só o pedido com cpf: true leva, e só ele.
	chamar(t, a, "PUT", "/perfil/cpf", Objeto{"cpf": cpfDeTeste}, "eu")
	r = chamar(t, a, "POST", "/vagas/linkedin-1/preencher", Objeto{"dominio": "carreiras.acme.com.br"}, "eu")
	if res := r.Corpo.(Objeto); res["cpfSalvo"] != true || j.conteudo["cpf"] != nil {
		t.Fatalf("primeiro passo: %v %v", r.Corpo, j.conteudo["cpf"])
	}
	for _, ruim := range []any{false, "sim", 1} {
		if r := chamar(t, a, "POST", "/vagas/linkedin-1/preencher", Objeto{"dominio": "carreiras.acme.com.br", "cpf": ruim}, "eu"); r.Status != 400 {
			t.Fatalf("cpf %v: %d", ruim, r.Status)
		}
	}
	if r := chamar(t, a, "POST", "/vagas/linkedin-1/preencher", Objeto{"dominio": "carreiras.acme.com.br", "cpf": true}, "claude"); r.Status != 403 {
		t.Fatalf("assistente: %d", r.Status)
	}
	r = chamar(t, a, "POST", "/vagas/linkedin-1/preencher", Objeto{"dominio": "carreiras.acme.com.br", "cpf": true}, "eu")
	if r.Status != 200 || j.conteudo["cpf"] != cpfDeTeste || j.conteudo["soCPF"] != true {
		t.Fatalf("cpf: %d %v %v", r.Status, r.Corpo, j.conteudo["cpf"])
	}
	if strings.Contains(fmt.Sprint(r.Corpo), "982") {
		t.Fatal("o CPF voltou na resposta")
	}
	// O CPF nunca entra no pacote da candidatura reservada.
	e, _ := a.arm.Carregar(t.Context())
	for _, c := range e.Candidaturas {
		if strings.Contains(fmt.Sprint(c), "52998224725") || strings.Contains(fmt.Sprint(c), cpfDeTeste) {
			t.Fatal("o CPF foi para o pacote")
		}
	}
}

func TestCPFAutomatico(t *testing.T) {
	j := &janelaDePreencher{host: "carreiras.acme.com.br", pedeCPF: 1}
	a := apiDePreencher(t, j)
	if r := chamar(t, a, "PATCH", "/perfil/cpf", Objeto{"automatico": true}, "eu"); r.Status != 409 {
		t.Fatalf("ligar sem CPF: %d", r.Status)
	}
	chamar(t, a, "PUT", "/perfil/cpf", Objeto{"cpf": cpfDeTeste}, "eu")
	if r := chamar(t, a, "PATCH", "/perfil/cpf", Objeto{"automatico": true}, "claude"); r.Status != 403 {
		t.Fatalf("assistente: %d", r.Status)
	}
	if r := chamar(t, a, "PATCH", "/perfil/cpf", Objeto{"automatico": "sim"}, "eu"); r.Status != 400 {
		t.Fatalf("valor errado: %d", r.Status)
	}
	r := chamar(t, a, "PATCH", "/perfil/cpf", Objeto{"automatico": true}, "eu")
	if r.Status != 200 || r.Corpo.(Objeto)["automatico"] != true || r.Corpo.(Objeto)["mascarado"] != "***.982.247-**" {
		t.Fatalf("ligar: %d %v", r.Status, r.Corpo)
	}

	// Ligado: o CPF vai já no Preencher (o domínio conferido), sem a pergunta; a resposta avisa.
	chamar(t, a, "POST", "/vagas/linkedin-1/abrir-para-preencher", Objeto{}, "eu")
	r = chamar(t, a, "POST", "/vagas/linkedin-1/preencher", Objeto{"dominio": "carreiras.acme.com.br"}, "eu")
	if res := r.Corpo.(Objeto); r.Status != 200 || res["comCPF"] != true || res["pedeCPF"] != nil || j.conteudo["cpf"] != cpfDeTeste || j.conteudo["soCPF"] != nil {
		t.Fatalf("automático: %d %v %v", r.Status, r.Corpo, j.conteudo)
	}
	if strings.Contains(fmt.Sprint(r.Corpo), "982") {
		t.Fatal("o CPF voltou na resposta")
	}

	// Desligado: volta a perguntar.
	chamar(t, a, "PATCH", "/perfil/cpf", Objeto{"automatico": false}, "eu")
	r = chamar(t, a, "POST", "/vagas/linkedin-1/preencher", Objeto{"dominio": "carreiras.acme.com.br"}, "eu")
	if res := r.Corpo.(Objeto); res["pedeCPF"] != 1 || j.conteudo["cpf"] != nil {
		t.Fatalf("desligado: %v %v", r.Corpo, j.conteudo["cpf"])
	}

	// Apagar o CPF desliga o automático.
	chamar(t, a, "PATCH", "/perfil/cpf", Objeto{"automatico": true}, "eu")
	chamar(t, a, "DELETE", "/perfil/cpf", nil, "eu")
	chamar(t, a, "PUT", "/perfil/cpf", Objeto{"cpf": cpfDeTeste}, "eu")
	if r := chamar(t, a, "GET", "/perfil/cpf", nil, "eu"); r.Corpo.(Objeto)["automatico"] != false {
		t.Fatalf("depois de apagar: %v", r.Corpo)
	}
}
