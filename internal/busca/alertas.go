package busca

import (
	"html"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Alertas de vaga por e-mail (LinkedIn e Indeed). A leitura do e-mail (POST /email/ler) entrega aqui o corpo de um
// alerta que a pessoa recebeu; daqui saem as vagas para o Radar, sem IA:
//   - o remetente é de uma lista fechada (FonteDoAlerta); a confirmação de candidatura do LinkedIn
//     (jobs-noreply@linkedin.com) e a do Indeed (indeedapply@indeed.com) não são alerta: são respostas;
//   - o id da vaga sai do link, e o link guardado é refeito a partir dele (linkedin.com/jobs/view/<id>,
//     indeed.com/viewjob?jk=<id>). O endereço do e-mail, com rastreio, nunca vai para a vaga;
//   - título, empresa e local são texto do e-mail (de terceiro): uma linha, com teto, sem link e sem <>.
//     Sem título ou sem empresa, a vaga fica de fora.

const maxVagasPorAlerta = 25

var (
	reAncora     = regexp.MustCompile(`(?is)<a\b[^>]*?\bhref\s*=\s*["']([^"']+)["'][^>]*>(.*?)</a\s*>`)
	reBlocoAl    = regexp.MustCompile(`(?is)<(script|style|head|title)[^>]*>.*?</(script|style|head|title)\s*>`)
	reQuebra     = regexp.MustCompile(`(?i)<br\s*/?>|</(p|div|td|th|tr|li|h[1-6]|table|span)\s*>`)
	reURLTexto   = regexp.MustCompile(`(?i)https?://\S+`)
	hostLinkedIn = regexp.MustCompile(`^(www\.|[a-z]{2}\.)?linkedin\.com$`)
	hostIndeedPa = regexp.MustCompile(`^[a-z]{2}\.indeed\.com$`)
	caminhoLI    = regexp.MustCompile(`^/(?:comm/)?jobs/view/(?:[^/]*-)?([0-9]{6,})/?$`)
	reJK         = regexp.MustCompile(`(?i)[?&](?:jk|vjk)=([0-9a-f]{16})\b`)
	// Linhas que não são título, empresa nem local (selos do LinkedIn e do Indeed, chamadas).
	reGenerica = regexp.MustCompile(`(?i)^(view job|ver vaga|see job|apply|apply now|candidatar(-se)?|candidate-se|easy apply|candidatura simplificada|promoted|promovida|actively recruiting|recrutando ativamente|new|nova|novo|be an early applicant|seja um dos primeiros|responds within .*|responde em .*|\d+ (applicants?|candidaturas?|candidatos?)|view all jobs|ver todas as vagas|see all jobs|unsubscribe|cancelar inscri.*)$`)
	rePrefixo  = regexp.MustCompile(`(?i)^(view job|ver vaga|see job|apply)\s*:?\s*`)
)

// FonteDoAlerta diz se o remetente é um alerta de vaga: "linkedin", "indeed" ou "" (não é alerta). O endereço vem
// sem o nome (caixa.SoEndereco). Compara o domínio inteiro, nunca "termina com".
func FonteDoAlerta(de string) string {
	de = strings.ToLower(strings.TrimSpace(de))
	i := strings.LastIndex(de, "@")
	if i <= 0 {
		return ""
	}
	local, dom := de[:i], de[i+1:]
	switch {
	case dom == "linkedin.com" && (local == "jobalerts-noreply" || local == "jobs-listings"):
		return "linkedin"
	case dom == "jobalert.indeed.com" || dom == "match.indeed.com":
		return "indeed"
	case dom == "indeed.com" && (strings.HasPrefix(local, "alert") || strings.HasPrefix(local, "jobalert")):
		return "indeed"
	}
	return ""
}

// idDaVaga devolve o id da vaga no link e o link refeito, ou "" quando o link não é de uma vaga da fonte.
func idDaVaga(fonte, href string) (id, link string) {
	bruto := strings.TrimSpace(html.UnescapeString(href))
	// Links de rastreio levam o endereço da vaga codificado na query: decodifica até duas vezes.
	candidatos := []string{bruto}
	for n, s := 0, bruto; n < 2; n++ {
		d, err := url.QueryUnescape(s)
		if err != nil || d == s {
			break
		}
		candidatos, s = append(candidatos, d), d
	}
	for _, c := range candidatos {
		u, err := url.Parse(c)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
			continue
		}
		host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
		switch fonte {
		case "linkedin":
			if !hostLinkedIn.MatchString(host) {
				continue
			}
			if g := caminhoLI.FindStringSubmatch(u.Path); g != nil {
				return g[1], "https://www.linkedin.com/jobs/view/" + g[1] + "/"
			}
			if j := u.Query().Get("currentJobId"); len(j) >= 6 && soNumeros(j) && strings.HasPrefix(u.Path, "/jobs") {
				return j, "https://www.linkedin.com/jobs/view/" + j + "/"
			}
		case "indeed":
			if host != "indeed.com" && !strings.HasSuffix(host, ".indeed.com") {
				continue
			}
			jk := strings.ToLower(u.Query().Get("jk"))
			if jk == "" {
				jk = strings.ToLower(u.Query().Get("vjk"))
			}
			if len(jk) != 16 || !hexadecimal(jk) {
				if g := reJK.FindStringSubmatch(c); g != nil {
					jk = strings.ToLower(g[1])
				} else {
					continue
				}
			}
			destino := "www.indeed.com"
			if hostIndeedPa.MatchString(host) {
				destino = host
			}
			return jk, "https://" + destino + "/viewjob?jk=" + jk
		}
	}
	return "", ""
}

