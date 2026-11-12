package triagem

import (
	"context"
	"maps"
	"regexp"
	"strings"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/ia"
)

// AvaliarRadar avalia o anúncio que passou pela qualificação local do Radar.
func AvaliarRadar(ctx context.Context, motor *ia.IA, perfil, vaga dominio.Objeto) (Resultado, error) {
	r := filtrosDoRadar(perfil, vaga)
	if r.Estado == "pulada" {
		return r, nil
	}
	_, minimo, maximo := ofertaMensal(vaga)
	if minimo > 0 || maximo > 0 {
		regras := lista(dominio.ObjetoDe(perfil["filtros"])["pularSe"])
		salario := compararSalario(perfil, vaga, regras)
		if salario.Estado == "pulada" {
			return salario, nil
		}
		r.Conferir = append(r.Conferir, salario.Conferir...)
	}
	dados := entrada(perfil, vaga, r.Conferir)
	criterios := dominio.ObjetoDe(dominio.ObjetoDe(dados["perfil"])["filtros"])
	if pais := texto(perfil["paisTrabalho"]); pais != "" {
		criterios["paisTrabalho"] = pais
	}
	if cidade := texto(perfil["cidadeTrabalho"]); cidade != "" && modelos[dominio.Normalizar(vaga["modelo"])] != "remoto" {
		criterios["cidadeTrabalho"] = cidade
	}
	if equivalentes := lista(perfil["titulosEquivalentes"]); len(equivalentes) > 0 {
		criterios["titulosEquivalentes"] = listaJSON(equivalentes)
	}
	anuncio := dominio.ObjetoDe(dados["vaga"])
	if paises := lista(vaga["paisesElegiveis"]); len(paises) > 0 {
		anuncio["paisesElegiveis"] = listaJSON(paises)
	}
	if vaga["remotoMundial"] == true {
		anuncio["remotoMundial"] = true
	}
	return avaliarComIA(ctx, motor, r, dados)
}

var nomeIntermediario = regexp.MustCompile(`\b(?:staffing(?: agency| solutions)?|recruit(?:ment|ing) agency|headhunt(?:ers?|ing)|agencia de (?:empregos?|recrutamento)|(?:consultoria de )?recrutamento e selecao)\b`)
var anuncioIntermediado = regexp.MustCompile(`\b(?:recrut(?:amos|ando)|contrat(?:amos|ando)|selecion(?:amos|ando)) para (?:um |uma |noss[oa] )?cliente\b|\b(?:recruiting|hiring|recruit) (?:on behalf of|for) (?:a|our) client\b|\bon behalf of our client\b`)
var nivelNoRadar = regexp.MustCompile(`\b(?:estagio|estagiario|internship|trainee|junior|jr|pleno|mid[- ]level|senior|sr|staff|principal|tech lead)\b`)

func niveisDoCargo(cargo any) []string {
	titulo := dominio.Normalizar(cargo)
	if intervaloDeNivel.MatchString(titulo) {
		return []string{"junior", "pleno", "senior"}
	}
	return nivelNoRadar.FindAllString(titulo, -1)
}

func filtrosDoRadar(perfil, vaga dominio.Objeto) Resultado {
	filtros := dominio.ObjetoDe(perfil["filtros"])
	if len(lista(filtros["niveis"])) == 0 {
		if ns := niveisDoCargo(perfil["cargo"]); len(ns) > 0 {
			filtros = maps.Clone(filtros)
			if filtros == nil {
				filtros = dominio.Objeto{}
			}
			filtros["niveis"] = ns
			perfil = maps.Clone(perfil)
			perfil["filtros"] = filtros
		}
	}
	if texto(vaga["nivel"]) == "" {
		if ns := niveisDoCargo(vaga["titulo"]); len(ns) > 0 {
			vaga = maps.Clone(vaga)
			vaga["nivel"] = strings.Join(ns, "/")
		}
	}
	return filtrosDoAlvo(perfil, vaga)
}

func dataRadar(s string, local *time.Location) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	t, err := time.ParseInLocation(time.DateOnly, s, local)
	return t, err == nil
}

func disponibilidadeRadar(vaga dominio.Objeto, agora time.Time) (string, string) {
	if vaga["etapa"] == "encerrada" || vaga["triagemBusca"] == "expirada" || vaga["aberta"] == false {
		return "excluir", "A vaga está encerrada."
	}
	incerta := ""
	if s := texto(vaga["expiraEm"]); s != "" {
		fim, ok := dataRadar(s, agora.Location())
		if len(s) == len(time.DateOnly) {
			fim = fim.AddDate(0, 0, 1)
		} // prazo sem hora vale até o fim do dia.
		if ok && !fim.After(agora) {
			return "excluir", "A vaga encerrou em " + s + "."
		}
		if !ok {
			incerta = "Confirme o prazo de inscrição informado no anúncio."
		}
	}
	s := texto(vaga["publicadaEm"])
	data, ok := dataRadar(s, agora.Location())
	if !ok || data.After(agora) {
		return "conferir", "A data original de publicação precisa ser confirmada."
	}
	limite := agora.AddDate(0, 0, -30)
	if len(s) == len(time.DateOnly) {
		limite, _ = time.ParseInLocation(time.DateOnly, limite.Format(time.DateOnly), agora.Location())
	}
	if data.Before(limite) {
		return "excluir", "O anúncio publicado em " + s + " tem mais de 30 dias."
	}
	if incerta != "" {
		return "conferir", incerta
	}
	return "forte", ""
}

// QualificarRadar confronta evidências públicas com o alvo ativo, antes da avaliação pela IA.
func QualificarRadar(perfil, vaga dominio.Objeto, agora time.Time) (classe string, motivo string) {
	if nomeIntermediario.MatchString(dominio.Normalizar(vaga["empresa"])) || anuncioIntermediado.MatchString(dominio.Normalizar(vaga["descricao"])) {
		return "excluir", "O anúncio identifica um intermediário recrutando para outro empregador."
	}
	filtros := filtrosDoRadar(perfil, vaga)
	if filtros.Estado == "pulada" {
		return "excluir", filtros.Motivo
	}
	regras := lista(dominio.ObjetoDe(perfil["filtros"])["pularSe"])
	if salario := compararSalario(perfil, vaga, regras); salario.Estado == "pulada" {
		return "excluir", salario.Motivo
	}
	prazo, motivoPrazo := disponibilidadeRadar(vaga, agora)
	local, motivoLocal := geografiaRadar(perfil, vaga)
	funcao, motivoFuncao := funcaoRadar(perfil, vaga)
	for _, criterio := range []struct{ classe, motivo string }{{prazo, motivoPrazo}, {local, motivoLocal}, {funcao, motivoFuncao}} {
		if criterio.classe == "excluir" {
			return "excluir", criterio.motivo
		}
		if criterio.classe == "conferir" {
			filtros.Conferir = append(filtros.Conferir, criterio.motivo)
		}
	}
	if len(filtros.Conferir) > 0 {
		return "conferir", strings.Join(filtros.Conferir, " ")
	}
	return "forte", motivoFuncao
}
