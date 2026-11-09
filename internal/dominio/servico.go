package dominio

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
)

// Operações sobre o estado da Central (porte de nucleo/servico.mjs). Todas são puras: recebem o
// estado, devolvem um estado novo e nunca alteram o original. Tela, CLI e MCP usam as mesmas.

// Estado são todos os dados de uma conta.
type Estado struct {
	Vagas       []Objeto `json:"vagas"`
	Eventos     []Objeto `json:"eventos"`
	Plataformas []Objeto `json:"plataformas"`
	Rodadas     []Objeto `json:"rodadas"`
	Perfil      Objeto   `json:"perfil"`
	Pedidos     []Objeto `json:"pedidos"`
	Emails      []Objeto `json:"emails"`
}

// EstadoVazio é o estado de uma conta nova.
func EstadoVazio() Estado {
	return Estado{Vagas: []Objeto{}, Eventos: []Objeto{}, Plataformas: []Objeto{}, Rodadas: []Objeto{}, Perfil: Objeto{}, Pedidos: []Objeto{}, Emails: []Objeto{}}
}

func clonarLista(l []Objeto) []Objeto {
	if l == nil {
		return nil
	}
	c := make([]Objeto, len(l))
	for i, o := range l {
		c[i] = clonarObj(o)
	}
	return c
}

// Clonar copia o estado inteiro (nada é compartilhado com o original).
func (e Estado) Clonar() Estado {
	return Estado{clonarLista(e.Vagas), clonarLista(e.Eventos), clonarLista(e.Plataformas), clonarLista(e.Rodadas), clonarObj(e.Perfil), clonarLista(e.Pedidos), clonarLista(e.Emails)}
}

// Ctx é o contexto de cada operação: quem fez, quando e de qual conta.
type Ctx struct {
	Autor        string // "eu" (tela) ou "claude" (assistente)
	ContaID      string // dono dos dados; vazio = "local". Nunca vem do corpo do pedido.
	Agora        string // AAAA-MM-DD
	DiasFollowUp int    // 0 = 7
	Texto        string // texto opcional (follow-up)
	Aleatorio    func() string
}

func (c Ctx) conta() string {
	if c.ContaID == "" {
		return "local"
	}
	return c.ContaID
}

func (c Ctx) dias() int {
	if c.DiasFollowUp == 0 {
		return 7
	}
	return c.DiasFollowUp
}

const base36 = "0123456789abcdefghijklmnopqrstuvwxyz"

// sufixo são 4 letras aleatórias que deixam o id único mesmo com dois registros no mesmo dia.
func (c Ctx) sufixo() string {
	if c.Aleatorio != nil {
		return c.Aleatorio()
	}
	b := make([]byte, 4)
	rand.Read(b)
	for i := range b {
		b[i] = base36[int(b[i])%36]
	}
	return string(b)
}

// ErroValidacao lista o que está errado nos dados.
type ErroValidacao struct{ Erros []string }

func (e *ErroValidacao) Error() string { return strings.Join(e.Erros, "; ") }

// ErroDuplicata diz que a vaga já existe.
type ErroDuplicata struct{ Existente Objeto }

func (e *ErroDuplicata) Error() string {
	x := e.Existente
	return fmt.Sprintf("já existe: %s · %s (%s, etapa %s)", campo(x, "empresa"), campo(x, "titulo"), campo(x, "id"), campo(x, "etapa"))
}

// ErroNaoEncontrado diz que a vaga, o pedido ou o email não existe.
type ErroNaoEncontrado struct{ Msg string }

func (e *ErroNaoEncontrado) Error() string { return e.Msg }

func validacao(erros ...string) error { return &ErroValidacao{erros} }

var erroDocumento = "parece conter CPF/RG — documentos não são guardados na central"

// Campos que a tela pode editar livremente.
var editaveis = []string{"titulo", "empresa", "link", "plataforma", "nivel", "contrato", "modelo", "stack", "ingles", "idioma", "aderencia", "motivoClaude", "notas", "subEtapa", "local", "mercado"}

func exigirValida(v Objeto) error {
	if erros := ValidarVaga(v); len(erros) > 0 {
		return &ErroValidacao{erros}
	}
	return nil
}

func acharIndice(e Estado, id any) (int, error) {
	for i, v := range e.Vagas {
		if igual(v["id"], id) {
			return i, nil
		}
	}
	return -1, &ErroNaoEncontrado{"vaga não encontrada: " + jsString(id)}
}

