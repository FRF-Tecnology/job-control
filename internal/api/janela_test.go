package api

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// jc-a2y.6.7: o envio roda sem janela; só a pessoa, pela tela, mostra o envio ou abre o login. Ganchos falsos:
// nenhum navegador abre neste teste.

type janelaFalsa struct {
	visivel, login bool
	enderecos      []string
	erro           error
}

func apiComJanela(t *testing.T, ligado bool) (*API, *armazem.Armazem, *janelaFalsa) {
	t.Helper()
	arm, err := armazem.Abrir(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	j := &janelaFalsa{}
	a := Nova(arm, Opcoes{Hoje: func() string { return "2026-11-09" }, Cofre: segredos.Memoria{}, ProvedoresIA: map[string]ia.Provedor{},
		DetectarCLIs: func(context.Context) []ia.EstadoCLI { return nil }, EnvioLigado: ligado,
		MostrarEnvio: func(v bool) { j.visivel = v },
		Entrar: func(e string) error {
			if j.erro != nil {
				return j.erro
			}
			j.enderecos = append(j.enderecos, e)
			j.login = true
			return nil
		},
		Janela: func() (bool, bool) { return j.visivel, j.login }})
	return a, arm, j
}

func estadoDe(t *testing.T, r Resposta) map[string]any {
	t.Helper()
	c, ok := r.Corpo.(map[string]any)
	if !ok {
		t.Fatalf("corpo = %v", r.Corpo)
	}
	return c
}

func TestJanelaDoEnvio(t *testing.T) {
	var saida bytes.Buffer
	log.SetOutput(&saida)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	// Build padrão: sempre oculta, e as duas rotas dão 404 (nem o gancho é chamado).
	p, _, jp := apiComJanela(t, false)
	if c := estadoDe(t, chamarEnvio(t, p, "GET", "/envio", nil, "eu")); c["janela"] != "oculta" || c["login"] != false {
		t.Fatalf("build padrão = %v", c)
	}
	if r := chamarEnvio(t, p, "POST", "/envio/janela", Objeto{"visivel": true}, "eu"); r.Status != 404 || jp.visivel {
		t.Fatalf("janela no build padrão = %d", r.Status)
	}

	a, _, j := apiComJanela(t, true)
	if c := estadoDe(t, chamarEnvio(t, a, "GET", "/envio", nil, "eu")); c["janela"] != "oculta" || c["login"] != false {
		t.Fatalf("ao abrir, sem janela = %v", c)
	}
	for _, corpo := range []Objeto{{}, {"visivel": "sim"}, {"visivel": true, "x": 1}} {
		if r := chamarEnvio(t, a, "POST", "/envio/janela", corpo, "eu"); r.Status != 400 {
			t.Errorf("corpo %v = %d", corpo, r.Status)
		}
	}
	// Só a pessoa abre a janela.
	if r := chamarEnvio(t, a, "POST", "/envio/janela", Objeto{"visivel": true}, "claude"); r.Status != 403 || j.visivel {
		t.Fatalf("assistente abre a janela = %d (visível %v)", r.Status, j.visivel)
	}
	if r := chamarEnvio(t, a, "POST", "/envio/janela", Objeto{"visivel": true}, "eu"); r.Status != 200 || estadoDe(t, r)["janela"] != "visivel" || !j.visivel {
		t.Fatalf("pessoa abre a janela = %d %v", r.Status, r.Corpo)
	}
	// Qualquer autor fecha (reduz a interferência, como pausar).
	if r := chamarEnvio(t, a, "POST", "/envio/janela", Objeto{"visivel": false}, "claude"); r.Status != 200 || estadoDe(t, r)["janela"] != "oculta" || j.visivel {
		t.Fatalf("assistente fecha a janela = %d %v", r.Status, r.Corpo)
	}
	if got := saida.String(); !strings.Contains(got, "envio: janela visível (eu)") || !strings.Contains(got, "envio: janela oculta (claude)") || strings.Count(got, "envio: janela") != 2 {
		t.Fatalf("log = %q", got)
	}
}

func TestEntrarNasPlataformas(t *testing.T) {
	var saida bytes.Buffer
	log.SetOutput(&saida)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	p, _, jp := apiComJanela(t, false)
	if r := chamarEnvio(t, p, "POST", "/navegador/entrar", Objeto{"plataforma": "linkedin"}, "eu"); r.Status != 404 || len(jp.enderecos) != 0 {
		t.Fatalf("entrar no build padrão = %d", r.Status)
	}

	a, arm, j := apiComJanela(t, true)
	const vagaGupy = "https://empresa-teste.gupy.io/jobs/9000001"
	if err := arm.Gravar(context.Background(), func(e dominio.Estado) (dominio.Estado, error) {
		e.Candidaturas = []Objeto{
			{"id": "cand-gupy", "vagaId": "v-1", "plataforma": "gupy", "estado": "aprovada", "url": vagaGupy},
			{"id": "cand-fora", "vagaId": "v-2", "plataforma": "outra", "estado": "aprovada", "url": "https://carreiras.exemplo.com/1"},
			{"id": "cand-falsa", "vagaId": "v-3", "plataforma": "linkedin", "estado": "aprovada", "url": "https://linkedin.com.exemplo.com/jobs/view/9000000001"},
		}
		return e, nil
	}); err != nil {
		t.Fatal(err)
	}
	recusas := []struct {
		corpo  Objeto
		autor  string
		status int
	}{
		{Objeto{"plataforma": "linkedin"}, "claude", 403}, // só a pessoa
		{Objeto{}, "eu", 400},                             // nenhum dos dois
		{Objeto{"plataforma": "linkedin", "candidatura": "cand-gupy"}, "eu", 400},
		{Objeto{"plataforma": "gupy"}, "eu", 400}, // login por empresa: pela candidatura
		{Objeto{"plataforma": "linkedin", "url": "https://exemplo.com"}, "eu", 400},
		{Objeto{"candidatura": "cand-nao-existe"}, "eu", 404},
		{Objeto{"candidatura": "cand-fora"}, "eu", 400},  // fora da lista fechada
		{Objeto{"candidatura": "cand-falsa"}, "eu", 400}, // host parecido, não é o LinkedIn
	}
	for _, c := range recusas {
		if r := chamarEnvio(t, a, "POST", "/navegador/entrar", c.corpo, c.autor); r.Status != c.status {
			t.Errorf("%v (%s) = %d; queria %d", c.corpo, c.autor, r.Status, c.status)
		}
	}
	if len(j.enderecos) != 0 {
		t.Fatalf("abriu o navegador numa recusa: %v", j.enderecos)
	}

	r := chamarEnvio(t, a, "POST", "/navegador/entrar", Objeto{"plataforma": "linkedin"}, "eu")
	if r.Status != 200 || estadoDe(t, r)["login"] != true || len(j.enderecos) != 1 || j.enderecos[0] != "https://www.linkedin.com/login" {
		t.Fatalf("entrar no LinkedIn = %d %v %v", r.Status, r.Corpo, j.enderecos)
	}
	if r := chamarEnvio(t, a, "POST", "/navegador/entrar", Objeto{"candidatura": "cand-gupy"}, "eu"); r.Status != 200 || j.enderecos[1] != vagaGupy {
		t.Fatalf("entrar pela vaga da Gupy = %d %v", r.Status, j.enderecos)
	}

	// O envio recusa (login já aberto): 409 com o texto dele; erro do sistema vira texto fixo, sem o detalhe.
	j.erro = &dominio.ErroConflito{Msg: "já há uma janela de login aberta: entre por ela ou feche-a antes"}
	if r := chamarEnvio(t, a, "POST", "/navegador/entrar", Objeto{"plataforma": "linkedin"}, "eu"); r.Status != 409 || !strings.Contains(estadoDe(t, r)["erro"].(string), "login aberta") {
		t.Fatalf("login já aberto = %d %v", r.Status, r.Corpo)
	}
	j.erro = errors.New(`exec: "C:\Users\Pessoa Ficticia\chrome.exe": acesso negado`)
	if r := chamarEnvio(t, a, "POST", "/navegador/entrar", Objeto{"plataforma": "linkedin"}, "eu"); r.Status != 409 || strings.Contains(estadoDe(t, r)["erro"].(string), "Ficticia") {
		t.Fatalf("erro do sistema = %d %v", r.Status, r.Corpo)
	}

	// Com candidatura em enviando, nada abre (o executor está numa página).
	j.erro = nil
	arm.Gravar(context.Background(), func(e dominio.Estado) (dominio.Estado, error) {
		e.Candidaturas = append(e.Candidaturas, Objeto{"id": "cand-agora", "vagaId": "v-4", "plataforma": "linkedin", "estado": "enviando", "url": "https://www.linkedin.com/jobs/view/9000000009"})
		return e, nil
	})
	antes := len(j.enderecos)
	if r := chamarEnvio(t, a, "POST", "/navegador/entrar", Objeto{"plataforma": "linkedin"}, "eu"); r.Status != 409 || len(j.enderecos) != antes {
		t.Fatalf("com envio em andamento = %d", r.Status)
	}

	// Log: só o fato e o autor; nenhum endereço.
	got := saida.String()
	if strings.Count(got, "navegador: janela de login aberta (eu)") != 2 || strings.Contains(got, "linkedin.com") || strings.Contains(got, "gupy.io") {
		t.Fatalf("log = %q", got)
	}
}
