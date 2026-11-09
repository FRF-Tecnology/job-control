package api

import (
	"bytes"
	"context"
	"log"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// jc-a2y.6.11: pausar e retomar uma plataforma. Pausar: qualquer autor; retomar: só a pessoa (x-so-tela). Parada
// até amanhã dá 409. GET /envio traz as três, só com estado, motivo e até.
func TestPausarERetomarPlataforma(t *testing.T) {
	var saida bytes.Buffer
	log.SetOutput(&saida)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	a, arm := apiDeEnvio(t, true)
	plataformaDe := func(r Resposta, nome string) map[string]any {
		t.Helper()
		for _, p := range r.Corpo.(map[string]any)["plataformas"].([]any) {
			if o := p.(map[string]any); o["plataforma"] == nome {
				return o
			}
		}
		t.Fatalf("sem %s em %v", nome, r.Corpo)
		return nil
	}

	r := chamarEnvio(t, a, "GET", "/envio", nil, "eu")
	lista := r.Corpo.(map[string]any)["plataformas"].([]any)
	if len(lista) != 3 || plataformaDe(r, "linkedin")["estado"] != "ativa" {
		t.Fatalf("plataformas = %v", lista)
	}
	for _, p := range lista {
		var chaves []string
		for k := range p.(map[string]any) {
			chaves = append(chaves, k)
		}
		sort.Strings(chaves)
		if strings.Join(chaves, ",") != "ate,estado,motivo,paginasHoje,plataforma,tetoPaginas,voltasHoje" {
			t.Fatalf("campos = %v", chaves)
		}
	}

	// O assistente pausa; não retoma.
	if r := chamarEnvio(t, a, "POST", "/envio/plataformas/linkedin/pausar", Objeto{}, "claude"); r.Status != 200 || plataformaDe(r, "linkedin")["estado"] != "pausada" {
		t.Fatalf("assistente pausar = %d %v", r.Status, r.Corpo)
	}
	if r := chamarEnvio(t, a, "POST", "/envio/plataformas/linkedin/retomar", Objeto{}, "claude"); r.Status != 403 {
		t.Fatalf("assistente retomar = %d", r.Status)
	}
	if r := chamarEnvio(t, a, "POST", "/envio/plataformas/linkedin/retomar", Objeto{"x": 1}, "eu"); r.Status != 400 {
		t.Fatalf("retomar com campo = %d", r.Status)
	}
	if r := chamarEnvio(t, a, "GET", "/envio", nil, "eu"); plataformaDe(r, "linkedin")["estado"] != "pausada" {
		t.Fatal("a recusa retomou")
	}
	if r := chamarEnvio(t, a, "POST", "/envio/plataformas/linkedin/retomar", Objeto{}, "eu"); r.Status != 200 || plataformaDe(r, "linkedin")["estado"] != "ativa" {
		t.Fatalf("pessoa retomar = %d %v", r.Status, r.Corpo)
	}
	if r := chamarEnvio(t, a, "POST", "/envio/plataformas/linkedin/retomar", Objeto{}, "eu"); r.Status != 409 {
		t.Fatalf("retomar ativa = %d", r.Status)
	}
	if r := chamarEnvio(t, a, "POST", "/envio/plataformas/outra/pausar", Objeto{}, "eu"); r.Status != 404 {
		t.Fatalf("plataforma sem envio = %d", r.Status)
	}

	// Parada até amanhã (captcha): retomar dá 409; o GET mostra motivo e até.
	agora := time.Now()
	arm.MudarPlataformas(context.Background(), func(p dominio.Plataformas, _ bool) (dominio.Plataformas, error) {
		return p.RegistrarResultado("gupy", "captcha", agora), nil
	})
	if r := chamarEnvio(t, a, "POST", "/envio/plataformas/gupy/retomar", Objeto{}, "eu"); r.Status != 409 {
		t.Fatalf("retomar parada-hoje = %d", r.Status)
	}
	r = chamarEnvio(t, a, "GET", "/envio", nil, "eu")
	if g := plataformaDe(r, "gupy"); g["estado"] != "parada-hoje" || g["motivo"] != "captcha" || g["ate"] != dominio.RenovaEm(agora).Format(time.RFC3339) {
		t.Fatalf("gupy = %v", g)
	}

	// Build padrão: 404.
	b, _ := apiDeEnvio(t, false)
	if r := chamarEnvio(t, b, "POST", "/envio/plataformas/linkedin/pausar", Objeto{}, "eu"); r.Status != 404 {
		t.Fatalf("build padrão = %d", r.Status)
	}
	got := saida.String()
	if !strings.Contains(got, "envio: plataforma linkedin pausada (claude)") || !strings.Contains(got, "envio: plataforma linkedin retomada (eu)") || strings.Count(got, "envio: plataforma") != 2 {
		t.Fatalf("log = %q", got)
	}
}
