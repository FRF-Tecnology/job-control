// Package caixa lê a caixa de e-mail da pessoa por IMAP, só para leitura.
//
// Duas garantias vêm do próprio protocolo, não de cuidado do código:
//   - a caixa é aberta com EXAMINE (Select com somente leitura): o servidor recusa qualquer mudança;
//   - o conteúdo é lido com BODY.PEEK[]: a mensagem não fica marcada como lida.
//
// O app nunca envia, responde, move, marca ou apaga e-mail. A conexão é sempre TLS na porta 993, com uma
// "senha de app" (a do Google, revogável a qualquer hora em myaccount.google.com/apppasswords), nunca a senha da
// conta. Quem guarda a senha é o cofre do sistema; este pacote só a recebe para conectar.
package caixa

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/mail"
	"sort"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-message/charset"
	gomail "github.com/emersion/go-message/mail"
)

func init() {
	// Lê e-mails em ISO-8859-1, Windows-1252 e afins (comuns em e-mails de RH no Brasil).
	imap.CharsetReader = charset.Reader
}

// Conta é o que a pessoa informa para conectar.
type Conta struct {
	Usuario  string // o endereço de e-mail
	Servidor string // host IMAP; vazio = deduzido do domínio do endereço
	Senha    string // a senha de app
}

// Mensagem lida da caixa. Texto e HTML vêm limitados (tetoCorpo) e sem anexos.
type Mensagem struct {
	UID        uint32
	IDMensagem string // o Message-ID do cabeçalho; vazio se o e-mail não tiver
	De         string // só o endereço, sem o nome (é dado de terceiro)
	Assunto    string
	Data       time.Time
	Texto      string
	HTML       string
}

var (
	ErrSemServidor = errors.New("não sei o servidor IMAP deste e-mail: informe o servidor")
	ErrSenha       = errors.New("o e-mail não aceitou a senha: use uma senha de app do Google (não a senha da conta)")
	ErrConexao     = errors.New("não consegui falar com o servidor de e-mail")
)

// servidores conhecidos pelo domínio do endereço (porta 993, TLS).
var servidores = map[string]string{
	"gmail.com": "imap.gmail.com", "googlemail.com": "imap.gmail.com",
	"outlook.com": "outlook.office365.com", "hotmail.com": "outlook.office365.com", "live.com": "outlook.office365.com",
	"icloud.com": "imap.mail.me.com", "me.com": "imap.mail.me.com", "mac.com": "imap.mail.me.com",
}

// ServidorDe devolve o host IMAP do endereço: o informado ou o conhecido pelo domínio.
func ServidorDe(c Conta) (string, error) {
	if s := strings.TrimSpace(c.Servidor); s != "" {
		return s, nil
	}
	if i := strings.LastIndex(c.Usuario, "@"); i >= 0 {
		if s, ok := servidores[strings.ToLower(strings.TrimSpace(c.Usuario[i+1:]))]; ok {
			return s, nil
		}
	}
	return "", ErrSemServidor
}

// Leitor conecta e lê. Discar só é trocado nos testes (servidor IMAP local, sem TLS).
type Leitor struct {
	Discar func(ctx context.Context, host string) (*client.Client, error)
}

// tetoCorpo: um e-mail de alerta tem algumas dezenas de KB; mais que isso é corpo que o app não precisa.
const tetoCorpo = 512 << 10

func discarTLS(ctx context.Context, host string) (*client.Client, error) {
	d := &net.Dialer{Timeout: 20 * time.Second}
	conn, err := tls.DialWithDialer(d, "tcp", net.JoinHostPort(host, "993"), &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
	if err != nil {
		return nil, err
	}
	if prazo, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(prazo)
	}
	return client.New(conn)
}

func (l Leitor) abrir(ctx context.Context, c Conta) (*client.Client, error) {
	host, err := ServidorDe(c)
	if err != nil {
		return nil, err
	}
	discar := l.Discar
	if discar == nil {
		discar = discarTLS
	}
	cli, err := discar(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrConexao, host)
	}
	if err := cli.Login(strings.TrimSpace(c.Usuario), strings.ReplaceAll(c.Senha, " ", "")); err != nil {
		cli.Logout()
		return nil, ErrSenha
	}
	// EXAMINE: a caixa abre só para leitura (o servidor recusa mudanças).
	if _, err := cli.Select("INBOX", true); err != nil {
		cli.Logout()
		return nil, fmt.Errorf("%w: a caixa de entrada não abriu", ErrConexao)
	}
	return cli, nil
}

// Testar confere endereço, servidor e senha: conecta, abre a caixa só para leitura e sai.
func (l Leitor) Testar(ctx context.Context, c Conta) error {
	cli, err := l.abrir(ctx, c)
	if err != nil {
		return err
	}
	return cli.Logout()
}

