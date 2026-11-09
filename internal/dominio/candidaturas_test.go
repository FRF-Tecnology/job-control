package dominio

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
	_ "time/tzdata" // fusos iguais no Windows, no macOS e no Linux
)

// Dados fictícios: vagas e pessoa de teste, nenhum endereço de vaga real.

func fuso(t *testing.T, nome string) *time.Location {
	t.Helper()
	l, err := time.LoadLocation(nome)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

var eu = Ctx{Autor: "eu", Agora: "2026-11-09", Aleatorio: func() string { return "abcd" }}
var claude = Ctx{Autor: "claude", Agora: "2026-11-09", Aleatorio: func() string { return "abcd" }}

func conteudoOK() Objeto {
	return Objeto{
		"curriculo": Objeto{"arquivo": "cv-pt.pdf", "sha256": strings.Repeat("ab", 32)},
		"idioma":    "pt",
		"contato":   Objeto{"nome": "Pessoa Teste", "email": "pessoa@exemplo.com"},
		"pretensao": Objeto{"valor": 8500.0, "moeda": "BRL", "tipo": "PJ"},
		"respostas": []any{Objeto{"pergunta": "Anos de Go?", "resposta": "3", "origem": "respostasPadrao"}},
		"mensagem":  nil,
	}
}

// comVaga adiciona uma vaga fictícia e, se aprovada, move para aplicar como a pessoa (Triagem).
func comVaga(t *testing.T, e Estado, id, plataforma, link string, aprovada bool) Estado {
	t.Helper()
	e, _, err := AdicionarVaga(e, Objeto{"id": id, "empresa": "Empresa " + id, "titulo": "Dev Go " + id, "plataforma": plataforma, "link": link}, eu, false)
	if err != nil {
		t.Fatal(err)
	}
	if aprovada {
		if e, _, err = MoverVaga(e, id, "aplicar", nil, eu); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

func linkLI(n string) string { return "https://www.linkedin.com/jobs/view/900000000" + n }

func preparada(t *testing.T, e Estado, vagaID string, agora time.Time) (Estado, Candidatura) {
	t.Helper()
	e, o, err := CriarCandidatura(e, vagaID, conteudoOK(), agora, eu)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := candidaturaDe(o)
	return e, c
}

func aprovada(t *testing.T, e Estado, vagaID string, agora time.Time, limites map[string]int) (Estado, Candidatura) {
	t.Helper()
	e, c := preparada(t, e, vagaID, agora)
	e, o, err := AprovarCandidatura(e, c.ID, c.Impressao, agora, limites, eu)
	if err != nil {
		t.Fatal(err)
	}
	c, _ = candidaturaDe(o)
	return e, c
}

func estadoDe(t *testing.T, e Estado, id string) Candidatura {
	t.Helper()
	_, c, err := acharCandidatura(e, id)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func etapaDe(e Estado, id string) any { return vagaDe(e, id)["etapa"] }

func conflito(t *testing.T, err error) *ErroConflito {
	t.Helper()
	var c *ErroConflito
	if !errors.As(err, &c) {
		t.Fatalf("erro = %v (%T); queria ErroConflito", err, err)
	}
	return c
}

func emJSONTeste(e Estado) string { b, _ := json.Marshal(e); return string(b) }

func TestURLDaPlataforma(t *testing.T) {
	casos := []struct {
		plataforma, url string
		ok              bool
	}{
		{"linkedin", "https://www.linkedin.com/jobs/view/1", true},
		{"linkedin", "https://linkedin.com/jobs/view/1", true},
		{"linkedin", "https://WWW.LinkedIn.com./jobs/view/1", true},
		{"gupy", "https://empresa-teste.gupy.io/jobs/1", true},
		{"inhire", "https://empresa.inhire.app/vagas/1", true},
		{"linkedin", "http://www.linkedin.com/jobs/view/1", false},
		{"linkedin", "https://evil-linkedin.com/jobs", false},
		{"linkedin", "https://linkedin.com.exemplo.io/jobs", false},
		{"linkedin", "https://linkedin.com@exemplo.io/jobs", false},
		{"linkedin", "https://usuario:x@www.linkedin.com/jobs", false},
		{"linkedin", "https://www.gupy.io/jobs", false},
		{"gupy", "javascript:alert(1)", false},
		{"inhire", "https:///sem-host", false},
		{"outra", "https://carreiras.exemplo.com/vaga", true},
		{"outra", "http://carreiras.exemplo.com/vaga", false},
		{"desconhecida", "https://www.linkedin.com/jobs", false},
	}
	for _, c := range casos {
		if got := URLDaPlataforma(c.plataforma, c.url); got != c.ok {
			t.Errorf("URLDaPlataforma(%q, %q) = %v; queria %v", c.plataforma, c.url, got, c.ok)
		}
	}
}

func TestValidarConteudo(t *testing.T) {
	if erros := ValidarConteudo(conteudoOK()); len(erros) > 0 {
		t.Fatalf("conteúdo bom recusado: %v", erros)
	}
	mudar := func(f func(Objeto)) Objeto { c := clonarObj(conteudoOK()); f(c); return c }
	casos := map[string]Objeto{
		"nil":                    nil,
		"campo a mais":           mudar(func(c Objeto) { c["cpf"] = "x" }),
		"campo a mais curriculo": mudar(func(c Objeto) { c["curriculo"].(Objeto)["caminho"] = "C:/x.pdf" }),
		"campo a mais contato":   mudar(func(c Objeto) { c["contato"].(Objeto)["rg"] = "x" }),
		"campo a mais pretensao": mudar(func(c Objeto) { c["pretensao"].(Objeto)["banco"] = "x" }),
		"campo a mais resposta":  mudar(func(c Objeto) { c["respostas"].([]any)[0].(Objeto)["extra"] = "x" }),
		"sem curriculo":          mudar(func(c Objeto) { delete(c, "curriculo") }),
		"arquivo com pasta":      mudar(func(c Objeto) { c["curriculo"].(Objeto)["arquivo"] = "../cv.pdf" }),
		"arquivo com barra":      mudar(func(c Objeto) { c["curriculo"].(Objeto)["arquivo"] = `pasta\cv.pdf` }),
		"arquivo com drive":      mudar(func(c Objeto) { c["curriculo"].(Objeto)["arquivo"] = "C:cv.pdf" }),
		"arquivo nao pdf":        mudar(func(c Objeto) { c["curriculo"].(Objeto)["arquivo"] = "cv.exe" }),
		"sem sha":                mudar(func(c Objeto) { delete(c["curriculo"].(Objeto), "sha256") }),
		"sha curto":              mudar(func(c Objeto) { c["curriculo"].(Objeto)["sha256"] = "abc" }),
		"idioma":                 mudar(func(c Objeto) { c["idioma"] = "es" }),
		"contato nao objeto":     mudar(func(c Objeto) { c["contato"] = "x" }),
		"contato numero":         mudar(func(c Objeto) { c["contato"].(Objeto)["telefone"] = 11.0 }),
		"moeda":                  mudar(func(c Objeto) { c["pretensao"].(Objeto)["moeda"] = "BTC" }),
		"tipo":                   mudar(func(c Objeto) { c["pretensao"].(Objeto)["tipo"] = "estagio" }),
		"valor texto":            mudar(func(c Objeto) { c["pretensao"].(Objeto)["valor"] = "8500" }),
		"respostas nao lista":    mudar(func(c Objeto) { c["respostas"] = Objeto{} }),
		"origem":                 mudar(func(c Objeto) { c["respostas"].([]any)[0].(Objeto)["origem"] = "pagina" }),
		"resposta sem texto":     mudar(func(c Objeto) { delete(c["respostas"].([]any)[0].(Objeto), "resposta") }),
		"mensagem numero":        mudar(func(c Objeto) { c["mensagem"] = 1.0 }),
		"cpf na resposta":        mudar(func(c Objeto) { c["respostas"].([]any)[0].(Objeto)["resposta"] = "CPF 123.456.789-00" }),
		"cpf na mensagem":        mudar(func(c Objeto) { c["mensagem"] = "meu cpf: 12345678900" }),
		"rg no contato":          mudar(func(c Objeto) { c["contato"].(Objeto)["cidade"] = "RG 12.345.678-9" }),
	}
	for nome, c := range casos {
		if erros := ValidarConteudo(c); len(erros) == 0 {
			t.Errorf("%s: deveria recusar", nome)
		}
	}
	// contato, pretensão e mensagem podem ser null ou faltar.
	ok := mudar(func(c Objeto) { c["contato"], c["pretensao"] = nil, nil; delete(c, "mensagem") })
	if erros := ValidarConteudo(ok); len(erros) > 0 {
		t.Errorf("null recusado: %v", erros)
	}
}

// Caminho feliz: cria, aprova, envia; a vaga vai para enviada na mesma operação; o estado original não muda.
func TestCicloDaCandidatura(t *testing.T) {
	sp := fuso(t, "America/Sao_Paulo")
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, sp)
	e0 := comVaga(t, EstadoVazio(), "li-1", "LinkedIn", linkLI("1"), true)
	antes := emJSONTeste(e0)

	e1, o, err := CriarCandidatura(e0, "li-1", conteudoOK(), agora, eu)
	if err != nil {
		t.Fatal(err)
	}
	if emJSONTeste(e0) != antes {
		t.Fatal("CriarCandidatura mexeu no estado original")
	}
	c, _ := candidaturaDe(o)
	if c.ID != "cand-2026-11-09-0001-abcd" || c.Estado != CandPreparada || c.Plataforma != "linkedin" || c.Versao != 1 || c.URL != linkLI("1") {
		t.Fatalf("candidatura criada = %+v", c)
	}
	quer, _ := ImpressaoCandidatura("li-1", "linkedin", linkLI("1"), conteudoOK())
	if c.Impressao != quer {
		t.Fatalf("impressão = %s; queria %s", c.Impressao, quer)
	}

	e2, o, err := AprovarCandidatura(e1, c.ID, c.Impressao, agora, nil, eu)
	if err != nil {
		t.Fatal(err)
	}
	c, _ = candidaturaDe(o)
	if c.Estado != CandAprovada || c.Aprovacao == nil || c.Aprovacao.ValidaAte != "2026-11-09T23:59:59-03:00" || c.Aprovacao.Fuso != "America/Sao_Paulo" || c.Aprovacao.Impressao != c.Impressao || c.Versao != 2 {
		t.Fatalf("aprovação = %+v %+v", c, c.Aprovacao)
	}

	e3, o, err := IniciarEnvio(e2, c.ID, c.Versao, agora.Add(time.Minute), nil)
	if err != nil {
		t.Fatal(err)
	}
	c, _ = candidaturaDe(o)
	if c.Estado != CandEnviando || c.Aprovacao.UsadaEm == "" || len(c.Tentativas) != 1 {
		t.Fatalf("enviando = %+v", c)
	}
	e4, o, err := MarcarEnviada(e3, c.ID, agora.Add(2*time.Minute), eu)
	if err != nil {
		t.Fatal(err)
	}
	c, _ = candidaturaDe(o)
	if c.Estado != CandEnviada || c.Resultado == nil || c.Resultado.Tipo != "enviada" {
		t.Fatalf("enviada = %+v", c)
	}
	v := vagaDe(e4, "li-1")
	if v["etapa"] != "enviada" || !FoiEnviada(v) || obj(v["envio"])["candidaturaId"] != c.ID {
		t.Fatalf("vaga depois do envio = %v", v)
	}
	if etapaDe(e3, "li-1") != "aplicar" {
		t.Fatal("MarcarEnviada mexeu no estado anterior")
	}
	// Vaga enviada: não cria outra candidatura.
	if _, _, err := CriarCandidatura(e4, "li-1", conteudoOK(), agora, eu); conflito(t, err).Existente != "li-1" {
		t.Fatalf("duplicata = %v", err)
	}
}

// M4: criar, editar, aprovar, cancelar, reabrir e concluir são só da pessoa; revogar não (reduz o risco).
func TestSoAPessoaMudaAFila(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	e := comVaga(t, EstadoVazio(), "li-1", "linkedin", linkLI("1"), true)
	var so *ErroSoPessoa
	if _, _, err := CriarCandidatura(e, "li-1", conteudoOK(), agora, claude); !errors.As(err, &so) {
		t.Fatalf("criar como claude = %v", err)
	}
	e, c := preparada(t, e, "li-1", agora)
	if _, _, err := AprovarCandidatura(e, c.ID, c.Impressao, agora, nil, claude); !errors.As(err, &so) {
		t.Fatalf("aprovar como claude = %v", err)
	}
	if _, _, err := EditarConteudo(e, c.ID, conteudoOK(), c.Versao, agora, claude); !errors.As(err, &so) {
		t.Fatalf("editar como claude = %v", err)
	}
	if _, _, err := CancelarCandidatura(e, c.ID, agora, claude); !errors.As(err, &so) {
		t.Fatalf("cancelar como claude = %v", err)
	}
	if _, _, err := ReabrirCandidatura(e, c.ID, agora, claude); !errors.As(err, &so) {
		t.Fatalf("reabrir como claude = %v", err)
	}
	if _, _, err := ConcluirManual(e, c.ID, agora, claude); !errors.As(err, &so) {
		t.Fatalf("concluir como claude = %v", err)
	}
	e, _, err := AprovarCandidatura(e, c.ID, c.Impressao, agora, nil, eu)
	if err != nil {
		t.Fatal(err)
	}
	e, o, err := RevogarCandidatura(e, c.ID, agora, claude)
	if err != nil {
		t.Fatalf("revogar como claude = %v", err)
	}
	if c, _ := candidaturaDe(o); c.Estado != CandPreparada || c.Aprovacao != nil {
		t.Fatalf("revogada = %+v", c)
	}
}

func TestCriarRecusa(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	e := comVaga(t, EstadoVazio(), "li-1", "linkedin", linkLI("1"), true)
	e = comVaga(t, e, "li-2", "linkedin", linkLI("2"), false)                             // só em descoberta
	e = comVaga(t, e, "gp-1", "gupy", "https://empresa-teste.gupy.io.exemplo.io/1", true) // host falso
	e = comVaga(t, e, "ou-1", "Site da empresa", "https://carreiras.exemplo.com/1", true)

	if _, _, err := CriarCandidatura(e, "nao-existe", conteudoOK(), agora, eu); !EhNaoEncontrado(err) {
		t.Errorf("vaga inexistente = %v", err)
	}
	if _, _, err := CriarCandidatura(e, "li-2", conteudoOK(), agora, eu); err == nil || conflito(t, err) == nil {
		t.Errorf("vaga não aprovada = %v", err)
	}
	var ev *ErroValidacao
	if _, _, err := CriarCandidatura(e, "gp-1", conteudoOK(), agora, eu); !errors.As(err, &ev) {
		t.Errorf("host falso = %v", err)
	}
	ruim := conteudoOK()
	ruim["extra"] = true
	if _, _, err := CriarCandidatura(e, "li-1", ruim, agora, eu); !errors.As(err, &ev) {
		t.Errorf("conteúdo com campo a mais = %v", err)
	}
	e2, c := preparada(t, e, "li-1", agora)
	if _, _, err := CriarCandidatura(e2, "li-1", conteudoOK(), agora, eu); conflito(t, err).Existente != c.ID {
		t.Errorf("segunda ativa = %v", err)
	}
	// Cancelada libera a vaga.
	e3, _, err := CancelarCandidatura(e2, c.ID, agora, eu)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := CriarCandidatura(e3, "li-1", conteudoOK(), agora, eu); err != nil {
		t.Errorf("depois de cancelar = %v", err)
	}
	// "outra" aceita qualquer https.
	_, o, err := CriarCandidatura(e, "ou-1", conteudoOK(), agora, eu)
	if err != nil || o["plataforma"] != "outra" {
		t.Errorf("plataforma outra = %v %v", o, err)
	}
	// Modo direto: grupo A em descoberta entra; B não.
	d := comVaga(t, EstadoVazio(), "li-a", "linkedin", linkLI("3"), false)
	d = comVaga(t, d, "li-b", "linkedin", linkLI("4"), false)
	d.Perfil["modo"] = "direto"
	d.Vagas[0]["aderencia"], d.Vagas[1]["aderencia"] = "A", "B"
	if _, _, err := CriarCandidatura(d, "li-a", conteudoOK(), agora, eu); err != nil {
		t.Errorf("grupo A no direto = %v", err)
	}
	if _, _, err := CriarCandidatura(d, "li-b", conteudoOK(), agora, eu); err == nil {
		t.Error("grupo B no direto deveria esperar a Triagem")
	}
}

// Duplicata por empresa e título já enviada (outra vaga) dá 409 com o id dela.
func TestCriarRecusaDuplicataEnviada(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	e, _, err := AdicionarVaga(EstadoVazio(), Objeto{"id": "gp-9", "empresa": "Acme Teste", "titulo": "Dev Go", "plataforma": "gupy", "link": "https://acme.gupy.io/9"}, eu, false)
	if err != nil {
		t.Fatal(err)
	}
	if e, _, err = MoverVaga(e, "gp-9", "enviada", nil, eu); err != nil {
		t.Fatal(err)
	}
	if e, _, err = AdicionarVaga(e, Objeto{"id": "li-9", "empresa": "Acme Teste", "titulo": "Dev Go", "plataforma": "linkedin", "link": linkLI("9")}, eu, true); err != nil {
		t.Fatal(err)
	}
	if e, _, err = MoverVaga(e, "li-9", "aplicar", nil, eu); err != nil {
		t.Fatal(err)
	}
	if _, _, err := CriarCandidatura(e, "li-9", conteudoOK(), agora, eu); conflito(t, err).Existente != "gp-9" {
		t.Fatalf("duplicata enviada = %v", err)
	}
}

func TestEditarEAprovarConferemImpressaoEVersao(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	e := comVaga(t, EstadoVazio(), "li-1", "linkedin", linkLI("1"), true)
	e, c := aprovada(t, e, "li-1", agora, nil)

	novo := conteudoOK()
	novo["idioma"] = "en"
	if _, _, err := EditarConteudo(e, c.ID, novo, c.Versao-1, agora, eu); conflito(t, err) == nil {
		t.Fatal("versão velha deveria dar conflito")
	}
	e, o, err := EditarConteudo(e, c.ID, novo, c.Versao, agora, eu)
	if err != nil {
		t.Fatal(err)
	}
	editada, _ := candidaturaDe(o)
	if editada.Estado != CandPreparada || editada.Aprovacao != nil || editada.Impressao == c.Impressao {
		t.Fatalf("editada = %+v", editada)
	}
	// Aprovar com a impressão que a tela mostrou antes da edição: 409 com a atual.
	cf := conflito(t, func() error { _, _, err := AprovarCandidatura(e, c.ID, c.Impressao, agora, nil, eu); return err }())
	if cf.ImpressaoAtual != editada.Impressao {
		t.Fatalf("impressaoAtual = %q; queria %q", cf.ImpressaoAtual, editada.Impressao)
	}
	if _, _, err := AprovarCandidatura(e, c.ID, editada.Impressao, agora, nil, eu); err != nil {
		t.Fatal(err)
	}
}

// Limite por plataforma, no dia local: falha conta, outra plataforma não, renova à meia-noite.
func TestLimiteDiario(t *testing.T) {
	sp := fuso(t, "America/Sao_Paulo")
	agora := time.Date(2026, 11, 9, 23, 0, 0, 0, sp)
	limites := map[string]int{"linkedin": 2}
	e := EstadoVazio()
	for _, n := range []string{"1", "2", "3"} {
		e = comVaga(t, e, "li-"+n, "linkedin", linkLI(n), true)
	}
	e = comVaga(t, e, "gp-1", "gupy", "https://acme.gupy.io/1", true)

	e, c1 := aprovada(t, e, "li-1", agora, limites)
	e, _, _ = IniciarEnvio(e, c1.ID, c1.Versao, agora, limites)
	e, _, _ = FalharCandidatura(e, c1.ID, "erro-tecnico", agora) // falha conta
	e, c2 := aprovada(t, e, "li-2", agora, limites)
	e, _, err := IniciarEnvio(e, c2.ID, c2.Versao, agora, limites)
	if err != nil {
		t.Fatal(err)
	}
	if n := UsadasNoDia(e, "linkedin", agora); n != 2 {
		t.Fatalf("usadas = %d; queria 2", n)
	}
	e, c3 := preparada(t, e, "li-3", agora)
	_, _, err = AprovarCandidatura(e, c3.ID, c3.Impressao, agora, limites, eu)
	var el *ErroLimite
	if !errors.As(err, &el) || el.Limite != 2 || el.Usadas != 2 || el.RenovaEm != "2026-11-10T00:00:00-03:00" || el.Plataforma != "linkedin" {
		t.Fatalf("limite = %v %+v", err, el)
	}
	// Gupy tem o próprio limite.
	eg, cg := preparada(t, e, "gp-1", agora)
	if _, _, err := AprovarCandidatura(eg, cg.ID, cg.Impressao, agora, limites, eu); err != nil {
		t.Fatalf("limite do LinkedIn barrou a Gupy: %v", err)
	}
	// No dia seguinte (meia-noite local) renova.
	amanha := time.Date(2026, 11, 10, 0, 0, 0, 0, sp)
	if n := UsadasNoDia(e, "linkedin", amanha); n != 0 {
		t.Fatalf("usadas no dia seguinte = %d", n)
	}
	if _, _, err := AprovarCandidatura(e, c3.ID, c3.Impressao, amanha, limites, eu); err != nil {
		t.Fatalf("aprovar no dia seguinte = %v", err)
	}
	// Limite padrão 30 e faixa 1..40.
	if n, ok := LimiteDe(nil, "inhire"); !ok || n != LimitePadrao {
		t.Errorf("padrão = %d %v", n, ok)
	}
	if n, _ := LimiteDe(map[string]int{"gupy": 41}, "gupy"); n != LimitePadrao {
		t.Errorf("limite fora da faixa deveria valer o padrão; veio %d", n)
	}
	if _, ok := LimiteDe(nil, "outra"); ok {
		t.Error("outra não tem limite")
	}
	for n, ok := range map[int]bool{0: false, 1: true, 40: true, 41: false} {
		if (ValidarLimite(n) == nil) != ok {
			t.Errorf("ValidarLimite(%d)", n)
		}
	}
}

// A aprovação vale até 23:59:59 do dia local; mudar o fuso do relógio não estende nem cria validade.
func TestValidadeNaViradaDoDiaENaTrocaDeFuso(t *testing.T) {
	sp, manaus, lisboa := fuso(t, "America/Sao_Paulo"), fuso(t, "America/Manaus"), fuso(t, "Europe/Lisbon")
	base := comVaga(t, EstadoVazio(), "li-1", "linkedin", linkLI("1"), true)
	e, c := aprovada(t, base, "li-1", time.Date(2026, 11, 9, 23, 59, 30, 0, sp), nil)
	if c.Aprovacao.ValidaAte != "2026-11-09T23:59:59-03:00" {
		t.Fatalf("validaAte = %s", c.Aprovacao.ValidaAte)
	}
	casos := []struct {
		nome  string
		agora time.Time
		vale  bool
	}{
		{"último segundo", time.Date(2026, 11, 9, 23, 59, 59, 0, sp), true},
		{"meia-noite", time.Date(2026, 11, 10, 0, 0, 0, 0, sp), false},
		{"Lisboa antes do instante", time.Date(2026, 11, 10, 2, 59, 0, 0, lisboa), true},
		{"Lisboa depois do instante", time.Date(2026, 11, 10, 3, 0, 0, 0, lisboa), false},
		// Em Manaus ainda é dia 9, mas o instante já passou: não estende.
		{"Manaus no mesmo dia local", time.Date(2026, 11, 9, 23, 30, 0, 0, manaus), false},
	}
	for _, k := range casos {
		_, _, err := IniciarEnvio(e, c.ID, c.Versao, k.agora, nil)
		if (err == nil) != k.vale {
			t.Errorf("%s: erro = %v; queria válido=%v", k.nome, err, k.vale)
		}
	}
	// Aprovada perto da meia-noite em outro fuso: grava o fuso do relógio daquele momento.
	_, c2 := aprovada(t, base, "li-1", time.Date(2026, 11, 9, 22, 0, 0, 0, manaus), nil)
	if c2.Aprovacao.ValidaAte != "2026-11-09T23:59:59-04:00" || c2.Aprovacao.Fuso != "America/Manaus" {
		t.Fatalf("aprovação em Manaus = %+v", c2.Aprovacao)
	}
	// Vencidas voltam a preparada.
	e2, n := VencerAprovacoes(e, time.Date(2026, 11, 10, 0, 0, 1, 0, sp))
	if n != 1 || estadoDe(t, e2, c.ID).Estado != CandPreparada || estadoDe(t, e, c.ID).Estado != CandAprovada {
		t.Fatalf("VencerAprovacoes n=%d", n)
	}
	// Tentativa às 23:59 conta no dia 9 em São Paulo e não no dia 10.
	e3, _, err := IniciarEnvio(e, c.ID, c.Versao, time.Date(2026, 11, 9, 23, 59, 50, 0, sp), nil)
	if err != nil {
		t.Fatal(err)
	}
	if UsadasNoDia(e3, "linkedin", time.Date(2026, 11, 9, 12, 0, 0, 0, sp)) != 1 || UsadasNoDia(e3, "linkedin", time.Date(2026, 11, 10, 0, 0, 1, 0, sp)) != 0 {
		t.Fatal("tentativa contada no dia errado")
	}
}

// M2: a conferência no instante do envio. Aprovação vale uma vez; pacote trocado no banco, vaga movida ou versão
// velha não enviam.
func TestIniciarEnvioConfereTudo(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	base := comVaga(t, EstadoVazio(), "li-1", "linkedin", linkLI("1"), true)
	e, c := aprovada(t, base, "li-1", agora, nil)

	if _, _, err := IniciarEnvio(e, c.ID, c.Versao-1, agora, nil); conflito(t, err) == nil {
		t.Error("versão velha")
	}
	adulterado := e.Clonar()
	adulterado.Candidaturas[0]["conteudo"].(Objeto)["idioma"] = "en"
	if _, _, err := IniciarEnvio(adulterado, c.ID, c.Versao, agora, nil); conflito(t, err).ImpressaoAtual == c.Impressao {
		t.Error("pacote trocado no banco passou")
	}
	urlTrocada := e.Clonar()
	urlTrocada.Candidaturas[0]["url"] = "https://linkedin.com.exemplo.io/jobs/1"
	if _, _, err := IniciarEnvio(urlTrocada, c.ID, c.Versao, agora, nil); err == nil {
		t.Error("endereço trocado passou")
	}
	// Mesmo com a impressão recalculada para o endereço falso, o host da plataforma barra (M5).
	falsa, _ := ImpressaoCandidatura("li-1", "linkedin", "https://linkedin.com.exemplo.io/jobs/1", c.Conteudo)
	urlTrocada.Candidaturas[0]["impressao"] = falsa
	urlTrocada.Candidaturas[0]["aprovacao"].(Objeto)["impressao"] = falsa
	if _, _, err := IniciarEnvio(urlTrocada, c.ID, c.Versao, agora, nil); err == nil || !strings.Contains(err.Error(), "endereço") {
		t.Errorf("endereço trocado com impressão recalculada = %v", err)
	}
	pulada, _, err := MoverVaga(e, "li-1", "pulada", Objeto{"motivo": "Stack"}, eu)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := IniciarEnvio(pulada, c.ID, c.Versao, agora, nil); conflito(t, err) == nil {
		t.Error("vaga pulada enviou")
	}
	e2, o, err := IniciarEnvio(e, c.ID, c.Versao, agora, nil)
	if err != nil {
		t.Fatal(err)
	}
	c2, _ := candidaturaDe(o)
	if _, _, err := IniciarEnvio(e2, c.ID, c2.Versao, agora, nil); conflito(t, err).Estado != CandEnviando {
		t.Error("aprovação usada duas vezes")
	}
	// Falha volta só pela pessoa e precisa de nova aprovação.
	e3, o, err := FalharCandidatura(e2, c.ID, "curriculo-mudou", agora)
	if err != nil {
		t.Fatal(err)
	}
	if f, _ := candidaturaDe(o); f.Resultado.Detalhe != DetalheDaFalha["curriculo-mudou"] {
		t.Errorf("detalhe = %q", f.Resultado.Detalhe)
	}
	if _, _, err := FalharCandidatura(e2, c.ID, "texto livre da página", agora); err == nil {
		t.Error("falha fora da lista aceita")
	}
	e4, o, err := ReabrirCandidatura(e3, c.ID, agora, eu)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := candidaturaDe(o)
	if r.Estado != CandPreparada || r.Aprovacao != nil || r.Resultado != nil || len(r.Tentativas) != 1 {
		t.Fatalf("reaberta = %+v", r)
	}
	if _, _, err := IniciarEnvio(e4, c.ID, r.Versao, agora, nil); conflito(t, err) == nil {
		t.Error("reaberta enviou sem nova aprovação")
	}
}

// M3: ao abrir o app, enviando vira incerta; incerta não é cancelada nem enviada de novo sozinha.
func TestIncertaSoSaiPelaPessoa(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	e, c := aprovada(t, comVaga(t, EstadoVazio(), "li-1", "linkedin", linkLI("1"), true), "li-1", agora, nil)
	e, _, err := IniciarEnvio(e, c.ID, c.Versao, agora, nil)
	if err != nil {
		t.Fatal(err)
	}
	e, n := MarcarIncertas(e, agora.Add(time.Hour))
	inc := estadoDe(t, e, c.ID)
	if n != 1 || inc.Estado != CandIncerta || inc.Resultado.Tipo != "incerto" {
		t.Fatalf("incerta = %d %+v", n, inc)
	}
	if _, n := MarcarIncertas(e, agora); n != 0 {
		t.Error("marcou de novo")
	}
	if _, _, err := CancelarCandidatura(e, c.ID, agora, eu); conflito(t, err).Estado != CandIncerta {
		t.Error("incerta foi cancelada")
	}
	if _, _, err := IniciarEnvio(e, c.ID, inc.Versao, agora, nil); err == nil {
		t.Error("incerta enviou de novo")
	}
	if _, _, err := MarcarEnviada(e, c.ID, agora, eu); err == nil {
		t.Error("executor concluiu uma incerta")
	}
	e2, o, err := ConcluirManual(e, c.ID, agora, eu)
	if err != nil {
		t.Fatal(err)
	}
	if m, _ := candidaturaDe(o); m.Estado != CandEnviada || m.Resultado.Tipo != "manual" || etapaDe(e2, "li-1") != "enviada" {
		t.Fatalf("concluída = %+v", m)
	}
}

// Bloqueio: motivo da lista fechada, pendência fixa nossa; a vaga vai para bloqueada junto; reabrir volta para aplicar.
func TestBloqueioEReabrir(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	e, c := aprovada(t, comVaga(t, EstadoVazio(), "li-1", "linkedin", linkLI("1"), true), "li-1", agora, nil)
	e, o, _ := IniciarEnvio(e, c.ID, c.Versao, agora, nil)
	if _, _, err := BloquearCandidatura(e, c.ID, "Digite seu CPF aqui", agora, eu); err == nil {
		t.Fatal("motivo fora da lista aceito")
	}
	if len(PendenciaDoMotivo) != 14 {
		t.Fatalf("motivos = %d; queria 14", len(PendenciaDoMotivo))
	}
	e2, o, err := BloquearCandidatura(e, c.ID, "captcha", agora, eu)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := candidaturaDe(o)
	v := vagaDe(e2, "li-1")
	if b.Estado != CandBloqueada || b.Bloqueio.Motivo != "captcha" || b.Bloqueio.Pendencia != PendenciaDoMotivo["captcha"] || v["etapa"] != "bloqueada" || v["pendencia"] != PendenciaDoMotivo["captcha"] {
		t.Fatalf("bloqueada = %+v / vaga %v", b, v)
	}
	if _, _, err := BloquearCandidatura(e2, c.ID, "login", agora, eu); conflito(t, err) == nil {
		t.Error("bloqueou duas vezes")
	}
	e3, _, err := ReabrirCandidatura(e2, c.ID, agora, eu)
	if err != nil {
		t.Fatal(err)
	}
	if etapaDe(e3, "li-1") != "aplicar" || !vagaAprovada(e3, "li-1") {
		t.Fatalf("vaga depois de reabrir = %v", vagaDe(e3, "li-1"))
	}
	// Concluir manual direto do bloqueio leva a vaga para enviada.
	e4, _, err := ConcluirManual(e2, c.ID, agora, eu)
	if err != nil || etapaDe(e4, "li-1") != "enviada" {
		t.Fatalf("concluir do bloqueio = %v %v", err, etapaDe(e4, "li-1"))
	}
}

// Cancelada é final; ações em estado errado dão 409 com o estado.
func TestTransicoesInvalidas(t *testing.T) {
	agora := time.Date(2026, 11, 9, 10, 0, 0, 0, fuso(t, "America/Sao_Paulo"))
	e, c := preparada(t, comVaga(t, EstadoVazio(), "li-1", "linkedin", linkLI("1"), true), "li-1", agora)
	if _, _, err := RevogarCandidatura(e, c.ID, agora, eu); conflito(t, err).Estado != CandPreparada {
		t.Error("revogar preparada")
	}
	if _, _, err := IniciarEnvio(e, c.ID, c.Versao, agora, nil); conflito(t, err) == nil {
		t.Error("enviar preparada")
	}
	if _, _, err := ReabrirCandidatura(e, c.ID, agora, eu); conflito(t, err) == nil {
		t.Error("reabrir preparada")
	}
	e, _, _ = AprovarCandidatura(e, c.ID, c.Impressao, agora, nil, eu)
	e, o, err := CancelarCandidatura(e, c.ID, agora, eu)
	if cc, _ := candidaturaDe(o); err != nil || cc.Estado != CandCancelada || cc.Aprovacao != nil {
		t.Fatalf("cancelar aprovada = %+v %v", cc, err)
	}
	for nome, f := range map[string]func() error{
		"aprovar":  func() error { _, _, err := AprovarCandidatura(e, c.ID, c.Impressao, agora, nil, eu); return err },
		"editar":   func() error { _, _, err := EditarConteudo(e, c.ID, conteudoOK(), 2, agora, eu); return err },
		"reabrir":  func() error { _, _, err := ReabrirCandidatura(e, c.ID, agora, eu); return err },
		"cancelar": func() error { _, _, err := CancelarCandidatura(e, c.ID, agora, eu); return err },
		"concluir": func() error { _, _, err := ConcluirManual(e, c.ID, agora, eu); return err },
	} {
		if cf := conflito(t, f()); cf.Estado != CandCancelada {
			t.Errorf("%s em cancelada: estado %q", nome, cf.Estado)
		}
	}
	if _, _, err := AprovarCandidatura(e, "cand-nao-existe", "", agora, nil, eu); !EhNaoEncontrado(err) {
		t.Errorf("inexistente = %v", err)
	}
}

// O estado sem candidaturas sai igual ao do Node (o golden não muda) e Clonar copia a fila.
func TestEstadoSemFilaIgualAoNode(t *testing.T) {
	b, _ := json.Marshal(EstadoVazio())
	if strings.Contains(string(b), "candidaturas") {
		t.Fatalf("estado vazio = %s", b)
	}
	e := EstadoVazio()
	e.Candidaturas = []Objeto{{"id": "cand-1", "conteudo": Objeto{"idioma": "pt"}}}
	c := e.Clonar()
	c.Candidaturas[0]["conteudo"].(Objeto)["idioma"] = "en"
	if !reflect.DeepEqual(e.Candidaturas[0]["conteudo"], Objeto{"idioma": "pt"}) {
		t.Fatal("Clonar compartilhou a fila")
	}
}
