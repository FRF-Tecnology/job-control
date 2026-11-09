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
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// jc-a2y.63: a chave nunca sai do cofre. Nas respostas, só o final (4 caracteres); em log e em erro, nada. Vale
// também quando o cofre falha com um texto que traz a chave (um cofre com defeito não pode vazar pela API).

const chaveMarcada = "sk-ant-ficticia-MARCADOR-COFRE-1234"

// cofreQueFalha recusa tudo com um texto que traz a chave e "não encontrada" (o caminho do 404 da API).
type cofreQueFalha struct{ chave string }

func (c cofreQueFalha) erro() error {
	return errors.New("cofre: " + c.chave + " não encontrada")
}
func (c cofreQueFalha) Ler(string) (string, error)  { return "", c.erro() }
func (c cofreQueFalha) Gravar(string, string) error { return c.erro() }
func (c cofreQueFalha) Apagar(string) error         { return c.erro() }

func apiComCofre(t *testing.T, cofre segredos.Cofre) *API {
	t.Helper()
	arm, err := armazem.Abrir(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	return Nova(arm, Opcoes{Hoje: func() string { return "2026-11-09" }, Cofre: cofre, ProvedoresIA: map[string]ia.Provedor{},
		DetectarCLIs: func(context.Context) []ia.EstadoCLI { return nil }})
}

func TestChaveNuncaSaiDoCofre(t *testing.T) {
	var saida bytes.Buffer
	log.SetOutput(&saida)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	a := apiComCofre(t, segredos.Memoria{})
	s, corpo := chamarHTTP(t, a, "PUT", "/api/v1/ia/chaves/anthropic", `{"chave":"`+chaveMarcada+`"}`)
	if s != 200 || !strings.Contains(corpo, "…1234") {
		t.Fatalf("gravar = %d %s", s, corpo)
	}
	for _, alvo := range []string{"/api/v1/ia", "/api/v1/ia/config", "/api/v1/estado"} {
		s, c := chamarHTTP(t, a, "GET", alvo, "")
		if s != 200 {
			t.Fatalf("GET %s = %d", alvo, s)
		}
		corpo += c
	}
	if strings.Contains(corpo, "MARCADOR") || strings.Contains(corpo, "sk-ant-ficticia") {
		t.Errorf("a chave saiu numa resposta: %s", corpo)
	}

	// Cofre com defeito: o texto do erro traz a chave. Nenhuma resposta nem o log mostram.
	f := apiComCofre(t, cofreQueFalha{chave: chaveMarcada})
	respostas := ""
	for _, p := range []struct{ m, alvo, corpo string }{
		{"PUT", "/api/v1/ia/chaves/anthropic", `{"chave":"` + chaveMarcada + `"}`},
		{"DELETE", "/api/v1/ia/chaves/anthropic", ""},
		{"GET", "/api/v1/ia", ""},
	} {
		s, c := chamarHTTP(t, f, p.m, p.alvo, p.corpo)
		if p.m != "GET" && s < 400 {
			t.Errorf("%s %s com o cofre falhando = %d", p.m, p.alvo, s)
		}
		respostas += c
	}
	if strings.Contains(respostas, "MARCADOR") || strings.Contains(respostas, "encontrada") {
		t.Errorf("o erro do cofre saiu na resposta: %s", respostas)
	}
	if strings.Contains(saida.String(), "MARCADOR") {
		t.Errorf("a chave foi para o log: %s", saida.String())
	}
}
