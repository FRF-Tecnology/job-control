package dominio

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// Envio de formulários externos (jc-a2y.7.2, parte 3b, docs/propostas/formularios.md). No envio, o executor relê a
// estrutura sem interagir e só preenche o que este plano devolve: a estrutura relida tem de ter o mesmo hash da
// aprovada (senão pergunta-nova), o destino tem de ser o guardado (senão site-externo), e cada valor sai só do pacote.

// RespostaFormulario: o valor do pacote para uma pergunta da estrutura aprovada.
type RespostaFormulario struct {
	Pergunta PerguntaFormulario
	Valor    string
}

// tiposDeOpcao: o valor tem de ser exatamente uma das opções lidas (nunca a mais parecida).
var tiposDeOpcao = map[string]bool{"escolha-unica": true, "lista": true, "escala": true, "multipla-escolha": true}

// hashDasPerguntas: o hash da estrutura (o mesmo do GravarEstruturaFormulario).
func hashDasPerguntas(perguntas []PerguntaFormulario) (string, error) {
	b, err := JSONCanonico(perguntasComoLista(perguntas))
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return "sha256:" + VersaoImpressao + ":" + hex.EncodeToString(h[:]), nil
}

// PlanoDoFormulario confere a estrutura relida no envio (lidas, no destino canônico urlFinal) com a aprovada e
// devolve, na ordem das perguntas, o que vai em cada uma. Pergunta opcional sem resposta fica de fora (em branco).
// motivo != "": um motivo de PendenciaDoMotivo, e nada é preenchido.
func PlanoDoFormulario(o Objeto, lidas []PerguntaFormulario, urlFinal string) (plano []RespostaFormulario, motivo string) {
	c, err := candidaturaDe(o)
	if err != nil || c.Plataforma != PlataformaFormulario || c.Estrutura == nil {
		return nil, "outro"
	}
	if urlFinal != c.Estrutura.URLFinal {
		return nil, "site-externo"
	}
	if len(validarPerguntas(lidas)) > 0 {
		return nil, "outro"
	}
	if h, err := hashDasPerguntas(lidas); err != nil || h != c.Estrutura.Hash {
		return nil, "pergunta-nova" // a estrutura mudou depois da aprovação: a pessoa revisa de novo
	}
	respostas := map[string]string{}
	for _, r := range lista(c.Conteudo["respostas"]) {
		ro := ObjetoDe(r)
		if pid := texto(ro["perguntaId"]); pid != "" {
			if v, ok := ro["resposta"].(string); ok && strings.TrimSpace(v) != "" {
				respostas[pid] = v
			}
		}
	}
	contato := ObjetoDe(c.Conteudo["contato"])
	for _, p := range c.Estrutura.Perguntas {
		if m := MotivoDoRotulo(p.Texto); m != "" {
			return nil, m
		}
		for _, op := range p.Opcoes {
			if MotivoDoRotulo(op) == "termos" { // aceite que aparece só na opção ("Li e aceito os termos")
				return nil, "termos"
			}
		}
		valor, ok := respostas[p.ID]
		if !ok {
			if campo, doContato := contatoPorTipo[p.Tipo]; doContato {
				valor = strings.TrimSpace(texto(contato[campo]))
			}
		}
		if valor == "" {
			if p.Obrigatoria {
				return nil, "pergunta-nova"
			}
			continue
		}
		switch {
		case p.Tipo == "outro":
			return nil, "outro" // tipo que o app não preenche: a pessoa envia
		case tiposDeOpcao[p.Tipo] && !contem(p.Opcoes, valor):
			return nil, "pergunta-nova"
		case p.Tipo == "data" && !dataISO(valor):
			return nil, "pergunta-nova"
		}
		plano = append(plano, RespostaFormulario{Pergunta: p, Valor: valor})
	}
	return plano, ""
}

func dataISO(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}
