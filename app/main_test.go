package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/api"
	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/conta"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// A janela pede tudo ao mesmo handler: /api/ vai para a API, o resto para as telas embutidas.
func TestRotearMandaApiParaApiETelasParaTelas(t *testing.T) {
	if _, err := os.Stat("frontend/dist/index.html"); err != nil {
		t.Skip("frontend/dist ainda não foi gerado (rode wails3 build)")
	}
	banco, err := armazem.Abrir(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer banco.Fechar()
	h := rotear(api.Nova(banco, api.Opcoes{}).Handler(), application.AssetFileServerFS(telas))

	pedir := func(caminho string) (int, string) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, caminho, nil))
		corpo, _ := io.ReadAll(rec.Body)
		return rec.Code, string(corpo)
	}

	if s, c := pedir("/api/v1/saude"); s != 200 || !strings.Contains(c, `"ok":true`) {
		t.Errorf("/api/v1/saude = %d %q", s, c)
	}
	if s, c := pedir("/"); s != 200 || !strings.Contains(c, "<title>Applyer</title>") {
		t.Errorf("/ = %d; não veio o index.html das telas", s)
	}
}

// Ruling A11: o build sem a nuvem (sem EnderecoNuvem e ChavePublica no ldflags) fica sem conta e sem trava, como antes.
func TestBuildSemNuvemFicaSemConta(t *testing.T) {
	e, c := conta.EnderecoNuvem, conta.ChavePublica
	conta.EnderecoNuvem, conta.ChavePublica = "", ""
	t.Cleanup(func() { conta.EnderecoNuvem, conta.ChavePublica = e, c })
	minha, podeAgir, err := ligarConta(segredos.Memoria{}, t.TempDir())
	if err != nil || minha != nil || podeAgir != nil {
		t.Fatalf("sem nuvem: conta %v, podeAgir %v, erro %v", minha != nil, podeAgir != nil, err)
	}
	o := opcoesDaConta(api.Opcoes{}, minha, podeAgir, nil)
	if o.Conta != nil || o.PodeAgir != nil {
		t.Fatal("sem nuvem, as Opcoes ganharam Conta ou PodeAgir")
	}
}

// Com a nuvem no build, o api.Opcoes do app tem a Conta, a trava (Conta.PodeAgir) e o abrir do navegador.
func TestBuildComNuvemLigaConta(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	e, c := conta.EnderecoNuvem, conta.ChavePublica
	conta.EnderecoNuvem, conta.ChavePublica = "https://nuvem.exemplo.com", base64.StdEncoding.EncodeToString(pub)
	t.Cleanup(func() { conta.EnderecoNuvem, conta.ChavePublica = e, c })
	minha, podeAgir, err := ligarConta(segredos.Memoria{}, t.TempDir())
	if err != nil || minha == nil || podeAgir == nil {
		t.Fatalf("com nuvem: conta %v, podeAgir %v, erro %v", minha != nil, podeAgir != nil, err)
	}
	abriu := ""
	o := opcoesDaConta(api.Opcoes{ContaID: "local"}, minha, podeAgir, func(u string) error { abriu = u; return nil })
	if o.Conta != minha || o.PodeAgir == nil || o.AbrirNavegador == nil || o.ContaID != "local" {
		t.Fatalf("Opcoes = %+v", o)
	}
	if pode, motivo := o.PodeAgir(); pode || motivo != conta.MotivoSemConta {
		t.Fatalf("PodeAgir sem entrar = %v %q", pode, motivo)
	}
	_ = o.AbrirNavegador("https://exemplo.com")
	if abriu != "https://exemplo.com" {
		t.Fatal("AbrirNavegador não é o do app")
	}
	// Chave inválida no build: erro (o app não abre com a nuvem pela metade).
	conta.ChavePublica = "curta"
	if _, _, err := ligarConta(segredos.Memoria{}, t.TempDir()); err == nil {
		t.Fatal("chave inválida passou")
	}
}