func novoEvento(e Estado, vagaID any, tipo, txt any, ctx Ctx, agora string) Objeto {
	n := len(e.Eventos) + 1
	return Objeto{
		"id": fmt.Sprintf("ev-%s-%06d-%s", agora, n, ctx.sufixo()), "contaId": ctx.conta(), "vagaId": vagaID,
		"tipo": tipo, "data": agora, "autor": ctx.Autor, "texto": ou(txt, ""),
	}
}

// AdicionarVaga grava uma vaga nova e o evento de descoberta (ou de envio, se já veio enviada).
// forcar aceita empresa+título repetidos quando o id é outro (vaga republicada); nunca aceita o mesmo id.
func AdicionarVaga(e Estado, dados any, ctx Ctx, forcar bool) (Estado, Objeto, error) {
	d, ok := dados.(map[string]any)
	if !ok {
		return e, nil, validacao("vaga vazia ou inválida")
	}
	forcar = forcar || verdadeiro(d["forcar"])
	vaga := Objeto{"plataforma": "outra", "etapa": "descoberta", "descobertaEm": ctx.Agora, "atualizadoEm": ctx.Agora}
	for k, v := range d {
		if k != "forcar" && k != "contaId" {
			vaga[k] = clonar(v)
		}
	}
	vaga["contaId"] = ctx.conta()
	if err := exigirValida(vaga); err != nil {
		return e, nil, err
	}
	var dup Objeto
	for _, v := range e.Vagas {
		if igual(v["id"], vaga["id"]) {
			dup = v
			break
		}
	}
	if dup == nil && !forcar {
		dup = AcharDuplicata(e.Vagas, vaga)
	}
	if dup != nil {
		return e, nil, &ErroDuplicata{clonarObj(dup)}
	}
	novo := e.Clonar()
	novo.Vagas = append(novo.Vagas, vaga)
	dataEvento := texto(ou(obj(vaga["envio"])["data"], ou(vaga["descobertaEm"], ctx.Agora)))
	novo.Eventos = append(novo.Eventos, novoEvento(novo, vaga["id"], vaga["etapa"], ou(d["motivoClaude"], d["notas"]), ctx, dataEvento))
	return novo, vaga, nil
}

// MoverVaga muda a etapa. Só motivo, pendência, subetapa, envio e texto podem vir junto (em extras).
func MoverVaga(e Estado, id any, etapa any, extras Objeto, ctx Ctx) (Estado, Objeto, error) {
	if !contem(Etapas, etapa) {
		return e, nil, validacao("etapa desconhecida: " + jsString(etapa))
	}
	if extras == nil {
		extras = Objeto{}
	}
	novo := e.Clonar()
	i, err := acharIndice(novo, id)
	if err != nil {
		return e, nil, err
	}
	antes := novo.Vagas[i]
	v := copiaRasa(antes)
	v["etapa"], v["atualizadoEm"] = etapa, ctx.Agora
	for _, k := range []string{"motivo", "pendencia", "subEtapa"} {
		if x, tem := extras[k]; tem {
			v[k] = clonar(x)
		}
	}
	if etapa != "bloqueada" {
		v["pendencia"] = clonar(ou(extras["pendencia"], nil))
	}
	if etapa != "pulada" && etapa != "encerrada" && !verdadeiro(extras["motivo"]) {
		v["motivo"] = nil
	}
	if _, tem := extras["subEtapa"]; etapa != "em-processo" && etapa != "oferta" && !tem {
		v["subEtapa"] = nil
	}
	envioExtra := obj(extras["envio"])
	if envioExtra != nil {
		envio := copiaRasa(obj(antes["envio"]))
		for k, x := range envioExtra {
			envio[k] = clonar(x)
		}
		v["envio"] = envio
	}
	dataPedida := ou(envioExtra["data"], ctx.Agora)
	if etapa == "enviada" && !verdadeiro(obj(antes["envio"])["data"]) {
		envio := copiaRasa(obj(v["envio"]))
		envio["data"] = dataPedida
		v["envio"] = envio
		v["followUp"] = Objeto{"proximo": SomarDias(texto(envio["data"]), ctx.dias()), "feitos": float64(0)}
	}
	// Entrar num processo sem envio registrado (ex.: recrutador chamou) conta como candidatura nesse dia.
	if (etapa == "em-processo" || etapa == "oferta") && !verdadeiro(obj(v["envio"])["data"]) {
		envio := copiaRasa(obj(v["envio"]))
		envio["data"] = dataPedida
		v["envio"] = envio
	}
	if ContemDocumento(extras["texto"]) {
		return e, nil, validacao(erroDocumento)
	}
	if err := exigirValida(v); err != nil {
		return e, nil, err
	}
	novo.Vagas[i] = v
	txt := ouV(extras["motivo"], ouV(extras["pendencia"], ouV(extras["texto"], "")))
	novo.Eventos = append(novo.Eventos, novoEvento(novo, id, etapa, txt, ctx, ctx.Agora))
	return novo, v, nil
}

