package dominio

import (
	"math"
	"sort"
	"strings"
	"time"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// Números do funil (porte de nucleo/funil.mjs).

// TeveRetorno: a empresa respondeu de alguma forma (andou no processo ou reprovou).
func TeveRetorno(v Objeto) bool {
	if !FoiEnviada(v) {
		return false
	}
	return igual(v["etapa"], "em-processo") || igual(v["etapa"], "oferta") || (igual(v["etapa"], "encerrada") && igual(v["motivo"], "Reprovada"))
}

func segundaFeira(iso string) string {
	p := strings.Split(iso, "-")
	if len(p) != 3 {
		return "NaN-NaN-NaN"
	}
	t, err := time.ParseInLocation("2006-01-02", iso, time.Local)
	if err != nil {
		return SomarDias(iso, 0)
	}
	dia := int(t.Weekday()) // 0 = domingo
	return SomarDias(iso, -((dia + 6) % 7))
}

// Resumo são os números do dia.
type Resumo struct {
	Total          int `json:"total"`
	Descoberta     int `json:"descoberta"`
	Aplicar        int `json:"aplicar"`
	Bloqueadas     int `json:"bloqueadas"`
	EmProcesso     int `json:"emProcesso"`
	Ofertas        int `json:"ofertas"`
	Enviadas       int `json:"enviadas"`
	EnviadasHoje   int `json:"enviadasHoje"`
	EnviadasSemana int `json:"enviadasSemana"`
	Retornos       int `json:"retornos"`
	TaxaRetorno    int `json:"taxaRetorno"`
	FollowUps      int `json:"followUps"`
}

func dataEnvio(v Objeto) string { return texto(obj(v["envio"])["data"]) }

// CalcularResumo conta vagas por etapa, envios (hoje e na semana) e a taxa de retorno.
func CalcularResumo(vagas []Objeto, hoje string) Resumo {
	inicio := segundaFeira(hoje)
	r := Resumo{Total: len(vagas), FollowUps: len(PendentesFollowUp(vagas, hoje))}
	for _, v := range vagas {
		switch v["etapa"] {
		case "descoberta":
			r.Descoberta++
		case "aplicar":
			r.Aplicar++
		case "bloqueada":
			r.Bloqueadas++
		case "em-processo":
			r.EmProcesso++
		case "oferta":
			r.Ofertas++
		}
		if !FoiEnviada(v) {
			continue
		}
		r.Enviadas++
		if dataEnvio(v) == hoje {
			r.EnviadasHoje++
		}
		if dataEnvio(v) >= inicio {
			r.EnviadasSemana++
		}
		if TeveRetorno(v) {
			r.Retornos++
		}
	}
	if r.Enviadas > 0 {
		r.TaxaRetorno = int(math.Floor(float64(r.Retornos)/float64(r.Enviadas)*100 + 0.5))
	}
	return r
}

// EtapaFunil é uma barra do funil.
type EtapaFunil struct {
	Etapa string `json:"etapa"`
	Total int    `json:"total"`
}

// Funil começa nas enviadas.
func Funil(vagas []Objeto) []EtapaFunil {
	var env, ret, proc, of int
	for _, v := range vagas {
		if !FoiEnviada(v) {
			continue
		}
		env++
		if TeveRetorno(v) {
			ret++
		}
		if igual(v["etapa"], "em-processo") {
			proc++
		}
		if igual(v["etapa"], "oferta") {
			of++
		}
	}
	return []EtapaFunil{{"Enviadas", env}, {"Com retorno", ret}, {"Em processo", proc}, {"Oferta", of}}
}

// Plataforma resume achadas, enviadas e retornos de uma plataforma.
type Plataforma struct {
	Plataforma any `json:"plataforma"`
	Achadas    int `json:"achadas"`
	Enviadas   int `json:"enviadas"`
	Retornos   int `json:"retornos"`
}

// PorPlataforma agrupa por plataforma (sem plataforma = "outra"), mais enviadas primeiro.
func PorPlataforma(vagas []Objeto) []Plataforma {
	var grupos []*Plataforma
	for _, v := range vagas {
		p := ouV(v["plataforma"], "outra")
		var g *Plataforma
		for _, x := range grupos {
			if igual(x.Plataforma, p) {
				g = x
			}
		}
		if g == nil {
			g = &Plataforma{Plataforma: p}
			grupos = append(grupos, g)
		}
		g.Achadas++
		if FoiEnviada(v) {
			g.Enviadas++
		}
		if TeveRetorno(v) {
			g.Retornos++
		}
	}
	saida := make([]Plataforma, len(grupos))
	for i, g := range grupos {
		saida[i] = *g
	}
	sort.SliceStable(saida, func(i, j int) bool {
		if saida[i].Enviadas != saida[j].Enviadas {
			return saida[i].Enviadas > saida[j].Enviadas
		}
		return saida[i].Achadas > saida[j].Achadas
	})
	return saida
}

// Semana é o total de envios de uma semana (começa na segunda-feira).
type Semana struct {
	Semana string `json:"semana"`
	Total  int    `json:"total"`
}

// PorSemana agrupa os envios pela segunda-feira da semana.
func PorSemana(vagas []Objeto) []Semana {
	contagem := map[string]int{}
	var ordem []string
	for _, v := range vagas {
		if !FoiEnviada(v) {
			continue
		}
		s := segundaFeira(dataEnvio(v))
		if _, ok := contagem[s]; !ok {
			ordem = append(ordem, s)
		}
		contagem[s]++
	}
	sort.Strings(ordem)
	saida := []Semana{}
	for _, s := range ordem {
		saida = append(saida, Semana{s, contagem[s]})
	}
	return saida
}

// MotivoPulo é quantas vagas foram puladas por um motivo.
type MotivoPulo struct {
	Motivo string `json:"motivo"`
	Total  int    `json:"total"`
}

// Ordem alfabética como a do localeCompare (acentos não pesam mais que a letra).
var ordemAlfabetica = collate.New(language.BrazilianPortuguese)

// MotivosDePulo agrupa as vagas puladas pelo motivo, mais frequentes primeiro.
func MotivosDePulo(vagas []Objeto) []MotivoPulo {
	contagem := map[string]int{}
	var ordem []string
	for _, v := range vagas {
		if !igual(v["etapa"], "pulada") {
			continue
		}
		m := texto(ouV(v["motivo"], "Outro"))
		if _, ok := contagem[m]; !ok {
			ordem = append(ordem, m)
		}
		contagem[m]++
	}
	saida := []MotivoPulo{}
	for _, m := range ordem {
		saida = append(saida, MotivoPulo{m, contagem[m]})
	}
	sort.SliceStable(saida, func(i, j int) bool {
		if saida[i].Total != saida[j].Total {
			return saida[i].Total > saida[j].Total
		}
		return ordemAlfabetica.CompareString(saida[i].Motivo, saida[j].Motivo) < 0
	})
	return saida
}
