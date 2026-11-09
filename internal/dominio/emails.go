package dominio

import (
	"regexp"
	"sort"
	"strings"
	"time"
)

// Emails de processos seletivos: pré-filtro, qual vaga é e o que muda nela (porte de nucleo/emails.mjs).
// Puro: recebe dados, devolve decisões. Quem grava é o serviço.

var TiposEmail = []string{"confirmacao", "rejeicao", "entrevista", "teste", "oferta", "outro"}

var NomeEmail = map[string]string{"confirmacao": "Candidatura recebida", "rejeicao": "Recusa", "entrevista": "Convite para entrevista", "teste": "Teste", "oferta": "Oferta", "outro": "Email"}

// Remetentes de ATS/testes e palavras de processo seletivo. O que não passa aqui nem chega à IA.
var (
	reATS      = regexp.MustCompile(`(?i)greenhouse|lever\.co|ashbyhq|gupy|inhire|workable|smartrecruiters|jobs-noreply@linkedin|myworkday|workday|icims|teamtailor|recruitee|bamboohr|breezy|jobvite|taleo|successfactors|kenoby|solides|pandape|hackerrank|codility|coderbyte|testgorilla|codesignal`)
	rePalavras = regexp.MustCompile(`(?i)\bapplication\b|\bapplying\b|candidatura|unfortunately|infelizmente|next steps?|pr[oó]ximas? etapas?|pr[oó]ximos passos|entrevista|interview|assessment|coding (test|challenge)|teste t[eé]cnico|desafio t[eé]cnico|job offer|offer letter|proposta de (trabalho|emprego)|processo seletivo`)
)

// PassaPreFiltro diz se o email parece de processo seletivo.
func PassaPreFiltro(email Objeto) bool {
	return reATS.MatchString(texto(email["de"])) || rePalavras.MatchString(texto(email["assunto"])+"\n"+texto(email["texto"]))
}

var (
	reNaoAlfaNum       = regexp.MustCompile(`[^a-z0-9]+`)
	reNaoAlfaNumEspaco = regexp.MustCompile(`[^a-z0-9 ]+`)
	reDominio          = regexp.MustCompile(`(?i)@([a-z0-9.-]+)`)
	reEspacos          = regexp.MustCompile(`\s+`)
)

func espacado(v any) string {
	return " " + aparar(reNaoAlfaNum.ReplaceAllString(Normalizar(v), " ")) + " "
}

var etapasComEmail = []string{"aplicar", "enviada", "bloqueada", "em-processo", "oferta", "encerrada"}

// Quanto o email "fala" desta vaga: empresa no domínio, no nome do remetente, no assunto ou no texto; título desempata.
func pontosEmail(vaga, email Objeto) int {
	chave := ChaveEmpresa(vaga["empresa"])
	primeira := "" // primeira palavra do nome da empresa
	if f := strings.Fields(reNaoAlfaNumEspaco.ReplaceAllString(Normalizar(vaga["empresa"]), " ")); len(f) > 0 {
		primeira = f[0]
	}
	// "Maria <maria@nortal.com.br>" → [nortal com br]
	dominio := ""
	if m := reDominio.FindStringSubmatch(texto(email["de"])); m != nil {
		dominio = m[1]
	}
	var rotulos []string
	for _, r := range strings.Split(Normalizar(dominio), ".") {
		if r != "" {
			rotulos = append(rotulos, r)
		}
	}
	t := espacado(texto(email["de"]) + " " + texto(email["assunto"]) + " " + texto(email["texto"]))
	colado := reEspacos.ReplaceAllString(t, "")
	temEmpresa := false
	if len(chave) >= 4 {
		if strings.Contains(colado, chave) {
			temEmpresa = true
		}
		for _, r := range rotulos {
			if strings.Contains(r, chave) {
				temEmpresa = true
			}
		}
	}
	if !temEmpresa && tamanhoJS(primeira) >= 3 {
		if strings.Contains(t, " "+primeira+" ") {
			temEmpresa = true
		}
		for _, r := range rotulos {
			if r == primeira {
				temEmpresa = true
			}
		}
	}
	if !temEmpresa {
		return 0
	}
	pontos := 10
	for _, p := range strings.Split(aparar(espacado(vaga["titulo"])), " ") {
		if tamanhoJS(p) >= 4 && strings.Contains(t, " "+p+" ") {
			pontos++
		}
	}
	return pontos
}

