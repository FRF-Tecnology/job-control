package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/conta"
	"github.com/FRF-Tecnology/job-control/internal/licenca"
)

// semRede troca o transporte padrão por um que falha o teste: a CLI nunca fala com a nuvem.
type semRede struct{ t *testing.T }

func (s semRede) RoundTrip(r *http.Request) (*http.Response, error) {
	s.t.Errorf("a CLI chamou a rede: %s", r.URL.Host)
	return nil, errors.New("sem rede no teste")
}

// buildComNuvem: as variáveis do build preenchidas (como o ldflags -X), com uma chave gerada agora.
func buildComNuvem(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	e, c, tr := conta.EnderecoNuvem, conta.ChavePublica, http.DefaultTransport
	conta.EnderecoNuvem, conta.ChavePublica = "https://nuvem.exemplo.com", base64.StdEncoding.EncodeToString(pub)
	http.DefaultTransport = semRede{t}
	t.Cleanup(func() { conta.EnderecoNuvem, conta.ChavePublica, http.DefaultTransport = e, c, tr })
	return priv
}

func gravarLicencaCLI(t *testing.T, pasta string, priv ed25519.PrivateKey, emitida time.Time) {
	t.Helper()
	d, err := licenca.Assinar(priv, licenca.Licenca{Usuario: "u1", Email: "ana@exemplo.com", Estado: licenca.Teste,
		EmitidaEm: emitida, ValidaAte: emitida.Add(30 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if err := conta.GravarLicenca(pasta, d); err != nil {
		t.Fatal(err)
	}
}

const vagaTrava = `{"vagas":[{"id":"v-trava","empresa":"Empresa Teste","titulo":"Dev Go","link":"https://exemplo.com/trava"}]}`

// Task 6: a CLI só lê licenca.json (sem rede, sem renovar). Válida: escreve; vencida: 402 com a frase de abrir o
// app; sem arquivo: sem-conta. Ler continua liberado.
func TestCLISoLeLicenca(t *testing.T) {
	priv := buildComNuvem(t)

	pasta := pastaDoApp(t)
	if _, err := cli(t, "vaga_adicionar", vagaTrava); !errors.Is(err, errRecusado) || !strings.Contains(err.Error(), "Entre na sua conta") {
		t.Fatalf("sem licenca.json = %v", err)
	}
	if saida, err := cli(t, "vagas_listar", `{}`); err != nil {
		t.Fatalf("ler sem licença = %q %v", saida, err)
	}

	gravarLicencaCLI(t, pasta, priv, time.Now().Add(-time.Hour)) // venceu há meia hora
	if _, err := cli(t, "vaga_adicionar", vagaTrava); !errors.Is(err, errRecusado) || !strings.Contains(err.Error(), "Abra o Applyer para renovar o acesso.") {
		t.Fatalf("vencida = %v", err)
	}

	gravarLicencaCLI(t, pasta, priv, time.Now())
	if _, err := cli(t, "vaga_adicionar", vagaTrava); err != nil {
		t.Fatalf("válida = %v", err)
	}
}

// Ruling A11: build sem nuvem, CLI como antes (sem trava).
func TestCLISemNuvemSemTrava(t *testing.T) {
	e, c := conta.EnderecoNuvem, conta.ChavePublica
	conta.EnderecoNuvem, conta.ChavePublica = "", ""
	t.Cleanup(func() { conta.EnderecoNuvem, conta.ChavePublica = e, c })
	pastaDoApp(t)
	if _, err := cli(t, "vaga_adicionar", vagaTrava); err != nil {
		t.Fatalf("sem nuvem = %v", err)
	}
}
