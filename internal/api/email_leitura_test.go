package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/caixa"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
	"github.com/emersion/go-imap/backend/memory"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-imap/server"
)

// Tudo fictício: endereços em example, empresa inventada, CPF e código inventados.

func mensagemDeTeste(id, de, assunto, data, corpo string) string {
	return "From: " + de + "\r\nTo: username@example.com\r\nSubject: " + assunto + "\r\nMessage-ID: <" + id + "@example.com>\r\n" +
		"Date: " + data + "\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" + corpo + "\r\n"
}

func leitorComEmails(t *testing.T, mensagens ...string) caixa.Leitor {
	t.Helper()
	be := memory.New()
	u, _ := be.Login(nil, "username", "password")
	cx, _ := u.GetMailbox("INBOX")
	mbox := cx.(*memory.Mailbox)
	mbox.Messages = nil
	// Data interna no "hoje" da API: o servidor em memória do go-imap trata SINCE como "depois do dia" (o RFC 3501
	// diz "no dia ou depois"); o Gmail segue o RFC, e o app volta um dia na busca.
	for i, m := range mensagens {
		if err := mbox.CreateMessage(nil, time.Date(2026, 11, 10, 8+i, 0, 0, 0, time.UTC), bytes.NewBufferString(m)); err != nil {
			t.Fatal(err)
		}
	}
	s := server.New(comEndereco{be})
	s.AllowInsecureAuth = true
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go s.Serve(l)
	t.Cleanup(func() { s.Close() })
	return caixa.Leitor{Discar: func(context.Context, string) (*client.Client, error) { return client.Dial(l.Addr().String()) }}
}

// iaDoEmail: classifica pelo assunto e guarda tudo o que recebeu.
type iaDoEmail struct {
	mu       sync.Mutex
	recebido []string
}

func (*iaDoEmail) Nome() string { return "anthropic" }
func (p *iaDoEmail) Chamar(_ context.Context, pedido ia.PedidoProvedor) (ia.RespostaProvedor, error) {
	p.mu.Lock()
	p.recebido = append(p.recebido, pedido.Prompt.Mensagem)
	p.mu.Unlock()
	saida := Objeto{"tipo": "outro", "vagaId": nil, "resumo": "nada"}
	switch {
	case strings.Contains(pedido.Prompt.Mensagem, "Atualização"):
		// Recusa com link e marcação no resumo: o app limpa.
		saida = Objeto{"tipo": "rejeicao", "vagaId": "gp-1", "resumo": "Não seguiram. Veja https://golpe.example/x <b>já</b>"}
	case strings.Contains(pedido.Prompt.Mensagem, "Convite"):
		saida = Objeto{"tipo": "entrevista", "vagaId": "gp-1", "resumo": "Convite para conversar", "proximoPasso": "Escolher um horário"}
	}
	b, _ := json.Marshal(saida)
	return ia.RespostaProvedor{Texto: string(b)}, nil
}

func (p *iaDoEmail) chamadas() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string{}, p.recebido...)
}

func apiDeLeitura(t *testing.T, leitor caixa.Leitor, comConta, comIA bool) (*API, *iaDoEmail) {
	t.Helper()
	ctx := context.Background()
	arm, err := armazem.Abrir(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	prov := &iaDoEmail{}
	cofre := segredos.Memoria{"anthropic": "chave-ficticia"}
	a := Nova(arm, Opcoes{Hoje: func() string { return "2026-11-10" }, Cofre: cofre, ProvedoresIA: map[string]ia.Provedor{"anthropic": prov},
		DetectarCLIs: func(context.Context) []ia.EstadoCLI { return nil }})
	a.leitorEmail = leitor
	if comIA {
		if err := arm.GravarMeta(ctx, chaveConfig, ia.Config{Rotas: map[string]ia.Rota{"padrao": {Provedor: "anthropic", Modelo: "teste"}}}); err != nil {
			t.Fatal(err)
		}
	}
	if comConta {
		cofre[chaveCofreEmail] = "password"
		if err := arm.GravarMeta(ctx, chaveMetaEmail, contaEmail{Usuario: "username@example.com", Servidor: "imap.example.test"}); err != nil {
			t.Fatal(err)
		}
	}
	err = a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		c := a.ctx(e, "eu")
		e, _, err := dominio.AdicionarVaga(e, Objeto{"id": "gp-1", "empresa": "Acme Pagamentos", "titulo": "Desenvolvedor Go",
			"link": "https://acmepagamentos.gupy.io/jobs/123"}, c, false)
		if err != nil {
			return e, err
		}
		e, _, err = dominio.MoverVaga(e, "gp-1", "enviada", nil, c)
		return e, err
	})
	if err != nil {
		t.Fatal(err)
	}
	return a, prov
}

