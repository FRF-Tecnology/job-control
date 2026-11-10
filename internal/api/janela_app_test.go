package api

import (
	"context"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/janela"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// Janela do app de PC (contrato 0.15.0): as escolhas da pessoa no Perfil. A janela de verdade é do app; aqui os
// ganchos são falsos e só contam o que a API pediu.

type janelaAppFalsa struct {
	ligado   bool   // o que o "app" está fazendo agora
	pedidos  []bool // cada chamada de SegundoPlano
	padrao   int    // vezes que TamanhoPadrao foi chamado
	semIcone bool   // o sistema não deixa criar o ícone: ligar falha
}

func (j *janelaAppFalsa) ganchos() *JanelaApp {
	return &JanelaApp{
		SegundoPlano: func(ligar bool) bool {
			j.pedidos = append(j.pedidos, ligar)
			if ligar && j.semIcone {
				return false
			}
			j.ligado = ligar
			return true
		},
		TamanhoPadrao: func() { j.padrao++ },
	}
}

func apiComJanelaApp(t *testing.T, j *janelaAppFalsa) (*API, *armazem.Armazem) {
	t.Helper()
	arm, err := armazem.Abrir(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	o := Opcoes{Hoje: func() string { return "2026-11-10" }, Cofre: segredos.Memoria{}, ProvedoresIA: map[string]ia.Provedor{},
		DetectarCLIs: func(context.Context) []ia.EstadoCLI { return nil }}
	if j != nil {
		o.JanelaApp = j.ganchos()
	}
	return Nova(arm, o), arm
}

func segundoPlanoNoBanco(t *testing.T, arm *armazem.Armazem) (valor, tem bool) {
	t.Helper()
	tem, err := arm.LerMeta(context.Background(), janela.ChaveSegundoPlano, &valor)
	if err != nil {
		t.Fatal(err)
	}
	return valor, tem
}

func TestJanelaAppSegundoPlano(t *testing.T) {
	j := &janelaAppFalsa{}
	a, arm := apiComJanelaApp(t, j)
	// Padrão: desligado. Ler é de qualquer autor.
	r := chamarEnvio(t, a, "GET", "/app/janela", nil, "claude")
	o, _ := r.Corpo.(map[string]any)
	if r.Status != 200 || o["disponivel"] != true || o["segundoPlano"] != false || len(o) != 2 {
		t.Fatalf("GET inicial = %d %v", r.Status, o)
	}
	// A pessoa liga: o app é avisado, a escolha fica no banco e a resposta já traz o novo estado.
	r = chamarEnvio(t, a, "PUT", "/app/janela", Objeto{"segundoPlano": true}, "eu")
	o, _ = r.Corpo.(map[string]any)
	if r.Status != 200 || o["segundoPlano"] != true || o["disponivel"] != true || len(o) != 2 {
		t.Fatalf("ligar = %d %v", r.Status, o)
	}
	if v, tem := segundoPlanoNoBanco(t, arm); !tem || !v || !j.ligado || len(j.pedidos) != 1 {
		t.Fatalf("depois de ligar: banco %v/%v, app %v, pedidos %v", v, tem, j.ligado, j.pedidos)
	}
	if o, _ := chamarEnvio(t, a, "GET", "/app/janela", nil, "eu").Corpo.(map[string]any); o["segundoPlano"] != true {
		t.Fatalf("GET depois de ligar = %v", o)
	}
	// E desliga.
	r = chamarEnvio(t, a, "PUT", "/app/janela", Objeto{"segundoPlano": false}, "eu")
	o, _ = r.Corpo.(map[string]any)
	if v, _ := segundoPlanoNoBanco(t, arm); r.Status != 200 || o["segundoPlano"] != false || v || j.ligado {
		t.Fatalf("desligar = %d %v; banco %v, app %v", r.Status, o, v, j.ligado)
	}
}

func TestJanelaAppSoAPessoa(t *testing.T) {
	j := &janelaAppFalsa{}
	a, arm := apiComJanelaApp(t, j)
	// O assistente (ou qualquer autor que não seja a pessoa) não mexe na janela.
	for _, autor := range []string{"claude", "assistente"} {
		if r := chamarEnvio(t, a, "PUT", "/app/janela", Objeto{"segundoPlano": true}, autor); r.Status != 403 {
			t.Errorf("PUT por %q = %d %v", autor, r.Status, r.Corpo)
		}
		if r := chamarEnvio(t, a, "POST", "/app/janela/padrao", Objeto{}, autor); r.Status != 403 {
			t.Errorf("POST padrao por %q = %d %v", autor, r.Status, r.Corpo)
		}
	}
	if _, tem := segundoPlanoNoBanco(t, arm); tem || len(j.pedidos) != 0 || j.padrao != 0 {
		t.Fatalf("recusa mudou algo: banco %v, pedidos %v, padrao %d", tem, j.pedidos, j.padrao)
	}
}

func TestJanelaAppCorpoInvalido(t *testing.T) {
	j := &janelaAppFalsa{}
	a, arm := apiComJanelaApp(t, j)
	for nome, corpo := range map[string]Objeto{
		"vazio":        {},
		"texto":        {"segundoPlano": "sim"},
		"número":       {"segundoPlano": 1},
		"nulo":         {"segundoPlano": nil},
		"campo a mais": {"segundoPlano": true, "largura": 100},
	} {
		if r := chamarEnvio(t, a, "PUT", "/app/janela", corpo, "eu"); r.Status != 400 {
			t.Errorf("PUT %s = %d %v", nome, r.Status, r.Corpo)
		}
	}
	if r := chamarEnvio(t, a, "POST", "/app/janela/padrao", Objeto{"largura": 100}, "eu"); r.Status != 400 {
		t.Errorf("POST padrao com campo = %d %v", r.Status, r.Corpo)
	}
	if _, tem := segundoPlanoNoBanco(t, arm); tem || len(j.pedidos) != 0 || j.padrao != 0 {
		t.Fatalf("pedido inválido mudou algo: banco %v, pedidos %v, padrao %d", tem, j.pedidos, j.padrao)
	}
}

func TestJanelaAppSemIcone(t *testing.T) {
	j := &janelaAppFalsa{semIcone: true}
	a, arm := apiComJanelaApp(t, j)
	r := chamarEnvio(t, a, "PUT", "/app/janela", Objeto{"segundoPlano": true}, "eu")
	o, _ := r.Corpo.(map[string]any)
	if r.Status != 409 || o["motivo"] != "sem-bandeja" || o["erro"] == "" || len(o) != 2 {
		t.Fatalf("sem ícone = %d %v", r.Status, o)
	}
	// Nada fica gravado: senão a tela diria que está ligado e o app fecharia junto com a janela.
	if _, tem := segundoPlanoNoBanco(t, arm); tem || j.ligado {
		t.Fatalf("sem ícone gravou: banco %v, app %v", tem, j.ligado)
	}
	if o, _ := chamarEnvio(t, a, "GET", "/app/janela", nil, "eu").Corpo.(map[string]any); o["segundoPlano"] != false {
		t.Fatalf("GET depois da recusa = %v", o)
	}
	// Desligar sempre dá certo.
	if r := chamarEnvio(t, a, "PUT", "/app/janela", Objeto{"segundoPlano": false}, "eu"); r.Status != 200 {
		t.Fatalf("desligar sem ícone = %d %v", r.Status, r.Corpo)
	}
}

func TestJanelaAppTamanhoPadrao(t *testing.T) {
	j := &janelaAppFalsa{}
	a, arm := apiComJanelaApp(t, j)
	arm.GravarMeta(context.Background(), janela.ChaveSegundoPlano, true)
	r := chamarEnvio(t, a, "POST", "/app/janela/padrao", Objeto{}, "eu")
	o, _ := r.Corpo.(map[string]any)
	// Voltar ao tamanho padrão não mexe no segundo plano.
	if r.Status != 200 || j.padrao != 1 || o["segundoPlano"] != true || len(j.pedidos) != 0 {
		t.Fatalf("padrao = %d %v; chamadas %d, pedidos %v", r.Status, o, j.padrao, j.pedidos)
	}
}

// Fora do app de PC (central-dev, MCP, CLI) não há janela: o GET diz que não há e as outras duas dão 404, mesmo que
// o banco tenha a escolha gravada por um app que já rodou nessa pasta.
func TestJanelaAppSemJanela(t *testing.T) {
	a, arm := apiComJanelaApp(t, nil)
	arm.GravarMeta(context.Background(), janela.ChaveSegundoPlano, true)
	r := chamarEnvio(t, a, "GET", "/app/janela", nil, "eu")
	o, _ := r.Corpo.(map[string]any)
	if r.Status != 200 || o["disponivel"] != false || o["segundoPlano"] != false || len(o) != 2 {
		t.Fatalf("GET sem janela = %d %v", r.Status, o)
	}
	for _, c := range []struct {
		m, p  string
		corpo Objeto
	}{{"PUT", "/app/janela", Objeto{"segundoPlano": false}}, {"POST", "/app/janela/padrao", Objeto{}}} {
		r := chamarEnvio(t, a, c.m, c.p, c.corpo, "eu")
		o, _ := r.Corpo.(map[string]any)
		if r.Status != 404 || o["erro"] != msgSemJanela {
			t.Errorf("%s %s sem janela = %d %v", c.m, c.p, r.Status, o)
		}
	}
	if v, _ := segundoPlanoNoBanco(t, arm); !v {
		t.Fatal("o pedido recusado mudou a escolha gravada")
	}
	// Métodos fora do contrato.
	for _, c := range [][2]string{{"POST", "/app/janela"}, {"DELETE", "/app/janela"}, {"GET", "/app/janela/padrao"}, {"PUT", "/app/janela/padrao"}} {
		if r := chamarEnvio(t, a, c[0], c[1], Objeto{}, "eu"); r.Status != 404 {
			t.Errorf("%s %s = %d", c[0], c[1], r.Status)
		}
	}
}
