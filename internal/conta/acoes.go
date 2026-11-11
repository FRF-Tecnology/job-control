package conta

import (
	"context"
)

// TermosVersao é a versão dos termos e da política de privacidade que o app envia ao criar a conta, entrar e
// entrar com Google. "0-rascunho" até a revisão jurídica; a nuvem recusa (409, motivo "termos") quando ela mudou.
const TermosVersao = "0-rascunho"

// As ações abaixo chamam a nuvem pela sessão da própria Conta: quem chama (a API local) não toca em token nem em
// dispositivo. Quando a nuvem devolve uma sessão nova, Entrou a guarda.

// Criar pede à nuvem o código de confirmação por email.
func (c *Conta) Criar(ctx context.Context, email, senha, termos string) error {
	return c.nuvem.Criar(ctx, email, senha, termos)
}

// Confirmar confirma o email com o código e já entra. A senha é a mesma do Criar (a nuvem a confere).
func (c *Conta) Confirmar(ctx context.Context, email, codigo, senha, termos string) error {
	d, err := c.idDispositivo()
	if err != nil {
		return err
	}
	p, err := c.nuvem.Confirmar(ctx, email, codigo, senha, d, termos)
	if err != nil {
		return err
	}
	return c.entrouComSessao(ctx, p)
}

// EntrarComSenha entra com email e senha.
func (c *Conta) EntrarComSenha(ctx context.Context, email, senha, termos string) error {
	d, err := c.idDispositivo()
	if err != nil {
		return err
	}
	p, err := c.nuvem.Entrar(ctx, email, senha, d, termos)
	if err != nil {
		return err
	}
	return c.entrouComSessao(ctx, p)
}

// Recuperar pede o código de nova senha por email.
func (c *Conta) Recuperar(ctx context.Context, email string) error {
	return c.nuvem.Recuperar(ctx, email)
}

// NovaSenha troca a senha com o código e já entra.
func (c *Conta) NovaSenha(ctx context.Context, email, codigo, senha string) error {
	d, err := c.idDispositivo()
	if err != nil {
		return err
	}
	p, err := c.nuvem.NovaSenha(ctx, email, codigo, senha, d)
	if err != nil {
		return err
	}
	return c.entrouComSessao(ctx, p)
}

// entrouComSessao guarda a sessão. Falha ao pedir a licença (ex.: sem internet) não desfaz a entrada: a
// Situacao mostra o motivo. Só falha de verdade quando a sessão não chegou a ser guardada.
func (c *Conta) entrouComSessao(ctx context.Context, p Par) error {
	err := c.Entrou(ctx, p)
	if err != nil && !c.Situacao().Entrada {
		return err
	}
	return nil
}

// Assinar devolve o endereço do Checkout.
func (c *Conta) Assinar(ctx context.Context) (string, error) {
	return c.comAcesso(ctx, c.nuvem.Assinar)
}

// Portal devolve o endereço do portal de gerenciamento da assinatura.
func (c *Conta) Portal(ctx context.Context) (string, error) {
	return c.comAcesso(ctx, c.nuvem.Portal)
}

// Notas lista as notas fiscais da conta.
func (c *Conta) Notas(ctx context.Context) ([]Nota, error) {
	a, err := c.Acesso(ctx)
	if err != nil {
		return nil, err
	}
	return c.nuvem.Notas(ctx, a)
}

// Excluir apaga a conta na nuvem (o email confirma) e, se deu certo, encerra a sessão aqui.
func (c *Conta) Excluir(ctx context.Context, email string) error {
	a, err := c.Acesso(ctx)
	if err != nil {
		return err
	}
	if err := c.nuvem.Excluir(ctx, a, email); err != nil {
		return err
	}
	return c.Sair(ctx)
}

func (c *Conta) comAcesso(ctx context.Context, f func(context.Context, string) (string, error)) (string, error) {
	a, err := c.Acesso(ctx)
	if err != nil {
		return "", err
	}
	return f(ctx, a)
}
