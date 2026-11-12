package busca

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

func TestConsultasEquivalentesSemHabilidadeIsolada(t *testing.T) {
	var termos []string
	c := &http.Client{Transport: transporte(func(req *http.Request) (*http.Response, error) {
		termos = append(termos, req.URL.Query().Get("jobName"))
		return resposta(200, `{"data":[]}`), nil
	})}
	perfil := dominio.Objeto{"foco": "nacional", "cargo": "Desenvolvedor backend Java Sênior", "stackPrincipal": []string{"Java", "Spring"}}
	consultar(context.Background(), c, perfil)
	for _, esperado := range []string{"Java Engineer", "Desenvolvedor Java", "Backend Engineer"} {
		if !slices.Contains(termos, esperado) {
			t.Errorf("%q ausente: %v", esperado, termos)
		}
	}
	if slices.Contains(termos, "Java") || slices.Contains(termos, "Spring") {
		t.Error(termos)
	}
	termos = nil
	consultar(context.Background(), c, dominio.Objeto{"foco": "nacional", "stackPrincipal": []string{"Spring"}})
	consultar(context.Background(), c, dominio.Objeto{"foco": "nacional", "nome": "Pessoa Fictícia", "cargo": "Pessoa Fictícia"})
	if len(termos) > 0 {
		t.Fatalf("cargo ausente ou igual ao nome enviado: %v", termos)
	}
}

func TestGupyPaginaPorDezAteTresPreservaParciaisERequisicoesDistintas(t *testing.T) {
	for _, falha := range []bool{false, true} {
		var offsets []int
		umaHora := time.Now().UTC().Add(-time.Hour).Truncate(time.Second).Format(time.RFC3339)
		antiga := time.Now().UTC().Add(-31 * 24 * time.Hour).Format(time.RFC3339)
		c := &http.Client{Transport: transporte(func(req *http.Request) (*http.Response, error) {
			q := req.URL.Query()
			off, _ := strconv.Atoi(q.Get("offset"))
			offsets = append(offsets, off)
			if q.Get("limit") != "10" {
				t.Errorf("limit=%s", q.Get("limit"))
			}
			if falha && off >= 10 {
				return resposta(503, ""), nil
			}
			dados := []dominio.Objeto{}
			for j := 1; j <= 10; j++ {
				id := off + j
				v := dominio.Objeto{"id": id, "name": "Contador", "careerPageName": "Contas Exemplo", "jobUrl": fmt.Sprintf("https://contas.gupy.io/jobs/%d", id), "workplaceType": "on-site", "city": "Curitiba", "state": "Paraná", "country": "Brasil", "publishedDate": umaHora}
				switch id {
				case 2:
					v["publishedDate"] = antiga
				case 3:
					delete(v, "publishedDate")
				case 4:
					v["workplaceType"] = "remote"
				}
				dados = append(dados, v)
			}
			body, _ := json.Marshal(dominio.Objeto{"data": dados})
			return resposta(200, string(body)), nil
		})}
		r := consultar(context.Background(), c, dominio.Objeto{"foco": "nacional", "cargo": "Contador"})
		if !falha && (!slices.Equal(offsets, []int{0, 10, 20}) || len(r.Vagas) != 29) {
			t.Errorf("offsets=%v vagas=%d", offsets, len(r.Vagas))
		}
		if falha && (len(r.Avisos) != 1 || len(r.Vagas) != 9) {
			t.Errorf("parciais: vagas=%d avisos=%v", len(r.Vagas), r.Avisos)
		}
		for _, v := range r.Vagas {
			switch v["id"] {
			case "gupy-1":
				if v["publicadaEm"] != umaHora || v["local"] != "Curitiba, Paraná, Brasil" {
					t.Errorf("metadados: %v", v)
				}
			case "gupy-2":
				t.Error("vaga de 31 dias")
			case "gupy-3":
				if v["publicadaEm"] != nil {
					t.Error("inventou publicação")
				}
			case "gupy-4":
				if v["local"] != "" || v["paisesElegiveis"] != nil {
					t.Error("sede virou restrição remota")
				}
			}
		}
	}
}

