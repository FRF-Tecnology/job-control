package api

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/config"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Apagar meus dados e sair das plataformas (LGPD, jc-a2y.49; contrato 0.4.10 e docs/propostas/apagar-dados.md).
// Só a tela; nada vai para a nuvem. O log leva só o resultado por item: nunca caminho (tem o nome do usuário),
// conteúdo nem chave.

// FraseApagar é a frase que a pessoa digita na tela; o servidor confere exatamente.
const FraseApagar = "APAGAR MEUS DADOS"

// Itens de ItemApagado, na ordem em que saem.
const (
	itemChaves     = "chaves"
	itemNavegador  = "navegador"
	itemCurriculos = "curriculos"
	itemBackups    = "backups"
	itemBanco      = "banco"
	// itemPastaAntiga: a pasta "Central de Candidaturas" que ficou ao lado da Applyer (jc-etv.7). Só aparece quando
	// ela existe; no apagar, o banco, os currículos, os backups e o navegador dela; no sair, o navegador.
	itemPastaAntiga = "pasta-antiga"
)

// limparPastaAntiga apaga na pasta antiga os mesmos itens, pelas mesmas regras (removerTudo; a pasta só sai se ficar
// vazia). Antes, o ProtegerPasta confere o dono (no Windows, pasta de outro usuário não é tocada) e fecha a DACL.
func limparPastaAntiga(antiga string, soNavegador bool) error {
	if err := config.ProtegerPasta(antiga); err != nil {
		return err
	}
	erros := []error{removerTudo(config.PastaNavegador(antiga))}
	if !soNavegador {
		erros = append(erros, removerTudo(filepath.Join(antiga, "curriculos")), removerTudo(filepath.Join(antiga, "backups")))
		for _, sufixo := range []string{"", "-wal", "-shm", "-journal"} {
			erros = append(erros, removerTudo(config.CaminhoBanco(antiga)+sufixo))
		}
		os.Remove(antiga) // só se ficou vazia
	}
	return errors.Join(erros...)
}

// esperaAntesDeFechar dá tempo para a resposta chegar à tela antes de o app fechar.
const esperaAntesDeFechar = 500 * time.Millisecond

// ResultadoApagar é a resposta das duas rotas.
type ResultadoApagar struct {
	Apagado  []string `json:"apagado"`
	Faltou   []string `json:"faltou"`
	Completo bool     `json:"completo"`
	Fechar   bool     `json:"fechar"`
}

// falhaDe classifica a falha numa lista fechada para o log (M7): nunca o texto do erro, que traz o caminho.
func falhaDe(nome string, err error) string {
	switch {
	case nome == itemChaves:
		return "cofre"
	case emUso(err):
		return "em-uso"
	case errors.Is(err, fs.ErrPermission):
		return "permissao"
	}
	return "outro"
}

func (r *ResultadoApagar) item(nome string, err error) {
	if err != nil {
		r.Faltou = append(r.Faltou, nome)
		log.Printf("apagar: %s faltou (%s)", nome, falhaDe(nome, err))
		return
	}
	r.Apagado = append(r.Apagado, nome)
	log.Printf("apagar: %s ok", nome)
}

// ErroFechando: depois de apagar os dados, todo pedido recebe 503 até o app fechar.
const ErroFechando = "os dados foram apagados; o app está fechando"

func respostaFechando() Resposta {
	return Resposta{Status: 503, Corpo: Objeto{"erro": ErroFechando}}
}

// ehPedidoDeApagar: POST /dados/apagar passa pela trava de escrita, fora do roteamento comum.
func ehPedidoDeApagar(ped Pedido) bool {
	caminho, _, _ := strings.Cut(ped.Caminho, "?")
	p := strings.TrimPrefix(caminho, Prefixo)
	return strings.ToUpper(ped.Metodo) == "POST" && p == "/dados/apagar"
}

