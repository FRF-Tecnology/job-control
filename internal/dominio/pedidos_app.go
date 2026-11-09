package dominio

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Pedidos atendidos pelo próprio app (jc-a2y.7.1, docs/propostas/pedidos-no-app.md). A IA só PROPÕE uma ação do
// catálogo fechado (internal/ia, interpretarPedido, já passada pela guarda). Aqui o servidor:
//   - monta a proposta com frases fixas (nunca com texto da IA no resumo), a chamada exata à rota que já existe e a
//     impressão sha256:v1 de {pedidoId, acao, parametros};
//   - confirma: só com a impressão e a chamada iguais às guardadas, e grava a ação e o pedido feito na MESMA
//     transação (quem chama grava o estado devolvido de uma vez).
//
// Fora do catálogo para sempre: aprovar, retomar, janela, entrar, modo direto, mover para aplicar, apagar, limite e
// chaves. Nenhuma ação daqui chega lá.

// Status do pedido.
const (
	PedidoAberto   = "aberto"
	PedidoProposta = "proposta"
	PedidoRecusado = "recusado"
	PedidoFeito    = "feito"
)

// Frases fixas da recusa pedida pela IA.
var frasesRecusa = map[string]string{
	"so-a-pessoa": "Isso só você faz, na tela do app: aprovar, enviar, retomar o envio, modo direto, apagar, limites e chaves.",
	"fora-do-app": "O app não faz isso.",
	"nao-entendi": "Não entendi o pedido. Tente com outras palavras.",
}

var nomesPlataforma = map[string]string{"linkedin": "LinkedIn", "gupy": "Gupy", "inhire": "InHire", "outra": "outra plataforma"}

// HostDoLink devolve o host do link em minúsculas, sem o ponto final. Em ASCII: a guarda só deixa passar host ASCII
// (o punycode xn-- fica como está, para a pessoa ver).
func HostDoLink(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
}

// PlataformaDoHost: lista exata, a mesma do envio (M5): LinkedIn em linkedin.com, www ou subdomínio de país; Gupy e
// InHire só no host de empresa (os hosts de login, portal, API e o formulário não contam). Fora disso, "outra".
func PlataformaDoHost(host string) string {
	switch {
	case hostLinkedIn.MatchString(host):
		return "linkedin"
	case hostEmpresaGupy.MatchString(host) && !hostsReservados[host]:
		return "gupy"
	case hostEmpresaIH.MatchString(host) && !hostsReservados[host]:
		return "inhire"
	}
	return "outra"
}

// ImpressaoProposta: sha256 do JSON canônico (RFC 8785) de {pedidoId, acao, parametros}, com o prefixo sha256:v1:.
func ImpressaoProposta(pedidoID, acao string, parametros Objeto) (string, error) {
	b, err := JSONCanonico(Objeto{"pedidoId": pedidoID, "acao": acao, "parametros": parametros})
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return "sha256:" + VersaoImpressao + ":" + hex.EncodeToString(h[:]), nil
}

func acharPedido(e Estado, id string) (int, error) {
	for i, p := range e.Pedidos {
		if igual(p["id"], id) {
			return i, nil
		}
	}
	return -1, &ErroNaoEncontrado{"pedido não encontrado: " + id}
}

// candidaturaAtivaDaVaga: a vaga tem candidatura em preparada, aprovada ou enviando na Fila.
func candidaturaAtivaDaVaga(e Estado, vagaID string) bool {
	for _, o := range e.Candidaturas {
		if igual(o["vagaId"], vagaID) && contem([]string{CandPreparada, CandAprovada, CandEnviando}, o["estado"]) {
			return true
		}
	}
	return false
}

func vagaPorID(e Estado, id string) Objeto {
	for _, v := range e.Vagas {
		if igual(v["id"], id) {
			return v
		}
	}
	return nil
}

func descreverVaga(v Objeto) string { return texto(v["empresa"]) + " — " + texto(v["titulo"]) }

// regraDaEtapa: o que impede registrar a etapa (vazio = pode). enviada só a partir de aplicar e sem candidatura ativa
// na Fila (o envio é do executor ou da pessoa, M2).
func regraDaEtapa(e Estado, v Objeto, etapa string) string {
	if etapa == "enviada" {
		if !igual(v["etapa"], "aplicar") {
			return "Só uma vaga em Aplicar pode ser marcada como enviada."
		}
		if candidaturaAtivaDaVaga(e, texto(v["id"])) {
			return "Esta vaga tem uma candidatura na Fila: o envio é pela Fila."
		}
	}
	return ""
}

