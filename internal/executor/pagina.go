package executor

import "context"

// Navegador abre páginas. O navegador de verdade (chromedp, com perfil próprio na pasta de dados) chega no
// jc-a2y.6.2.5; o detector dos motivos de bloqueio, no jc-a2y.6.2.4. Aqui só o contrato e as regras do envio.
type Navegador interface {
	NovaPagina(ctx context.Context) (Pagina, error)
}

// Pagina é uma aba do navegador do app. Nada do que ela devolve (rótulos, textos, URL) muda a fila: o executor
// só escolhe entre resultados de listas fechadas, e nada disso vai para o log.
type Pagina interface {
	// Abrir navega para o endereço aprovado e devolve o endereço onde a página parou (depois de redirecionar).
	Abrir(ctx context.Context, url string) (final string, err error)
	// Bloqueio devolve um motivo da lista fechada (PendenciaDoMotivo) quando a página pede algo que só a pessoa
	// faz, ou "" quando não há nada. Motivo fora da lista vira "outro".
	Bloqueio(ctx context.Context) (motivo string, err error)
	// Campos lista os campos do formulário de candidatura.
	Campos(ctx context.Context) ([]Campo, error)
	Preencher(ctx context.Context, c Campo, valor string) error
	// Anexar põe o arquivo do caminho no campo (o executor passa uma cópia já conferida, nunca o original).
	Anexar(ctx context.Context, c Campo, caminho string) error
	// Enviar clica no botão final. Depois dele, sem confirmação, a candidatura fica incerta (nunca reenvia).
	Enviar(ctx context.Context) error
	// Confirmacao diz se a página mostrou que a candidatura foi recebida.
	Confirmacao(ctx context.Context) (bool, error)
	Fechar() error
}

// Tipos de campo que o adaptador da plataforma reconhece. Os de pedido sensível bloqueiam com o motivo de mesmo nome.
const (
	CampoCurriculo = "curriculo"
	CampoNome      = "nome"
	CampoEmail     = "email"
	CampoTelefone  = "telefone"
	CampoCidade    = "cidade"
	CampoPretensao = "pretensao"
	CampoMensagem  = "mensagem"
	CampoPergunta  = "pergunta" // pergunta de texto livre: só com resposta do pacote para o mesmo texto
	CampoOutro     = "outro"
)

// Campo é um campo do formulário. Rotulo é o texto da pergunta na página: serve só para achar a resposta do
// pacote com o mesmo texto e nunca vai para o log.
type Campo struct {
	ID          string
	Tipo        string
	Rotulo      string
	Obrigatorio bool
}
