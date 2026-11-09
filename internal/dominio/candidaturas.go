package dominio

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Fila de candidaturas (P5, jc-a2y.6.2; contrato 0.4.x e docs/propostas/p5-contrato-fila.md v3).
// Uma candidatura é o pacote exato que vai para a vaga. A pessoa aprova aquele pacote (pela impressão), a aprovação
// vale uma vez e até o fim do dia local, e o executor só envia o pacote aprovado. Funções puras: recebem o estado e
// devolvem um estado novo, sem mexer no original.

// Estados da candidatura.
const (
	CandPreparada = "preparada"
	CandAprovada  = "aprovada"
	CandEnviando  = "enviando"
	CandEnviada   = "enviada"
	CandBloqueada = "bloqueada"
	CandFalhou    = "falhou"
	CandIncerta   = "incerta"
	CandCancelada = "cancelada"
)

// EstadosCandidatura: todos os estados, na ordem do contrato.
var EstadosCandidatura = []string{CandPreparada, CandAprovada, CandEnviando, CandEnviada, CandBloqueada, CandFalhou, CandIncerta, CandCancelada}

// Estados em que a candidatura ainda ocupa a vaga (só uma por vaga).
var estadosAtivos = []string{CandPreparada, CandAprovada, CandEnviando, CandBloqueada, CandFalhou, CandIncerta}

// PendenciaDoMotivo: lista fechada dos motivos de bloqueio e a pendência fixa de cada um (texto nosso, nada da página).
var PendenciaDoMotivo = map[string]string{
	"cpf":           "A página pede CPF ou outro número fiscal (CNPJ, RFC, CUIT, RUT, SSN…): digite você e envie",
	"rg":            "A página pede RG ou outro documento de identidade (DNI, CURP, cédula…): digite você e envie",
	"senha":         "A página pede uma senha: entre você e envie",
	"dado-bancario": "A página pede dados bancários: preencha você",
	"captcha":       "A página tem captcha: resolva você e envie",
	"aviso-conta":   "A plataforma mostrou um aviso sobre a sua conta: veja você antes de tentar de novo",
	"login":         "A sessão caiu ou pede verificação (2FA ou código): entre você",
	"conta":         "A plataforma pede para criar conta: crie você",
	"teste-pessoal": "A vaga pede teste pessoal (comportamental, vídeo ou entrevista por IA): faça você",
	"documento":     "A página pede um documento além do currículo: anexe você",
	"dado-sensivel": "A página pergunta dado pessoal sensível (raça, deficiência, saúde, religião, data de nascimento…): responda você",
	"termos":        "A página pede aceite de termos ou consentimento: marque você",
	"pergunta-nova": "A página tem uma pergunta obrigatória fora do pacote aprovado: responda você",
	"site-externo":  "A candidatura continua num site fora da plataforma: siga você",
	"outro":         "O envio parou: confira a página",
}

// DetalheDaFalha: lista fechada das falhas técnicas e o texto fixo de cada uma.
var DetalheDaFalha = map[string]string{
	"curriculo-mudou": "O currículo mudou depois da aprovação: revise e aprove de novo",
	"erro-tecnico":    "O envio falhou por um erro técnico: reabra para tentar de novo",
}

// Limite diário por plataforma (LinkedIn, Gupy, InHire). "outra" não tem envio automático nem limite.
const (
	LimitePadrao = 30
	LimiteMaximo = 40
)

// hostsDaPlataforma: o host da plataforma (e os subdomínios dele).
var hostsDaPlataforma = map[string]string{"linkedin": "linkedin.com", "gupy": "gupy.io", "inhire": "inhire.app"}

// PlataformasComLimite são as plataformas com envio automático e limite diário.
var PlataformasComLimite = []string{"linkedin", "gupy", "inhire"}

// Candidatura é a forma tipada do item guardado na coleção "candidaturas".
type Candidatura struct {
	ID         string     `json:"id"`
	VagaID     string     `json:"vagaId"`
	Plataforma string     `json:"plataforma"`
	URL        string     `json:"url"`
	Estado     string     `json:"estado"`
	Conteudo   Objeto     `json:"conteudo"`
	Impressao  string     `json:"impressao"`
	Aprovacao  *Aprovacao `json:"aprovacao"`
	Bloqueio   *Bloqueio  `json:"bloqueio"`
	Resultado  *Resultado `json:"resultado"`
	Tentativas []string   `json:"tentativas,omitempty"` // instantes em que entrou em enviando (contam no limite)
	// Retentativas: voltas automáticas para aprovada depois de falha técnica antes de interagir (jc-a2y.6.11). Não
	// contam no limite; no máximo MaxRetentativas por aprovação.
	Retentativas       []Retentativa `json:"retentativas,omitempty"`
	ProximaTentativaEm string        `json:"proximaTentativaEm,omitempty"` // o executor só pega a aprovada depois disto
	// Marcas gravadas da tentativa em curso (jc-a2y.6.11), cada uma numa transação ANTES do passo: interagiuEm antes
	// do primeiro preencher, anexar, desmarcar ou avançar; clicouEnviarEm antes do clique final. Com qualquer uma, a
	// candidatura nunca volta sozinha para aprovada.
	InteragiuEm    string `json:"interagiuEm,omitempty"`
	ClicouEnviarEm string `json:"clicouEnviarEm,omitempty"`
	Versao         int    `json:"versao"`
	CriadaEm       string `json:"criadaEm"`
	AtualizadaEm   string `json:"atualizadaEm"`
}