// AtualizarVaga muda campos editáveis. Se patch traz "notasAntes", recusa quando as notas mudaram no meio da edição.
func AtualizarVaga(e Estado, id any, patch Objeto, ctx Ctx) (Estado, Objeto, error) {
	novo := e.Clonar()
	i, err := acharIndice(novo, id)
	if err != nil {
		return e, nil, err
	}
	if antes, tem := patch["notasAntes"]; tem && !igual(ou(novo.Vagas[i]["notas"], ""), ou(antes, "")) {
		return e, nil, validacao("as notas mudaram enquanto você editava — recarregue a vaga e junte as duas versões")
	}
	v := copiaRasa(novo.Vagas[i])
	for k, x := range patch {
		if contem(editaveis, k) {
			v[k] = clonar(x)
		}
	}
	v["atualizadoEm"] = ctx.Agora
	if err := exigirValida(v); err != nil {
		return e, nil, err
	}
	novo.Vagas[i] = v
	return novo, v, nil
}

// RegistrarEvento acrescenta uma linha na linha do tempo da vaga.
func RegistrarEvento(e Estado, id any, tipo, txt any, ctx Ctx) (Estado, Objeto, error) {
	if t, ok := tipo.(string); !ok || aparar(t) == "" {
		return e, nil, validacao("tipo do evento deve ser texto")
	}
	if _, ok := txt.(string); txt != nil && !ok {
		return e, nil, validacao("texto do evento deve ser texto")
	}
	if ContemDocumento(txt) {
		return e, nil, validacao(erroDocumento)
	}
	novo := e.Clonar()
	if _, err := acharIndice(novo, id); err != nil {
		return e, nil, err
	}
	ev := novoEvento(novo, id, tipo, txt, ctx, ctx.Agora)
	novo.Eventos = append(novo.Eventos, ev)
	return novo, ev, nil
}

// FollowUpFeito conta o follow-up e agenda o próximo.
func FollowUpFeito(e Estado, id any, ctx Ctx) (Estado, Objeto, error) {
	if ContemDocumento(ctx.Texto) {
		return e, nil, validacao(erroDocumento)
	}
	novo := e.Clonar()
	i, err := acharIndice(novo, id)
	if err != nil {
		return e, nil, err
	}
	v := novo.Vagas[i]
	feitos, _ := numeroDe(ou(obj(v["followUp"])["feitos"], float64(0)))
	v["followUp"] = Objeto{"proximo": SomarDias(ctx.Agora, ctx.dias()), "feitos": feitos + 1}
	v["atualizadoEm"] = ctx.Agora
	novo.Eventos = append(novo.Eventos, novoEvento(novo, id, "follow-up", ouV(ctx.Texto, "follow-up feito"), ctx, ctx.Agora))
	return novo, v, nil
}

// AdiarFollowUp empurra o próximo follow-up em alguns dias a partir de hoje.
func AdiarFollowUp(e Estado, id any, dias int, ctx Ctx) (Estado, Objeto, error) {
	novo := e.Clonar()
	i, err := acharIndice(novo, id)
	if err != nil {
		return e, nil, err
	}
	v := novo.Vagas[i]
	v["followUp"] = Objeto{"feitos": ou(obj(v["followUp"])["feitos"], float64(0)), "proximo": SomarDias(ctx.Agora, dias)}
	v["atualizadoEm"] = ctx.Agora
	return novo, v, nil
}

// Pedidos: o que a pessoa digita na caixa "O que vamos fazer hoje?" e o assistente ainda não fez.

