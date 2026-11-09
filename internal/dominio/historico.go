package dominio

import (
	"fmt"
	"sort"
	"strings"
)

// Histórico em Markdown gerado a partir dos dados (porte de nucleo/historico.mjs).

func dataBR(iso any) string {
	if !verdadeiro(iso) {
		return ""
	}
	p := strings.Split(texto(iso), "-")
	for i, j := 0, len(p)-1; i < j; i, j = i+1, j-1 {
		p[i], p[j] = p[j], p[i]
	}
	return strings.Join(p, "/")
}

// numeroOuMenosUm imita Number(x) || -1.
func numeroOuMenosUm(x any) float64 {
	if f, ok := numeroDe(x); ok && f != 0 {
		return f
	}
	return -1
}

// GerarHistoricoMd gera o histórico de candidaturas em Markdown.
func GerarHistoricoMd(e Estado, hoje string) string {
	r := CalcularResumo(e.Vagas, hoje)
	linhas := []string{
		"# Histórico de candidaturas",
		"",
		"> Gerado pela Central em " + dataBR(hoje) + ". Não edite à mão: use a Central ou `node ferramentas/cli.mjs`.",
		fmt.Sprintf("> Enviadas: **%d** · com retorno: %d (%d%%) · precisa de você: %d · esperando triagem: %d", r.Enviadas, r.Retornos, r.TaxaRetorno, r.Bloqueadas, r.Descoberta),
		"",
	}
	type grupo struct {
		chave any
		vagas []Objeto
	}
	var grupos []*grupo
	for _, v := range e.Vagas {
		if !FoiEnviada(v) {
			continue
		}
		k := ou(obj(v["envio"])["rodada"], "sem rodada")
		var g *grupo
		for _, x := range grupos {
			if igual(x.chave, k) {
				g = x
			}
		}
		if g == nil {
			g = &grupo{chave: k}
			grupos = append(grupos, g)
		}
		g.vagas = append(g.vagas, v)
	}
	sort.SliceStable(grupos, func(i, j int) bool { return numeroOuMenosUm(grupos[i].chave) > numeroOuMenosUm(grupos[j].chave) })
	for _, g := range grupos {
		var info Objeto
		for _, x := range e.Rodadas {
			if jsString(x["n"]) == jsString(g.chave) {
				info = x
				break
			}
		}
		sort.SliceStable(g.vagas, func(i, j int) bool { return dataEnvio(g.vagas[i]) < dataEnvio(g.vagas[j]) })
		titulo := "## Rodada " + jsString(g.chave)
		if verdadeiro(info["data"]) {
			titulo += " (" + dataBR(info["data"]) + ")"
		}
		linhas = append(linhas, fmt.Sprintf("%s – %d enviadas", titulo, len(g.vagas)))
		if verdadeiro(info["resumo"]) {
			linhas = append(linhas, "_"+jsString(info["resumo"])+"_")
		}
		for _, v := range g.vagas {
			envio := obj(v["envio"])
			var extra []string
			for _, x := range []any{v["plataforma"], v["contrato"], pretensao(envio), etapaComMotivo(v)} {
				if verdadeiro(x) {
					extra = append(extra, jsString(x))
				}
			}
			respostas := ""
			if verdadeiro(envio["respostas"]) {
				respostas = " — " + jsString(envio["respostas"])
			}
			linhas = append(linhas, fmt.Sprintf("- **%s** – %s (%s; %s)%s", campo(v, "empresa"), campo(v, "titulo"), dataBR(envio["data"]), strings.Join(extra, " · "), respostas))
		}
		linhas = append(linhas, "")
	}
	var bloq, pul []Objeto
	for _, v := range e.Vagas {
		switch v["etapa"] {
		case "bloqueada":
			bloq = append(bloq, v)
		case "pulada":
			pul = append(pul, v)
		}
	}
	if len(bloq) > 0 {
		linhas = append(linhas, "## Precisa de você")
		for _, v := range bloq {
			link := ""
			if verdadeiro(v["link"]) {
				link = " — " + jsString(v["link"])
			}
			linhas = append(linhas, fmt.Sprintf("- **%s** – %s: %s%s", campo(v, "empresa"), campo(v, "titulo"), campo(v, "pendencia"), link))
		}
		linhas = append(linhas, "")
	}
	if len(pul) > 0 {
		linhas = append(linhas, "## Puladas")
		for _, v := range pul {
			linhas = append(linhas, fmt.Sprintf("- %s – %s: %s", campo(v, "empresa"), campo(v, "titulo"), campo(v, "motivo")))
		}
		linhas = append(linhas, "")
	}
	return strings.Join(linhas, "\n")
}

func pretensao(envio Objeto) any {
	if verdadeiro(envio["pretensao"]) {
		return "pretensão " + jsString(envio["pretensao"])
	}
	return ""
}

func etapaComMotivo(v Objeto) any {
	if igual(v["etapa"], "enviada") {
		return ""
	}
	nome, ok := NomeEtapa[texto(v["etapa"])]
	if !ok {
		nome = "undefined"
	}
	if verdadeiro(v["motivo"]) {
		return nome + ": " + jsString(v["motivo"])
	}
	return nome
}
