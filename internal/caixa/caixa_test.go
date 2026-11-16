package caixa

import (
	"bytes"
	"context"
	"errors"
	"net"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend/memory"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-imap/server"
)

// Servidor IMAP de mentira (memória do go-imap, usuário "username"/"password"), sem TLS, só neste teste.
// O servidor de memória nunca marca nada como lido; por isso o teste confere os comandos que o app manda.
func servidorDeTeste(t *testing.T, mensagens ...string) (Leitor, *memory.Mailbox, func() string) {
	t.Helper()
	be := memory.New()
	u, _ := be.Login(nil, "username", "password")
	caixa, _ := u.GetMailbox("INBOX")
	mbox := caixa.(*memory.Mailbox)
	mbox.Messages = nil // a caixa de exemplo do go-imap vem com uma mensagem
	base := time.Date(2026, 11, 10, 9, 0, 0, 0, time.UTC)
	for i, m := range mensagens {
		if err := mbox.CreateMessage(nil, base.Add(time.Duration(i)*time.Hour), bytes.NewBufferString(m)); err != nil {
			t.Fatal(err)
		}
	}
	s := server.New(be)
	s.AllowInsecureAuth = true
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go s.Serve(l)
	t.Cleanup(func() { s.Close() })
	g := &gravador{enviado: &bytes.Buffer{}}
	discar := func(context.Context, string) (*client.Client, error) {
		conn, err := net.Dial("tcp", l.Addr().String())
		if err != nil {
			return nil, err
		}
		g.Conn = conn
		return client.New(g)
	}
	return Leitor{Discar: discar}, mbox, g.texto
}

// gravador guarda tudo o que o app manda ao servidor, para conferir os comandos IMAP de verdade.
type gravador struct {
	net.Conn
	mu      sync.Mutex
	enviado *bytes.Buffer
}

func (g *gravador) Write(b []byte) (int, error) {
	g.mu.Lock()
	g.enviado.Write(b)
	g.mu.Unlock()
	return g.Conn.Write(b)
}

func (g *gravador) texto() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.enviado.String()
}

func emailDeTeste(de, assunto, corpo string) string {
	return "From: " + de + "\r\nTo: fulana@example.com\r\nSubject: " + assunto + "\r\nMessage-ID: <" + strings.ReplaceAll(assunto, " ", "-") + "@example.com>\r\n" +
		"Date: Tue, 10 Nov 2026 09:00:00 +0000\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" + corpo + "\r\n"
}

var contaDeTeste = Conta{Usuario: "username", Servidor: "imap.example.test", Senha: "password"}

func TestTestarConfereASenha(t *testing.T) {
	l, _, _ := servidorDeTeste(t)
	if err := l.Testar(context.Background(), contaDeTeste); err != nil {
		t.Fatal(err)
	}
	errada := contaDeTeste
	errada.Senha = "outra"
	if err := l.Testar(context.Background(), errada); !errors.Is(err, ErrSenha) {
		t.Fatalf("senha errada devia dar ErrSenha, deu %v", err)
	}
}

