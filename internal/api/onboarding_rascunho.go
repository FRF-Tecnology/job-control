package api

import (
	"context"
	"errors"
	"math"
	"slices"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

var camposRascunho = []string{"cargo", "niveis", "modelos", "foco", "principal", "secundaria", "pular", "ingles", "moeda", "minimo", "maximo"}

func revisaoDoRascunho(c Objeto) (int64, error) {
	v, ok := c["revisao"].(float64)
	if !ok || v < 0 || v > 9_007_199_254_740_991 || math.Trunc(v) != v {
		return 0, erroCampo("informe a revisão do rascunho")
	}
	return int64(v), nil
}
func referenciaDoPedido(ctx context.Context, a *API, c Objeto) (*armazem.ReferenciaRascunho, error) {
	id, tem := c["arquivoId"].(string)
	if !tem {
		return nil, nil
	}
	if _, ok := ctx.Value(chaveProvaDoPerfil{}).(uint64); !ok {
		return nil, falha(412, "verifique a conexão de IA antes de usar o currículo salvo")
	}
	if err := a.verificarProvaDoPerfil(ctx); err != nil {
		return nil, err
	}
	rev, err := revisaoDoRascunho(c)
	if err != nil {
		return nil, err
	}
	return &armazem.ReferenciaRascunho{ArquivoID: id, Revisao: rev}, nil
}
func erroDoRascunho(err error) error {
	if errors.Is(err, armazem.ErrRevisaoRascunho) {
		return falha(409, armazem.ErrRevisaoRascunho.Error())
	}
	if errors.Is(err, armazem.ErrOnboardingConcluido) {
		return falha(409, armazem.ErrOnboardingConcluido.Error())
	}
	return err
}

// Uma troca de PDF ou revisão durante a extração invalida as sugestões anteriores.
func (a *API) conferirReferenciaAtual(ctx context.Context, ref *armazem.ReferenciaRascunho) error {
	if ref == nil {
		return nil
	}
	d, err := a.arm.LerRascunho(ctx)
	if err != nil {
		return erroDoRascunho(err)
	}
	if d.Curriculo == nil || d.Curriculo.ArquivoID != ref.ArquivoID || d.Revisao != ref.Revisao {
		return erroDoRascunho(armazem.ErrRevisaoRascunho)
	}
	return nil
}

func atualizarCamposRascunho(r *armazem.Rascunho, c Objeto) error {
	if err := soCampos(c, "revisao", "dados", "editados", "etapa", "tentativaExterna"); err != nil {
		return err
	}
	if etapa, ok := c["etapa"]; ok {
		n, ok := etapa.(float64)
		if !ok || n != 1 && n != 2 {
			return erroCampo("etapa desconhecida")
		}
		if n == 2 && r.Curriculo == nil {
			return erroCampo("selecione o PDF para avançar")
		}
		r.Etapa = int(n)
	}
	if tentativa, ok := c["tentativaExterna"]; ok {
		tipo, ok := tentativa.(string)
		if !ok || !slices.Contains([]string{"", "instalar-claude", "login-claude"}, tipo) {
			return erroCampo("tentativa externa desconhecida")
		}
		r.TentativaExterna = tipo
	}
	if raw, ok := c["dados"]; ok {
		dados, ok := raw.(map[string]any)
		if !ok {
			return erroCampo("dados deve ser um objeto")
		}
		if err := soCampos(dados, camposRascunho...); err != nil {
			return err
		}
		if dominio.ContemDocumentoEm(dados) {
			return erroCampo("não inclua documentos nos critérios de busca")
		}
		for k, v := range dados {
			if k == "niveis" || k == "modelos" {
				if !listaOnboarding(v, false) || len(dominio.ListaDe(v)) > 30 {
					return erroCampo("escolhas inválidas para " + k)
				}
				for _, s := range dominio.ListaDe(v) {
					if len(s.(string)) > 150 {
						return erroCampo("opção extensa demais")
					}
				}
			} else if s, ok := v.(string); !ok || len(s) > 4096 {
				return erroCampo("texto inválido para " + k)
			}
			r.Dados[k] = v
		}
	}
	if raw, ok := c["editados"]; ok {
		if !listaOnboarding(raw, false) {
			return erroCampo("editados deve listar os campos")
		}
		editados := []string{}
		for _, v := range dominio.ListaDe(raw) {
			campo := v.(string)
			if !slices.Contains(camposRascunho, campo) {
				return erroCampo("campo editado desconhecido")
			}
			if !slices.Contains(editados, campo) {
				editados = append(editados, campo)
			}
		}
		r.Editados = editados
	}
	return nil
}

func (a *API) rotaRascunho(ctx context.Context, m string, c Objeto) (Resposta, error) {
	if m == "GET" {
		d, err := a.arm.LerRascunho(ctx)
		if err != nil {
			return Resposta{}, erroDoRascunho(err)
		}
		return ok(d)
	}
	rev, err := revisaoDoRascunho(c)
	if err != nil {
		return Resposta{}, err
	}
	var d armazem.Rascunho
	switch m {
	case "PATCH":
		d, err = a.arm.EditarRascunho(ctx, rev, func(d *armazem.Rascunho) error { return atualizarCamposRascunho(d, c) })
	case "DELETE":
		if err := soCampos(c, "revisao"); err != nil {
			return Resposta{}, err
		}
		d, err = a.arm.TrocarPDFRascunho(ctx, rev, "", nil)
	case "PUT":
		if err := soCampos(c, "revisao", "curriculo", "nomeArquivo"); err != nil {
			return Resposta{}, err
		}
		pdf, err := pdfDoOnboarding(c)
		if err != nil {
			return Resposta{}, err
		}
		nome := nomeDeExibicao(dominio.Texto(c["nomeArquivo"]))
		if nome == "" {
			return Resposta{}, erroCampo("selecione um arquivo PDF")
		}
		if dominio.ContemDocumento(nome) {
			nome = "Currículo.pdf"
		}
		d, err = a.arm.TrocarPDFRascunho(ctx, rev, nome, pdf)
		if err != nil {
			return Resposta{}, erroDoRascunho(err)
		}
		return ok(Objeto{"revisao": d.Revisao, "arquivoId": d.Curriculo.ArquivoID, "nomeExibicao": nome, "tamanho": d.Curriculo.Tamanho})
	}
	if err != nil {
		return Resposta{}, erroDoRascunho(err)
	}
	return ok(d)
}
