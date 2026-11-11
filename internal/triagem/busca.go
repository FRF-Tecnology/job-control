// Package triagem confronta os dados da busca com o perfil e pede uma opinião à IA da pessoa.
// O resultado não muda a etapa da vaga e não aprova uma candidatura.
package triagem

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/ia"
)

type Resultado struct {
	Estado     string   `json:"estado"`
	Aderencia  string   `json:"aderencia,omitempty"`
	Decisao    string   `json:"decisao,omitempty"`
	Motivo     string   `json:"motivo"`
	MotivoPulo string   `json:"motivoPulo,omitempty"`
	Conferir   []string `json:"conferir,omitempty"`
}

func texto(v any) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

func lista(v any) []string {
	if l, ok := v.([]string); ok {
		return l
	}
	var r []string
	for _, item := range dominio.ListaDe(v) {
		if s := texto(item); s != "" {
			r = append(r, s)
		}
	}
	return r
}

// O perfil pode vir do SQLite ([]any) ou de código Go ([]string). O prompt e a
// verificação de documentos usam a representação JSON do domínio.
func listaJSON(itens []string) []any {
	r := make([]any, len(itens))
	for i, s := range itens {
		r[i] = s
	}
	return r
}

var niveis = map[string]string{
	"estagio": "estagio", "intern": "estagio", "internship": "estagio", "estagiario": "estagio", "trainee": "trainee",
	"junior": "junior", "jr": "junior", "entry level": "junior", "entry-level": "junior",
	"pleno": "pleno", "mid": "pleno", "mid-level": "pleno", "mid level": "pleno", "middle": "pleno",
	"senior": "senior", "sr": "senior", "staff": "staff", "lead": "lead", "tech lead": "lead", "principal": "principal",
	"especialista": "especialista", "lideranca": "lideranca",
}
var modelos = map[string]string{
	"remoto": "remoto", "remota": "remoto", "remote": "remoto", "fully remote": "remoto", "100% remoto": "remoto", "home office": "remoto",
	"hibrido": "hibrido", "hibrida": "hibrido", "hybrid": "hibrido",
	"presencial": "presencial", "on-site": "presencial", "onsite": "presencial", "on site": "presencial",
}
var alternativas = regexp.MustCompile(`\s*(?:[/,;|]|\s+(?:ou|or|e|and|a|to)\s+)\s*`)
var nivelNoTitulo = regexp.MustCompile(`\b(?:estagio|estagiario|internship|trainee|junior|jr|pleno|mid[- ]level|senior|sr)\b`)
var intervaloDeNivel = regexp.MustCompile(`\b(?:junior|jr)\.?\s*(?:-|–|a|ate|to)\s*(?:senior|sr)\b`)

// Uma expressão desconhecida não vira prova de incompatibilidade.
func opcoes(valores []string, nomes map[string]string) ([]string, bool) {
	var r []string
	for _, valor := range valores {
		for _, parte := range alternativas.Split(dominio.Normalizar(valor), -1) {
			nome, ok := nomes[strings.Trim(strings.TrimSpace(parte), ".")]
			if !ok {
				return nil, false
			}
			if !slices.Contains(r, nome) {
				r = append(r, nome)
			}
		}
	}
	return r, len(r) > 0
}

func compativel(a, b []string) bool {
	return slices.ContainsFunc(a, func(s string) bool { return slices.Contains(b, s) })
}

func exclusao(regras, oferecidas []string, nomes map[string]string) bool {
	var excluidas []string
	for _, regra := range regras {
		if itens, ok := opcoes([]string{regra}, nomes); ok {
			excluidas = append(excluidas, itens...)
		}
	}
	return len(oferecidas) > 0 && !slices.ContainsFunc(oferecidas, func(s string) bool { return !slices.Contains(excluidas, s) })
}

func pular(motivo, categoria string) Resultado {
	return Resultado{Estado: "pulada", Decisao: "pular", Motivo: motivo, MotivoPulo: categoria}
}

func numero(v any) (float64, bool) {
	switch n := v.(type) {
	case string:
		if strings.TrimSpace(n) == "" {
			return 0, false
		}
	case float64, int:
	default:
		return 0, false
	}
	n, ok := dominio.NumeroDe(v)
	return n, ok && n >= 0 && !math.IsInf(n, 0)
}

func numeroPositivo(v any) float64 {
	n, ok := numero(v)
	if !ok {
		return 0
	}
	return n
}