// apagarComTrava confere o pedido sem trava (400, 403) e só então pega a trava de escrita: os pedidos em curso
// terminam antes, e nenhum outro (nem um POST /backup do assistente) grava entre apagar e fechar o banco.
func (a *API) apagarComTrava(ctx context.Context, ped Pedido) (Resposta, error) {
	autor := ped.Autor
	if autor == "" {
		autor = "eu"
	}
	if !ehObjeto(ped.Corpo) {
		return Resposta{}, falha(400, "envie um objeto JSON")
	}
	c := dominio.ObjetoDe(ped.Corpo)
	if err := validarApagar(c, autor); err != nil {
		return Resposta{}, err
	}
	a.trava.Lock()
	defer a.trava.Unlock()
	if a.apagando {
		return respostaFechando(), nil
	}
	return a.apagarDados(ctx)
}

func validarApagar(c Objeto, autor string) error {
	if err := soCampos(c, "confirmacao"); err != nil {
		return err
	}
	if frase, _ := c["confirmacao"].(string); frase != FraseApagar {
		return erroCampo("confirmacao: digite exatamente " + FraseApagar)
	}
	if autor != "eu" {
		return &dominio.ErroSoPessoa{Msg: "só a pessoa pode apagar os dados"}
	}
	return nil
}

func (a *API) rotasDados(ctx context.Context, m, p string, c Objeto, autor string) (Resposta, bool, error) {
	switch {
	case m == "POST" && p == "/navegador/sair":
		r, err := a.sairDasPlataformas(ctx, c, autor)
		return r, true, err
	case m == "POST" && p == "/navegador/entrar":
		r, err := a.entrarNoNavegador(ctx, c, autor)
		return r, true, err
	case m == "GET" && p == "/dados/pasta":
		r, err := a.estadoDaPasta(ctx)
		return r, true, err
	}
	return Resposta{}, false, nil
}

// avisosDaPasta: lista fechada do contrato (o mesmo texto de config.AvisoAntigaFicou e config.AvisoMigracaoAdiada).
var avisosDaPasta = map[string]bool{config.AvisoAntigaFicou: true, config.AvisoMigracaoAdiada: true}

// estadoDaPasta (jc-etv.14): {aviso, migradaEm}, sem caminho. O aviso é o desta abertura (Opcoes.AvisoPasta); fora
// da lista, null. migradaEm vem da meta "pasta" que o app grava no rename; só texto passa.
func (a *API) estadoDaPasta(ctx context.Context) (Resposta, error) {
	o := Objeto{"aviso": nil, "migradaEm": nil}
	if avisosDaPasta[a.avisoPasta] {
		o["aviso"] = a.avisoPasta
	}
	var meta map[string]any
	if achou, err := a.arm.LerMeta(ctx, "pasta", &meta); err == nil && achou {
		if em, ok := meta["migradaEm"].(string); ok && em != "" {
			o["migradaEm"] = em
		}
	}
	return ok(o)
}

// semEnvioEmAndamento: com candidatura em enviando, o executor está numa página: nada é apagado (409).
func (a *API) semEnvioEmAndamento(ctx context.Context, msg string) error {
	e, err := a.arm.Carregar(ctx)
	if err != nil {
		return err
	}
	for _, o := range e.Candidaturas {
		if dominio.Igual(o["estado"], dominio.CandEnviando) {
			return &dominio.ErroConflito{Msg: msg, Estado: dominio.CandEnviando}
		}
	}
	return nil
}

// removerTudo apaga o caminho e confere que ele não existe mais.
func removerTudo(caminho string) error {
	if err := os.RemoveAll(caminho); err != nil {
		return err
	}
	if _, err := os.Lstat(caminho); !errors.Is(err, os.ErrNotExist) {
		return errors.New("ainda existe")
	}
	return nil
}

