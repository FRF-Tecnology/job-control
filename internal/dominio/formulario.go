package dominio

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// ErroBloqueioLeitura: a estrutura lida tem o que só a pessoa faz (motivo de PendenciaDoMotivo). Quem lê bloqueia a
// candidatura com esse motivo (BloquearLeitura); nada da estrutura é guardado.
type ErroBloqueioLeitura struct{ Motivo string }

func (e *ErroBloqueioLeitura) Error() string {
	return "o formulário pede o que só a pessoa responde: " + e.Motivo
}

// PerguntaFormulario: uma pergunta lida do formulário (texto da página: dado não confiável, a tela mostra como texto;
// nunca vai para a IA nem para o log).
type PerguntaFormulario struct {
	ID          string   `json:"id"`
	Texto       string   `json:"texto"`
	Tipo        string   `json:"tipo"`
	Obrigatoria bool     `json:"obrigatoria"`
	Opcoes      []string `json:"opcoes,omitempty"`
}

// EstruturaFormulario: a estrutura lida sem interagir. O hash entra na impressão; no envio, o executor relê e compara.
type EstruturaFormulario struct {
	Hash      string               `json:"hash"`
	URLFinal  string               `json:"urlFinal"`
	Perguntas []PerguntaFormulario `json:"perguntas"`
}

// Limites e listas do contrato (Candidatura.estrutura).
const (
	maxPerguntas     = 200
	maxTextoPergunta = 500
	maxOpcoes        = 100
	maxTextoOpcao    = 200
)

var (
	tiposPergunta  = []string{"texto-curto", "texto-longo", "escolha-unica", "multipla-escolha", "lista", "data", "numero", "email", "telefone", "escala", "outro"}
	reIDPergunta   = regexp.MustCompile(`^[A-Za-z0-9._-]{1,80}$`)
	contatoPorTipo = map[string]string{"email": "email", "telefone": "telefone"}
)

// GravarEstruturaFormulario guarda a estrutura lida (pelo executor, sem interagir) numa candidatura de formulário em
// preparada, monta as respostas só por casamento exato com as respostasPadrao do perfil (e-mail e telefone pelo
// contato do pacote), lista as obrigatórias sem resposta em pendentes e recalcula a impressão com a estrutura.
// urlFinal: o destino canônico já resolvido no navegador (o do forms.gle); tem de ser um formulário da lista e, fora
// do forms.gle, o mesmo da candidatura.
func GravarEstruturaFormulario(e Estado, id string, perguntas []PerguntaFormulario, urlFinal string, agora time.Time) (Estado, Objeto, error) {
	i, c, err := acharCandidatura(e, id)
	if err != nil {
		return e, nil, err
	}
	if c.Plataforma != PlataformaFormulario {
		return e, nil, &ErroConflito{Msg: "a candidatura não é de formulário", Estado: c.Estado}
	}
	if c.Estado != CandPreparada {
		return e, nil, conflitoEstado(c, "ler o formulário de")
	}
	if erros := validarPerguntas(perguntas); len(erros) > 0 {
		return e, nil, &ErroValidacao{erros}
	}
	// Pergunta sensível (CPF, RG, senha, banco, documento, dado sensível, termos): o formulário fica com a pessoa.
	// Nunca é preenchida, nem por respostasPadrao (achado do CyberSec no #120): a estrutura nem é guardada.
	// O aceite de termos também aparece só na opção ("Confirmação" com a opção "Li e aceito os termos"): bloqueia igual,
	// já na leitura, como o envio faz (formulario_envio.go). Nas opções só o aceite conta; outra palavra sensível numa
	// opção (por exemplo "Documento" numa lista de assuntos) não é pedido de dado.
	for _, p := range perguntas {
		if m := MotivoDoRotulo(p.Texto); m != "" {
			return e, nil, &ErroBloqueioLeitura{Motivo: m}
		}
		for _, op := range p.Opcoes {
			if MotivoDoRotulo(op) == "termos" {
				return e, nil, &ErroBloqueioLeitura{Motivo: "termos"}
			}
		}
	}
	final, _, ok := FormularioDoEndereco(urlFinal)
	if !ok || strings.HasPrefix(final, "https://forms.gle/") {
		return e, nil, validacao("o destino do formulário não está na lista de formulários")
	}
	if !strings.HasPrefix(c.URL, "https://forms.gle/") && final != c.URL {
		return e, nil, validacao("o destino do formulário não é o da candidatura")
	}
	hash, err := hashDasPerguntas(perguntas)
	if err != nil {
		return e, nil, err
	}
	c.Estrutura = &EstruturaFormulario{Hash: hash, URLFinal: final, Perguntas: append([]PerguntaFormulario(nil), perguntas...)}
	c.Conteudo = clonarObj(c.Conteudo)
	c.Conteudo["respostas"] = respostasDoFormulario(e.Perfil, c.Conteudo, perguntas)
	c.Pendentes = pendentesDe(c.Estrutura, c.Conteudo)
	if c.Impressao, err = impressaoDe(c, c.Conteudo); err != nil {
		return e, nil, err
	}
	novo, o := salvarCandidatura(e, i, c, agora)
	return novo, o, nil
}

