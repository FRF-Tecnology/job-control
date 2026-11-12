package api

import (
	"context"
	"errors"
	"path"
	"strings"
	"unicode"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Só o basename serve de rótulo no perfil; a escrita usa o identificador gerado.
func nomeDeExibicao(s string) string {
	n := path.Base(strings.ReplaceAll(s, "\\", "/"))
	n = strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, n))
	if len(n) > 255 || !strings.HasSuffix(strings.ToLower(n), ".pdf") {
		return ""
	}
	return n
}

func (a *API) substituirCurriculo(ctx context.Context, perfilID string, corpo Objeto) (Resposta, error) {
	if perfilID == "" {
		return Resposta{}, erroCampo("selecione um perfil de busca")
	}
	pdf, err := pdfDoOnboarding(corpo)
	if err != nil {
		return Resposta{}, err
	}
	var arquivo string
	err = a.arm.SubstituirCurriculo(ctx, pdf, func(e dominio.Estado, nome string) (dominio.Objeto, error) {
		if err := conferirPerfilBusca(e, perfilID); err != nil {
			return nil, err
		}
		cvs := Objeto{}
		for k, v := range dominio.ObjetoDe(e.Perfil["curriculos"]) {
			cvs[k] = v
		}
		arquivo = "curriculos/" + nome
		cvs["PT"] = arquivo
		p := MesclarPerfil(e.Perfil, Objeto{"curriculos": cvs})
		p["curriculoExibicao"] = Objeto{"arquivo": arquivo, "nome": corpo["nomeArquivo"]}
		return p, exigirPerfilOk(p)
	})
	if errors.Is(err, armazem.ErrOnboardingIncompleto) {
		return Resposta{}, falha(409, err.Error())
	}
	if err != nil {
		return Resposta{}, err
	}
	return ok(Objeto{"arquivo": arquivo})
}