// apagarDados roda com a trava de escrita da API (apagarComTrava).
func (a *API) apagarDados(ctx context.Context) (Resposta, error) {
	pasta := a.arm.Pasta()
	if pasta == "" {
		return Resposta{}, erroCampo("banco em memória: não há pasta de dados para apagar")
	}
	// 1) Para o executor: espera o ciclo acabar e o Chrome sair (no build padrão, não faz nada). Só depois confere
	// enviando, para não haver corrida com um envio que começasse no meio. Com 409, o executor fica parado.
	a.pararEnvio()
	if err := a.semEnvioEmAndamento(ctx, "há uma candidatura sendo enviada agora: nada foi apagado, e o envio ficou parado até você abrir o app de novo"); err != nil {
		return Resposta{}, err
	}
	// Daqui em diante, todo pedido recebe 503 (a trava de escrita já barra os que chegarem agora).
	a.apagando = true
	res := ResultadoApagar{Apagado: []string{}, Faltou: []string{}, Fechar: true}

	// 2) Chaves de IA no cofre do sistema (a que já não existe conta como apagada).
	var errChaves error
	for _, p := range comChave {
		if err := a.cofre.Apagar(p); err != nil {
			errChaves = err
		}
	}
	res.item(itemChaves, errChaves)

	// 3) Perfil do navegador, currículos e backups.
	res.item(itemNavegador, removerTudo(config.PastaNavegador(pasta)))
	res.item(itemCurriculos, removerTudo(filepath.Join(pasta, "curriculos")))
	res.item(itemBackups, removerTudo(filepath.Join(pasta, "backups")))

	// 4) O banco por último: fecha antes (no Windows, arquivo aberto não sai) e apaga também -wal e -shm.
	var errBanco error
	if err := a.arm.Fechar(); err != nil {
		errBanco = err
	}
	for _, sufixo := range []string{"", "-wal", "-shm", "-journal"} {
		if err := removerTudo(config.CaminhoBanco(pasta) + sufixo); err != nil {
			errBanco = err
		}
	}
	res.item(itemBanco, errBanco)

	// 5) A pasta de dados sai se ficou vazia (os.Remove não apaga pasta com coisa dentro).
	os.Remove(pasta)

	// 6) A pasta antiga que ficou ao lado (jc-etv.7): os mesmos itens, pelas mesmas regras.
	if antiga := config.PastaAntigaQueFicou(pasta); antiga != "" {
		res.item(itemPastaAntiga, limparPastaAntiga(antiga, false))
	}

	res.Completo = len(res.Faltou) == 0
	// 6) O app fecha logo depois de a resposta chegar à tela.
	time.AfterFunc(esperaAntesDeFechar, a.aoApagar)
	return ok(res)
}

func (a *API) sairDasPlataformas(ctx context.Context, c Objeto, autor string) (Resposta, error) {
	if err := soCampos(c); err != nil {
		return Resposta{}, err
	}
	if autor != "eu" {
		return Resposta{}, &dominio.ErroSoPessoa{Msg: "só a pessoa pode sair das plataformas"}
	}
	pasta := a.arm.Pasta()
	if pasta == "" {
		return Resposta{}, erroCampo("banco em memória: não há perfil do navegador")
	}
	// Nenhum ciclo novo do executor até o fim; o atual termina antes. O navegador fecha esperando o processo sair.
	soltar := a.suspenderEnvio()
	defer soltar()
	if err := a.semEnvioEmAndamento(ctx, "há uma candidatura sendo enviada agora; espere terminar"); err != nil {
		return Resposta{}, err
	}
	a.fecharNavegador()
	res := ResultadoApagar{Apagado: []string{}, Faltou: []string{}}
	res.item(itemNavegador, removerTudo(config.PastaNavegador(pasta)))
	if antiga := config.PastaAntigaQueFicou(pasta); antiga != "" { // a sessão que ficou na pasta antiga (jc-etv.7)
		res.item(itemPastaAntiga, limparPastaAntiga(antiga, true))
	}
	res.Completo = len(res.Faltou) == 0
	return ok(res)
}