// Busca diz o que procurar: e-mails cujo remetente, assunto ou corpo contém algum dos trechos (a união das buscas).
// Tudo vazio = qualquer e-mail. Os trechos vão na busca do servidor; use só ASCII.
type Busca struct {
	Remetentes []string
	Assuntos   []string
	Corpo      []string
}

// Ler devolve as mensagens recebidas desde `desde` (o dia, como o IMAP entende) cujo remetente contém um dos
// trechos de `remetentes` (vazio = qualquer remetente), da mais nova para a mais antiga, no máximo `max`.
// Não marca nada como lido.
func (l Leitor) Ler(ctx context.Context, c Conta, desde time.Time, remetentes []string, max int) ([]Mensagem, error) {
	return l.Buscar(ctx, c, desde, Busca{Remetentes: remetentes}, max)
}

// Buscar é o Ler com remetente, assunto e corpo. Não marca nada como lido.
func (l Leitor) Buscar(ctx context.Context, c Conta, desde time.Time, b Busca, max int) ([]Mensagem, error) {
	cli, err := l.abrir(ctx, c)
	if err != nil {
		return nil, err
	}
	defer cli.Logout()

	uids := map[uint32]bool{}
	buscar := func(campo, trecho string) error {
		crit := imap.NewSearchCriteria()
		crit.Since = desde
		switch campo {
		case "From", "Subject":
			crit.Header.Add(campo, trecho)
		case "Body":
			crit.Body = []string{trecho}
		}
		achados, err := cli.UidSearch(crit)
		for _, u := range achados {
			uids[u] = true
		}
		return err
	}
	if len(b.Remetentes)+len(b.Assuntos)+len(b.Corpo) == 0 {
		err = buscar("", "")
	}
	for _, grupo := range []struct {
		campo   string
		trechos []string
	}{{"From", b.Remetentes}, {"Subject", b.Assuntos}, {"Body", b.Corpo}} {
		for _, t := range grupo.trechos {
			if err == nil {
				err = buscar(grupo.campo, t)
			}
		}
	}
	if err != nil {
		return nil, fmt.Errorf("%w: a busca falhou", ErrConexao)
	}
	lista := make([]uint32, 0, len(uids))
	for u := range uids {
		lista = append(lista, u)
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i] > lista[j] }) // mais novas primeiro
	if max > 0 && len(lista) > max {
		lista = lista[:max]
	}
	if len(lista) == 0 {
		return []Mensagem{}, nil
	}

	conjunto := new(imap.SeqSet)
	conjunto.AddNum(lista...)
	secao := &imap.BodySectionName{Peek: true} // BODY.PEEK[]: não marca como lida
	itens := []imap.FetchItem{imap.FetchUid, imap.FetchEnvelope, secao.FetchItem()}
	canal := make(chan *imap.Message, len(lista))
	feito := make(chan error, 1)
	go func() { feito <- cli.UidFetch(conjunto, itens, canal) }()

	var saida []Mensagem
	for msg := range canal {
		m := Mensagem{UID: msg.Uid}
		if env := msg.Envelope; env != nil {
			m.IDMensagem, m.Assunto, m.Data = strings.Trim(env.MessageId, "<> "), env.Subject, env.Date
			if len(env.From) > 0 {
				m.De = strings.ToLower(env.From[0].Address())
			}
		}
		if corpo := msg.GetBody(secao); corpo != nil {
			m.Texto, m.HTML = lerCorpo(io.LimitReader(corpo, tetoCorpo))
		}
		saida = append(saida, m)
	}
	if err := <-feito; err != nil {
		return nil, fmt.Errorf("%w: a leitura falhou", ErrConexao)
	}
	sort.Slice(saida, func(i, j int) bool { return saida[i].UID > saida[j].UID })
	return saida, nil
}

// lerCorpo junta as partes text/plain e text/html (sem anexos). E-mail malformado devolve o que deu para ler.
func lerCorpo(r io.Reader) (texto, html string) {
	leitor, err := gomail.CreateReader(r)
	if err != nil {
		return "", ""
	}
	var t, h strings.Builder
	for {
		parte, err := leitor.NextPart()
		if err != nil {
			break
		}
		cab, eTexto := parte.Header.(*gomail.InlineHeader)
		if !eTexto {
			continue // anexo
		}
		tipo, _, _ := cab.ContentType()
		b, _ := io.ReadAll(io.LimitReader(parte.Body, tetoCorpo))
		switch tipo {
		case "text/plain":
			t.Write(b)
		case "text/html":
			h.Write(b)
		}
	}
	return t.String(), h.String()
}

// SoEndereco tira o nome de um remetente ("Fulana <f@x.com>" vira "f@x.com").
func SoEndereco(de string) string {
	if a, err := mail.ParseAddress(de); err == nil {
		return strings.ToLower(a.Address)
	}
	return strings.ToLower(strings.Trim(strings.TrimSpace(de), "<>"))
}
