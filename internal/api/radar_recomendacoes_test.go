package api

import (
	"context"
	"fmt"
	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/busca"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Mesmo um provedor que dê A a tudo não pode recomendar um falso positivo.
type recomendaTudo struct{ chamadas int }

func (*recomendaTudo) Nome() string { return "anthropic" }
func (p *recomendaTudo) Chamar(context.Context, ia.PedidoProvedor) (ia.RespostaProvedor, error) {
	p.chamadas++
	return ia.RespostaProvedor{Texto: `{"decisao":"aplicar","aderencia":"A","motivo":"Desenvolvimento de serviços."}`}, nil
}

func TestCargoLivreNaoHerdaSinonimosEGeografiaPermanecePorPerfil(t *testing.T) {
	e := dominio.EstadoVazio()
	e.Perfil = Objeto{"cargo": "Fisioterapeuta", "paisTrabalho": "BR", "cidadeTrabalho": "Recife", "titulosEquivalentes": []any{"Physical Therapist"}}
	novo, err := dominio.CriarPerfilComCargo(e, "enf", "Saúde em Portugal", "outro", "Enfermeira")
	if err != nil {
		t.Fatal(err)
	}
	if novo.Perfil["cargo"] != "Enfermeira" || len(dominio.ListaDe(novo.Perfil["titulosEquivalentes"])) > 0 {
		t.Fatal(novo.Perfil)
	}
	novo.Perfil["paisTrabalho"] = "PT"
	novo.Perfil["cidadeTrabalho"] = "Porto"
	anterior, err := dominio.AtivarPerfilBusca(novo, dominio.PerfilOriginal)
	if err != nil || anterior.Perfil["cidadeTrabalho"] != "Recife" || anterior.Perfil["paisTrabalho"] != "BR" {
		t.Fatal(anterior.Perfil, err)
	}
}

type transporteDoAnuncio func(*http.Request) (*http.Response, error)

func TestRevalidarJobicyPorID(t *testing.T) {
	for _, estado := range []string{"closed", "active", "unknown"} {
		chamou := false
		cliente := &http.Client{Transport: transporteDoAnuncio(func(r *http.Request) (*http.Response, error) {
			chamou = true
			if r.Method != "GET" || r.URL.String() != "https://jobicy.com/api/jobs/status?ids=123" {
				t.Fatal(r.Method, r.URL)
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"jobs":[{"id":123,"status":%q}]}`, estado))), Header: make(http.Header)}, nil
		})}
		err := revalidarAnuncioRadar(context.Background(), cliente, Objeto{"radarVersao": 1, "fonteBusca": "Jobicy", "link": "https://jobicy.com/jobs/123-exemplo"}, time.Now())
		if !chamou || (err == nil) != (estado == "active") {
			t.Fatalf("estado=%s err=%v", estado, err)
		}
	}
}

func (t transporteDoAnuncio) RoundTrip(r *http.Request) (*http.Response, error) { return t(r) }
func TestRevalidarAnuncioNaoAbreEncerradaNemSegueDestinoExterno(t *testing.T) {
	chamadas := 0
	cliente := &http.Client{Transport: transporteDoAnuncio(func(r *http.Request) (*http.Response, error) {
		chamadas++
		if r.URL.Hostname() != "exemplo.gupy.io" {
			t.Fatal(r.URL)
		}
		return &http.Response{StatusCode: 410, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}
	err := revalidarAnuncioRadar(context.Background(), cliente, Objeto{"radarVersao": 1, "link": "https://exemplo.gupy.io/jobs/123", "fonteBusca": "Gupy"}, time.Now())
	if err == nil || chamadas != 1 {
		t.Fatal(err, chamadas)
	}
	_ = revalidarAnuncioRadar(context.Background(), cliente, Objeto{"radarVersao": 1, "link": "https://empresa.example.invalid/vaga", "fonteBusca": "Gupy"}, time.Now())
	if chamadas != 1 {
		t.Fatal("consulta de endereço fora da fonte")
	}
}
func TestRadarFiltraAntesDaIARetomaLoteEIsolaGeografia(t *testing.T) {
	ctx := context.Background()
	arm, err := armazem.Abrir(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer arm.Fechar()
	p := &recomendaTudo{}
	a := Nova(arm, Opcoes{Cofre: segredos.Memoria{"anthropic": "ficticia"}, ProvedoresIA: map[string]ia.Provedor{"anthropic": p}})
	if err := arm.GravarMeta(ctx, chaveConfig, ia.Config{Rotas: map[string]ia.Rota{"padrao": {Provedor: "anthropic", Modelo: "teste"}}}); err != nil {
		t.Fatal(err)
	}
	if err := arm.Gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		e.Perfil = Objeto{"cargo": "Java Engineer", "stackPrincipal": []any{"Java", "Spring Boot"}, "paisTrabalho": "BR", "filtros": Objeto{"modelo": []any{"Remoto"}}}
		return e, nil
	}); err != nil {
		t.Fatal(err)
	}
	var amostra []Objeto
	vaga := func(i int, titulo, descricao string) Objeto {
		return Objeto{"id": fmt.Sprint(i), "link": fmt.Sprintf("https://vaga.example.invalid/%d", i), "empresa": "Produto Fictício", "titulo": titulo, "descricao": descricao, "nivel": "Sênior", "modelo": "Remoto", "local": "Worldwide", "remotoMundial": true, "mercado": "internacional", "fonteBusca": "Teste", "publicadaEm": a.relogio().UTC().Add(-time.Hour).Format(time.RFC3339)}
	}
	amostra = append(amostra, vaga(1, "Analista de Atendimento ao Cliente", "Atender clientes que usam Java."))
	for i := 2; i <= 13; i++ {
		amostra = append(amostra, vaga(i, "Senior Java Engineer", "Desenvolver APIs e serviços backend em Java e Spring Boot."))
	}
	a.consultarVagas = func(context.Context, Objeto) busca.Coleta { return busca.Coleta{Vagas: amostra} }
	rodar := func() balancoBusca {
		r, e := a.buscarETriar(ctx, "original")
		if e != nil {
			t.Fatal(e)
		}
		return r.Corpo.(balancoBusca)
	}
	primeira := rodar()
	if primeira.Grupos["A"] != 10 || primeira.Pendentes != 2 || p.chamadas != 10 {
		t.Fatalf("primeira rodada: %+v, IA=%d", primeira, p.chamadas)
	}
	amostra = nil // a próxima rodada não pode depender de receber outra vez as páginas.
	segunda := rodar()
	if segunda.Grupos["A"] != 2 || segunda.Pendentes != 0 {
		t.Fatalf("retomada: %+v", segunda)
	}
	e, err := arm.Carregar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Vagas) != 13 {
		t.Fatalf("duas requisições com o mesmo cargo na mesma empresa não são duplicatas: %d", len(e.Vagas))
	}
	for _, v := range e.Vagas {
		if v["id"] == "1" {
			if v["radarClasse"] != "excluir" || v["aderencia"] == "A" {
				t.Fatal(v)
			}
			continue
		}
		if v["radarClasse"] != "forte" || v["radarMotivo"] == nil || v["radarVersao"] == nil {
			t.Fatal(v)
		}
	}
	antiga := assinaturaDeBusca(e.Perfil)
	e.Perfil["paisTrabalho"] = "CA"
	if antiga == assinaturaDeBusca(e.Perfil) {
		t.Fatal("mudança de país não invalidou as notas")
	}
}
