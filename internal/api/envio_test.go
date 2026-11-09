package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// Estado do envio, pausar e retomar (jc-a2y.51). Fila fictícia gravada direto no banco.

const marcaEnvio = "Conteudo-Ficticio-Marcado-5e21" // não pode sair no GET /envio

func apiDeEnvio(t *testing.T, ligado bool) (*API, *armazem.Armazem) {
	t.Helper()
	arm, err := armazem.Abrir(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	a := Nova(arm, Opcoes{Hoje: func() string { return "2026-11-09" }, Cofre: segredos.Memoria{}, ProvedoresIA: map[string]ia.Provedor{},
		DetectarCLIs: func(context.Context) []ia.EstadoCLI { return nil }, EnvioLigado: ligado})
	return a, arm
}

func chamarEnvio(t *testing.T, a *API, m, p string, corpo any, autor string) Resposta {
	t.Helper()
	r, err := a.Chamar(context.Background(), Pedido{Metodo: m, Caminho: Prefixo + p, Corpo: corpo, Autor: autor})
	if err != nil {
		t.Fatal(err)
	}
	// Ida e volta por JSON, como no HTTP.
	b, _ := json.Marshal(r.Corpo)
	var o any
	json.Unmarshal(b, &o)
	r.Corpo = o
	return r
}

func TestEnvioEstadoPausarRetomar(t *testing.T) {
	var saida bytes.Buffer
	log.SetOutput(&saida)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	a, _ := apiDeEnvio(t, false)
	r := chamarEnvio(t, a, "GET", "/envio", nil, "eu")
	c := r.Corpo.(map[string]any)
	if r.Status != 200 || c["ligado"] != false || c["pausado"] != true || c["enviandoAgora"] != nil || len(c["desfechos"].([]any)) != 0 {
		t.Fatalf("build padrão, banco novo = %d %v", r.Status, c)
	}
	if b, _ := apiDeEnvio(t, true); chamarEnvio(t, b, "GET", "/envio", nil, "eu").Corpo.(map[string]any)["ligado"] != true {
		t.Fatal("ligado no build envio_dev")
	}
	// Retomar: só a pessoa; corpo estrito.
	if r := chamarEnvio(t, a, "POST", "/envio/retomar", Objeto{}, "claude"); r.Status != 403 {
		t.Fatalf("assistente retomar = %d", r.Status)
	}
	if r := chamarEnvio(t, a, "POST", "/envio/retomar", Objeto{"pausado": false}, "eu"); r.Status != 400 {
		t.Fatalf("retomar com campo = %d", r.Status)
	}
	if r := chamarEnvio(t, a, "GET", "/envio", nil, "eu"); r.Corpo.(map[string]any)["pausado"] != true {
		t.Fatal("a recusa retomou")
	}
	if r := chamarEnvio(t, a, "POST", "/envio/retomar", Objeto{}, "eu"); r.Status != 200 || r.Corpo.(map[string]any)["pausado"] != false {
		t.Fatalf("retomar = %d %v", r.Status, r.Corpo)
	}
	// Pausar: qualquer autor (reduz o risco).
	if r := chamarEnvio(t, a, "POST", "/envio/pausar", Objeto{"x": 1}, "claude"); r.Status != 400 {
		t.Fatalf("pausar com campo = %d", r.Status)
	}
	if r := chamarEnvio(t, a, "POST", "/envio/pausar", Objeto{}, "claude"); r.Status != 200 || r.Corpo.(map[string]any)["pausado"] != true {
		t.Fatalf("assistente pausar = %d %v", r.Status, r.Corpo)
	}
	if r := chamarEnvio(t, a, "GET", "/envio?desde=ontem", nil, "eu"); r.Status != 400 {
		t.Fatalf("desde inválido = %d", r.Status)
	}
	// Log: só o fato e o autor, e nada nas recusas.
	if got := saida.String(); !strings.Contains(got, "envio: retomado (eu)") || !strings.Contains(got, "envio: pausado (claude)") || strings.Count(got, "envio: ") != 2 {
		t.Fatalf("log = %q", got)
	}
}

// Desfechos: só os do executor, sem conteúdo (id, estado, motivo, em), do mais novo para o mais antigo, até 20,
// com o filtro desde; e a candidatura em enviando agora.
func TestEnvioDesfechos(t *testing.T) {
	a, arm := apiDeEnvio(t, true)
	base := time.Date(2026, 11, 9, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600))
	em := func(min int) string { return base.Add(time.Duration(min) * time.Minute).Format(time.RFC3339) }
	cand := func(id, estado string, extra Objeto) Objeto {
		o := Objeto{"id": id, "vagaId": "v-" + id, "estado": estado, "conteudo": Objeto{"mensagem": marcaEnvio}, "url": "https://www.linkedin.com/jobs/view/9000000001"}
		for k, v := range extra {
			o[k] = v
		}
		return o
	}
	fila := []Objeto{
		cand("cand-a", "enviada", Objeto{"resultado": Objeto{"em": em(1), "tipo": "enviada"}}),
		cand("cand-b", "bloqueada", Objeto{"bloqueio": Objeto{"motivo": "cpf", "pendencia": "texto fixo", "em": em(3)}}),
		cand("cand-c", "falhou", Objeto{"resultado": Objeto{"em": em(2), "tipo": "falhou", "detalhe": "x"}}),
		cand("cand-d", "incerta", Objeto{"resultado": Objeto{"em": em(4), "tipo": "incerto"}}),
		cand("cand-e", "enviada", Objeto{"resultado": Objeto{"em": em(5), "tipo": "manual"}}), // a pessoa: não conta
		cand("cand-f", "cancelada", nil),
		cand("cand-g", "preparada", nil),
		cand("cand-h", "enviando", Objeto{"tentativas": []any{em(0), em(6)}}),
	}
	for i := 0; i < 25; i++ { // para o corte em 20
		fila = append(fila, cand(fmt.Sprintf("cand-z%02d", i), "enviada", Objeto{"resultado": Objeto{"em": em(-100 - i), "tipo": "enviada"}}))
	}
	if err := arm.Gravar(context.Background(), func(e dominio.Estado) (dominio.Estado, error) {
		e.Candidaturas = fila
		return e, nil
	}); err != nil {
		t.Fatal(err)
	}
	r := chamarEnvio(t, a, "GET", "/envio", nil, "eu")
	c := r.Corpo.(map[string]any)
	// plataformas (jc-a2y.6.11) traz o nome da plataforma como código fixo; o resto não pode trazer nada da fila.
	semPlataformas := map[string]any{}
	for k, v := range c {
		if k != "plataformas" {
			semPlataformas[k] = v
		}
	}
	b, _ := json.Marshal(semPlataformas)
	for _, proibido := range []string{marcaEnvio, "v-cand", "linkedin", "texto fixo", "conteudo", "pendencia", "detalhe", "cand-e", "cand-f", "cand-g"} {
		if strings.Contains(string(b), proibido) {
			t.Errorf("GET /envio tem %q: %s", proibido, b)
		}
	}
	if ea := c["enviandoAgora"].(map[string]any); ea["id"] != "cand-h" || ea["desde"] != em(6) {
		t.Fatalf("enviandoAgora = %v", ea)
	}
	ds := c["desfechos"].([]any)
	if len(ds) != 20 {
		t.Fatalf("desfechos = %d; queria 20", len(ds))
	}
	ids := []string{}
	for i, d := range ds {
		o := d.(map[string]any)
		chaves := []string{}
		for k := range o {
			chaves = append(chaves, k)
		}
		sort.Strings(chaves)
		if strings.Join(chaves, ",") != "em,estado,id,motivo" {
			t.Fatalf("campos do desfecho %d = %v", i, chaves)
		}
		if i < 4 {
			ids = append(ids, fmt.Sprint(o["id"], ":", o["estado"], ":", o["motivo"]))
		}
	}
	if strings.Join(ids, " ") != "cand-d:incerta:<nil> cand-b:bloqueada:cpf cand-c:falhou:<nil> cand-a:enviada:<nil>" {
		t.Fatalf("ordem = %v", ids)
	}
	// desde: só os depois do instante.
	r = chamarEnvio(t, a, "GET", "/envio?desde="+strings.ReplaceAll(em(2), "+", "%2B"), nil, "eu")
	ds = r.Corpo.(map[string]any)["desfechos"].([]any)
	if len(ds) != 2 || ds[0].(map[string]any)["id"] != "cand-d" || ds[1].(map[string]any)["id"] != "cand-b" {
		t.Fatalf("desde = %v", ds)
	}
}
