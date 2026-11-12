package api

import (
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"strings"
)

func (a *API) iniciarManualRadar(e dominio.Estado, id string, cx dominio.Ctx) (dominio.Estado, Objeto, error) {
	// Uma candidatura que já existe mantém seu pacote, inclusive se o PDF foi movido depois.
	for _, c := range e.Candidaturas {
		if c["vagaId"] == id && c["estado"] != dominio.CandCancelada {
			return dominio.IniciarManualRadar(e, id, nil, a.relogio(), cx)
		}
	}
	var vaga Objeto
	for _, v := range e.Vagas {
		if v["id"] == id {
			vaga = v
			break
		}
	}
	if vaga == nil {
		return e, nil, falha(404, "vaga não encontrada")
	}
	idioma := "pt"
	if vaga["idioma"] == "en" {
		idioma = "en"
	}
	cvs := dominio.ObjetoDe(e.Perfil["curriculos"])
	nome := dominio.Texto(cvs[strings.ToUpper(idioma)])
	if nome == "" {
		nome = dominio.Texto(cvs["PT"])
	}
	nome = strings.TrimPrefix(nome, "curriculos/")
	if !dominio.NomeDeCurriculoValido(nome) {
		return e, nil, erroCampo("selecione um currículo em Perfil antes de aplicar")
	}
	pacote, err := a.prepararConteudo(Objeto{"curriculo": Objeto{"arquivo": nome}, "idioma": idioma, "respostas": []any{}})
	if err != nil {
		return e, nil, err
	}
	return dominio.IniciarManualRadar(e, id, pacote, a.relogio(), cx)
}
