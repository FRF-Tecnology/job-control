package dominio

import (
	"errors"
	"time"
)

// Aprovar a lista do dia em lote (jc-a2y.6.13, docs/propostas/aprovar-lote.md). Cada item traz a impressão que a
// tela mostrou e passa pela mesma conferência do aprovar de uma (M2). Item recusado não afeta os outros. Puro: recebe
// o estado e devolve o novo; quem chama grava tudo numa transação só.

// ItemLote: um item do pedido.
type ItemLote struct {
	ID        string
	Impressao string
}

// Recusa: um item que não foi aprovado, com o motivo da lista fechada (sem texto da candidatura).
type Recusa struct {
	ID     string `json:"id"`
	Motivo string `json:"motivo"`
}

// Motivos de recusa do lote.
const (
	RecusaNaoEncontrada  = "nao-encontrada"
	RecusaEstado         = "estado"
	RecusaImpressaoMudou = "impressao-mudou"
	RecusaVagaFora       = "vaga-fora"
	RecusaLimite         = "limite"
)

// MaxItensLote: 3 plataformas × o maior limite.
const MaxItensLote = 3 * LimiteMaximo

// AprovadasPendentes: aprovadas na plataforma que ainda não saíram (estado aprovada, aprovação válida agora). Não
// olha a marca de uso: na volta automática do .6.11 a tentativa sai de tentativas, e a candidatura conta uma vez
// só, aqui. Em enviando, ela conta só nas tentativas (UsadasNoDia).
func AprovadasPendentes(e Estado, plataforma string, agora time.Time) int {
	n := 0
	for _, o := range e.Candidaturas {
		c, err := candidaturaDe(o)
		if err != nil || c.Plataforma != plataforma || c.Estado != CandAprovada || c.Aprovacao == nil {
			continue
		}
		if AprovacaoValida(c.Aprovacao, agora) {
			n++
		}
	}
	return n
}

// AprovarLote aprova os itens na ordem do pedido. Só a pessoa (nada é aprovado sem ela). id repetido, lista vazia
// ou grande demais: validação, e nada muda. O limite de cada plataforma conta as já enviadas hoje, as aprovadas que
// ainda não saíram e as aprovadas neste lote.
func AprovarLote(e Estado, itens []ItemLote, agora time.Time, limites map[string]int, ctx Ctx) (Estado, []string, []Recusa, error) {
	if err := soPessoa(ctx, "aprovar"); err != nil {
		return e, nil, nil, err
	}
	if len(itens) == 0 || len(itens) > MaxItensLote {
		return e, nil, nil, validacao("itens: de 1 a 120")
	}
	vistos := map[string]bool{}
	for _, it := range itens {
		if vistos[it.ID] {
			return e, nil, nil, validacao("itens: id repetido")
		}
		vistos[it.ID] = true
	}
	novo := e
	aprovadas, recusadas := []string{}, []Recusa{}
	for _, it := range itens {
		motivo := ""
		_, c, err := acharCandidatura(novo, it.ID)
		var nao *ErroNaoEncontrado
		switch {
		case errors.As(err, &nao):
			motivo = RecusaNaoEncontrada
		case err != nil:
			return e, nil, nil, err
		case c.Estado != CandPreparada:
			motivo = RecusaEstado
		case it.Impressao != c.Impressao:
			motivo = RecusaImpressaoMudou
		case !vagaAprovada(novo, c.VagaID):
			motivo = RecusaVagaFora
		}
		if motivo != "" {
			recusadas = append(recusadas, Recusa{ID: it.ID, Motivo: motivo})
			continue
		}
		depois, _, err := AprovarCandidatura(novo, it.ID, it.Impressao, agora, limites, ctx)
		if err != nil {
			var el *ErroLimite // o limite é o do aprovar de uma: tentativas + aprovadas pendentes, já com as do lote
			if errors.As(err, &el) {
				recusadas = append(recusadas, Recusa{ID: it.ID, Motivo: RecusaLimite})
				continue
			}
			return e, nil, nil, err
		}
		novo = depois
		aprovadas = append(aprovadas, it.ID)
	}
	return novo, aprovadas, recusadas, nil
}