// Retentativa: uma volta automática para aprovada (instante e código da falha, da lista fechada).
type Retentativa struct {
	Em     string `json:"em"`
	Codigo string `json:"codigo"`
}

// Aprovacao: a impressão aprovada, até quando vale (fim do dia local) e o fuso gravado.
type Aprovacao struct {
	Em        string `json:"em"`
	Impressao string `json:"impressao"`
	ValidaAte string `json:"validaAte"`
	Fuso      string `json:"fuso"`
	UsadaEm   string `json:"usadaEm,omitempty"`
}

// Bloqueio: motivo da lista fechada e a pendência fixa.
type Bloqueio struct {
	Motivo    string `json:"motivo"`
	Pendencia string `json:"pendencia"`
	Em        string `json:"em"`
}

// Resultado: enviada | falhou | incerto | manual, com detalhe fixo.
type Resultado struct {
	Em      string `json:"em"`
	Tipo    string `json:"tipo"`
	Detalhe string `json:"detalhe,omitempty"`
}

// ErroConflito (409): estado que não permite a ação, impressão ou versão velha, duplicata ou vaga fora da etapa.
type ErroConflito struct {
	Msg            string
	Estado         string
	ImpressaoAtual string
	Existente      string
	Motivo         string // só no /navegador/entrar: enviando, login-aberto ou sem-navegador (jc-a2y.6.8)
}

func (e *ErroConflito) Error() string { return e.Msg }

// ErroLimite (429): limite diário da plataforma atingido.
type ErroLimite struct {
	Plataforma string
	Limite     int
	Usadas     int
	RenovaEm   string
}

func (e *ErroLimite) Error() string {
	return fmt.Sprintf("limite diário de %d candidaturas em %s atingido; renova em %s", e.Limite, e.Plataforma, e.RenovaEm)
}

// ErroSoPessoa: a ação é só da pessoa (tela). Na P6, MCP e CLI recebem 403 (regra M4).
type ErroSoPessoa struct{ Msg string }

func (e *ErroSoPessoa) Error() string { return e.Msg }

func soPessoa(ctx Ctx, acao string) error {
	if ctx.EhPessoa() {
		return nil
	}
	return &ErroSoPessoa{"só a pessoa pode " + acao + " uma candidatura"}
}

// PlataformaDaVaga: linkedin, gupy ou inhire pelo campo plataforma da vaga; o resto é "outra".
func PlataformaDaVaga(v Objeto) string {
	p := strings.ToLower(strings.TrimSpace(texto(v["plataforma"])))
	if _, ok := hostsDaPlataforma[p]; ok {
		return p
	}
	return "outra"
}

// URLDaPlataforma diz se o endereço é https e, para linkedin/gupy/inhire, se o host é o da plataforma ou um
// subdomínio dele. Compara o host já analisado pela biblioteca de URL (M5): "linkedin.com@x.io", "evil-linkedin.com"
// e "linkedin.com.x.io" não passam. Para "outra", basta https com host.
func URLDaPlataforma(plataforma, endereco string) bool {
	u, err := url.Parse(strings.TrimSpace(endereco))
	if err != nil || u.Scheme != "https" || u.User != nil {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return false
	}
	base, conhecida := hostsDaPlataforma[plataforma]
	if !conhecida {
		return plataforma == "outra"
	}
	return host == base || strings.HasSuffix(host, "."+base)
}

// LimiteDe devolve o limite configurado da plataforma (padrão 30); ok é false para "outra" (sem limite).
func LimiteDe(limites map[string]int, plataforma string) (int, bool) {
	if !contem(PlataformasComLimite, plataforma) {
		return 0, false
	}
	if n, tem := limites[plataforma]; tem && n >= 1 && n <= LimiteMaximo {
		return n, true
	}
	return LimitePadrao, true
}

// ValidarLimite: de 1 a 40.
func ValidarLimite(n int) error {
	if n < 1 || n > LimiteMaximo {
		return validacao(fmt.Sprintf("o limite diário vai de 1 a %d", LimiteMaximo))
	}
	return nil
}

func mesmoDia(a, b time.Time) bool {
	ya, ma, da := a.Date()
	yb, mb, db := b.Date()
	return ya == yb && ma == mb && da == db
}

// UsadasNoDia conta as tentativas (entradas em enviando) da plataforma no dia local de agora. Falha e incerta contam.
func UsadasNoDia(e Estado, plataforma string, agora time.Time) int {
	n := 0
	for _, o := range e.Candidaturas {
		if !igual(o["plataforma"], plataforma) {
			continue
		}
		for _, t := range lista(o["tentativas"]) {
			if quando, err := time.Parse(time.RFC3339, texto(t)); err == nil && mesmoDia(quando.In(agora.Location()), agora) {
				n++
			}
		}
	}
	return n
}

// FimDoDia é o último segundo do dia local de agora; RenovaEm é o início do dia seguinte.
func FimDoDia(agora time.Time) time.Time {
	y, m, d := agora.Date()
	return time.Date(y, m, d, 23, 59, 59, 0, agora.Location())
}

func RenovaEm(agora time.Time) time.Time {
	y, m, d := agora.Date()
	return time.Date(y, m, d+1, 0, 0, 0, 0, agora.Location())
}

