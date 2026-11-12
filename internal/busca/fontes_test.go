package busca

import (
	"context"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

type transporte func(*http.Request) (*http.Response, error)

func (t transporte) RoundTrip(r *http.Request) (*http.Response, error) { return t(r) }
func resposta(codigo int, dados string) *http.Response {
	return &http.Response{StatusCode: codigo, Body: io.NopCloser(strings.NewReader(dados)), Header: make(http.Header)}
}

const vagasGupy = `{"data":[
	{"id":11,"name":"Desenvolvedor Java Pleno","careerPageName":"Empresa Local","jobUrl":"https://exemplo.gupy.io/jobs/11","description":"<p>APIs &amp; dados</p>","workplaceType":"on-site"},
	{"id":12,"name":"Encerrada","careerPageName":"Exemplo","jobUrl":"https://exemplo.gupy.io/jobs/12","applicationDeadline":"2015-01-01T00:00:00Z"},
	{"id":13,"name":"URL inválida","careerPageName":"Exemplo","jobUrl":"https://gupy.io.example.org/jobs/13"}
]}`
const vagasHimalayas = `{"jobs":[
	{"title":"Java Backend Engineer","companyName":"Empresa Externa","guid":"https://himalayas.app/companies/exemplo/jobs/java","applicationLink":"https://himalayas.app/companies/exemplo/jobs/java/apply?ref=api","currency":"USD","salaryPeriod":"annual","minSalary":48000,"maxSalary":60000,"seniority":["Entry-level","Mid-level"],"locationRestrictions":["Brazil"],"categories":["Backend"],"expiryDate":4102444800},
	{"title":"Java Engineer por hora","companyName":"Empresa Externa","guid":"id-horario","applicationLink":"https://himalayas.app/jobs/2","currency":"USD","salaryPeriod":"hourly","minSalary":42},
	{"title":"Java Developer europeu","companyName":"Empresa Externa","guid":"id-eur","applicationLink":"https://himalayas.app/jobs/3","currency":"EUR","salaryPeriod":"monthly","minSalary":4000},
	{"title":"Java Engineer mensal","companyName":"Empresa Externa","guid":"id-mensal","applicationLink":"https://himalayas.app/jobs/4","currency":"USD","salaryPeriod":"monthly","maxSalary":3800},
	{"title":"Expirada","companyName":"Empresa Externa","guid":"id-exp","applicationLink":"https://himalayas.app/jobs/5","expiryDate":100000}
]}`
const vagasArbeitnow = `{"data":[
	{"slug":"a","title":"Java-Entwickler","company_name":"Estúdio Europa","description":"APIs","url":"https://www.arbeitnow.com/jobs/a","location":"Berlin","tags":["Java"],"remote":false},
	{"slug":"b","title":"Java Developer web","company_name":"Oficina Exterior","url":"https://www.arbeitnow.com/jobs/b","tags":["Java","Hybrid"],"job_types":["full-time"],"remote":false},
	{"slug":"c","title":"JavaScript developer","company_name":"Teste Frontend","url":"https://www.arbeitnow.com/jobs/c","tags":["JavaScript"],"remote":false},
	{"slug":"d","title":"Contabilidade","company_name":"Outra área","description":"Usamos ferramentas Java","url":"https://www.arbeitnow.com/jobs/d","tags":["Contabilidade"],"remote":false},
	{"slug":"e","title":"Java Developer Madrid","company_name":"Teste Remoto","url":"https://www.arbeitnow.com/jobs/e","tags":["Java"],"remote":true}
],"links":{"next":null}}`

func TestBuscaCruzaFontesDeduplicaEConservaDadosOriginais(t *testing.T) {
	var lock sync.Mutex
	var chamadas int
	c := &http.Client{Transport: transporte(func(req *http.Request) (*http.Response, error) {
		lock.Lock()
		chamadas++
		lock.Unlock()
		if req.Method != "GET" || strings.Contains(req.URL.String(), "PRIVADO") {
			t.Errorf("consulta externa incorreta: %s %s", req.Method, req.URL)
		}
		switch req.URL.Host {
		case "portal.gupy.io":
			return resposta(200, vagasGupy), nil
		case "himalayas.app":
			return resposta(200, vagasHimalayas), nil
		case "weworkremotely.com":
			return resposta(200, `<rss><channel/></rss>`), nil
		case "jobicy.com":
			return resposta(200, `{"jobs":[]}`), nil
		case "www.arbeitnow.com":
			if req.URL.Query().Get("page") != "1" || len(req.URL.Query()) != 1 {
				t.Errorf("termo exposto ao feed: %v", req.URL)
			}
			return resposta(200, vagasArbeitnow), nil
		default:
			t.Errorf("host inesperado: %s", req.URL.Host)
			return resposta(404, ""), nil
		}
	})}
	res := consultar(context.Background(), c, dominio.Objeto{"cargo": "Java", "stackPrincipal": []any{"Kotlin"}, "foco": "ambos", "email": "PRIVADO", "filtros": dominio.Objeto{"pularSe": []any{"PRIVADO"}}})
	if len(res.Avisos) != 0 || chamadas != 11 || len(res.Vagas) != 8 {
		t.Fatalf("chamadas=%d vagas=%d avisos=%v", chamadas, len(res.Vagas), res.Avisos)
	}
	porTitulo := map[string]dominio.Objeto{}
	for _, v := range res.Vagas {
		porTitulo[dominio.Texto(v["titulo"])] = v
		if v["fonteURL"] == nil || v["fonteBusca"] == nil {
			t.Fatal("sem atribuição")
		}
	}
	if v := porTitulo["Desenvolvedor Java Pleno"]; v["descricao"] != "APIs & dados" || v["modelo"] != "Presencial" {
		t.Fatalf("Gupy: %+v", v)
	}
	v := porTitulo["Java Backend Engineer"]
	if v["salarioUSDMin"] != 4000.0 || v["salarioUSDMax"] != 5000.0 || v["salarioMin"] != 48000.0 || v["periodoSalario"] != "annual" || v["fonteURL"] != "https://himalayas.app/companies/exemplo/jobs/java" || v["expiraEm"] == nil || v["link"] != "https://himalayas.app/companies/exemplo/jobs/java/apply?ref=api" {
		t.Fatalf("Himalayas: %+v", v)
	}
	if porTitulo["Java Engineer por hora"]["salarioUSDMin"] != nil || porTitulo["Java Developer europeu"]["salarioUSDMin"] != nil || porTitulo["Java Engineer mensal"]["salarioUSDMax"] != 3800.0 {
		t.Fatal("moeda/intervalo incorreto")
	}
	if porTitulo["Java-Entwickler"]["modelo"] != nil || porTitulo["Java Developer web"]["modelo"] != "Híbrido" || porTitulo["Java Developer Madrid"]["modelo"] != "Remoto" {
		t.Fatal("modalidade inferida incorretamente")
	}
	if porTitulo["JavaScript developer"] != nil || porTitulo["Contabilidade"] != nil || porTitulo["Expirada"] != nil || porTitulo["Encerrada"] != nil {
		t.Fatal("vagas alheias/expiradas chegaram ao resultado")
	}
}

func TestFontesPorPreferenciaEColetaParcial(t *testing.T) {
	for _, tc := range []struct {
		foco    string
		modelos []string
		hosts   map[string]int
	}{
		{"nacional", []string{"Híbrido"}, map[string]int{"portal.gupy.io": 1}},
		{"internacional", []string{"Remoto"}, map[string]int{"himalayas.app": 1, "weworkremotely.com": 1, "jobicy.com": 1}},
		{"internacional", []string{"Híbrido", "Presencial"}, map[string]int{"www.arbeitnow.com": 1}},
		{"internacional", []string{"Remoto", "Presencial"}, map[string]int{"himalayas.app": 1, "www.arbeitnow.com": 1, "weworkremotely.com": 1, "jobicy.com": 1}},
	} {
		chamadas := map[string]int{}
		var lock sync.Mutex
		c := &http.Client{Transport: transporte(func(req *http.Request) (*http.Response, error) {
			lock.Lock()
			chamadas[req.URL.Host]++
			lock.Unlock()
			if req.URL.Host == "www.arbeitnow.com" {
				return resposta(200, vagasArbeitnow), nil
			}
			return resposta(503, ""), nil
		})}
		res := consultar(context.Background(), c, dominio.Objeto{"cargo": "Java", "foco": tc.foco, "filtros": dominio.Objeto{"modelo": tc.modelos}})
		if !reflect.DeepEqual(chamadas, tc.hosts) {
			t.Fatalf("fontes %v, esperado %v", chamadas, tc.hosts)
		}
		if len(tc.hosts) == 4 && (len(res.Avisos) != 3 || len(res.Vagas) == 0) {
			t.Fatalf("perdeu a coleta parcial: %+v", res)
		}
		if len(tc.hosts) == 1 && tc.hosts["www.arbeitnow.com"] > 0 && len(res.Vagas) != 2 {
			t.Fatal("remota entrou em busca apenas híbrida/presencial")
		}
	}
}

func TestArbeitnowLimitaPaginasEIgnoraDestinoDoFeed(t *testing.T) {
	var paginas []string
	c := &http.Client{Transport: transporte(func(req *http.Request) (*http.Response, error) {
		paginas = append(paginas, req.URL.String())
		return resposta(200, strings.Replace(vagasArbeitnow, `"next":null`, `"next":"https://example.invalid/fora"`, 1)), nil
	})}
	v, err := arbeitnow(context.Background(), c, []string{"Contabilidade"}, true)
	if err != nil || len(v) == 0 || !reflect.DeepEqual(paginas, []string{"https://www.arbeitnow.com/api/job-board-api?page=1", "https://www.arbeitnow.com/api/job-board-api?page=2", "https://www.arbeitnow.com/api/job-board-api?page=3"}) {
		t.Fatal(v, err, paginas)
	}
}

// Executar com APPLYER_TESTAR_FONTES_PUBLICAS=1 para verificar a integração com feeds vivos.
func TestFontesPublicas(t *testing.T) {
	if os.Getenv("APPLYER_TESTAR_FONTES_PUBLICAS") != "1" {
		t.Skip("smoke público optativo")
	}
	for _, cargo := range []string{"Java Engineer", "AI Engineer"} {
		res := Consultar(context.Background(), dominio.Objeto{"cargo": cargo, "paisTrabalho": "BR", "foco": "internacional", "filtros": dominio.Objeto{"modelo": []string{"Remoto"}, "niveis": []string{"Sênior"}}})
		porFonte, datadas := map[string]int{}, map[string]int{}
		for _, v := range res.Vagas {
			fonte := dominio.Texto(v["fonteBusca"])
			porFonte[fonte]++
			if v["publicadaEm"] != nil {
				if datadas[fonte] == 0 {
					t.Logf("%s · %s · %s · %s · %s", cargo, v["titulo"], v["local"], v["publicadaEm"], v["fonteURL"])
				}
				datadas[fonte]++
			}
		}
		t.Logf("%s: vagas %v; com data %v; avisos %v", cargo, porFonte, datadas, res.Avisos)
		if len(res.Avisos) > 0 {
			t.Errorf("cobertura parcial no teste ao vivo: %v", res.Avisos)
		}
	}
}
