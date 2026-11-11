package busca

import (
	"context"
	"fmt"
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
	{"id":11,"name":"Java Pleno","careerPageName":"Empresa Local","jobUrl":"https://exemplo.gupy.io/jobs/11","description":"<p>APIs &amp; dados</p>","workplaceType":"on-site"},
	{"id":12,"name":"Encerrada","careerPageName":"Exemplo","jobUrl":"https://exemplo.gupy.io/jobs/12","applicationDeadline":"2015-01-01T00:00:00Z"},
	{"id":13,"name":"URL inválida","careerPageName":"Exemplo","jobUrl":"https://gupy.io.example.org/jobs/13"}
]}`
const vagasHimalayas = `{"jobs":[
	{"title":"Java Backend","companyName":"Empresa Externa","guid":"https://himalayas.app/companies/exemplo/jobs/java","applicationLink":"https://himalayas.app/companies/exemplo/jobs/java/apply?ref=api","currency":"USD","salaryPeriod":"annual","minSalary":48000,"maxSalary":60000,"seniority":["Entry-level","Mid-level"],"locationRestrictions":["Brazil"],"categories":["Backend"],"expiryDate":4102444800},
	{"title":"Trabalho por hora","companyName":"Empresa Externa","guid":"id-horario","applicationLink":"https://himalayas.app/jobs/2","currency":"USD","salaryPeriod":"hourly","minSalary":42},
	{"title":"Contrato europeu","companyName":"Empresa Externa","guid":"id-eur","applicationLink":"https://himalayas.app/jobs/3","currency":"EUR","salaryPeriod":"monthly","minSalary":4000},
	{"title":"Salário mensal","companyName":"Empresa Externa","guid":"id-mensal","applicationLink":"https://himalayas.app/jobs/4","currency":"USD","salaryPeriod":"monthly","maxSalary":3800},
	{"title":"Expirada","companyName":"Empresa Externa","guid":"id-exp","applicationLink":"https://himalayas.app/jobs/5","expiryDate":100000}
]}`
const vagasArbeitnow = `{"data":[
	{"slug":"a","title":"Java-Entwickler","company_name":"Estúdio Europa","description":"APIs","url":"https://www.arbeitnow.com/jobs/a","location":"Berlin","tags":["Java"],"remote":false},
	{"slug":"b","title":"Serviços web","company_name":"Oficina Exterior","url":"https://www.arbeitnow.com/jobs/b","tags":["Java","Hybrid"],"job_types":["full-time"],"remote":false},
	{"slug":"c","title":"JavaScript developer","company_name":"Teste Frontend","url":"https://www.arbeitnow.com/jobs/c","tags":["JavaScript"],"remote":false},
	{"slug":"d","title":"Contabilidade","company_name":"Outra área","description":"Usamos ferramentas Java","url":"https://www.arbeitnow.com/jobs/d","tags":["Contabilidade"],"remote":false},
	{"slug":"e","title":"Java Madrid","company_name":"Teste Remoto","url":"https://www.arbeitnow.com/jobs/e","tags":["Java"],"remote":true}
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
	if len(res.Avisos) != 0 || chamadas != 5 || len(res.Vagas) != 8 {
		t.Fatalf("chamadas=%d vagas=%d avisos=%v", chamadas, len(res.Vagas), res.Avisos)
	}
	porTitulo := map[string]dominio.Objeto{}
	for _, v := range res.Vagas {
		porTitulo[dominio.Texto(v["titulo"])] = v
		if v["fonteURL"] == nil || v["fonteBusca"] == nil {
			t.Fatal("sem atribuição")
		}
	}
	if v := porTitulo["Java Pleno"]; v["descricao"] != "APIs & dados" || v["modelo"] != "Presencial" {
		t.Fatalf("Gupy: %+v", v)
	}
	v := porTitulo["Java Backend"]
	if v["salarioUSDMin"] != 4000.0 || v["salarioUSDMax"] != 5000.0 || v["salarioMin"] != 48000.0 || v["periodoSalario"] != "annual" || v["fonteURL"] != "https://himalayas.app/companies/exemplo/jobs/java" || v["expiraEm"] == nil || v["link"] != "https://himalayas.app/companies/exemplo/jobs/java/apply?ref=api" {
		t.Fatalf("Himalayas: %+v", v)
	}
	if porTitulo["Trabalho por hora"]["salarioUSDMin"] != nil || porTitulo["Contrato europeu"]["salarioUSDMin"] != nil || porTitulo["Salário mensal"]["salarioUSDMax"] != 3800.0 {
		t.Fatal("moeda/intervalo incorreto")
	}
	if porTitulo["Java-Entwickler"]["modelo"] != nil || porTitulo["Serviços web"]["modelo"] != "Híbrido" || porTitulo["Java Madrid"]["modelo"] != "Remoto" {
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
		{"internacional", []string{"Remoto"}, map[string]int{"himalayas.app": 1}},
		{"internacional", []string{"Híbrido", "Presencial"}, map[string]int{"www.arbeitnow.com": 1}},
		{"internacional", []string{"Remoto", "Presencial"}, map[string]int{"himalayas.app": 1, "www.arbeitnow.com": 1}},
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
		if len(tc.hosts) == 2 && (len(res.Avisos) != 1 || len(res.Vagas) == 0) {
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
	if err != nil || len(v) == 0 || !reflect.DeepEqual(paginas, []string{"https://www.arbeitnow.com/api/job-board-api?page=1", "https://www.arbeitnow.com/api/job-board-api?page=2"}) {
		t.Fatal(v, err, paginas)
	}
}

// Executar com APPLYER_TESTAR_FONTES_PUBLICAS=1 para verificar a integração com feeds vivos.
func TestFontesPublicas(t *testing.T) {
	if os.Getenv("APPLYER_TESTAR_FONTES_PUBLICAS") != "1" {
		t.Skip("smoke público optativo")
	}
	res := Consultar(context.Background(), dominio.Objeto{"cargo": "Java", "stackPrincipal": []any{"Software", "SAP"}, "foco": "ambos"})
	porFonte := map[string]int{}
	for _, v := range res.Vagas {
		porFonte[dominio.Texto(v["fonteBusca"])]++
	}
	t.Logf("vagas por fonte: %v; avisos: %v", porFonte, res.Avisos)
	for _, nome := range []string{"Gupy", "Himalayas", "Arbeitnow"} {
		if porFonte[nome] == 0 {
			t.Error(fmt.Sprintf("%s sem resultados nesta amostra", nome))
		}
	}
}