var (
	reArquivoCurriculo = regexp.MustCompile(`^[^/\\:]+\.pdf$`)
	reSHA256           = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// NomeDeCurriculoValido: só o nome de um PDF, sem pasta, barra, drive nem fluxo alternativo (:).
func NomeDeCurriculoValido(nome string) bool { return reArquivoCurriculo.MatchString(nome) }

// ValidarConteudo confere o pacote com o esquema fechado do contrato (additionalProperties: false em todo nível)
// e recusa CPF/RG em qualquer texto. O sha256 do currículo é calculado pela API antes de chegar aqui.
func ValidarConteudo(c Objeto) []string {
	var erros []string
	ruim := func(f string, a ...any) { erros = append(erros, fmt.Sprintf(f, a...)) }
	soCampos := func(o Objeto, onde string, campos ...string) {
		for k := range o {
			if !contem(campos, k) {
				ruim("%s: campo não permitido: %s", onde, k)
			}
		}
	}
	if c == nil {
		return []string{"conteudo: obrigatório"}
	}
	soCampos(c, "conteudo", "curriculo", "idioma", "contato", "pretensao", "respostas", "mensagem")
	cv, ok := c["curriculo"].(Objeto)
	if !ok {
		ruim("conteudo.curriculo: obrigatório")
	} else {
		soCampos(cv, "conteudo.curriculo", "arquivo", "sha256")
		if a, _ := cv["arquivo"].(string); !NomeDeCurriculoValido(a) {
			ruim("conteudo.curriculo.arquivo: nome de um PDF da pasta de currículos, sem caminho")
		} else if s, _ := cv["sha256"].(string); !reSHA256.MatchString(s) {
			ruim("conteudo.curriculo.sha256: ausente")
		}
	}
	if i := c["idioma"]; i != "pt" && i != "en" {
		ruim("conteudo.idioma: pt ou en")
	}
	if ct, tem := c["contato"]; tem && ct != nil {
		o, ok := ct.(Objeto)
		if !ok {
			ruim("conteudo.contato: objeto ou null")
		} else {
			soCampos(o, "conteudo.contato", "nome", "email", "telefone", "cidade")
			for k, v := range o {
				if _, ok := v.(string); !ok {
					ruim("conteudo.contato.%s: texto", k)
				}
			}
		}
	}
	if p, tem := c["pretensao"]; tem && p != nil {
		o, ok := p.(Objeto)
		if !ok {
			ruim("conteudo.pretensao: objeto ou null")
		} else {
			soCampos(o, "conteudo.pretensao", "valor", "moeda", "tipo")
			if v, tem := o["valor"]; tem {
				if _, ok := v.(float64); !ok {
					ruim("conteudo.pretensao.valor: número")
				}
			}
			if m, tem := o["moeda"]; tem && !contem([]string{"BRL", "USD", "EUR"}, texto(m)) {
				ruim("conteudo.pretensao.moeda: BRL, USD ou EUR")
			}
			if tp, tem := o["tipo"]; tem && !contem([]string{"CLT", "PJ", "contractor"}, texto(tp)) {
				ruim("conteudo.pretensao.tipo: CLT, PJ ou contractor")
			}
		}
	}
	rs, ok := c["respostas"].([]any)
	if !ok {
		ruim("conteudo.respostas: lista")
	}
	for i, r := range rs {
		o, ok := r.(Objeto)
		if !ok {
			ruim("conteudo.respostas[%d]: objeto", i)
			continue
		}
		soCampos(o, fmt.Sprintf("conteudo.respostas[%d]", i), "pergunta", "resposta", "origem")
		for _, k := range []string{"pergunta", "resposta"} {
			if _, ok := o[k].(string); !ok {
				ruim("conteudo.respostas[%d].%s: texto", i, k)
			}
		}
		if m := perguntaSensivel(texto(o["pergunta"])); m != "" {
			ruim("conteudo.respostas[%d].pergunta: só a pessoa responde na página (%s)", i, m)
		}
		if !contem([]string{"respostasPadrao", "pessoa", "ia"}, texto(o["origem"])) {
			ruim("conteudo.respostas[%d].origem: respostasPadrao, pessoa ou ia", i)
		}
	}
	if m, tem := c["mensagem"]; tem && m != nil {
		if _, ok := m.(string); !ok {
			ruim("conteudo.mensagem: texto ou null")
		}
	}
	if len(erros) == 0 && ContemDocumentoEm(c) {
		ruim("%s", erroDocumento)
	}
	return erros
}

func candidaturaDe(o Objeto) (Candidatura, error) {
	var c Candidatura
	b, err := json.Marshal(o)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}

func (c Candidatura) objeto() Objeto {
	b, _ := json.Marshal(c)
	var o Objeto
	json.Unmarshal(b, &o)
	return o
}

func acharCandidatura(e Estado, id any) (int, Candidatura, error) {
	for i, o := range e.Candidaturas {
		if igual(o["id"], id) {
			c, err := candidaturaDe(o)
			return i, c, err
		}
	}
	return -1, Candidatura{}, &ErroNaoEncontrado{"candidatura não encontrada: " + jsString(id)}
}

func conflitoEstado(c Candidatura, acao string) error {
	return &ErroConflito{Msg: fmt.Sprintf("não dá para %s uma candidatura %s", acao, c.Estado), Estado: c.Estado}
}

// salvarCandidatura grava a candidatura no estado novo (cópia), subindo a versão e a data de atualização.
func salvarCandidatura(e Estado, i int, c Candidatura, agora time.Time) (Estado, Objeto) {
	novo := e.Clonar()
	c.Versao++
	c.AtualizadaEm = agora.Format(time.RFC3339)
	o := c.objeto()
	if i < 0 {
		novo.Candidaturas = append(novo.Candidaturas, o)
	} else {
		novo.Candidaturas[i] = o
	}
	return novo, clonarObj(o)
}

func vagaDe(e Estado, id string) Objeto {
	for _, v := range e.Vagas {
		if igual(v["id"], id) {
			return v
		}
	}
	return nil
}

// vagaAprovada: está na fila do /aplicar (aprovada pela pessoa na Triagem ou grupo A no modo direto).
func vagaAprovada(e Estado, id string) bool {
	for _, v := range FilaAplicar(e) {
		if igual(v["id"], id) {
			return true
		}
	}
	return false
}

// CriarCandidatura cria a candidatura de uma vaga aprovada para aplicar → preparada. Só a pessoa cria (M4).
func CriarCandidatura(e Estado, vagaID string, conteudo Objeto, agora time.Time, ctx Ctx) (Estado, Objeto, error) {
	if err := soPessoa(ctx, "criar"); err != nil {
		return e, nil, err
	}
	if erros := ValidarConteudo(conteudo); len(erros) > 0 { // entrada primeiro: 400 antes de 404/409
		return e, nil, &ErroValidacao{erros}
	}
	vaga := vagaDe(e, vagaID)
	if vaga == nil {
		return e, nil, &ErroNaoEncontrado{"vaga não encontrada: " + vagaID}
	}
	if FoiEnviada(vaga) {
		return e, nil, &ErroConflito{Msg: "esta vaga já foi enviada", Existente: vagaID}
	}
	for _, o := range e.Candidaturas {
		if igual(o["vagaId"], vagaID) && contem(estadosAtivos, texto(o["estado"])) {
			return e, nil, &ErroConflito{Msg: "esta vaga já tem uma candidatura ativa", Existente: texto(o["id"])}
		}
	}
	for _, d := range DuplicatasPorTitulo(e.Vagas, vaga, vagaID) {
		if FoiEnviada(d) {
			return e, nil, &ErroConflito{Msg: "já há candidatura enviada para a mesma empresa e título", Existente: texto(d["id"])}
		}
	}
	if !vagaAprovada(e, vagaID) {
		return e, nil, &ErroConflito{Msg: "a vaga não está aprovada para aplicar (Triagem)"}
	}
	plataforma := PlataformaDaVaga(vaga)
	link := strings.TrimSpace(texto(vaga["link"]))
	if !URLDaPlataforma(plataforma, link) {
		return e, nil, validacao("o link da vaga precisa ser https e do site da plataforma (" + plataforma + ")")
	}
	impressao, err := ImpressaoCandidatura(vagaID, plataforma, link, conteudo)
	if err != nil {
		return e, nil, err
	}
	c := Candidatura{
		ID:         fmt.Sprintf("cand-%s-%04d-%s", ctx.Agora, len(e.Candidaturas)+1, ctx.sufixo()),
		VagaID:     vagaID,
		Plataforma: plataforma,
		URL:        link,
		Estado:     CandPreparada,
		Conteudo:   clonarObj(conteudo),
		Impressao:  impressao,
		CriadaEm:   agora.Format(time.RFC3339),
	}
	novo, o := salvarCandidatura(e, -1, c, agora)
	return novo, o, nil
}

// EditarConteudo troca o pacote em preparada ou aprovada. Aprovada volta a preparada e perde a aprovação.
func EditarConteudo(e Estado, id string, conteudo Objeto, versao int, agora time.Time, ctx Ctx) (Estado, Objeto, error) {
	if err := soPessoa(ctx, "editar"); err != nil {
		return e, nil, err
	}
	i, c, err := acharCandidatura(e, id)
	if err != nil {
		return e, nil, err
	}
	if c.Estado != CandPreparada && c.Estado != CandAprovada {
		return e, nil, conflitoEstado(c, "editar")
	}
	if versao != c.Versao {
		return e, nil, &ErroConflito{Msg: "a candidatura mudou enquanto você editava; recarregue", Estado: c.Estado}
	}
	if erros := ValidarConteudo(conteudo); len(erros) > 0 {
		return e, nil, &ErroValidacao{erros}
	}
	impressao, err := ImpressaoCandidatura(c.VagaID, c.Plataforma, c.URL, conteudo)
	if err != nil {
		return e, nil, err
	}
	c.Conteudo, c.Impressao, c.Estado, c.Aprovacao = clonarObj(conteudo), impressao, CandPreparada, nil
	novo, o := salvarCandidatura(e, i, c, agora)
	return novo, o, nil
}

// AprovarCandidatura: preparada → aprovada, só se a impressão enviada for a atual. Vale uma vez, até o fim do dia
// local, no fuso de agora (gravado). Limite atingido dá ErroLimite.
func AprovarCandidatura(e Estado, id, impressao string, agora time.Time, limites map[string]int, ctx Ctx) (Estado, Objeto, error) {
	if err := soPessoa(ctx, "aprovar"); err != nil {
		return e, nil, err
	}
	i, c, err := acharCandidatura(e, id)
	if err != nil {
		return e, nil, err
	}
	if c.Estado != CandPreparada {
		return e, nil, conflitoEstado(c, "aprovar")
	}
	if impressao != c.Impressao {
		return e, nil, &ErroConflito{Msg: "o conteúdo mudou; revise de novo", Estado: c.Estado, ImpressaoAtual: c.Impressao}
	}
	if !vagaAprovada(e, c.VagaID) {
		return e, nil, &ErroConflito{Msg: "a vaga não está mais aprovada para aplicar", Estado: c.Estado}
	}
	if err := conferirLimite(e, c.Plataforma, agora, limites); err != nil {
		return e, nil, err
	}
	c.ProximaTentativaEm = "" // aprovação nova: as voltas recomeçam do zero (no máximo 3 por aprovação; o histórico fica)
	c.Aprovacao = &Aprovacao{
		Em:        agora.Format(time.RFC3339),
		Impressao: c.Impressao,
		ValidaAte: FimDoDia(agora).Format(time.RFC3339),
		Fuso:      agora.Location().String(),
	}
	c.Estado = CandAprovada
	novo, o := salvarCandidatura(e, i, c, agora)
	return novo, o, nil
}

func conferirLimite(e Estado, plataforma string, agora time.Time, limites map[string]int) error {
	limite, tem := LimiteDe(limites, plataforma)
	if !tem {
		return nil
	}
	if usadas := UsadasNoDia(e, plataforma, agora); usadas >= limite {
		return &ErroLimite{Plataforma: plataforma, Limite: limite, Usadas: usadas, RenovaEm: RenovaEm(agora).Format(time.RFC3339)}
	}
	// Teto firme de páginas abertas (jc-a2y.6.11): as voltas não contam no limite, mas contadas + devolvidas no dia
	// nunca passam de LimiteMaximo por plataforma.
	if abertas := AberturasNoDia(e, plataforma, agora); abertas >= LimiteMaximo {
		return &ErroLimite{Plataforma: plataforma, Limite: limite, Usadas: UsadasNoDia(e, plataforma, agora), RenovaEm: RenovaEm(agora).Format(time.RFC3339)}
	}
	return nil
}

// AberturasNoDia: páginas abertas hoje na plataforma, contando as tentativas devolvidas (retentativas).
func AberturasNoDia(e Estado, plataforma string, agora time.Time) int {
	n := UsadasNoDia(e, plataforma, agora)
	for _, o := range e.Candidaturas {
		if !igual(o["plataforma"], plataforma) {
			continue
		}
		for _, r := range lista(o["retentativas"]) {
			if quando, err := time.Parse(time.RFC3339, texto(ObjetoDe(r)["em"])); err == nil && mesmoDia(quando.In(agora.Location()), agora) {
				n++
			}
		}
	}
	return n
}

// RevogarCandidatura: aprovada → preparada (tira a aprovação). Pode vir de qualquer autor: só reduz o risco.
func RevogarCandidatura(e Estado, id string, agora time.Time, ctx Ctx) (Estado, Objeto, error) {
	i, c, err := acharCandidatura(e, id)
	if err != nil {
		return e, nil, err
	}
	if c.Estado != CandAprovada {
		return e, nil, conflitoEstado(c, "revogar")
	}
	c.Estado, c.Aprovacao = CandPreparada, nil
	c.ProximaTentativaEm = ""
	novo, o := salvarCandidatura(e, i, c, agora)
	return novo, o, nil
}

// CancelarCandidatura: preparada, aprovada, bloqueada ou falhou → cancelada (final). Incerta não. Só a pessoa (R5).
func CancelarCandidatura(e Estado, id string, agora time.Time, ctx Ctx) (Estado, Objeto, error) {
	if err := soPessoa(ctx, "cancelar"); err != nil {
		return e, nil, err
	}
	i, c, err := acharCandidatura(e, id)
	if err != nil {
		return e, nil, err
	}
	if !contem([]string{CandPreparada, CandAprovada, CandBloqueada, CandFalhou}, c.Estado) {
		return e, nil, conflitoEstado(c, "cancelar")
	}
	c.Estado, c.Aprovacao = CandCancelada, nil
	novo, o := salvarCandidatura(e, i, c, agora)
	return novo, o, nil
}

// ReabrirCandidatura: bloqueada, falhou ou incerta → preparada; enviar de novo exige nova aprovação. Se a vaga
// estava bloqueada por esta candidatura, volta para aplicar. Só a pessoa.
func ReabrirCandidatura(e Estado, id string, agora time.Time, ctx Ctx) (Estado, Objeto, error) {
	if err := soPessoa(ctx, "reabrir"); err != nil {
		return e, nil, err
	}
	i, c, err := acharCandidatura(e, id)
	if err != nil {
		return e, nil, err
	}
	if !contem([]string{CandBloqueada, CandFalhou, CandIncerta}, c.Estado) {
		return e, nil, conflitoEstado(c, "reabrir")
	}
	c.Estado, c.Aprovacao, c.Bloqueio, c.Resultado = CandPreparada, nil, nil, nil
	novo, o := salvarCandidatura(e, i, c, agora)
	if v := vagaDe(novo, c.VagaID); v != nil && igual(v["etapa"], "bloqueada") {
		movido, _, err := MoverVaga(novo, c.VagaID, "aplicar", Objeto{}, ctx)
		if err != nil {
			return e, nil, err
		}
		novo = movido
	}
	return novo, o, nil
}

// ConcluirManual: bloqueada ou incerta → enviada (a pessoa enviou sozinha ou conferiu na plataforma). A vaga vai
// para enviada na mesma transação. Só a pessoa.
func ConcluirManual(e Estado, id string, agora time.Time, ctx Ctx) (Estado, Objeto, error) {
	if err := soPessoa(ctx, "marcar como enviada"); err != nil {
		return e, nil, err
	}
	i, c, err := acharCandidatura(e, id)
	if err != nil {
		return e, nil, err
	}
	if c.Estado != CandBloqueada && c.Estado != CandIncerta {
		return e, nil, conflitoEstado(c, "marcar como enviada")
	}
	c.Estado = CandEnviada
	c.Resultado = &Resultado{Em: agora.Format(time.RFC3339), Tipo: "manual"}
	return concluirEnvio(e, i, c, agora, ctx)
}

func concluirEnvio(e Estado, i int, c Candidatura, agora time.Time, ctx Ctx) (Estado, Objeto, error) {
	novo, o := salvarCandidatura(e, i, c, agora)
	if v := vagaDe(novo, c.VagaID); v != nil && !FoiEnviada(v) {
		movido, _, err := MoverVaga(novo, c.VagaID, "enviada", Objeto{"envio": Objeto{"data": ctx.Agora, "candidaturaId": c.ID}}, ctx)
		if err != nil {
			return e, nil, err
		}
		novo = movido
	}
	return novo, o, nil
}

// ---------- executor (jc-a2y.6.2.3): código interno, sem rota HTTP ----------

// IniciarEnvio: aprovada → enviando, com a conferência atômica do M2 (versão, impressão aprovada igual à atual,
// validade no instante de agora e limite). Consome a aprovação e registra a tentativa (conta no limite).
func IniciarEnvio(e Estado, id string, versao int, agora time.Time, limites map[string]int) (Estado, Objeto, error) {
	i, c, err := acharCandidatura(e, id)
	if err != nil {
		return e, nil, err
	}
	if c.Estado != CandAprovada || c.Aprovacao == nil || c.Aprovacao.UsadaEm != "" {
		return e, nil, conflitoEstado(c, "enviar")
	}
	// "outra" não tem envio automático: recusa antes de qualquer mudança (a aprovação continua sem uso).
	if _, temEnvio := LimiteDe(nil, c.Plataforma); !temEnvio {
		return e, nil, &ErroConflito{Msg: "candidatura fora de LinkedIn, Gupy e InHire não tem envio automático", Estado: c.Estado}
	}
	if versao != c.Versao {
		return e, nil, &ErroConflito{Msg: "a candidatura mudou depois de lida", Estado: c.Estado}
	}
	// A impressão é recalculada do pacote guardado: um campo trocado direto no banco não passa (M1).
	atual, err := ImpressaoCandidatura(c.VagaID, c.Plataforma, c.URL, c.Conteudo)
	if err != nil || c.Aprovacao.Impressao != atual || c.Impressao != atual {
		return e, nil, &ErroConflito{Msg: "a aprovação não é deste conteúdo", Estado: c.Estado, ImpressaoAtual: atual}
	}
	if !AprovacaoValida(c.Aprovacao, agora) {
		return e, nil, &ErroConflito{Msg: "a aprovação venceu", Estado: c.Estado}
	}
	if !ProntaParaTentar(c, agora) {
		return e, nil, &ErroConflito{Msg: "a próxima tentativa ainda não chegou", Estado: c.Estado}
	}
	if !URLDaPlataforma(c.Plataforma, c.URL) {
		return e, nil, &ErroConflito{Msg: "o endereço não é do site da plataforma", Estado: c.Estado}
	}
	if v := vagaDe(e, c.VagaID); v == nil || FoiEnviada(v) || !vagaAprovada(e, c.VagaID) {
		return e, nil, &ErroConflito{Msg: "a vaga não está mais aprovada para aplicar", Estado: c.Estado}
	}
	if err := conferirLimite(e, c.Plataforma, agora, limites); err != nil {
		return e, nil, err
	}
	c.Estado = CandEnviando
	c.Aprovacao.UsadaEm = agora.Format(time.RFC3339)
	c.Tentativas = append(c.Tentativas, agora.Format(time.RFC3339))
	c.InteragiuEm, c.ClicouEnviarEm = "", "" // tentativa nova: as marcas são desta tentativa
	novo, o := salvarCandidatura(e, i, c, agora)
	return novo, o, nil
}

// ProntaParaTentar: sem próxima tentativa marcada, ou já passou dela.
func ProntaParaTentar(c Candidatura, agora time.Time) bool {
	if c.ProximaTentativaEm == "" {
		return true
	}
	quando, err := time.Parse(time.RFC3339, c.ProximaTentativaEm)
	return err == nil && !agora.Before(quando)
}

// MaxRetentativas e o espaço entre elas (jc-a2y.6.11): 5 min, 30 min, 2 h.
const MaxRetentativas = 3

var esperaDaRetentativa = []time.Duration{5 * time.Minute, 30 * time.Minute, 2 * time.Hour}

// CodigosDeVolta: o que pode devolver a candidatura para aprovada (sempre antes de interagir com o formulário).
var CodigosDeVolta = map[string]bool{"erro-tecnico": true, "login": true}

// DevolverAprovada (jc-a2y.6.11): enviando → aprovada, quando o executor falhou ANTES de interagir com o
// formulário. "Antes" é a marca gravada, não dedução: com interagiuEm ou clicouEnviarEm, recusa (409) e o executor
// segue para falhou, bloqueada ou incerta, como sempre (M2, M3). A volta não é aprovação nova:
//   - versao tem de ser a lida depois de entrar em enviando (qualquer gravação no meio, como uma marca, recusa);
//   - a impressão recalculada do pacote tem de ser a aprovada;
//   - a aprovação volta a não usada, com a mesma impressão e a mesma validade; vencida (passou da meia-noite), a
//     candidatura volta a preparada, sem aprovação;
//   - a tentativa sai de tentativas (não conta no limite) e entra em retentativas (histórico; conta no teto de
//     páginas, AberturasNoDia);
//   - a próxima tentativa fica para daqui a 5 min, 30 min ou 2 h.
//
// Na volta de número MaxRetentativas+1 desde a aprovação, a candidatura vai para falhou/erro-tecnico. Só o executor
// chama isto.
func DevolverAprovada(e Estado, id string, versao int, codigo string, agora time.Time) (Estado, Objeto, error) {
	if !CodigosDeVolta[codigo] {
		return e, nil, validacao("código de volta desconhecido: " + codigo)
	}
	i, c, err := acharCandidatura(e, id)
	if err != nil {
		return e, nil, err
	}
	if c.Estado != CandEnviando || c.Aprovacao == nil {
		return e, nil, conflitoEstado(c, "devolver")
	}
	if c.InteragiuEm != "" || c.ClicouEnviarEm != "" {
		return e, nil, &ErroConflito{Msg: "a tentativa já interagiu com o formulário", Estado: c.Estado}
	}
	if versao != c.Versao {
		return e, nil, &ErroConflito{Msg: "a candidatura mudou depois de lida", Estado: c.Estado}
	}
	atual, err := ImpressaoCandidatura(c.VagaID, c.Plataforma, c.URL, c.Conteudo)
	if err != nil || c.Aprovacao.Impressao != atual || c.Impressao != atual {
		return e, nil, &ErroConflito{Msg: "a aprovação não é deste conteúdo", Estado: c.Estado, ImpressaoAtual: atual}
	}
	feitas := RetentativasDaAprovacao(c)
	if feitas >= MaxRetentativas {
		return FalharCandidatura(e, id, "erro-tecnico", agora)
	}
	c.Estado = CandAprovada
	c.Aprovacao.UsadaEm = ""
	if n := len(c.Tentativas); n > 0 {
		c.Tentativas = c.Tentativas[:n-1] // a tentativa que acabou de entrar em enviando não conta no limite
	}
	c.Retentativas = append(c.Retentativas, Retentativa{Em: agora.Format(time.RFC3339), Codigo: codigo})
	c.ProximaTentativaEm = agora.Add(esperaDaRetentativa[feitas]).Format(time.RFC3339)
	if !AprovacaoValida(c.Aprovacao, agora) {
		c.Estado, c.Aprovacao, c.ProximaTentativaEm = CandPreparada, nil, "" // a aprovação era do dia que passou
	}
	novo, o := salvarCandidatura(e, i, c, agora)
	return novo, o, nil
}

// RetentativasDaAprovacao: voltas desde a aprovação em vigor (as de aprovações antigas ficam só no histórico).
func RetentativasDaAprovacao(c Candidatura) int {
	if c.Aprovacao == nil {
		return 0
	}
	desde, err := time.Parse(time.RFC3339, c.Aprovacao.Em)
	if err != nil {
		return len(c.Retentativas)
	}
	n := 0
	for _, r := range c.Retentativas {
		if em, err := time.Parse(time.RFC3339, r.Em); err != nil || !em.Before(desde) {
			n++
		}
	}
	return n
}

// Marcas da tentativa (jc-a2y.6.11). O executor grava a marca, numa transação, ANTES do passo.
const (
	MarcaInteragiu    = "interagiu"     // antes do primeiro preencher, anexar, desmarcar ou avançar
	MarcaClicouEnviar = "clicou-enviar" // antes do clique final
)

// MarcarTentativa grava a marca na candidatura em enviando. Repetir não muda o instante gravado. clicou-enviar
// também grava interagiu, se faltar (formulário sem campo nenhum).
func MarcarTentativa(e Estado, id, marca string, agora time.Time) (Estado, Objeto, error) {
	if marca != MarcaInteragiu && marca != MarcaClicouEnviar {
		return e, nil, validacao("marca desconhecida: " + marca)
	}
	i, c, err := acharCandidatura(e, id)
	if err != nil {
		return e, nil, err
	}
	if c.Estado != CandEnviando {
		return e, nil, conflitoEstado(c, "marcar a tentativa")
	}
	quando := agora.Format(time.RFC3339)
	if c.InteragiuEm == "" {
		c.InteragiuEm = quando
	}
	if marca == MarcaClicouEnviar && c.ClicouEnviarEm == "" {
		c.ClicouEnviarEm = quando
	}
	novo, o := salvarCandidatura(e, i, c, agora)
	return novo, o, nil
}

// AprovacaoValida compara instantes: mudar o relógio de fuso não estende a validade gravada.
func AprovacaoValida(a *Aprovacao, agora time.Time) bool {
	if a == nil {
		return false
	}
	ate, err := time.Parse(time.RFC3339, a.ValidaAte)
	return err == nil && !agora.After(ate)
}

// VencerAprovacoes: aprovada com validade vencida volta a preparada.
func VencerAprovacoes(e Estado, agora time.Time) (Estado, int) {
	novo, n := e.Clonar(), 0
	for i, o := range novo.Candidaturas {
		c, err := candidaturaDe(o)
		if err != nil || c.Estado != CandAprovada || AprovacaoValida(c.Aprovacao, agora) {
			continue
		}
		c.Estado, c.Aprovacao = CandPreparada, nil
		c.ProximaTentativaEm = ""
		c.Versao++
		c.AtualizadaEm = agora.Format(time.RFC3339)
		novo.Candidaturas[i] = c.objeto()
		n++
	}
	return novo, n
}

// MarcarEnviada: enviando → enviada; a vaga vai para enviada na mesma transação.
func MarcarEnviada(e Estado, id string, agora time.Time, ctx Ctx) (Estado, Objeto, error) {
	i, c, err := acharCandidatura(e, id)
	if err != nil {
		return e, nil, err
	}
	if c.Estado != CandEnviando {
		return e, nil, conflitoEstado(c, "concluir")
	}
	c.Estado = CandEnviada
	c.Resultado = &Resultado{Em: agora.Format(time.RFC3339), Tipo: "enviada"}
	return concluirEnvio(e, i, c, agora, ctx)
}

// BloquearCandidatura: enviando → bloqueada, com motivo da lista fechada e pendência fixa; a vaga vai para
// bloqueada com a mesma pendência, na mesma transação.
func BloquearCandidatura(e Estado, id, motivo string, agora time.Time, ctx Ctx) (Estado, Objeto, error) {
	pendencia, ok := PendenciaDoMotivo[motivo]
	if !ok {
		return e, nil, validacao("motivo de bloqueio desconhecido: " + motivo)
	}
	i, c, err := acharCandidatura(e, id)
	if err != nil {
		return e, nil, err
	}
	if c.Estado != CandEnviando {
		return e, nil, conflitoEstado(c, "bloquear")
	}
	c.Estado = CandBloqueada
	c.Bloqueio = &Bloqueio{Motivo: motivo, Pendencia: pendencia, Em: agora.Format(time.RFC3339)}
	novo, o := salvarCandidatura(e, i, c, agora)
	if v := vagaDe(novo, c.VagaID); v != nil && !FoiEnviada(v) {
		movido, _, err := MoverVaga(novo, c.VagaID, "bloqueada", Objeto{"pendencia": pendencia}, ctx)
		if err != nil {
			return e, nil, err
		}
		novo = movido
	}
	return novo, o, nil
}

// FalharCandidatura: enviando → falhou, com detalhe da lista fechada.
func FalharCandidatura(e Estado, id, codigo string, agora time.Time) (Estado, Objeto, error) {
	detalhe, ok := DetalheDaFalha[codigo]
	if !ok {
		return e, nil, validacao("falha desconhecida: " + codigo)
	}
	i, c, err := acharCandidatura(e, id)
	if err != nil {
		return e, nil, err
	}
	if c.Estado != CandEnviando {
		return e, nil, conflitoEstado(c, "marcar como falha")
	}
	c.Estado = CandFalhou
	c.Resultado = &Resultado{Em: agora.Format(time.RFC3339), Tipo: "falhou", Detalhe: detalhe}
	novo, o := salvarCandidatura(e, i, c, agora)
	return novo, o, nil
}

// EncaminharExterna: aprovada de "outra" plataforma → bloqueada/site-externo, sem abrir página nem contar no limite.
// A pessoa envia sozinha e usa concluir-manual. A aprovação fica como estava (sem uso).
func EncaminharExterna(e Estado, id string, agora time.Time, ctx Ctx) (Estado, Objeto, error) {
	i, c, err := acharCandidatura(e, id)
	if err != nil {
		return e, nil, err
	}
	if c.Estado != CandAprovada {
		return e, nil, conflitoEstado(c, "encaminhar")
	}
	if _, temEnvio := LimiteDe(nil, c.Plataforma); temEnvio {
		return e, nil, &ErroConflito{Msg: "a plataforma tem envio automático", Estado: c.Estado}
	}
	pendencia := PendenciaDoMotivo["site-externo"]
	c.Estado = CandBloqueada
	c.Bloqueio = &Bloqueio{Motivo: "site-externo", Pendencia: pendencia, Em: agora.Format(time.RFC3339)}
	novo, o := salvarCandidatura(e, i, c, agora)
	if v := vagaDe(novo, c.VagaID); v != nil && !FoiEnviada(v) {
		movido, _, err := MoverVaga(novo, c.VagaID, "bloqueada", Objeto{"pendencia": pendencia}, ctx)
		if err != nil {
			return e, nil, err
		}
		novo = movido
	}
	return novo, o, nil
}

// TornarIncerta (M3): enviando → incerta quando o executor clicou em enviar e não sabe se foi. Nunca reenvia.
func TornarIncerta(e Estado, id string, agora time.Time) (Estado, Objeto, error) {
	i, c, err := acharCandidatura(e, id)
	if err != nil {
		return e, nil, err
	}
	if c.Estado != CandEnviando {
		return e, nil, conflitoEstado(c, "marcar como incerta")
	}
	c.Estado = CandIncerta
	c.Resultado = &Resultado{Em: agora.Format(time.RFC3339), Tipo: "incerto"}
	novo, o := salvarCandidatura(e, i, c, agora)
	return novo, o, nil
}

// MarcarIncertas (M3): ao abrir o app, toda candidatura que ficou em enviando vai para incerta. Nunca há nova
// tentativa automática; ela sai de incerta só pela pessoa (concluir-manual ou reabrir).
func MarcarIncertas(e Estado, agora time.Time) (Estado, int) {
	novo, n := e.Clonar(), 0
	for i, o := range novo.Candidaturas {
		c, err := candidaturaDe(o)
		if err != nil || c.Estado != CandEnviando {
			continue
		}
		c.Estado = CandIncerta
		c.Resultado = &Resultado{Em: agora.Format(time.RFC3339), Tipo: "incerto"}
		c.Versao++
		c.AtualizadaEm = agora.Format(time.RFC3339)
		novo.Candidaturas[i] = c.objeto()
		n++
	}
	return novo, n
}
