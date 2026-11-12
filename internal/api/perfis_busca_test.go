package api

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/busca"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

func TestPerfisDeBuscaIsolamResultadosECurriculosMesmoComOMesmoAnuncio(t *testing.T) {
	ctx := context.Background()
	banco := filepath.Join(t.TempDir(), "dados.db")
	abrir := func() (*armazem.Armazem, *API) {
		arm, err := armazem.Abrir(ctx, banco)
		if err != nil {
			t.Fatal(err)
		}
		a := Nova(arm, Opcoes{Cofre: segredos.Memoria{"anthropic": "ficticia"}, ProvedoresIA: map[string]ia.Provedor{"anthropic": &provedorDaRodada{}}})
		a.consultarVagas = func(context.Context, Objeto) busca.Coleta {
			return busca.Coleta{Vagas: []Objeto{{"id": "vaga-comum", "titulo": "Software Engineer", "empresa": "Empresa Fictícia", "fonteBusca": "Teste", "descricao": "Java e Machine Learning"}}}
		}
		return arm, a
	}
	arm, a := abrir()
	defer func() { arm.Fechar() }()
	if err := arm.GravarMeta(ctx, chaveConfig, ia.Config{Rotas: map[string]ia.Rota{"padrao": {Provedor: "anthropic", Modelo: "teste"}}}); err != nil {
		t.Fatal(err)
	}
	chamar := func(status int, metodo, caminho string, corpo Objeto) any {
		t.Helper()
		r, err := a.Chamar(ctx, Pedido{Metodo: metodo, Caminho: Prefixo + caminho, Corpo: corpo, Autor: "eu"})
		if err != nil || r.Status != status {
			t.Fatalf("%s %s: %v %v", metodo, caminho, r, err)
		}
		return r.Corpo
	}
	chamar(200, "PUT", "/perfil", Objeto{"cargo": "AI Engineer", "nome": "Pessoa Fictícia", "curriculos": Objeto{"PT": "curriculos/inicial.pdf"}, "preferenciaLegada": "preservar"})
	chamar(200, "POST", "/busca", Objeto{})
	chamar(201, "POST", "/perfis-busca", Objeto{"modelo": "java", "nome": "Java Engineer", "perfilOrigem": "original"})
	estado := chamar(200, "GET", "/estado", nil).(Objeto)
	java := estado["perfilBuscaAtivo"].(string)
	if java == "" || java == "original" || estado["perfil"].(Objeto)["cargo"] != "Java Engineer" {
		t.Fatal(estado)
	}
	perfilJava := estado["perfil"].(Objeto)
	if dominio.ObjetoDe(perfilJava["curriculos"])["PT"] != "curriculos/inicial.pdf" {
		t.Fatal(perfilJava)
	}
	chamar(200, "PATCH", "/perfil?perfilBuscaId="+java, Objeto{"curriculos": Objeto{"PT": "curriculos/java.pdf"}, "cargo": "Backend Java"})
	chamar(409, "PATCH", "/perfil?perfilBuscaId=original", Objeto{"cargo": "não salvar"})
	chamar(409, "POST", "/perfis-busca", Objeto{"modelo": "java", "nome": "jáva engineer", "perfilOrigem": java})
	chamar(200, "POST", "/busca", Objeto{"perfilBuscaId": java})
	arm.Fechar()
	arm, a = abrir()
	estado = chamar(200, "GET", "/estado", nil).(Objeto)
	if estado["perfilBuscaAtivo"] != java || estado["perfil"].(Objeto)["cargo"] != "Backend Java" {
		t.Fatal(estado)
	}
	e, err := arm.Carregar(ctx)
	if err != nil || len(e.Vagas) != 2 || len(e.Rodadas) != 2 || e.Vagas[0]["id"] == e.Vagas[1]["id"] || e.Vagas[1]["perfilBuscaId"] != java || e.Rodadas[1]["perfilBuscaId"] != java {
		t.Fatalf("vagas=%v rodadas=%v err=%v", e.Vagas, e.Rodadas, err)
	}
	chamar(200, "POST", "/perfis-busca/original/ativar", Objeto{})
	perfil := chamar(200, "GET", "/perfil", nil).(Objeto)
	if perfil["preferenciaLegada"] != "preservar" || perfil["cargo"] != "AI Engineer" || dominio.ObjetoDe(perfil["curriculos"])["PT"] != "curriculos/inicial.pdf" {
		t.Fatal(perfil)
	}
	chamar(409, "POST", "/busca", Objeto{"perfilBuscaId": java})
	chamar(200, "POST", "/vagas/vaga-comum/mover", Objeto{"etapa": "aplicar"})
	chamar(409, "POST", "/vagas/"+dominio.Texto(e.Vagas[1]["id"])+"/mover", Objeto{"etapa": "aplicar"})
}

func TestResultadoDeBuscaEmAndamentoNaoEntraNoPerfilNovoComOsMesmosCriterios(t *testing.T) {
	ctx := context.Background()
	arm, err := armazem.Abrir(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer arm.Fechar()
	a := Nova(arm, Opcoes{Cofre: segredos.Memoria{"anthropic": "ficticia"}, ProvedoresIA: map[string]ia.Provedor{"anthropic": &provedorDaRodada{}}})
	if err := arm.GravarMeta(ctx, chaveConfig, ia.Config{Rotas: map[string]ia.Rota{"padrao": {Provedor: "anthropic", Modelo: "teste"}}}); err != nil {
		t.Fatal(err)
	}
	criar := func(nome string) {
		t.Helper()
		r, err := a.Chamar(ctx, Pedido{Metodo: "POST", Caminho: Prefixo + "/perfis-busca", Corpo: Objeto{"nome": nome, "modelo": "java"}})
		if err != nil || r.Status != 201 {
			t.Fatalf("%v %v", r, err)
		}
	}
	criar("Java A")
	a.consultarVagas = func(context.Context, Objeto) busca.Coleta {
		criar("Java B")
		return busca.Coleta{Vagas: []Objeto{{"id": "j", "empresa": "Fictícia", "titulo": "Java"}}}
	}
	r, err := a.Chamar(ctx, Pedido{Metodo: "POST", Caminho: Prefixo + "/busca", Corpo: Objeto{}})
	e, lerErr := arm.Carregar(ctx)
	if err != nil || r.Status != 409 || lerErr != nil || len(e.Vagas) != 0 || len(e.Rodadas) != 0 {
		t.Fatalf("%v %v %v %v", r, err, lerErr, e)
	}
}
