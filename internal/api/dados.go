package api

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
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
)

// esperaAntesDeFechar dá tempo para a resposta chegar à tela antes de o app fechar.
const esperaAntesDeFechar = 500 * time.Millisecond

// ResultadoApagar é a resposta das duas rotas.
type ResultadoApagar struct {
	Apagado  []string `json:"apagado"`
	Faltou   []string `json:"faltou"`
	Completo bool     `json:"completo"`
	Fechar   bool     `json:"fechar"`
}

func (r *ResultadoApagar) item(nome string, err error) {
	if err != nil {
		r.Faltou = append(r.Faltou, nome)
		log.Printf("apagar: %s faltou (%T)", nome, err)
		return
	}
	r.Apagado = append(r.Apagado, nome)
	log.Printf("apagar: %s ok", nome)
}

func (a *API) rotasDados(ctx context.Context, m, p string, c Objeto, autor string) (Resposta, bool, error) {
	switch {
	case m == "POST" && p == "/dados/apagar":
		r, err := a.apagarDados(ctx, c, autor)
		return r, true, err
	case m == "POST" && p == "/navegador/sair":
		r, err := a.sairDasPlataformas(ctx, c, autor)
		return r, true, err
	}
	return Resposta{}, false, nil
}

// semEnvioEmAndamento: com candidatura em enviando, o executor está numa página: nada é apagado (409).
func (a *API) semEnvioEmAndamento(ctx context.Context) error {
	e, err := a.arm.Carregar(ctx)
	if err != nil {
		return err
	}
	for _, o := range e.Candidaturas {
		if dominio.Igual(o["estado"], dominio.CandEnviando) {
			return &dominio.ErroConflito{Msg: "há uma candidatura sendo enviada agora; espere terminar", Estado: dominio.CandEnviando}
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

func (a *API) apagarDados(ctx context.Context, c Objeto, autor string) (Resposta, error) {
	if err := soCampos(c, "confirmacao"); err != nil {
		return Resposta{}, err
	}
	if frase, _ := c["confirmacao"].(string); frase != FraseApagar {
		return Resposta{}, erroCampo("confirmacao: digite exatamente " + FraseApagar)
	}
	if autor != "eu" {
		return Resposta{}, &dominio.ErroSoPessoa{Msg: "só a pessoa pode apagar os dados"}
	}
	pasta := a.arm.Pasta()
	if pasta == "" {
		return Resposta{}, erroCampo("banco em memória: não há pasta de dados para apagar")
	}
	// 1) Para o executor: espera o ciclo acabar e o Chrome sair (no build padrão, não faz nada). Só depois confere
	// enviando, para não haver corrida com um envio que começasse no meio. Com 409, o executor fica parado.
	a.pararEnvio()
	if err := a.semEnvioEmAndamento(ctx); err != nil {
		return Resposta{}, err
	}
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
	if err := a.semEnvioEmAndamento(ctx); err != nil {
		return Resposta{}, err
	}
	a.fecharNavegador()
	res := ResultadoApagar{Apagado: []string{}, Faltou: []string{}}
	res.item(itemNavegador, removerTudo(config.PastaNavegador(pasta)))
	res.Completo = len(res.Faltou) == 0
	return ok(res)
}
