package dominio

import (
	"errors"
	"strings"
	"testing"
)

// Vaga fictícia enviada, com link na Gupy.
func estadoComVagaEnviada(t *testing.T) (Estado, Ctx) {
	t.Helper()
	ctx := Ctx{Autor: "eu", Agora: "2026-11-10"}
	e, _, err := AdicionarVaga(EstadoVazio(), Objeto{"id": "gp-1", "empresa": "Acme Pagamentos", "titulo": "Desenvolvedor Go",
		"link": "https://acmepagamentos.gupy.io/jobs/123"}, ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if e, _, err = MoverVaga(e, "gp-1", "enviada", nil, ctx); err != nil {
		t.Fatal(err)
	}
	return e, ctx
}

func emailLido(id, de, assunto string) Objeto {
	return Objeto{"idMensagem": id, "de": de, "assunto": assunto, "data": "2026-11-10T09:00:00Z", "texto": "Olá! Sobre a vaga de Desenvolvedor Go na Acme Pagamentos."}
}

func etapaDaVaga(e Estado, id string) string {
	for _, v := range e.Vagas {
		if v["id"] == id {
			return texto(v["etapa"])
		}
	}
	return ""
}

func TestRemetenteDaVaga(t *testing.T) {
	vaga := Objeto{"empresa": "Acme Pagamentos S.A.", "link": "https://acmepagamentos.gupy.io/jobs/123"}
	casos := map[string]bool{
		"no-reply@gupy.io":                    true,  // o domínio do link
		"talentos@mail.gupy.io":               true,  // subdomínio do mesmo domínio registrável
		"rh@acmepagamentos.com.br":            true,  // a empresa é um rótulo do domínio
		"rh@acmepagamentos-vagas.com":         false, // parecido não é igual
		"rh@falsoacmepagamentos.com":          false,
		"rh@acmepagamentos.com.golpe.example": false, // o domínio registrável é golpe.example
		"recrutador@gmail.com":                false,
		"sem-arroba":                          false,
	}
	for de, quer := range casos {
		if RemetenteDaVaga(vaga, de) != quer {
			t.Errorf("RemetenteDaVaga(%q) = %v", de, !quer)
		}
	}
}

func TestRegistrarEmailLidoSoMudaSozinhoOQueEReversivelEDoDominioDaVaga(t *testing.T) {
	casos := []struct {
		nome, de, tipo, status, etapa string
	}{
		{"recusa do domínio da vaga", "no-reply@gupy.io", "rejeicao", "a-confirmar", "enviada"},
		{"oferta do domínio da vaga", "no-reply@gupy.io", "oferta", "a-confirmar", "enviada"},
		{"convite do domínio da vaga", "no-reply@gupy.io", "entrevista", "ligado", "em-processo"},
		{"teste do domínio da vaga", "rh@acmepagamentos.com.br", "teste", "ligado", "em-processo"},
		{"convite de outro domínio", "recrutadora@golpe.example", "entrevista", "a-confirmar", "enviada"},
		{"confirmação só registra", "no-reply@gupy.io", "confirmacao", "ligado", "enviada"},
	}
	for _, c := range casos {
		e, ctx := estadoComVagaEnviada(t)
		novo, r, err := RegistrarEmailLido(e, emailLido("m-1", c.de, "Processo seletivo Acme Pagamentos"), Objeto{"tipo": c.tipo, "vagaId": "gp-1", "resumo": "resumo"}, ctx)
		if err != nil {
			t.Fatalf("%s: %v", c.nome, err)
		}
		if r["status"] != c.status || etapaDaVaga(novo, "gp-1") != c.etapa {
			t.Fatalf("%s: status %v, etapa %q; queria %s, %s", c.nome, r["status"], etapaDaVaga(novo, "gp-1"), c.status, c.etapa)
		}
		if c.status == "a-confirmar" {
			reg := novo.Emails[len(novo.Emails)-1]
			if reg["vagaId"] != "gp-1" || ObjetoDe(reg["proposta"])["etapa"] == nil {
				t.Fatalf("%s: registro %v", c.nome, reg)
			}
			if len(e.Emails) != 0 {
				t.Fatalf("%s: mexeu no estado antigo", c.nome)
			}
		}
		// O mesmo e-mail de novo não duplica.
		if _, r, _ := RegistrarEmailLido(novo, emailLido("m-1", c.de, "x"), Objeto{"tipo": c.tipo}, ctx); r["status"] != "duplicado" {
			t.Fatalf("%s: repetido = %v", c.nome, r["status"])
		}
	}
}

// Sem vaga no app: "candidatura recebida" só fica registrada (sem pedir nada); convite pede a vaga.
func TestRegistrarEmailLidoSemVaga(t *testing.T) {
	for tipo, status := range map[string]string{"confirmacao": "ignorado", "rejeicao": "ignorado", "entrevista": "sem-vaga", "teste": "sem-vaga"} {
		e, ctx := estadoComVagaEnviada(t)
		email := emailLido("m-9", "rh@outra.example", "Recebemos sua candidatura - Outra Empresa")
		email["texto"] = "Olá! Recebemos sua candidatura para Analista na Outra Empresa."
		novo, r, err := RegistrarEmailLido(e, email, Objeto{"tipo": tipo, "resumo": "resumo", "empresa": " Outra   Empresa ", "cargo": "Analista <b>Go</b>"}, ctx)
		if err != nil {
			t.Fatalf("%s: %v", tipo, err)
		}
		reg := novo.Emails[len(novo.Emails)-1]
		if r["status"] != status || reg["status"] != status || reg["tipo"] != tipo || etapaDaVaga(novo, "gp-1") != "enviada" {
			t.Fatalf("%s: resposta %v, registro %v", tipo, r, reg)
		}
		// Só quem pede a vaga guarda a sugestão da IA, já limpa.
		sug := ObjetoDe(reg["sugestao"])
		if status == "sem-vaga" && (sug["empresa"] != "Outra Empresa" || sug["cargo"] != "Analista bGo/b") {
			t.Fatalf("%s: sugestão %v", tipo, reg["sugestao"])
		}
		if status == "ignorado" && reg["sugestao"] != nil {
			t.Fatalf("%s: guardou sugestão sem pedir vaga", tipo)
		}
		if _, r, _ := RegistrarEmailLido(novo, emailLido("m-9", "rh@outra.example", "x"), Objeto{"tipo": tipo}, ctx); r["status"] != "duplicado" {
			t.Fatalf("%s: repetido = %v", tipo, r["status"])
		}
	}
}

func TestTextoDeSugestao(t *testing.T) {
	for entrada, quer := range map[any]string{
		"  Acme \n Pagamentos ": "Acme Pagamentos", "<script>x</script>": "scriptx/script", "https://golpe.example": "",
		"veja www.golpe.example": "", nil: "", strings.Repeat("a", 200): strings.Repeat("a", 120),
	} {
		if got := TextoDeSugestao(entrada); got != quer {
			t.Errorf("TextoDeSugestao(%q) = %q, queria %q", entrada, got, quer)
		}
	}
}

// Convite de vaga que não está no app: a pessoa cria a vaga, que entra em processo e fica ligada ao e-mail.
func TestCriarVagaDoEmail(t *testing.T) {
	semVaga := func(t *testing.T, tipo string) (Estado, Ctx) {
		e, ctx := estadoComVagaEnviada(t)
		email := emailLido("m-5", "noreply@mailer.example", "Convite: entrevista para Backend Sênior")
		email["texto"], email["data"] = "Queremos conversar com você sobre a vaga.", "2026-11-08T09:00:00Z"
		e, r, err := RegistrarEmailLido(e, email, Objeto{"tipo": tipo, "resumo": "convite"}, ctx)
		if err != nil || r["status"] != "sem-vaga" {
			t.Fatalf("preparar: %v %v", r, err)
		}
		return e, ctx
	}

	e, ctx := semVaga(t, "entrevista")
	novo, r, err := CriarVagaDoEmail(e, "m-5", "Beta Tech", "Backend Sênior", ctx)
	if err != nil || r["criada"] != true || r["status"] != "ligado" {
		t.Fatalf("criar: %v %v", r, err)
	}
	var v Objeto
	for _, x := range novo.Vagas {
		if x["id"] == r["vagaId"] {
			v = x
		}
	}
	if v == nil || v["empresa"] != "Beta Tech" || v["titulo"] != "Backend Sênior" || v["etapa"] != "em-processo" ||
		v["subEtapa"] != "entrevista-rh" || ObjetoDe(v["envio"])["data"] != "2026-11-08" {
		t.Fatalf("vaga criada: %v", v)
	}
	if reg := novo.Emails[len(novo.Emails)-1]; reg["status"] != "ligado" || reg["vagaId"] != v["id"] {
		t.Fatalf("e-mail: %v", reg)
	}
	if len(e.Vagas) != 1 {
		t.Fatal("mexeu no estado antigo")
	}
	// De novo: o e-mail já não espera vaga.
	if _, _, err := CriarVagaDoEmail(novo, "m-5", "Beta Tech", "Backend Sênior", ctx); err == nil {
		t.Fatal("criou duas vezes")
	}

	// Já existe vaga com a mesma empresa e cargo: liga a ela, sem criar outra.
	e, ctx = semVaga(t, "teste")
	novo, r, err = CriarVagaDoEmail(e, "m-5", "Acme Pagamentos", "Desenvolvedor Go", ctx)
	if err != nil || r["vagaId"] != "gp-1" || r["criada"] != nil || len(novo.Vagas) != 1 || etapaDaVaga(novo, "gp-1") != "em-processo" {
		t.Fatalf("duplicada: %v %v, %d vagas", r, err, len(novo.Vagas))
	}

	// Empresa ou cargo vazios (ou só um link) e e-mail desconhecido: nada muda.
	e, ctx = semVaga(t, "entrevista")
	for _, c := range [][2]string{{"", "Backend"}, {"Beta", " "}, {"https://golpe.example", "Backend"}} {
		if _, _, err := CriarVagaDoEmail(e, "m-5", c[0], c[1], ctx); !ehValidacao(err) {
			t.Fatalf("%q: %v", c, err)
		}
	}
	if _, _, err := CriarVagaDoEmail(e, "nao-existe", "Beta", "Backend", ctx); !EhNaoEncontrado(err) {
		t.Fatalf("desconhecido: %v", err)
	}
}

func TestConfirmarEmail(t *testing.T) {
	e, ctx := estadoComVagaEnviada(t)
	e, _, err := RegistrarEmailLido(e, emailLido("m-1", "no-reply@gupy.io", "Atualização Acme Pagamentos"), Objeto{"tipo": "rejeicao", "vagaId": "gp-1", "resumo": "não seguiram"}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	novo, r, err := ConfirmarEmail(e, "m-1", ctx)
	if err != nil || r["transicao"] != "encerrada" || etapaDaVaga(novo, "gp-1") != "encerrada" {
		t.Fatalf("confirmar: %v %v etapa %q", r, err, etapaDaVaga(novo, "gp-1"))
	}
	if reg := novo.Emails[0]; reg["status"] != "ligado" || reg["proposta"] != nil {
		t.Fatalf("registro depois de confirmar: %v", reg)
	}
	if _, _, err := ConfirmarEmail(novo, "m-1", ctx); err == nil {
		t.Fatal("confirmar duas vezes devia falhar")
	}

	// A vaga andou antes da confirmação (a pessoa encerrou à mão): nada muda.
	e2, ctx := estadoComVagaEnviada(t)
	e2, _, _ = RegistrarEmailLido(e2, emailLido("m-2", "no-reply@gupy.io", "Atualização Acme Pagamentos"), Objeto{"tipo": "oferta", "vagaId": "gp-1"}, ctx)
	e2, _, _ = MoverVaga(e2, "gp-1", "encerrada", Objeto{"motivo": "Desisti"}, ctx)
	var conflito *ErroConflito
	if _, _, err := ConfirmarEmail(e2, "m-2", ctx); !errors.As(err, &conflito) || conflito.Motivo != "vaga-mudou" {
		t.Fatalf("vaga que já andou: %v", err)
	}

	// Descartar tira da lista sem mexer na vaga.
	e3, ctx := estadoComVagaEnviada(t)
	e3, _, _ = RegistrarEmailLido(e3, emailLido("m-3", "no-reply@gupy.io", "Atualização Acme Pagamentos"), Objeto{"tipo": "rejeicao", "vagaId": "gp-1"}, ctx)
	e3, _, err = DescartarEmail(e3, "m-3")
	if err != nil || etapaDaVaga(e3, "gp-1") != "enviada" || e3.Emails[0]["status"] != "descartado" {
		t.Fatalf("descartar: %v etapa %q", err, etapaDaVaga(e3, "gp-1"))
	}
}

func TestVagasCandidatasDoEmail(t *testing.T) {
	e, _ := estadoComVagaEnviada(t)
	if c := VagasCandidatasDoEmail(e.Vagas, emailLido("x", "no-reply@gupy.io", "Acme Pagamentos")); len(c) != 1 || c[0]["id"] != "gp-1" {
		t.Fatalf("candidatas: %v", c)
	}
	semEmpresa := Objeto{"de": "x@outra.example", "assunto": "Newsletter", "texto": "nada"}
	if c := VagasCandidatasDoEmail(e.Vagas, semEmpresa); len(c) != 0 {
		t.Fatalf("e-mail sem a empresa trouxe candidatas: %v", c)
	}
}

func ehValidacao(err error) bool {
	var ev *ErroValidacao
	return errors.As(err, &ev)
}