func soNumeros(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

func hexadecimal(s string) bool {
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

// linha limpa um texto do e-mail para título, empresa ou local: uma linha, com teto, sem link, sem @ e sem <>.
// Vazio quando não sobra nada aproveitável.
func linha(s string, teto int) string {
	s = rePrefixo.ReplaceAllString(texto(s, 1000), "")
	s = strings.Trim(strings.NewReplacer("<", "", ">", "").Replace(s), " ·•|-–—")
	baixo := strings.ToLower(s)
	if s == "" || reGenerica.MatchString(s) || strings.Contains(baixo, "http") || strings.Contains(baixo, "www.") || strings.Contains(s, "@") || soNumeros(strings.ReplaceAll(s, " ", "")) {
		return ""
	}
	if utf8.RuneCountInString(s) > teto {
		return ""
	}
	return s
}

// linhasDoHTML: o trecho de HTML (fora de âncoras) em linhas de texto.
func linhasDoHTML(h string) []string {
	h = reQuebra.ReplaceAllString(h, "\n")
	var saida []string
	for _, l := range strings.Split(tag.ReplaceAllString(h, " "), "\n") {
		if l = strings.TrimSpace(texto(l, 1000)); l != "" {
			saida = append(saida, l)
		}
	}
	return saida
}

type pedaco struct {
	id, link string // vazio: texto solto
	texto    []string
}

// VagasDoAlerta tira as vagas de um alerta. Prefere o HTML (o título é o texto do link da vaga; empresa e local são
// as linhas logo depois); sem vaga no HTML, tenta o texto simples (título, empresa e local nas linhas antes do link).
func VagasDoAlerta(fonte, textoSimples, corpoHTML string) []dominio.Objeto {
	if fonte != "linkedin" && fonte != "indeed" {
		return nil
	}
	vagas := vagasDoHTML(fonte, corpoHTML)
	if len(vagas) == 0 {
		vagas = vagasDoTexto(fonte, textoSimples)
	}
	if len(vagas) > maxVagasPorAlerta {
		vagas = vagas[:maxVagasPorAlerta]
	}
	return vagas
}

func novaVaga(fonte, id, link, titulo, empresa, local string) dominio.Objeto {
	nome := map[string]string{"linkedin": "LinkedIn", "indeed": "Indeed"}[fonte]
	plataforma := "outra"
	if fonte == "linkedin" {
		plataforma = "linkedin"
	}
	v := dominio.Objeto{"id": fonte + "-" + id, "titulo": titulo, "empresa": empresa, "link": link, "fonteURL": link,
		"plataforma": plataforma, "fonteBusca": "Alerta do " + nome + " (e-mail)"}
	if local != "" {
		v["local"] = local
	}
	return v
}

// empresaELocal: "Acme · São Paulo" vira empresa e local; senão a primeira linha é a empresa e a segunda, o local.
func empresaELocal(linhas []string) (empresa, local string) {
	var limpas []string
	for _, l := range linhas {
		partes := strings.Split(l, " · ")
		for _, p := range partes {
			if p = linha(p, 180); p != "" {
				limpas = append(limpas, p)
			}
		}
		if len(limpas) >= 2 {
			break
		}
	}
	if len(limpas) > 0 {
		empresa = limpas[0]
	}
	if len(limpas) > 1 {
		local = limpas[1]
	}
	return
}

func vagasDoHTML(fonte, h string) []dominio.Objeto {
	if strings.TrimSpace(h) == "" {
		return nil
	}
	h = reBlocoAl.ReplaceAllString(h, " ")
	var pedacos []pedaco
	ultimo := 0
	for _, m := range reAncora.FindAllStringSubmatchIndex(h, -1) {
		if solto := linhasDoHTML(h[ultimo:m[0]]); len(solto) > 0 {
			pedacos = append(pedacos, pedaco{texto: solto})
		}
		id, link := idDaVaga(fonte, h[m[2]:m[3]])
		pedacos = append(pedacos, pedaco{id: id, link: link, texto: linhasDoHTML(h[m[4]:m[5]])})
		ultimo = m[1]
	}
	if solto := linhasDoHTML(h[ultimo:]); len(solto) > 0 {
		pedacos = append(pedacos, pedaco{texto: solto})
	}

	var vagas []dominio.Objeto
	vistos := map[string]bool{}
	for i, p := range pedacos {
		if p.id == "" || vistos[p.id] {
			continue
		}
		titulo := linha(strings.Join(p.texto, " "), 200)
		if titulo == "" {
			continue // o link da imagem; o do título vem depois
		}
		// Empresa e local: as linhas depois do título, até o próximo link de vaga (links de empresa contam como texto).
		var depois []string
		for _, q := range pedacos[i+1:] {
			if q.id == p.id {
				continue // outro link da mesma vaga ("Ver vaga")
			}
			if q.id != "" {
				break
			}
			depois = append(depois, q.texto...)
		}
		empresa, local := empresaELocal(depois)
		if empresa == "" {
			continue
		}
		vistos[p.id] = true
		vagas = append(vagas, novaVaga(fonte, p.id, p.link, titulo, empresa, local))
	}
	return vagas
}

func vagasDoTexto(fonte, t string) []dominio.Objeto {
	var vagas []dominio.Objeto
	vistos := map[string]bool{}
	var bloco []string
	for _, bruta := range strings.Split(strings.ReplaceAll(t, "\r", ""), "\n") {
		l := strings.TrimSpace(bruta)
		if l == "" {
			continue
		}
		if strings.Trim(l, "-=_*· ") == "" {
			bloco = nil // separador entre vagas
			continue
		}
		urls := reURLTexto.FindAllString(l, -1)
		if len(urls) == 0 {
			if c := linha(l, 200); c != "" {
				bloco = append(bloco, c)
			}
			continue
		}
		id, link := "", ""
		for _, u := range urls {
			if id, link = idDaVaga(fonte, strings.TrimRight(u, ").,>")); id != "" {
				break
			}
		}
		if id != "" && !vistos[id] && len(bloco) >= 2 {
			// título, empresa e local são as últimas linhas antes do link
			ini := len(bloco) - 3
			if ini < 0 {
				ini = 0
			}
			b := bloco[ini:]
			titulo, empresa, local := b[0], b[1], ""
			if len(b) == 3 {
				local = b[2]
			}
			vistos[id] = true
			vagas = append(vagas, novaVaga(fonte, id, link, titulo, empresa, local))
		}
		bloco = nil
	}
	return vagas
}