// InterpretarPedido guarda no pedido aberto a proposta montada a partir da saída da IA (já passada pela guarda):
//   - adicionar-vaga e registrar-etapa: status proposta, com resumo, chamada e impressão (a pessoa confirma);
//   - preparar-pacote e responder: não gravam nada no domínio; o pedido vira feito (abrir a Fila ou a resposta);
//   - recusar, ou ação que não passa nas regras de agora: status recusado, com a frase fixa.
func InterpretarPedido(e Estado, id string, saida Objeto, agora time.Time, ctx Ctx) (Estado, Objeto, error) {
	i, err := acharPedido(e, id)
	if err != nil {
		return e, nil, err
	}
	if !igual(e.Pedidos[i]["status"], PedidoAberto) {
		return e, nil, &ErroConflito{Msg: "o pedido não está aberto"}
	}
	acao := texto(saida["acao"])
	params := clonarObj(ObjetoDe(saida["parametros"]))
	if params == nil {
		params = Objeto{}
	}
	impressao, err := ImpressaoProposta(id, acao, params)
	if err != nil {
		return e, nil, err
	}
	proposta := Objeto{"acao": acao, "parametros": params, "resumo": "", "chamada": nil, "impressao": impressao, "falta": []any{}, "criadaEm": agora.Format(time.RFC3339)}
	status, resultado, resposta := PedidoProposta, Objeto(nil), any(nil)
	recusar := func(frase string) { status, proposta["resumo"] = PedidoRecusado, frase }

	switch acao {
	case "adicionar-vaga":
		link := texto(params["url"])
		host := HostDoLink(link)
		plataforma := PlataformaDoHost(host)
		corpo := Objeto{"link": link}
		for _, k := range []string{"empresa", "titulo"} {
			if v, tem := params[k]; tem && v != nil {
				corpo[k] = v
			}
		}
		proposta["resumo"] = fmt.Sprintf("Adicionar uma vaga em Descoberta com o link de %s (%s).", host, nomesPlataforma[plataforma])
		proposta["chamada"] = Objeto{"metodo": "POST", "caminho": "/vagas", "corpo": corpo}
		proposta["falta"] = []any{"A triagem da vaga continua com você."}
		for _, v := range e.Vagas {
			if aparar(texto(v["link"])) == link {
				recusar("Essa vaga já está na sua lista: " + descreverVaga(v) + ".")
				proposta["chamada"], proposta["falta"] = nil, []any{}
			}
		}
	case "registrar-etapa":
		v := vagaPorID(e, texto(params["vagaId"]))
		etapa := texto(params["etapa"])
		switch {
		case v == nil:
			recusar("A vaga do pedido não está mais na sua lista.")
		case regraDaEtapa(e, v, etapa) != "":
			recusar(regraDaEtapa(e, v, etapa))
		default:
			corpo := Objeto{"etapa": etapa}
			frase := fmt.Sprintf("Mudar a vaga %s de %s para %s", descreverVaga(v), NomeEtapa[texto(v["etapa"])], NomeEtapa[etapa])
			if m, tem := params["motivo"]; tem && m != nil {
				corpo["motivo"] = m
				frase += fmt.Sprintf(" (motivo: %s)", texto(m))
			}
			if s, tem := params["sub"]; tem && s != nil {
				corpo["subEtapa"] = s
				frase += fmt.Sprintf(" (%s)", texto(s))
			}
			proposta["resumo"] = frase + "."
			proposta["chamada"] = Objeto{"metodo": "POST", "caminho": "/vagas/" + texto(v["id"]) + "/mover", "corpo": corpo}
		}
	case "preparar-pacote":
		v := vagaPorID(e, texto(params["vagaId"]))
		if v == nil || !igual(v["etapa"], "aplicar") {
			recusar("Só uma vaga em Aplicar tem pacote para preparar.")
			break
		}
		status = PedidoFeito
		proposta["resumo"] = "Abrir a Fila para preparar o pacote da vaga " + descreverVaga(v) + "."
		proposta["falta"] = []any{"Você revisa e aprova o pacote na Fila."}
		resultado = Objeto{"vagaId": texto(v["id"]), "abrir": "#/fila?preparar=" + url.QueryEscape(texto(v["id"]))}
	case "responder":
		status = PedidoFeito
		proposta["resumo"] = "Resposta da IA (só leitura)."
		resposta = params["texto"]
	case "recusar":
		frase, ok := frasesRecusa[texto(params["motivo"])]
		if !ok {
			frase = frasesRecusa["nao-entendi"]
		}
		recusar(frase)
	default:
		return e, nil, validacao("ação fora do catálogo")
	}

	novo := e.Clonar()
	p := novo.Pedidos[i]
	p["status"], p["proposta"] = status, proposta
	if resultado != nil {
		p["resultado"] = resultado
	}
	if resposta != nil {
		p["resposta"] = resposta
	}
	if status == PedidoFeito {
		p["concluidoEm"] = ctx.Agora
	}
	return novo, clonarObj(p), nil
}

