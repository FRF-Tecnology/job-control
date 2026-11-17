package dominio

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"sort"
	"strings"
)

// E-mails que o próprio app leu da caixa da pessoa (IMAP). O texto vem de terceiros e passou por uma IA: ele pode
// ter sido escrito para enganar. Por isso, o que muda a vaga sozinho é pouco (proposta do #133, aprovada pelo
// CyberSec):
//   - recusa e oferta nunca mudam a vaga sem a pessoa: ficam "a-confirmar";
//   - convite para entrevista e teste mudam sozinhos só quando o remetente é do domínio da vaga (o mesmo domínio do
//     link da vaga ou a empresa no domínio); de outro domínio, também ficam "a-confirmar";
//   - confirmação e "outro" só ficam registrados, como no RegistrarEmail.
// Tudo que muda sozinho é reversível (em-processo com uma pendência).

// VagasCandidatasDoEmail: as vagas em andamento de que o e-mail parece falar (as mesmas regras do AcharVagaDoEmail),
// da mais provável para a menos. É só esta lista que vai para a IA.
func VagasCandidatasDoEmail(vagas []Objeto, email Objeto) []Objeto {
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
	saida := []Objeto{}
	for _, c := range cs {
		saida = append(saida, c.vaga)
	}
	return saida
}

// sufixos de dois níveis comuns (empresa.com.br): o domínio registrável tem três rótulos.
var segundoNivel = map[string]bool{"com.br": true, "net.br": true, "org.br": true, "gov.br": true, "co.uk": true, "com.au": true, "com.ar": true, "com.mx": true, "co.in": true}

// dominioRegistravel: "rh.empresa.com.br" → "empresa.com.br"; "jobs.gupy.io" → "gupy.io".
func dominioRegistravel(host string) string {
	host = strings.Trim(strings.ToLower(aparar(host)), ".")
	r := strings.Split(host, ".")
	if len(r) < 2 {
		return ""
	}
	n := 2
	if len(r) >= 3 && segundoNivel[strings.Join(r[len(r)-2:], ".")] {
		n = 3
	}
	return strings.Join(r[len(r)-n:], ".")
}

func hostDoLink(link string) string {
	l := strings.ToLower(aparar(link))
	for _, p := range []string{"https://", "http://"} {
		l = strings.TrimPrefix(l, p)
	}
	if i := strings.IndexAny(l, "/?#"); i >= 0 {
		l = l[:i]
	}
	if i := strings.LastIndex(l, "@"); i >= 0 {
		l = l[i+1:]
	}
	if i := strings.Index(l, ":"); i >= 0 {
		l = l[:i]
	}
	return l
}

// RemetenteDaVaga: o endereço vem do domínio registrável do link da vaga, ou um rótulo do domínio dele é a empresa
// da vaga (4 letras ou mais). Comparação exata por rótulo, nunca "termina com".
func RemetenteDaVaga(vaga Objeto, de string) bool {
	i := strings.LastIndex(de, "@")
	if i < 0 {
		return false
	}
	host := strings.ToLower(aparar(strings.Trim(de[i+1:], "<> ")))
	reg := dominioRegistravel(host)
	if reg == "" {
		return false
	}
	if reg == dominioRegistravel(hostDoLink(texto(vaga["link"]))) {
		return true
	}
	nomes := nomesDaEmpresa(vaga["empresa"])
	for _, r := range strings.Split(reg, ".") {
		if nomes[r] {
			return true
		}
	}
	return false
}

// nomesDaEmpresa: a chave da empresa e as palavras do começo do nome emendadas ("Acme Pagamentos S.A." → acme,
// acmepagamentos, acmepagamentossa e a chave), cada uma com 4 letras ou mais. É assim que empresas viram domínio.
func nomesDaEmpresa(empresa any) map[string]bool {
	nomes := map[string]bool{}
	if c := ChaveEmpresa(empresa); tamanhoJS(c) >= 4 {
		nomes[c] = true
	}
	junto := ""
	for _, p := range strings.Fields(reNaoAlfaNumEspaco.ReplaceAllString(Normalizar(empresa), " ")) {
		junto += p
		if tamanhoJS(junto) >= 4 {
			nomes[junto] = true
		}
	}
	return nomes
}

// precisaDaPessoa: o tipo de e-mail que só muda a vaga com a confirmação da pessoa.
func precisaDaPessoa(tipo string, vaga Objeto, de string) bool {
	switch tipo {
	case "rejeicao", "oferta":
		return true
	case "entrevista", "teste":
		return !RemetenteDaVaga(vaga, de)
	}
	return false
}

