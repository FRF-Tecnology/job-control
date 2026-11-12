package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func chamadaRascunho(t *testing.T, a *API, metodo, rota string, dados Objeto, prova string, esperado int) Objeto {
	t.Helper()
	b, err := json.Marshal(dados)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(metodo, Prefixo+rota, strings.NewReader(string(b)))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	r.Header.Set("X-Applyer-Onboarding", "1")
	if prova != "" {
		r.Header.Set("X-Applyer-Verificacao", prova)
	}
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	if w.Code != esperado {
		t.Fatalf("%s %s: status %d, queria %d: %s", metodo, rota, w.Code, esperado, w.Body.String())
	}
	var obj Objeto
	if err = json.Unmarshal(w.Body.Bytes(), &obj); err != nil {
		t.Fatal(err)
	}
	return obj
}

func TestRascunhoReabrePreservaEDescartaPDFComRevisao(t *testing.T) {
	pasta := t.TempDir()
	a := apiPrimeiroAcesso(t, pasta)
	const rota = "/onboarding/rascunho"
	const cv = "/onboarding/curriculo"
	inicial := chamadaRascunho(t, a, "GET", rota, nil, "", 200)
	if inicial["revisao"] != 0.0 || inicial["etapa"] != 1.0 || inicial["curriculo"] != nil {
		t.Fatal(inicial)
	}
	chamadaRascunho(t, a, "PATCH", rota, Objeto{"etapa": 1}, "", 400)
	chamadaRascunho(t, a, "PATCH", rota, Objeto{"revisao": false, "etapa": 1}, "", 400)
	pdf := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4\n%%EOF"))
	meta := chamadaRascunho(t, a, "PUT", cv, Objeto{"revisao": 0, "curriculoBase64": pdf, "nomeArquivo": "pasta/CV.pdf"}, "", 200)
	if meta["nomeExibicao"] != "CV.pdf" || meta["revisao"] != 1.0 || meta["arquivoId"] == "" {
		t.Fatal(meta)
	}
	chamadaRascunho(t, a, "PATCH", rota, Objeto{"revisao": 1, "etapa": 2, "dados": Objeto{"cargo": "Designer", "principal": "Figma"}, "editados": []string{"cargo"}}, "", 200)
	chamadaRascunho(t, a, "PUT", cv, Objeto{"revisao": 2, "nomeArquivo": "invalido.pdf", "curriculoBase64": "aW52YWxpZG8="}, "", 400)
	chamadaRascunho(t, a, "DELETE", cv, Objeto{"revisao": 1}, "", 409)
	if err := a.arm.Fechar(); err != nil {
		t.Fatal(err)
	}
	b := apiPrimeiroAcesso(t, pasta)
	estado := estadoInicial(t, b)
	if estado["onboardingConcluido"] != false || len(estado["curriculos"].([]Curriculo)) != 0 || estado["perfil"].(Objeto)["cargo"] != nil {
		t.Fatalf("rascunho virou perfil: %v", estado)
	}
	salvo := chamadaRascunho(t, b, "GET", rota, nil, "", 200)
	if salvo["etapa"] != 2.0 || salvo["revisao"] != 2.0 || salvo["curriculo"].(Objeto)["arquivoId"] != meta["arquivoId"] || salvo["dados"].(Objeto)["principal"] != "Figma" {
		t.Fatal(salvo)
	}
	limpeza := chamadaRascunho(t, b, "DELETE", cv, Objeto{"revisao": 2}, "", 200)
	if limpeza["etapa"] != 1.0 || limpeza["curriculo"] != nil || limpeza["dados"].(Objeto)["cargo"] != "Designer" || limpeza["dados"].(Objeto)["principal"] != nil {
		t.Fatal(limpeza)
	}
	chamadaRascunho(t, b, "POST", "/onboarding/sugerir", Objeto{"arquivoId": meta["arquivoId"], "revisao": 3}, "", 412)
}

func TestRascunhoPromocaoExigeProvaERepeticaoNaoDuplica(t *testing.T) {
	a := apiPrimeiroAcesso(t, t.TempDir())
	pdf := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4\n%%EOF"))
	meta := chamadaRascunho(t, a, "PUT", "/onboarding/curriculo", Objeto{"revisao": 0, "nomeArquivo": "CV.pdf", "curriculoBase64": pdf}, "", 200)
	var perfil Objeto
	if err := json.Unmarshal([]byte(perfilInicialJSON), &perfil); err != nil {
		t.Fatal(err)
	}
	fim := Objeto{"arquivoId": meta["arquivoId"], "revisao": 1, "perfil": perfil}
	chamadaRascunho(t, a, "POST", "/onboarding", fim, "", 412)
	gen := a.provaIA.invalidar()
	a.provaIA.confirmar(gen)
	prova := fmt.Sprint(gen | 1)
	chamadaRascunho(t, a, "POST", "/onboarding", Objeto{"arquivoId": "outra-pasta", "revisao": 1, "perfil": perfil}, prova, 409)
	chamadaRascunho(t, a, "POST", "/onboarding", fim, prova, 200)
	chamadaRascunho(t, a, "POST", "/onboarding", fim, prova, 200)
	estado := estadoInicial(t, a)
	if estado["onboardingConcluido"] != true || len(estado["curriculos"].([]Curriculo)) != 1 || estado["perfil"].(Objeto)["cargo"] != "Designer" {
		t.Fatal(estado)
	}
	chamadaRascunho(t, a, "DELETE", "/onboarding/curriculo", Objeto{"revisao": 1}, "", 409)
}