func TestRemotasUsamPaisEDatasDePublicacaoDaFonte(t *testing.T) {
	date := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Second)
	fim := date.Add(10 * 24 * time.Hour).Format(time.RFC1123Z)
	xmlVaga := func(slug, regiao, vence string) string {
		return fmt.Sprintf("<item><title>Exemplo: Backend Engineer</title><region>%s</region><link>https://weworkremotely.com/remote-jobs/%s</link><pubDate>%s</pubDate><expires_at>%s</expires_at></item>", regiao, slug, date.Format(time.RFC1123Z), vence)
	}
	var paginas []string
	c := &http.Client{Transport: transporte(func(req *http.Request) (*http.Response, error) {
		if req.URL.Host == "jobicy.com" {
			if req.URL.Query().Get("get") == "locations" {
				return resposta(200, `{"locations":[]}`), nil
			}
			return resposta(200, `{"jobs":[]}`), nil
		}
		if req.URL.Host == "weworkremotely.com" {
			return resposta(200, "<rss><channel>"+xmlVaga("a", "Anywhere in the World", fim)+xmlVaga("b", "USA Only", fim)+xmlVaga("c", "Brazil", date.Format(time.RFC1123Z))+"</channel></rss>"), nil
		}
		if req.URL.Host != "himalayas.app" {
			t.Errorf("host inesperado %s", req.URL.Host)
			return resposta(503, ""), nil
		}
		if req.URL.Query().Get("country") != "BR" {
			t.Errorf("país não aplicado: %s", req.URL)
		}
		paginas = append(paginas, req.URL.Query().Get("page"))
		return resposta(200, `{"jobs":[{"title":"Backend Engineer","companyName":"Vaga Exemplo","guid":"https://himalayas.app/companies/vaga/jobs/backend","applicationLink":"https://himalayas.app/companies/vaga/jobs/backend?utm_source=api","pubDate":1234,"locationRestrictions":["Brazil"],"timezoneRestrictions":[-3]}]}`), nil
	})}
	r := consultar(context.Background(), c, dominio.Objeto{"cargo": "Backend Engineer", "foco": "internacional", "paisTrabalho": "Brasil", "filtros": dominio.Objeto{"modelo": []string{"Remoto"}}})
	if len(r.Vagas) != 3 || len(r.Avisos) != 0 || len(paginas) == 0 {
		t.Fatalf("vagas=%v avisos=%v", r.Vagas, r.Avisos)
	}
	for _, v := range r.Vagas {
		switch {
		case v["fonteBusca"] == "Himalayas":
			if v["publicadaEm"] != nil || !slices.Equal(listaTextos(v["paisesElegiveis"]), []string{"Brazil"}) {
				t.Error(v)
			}
		case strings.HasSuffix(dominio.Texto(v["link"]), "/a"):
			if v["publicadaEm"] != date.Format(time.RFC3339) || v["remotoMundial"] != true {
				t.Error(v)
			}
		case strings.HasSuffix(dominio.Texto(v["link"]), "/b"):
			if v["local"] != "USA Only" || v["remotoMundial"] == true {
				t.Error(v)
			}
		default:
			t.Error(v)
		}
	}
}

func TestCargoEscolhidoTemPrecedenciaSobreHabilidadeEDaEspacoAOutrasProfissoes(t *testing.T) {
	c := dominio.Objeto{"cargo": "Desenvolvedora Python", "stackPrincipal": []string{"Java"}}
	aliases := consultasDoPerfil(c)
	if !slices.Contains(aliases, "Python Engineer") || slices.Contains(aliases, "Java Engineer") {
		t.Error(aliases)
	}
	c = dominio.Objeto{"cargo": "Restauradora de Acervos", "titulosEquivalentes": []any{"Paper Conservator"}}
	aliases = consultasDoPerfil(c)
	if !slices.Contains(aliases, "Restauradora de Acervos") || !slices.Contains(aliases, "Paper Conservator") {
		t.Error(aliases)
	}
}

