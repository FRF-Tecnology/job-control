package api

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// "Ler a descrição da vaga" com o navegador falso. Dados fictícios.

type leitorDeVaga struct {
	pedidos  []string
	texto    string
	bloqueio string
}

func (l *leitorDeVaga) ler(_ context.Context, plataforma, endereco string) (string, string, error) {
	l.pedidos = append(l.pedidos, plataforma+" "+endereco)
	return l.texto, l.bloqueio, nil
}

func apiDoRadar(t *testing.T, leitor *leitorDeVaga) (*API, *provedorDaRodada) {
	t.Helper()
	ctx := context.Background()
	arm, err := armazem.Abrir(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	prov := &provedorDaRodada{}
	o := Opcoes{Cofre: segredos.Memoria{"anthropic": "chave-ficticia"}, ProvedoresIA: map[string]ia.Provedor{"anthropic": prov},
		DetectarCLIs: func(context.Context) []ia.EstadoCLI { return nil }}
	if leitor != nil {
		o.LerDescricao = leitor.ler
	}
	a := Nova(arm, o)
	if err := arm.GravarMeta(ctx, chaveConfig, ia.Config{Rotas: map[string]ia.Rota{"padrao": {Provedor: "anthropic", Modelo: "teste"}}}); err != nil {
		t.Fatal(err)
	}
	hoje := a.relogio().Format("2006-01-02")
	err = arm.Gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		e.Perfil = Objeto{"cargo": "Engenheira de software", "paisTrabalho": "BR", "modo": "direto"}
		e.Vagas = append(e.Vagas,
			Objeto{"id": "linkedin-4012345678", "anuncioId": "linkedin-4012345678", "titulo": "Engenheira de Software Go", "empresa": "Acme",
				"link": "https://www.linkedin.com/jobs/view/4012345678/", "plataforma": "linkedin", "etapa": "descoberta",
				"triagemBusca": "pendente", "publicadaEm": hoje, "fonteBusca": "Alerta do LinkedIn (e-mail)"},
			Objeto{"id": "indeed-1a2b3c4d5e6f7a8b", "titulo": "Analista", "empresa": "Beta", "link": "https://br.indeed.com/viewjob?jk=1a2b3c4d5e6f7a8b",
				"plataforma": "outra", "etapa": "descoberta", "triagemBusca": "pendente"})
		return e, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return a, prov
}

func vagaDoRadar(t *testing.T, a *API, id string) Objeto {
	t.Helper()
	e, _ := a.arm.Carregar(context.Background())
	for _, v := range e.Vagas {
		if v["id"] == id {
			return v
		}
	}
	t.Fatalf("vaga %s sumiu", id)
	return nil
}

func TestLerDescricaoDaVaga(t *testing.T) {
	leitor := &leitorDeVaga{texto: "  Sobre a vaga:\n\nGo, <script>x</script> PostgreSQL e Kubernetes.  \n"}
	a, prov := apiDoRadar(t, leitor)
	rota := "/vagas/linkedin-4012345678/ler-descricao"

	// Só a pessoa.
	if r := chamar(t, a, "POST", rota, Objeto{}, "claude"); r.Status != 403 || len(leitor.pedidos) != 0 {
		t.Fatalf("assistente: %d, abriu %d páginas", r.Status, len(leitor.pedidos))
	}
	// Corpo estrito.
	if r := chamar(t, a, "POST", rota, Objeto{"url": "https://golpe.example"}, "eu"); r.Status != 400 || len(leitor.pedidos) != 0 {
		t.Fatalf("corpo com campo: %d", r.Status)
	}
	// Só LinkedIn.
	if r := chamar(t, a, "POST", "/vagas/indeed-1a2b3c4d5e6f7a8b/ler-descricao", Objeto{}, "eu"); r.Status != 409 || !strings.Contains(fmt.Sprint(r.Corpo), "plataforma") || len(leitor.pedidos) != 0 {
		t.Fatalf("indeed: %d %v", r.Status, r.Corpo)
	}

	// Login: nada muda na vaga.
	leitor.bloqueio = dominio.EnderecoLogin
	if r := chamar(t, a, "POST", rota, Objeto{}, "eu"); r.Status != 409 || !strings.Contains(fmt.Sprint(r.Corpo), "login") {
		t.Fatalf("login: %d %v", r.Status, r.Corpo)
	}
	if v := vagaDoRadar(t, a, "linkedin-4012345678"); v["descricao"] != nil || v["triagemBusca"] != "pendente" {
		t.Fatalf("login mudou a vaga: %v", v)
	}
	// A página parou em outro lugar.
	leitor.bloqueio = dominio.EnderecoOutroHost
	if r := chamar(t, a, "POST", rota, Objeto{}, "eu"); r.Status != 409 || !strings.Contains(fmt.Sprint(r.Corpo), "fora") {
		t.Fatalf("outro host: %d %v", r.Status, r.Corpo)
	}

	// Lê, grava a descrição limpa e a IA avalia na hora.
	leitor.bloqueio = ""
	r := chamar(t, a, "POST", rota, Objeto{}, "eu")
	if r.Status != 200 {
		t.Fatalf("ler: %d %v", r.Status, r.Corpo)
	}
	if res := r.Corpo.(Objeto); res["avaliada"] != true || res["vaga"].(Objeto)["aderencia"] != "B" {
		t.Fatalf("não avaliou ou devolveu a vaga de antes da nota: %v", res)
	}
	if ultimo := leitor.pedidos[len(leitor.pedidos)-1]; ultimo != "linkedin https://www.linkedin.com/jobs/view/4012345678/" {
		t.Fatalf("abriu %q", ultimo)
	}
	v := vagaDoRadar(t, a, "linkedin-4012345678")
	if v["descricao"] != "Sobre a vaga: Go, scriptx/script PostgreSQL e Kubernetes." {
		t.Errorf("descrição: %q", v["descricao"])
	}
	if v["triagemBusca"] == "pendente" || v["aderencia"] != "B" || v["etapa"] != "descoberta" || prov.chamadas != 1 {
		t.Errorf("avaliação: triagem %v, nota %v, etapa %v, IA %d", v["triagemBusca"], v["aderencia"], v["etapa"], prov.chamadas)
	}
	if !strings.Contains(prov.ultima, "PostgreSQL") {
		t.Error("a descrição não foi para a triagem")
	}
}

func TestLerDescricaoSemEnvio(t *testing.T) {
	a, _ := apiDoRadar(t, nil)
	if r := chamar(t, a, "POST", "/vagas/linkedin-4012345678/ler-descricao", Objeto{}, "eu"); r.Status != 404 {
		t.Fatalf("sem o envio: %d", r.Status)
	}
}

func TestLerDescricaoSemIAFicaPendente(t *testing.T) {
	leitor := &leitorDeVaga{texto: "Descrição fictícia com o bastante para ler."}
	a, prov := apiDoRadar(t, leitor)
	prov.falhar = true
	r := chamar(t, a, "POST", "/vagas/linkedin-4012345678/ler-descricao", Objeto{}, "eu")
	if r.Status != 200 || r.Corpo.(Objeto)["avaliada"] != false {
		t.Fatalf("%d %v", r.Status, r.Corpo)
	}
	if v := vagaDoRadar(t, a, "linkedin-4012345678"); v["descricao"] == nil || v["triagemBusca"] != "pendente" {
		t.Fatalf("a vaga devia ficar pendente com a descrição: %v", v)
	}
}