func filtros(perfil, vaga dominio.Objeto) Resultado {
	f := dominio.ObjetoDe(perfil["filtros"])
	regras := lista(f["pularSe"])
	var conferir []string
	foco, mercado := dominio.Normalizar(perfil["foco"]), dominio.Normalizar(vaga["mercado"])
	if slices.Contains(dominio.Mercados, foco) {
		if !slices.Contains(dominio.Mercados, mercado) {
			conferir = append(conferir, "Mercado da vaga a confirmar.")
		} else if foco != mercado {
			return pular("Mercado "+mercado+" fora do foco selecionado.", "Outro")
		}
	}

	for _, filtro := range []struct {
		campo, preferencia, rotulo string
		nomes                      map[string]string
	}{
		{"nivel", "niveis", "Nível", niveis},
		{"modelo", "modelo", "Modelo de trabalho", modelos},
	} {
		raw := texto(vaga[filtro.campo])
		dados := []string{raw}
		if raw == "" && filtro.campo == "nivel" {
			dados = nivelNoTitulo.FindAllString(dominio.Normalizar(vaga["titulo"]), -1)
			raw = strings.Join(dados, "/")
		}
		oferta, conhecida := opcoes(dados, filtro.nomes)
		if filtro.campo == "nivel" && (intervaloDeNivel.MatchString(dominio.Normalizar(raw)) || texto(vaga["nivel"]) == "" && intervaloDeNivel.MatchString(dominio.Normalizar(vaga["titulo"]))) {
			oferta, conhecida = []string{"junior", "pleno", "senior"}, true
		}
		preferencias := lista(f[filtro.preferencia])
		aceitas, reconhecidas := opcoes(preferencias, filtro.nomes)
		categoria := "Outro"
		if filtro.campo == "modelo" {
			categoria = "Presencial/Local"
		} else if compativel(oferta, []string{"senior", "staff", "lead", "principal"}) {
			categoria = "Sênior"
		}
		if conhecida && exclusao(regras, oferta, filtro.nomes) {
			return pular(filtro.rotulo+" da vaga corresponde à exclusão do perfil: "+raw+".", categoria)
		}
		if conhecida && reconhecidas && !compativel(oferta, aceitas) {
			return pular(filtro.rotulo+" fora das opções aceitas: "+raw+".", categoria)
		}
		if len(preferencias) > 0 && (!conhecida || !reconhecidas) {
			conferir = append(conferir, filtro.rotulo+" a confirmar.")
		}
	}
	if motivo := exclusaoPersonalizada(regras, vaga); motivo != "" {
		return pular(motivo, "Outro")
	}

	salario := compararSalario(perfil, vaga, regras)
	if salario.Estado == "pulada" {
		return salario
	}
	conferir = append(conferir, salario.Conferir...)
	if texto(perfil["cargo"]) == "" {
		conferir = append(conferir, "Cargo desejado a confirmar.")
	}
	if texto(vaga["descricao"]) == "" {
		conferir = append(conferir, "Descrição da vaga não informada.")
	}
	if len(lista(perfil["stackPrincipal"])) > 0 && len(lista(vaga["stack"])) == 0 {
		conferir = append(conferir, "Confira os requisitos técnicos na descrição.")
	}
	if texto(perfil["ingles"]) != "" && texto(vaga["ingles"]) == "" {
		conferir = append(conferir, "Requisito de inglês a conferir na descrição.")
	}
	return Resultado{Conferir: conferir}
}

func ofertaMensal(vaga dominio.Objeto) (moeda string, minimo, maximo float64) {
	moeda = strings.ToUpper(texto(vaga["moeda"]))
	if moeda == "USD" || moeda == "" {
		minimo, maximo = numeroPositivo(vaga["salarioUSDMin"]), numeroPositivo(vaga["salarioUSDMax"])
		if fixo := numeroPositivo(vaga["salarioUSDMensal"]); fixo > 0 {
			return "USD", fixo, fixo
		}
		if minimo > 0 || maximo > 0 {
			return "USD", minimo, maximo
		}
	}
	minimo, maximo = numeroPositivo(vaga["salarioMin"]), numeroPositivo(vaga["salarioMax"])
	switch dominio.Normalizar(vaga["periodoSalario"]) {
	case "monthly", "mensal", "mes":
		return moeda, minimo, maximo
	case "annual", "yearly", "anual", "ano":
		return moeda, minimo / 12, maximo / 12
	}
	return moeda, 0, 0
}

func compararSalario(perfil, vaga dominio.Objeto, regras []string) Resultado {
	faixa := dominio.ObjetoDe(perfil["faixaSalarial"])
	moedaPretendida := texto(faixa["moeda"])
	piso := numeroPositivo(faixa["min"])
	if moedaPretendida == "" {
		if piso = numeroPositivo(perfil["pretensaoUSD"]); piso > 0 {
			moedaPretendida = "USD"
		}
	}
	moeda, minimo, maximo := ofertaMensal(vaga)
	pisoNaMoeda := piso
	if moeda != moedaPretendida {
		pisoNaMoeda = 0
	}
	excluirAbaixo := pisoDeExclusao(regras, moeda, pisoNaMoeda)
	var aviso string
	switch {
	case moeda != "" && maximo > 0 && minimo > maximo:
		aviso = "Faixa salarial mensal em " + moeda + " inconsistente na fonte."
	case maximo > 0 && maximo < excluirAbaixo:
		return pular(fmt.Sprintf("Teto de %s %.2f/mês abaixo do piso explícito de exclusão de %s %.2f/mês.", moeda, maximo, moeda, excluirAbaixo), "Outro")
	case maximo > 0 && maximo < pisoNaMoeda:
		aviso = fmt.Sprintf("Teto de %s %.2f/mês abaixo da pretensão de %s %.2f/mês.", moeda, maximo, moeda, pisoNaMoeda)
	case moedaPretendida != "" && (moeda != moedaPretendida || minimo == 0 && maximo == 0):
		aviso = "Confirmar se a oferta por mês atende à faixa em " + moedaPretendida + "."
	case minimo < max(pisoNaMoeda, excluirAbaixo) && maximo == 0:
		aviso = "Confirmar o teto da oferta mensal em " + moeda + "."
	}
	if aviso != "" {
		return Resultado{Conferir: []string{aviso}}
	}
	return Resultado{}
}