func TestIdentidadeDaRequisicaoEntrePerfis(t *testing.T) {
	original := dominio.Objeto{"id": "gupy-8", "empresa": "Exemplo", "titulo": "Contador"}
	outra := dominio.Objeto{"id": "p2-gupy-8", "anuncioId": "gupy-8"}
	if !MesmoAnuncio(original, outra) {
		t.Fatal("prefixo do perfil mudou a requisição")
	}
	a := dominio.Objeto{"id": "123", "link": "https://exemplo.org/jobs?id=A&utm_source=x"}
	b := dominio.Objeto{"id": "456", "link": "https://exemplo.org/jobs?id=B", "fonteURL": "https://exemplo.org/jobs"}
	if MesmoAnuncio(a, b) {
		t.Fatal("parâmetro da requisição perdido")
	}
	a["fonteURL"] = "https://exemplo.org/jobs"
	if MesmoAnuncio(a, b) {
		t.Fatal("página de carreiras confundida com anúncio individual")
	}
}

func TestArbeitnowNaoPublicaTagsComoCargoENaoUsaSedeRemota(t *testing.T) {
	pub := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Second)
	c := &http.Client{Transport: transporte(func(_ *http.Request) (*http.Response, error) {
		return resposta(200, fmt.Sprintf(`{"data":[{"slug":"a","title":"Java Developer","company_name":"Teste","url":"https://www.arbeitnow.com/jobs/a","remote":true,"location":"Berlin","created_at":%d},{"slug":"b","title":"Serviços financeiros","company_name":"Teste","url":"https://www.arbeitnow.com/jobs/b","tags":["Java","Developer"]}]}`, pub.Unix())), nil
	})}
	v, err := arbeitnow(context.Background(), c, []string{"Java Developer"}, true)
	if err != nil || len(v) != 1 || v[0]["publicadaEm"] != pub.Format(time.RFC3339) || v[0]["local"] != nil {
		t.Fatal(v, err)
	}
}

func TestFonteDiretaPrevaleceQuandoAReplicaCompartilhaARequisicao(t *testing.T) {
	c := &http.Client{Transport: transporte(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "jobicy.com":
			return resposta(200, `{"jobs":[]}`), nil
		case "portal.gupy.io":
			return resposta(200, `{"data":[{"id":7,"name":"Contador","careerPageName":"Fictícia","jobUrl":"https://ficticia.gupy.io/jobs/7"},{"id":8,"name":"Contador","careerPageName":"Fictícia","jobUrl":"https://ficticia.gupy.io/jobs/8"}]}`), nil
		case "himalayas.app":
			return resposta(200, `{"jobs":[{"title":"Contador","companyName":"Fictícia","guid":"https://himalayas.app/companies/ficticia/jobs/sete","applicationLink":"https://ficticia.gupy.io/jobs/8?utm_source=espelho"}]}`), nil
		default:
			return resposta(200, `<rss><channel/></rss>`), nil
		}
	})}
	r := consultar(context.Background(), c, dominio.Objeto{"cargo": "Contador", "foco": "ambos", "filtros": dominio.Objeto{"modelo": []string{"Remoto"}}})
	if len(r.Vagas) != 2 {
		t.Fatalf("%v", r.Vagas)
	}
	for _, v := range r.Vagas {
		if v["fonteBusca"] != "Gupy" {
			t.Fatalf("espelho: %v", v)
		}
	}
}

