package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/aovivo"
	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// "Ao vivo" (jc-a2y.6.9, docs/propostas/ao-vivo.md). Quadros fictícios, só em memória.

var quadroFicticio = base64.StdEncoding.EncodeToString([]byte("Quadro-Ficticio-Marcado-77ab"))

type cenarioAoVivo struct {
	a     *API
	arm   *armazem.Armazem
	av    *aovivo.AoVivo
	login bool
	log   *bytes.Buffer
}

func novoAoVivo(t *testing.T, comGancho bool) *cenarioAoVivo {
	t.Helper()
	var saida bytes.Buffer
	log.SetOutput(&saida)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	arm, err := armazem.Abrir(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	c := &cenarioAoVivo{arm: arm, log: &saida}
	o := Opcoes{Hoje: func() string { return "2026-11-09" }, Cofre: segredos.Memoria{}, ProvedoresIA: map[string]ia.Provedor{},
		DetectarCLIs: func(context.Context) []ia.EstadoCLI { return nil }, EnvioLigado: comGancho,
		Janela: func() (bool, bool) { return false, c.login }}
	if comGancho {
		c.av = aovivo.Novo(time.Now)
		o.AoVivo = c.av
	}
	c.a = Nova(arm, o)
	if err := arm.GravarEnvioPausado(context.Background(), false); err != nil { // o envio começa pausado
		t.Fatal(err)
	}
	return c
}

func (c *cenarioAoVivo) chamar(t *testing.T, m string, corpo any, autor string, daJanela bool) Resposta {
	t.Helper()
	r, err := c.a.Chamar(context.Background(), Pedido{Metodo: m, Caminho: Prefixo + "/envio/ao-vivo", Corpo: corpo, Autor: autor, DaJanela: daJanela})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(r.Corpo)
	var o any
	json.Unmarshal(b, &o)
	r.Corpo = o
	return r
}

func corpoDe(r Resposta) map[string]any { o, _ := r.Corpo.(map[string]any); return o }

func TestAoVivoNoBuildPadraoEh404(t *testing.T) {
	c := novoAoVivo(t, false)
	for _, m := range []string{"GET", "POST"} {
		if r := c.chamar(t, m, Objeto{"ligado": true}, "eu", true); r.Status != 404 {
			t.Fatalf("%s sem o gancho = %d", m, r.Status)
		}
	}
}

func TestAoVivoSoATelaLeELiga(t *testing.T) {
	c := novoAoVivo(t, true)
	// Ler: o assistente e quem não é a janela levam 403.
	for nome, p := range map[string]struct {
		autor    string
		daJanela bool
	}{"assistente": {"claude", false}, "assistente com marca": {"claude", true}, "eu fora da janela": {"eu", false}} {
		if r := c.chamar(t, "GET", nil, p.autor, p.daJanela); r.Status != 403 {
			t.Fatalf("GET %s = %d", nome, r.Status)
		}
		if r := c.chamar(t, "POST", Objeto{"ligado": true}, p.autor, p.daJanela); r.Status != 403 {
			t.Fatalf("ligar %s = %d", nome, r.Status)
		}
	}
	if c.av.Querendo() || c.av.Ler().Ligado {
		t.Fatal("a recusa ligou")
	}
	// Corpo estrito.
	for nome, corpo := range map[string]any{"vazio": Objeto{}, "texto": Objeto{"ligado": "sim"}, "campo a mais": Objeto{"ligado": true, "fps": 30}, "lista": []any{true}} {
		if r := c.chamar(t, "POST", corpo, "eu", true); r.Status != 400 {
			t.Fatalf("corpo %s = %d", nome, r.Status)
		}
	}
	// A tela liga e lê.
	r := c.chamar(t, "POST", Objeto{"ligado": true}, "eu", true)
	if r.Status != 200 || corpoDe(r)["ligado"] != true || corpoDe(r)["etapa"] != "sem-envio" || corpoDe(r)["quadro"] != nil || corpoDe(r)["candidatura"] != nil {
		t.Fatalf("ligar = %d %v", r.Status, r.Corpo)
	}
	c.av.Etapa("cand-2026-11-09-0001-abcd", aovivo.Preenchendo)
	c.av.Quadro(quadroFicticio, 800, 600)
	r = c.chamar(t, "GET", nil, "eu", true)
	o := corpoDe(r)
	q, _ := o["quadro"].(map[string]any)
	if r.Status != 200 || o["etapa"] != "preenchendo" || o["candidatura"] != "cand-2026-11-09-0001-abcd" || q == nil {
		t.Fatalf("ler = %d %v", r.Status, o)
	}
	chaves := []string{}
	for k := range q {
		chaves = append(chaves, k)
	}
	sort.Strings(chaves)
	if strings.Join(chaves, ",") != "altura,em,imagem,largura" || q["imagem"] != "data:image/jpeg;base64,"+quadroFicticio || q["largura"] != float64(800) {
		t.Fatalf("quadro = %v", q)
	}
	// Qualquer autor desliga (só reduz o risco), e o quadro some.
	r = c.chamar(t, "POST", Objeto{"ligado": false}, "claude", false)
	if r.Status != 200 || corpoDe(r)["ligado"] != false || corpoDe(r)["etapa"] != "desligado" || corpoDe(r)["quadro"] != nil {
		t.Fatalf("assistente desligou = %d %v", r.Status, r.Corpo)
	}
	if c.av.Querendo() {
		t.Fatal("continua querendo quadros")
	}
	// O log: só ligado e desligado, com o autor. Nada do quadro nem da etapa.
	linhas := []string{}
	for _, l := range strings.Split(c.log.String(), "\n") {
		if i := strings.Index(l, "ao vivo:"); i >= 0 {
			linhas = append(linhas, l[i:])
		}
	}
	if strings.Join(linhas, "|") != "ao vivo: ligado (eu)|ao vivo: desligado (claude)" {
		t.Fatalf("log = %v", linhas)
	}
	if strings.Contains(c.log.String(), quadroFicticio) || strings.Contains(c.log.String(), "cand-") || strings.Contains(c.log.String(), "preenchendo") {
		t.Fatal("o log tem quadro, candidatura ou etapa")
	}
}

func TestAoVivoPausadoELoginSemQuadro(t *testing.T) {
	c := novoAoVivo(t, true)
	if err := c.arm.GravarEnvioPausado(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	ligarComQuadro := func() {
		c.chamar(t, "POST", Objeto{"ligado": true}, "eu", true)
		c.av.Etapa("cand-2026-11-09-0001-abcd", aovivo.Revisando)
		c.av.Quadro(quadroFicticio, 800, 600)
		if corpoDe(c.chamar(t, "GET", nil, "eu", true))["quadro"] == nil {
			t.Fatal("sem quadro antes")
		}
	}
	// Janela de login aberta: login-aberto, sem quadro, e o quadro é apagado.
	ligarComQuadro()
	c.login = true
	o := corpoDe(c.chamar(t, "GET", nil, "eu", true))
	if o["etapa"] != "login-aberto" || o["quadro"] != nil || o["candidatura"] != nil {
		t.Fatalf("login aberto = %v", o)
	}
	c.login = false
	if o := corpoDe(c.chamar(t, "GET", nil, "eu", true)); o["quadro"] != nil {
		t.Fatalf("o quadro voltou depois do login = %v", o)
	}
	// Pausar pela rota apaga o quadro; pausado, a etapa é pausado.
	ligarComQuadro()
	if r := chamarEnvio(t, c.a, "POST", "/envio/pausar", Objeto{}, "claude"); r.Status != 200 {
		t.Fatalf("pausar = %d", r.Status)
	}
	o = corpoDe(c.chamar(t, "GET", nil, "eu", true))
	if o["etapa"] != "pausado" || o["quadro"] != nil || o["candidatura"] != nil {
		t.Fatalf("pausado = %v", o)
	}
	if c.av.Querendo() {
		t.Fatal("pausado e querendo quadros")
	}
}

// O Handler só marca o pedido como da janela com Sec-Fetch-Site same-origin E o Host da janela do app.
func TestAoVivoPeloHandler(t *testing.T) {
	c := novoAoVivo(t, true)
	h := c.a.Handler()
	pedir := func(m, host, site string, corpo string) *httptest.ResponseRecorder {
		var b *strings.Reader
		if corpo == "" {
			b = strings.NewReader("")
		} else {
			b = strings.NewReader(corpo)
		}
		r := httptest.NewRequest(m, "http://"+host+"/api/v1/envio/ao-vivo", b)
		r.Host = host
		if site != "" {
			r.Header.Set("Sec-Fetch-Site", site)
		}
		if m == http.MethodPost {
			r.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	for _, host := range []string{"wails.localhost", "localhost"} {
		if w := pedir("POST", host, "same-origin", `{"ligado":true}`); w.Code != 200 {
			t.Fatalf("ligar pela janela (%s) = %d %s", host, w.Code, w.Body)
		}
		w := pedir("GET", host, "same-origin", "")
		if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("ler pela janela (%s) = %d, Cache-Control %q", host, w.Code, w.Header().Get("Cache-Control"))
		}
	}
	for nome, p := range map[string][2]string{
		"sem Sec-Fetch-Site":   {"wails.localhost", ""},
		"Sec-Fetch-Site none":  {"wails.localhost", "none"},
		"outro site":           {"wails.localhost", "cross-site"},
		"mesmo site, não orig": {"wails.localhost", "same-site"},
		"porta local":          {"127.0.0.1:4500", "same-origin"},
		"localhost com porta":  {"localhost:4500", "same-origin"},
		"host parecido":        {"wails.localhost.exemplo.com", "same-origin"},
	} {
		if w := pedir("GET", p[0], p[1], ""); w.Code != 403 {
			t.Fatalf("ler %s = %d", nome, w.Code)
		}
	}
	if w := pedir("POST", "127.0.0.1:4500", "same-origin", `{"ligado":true}`); w.Code != 403 {
		t.Fatalf("ligar fora da janela = %d", w.Code)
	}
	// Desligar não precisa ser da janela.
	if w := pedir("POST", "127.0.0.1:4500", "", `{"ligado":false}`); w.Code != 200 {
		t.Fatalf("desligar fora da janela = %d", w.Code)
	}
}