func TestLerFiltraPorRemetenteSemMarcarComoLida(t *testing.T) {
	l, mbox, enviado := servidorDeTeste(t,
		emailDeTeste("LinkedIn <jobalerts-noreply@linkedin.com>", "Vagas novas um", "alerta 1"),
		emailDeTeste("Amiga <amiga@example.com>", "Almoco", "pessoal"),
		emailDeTeste("LinkedIn <jobalerts-noreply@linkedin.com>", "Vagas novas dois", "alerta 2"),
		emailDeTeste("Gupy <no-reply@gupy.io>", "Sua candidatura", "retorno"),
	)
	desde := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	msgs, err := l.Ler(context.Background(), contaDeTeste, desde, []string{"linkedin.com", "gupy.io"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	var assuntos []string
	for _, m := range msgs {
		assuntos = append(assuntos, m.Assunto)
		if strings.Contains(m.De, "<") || m.De != strings.ToLower(m.De) {
			t.Fatalf("remetente devia ser só o endereço, em minúsculas: %q", m.De)
		}
	}
	if strings.Join(assuntos, "|") != "Sua candidatura|Vagas novas dois|Vagas novas um" {
		t.Fatalf("esperava os três do LinkedIn e da Gupy, do mais novo ao mais antigo: %v", assuntos)
	}
	if !strings.Contains(msgs[1].Texto, "alerta 2") || msgs[1].IDMensagem != "Vagas-novas-dois@example.com" {
		t.Fatalf("corpo ou Message-ID: %+v", msgs[1])
	}
	for _, m := range mbox.Messages {
		for _, f := range m.Flags {
			if f == imap.SeenFlag {
				t.Fatal("a leitura marcou um e-mail como lido")
			}
		}
	}
	// As duas garantias de "só leitura" estão nos comandos que o app manda: EXAMINE (nunca SELECT) e BODY.PEEK[]
	// (nunca BODY[] sem PEEK). E nenhum comando que muda a caixa.
	cmds := enviado()
	if !strings.Contains(cmds, "EXAMINE INBOX") || regexp.MustCompile(`(?i)\bSELECT\b`).MatchString(cmds) {
		t.Fatalf("a caixa tem de abrir só com EXAMINE:\n%s", cmds)
	}
	if !strings.Contains(cmds, "BODY.PEEK[]") || regexp.MustCompile(`BODY\[`).MatchString(cmds) {
		t.Fatalf("o corpo tem de ser lido com BODY.PEEK[]:\n%s", cmds)
	}
	if regexp.MustCompile(`(?i)\b(STORE|COPY|MOVE|EXPUNGE|APPEND|DELETE|RENAME|CREATE)\b`).MatchString(cmds) {
		t.Fatalf("o app mandou um comando que muda a caixa:\n%s", cmds)
	}
	// max: só as mais novas.
	if msgs, _ := l.Ler(context.Background(), contaDeTeste, desde, nil, 2); len(msgs) != 2 || msgs[0].Assunto != "Sua candidatura" {
		t.Fatalf("max 2: %+v", msgs)
	}
}

func TestServidorDe(t *testing.T) {
	casos := map[Conta]string{
		{Usuario: "Fulana@Gmail.com"}:                       "imap.gmail.com",
		{Usuario: "f@hotmail.com"}:                          "outlook.office365.com",
		{Usuario: "f@icloud.com"}:                           "imap.mail.me.com",
		{Usuario: "f@empresa.com.br", Servidor: "imap.x.y"}: "imap.x.y",
	}
	for c, quer := range casos {
		if s, err := ServidorDe(c); err != nil || s != quer {
			t.Fatalf("%+v: %q %v", c, s, err)
		}
	}
	if _, err := ServidorDe(Conta{Usuario: "f@empresa.com.br"}); !errors.Is(err, ErrSemServidor) {
		t.Fatal("domínio desconhecido sem servidor devia pedir o servidor")
	}
}

func TestSoEndereco(t *testing.T) {
	if SoEndereco("Fulana Exemplo <Fulana@Example.com>") != "fulana@example.com" || SoEndereco("x@y.com") != "x@y.com" {
		t.Fatal("SoEndereco")
	}
}

func TestBuscarPorAssuntoECorpo(t *testing.T) {
	l, _, _ := servidorDeTeste(t,
		emailDeTeste("Acme <rh@acme.example>", "Sua candidatura", "obrigado"),
		emailDeTeste("Beta <rh@beta.example>", "Atualizacao", "infelizmente nao seguiremos"),
		emailDeTeste("Amiga <amiga@example.com>", "Almoco", "pessoal"),
	)
	desde := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	msgs, err := l.Buscar(context.Background(), contaDeTeste, desde, Busca{Assuntos: []string{"candidatura"}, Corpo: []string{"infelizmente"}}, 10)
	if err != nil {
		t.Fatal(err)
	}
	var assuntos []string
	for _, m := range msgs {
		assuntos = append(assuntos, m.Assunto)
	}
	if strings.Join(assuntos, "|") != "Atualizacao|Sua candidatura" {
		t.Fatalf("esperava o do assunto e o do corpo, sem o pessoal: %v", assuntos)
	}
}