// RegistrarEmailLido registra um e-mail lido pelo app. Igual ao RegistrarEmail, menos quando a mudança precisa da
// pessoa: aí o e-mail fica "a-confirmar", ligado à vaga e com a mudança proposta, e a vaga não muda.
func RegistrarEmailLido(e Estado, email, cls Objeto, ctx Ctx) (Estado, Objeto, error) {
	if erros := exigirEmail(email, cls); len(erros) > 0 {
		return e, nil, &ErroValidacao{erros}
	}
	idMensagem := aparar(jsString(email["idMensagem"]))
	tipo := texto(cls["tipo"])
	for _, x := range e.Emails {
		if igual(x["idMensagem"], idMensagem) {
			return RegistrarEmail(e, email, cls, ctx) // devolve "duplicado"
		}
	}
	vaga := AcharVagaDoEmail(e.Vagas, email, cls["vagaId"])
	if vaga == nil {
		return registrarSemVaga(e, email, cls, ctx)
	}
	if vaga == nil || !precisaDaPessoa(tipo, vaga, texto(email["de"])) {
		return RegistrarEmail(e, email, cls, ctx)
	}
	reg := Objeto{
		"idMensagem": idMensagem, "contaId": ctx.conta(), "data": DataDoEmail(email["data"], ctx.Agora), "tipo": tipo,
		"de": ou(email["de"], ""), "assunto": ou(email["assunto"], ""), "resumo": ou(cls["resumo"], ""), "vagaId": vaga["id"],
		"registradoEm": ctx.Agora,
	}
	if verdadeiro(cls["proximoPasso"]) {
		reg["proximoPasso"] = cls["proximoPasso"]
	}
	t := TransicaoDoEmail(tipo, vaga, reg)
	if t == nil || (igual(vaga["etapa"], t.Etapa) && igual(ou(t.Extras["subEtapa"], nil), ou(vaga["subEtapa"], nil))) {
		return RegistrarEmail(e, email, cls, ctx) // não mudaria nada: só registra
	}
	reg["status"] = "a-confirmar"
	reg["proposta"] = Objeto{"etapa": t.Etapa, "subEtapa": ou(t.Extras["subEtapa"], nil)}
	novo := e.Clonar()
	novo.Emails = append(append([]Objeto{}, novo.Emails...), reg)
	return novo, Objeto{"idMensagem": idMensagem, "status": "a-confirmar", "vagaId": vaga["id"], "transicao": nil}, nil
}

// registrarSemVaga: e-mail lido pelo app sem vaga no Applyer (em geral, candidatura feita fora dele).
//   - "candidatura recebida" e recusa não mudariam etapa nenhuma que importe: ficam registradas como "ignorado" (o id
//     impede ler de novo) e não pedem nada à pessoa;
//   - convite, teste e oferta ficam "sem-vaga", com a sugestão de empresa e cargo que a IA tirou do e-mail, para a
//     pessoa ligar a uma vaga ou criar a vaga (CriarVagaDoEmail).
func registrarSemVaga(e Estado, email, cls Objeto, ctx Ctx) (Estado, Objeto, error) {
	novo, res, err := RegistrarEmail(e, email, cls, ctx)
	if err != nil || res["status"] != "sem-vaga" {
		return novo, res, err
	}
	reg := novo.Emails[len(novo.Emails)-1]
	switch texto(cls["tipo"]) {
	case "confirmacao", "rejeicao":
		reg["status"] = "ignorado"
		res["status"] = "ignorado"
	default:
		sug := Objeto{}
		if v := TextoDeSugestao(cls["empresa"]); v != "" {
			sug["empresa"] = v
		}
		if v := CargoSemLocal(TextoDeSugestao(cls["cargo"])); v != "" {
			sug["cargo"] = v
		}
		if len(sug) > 0 {
			reg["sugestao"] = sug
		}
	}
	return novo, res, nil
}

// TextoDeSugestao limpa o que a IA (ou a pessoa) diz ser a empresa ou o cargo: uma linha, até 120 caracteres, sem
// link e sem <>. Vazio quando não sobra nada ou quando parece link.
func TextoDeSugestao(x any) string {
	s := aparar(strings.Join(strings.Fields(texto(x)), " "))
	s = strings.NewReplacer("<", "", ">", "").Replace(s)
	if l := strings.ToLower(s); strings.Contains(l, "http") || strings.Contains(l, "www.") {
		return ""
	}
	return cortarJS(s, 120)
}

var (
	codigoDaVaga = regexp.MustCompile(`^\s*[\[(](job|vaga|req|ref)?[\s#:-]*[A-Za-z0-9-]*\d[A-Za-z0-9-]*[\])]\s*`)
	localNoFim   = regexp.MustCompile(`(?i)\s*(,|-|–|\||/)\s*(brazil|brasil|remote|remoto|latam|worldwide|híbrido|hibrido)(\s*\(?(remote|remoto)\)?)?\s*$`)
)