// Somente dados profissionais. Não entram contato, respostas pessoais, links, PDFs nem seus caminhos.
func entrada(perfil, vaga dominio.Objeto, conferir []string) dominio.Objeto {
	p, v := dominio.Objeto{}, dominio.Objeto{}
	for _, campo := range []string{"cargo", "ingles", "formacao"} {
		if s := texto(perfil[campo]); s != "" {
			p[campo] = s
		}
	}
	for _, campo := range []string{"experienciaAnos", "pretensaoUSD", "pretensaoCLT", "pretensaoPJ"} {
		if n, ok := numero(perfil[campo]); ok {
			p[campo] = n
		}
	}
	if faixa := dominio.ObjetoDe(perfil["faixaSalarial"]); faixa != nil {
		resumo := dominio.Objeto{"moeda": texto(faixa["moeda"])}
		for _, limite := range []string{"min", "max"} {
			if n := numeroPositivo(faixa[limite]); n > 0 {
				resumo[limite] = n
			}
		}
		p["faixaSalarial"] = resumo
		delete(p, "pretensaoUSD")
	}
	for _, campo := range []string{"stackPrincipal", "stackSecundaria", "contratos"} {
		if l := lista(perfil[campo]); len(l) > 0 {
			p[campo] = listaJSON(l)
		}
	}
	f := dominio.Objeto{}
	for _, campo := range []string{"niveis", "modelo", "pularSe"} {
		if l := lista(dominio.ObjetoDe(perfil["filtros"])[campo]); len(l) > 0 {
			f[campo] = listaJSON(l)
		}
	}
	if foco := texto(perfil["foco"]); foco != "" {
		f["foco"] = foco
	}
	p["filtros"] = f
	for _, campo := range []string{"id", "empresa", "titulo", "descricao", "nivel", "modelo", "mercado", "local", "contrato", "ingles", "moeda", "salario"} {
		if s := texto(vaga[campo]); s != "" {
			v[campo] = s
		}
	}
	for _, campo := range []string{"stack", "categorias"} {
		if l := lista(vaga[campo]); len(l) > 0 {
			v[campo] = listaJSON(l)
		}
	}
	if periodo := texto(vaga["periodoSalario"]); periodo != "" {
		v["periodoSalario"] = periodo
		for _, campo := range []string{"salarioMin", "salarioMax"} {
			if n := numeroPositivo(vaga[campo]); n > 0 {
				v[campo] = n
			}
		}
	}
	if moeda := texto(vaga["moeda"]); moeda == "" || strings.EqualFold(moeda, "USD") {
		for _, campo := range []string{"salarioUSDMensal", "salarioUSDMin", "salarioUSDMax"} {
			if n := numeroPositivo(vaga[campo]); n > 0 {
				v[campo] = n
			}
		}
	}
	if len(conferir) > 0 {
		v["conferir"] = conferir
	}
	return dominio.Objeto{"perfil": p, "vaga": v, "triagemBusca": true}
}

// Avaliar aplica apenas exclusões comprovadas localmente. Casos livres vão para triarVaga
// no provedor configurado, com validação do esquema e da guarda usuais.
func Avaliar(ctx context.Context, motor *ia.IA, perfil, vaga dominio.Objeto) (Resultado, error) {
	r := filtros(perfil, vaga)
	if r.Estado == "pulada" {
		return r, nil
	}
	r.Estado = "pendente"
	r.Motivo = "Triagem pendente. Tente novamente."
	if err := ctx.Err(); err != nil {
		return r, err
	}
	if motor == nil {
		return r, &ia.ErroIA{Tipo: "config", Mensagem: "nenhuma IA conectada: escolha uma em Conectar IA"}
	}
	dados := entrada(perfil, vaga, r.Conferir)
	if dominio.ContemDocumentoEm(dados) {
		return r, &ia.ErroIA{Tipo: "regra", Mensagem: "a vaga ou o perfil contém documento pessoal"}
	}
	saida, err := motor.Executar(ctx, "triarVaga", dados)
	if err != nil {
		return r, err
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	motivo := texto(saida["motivo"])
	if motivo == "" {
		return r, &ia.ErroIA{Tipo: "esquema", Mensagem: "a IA não explicou a classificação da vaga"}
	}
	r.Estado, r.Aderencia, r.Decisao = "classificada", texto(saida["aderencia"]), texto(saida["decisao"])
	r.Motivo, r.MotivoPulo = motivo, texto(saida["motivoPulo"])
	return r, nil
}