// DescartarProposta: o pedido em proposta volta a aberto, sem a proposta. Qualquer autor (só reduz o risco).
func DescartarProposta(e Estado, id string) (Estado, Objeto, error) {
	i, err := acharPedido(e, id)
	if err != nil {
		return e, nil, err
	}
	if !igual(e.Pedidos[i]["status"], PedidoProposta) {
		return e, nil, &ErroConflito{Msg: "o pedido não tem proposta"}
	}
	novo := e.Clonar()
	p := novo.Pedidos[i]
	p["status"] = PedidoAberto
	delete(p, "proposta")
	return novo, clonarObj(p), nil
}

// ConfirmarProposta grava a ação confirmada pela pessoa e o pedido feito, no mesmo estado novo (uma transação). Exige:
// a pessoa; o pedido em proposta; a impressão recebida igual à guardada e à recalculada dos parâmetros guardados; a
// chamada recebida (método, caminho e corpo canônico) igual à guardada. Qualquer diferença: conflito, nada muda.
// Devolve a vaga criada ou movida.
func ConfirmarProposta(e Estado, id, impressao, metodo, caminho string, corpo any, ctx Ctx) (Estado, Objeto, error) {
	if !ctx.EhPessoa() {
		return e, nil, &ErroSoPessoa{Msg: "só a pessoa confirma a proposta de um pedido"}
	}
	i, err := acharPedido(e, id)
	if err != nil {
		return e, nil, err
	}
	pedido := e.Pedidos[i]
	if !igual(pedido["status"], PedidoProposta) {
		return e, nil, &ErroConflito{Msg: "o pedido não está em proposta"}
	}
	prop := ObjetoDe(pedido["proposta"])
	acao, params := texto(prop["acao"]), ObjetoDe(prop["parametros"])
	recalculada, err := ImpressaoProposta(id, acao, params)
	if err != nil || impressao != texto(prop["impressao"]) || impressao != recalculada {
		return e, nil, &ErroConflito{Msg: "a impressão não é a da proposta"}
	}
	chamada := ObjetoDe(prop["chamada"])
	guardado, err1 := JSONCanonico(chamada["corpo"])
	recebido, err2 := JSONCanonico(corpo)
	if chamada == nil || err1 != nil || err2 != nil || metodo != texto(chamada["metodo"]) || caminho != texto(chamada["caminho"]) || string(guardado) != string(recebido) {
		return e, nil, &ErroConflito{Msg: "a chamada não é a da proposta"}
	}

	var novo Estado
	var vaga Objeto
	switch acao {
	case "adicionar-vaga":
		c := ObjetoDe(corpo)
		link := texto(c["link"])
		dados := Objeto{
			"id": fmt.Sprintf("pd-vaga-%s-%s", ctx.Agora, ctx.sufixo()), "link": link,
			"empresa": ou(c["empresa"], "(empresa a confirmar)"), "titulo": ou(c["titulo"], "(título a confirmar)"),
			"plataforma": PlataformaDoHost(HostDoLink(link)),
			"etapa":      "descoberta", // sempre: a triagem continua com a pessoa
		}
		novo, vaga, err = AdicionarVaga(e, dados, ctx, false)
	case "registrar-etapa":
		vagaID, etapa := texto(params["vagaId"]), texto(params["etapa"])
		v := vagaPorID(e, vagaID)
		if v == nil {
			return e, nil, &ErroNaoEncontrado{"vaga não encontrada: " + vagaID}
		}
		if motivo := regraDaEtapa(e, v, etapa); motivo != "" {
			return e, nil, &ErroConflito{Msg: motivo}
		}
		c := ObjetoDe(corpo)
		extras := Objeto{}
		for _, k := range []string{"motivo", "subEtapa"} {
			if x, tem := c[k]; tem {
				extras[k] = x
			}
		}
		novo, vaga, err = MoverVaga(e, vagaID, etapa, extras, ctx)
	default:
		return e, nil, &ErroConflito{Msg: "a proposta não tem chamada para confirmar"}
	}
	if err != nil {
		return e, nil, err
	}
	j, _ := acharPedido(novo, id)
	p := novo.Pedidos[j]
	p["status"], p["resultado"], p["concluidoEm"] = PedidoFeito, Objeto{"vagaId": texto(vaga["id"])}, ctx.Agora
	return novo, vaga, nil
}