func validarPerguntas(perguntas []PerguntaFormulario) []string {
	erros := []string{}
	if len(perguntas) == 0 || len(perguntas) > maxPerguntas {
		return append(erros, fmt.Sprintf("estrutura.perguntas: de 1 a %d", maxPerguntas))
	}
	vistos := map[string]bool{}
	for n, p := range perguntas {
		switch {
		case !reIDPergunta.MatchString(p.ID) || vistos[p.ID]:
			erros = append(erros, fmt.Sprintf("estrutura.perguntas[%d].id: fora do formato ou repetido", n))
		case strings.TrimSpace(p.Texto) == "" || utf8.RuneCountInString(p.Texto) > maxTextoPergunta:
			erros = append(erros, fmt.Sprintf("estrutura.perguntas[%d].texto: de 1 a %d caracteres", n, maxTextoPergunta))
		case !contem(tiposPergunta, p.Tipo):
			erros = append(erros, fmt.Sprintf("estrutura.perguntas[%d].tipo: fora da lista", n))
		case len(p.Opcoes) > maxOpcoes:
			erros = append(erros, fmt.Sprintf("estrutura.perguntas[%d].opcoes: até %d", n, maxOpcoes))
		}
		for _, o := range p.Opcoes {
			if utf8.RuneCountInString(o) > maxTextoOpcao {
				erros = append(erros, fmt.Sprintf("estrutura.perguntas[%d].opcoes: até %d caracteres cada", n, maxTextoOpcao))
				break
			}
		}
		vistos[p.ID] = true
	}
	return erros
}

func perguntasComoLista(perguntas []PerguntaFormulario) []any {
	l := make([]any, 0, len(perguntas))
	for _, p := range perguntas {
		o := Objeto{"id": p.ID, "texto": p.Texto, "tipo": p.Tipo, "obrigatoria": p.Obrigatoria}
		if len(p.Opcoes) > 0 {
			ops := make([]any, len(p.Opcoes))
			for k, op := range p.Opcoes {
				ops[k] = op
			}
			o["opcoes"] = ops
		}
		l = append(l, o)
	}
	return l
}

// normalizarPergunta: o "texto normalizado igual" do casamento exato: minúsculas, espaços juntos e sem a pontuação
// do fim. Nada de parecido: ou é a mesma pergunta, ou fica pendente.
func normalizarPergunta(s string) string {
	s = strings.Join(strings.Fields(strings.ToLower(s)), " ")
	return strings.TrimRight(s, "?:. ")
}

// respostasDoFormulario: as respostas do pacote para as perguntas lidas. Fica a resposta que a pessoa já deu para a
// pergunta (perguntaId); senão, a respostasPadrao de texto igual. Respostas sem perguntaId (de antes) ficam como estão.
func respostasDoFormulario(perfil, conteudo Objeto, perguntas []PerguntaFormulario) []any {
	padrao := map[string]string{}
	for _, r := range lista(ObjetoDe(perfil)["respostasPadrao"]) {
		ro := ObjetoDe(r)
		if p, ok := ro["pergunta"].(string); ok {
			if v, ok := ro["resposta"].(string); ok && v != "" {
				padrao[normalizarPergunta(p)] = v
			}
		}
	}
	daPessoa := map[string]Objeto{}
	saida := []any{}
	for _, r := range lista(conteudo["respostas"]) {
		ro := ObjetoDe(r)
		pid := texto(ro["perguntaId"])
		switch {
		case pid == "":
			saida = append(saida, r)
		case texto(ro["origem"]) == "pessoa":
			daPessoa[pid] = ro
		}
	}
	for _, p := range perguntas {
		if ro, ok := daPessoa[p.ID]; ok {
			saida = append(saida, ro)
			continue
		}
		if MotivoDoRotulo(p.Texto) != "" {
			continue // defesa a mais: pergunta sensível nunca recebe resposta automática
		}
		if v, ok := padrao[normalizarPergunta(p.Texto)]; ok {
			saida = append(saida, Objeto{"pergunta": p.Texto, "resposta": v, "origem": "respostasPadrao", "perguntaId": p.ID})
		}
	}
	return saida
}