// CriarPedido guarda um pedido aberto.
func CriarPedido(e Estado, txt any, ctx Ctx) (Estado, Objeto, error) {
	t := aparar(texto(txt))
	switch {
	case t == "":
		return e, nil, validacao("escreva o pedido")
	case tamanhoJS(t) > 500:
		return e, nil, validacao("pedido longo demais (máximo 500 caracteres)")
	case ContemDocumento(t):
		return e, nil, validacao(erroDocumento)
	}
	novo := e.Clonar()
	if novo.Pedidos == nil {
		novo.Pedidos = []Objeto{}
	}
	p := Objeto{
		"id": fmt.Sprintf("pd-%s-%04d-%s", ctx.Agora, len(novo.Pedidos)+1, ctx.sufixo()), "contaId": ctx.conta(),
		"texto": t, "autor": ctx.Autor, "criadoEm": ctx.Agora, "status": "aberto",
	}
	novo.Pedidos = append(novo.Pedidos, p)
	return novo, p, nil
}

// ConcluirPedido marca o pedido como feito, com a resposta curta do assistente.
func ConcluirPedido(e Estado, id any, resposta any, ctx Ctx) (Estado, Objeto, error) {
	if _, ok := resposta.(string); resposta != nil && !ok {
		return e, nil, validacao("resposta deve ser texto")
	}
	if ContemDocumento(resposta) {
		return e, nil, validacao(erroDocumento)
	}
	novo := e.Clonar()
	for _, p := range novo.Pedidos {
		if igual(p["id"], id) {
			p["status"], p["resposta"], p["concluidoEm"] = "feito", ou(resposta, ""), ctx.Agora
			return novo, p, nil
		}
	}
	return e, nil, &ErroNaoEncontrado{"pedido não encontrado: " + jsString(id)}
}

// ---------- emails ----------
// Guardamos só o necessário: id da mensagem (para não processar de novo), remetente, assunto e um resumo curto.
// O corpo do email nunca fica na Central.

func exigirEmail(email, cls Objeto) []string {
	if email == nil {
		return []string{"email vazio"}
	}
	erros := []string{}
	if aparar(texto(email["idMensagem"])) == "" {
		erros = append(erros, "email.idMensagem é obrigatório (evita processar o mesmo email duas vezes)")
	}
	if !contem(TiposEmail, cls["tipo"]) {
		erros = append(erros, "classificacao.tipo deve ser "+strings.Join(TiposEmail, ", "))
	}
	for _, c := range []struct {
		nome  string
		valor any
		max   int
	}{{"resumo", cls["resumo"], 300}, {"proximoPasso", cls["proximoPasso"], 300}, {"assunto", email["assunto"], 300}, {"de", email["de"], 200}} {
		if _, ok := c.valor.(string); c.valor != nil && !ok {
			erros = append(erros, c.nome+" deve ser texto")
		} else if tamanhoJS(texto(c.valor)) > c.max {
			erros = append(erros, fmt.Sprintf("%s longo demais (máximo %d)", c.nome, c.max))
		}
	}
	if ContemDocumento(texto(email["assunto"]) + " " + texto(cls["resumo"]) + " " + texto(cls["proximoPasso"])) {
		erros = append(erros, erroDocumento)
	}
	return erros
}

// Registra o email na linha do tempo da vaga e aplica a transição (recusa → encerrada, convite → entrevista...).
func aplicarEmailNaVaga(e Estado, vagaID any, reg Objeto, ctx Ctx) (Estado, any, error) {
	tipo := texto(reg["tipo"])
	txt := aparar(NomeEmail[tipo] + ": " + texto(ouV(reg["resumo"], ouV(reg["assunto"], ""))))
	e, _, err := RegistrarEvento(e, vagaID, "email", txt, ctx)
	if err != nil {
		return e, nil, err
	}
	var vaga Objeto
	for _, v := range e.Vagas {
		if igual(v["id"], vagaID) {
			vaga = v
		}
	}
	t := TransicaoDoEmail(tipo, vaga, reg)
	if t == nil || (igual(vaga["etapa"], t.Etapa) && igual(ou(t.Extras["subEtapa"], nil), ou(vaga["subEtapa"], nil))) {
		return e, nil, nil
	}
	extras := copiaRasa(t.Extras)
	extras["texto"] = `pelo email "` + texto(reg["assunto"]) + `"`
	e, _, err = MoverVaga(e, vagaID, t.Etapa, extras, ctx)
	if err != nil {
		return e, nil, err
	}
	return e, t.Etapa, nil
}