func chamar(t *testing.T, a *API, m, caminho string, corpo Objeto, autor string) Resposta {
	t.Helper()
	r, err := a.Chamar(context.Background(), Pedido{Metodo: m, Caminho: Prefixo + caminho, Corpo: corpo, Autor: autor})
	if err != nil {
		t.Fatalf("%s %s: erro inesperado %v", m, caminho, err)
	}
	return r
}

func vagaGp1(t *testing.T, a *API) Objeto {
	t.Helper()
	e, err := a.arm.Carregar(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range e.Vagas {
		if v["id"] == "gp-1" {
			return v
		}
	}
	t.Fatal("vaga gp-1 sumiu")
	return nil
}

func TestLerRespostasDoEmail(t *testing.T) {
	leitor := leitorComEmails(t,
		mensagemDeTeste("recusa", "Acme <no-reply@gupy.io>", "Atualização do processo seletivo - Acme Pagamentos",
			"Mon, 09 Nov 2026 08:00:00 +0000", "Infelizmente não vamos seguir com a sua candidatura para Desenvolvedor Go. CPF 123.456.789-09, código 482913, https://golpe.example/x e ana.ficticia@example.com."),
		mensagemDeTeste("seguranca", "Plataforma <no-reply@plataforma.example>", "Código de verificação da sua candidatura",
			"Mon, 09 Nov 2026 09:00:00 +0000", "Seu código é 778899."),
		mensagemDeTeste("conta", "Google <no-reply@accounts.google.com>", "Alerta de segurança: novo acesso à candidatura",
			"Mon, 09 Nov 2026 09:30:00 +0000", "Novo login."),
		mensagemDeTeste("promo", "Loja <promo@loja.example>", "Black Friday: offer imperdível",
			"Mon, 09 Nov 2026 10:00:00 +0000", "Descontos."),
		mensagemDeTeste("almoco", "Amiga <amiga@example.com>", "Almoço sábado?",
			"Mon, 09 Nov 2026 11:00:00 +0000", "Bora?"),
		mensagemDeTeste("convite", "RH Acme <rh@acmepagamentos.com.br>", "Convite para entrevista - Acme Pagamentos",
			"Mon, 09 Nov 2026 12:00:00 +0000", "Queremos conversar sobre a vaga de Desenvolvedor Go."),
	)
	a, prov := apiDeLeitura(t, leitor, true, true)
	var registro bytes.Buffer
	antes := log.Writer()
	log.SetOutput(&registro)
	t.Cleanup(func() { log.SetOutput(antes) })

	r := chamar(t, a, "POST", "/email/ler", Objeto{}, "eu")
	if r.Status != 200 {
		t.Fatalf("ler: %d %v", r.Status, r.Corpo)
	}
	res := r.Corpo.(ResultadoLeitura)
	quer := ResultadoLeitura{Lidos: 5, Sensiveis: 2, Ignorados: 1, Registrados: 2, Mudaram: 1, AConfirmar: 1, UltimaLeitura: "2026-11-10"}
	if res != quer {
		t.Fatalf("resultado:\n %+v\nqueria\n %+v", res, quer)
	}

	// A IA recebeu só os dois e-mails de processo, sem número, link, endereço de terceiro nem os e-mails sensíveis.
	recebido := prov.chamadas()
	if len(recebido) != 2 {
		t.Fatalf("a IA foi chamada %d vezes; queria 2", len(recebido))
	}
	tudo := strings.Join(recebido, "\n")
	for _, proibido := range []string{"123.456.789-09", "482913", "778899", "golpe.example", "ana.ficticia", "accounts.google", "Black Friday", "Almoço", "Acme <"} {
		if strings.Contains(tudo, proibido) {
			t.Errorf("chegou à IA: %q", proibido)
		}
	}
	if !strings.Contains(tudo, "•••") || !strings.Contains(tudo, "[link]") || !strings.Contains(tudo, "[e-mail]") {
		t.Errorf("a máscara não apareceu no trecho:\n%s", tudo)
	}

	// Convite do domínio da empresa mudou a vaga; a recusa espera a pessoa.
	if v := vagaGp1(t, a); v["etapa"] != "em-processo" {
		t.Fatalf("depois do convite, etapa %v", v["etapa"])
	}
	e, _ := a.arm.Carregar(context.Background())
	var recusa Objeto
	for _, x := range e.Emails {
		s, _ := json.Marshal(x)
		for _, proibido := range []string{"Código de verificação", "Alerta de segurança", "seguranca@", "conta@", "Acme <", "golpe.example", "<b>"} {
			if strings.Contains(string(s), proibido) {
				t.Errorf("ficou guardado: %q em %s", proibido, s)
			}
		}
		if x["idMensagem"] == "promo@example.com" && (x["assunto"] != nil || x["de"] != nil) {
			t.Errorf("e-mail fora do processo guardou mais que o id: %v", x)
		}
		if x["status"] == "a-confirmar" {
			recusa = x
		}
	}
	if len(e.Emails) != 3 || recusa == nil || recusa["tipo"] != "rejeicao" || recusa["de"] != "no-reply@gupy.io" {
		t.Fatalf("e-mails guardados: %v", e.Emails)
	}
	if strings.Contains(dominio.Texto(recusa["resumo"]), "https://") {
		t.Fatalf("link da IA no resumo: %v", recusa["resumo"])
	}

	// O log leva só contagens.
	for _, proibido := range []string{"gupy", "Acme", "Atualização", "username", "Convite"} {
		if strings.Contains(registro.String(), proibido) {
			t.Errorf("o log levou %q:\n%s", proibido, registro.String())
		}
	}

	// Ler de novo: nada novo chega à IA.
	r = chamar(t, a, "POST", "/email/ler", Objeto{}, "eu")
	if res := r.Corpo.(ResultadoLeitura); res.JaLidos != 3 || res.Registrados != 0 || len(prov.chamadas()) != 2 {
		t.Fatalf("segunda leitura: %+v, IA %d", res, len(prov.chamadas()))
	}

	// Confirmar a recusa: só a pessoa, pela tela.
	caminho := "/emails/" + url.PathEscape(dominio.Texto(recusa["idMensagem"])) + "/confirmar"
	if r := chamar(t, a, "POST", caminho, Objeto{}, "claude"); r.Status != 403 {
		t.Fatalf("confirmar pelo assistente: %d", r.Status)
	}
	if v := vagaGp1(t, a); v["etapa"] != "em-processo" {
		t.Fatalf("o assistente mudou a vaga: %v", v["etapa"])
	}
	if r := chamar(t, a, "POST", caminho, Objeto{}, "eu"); r.Status != 200 {
		t.Fatalf("confirmar: %d %v", r.Status, r.Corpo)
	}
	if v := vagaGp1(t, a); v["etapa"] != "encerrada" {
		t.Fatalf("depois de confirmar a recusa, etapa %v", v["etapa"])
	}
}

func TestLerRespostasPedeContaIAEPessoa(t *testing.T) {
	leitor := leitorComEmails(t)
	casos := []struct {
		nome          string
		conta, comIA  bool
		autor, motivo string
		status        int
	}{
		{"sem conta", false, true, "eu", "sem-conta", 409},
		{"sem IA", true, false, "eu", "sem-ia", 409},
		{"pelo assistente", true, true, "claude", "", 403},
	}
	for _, c := range casos {
		a, _ := apiDeLeitura(t, leitor, c.conta, c.comIA)
		r := chamar(t, a, "POST", "/email/ler", Objeto{}, c.autor)
		corpo, _ := r.Corpo.(Objeto)
		if r.Status != c.status || (c.motivo != "" && corpo["motivo"] != c.motivo) {
			t.Fatalf("%s: %d %v", c.nome, r.Status, r.Corpo)
		}
	}
	// Senha revogada no Google: pede para conectar de novo.
	a, _ := apiDeLeitura(t, leitor, true, true)
	a.cofre.Gravar(chaveCofreEmail, "revogada")
	if r := chamar(t, a, "POST", "/email/ler", Objeto{}, "eu"); r.Status != 409 || r.Corpo.(Objeto)["motivo"] != "senha" {
		t.Fatalf("senha revogada: %d %v", r.Status, r.Corpo)
	}
	// Corpo com campo: 400.
	if r := chamar(t, a, "POST", "/email/ler", Objeto{"desde": "2020-01-01"}, "eu"); r.Status != 400 {
		t.Fatalf("campo a mais: %d", r.Status)
	}
}

func TestMascararESensivel(t *testing.T) {
	m := mascarar("CPF 123.456.789-09, cartão 4111 1111 1111 1111, código 123456, ano 2026? veja https://x.example/a e www.y.example, fale com a@b.example")
	for _, proibido := range []string{"123.456", "4111", "123456", "x.example", "www.", "a@b"} {
		if strings.Contains(m, proibido) {
			t.Errorf("sobrou %q em %q", proibido, m)
		}
	}
	for _, c := range []struct {
		de, assunto string
		quer        bool
	}{
		{"no-reply@gupy.io", "Infelizmente não seguiremos; você fica no nosso banco de talentos", false},
		{"rh@empresa.example", "Seu código de verificação", true},
		{"x@nubank.com.br", "Oi", true},
		{"rh@empresa.example", "Sua fatura de novembro", true},
		{"rh@empresa.example", "Reset your password", true},
		{"rh@empresa.example", "Interview invitation", false},
	} {
		if sensivel(c.de, c.assunto, "") != c.quer {
			t.Errorf("sensivel(%q, %q) = %v", c.de, c.assunto, !c.quer)
		}
	}
}
