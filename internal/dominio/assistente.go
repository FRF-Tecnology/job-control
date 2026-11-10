package dominio

// O assistente pelo MCP e pela CLI (jc-a2y.7.4, docs/propostas/mcp-cli.md, regra M4). O canal é fixado pelo binário
// (nunca pelo pedido), vai no Ctx e fica gravado no evento e em quem pausou ou revogou. O autor do assistente é sempre
// "claude": com um canal de assistente, o Ctx nunca é da pessoa, qualquer que seja o Autor.

// Canais.
const (
	CanalTela = "tela"
	CanalMCP  = "mcp"
	CanalCLI  = "cli"
)

// CanalDoAssistente: mcp ou cli.
func CanalDoAssistente(canal string) bool { return canal == CanalMCP || canal == CanalCLI }

// Quem fez uma ação que reduz o risco (pausar, revogar): a pessoa pela tela, ou o assistente por um canal. As únicas
// combinações são eu/tela, claude/mcp e claude/cli.
type Quem struct {
	Autor string `json:"autor"`
	Canal string `json:"canal"`
}

// QuemDe: quem está chamando, pelo Ctx que o servidor montou (nunca pelo corpo). nil quando não dá para dizer (o
// assistente por um caminho antigo, sem canal).
func QuemDe(ctx Ctx) *Quem {
	switch {
	case CanalDoAssistente(ctx.Canal):
		return &Quem{Autor: "claude", Canal: ctx.Canal}
	case ctx.EhPessoa():
		return &Quem{Autor: "eu", Canal: CanalTela}
	}
	return nil
}

// QuemValido: só as três combinações (o que vem do banco é conferido de novo).
func QuemValido(q *Quem) bool {
	return q != nil && ((q.Autor == "eu" && q.Canal == CanalTela) || (q.Autor == "claude" && CanalDoAssistente(q.Canal)))
}

// candidaturaAtiva: a vaga tem uma candidatura que o envio ainda vai usar (preparada, aprovada ou enviando).
func candidaturaAtiva(e Estado, vagaID any) bool {
	for _, o := range e.Candidaturas {
		if !igual(o["vagaId"], vagaID) {
			continue
		}
		switch texto(o["estado"]) {
		case CandPreparada, CandAprovada, CandEnviando:
			return true
		}
	}
	return false
}

// camposDoEnvio: o que define para onde o envio vai e o que a pessoa aprovou (requisito A3 do CyberSec).
var camposDoEnvio = []string{"link", "plataforma", "empresa", "titulo"}

// conferirMoverDoAssistente: só o executor e a pessoa (concluir-manual) registram envio (decisão do PO no #135). Pelo
// canal do assistente: enviada e o campo envio dão 403; entrar em processo ou oferta sem envio registrado (o que
// gravaria a data de envio) dá 409.
func conferirMoverDoAssistente(antes Objeto, etapa any, extras Objeto) error {
	if igual(etapa, "enviada") {
		return &ErroSoPessoa{Msg: "só a pessoa registra um envio (concluir manualmente, na tela)"}
	}
	if _, tem := extras["envio"]; tem {
		return &ErroSoPessoa{Msg: "só a pessoa registra um envio (concluir manualmente, na tela)"}
	}
	if (igual(etapa, "em-processo") || igual(etapa, "oferta")) && !FoiEnviada(antes) {
		return &ErroConflito{Msg: "a vaga não tem envio registrado: só a pessoa registra um envio"}
	}
	return nil
}

// conferirEdicaoDoAssistente (A3): com candidatura ativa, o assistente não muda link, plataforma, empresa nem título.
// Um link novo tem de ser da plataforma da vaga, como na candidatura.
func conferirEdicaoDoAssistente(e Estado, antes, depois Objeto) error {
	mudou := func(k string) bool { return !igual(antes[k], depois[k]) }
	if candidaturaAtiva(e, antes["id"]) {
		for _, k := range camposDoEnvio {
			if mudou(k) {
				return &ErroConflito{Msg: "a vaga tem candidatura em andamento: " + k + " não muda pelo assistente"}
			}
		}
	}
	if mudou("link") {
		if p := PlataformaDaVaga(depois); contem(PlataformasComLimite, p) && !URLDaPlataforma(p, texto(depois["link"])) {
			return validacao("link: fora do site da plataforma da vaga")
		}
	}
	return nil
}