func TestJobicyEncontraEngenhariaComFuncaoGenericaEPapelEspecializado(t *testing.T) {
	pub := time.Now().UTC().Add(-24 * time.Hour).Truncate(time.Second).Format(time.RFC3339)
	termos := map[string]bool{}
	c := &http.Client{Transport: transporte(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Host {
		case "himalayas.app":
			return resposta(200, `{"jobs":[]}`), nil
		case "weworkremotely.com":
			return resposta(200, `<rss><channel/></rss>`), nil
		case "jobicy.com":
			q := req.URL.Query()
			if q.Get("get") == "locations" {
				return resposta(200, `{"locations":[{"geoName":"Brazil","geoSlug":"brazil"}]}`), nil
			}
			if q.Get("count") != "10" || q.Get("geo") != "brazil" {
				t.Errorf("filtro %s", req.URL)
			}
			if len([]rune(q.Get("tag"))) > 20 {
				t.Errorf("Jobicy rejeita o termo %q", q.Get("tag"))
			}
			termos[q.Get("tag")] = true
			if q.Get("cursor") != "" {
				return resposta(200, `{"jobs":[],"nextCursor":null}`), nil
			}
			return resposta(200, fmt.Sprintf(`{"nextCursor":"next","jobs":[{"id":123,"jobTitle":"Senior Software Engineer","companyName":"Exemplo","url":"https://jobicy.com/jobs/123-software","jobGeo":"LATAM, Canada","jobLevel":"Senior","jobDescription":"Build production machine learning pipelines and AI services.","pubDate":%q,"salaryMin":70000,"salaryCurrency":"EUR","salaryPeriod":"yearly"},{"id":124,"jobTitle":"Senior Software Engineer","companyName":"Exemplo","url":"https://jobicy.com/jobs/124-platform","jobGeo":"Anywhere","jobDescription":"Maintain production inventory services in PHP."}]}`, pub)), nil
		}
		return resposta(404, ""), nil
	})}
	r := consultar(context.Background(), c, dominio.Objeto{"foco": "internacional", "cargo": "AI Engineer", "paisTrabalho": "BR", "filtros": dominio.Objeto{"modelo": []string{"Remoto"}}})
	if !termos["AI Engineer"] || len(r.Vagas) != 1 || len(r.Avisos) != 0 {
		t.Fatalf("termos=%v coleta=%+v", termos, r)
	}
	v := r.Vagas[0]
	if v["publicadaEm"] != pub || v["fonteBusca"] != "Jobicy" || v["moeda"] != "EUR" || v["periodoSalario"] != "yearly" || v["salarioMin"] != 70000.0 || !slices.Equal(listaTextos(v["paisesElegiveis"]), []string{"LATAM", "Canada"}) {
		t.Error(v)
	}
}

func TestColetaAceitaFuncaoConfirmadaNasAtribuicoes(t *testing.T) {
	for _, tc := range []struct{ cargo, titulo, descricao string }{
		{"Java Engineer", "Senior Backend Engineer", "Desenvolver APIs em Java e Spring Boot."},
		{"Advogada Trabalhista", "Advogada Sênior", "Atuar em ações de direito trabalhista e negociações coletivas."},
	} {
		c := &http.Client{Transport: transporte(func(req *http.Request) (*http.Response, error) {
			body, _ := json.Marshal(dominio.Objeto{"data": []dominio.Objeto{{"id": 123, "jobUrl": "https://exemplo.gupy.io/jobs/123", "careerPageName": "Exemplo", "name": tc.titulo, "description": tc.descricao}, {"id": 124, "jobUrl": "https://exemplo.gupy.io/jobs/124", "careerPageName": "Exemplo", "name": "Atendimento ao Cliente", "description": tc.cargo}}})
			return resposta(200, string(body)), nil
		})}
		r := consultar(context.Background(), c, dominio.Objeto{"cargo": tc.cargo, "foco": "nacional"})
		if len(r.Vagas) != 1 || r.Vagas[0]["titulo"] != tc.titulo {
			t.Errorf("%s: %+v", tc.cargo, r)
		}
	}
}

func TestFilaDaTriagemRecebePrimeiroPublicacaoComprovada(t *testing.T) {
	pub := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	c := &http.Client{Transport: transporte(func(req *http.Request) (*http.Response, error) {
		return resposta(200, fmt.Sprintf(`{"data":[{"id":1,"name":"Contador","careerPageName":"Empresa","jobUrl":"https://exemplo.gupy.io/jobs/1"},{"id":2,"name":"Contador","careerPageName":"Empresa","jobUrl":"https://exemplo.gupy.io/jobs/2","publishedDate":%q}]}`, pub)), nil
	})}
	r := consultar(context.Background(), c, dominio.Objeto{"cargo": "Contador", "foco": "nacional"})
	if len(r.Vagas) != 2 || r.Vagas[0]["id"] != "gupy-2" {
		t.Fatalf("%+v", r)
	}
}

func TestLimiteDaFonteExplicaCoberturaParcial(t *testing.T) {
	c := &http.Client{Transport: transporte(func(_ *http.Request) (*http.Response, error) { return resposta(429, ""), nil })}
	r := consultar(context.Background(), c, dominio.Objeto{"cargo": "Contador", "foco": "nacional"})
	if len(r.Avisos) != 1 || !strings.Contains(r.Avisos[0], "429") {
		t.Fatalf("%v", r.Avisos)
	}
}