// RegistrarEmail guarda um email de processo seletivo, acha a vaga e aplica a mudança de etapa.
// O mesmo email (idMensagem) duas vezes não duplica nada.
func RegistrarEmail(e Estado, email, cls Objeto, ctx Ctx) (Estado, Objeto, error) {
	if erros := exigirEmail(email, cls); len(erros) > 0 {
		return e, nil, &ErroValidacao{erros}
	}
	idMensagem := aparar(jsString(email["idMensagem"]))
	for _, x := range e.Emails {
		if igual(x["idMensagem"], idMensagem) {
			return e, Objeto{"idMensagem": idMensagem, "status": "duplicado", "vagaId": ou(x["vagaId"], nil), "transicao": nil}, nil
		}
	}
	novo := e.Clonar()
	if novo.Emails == nil {
		novo.Emails = []Objeto{}
	}
	data := DataDoEmail(email["data"], ctx.Agora)
	tipo := cls["tipo"].(string)
	// Fora do pré-filtro e sem cara de processo: só o id, para não ler de novo.
	if tipo == "outro" && !PassaPreFiltro(email) {
		novo.Emails = append(novo.Emails, Objeto{"idMensagem": idMensagem, "contaId": ctx.conta(), "data": data, "status": "ignorado"})
		return novo, Objeto{"idMensagem": idMensagem, "status": "ignorado", "vagaId": nil, "transicao": nil}, nil
	}
	vaga := AcharVagaDoEmail(novo.Vagas, email, cls["vagaId"])
	status := "sem-vaga"
	var vagaID any
	if vaga != nil {
		status, vagaID = "ligado", vaga["id"]
	} else if tipo == "outro" {
		status = "ignorado"
	}
	reg := Objeto{
		"idMensagem": idMensagem, "contaId": ctx.conta(), "data": data, "tipo": tipo, "de": ou(email["de"], ""),
		"assunto": ou(email["assunto"], ""), "resumo": ou(cls["resumo"], ""), "vagaId": vagaID, "status": status, "registradoEm": ctx.Agora,
	}
	if verdadeiro(cls["proximoPasso"]) {
		reg["proximoPasso"] = cls["proximoPasso"]
	}
	novo.Emails = append(novo.Emails, reg)
	var transicao any
	if vaga != nil {
		var err error
		if novo, transicao, err = aplicarEmailNaVaga(novo, vagaID, reg, ctx); err != nil {
			return e, nil, err
		}
	}
	return novo, Objeto{"idMensagem": idMensagem, "status": status, "vagaId": vagaID, "transicao": transicao}, nil
}

// LigarEmail: a pessoa diz de qual vaga é um email "sem vaga"; aí a mudança de etapa acontece.
func LigarEmail(e Estado, idMensagem, vagaID any, ctx Ctx) (Estado, Objeto, error) {
	var reg Objeto
	for _, x := range e.Emails {
		if igual(x["idMensagem"], idMensagem) {
			reg = x
		}
	}
	if reg == nil {
		return e, nil, &ErroNaoEncontrado{"email não encontrado: " + jsString(idMensagem)}
	}
	if verdadeiro(reg["vagaId"]) {
		return e, nil, validacao("este email já está ligado à vaga " + jsString(reg["vagaId"]))
	}
	if _, err := acharIndice(e, vagaID); err != nil {
		return e, nil, err
	}
	novo := e.Clonar()
	var r Objeto
	for _, x := range novo.Emails {
		if igual(x["idMensagem"], idMensagem) && r == nil {
			r = x
		}
	}
	r["vagaId"], r["status"] = vagaID, "ligado"
	novo, transicao, err := aplicarEmailNaVaga(novo, vagaID, r, ctx)
	if err != nil {
		return e, nil, err
	}
	return novo, Objeto{"idMensagem": idMensagem, "status": "ligado", "vagaId": vagaID, "transicao": transicao}, nil
}

// DescartarEmail tira um email da lista "sem vaga".
func DescartarEmail(e Estado, idMensagem any) (Estado, Objeto, error) {
	novo := e.Clonar()
	for _, r := range novo.Emails {
		if igual(r["idMensagem"], idMensagem) {
			r["status"] = "descartado"
			return novo, Objeto{"idMensagem": idMensagem, "status": "descartado"}, nil
		}
	}
	return e, nil, &ErroNaoEncontrado{"email não encontrado: " + jsString(idMensagem)}
}

// EhNaoEncontrado diz se o erro é de algo que não existe (a API responde 404).
func EhNaoEncontrado(err error) bool {
	var n *ErroNaoEncontrado
	return errors.As(err, &n)
}