// pendentesDe: as obrigatórias sem resposta no pacote (por perguntaId; e-mail e telefone pelo contato).
func pendentesDe(est *EstruturaFormulario, conteudo Objeto) []string {
	if est == nil {
		return nil
	}
	respondidas := map[string]bool{}
	for _, r := range lista(conteudo["respostas"]) {
		ro := ObjetoDe(r)
		if pid := texto(ro["perguntaId"]); pid != "" && strings.TrimSpace(texto(ro["resposta"])) != "" {
			respondidas[pid] = true
		}
	}
	contato := ObjetoDe(conteudo["contato"])
	pendentes := []string{}
	for _, p := range est.Perguntas {
		if !p.Obrigatoria || respondidas[p.ID] {
			continue
		}
		if campo, ok := contatoPorTipo[p.Tipo]; ok && strings.TrimSpace(texto(contato[campo])) != "" {
			continue
		}
		pendentes = append(pendentes, p.ID)
	}
	return pendentes
}

// impressaoDe: a impressão da candidatura com este conteúdo. Formulário com estrutura lida: sha256 do JSON canônico de
// {vagaId, plataforma, url, conteudo, estrutura}; o resto, a fórmula de sempre.
func impressaoDe(c Candidatura, conteudo Objeto) (string, error) {
	if c.Plataforma != PlataformaFormulario || c.Estrutura == nil {
		return ImpressaoCandidatura(c.VagaID, c.Plataforma, c.URL, conteudo)
	}
	est := Objeto{"hash": c.Estrutura.Hash, "urlFinal": c.Estrutura.URLFinal, "perguntas": perguntasComoLista(c.Estrutura.Perguntas)}
	b, err := JSONCanonico(Objeto{"vagaId": c.VagaID, "plataforma": c.Plataforma, "url": c.URL, "conteudo": conteudo, "estrutura": est})
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return "sha256:" + VersaoImpressao + ":" + hex.EncodeToString(h[:]), nil
}

// faltamRespostas: o formulário ainda não pode ser aprovado (sem estrutura lida, ou com pendência).
func faltamRespostas(c Candidatura) bool {
	return c.Plataforma == PlataformaFormulario && (c.Estrutura == nil || len(c.Pendentes) > 0)
}

// Formulários externos (jc-a2y.7.2, docs/propostas/formularios.md): Google Forms e Typeform viram a plataforma
// "formulario". Lista exata de hosts e caminhos, nunca endsWith; o servidor nunca busca a URL (o forms.gle é
// resolvido só no navegador do envio). A URL guardada é a canônica, sem query nem fragmento: o pré-preenchimento
// (entry.* do Google, campos ocultos do Typeform) iria junto no envio sem estar no pacote, então é tirado, e a
// candidatura guarda só o fato (preenchimentoIgnorado), nunca os valores.

// PlataformaFormulario: a plataforma dos formulários externos.
const PlataformaFormulario = "formulario"

