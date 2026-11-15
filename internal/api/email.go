package api

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/caixa"
)

// Conta de e-mail (IMAP, só leitura): o app lê as respostas das empresas e os alertas de vaga da caixa da pessoa.
// Só a tela conecta e desconecta. A senha de app vai para o cofre do sistema (nunca para o banco, o log ou a
// resposta); o endereço e o servidor ficam no meta do banco. O log não leva endereço nem senha.

const (
	chaveCofreEmail = "email-imap"  // no cofre: a senha de app
	chaveMetaEmail  = "email-conta" // no meta: {usuario, servidor}
	prazoTesteEmail = 30 * time.Second
)

type contaEmail struct {
	Usuario  string `json:"usuario"`
	Servidor string `json:"servidor"`
}

func (a *API) rotasEmail(ctx context.Context, m, p string, c Objeto, autor string) (Resposta, bool, error) {
	conta := p == "/email/conta" && (m == "GET" || m == "PUT" || m == "DELETE")
	if !conta {
		return Resposta{}, false, nil
	}
	if autor != "eu" || a.canal != "" {
		return Resposta{}, true, falha(403, msgForaDoAssistente)
	}
	switch m {
	case "GET":
		r, err := a.estadoEmail(ctx)
		return r, true, err
	case "PUT":
		r, err := a.conectarEmail(ctx, c)
		return r, true, err
	}
	r, err := a.desconectarEmail(ctx)
	return r, true, err
}

// estadoEmail: {conectada, usuario, servidor}. conectada exige endereço no banco e senha no cofre.
func (a *API) estadoEmail(ctx context.Context) (Resposta, error) {
	var conta contaEmail
	if _, err := a.arm.LerMeta(ctx, chaveMetaEmail, &conta); err != nil {
		return Resposta{}, err
	}
	senha, err := a.cofre.Ler(chaveCofreEmail)
	if err != nil {
		return Resposta{}, err
	}
	conectada := conta.Usuario != "" && senha != ""
	if !conectada {
		conta = contaEmail{}
	}
	return ok(Objeto{"conectada": conectada, "usuario": conta.Usuario, "servidor": conta.Servidor})
}

// conectarEmail confere a conta no servidor (entra e abre a caixa só para leitura) antes de gravar.
func (a *API) conectarEmail(ctx context.Context, c Objeto) (Resposta, error) {
	if err := soCampos(c, "usuario", "senha", "servidor"); err != nil {
		return Resposta{}, err
	}
	usuario, _ := c["usuario"].(string)
	senha, _ := c["senha"].(string)
	servidor, _ := c["servidor"].(string)
	usuario, servidor = strings.TrimSpace(usuario), strings.TrimSpace(servidor)
	if !strings.Contains(usuario, "@") || len(usuario) > 254 {
		return Resposta{}, erroCampo("usuario: o endereço de e-mail")
	}
	if strings.TrimSpace(senha) == "" || len(senha) > 256 {
		return Resposta{}, erroCampo("senha: a senha de app do e-mail")
	}
	if strings.ContainsAny(servidor, " /:@") || len(servidor) > 253 {
		return Resposta{}, erroCampo("servidor: só o nome do servidor IMAP, como imap.exemplo.com")
	}
	conta := caixa.Conta{Usuario: usuario, Servidor: servidor, Senha: senha}
	host, err := caixa.ServidorDe(conta)
	if err != nil {
		return Resposta{}, erroCampo(err.Error())
	}
	teste, cancelar := context.WithTimeout(ctx, prazoTesteEmail)
	defer cancelar()
	switch err := a.leitorEmail.Testar(teste, conta); {
	case errors.Is(err, caixa.ErrSenha):
		return Resposta{}, erroCampo(err.Error())
	case err != nil:
		return Resposta{}, falha(502, "não consegui falar com "+host+": confira a internet e o servidor e tente de novo")
	}
	if err := a.cofre.Gravar(chaveCofreEmail, strings.ReplaceAll(senha, " ", "")); err != nil {
		return Resposta{}, err
	}
	if err := a.arm.GravarMeta(ctx, chaveMetaEmail, contaEmail{Usuario: usuario, Servidor: host}); err != nil {
		return Resposta{}, err
	}
	return ok(Objeto{"conectada": true, "usuario": usuario, "servidor": host})
}

// desconectarEmail apaga a senha do cofre e a conta do banco. Sem conta, não faz nada (e responde igual).
func (a *API) desconectarEmail(ctx context.Context) (Resposta, error) {
	if err := a.cofre.Apagar(chaveCofreEmail); err != nil {
		return Resposta{}, err
	}
	if err := a.arm.GravarMeta(ctx, chaveMetaEmail, contaEmail{}); err != nil {
		return Resposta{}, err
	}
	return ok(Objeto{"conectada": false, "usuario": "", "servidor": ""})
}
