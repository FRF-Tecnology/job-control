package emails

import (
	"bufio"
	"context"
	"io"
	"mime/quotedprintable"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/nuvem/codigos"
)

const aviso = "Nós nunca pedimos sua senha por email."

func TestModeloCodigo(t *testing.T) {
	for _, tp := range []codigos.Tipo{codigos.Confirmar, codigos.Recuperar} {
		m := Codigo("ana@exemplo.com", "123456", tp)
		if !strings.Contains(m.Texto, "\n123456\n") || !strings.Contains(m.Texto, aviso) || !strings.Contains(m.Texto, "15 minutos") {
			t.Errorf("%s: texto: %q", tp, m.Texto)
		}
		if m.Assunto != "Seu código do Applyer" || m.Para != "ana@exemplo.com" {
			t.Errorf("%s: %+v", tp, m)
		}
	}
	if !strings.Contains(Codigo("a@b.co", "1", codigos.Recuperar).Texto, "trocar a sua senha") {
		t.Error("recuperar sem o motivo")
	}
}

func TestModelosAvisos(t *testing.T) {
	for _, m := range []Mensagem{JaTemConta("a@b.co"), SenhaTrocada("a@b.co")} {
		if !strings.Contains(m.Texto, aviso) || m.Assunto == "" {
			t.Errorf("%+v", m)
		}
	}
}

func TestFalsoGuarda(t *testing.T) {
	f := &Falso{}
	_ = f.Enviar(context.Background(), Mensagem{Para: "a@b.co"})
	if len(f.Lista()) != 1 {
		t.Fatal("não guardou")
	}
}

func TestSMTPRecusaInjecao(t *testing.T) {
	s := SMTP{Host: "127.0.0.1", Porta: "1", Remetente: "app@exemplo.com"}
	if err := s.Enviar(context.Background(), Mensagem{Para: "a@b.co\r\nBcc: x@y.z", Assunto: "oi"}); err == nil {
		t.Error("Para com CRLF")
	}
	if err := s.Enviar(context.Background(), Mensagem{Para: "a@b.co", Assunto: "oi\nBcc: x@y.z"}); err == nil {
		t.Error("Assunto com LF")
	}
}

func TestMontarCabecalhos(t *testing.T) {
	b := string(montar("app@exemplo.com", "a@b.co", Mensagem{Assunto: "Seu código", Texto: "linha1\nlinha2"},
		time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC), "<abc@exemplo.com>"))
	for _, quer := range []string{"Subject: =?utf-8?q?Seu_c=C3=B3digo?=\r\n", "Content-Type: text/plain; charset=UTF-8\r\n",
		"Content-Transfer-Encoding: quoted-printable\r\n", "Message-ID: <abc@exemplo.com>\r\n",
		"Date: ", "From: ", "To: ", "\r\n\r\nlinha1\r\nlinha2"} {
		if !strings.Contains(b, quer) {
			t.Errorf("falta %q em %q", quer, b)
		}
	}
}

// Servidor SMTP de teste que NÃO anuncia STARTTLS: o envio tem que falhar antes de mandar dados.
func TestSMTPExigeTLS(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	recebeuDados := make(chan bool, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		r := bufio.NewReader(c)
		c.Write([]byte("220 teste ESMTP\r\n"))
		for {
			l, err := r.ReadString('\n')
			if err != nil {
				return
			}
			l = strings.ToUpper(strings.TrimSpace(l))
			switch {
			case strings.HasPrefix(l, "EHLO"):
				c.Write([]byte("250-teste\r\n250 8BITMIME\r\n"))
			case strings.HasPrefix(l, "QUIT"):
				c.Write([]byte("221 tchau\r\n"))
				return
			case strings.HasPrefix(l, "DATA"):
				recebeuDados <- true
				c.Write([]byte("354 vai\r\n"))
			default:
				c.Write([]byte("250 ok\r\n"))
			}
		}
	}()
	host, porta, _ := net.SplitHostPort(ln.Addr().String())
	s := SMTP{Host: host, Porta: porta, Usuario: "u", Senha: "s", Remetente: "app@exemplo.com"}
	err = s.Enviar(context.Background(), Mensagem{Para: "ana@exemplo.com", Assunto: "oi", Texto: "x"})
	if err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("esperava erro de STARTTLS, veio %v", err)
	}
	select {
	case <-recebeuDados:
		t.Fatal("enviou dados sem TLS")
	default:
	}
}

// O corpo vai em quoted-printable: só ASCII, linhas de no máximo 76 caracteres, e volta igual ao texto.
func TestCorpoQuotedPrintable(t *testing.T) {
	texto := "Olá! Use o código abaixo para confirmar o seu email no Applyer:\n\n123456\n\n" + strings.Repeat("ação ", 60) + "\n"
	b := string(montar("app@exemplo.com", "a@b.co", Mensagem{Assunto: "x", Texto: texto}, time.Now(), "<id@exemplo.com>"))
	i := strings.Index(b, "\r\n\r\n")
	corpo := b[i+4:]
	for _, l := range strings.Split(corpo, "\r\n") {
		if len(l) > 76 {
			t.Fatalf("linha com %d caracteres: %q", len(l), l)
		}
		for _, c := range []byte(l) {
			if c > 126 {
				t.Fatalf("byte não ASCII no corpo: %q", l)
			}
		}
	}
	lido, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(corpo)))
	if err != nil {
		t.Fatal(err)
	}
	if quer := strings.ReplaceAll(texto, "\n", "\r\n"); string(lido) != quer {
		t.Fatalf("corpo decodificado diferente:\n%q\n%q", lido, quer)
	}
}

func TestMessageID(t *testing.T) {
	a, err := novoMessageID("app@exemplo.com")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := novoMessageID("app@exemplo.com")
	if a == b || !strings.HasPrefix(a, "<") || !strings.HasSuffix(a, "@exemplo.com>") || len(a) < 20 {
		t.Fatalf("Message-ID %q / %q", a, b)
	}
}