// AcharVagaDoEmail: a vaga indicada na classificação (se existir), senão a única com mais pontos. Empate ou nada = nil.
func AcharVagaDoEmail(vagas []Objeto, email Objeto, vagaID any) Objeto {
	if verdadeiro(vagaID) {
		for _, v := range vagas {
			if igual(v["id"], vagaID) {
				return v
			}
		}
	}
	type candidata struct {
		vaga   Objeto
		pontos int
	}
	var cs []candidata
	for _, v := range vagas {
		if !contem(etapasComEmail, v["etapa"]) && !FoiEnviada(v) {
			continue
		}
		if p := pontosEmail(v, email); p > 0 {
			cs = append(cs, candidata{v, p})
		}
	}
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].pontos > cs[j].pontos })
	if len(cs) == 0 || (len(cs) > 1 && cs[1].pontos == cs[0].pontos) {
		return nil
	}
	return cs[0].vaga
}

// Transicao é a mudança de etapa que um email provoca.
type Transicao struct {
	Etapa  string `json:"etapa"`
	Extras Objeto `json:"extras"`
}

var reTecnica = regexp.MustCompile(`(?i)t[eé]cnic|technical|coding|pair|system design|live code`)

// TransicaoDoEmail: o que muda na vaga. Nunca anda para trás e não mexe em vaga encerrada ou pulada.
func TransicaoDoEmail(tipo string, vaga, email Objeto) *Transicao {
	if igual(vaga["etapa"], "encerrada") || igual(vaga["etapa"], "pulada") {
		return nil
	}
	tecnica := reTecnica.MatchString(texto(email["assunto"]) + " " + texto(email["resumo"]))
	// O próximo passo do email vira a pendência; sem ele, um texto padrão.
	passo := func(padrao string) string {
		if p := cortarJS(aparar(texto(email["proximoPasso"])), 140); p != "" {
			return p
		}
		return padrao
	}
	switch tipo {
	case "rejeicao":
		return &Transicao{"encerrada", Objeto{"motivo": "Reprovada"}}
	case "oferta":
		return &Transicao{"oferta", Objeto{"subEtapa": "proposta"}}
	case "entrevista":
		if igual(vaga["etapa"], "oferta") {
			return nil
		}
		sub := "entrevista-rh"
		if tecnica {
			sub = "entrevista-tecnica"
		}
		return &Transicao{"em-processo", Objeto{"subEtapa": sub, "pendencia": passo("Agendar a entrevista")}}
	case "teste":
		if igual(vaga["etapa"], "oferta") {
			return nil
		}
		return &Transicao{"em-processo", Objeto{"subEtapa": "teste", "pendencia": passo("Fazer o teste")}}
	}
	return nil // confirmação e outro só ficam registrados
}

var reInicioISO = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}`)

var formatosEmail = []string{
	time.RFC1123Z, time.RFC1123, "Mon, 2 Jan 2006 15:04:05 -0700", "Mon, 2 Jan 2006 15:04:05 MST",
	"2 Jan 2006 15:04:05 -0700", "02 Jan 2006 15:04:05 -0700", time.RFC822Z, time.RFC822, time.RFC3339,
}

// DataDoEmail devolve a data do email como AAAA-MM-DD (aceita ISO, RFC 2822 ou vazio).
func DataDoEmail(valor any, padrao string) string {
	s := texto(valor)
	if reInicioISO.MatchString(s) {
		return s[:10]
	}
	if !verdadeiro(valor) {
		return padrao
	}
	if f, ok := valor.(float64); ok {
		return HojeISO(time.UnixMilli(int64(f)))
	}
	limpo := aparar(s)
	if i := strings.Index(limpo, " ("); i > 0 { // "... +0000 (UTC)"
		limpo = limpo[:i]
	}
	for _, f := range formatosEmail {
		if t, err := time.Parse(f, limpo); err == nil {
			return HojeISO(t)
		}
	}
	return padrao
}