// CargoSemLocal tira do título o código da vaga no começo ("[Job-123]") e o local no fim (", Brazil", "- Remote"):
// o cargo da vaga criada de um e-mail fica só o título.
func CargoSemLocal(s string) string {
	s = codigoDaVaga.ReplaceAllString(s, "")
	for localNoFim.MatchString(s) {
		s = localNoFim.ReplaceAllString(s, "")
	}
	return strings.TrimSpace(s)
}

// CriarVagaDoEmail: a pessoa cria a vaga de um e-mail "sem vaga" (candidatura feita fora do app) e liga o e-mail a
// ela. A vaga entra como enviada, na data do e-mail, e o e-mail aplica a etapa (convite e teste: em processo; oferta:
// oferta). Se já existe vaga com a mesma empresa e cargo, liga a ela em vez de criar outra.
func CriarVagaDoEmail(e Estado, idMensagem, empresa, cargo any, ctx Ctx) (Estado, Objeto, error) {
	var reg Objeto
	for _, x := range e.Emails {
		if igual(x["idMensagem"], idMensagem) {
			reg = x
		}
	}
	if reg == nil {
		return e, nil, &ErroNaoEncontrado{"email não encontrado: " + jsString(idMensagem)}
	}
	if !igual(reg["status"], "sem-vaga") {
		return e, nil, validacao("este email não está esperando uma vaga")
	}
	emp, carg := TextoDeSugestao(empresa), TextoDeSugestao(cargo)
	erros := []string{}
	if emp == "" {
		erros = append(erros, "informe a empresa")
	}
	if carg == "" {
		erros = append(erros, "informe o cargo")
	}
	if len(erros) > 0 {
		return e, nil, &ErroValidacao{erros}
	}
	data := texto(reg["data"])
	if !EhDataISO(data) {
		data = ctx.Agora
	}
	vaga := Objeto{
		"id": "email-" + ChaveEmpresa(emp) + "-" + hashCurto(texto(idMensagem)), "empresa": emp, "titulo": carg,
		"etapa": "enviada", "envio": Objeto{"data": data}, "notas": "Criada a partir de um e-mail (candidatura feita fora do Applyer).",
	}
	novo, criada, err := AdicionarVaga(e, vaga, ctx, false)
	var dup *ErroDuplicata
	if errors.As(err, &dup) {
		return LigarEmail(e, idMensagem, dup.Existente["id"], ctx)
	}
	if err != nil {
		return e, nil, err
	}
	novo, res, err := LigarEmail(novo, idMensagem, criada["id"], ctx)
	if err != nil {
		return e, nil, err
	}
	res["criada"] = true
	return novo, res, nil
}

// ConfirmarEmail: a pessoa confirma a mudança proposta de um e-mail "a-confirmar". A mudança é recalculada com a vaga
// de agora; se ela já andou e a mudança não vale mais, nada muda (conflito).
func ConfirmarEmail(e Estado, idMensagem any, ctx Ctx) (Estado, Objeto, error) {
	var reg Objeto
	for _, x := range e.Emails {
		if igual(x["idMensagem"], idMensagem) {
			reg = x
		}
	}
	if reg == nil {
		return e, nil, &ErroNaoEncontrado{"email não encontrado: " + jsString(idMensagem)}
	}
	if !igual(reg["status"], "a-confirmar") {
		return e, nil, validacao("este email não está esperando confirmação")
	}
	i, err := acharIndice(e, reg["vagaId"])
	if err != nil {
		return e, nil, err
	}
	vaga := e.Vagas[i]
	t := TransicaoDoEmail(texto(reg["tipo"]), vaga, reg)
	if t == nil || (igual(vaga["etapa"], t.Etapa) && igual(ou(t.Extras["subEtapa"], nil), ou(vaga["subEtapa"], nil))) {
		return e, nil, &ErroConflito{Msg: "a vaga já mudou desde este email: nada a confirmar", Motivo: "vaga-mudou"}
	}
	novo := e.Clonar()
	var r Objeto
	for _, x := range novo.Emails {
		if igual(x["idMensagem"], idMensagem) && r == nil {
			r = x
		}
	}
	r["status"] = "ligado"
	delete(r, "proposta")
	novo, transicao, err := aplicarEmailNaVaga(novo, reg["vagaId"], r, ctx)
	if err != nil {
		return e, nil, err
	}
	return novo, Objeto{"idMensagem": idMensagem, "status": "ligado", "vagaId": reg["vagaId"], "transicao": transicao}, nil
}

func hashCurto(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:4])
}
