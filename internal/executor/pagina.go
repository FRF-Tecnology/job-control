package executor

import (
	"context"
	"errors"
)

// Erros fixos que a Pagina devolve para o executor decidir sem olhar texto (jc-a2y.6.5.1).
var (
	// ErrSemBotaoEnviar: o botão final não foi achado; nada foi clicado.
	ErrSemBotaoEnviar = errors.New("navegador: botão de enviar não encontrado")
	// ErrSemOpcao: o valor não é uma das opções do select ou do grupo de rádio.
	ErrSemOpcao = errors.New("navegador: valor fora das opções do campo")
)

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
	// Igual confere, dentro da página, se o valor que o campo já tem é um dos esperados (e-mail e nome sem diferença
	// de caixa, telefone com os mesmos dígitos, código do país pelo número). O valor da página nunca volta.
	Igual(ctx context.Context, c Campo, esperados []string) (bool, error)
	// Desmarcar desmarca uma caixa marcada e confere que ficou desmarcada. Nunca marca (jc-a2y.6.5.2).
	Desmarcar(ctx context.Context, c Campo) error
	// Anexar põe o arquivo do caminho no campo (o executor passa uma cópia já conferida, nunca o original).
	Anexar(ctx context.Context, c Campo, caminho string) error
	// URLAtual é onde a página está agora (o executor confere a cada etapa e logo antes do clique).
	URLAtual(ctx context.Context) (string, error)
	// Proxima avança para a próxima etapa do formulário, sem enviar. Devolve false quando a etapa atual é a
	// última (a que tem o botão de enviar).
	Proxima(ctx context.Context) (bool, error)
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
	CampoDDI       = "ddi"    // código do país do telefone
	CampoSeguir    = "seguir" // caixa "Seguir <empresa>": o app só desmarca, nunca marca (jc-a2y.6.5.2)
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
	// Preenchido: a página já trouxe um valor (texto, opção marcada, rádio ou caixa). O valor não é lido.
	Preenchido bool
}
