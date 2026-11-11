package emails

import (
	"github.com/FRF-Tecnology/job-control/internal/nuvem/codigos"
)

const avisoSenha = "Nós nunca pedimos sua senha por email."

// Codigo é o email com o código de 6 dígitos.
func Codigo(para, codigo string, t codigos.Tipo) Mensagem {
	motivo := "confirmar o seu email"
	if t == codigos.Recuperar {
		motivo = "trocar a sua senha"
	}
	return Mensagem{
		Para:    para,
		Assunto: "Seu código do Applyer",
		Texto: "Olá!\n\nUse o código abaixo para " + motivo + " no Applyer:\n\n" +
			codigo + "\n\nEle vale por 15 minutos. Se não foi você que pediu, é só ignorar este email.\n\n" +
			avisoSenha + "\n",
	}
}

// JaTemConta avisa quem tentou criar uma conta com um email que já tem cadastro.
func JaTemConta(para string) Mensagem {
	return Mensagem{
		Para:    para,
		Assunto: "Você já tem uma conta no Applyer",
		Texto: "Olá!\n\nAlguém tentou criar uma conta no Applyer com este email, mas ele já tem cadastro. " +
			"Se foi você, é só entrar com o seu email e a sua senha; se esqueceu a senha, use a opção de recuperar a senha.\n\n" +
			"Se não foi você, pode ignorar este email.\n\n" + avisoSenha + "\n",
	}
}

// SenhaTrocada avisa que a senha foi alterada.
func SenhaTrocada(para string) Mensagem {
	return Mensagem{
		Para:    para,
		Assunto: "Sua senha do Applyer foi trocada",
		Texto: "Olá!\n\nA senha da sua conta no Applyer acabou de ser trocada.\n\n" +
			"Se foi você, está tudo certo. Se não foi, recupere a sua conta agora pela opção de recuperar a senha " +
			"e nos avise respondendo este email.\n\n" + avisoSenha + "\n",
	}
}
