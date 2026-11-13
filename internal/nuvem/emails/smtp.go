package emails

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

// SMTP envia por um servidor SMTP exigindo STARTTLS (recusa servidor sem ele).
type SMTP struct {
	Host, Porta, Usuario, Senha, Remetente string
}

const prazoSMTP = 30 * time.Second

// Enviar entrega a mensagem. Recusa CR/LF em Para e Assunto.
func (s SMTP) Enviar(ctx context.Context, m Mensagem) error {
	if strings.ContainsAny(m.Para, "\r\n") || strings.ContainsAny(m.Assunto, "\r\n") {
		return errors.New("cabeçalho inválido")
	}
	para, err := mail.ParseAddress(m.Para)
	if err != nil {
		return fmt.Errorf("destinatário inválido: %w", err)
	}
	de, err := mail.ParseAddress(s.Remetente)
	if err != nil {
		return fmt.Errorf("remetente inválido: %w", err)
	}

	d := net.Dialer{Timeout: prazoSMTP}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(s.Host, s.Porta))
	if err != nil {
		return fmt.Errorf("conectar no SMTP: %w", err)
	}
	limite := time.Now().Add(prazoSMTP)
	if dl, ok := ctx.Deadline(); ok && dl.Before(limite) {
		limite = dl
	}
	conn.SetDeadline(limite)
	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("conversar com o SMTP: %w", err)
	}
	defer c.Close()

	if ok, _ := c.Extension("STARTTLS"); !ok {
		return errors.New("o servidor SMTP não oferece STARTTLS")
	}
	if err := c.StartTLS(&tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}); err != nil {
		return fmt.Errorf("STARTTLS: %w", err)
	}
	if s.Usuario != "" {
		if err := c.Auth(smtp.PlainAuth("", s.Usuario, s.Senha, s.Host)); err != nil {
			return fmt.Errorf("autenticar no SMTP: %w", err)
		}
	}
	if err := c.Mail(de.Address); err != nil {
		return err
	}
	if err := c.Rcpt(para.Address); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	id, err := novoMessageID(de.Address)
	if err != nil {
		return err
	}
	if _, err := w.Write(montar(de.String(), para.String(), m, time.Now(), id)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// novoMessageID gera um Message-ID aleatório no domínio do remetente (alguns filtros de spam o exigem).
func novoMessageID(remetente string) (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "<" + hex.EncodeToString(b[:]) + remetente[strings.LastIndex(remetente, "@"):] + ">", nil
}

// montar escreve o email com quebras CRLF: cabeçalhos e corpo UTF-8 em quoted-printable (só ASCII, linhas
// de até 76 caracteres, sem depender de 8BITMIME no servidor).
func montar(de, para string, m Mensagem, agora time.Time, messageID string) []byte {
	var corpo strings.Builder
	qp := quotedprintable.NewWriter(&corpo)
	// Escrever num strings.Builder não falha. O Writer troca cada \n do texto por CRLF.
	_, _ = qp.Write([]byte(strings.ReplaceAll(m.Texto, "\r\n", "\n")))
	_ = qp.Close()
	cab := []string{
		"From: " + de,
		"To: " + para,
		"Subject: " + mime.QEncoding.Encode("utf-8", m.Assunto),
		"Date: " + agora.Format(time.RFC1123Z),
		"Message-ID: " + messageID,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: quoted-printable",
	}
	return []byte(strings.Join(cab, "\r\n") + "\r\n\r\n" + corpo.String())
}