var (
	idFormulario      = `[A-Za-z0-9_-]{1,120}`
	caminhoGoogleForm = regexp.MustCompile(`^/forms/d/(e/)?` + idFormulario + `/viewform$`)
	caminhoFormsGle   = regexp.MustCompile(`^/` + idFormulario + `$`)
	caminhoTypeform   = regexp.MustCompile(`^/to/` + idFormulario + `$`)
	hostContaTypeform = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?\.typeform\.com$`)
)

// FormularioDoEndereco: a URL canônica (https, host em minúsculas, sem query nem fragmento) quando o endereço é de um
// formulário da lista exata, e se ele trazia pré-preenchimento. Fora da lista: ok false.
func FormularioDoEndereco(endereco string) (canonica string, preenchido bool, ok bool) {
	u, host, valido := enderecoAnalisado(endereco)
	if !valido || u.RawPath != "" || strings.Contains(u.Path, "..") {
		return "", false, false
	}
	caminho := u.Path
	switch {
	case host == "docs.google.com" && caminhoGoogleForm.MatchString(caminho):
		for chave := range u.Query() {
			if strings.HasPrefix(chave, "entry.") {
				preenchido = true
			}
		}
	case host == "forms.gle" && caminhoFormsGle.MatchString(caminho):
		preenchido = temCampoDeFora(u)
	case (host == "form.typeform.com" || hostContaTypeform.MatchString(host)) && caminhoTypeform.MatchString(caminho):
		preenchido = temCampoDeFora(u)
	default:
		return "", false, false
	}
	return "https://" + host + caminho, preenchido, true
}

// temCampoDeFora: o link traz valores que iriam para o formulário (query que não é de campanha, ou fragmento, onde o
// Typeform lê os campos ocultos).
func temCampoDeFora(u *url.URL) bool {
	if u.Fragment != "" {
		return true
	}
	for chave := range u.Query() {
		if !strings.HasPrefix(strings.ToLower(chave), "utm_") {
			return true
		}
	}
	return false
}

// LeituraFormulario: o que o executor leu da página do formulário, sem interagir. Bloqueio vem de uma lista fechada
// (PendenciaDoMotivo): login, termos, site-externo, varias-etapas, documento, outro...; com bloqueio, as perguntas
// não são usadas.
type LeituraFormulario struct {
	Perguntas []PerguntaFormulario
	URLFinal  string
	Bloqueio  string
}

// LeiturasNoDia: leituras de formulário hoje na plataforma. Cada uma abre uma página no navegador do envio e conta no
// teto de páginas abertas (AberturasNoDia), não no limite do dia.
func LeiturasNoDia(e Estado, plataforma string, agora time.Time) int {
	n := 0
	for _, o := range e.Candidaturas {
		if !igual(o["plataforma"], plataforma) {
			continue
		}
		for _, l := range lista(o["leituras"]) {
			if quando, err := time.Parse(time.RFC3339, texto(l)); err == nil && mesmoDia(quando.In(agora.Location()), agora) {
				n++
			}
		}
	}
	return n
}

// RegistrarLeitura: antes de abrir o formulário para ler, a leitura fica gravada (conta no teto de 40 páginas abertas
// por plataforma no dia). Só formulário em preparada; no teto, ErroLimite e nada muda.
func RegistrarLeitura(e Estado, id string, agora time.Time) (Estado, Objeto, error) {
	i, c, err := acharCandidatura(e, id)
	if err != nil {
		return e, nil, err
	}
	if c.Plataforma != PlataformaFormulario {
		return e, nil, &ErroConflito{Msg: "a candidatura não é de formulário", Estado: c.Estado}
	}
	if c.Estado != CandPreparada {
		return e, nil, conflitoEstado(c, "ler o formulário de")
	}
	if abertas := AberturasNoDia(e, c.Plataforma, agora); abertas >= LimiteMaximo {
		return e, nil, &ErroLimite{Plataforma: c.Plataforma, Limite: LimiteMaximo, Usadas: abertas, RenovaEm: RenovaEm(agora).Format(time.RFC3339)}
	}
	c.Leituras = append(c.Leituras, agora.Format(time.RFC3339))
	novo, o := salvarCandidatura(e, i, c, agora)
	return novo, o, nil
}

// BloquearLeitura: a leitura achou um bloqueio (login, e-mail registrado, termos, iframe, várias etapas...): a
// candidatura vai de preparada para bloqueada, com a pendência fixa, e a vaga junto.
func BloquearLeitura(e Estado, id, motivo string, agora time.Time, ctx Ctx) (Estado, Objeto, error) {
	_, c, err := acharCandidatura(e, id)
	if err != nil {
		return e, nil, err
	}
	if c.Plataforma != PlataformaFormulario {
		return e, nil, &ErroConflito{Msg: "a candidatura não é de formulário", Estado: c.Estado}
	}
	return bloquear(e, id, motivo, CandPreparada, agora, ctx)
}
