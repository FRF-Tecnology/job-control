package ia

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"sort"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// funcoes.json é gerado do Node (ferramentas/gerar-golden-ia.mjs): contratos e instruções de cada função.
//
//go:embed funcoes.json
var funcoesJSON []byte

type contrato struct {
	Entrada json.RawMessage `json:"entrada"`
	Saida   json.RawMessage `json:"saida"`
	Sistema string          `json:"sistema"`

	entrada, saida *esquema
	saidaCompacta  string // o esquema de saída como o JSON.stringify(f.saida)
}

var contratos = func() (c struct {
	RegrasFixas string               `json:"regrasFixas"`
	Funcoes     map[string]*contrato `json:"funcoes"`
}) {
	if err := json.Unmarshal(funcoesJSON, &c); err != nil {
		panic("funcoes.json: " + err.Error())
	}
	for nome, f := range c.Funcoes {
		f.entrada, f.saida = lerEsquema(f.Entrada), lerEsquema(f.Saida)
		var b bytes.Buffer
		if err := json.Compact(&b, f.Saida); err != nil {
			panic(nome + ": " + err.Error())
		}
		f.saidaCompacta = b.String()
	}
	return
}()

// Funcoes lista os nomes das funções de IA.
func Funcoes() []string {
	nomes := make([]string, 0, len(contratos.Funcoes))
	for n := range contratos.Funcoes {
		nomes = append(nomes, n)
	}
	sort.Strings(nomes)
	return nomes
}

// EsquemaSaida devolve o esquema JSON de saída da função (para a saída estruturada dos provedores).
func EsquemaSaida(funcao string) json.RawMessage {
	if f := contratos.Funcoes[funcao]; f != nil {
		return json.RawMessage(f.saidaCompacta)
	}
	return nil
}

// Só o que a IA precisa do perfil (menos tokens, nada de caminhos de arquivo), nesta ordem.
var camposPerfil = []string{"cargo", "experienciaAnos", "stackPrincipal", "stackSecundaria", "ingles", "formacao", "cidade", "contratos",
	"pretensaoCLT", "pretensaoPJ", "pretensaoUSD", "inicio", "filtros", "prioridadeVagas", "respostasPadrao", "nome", "linkedin", "github"}

func resumoPerfil(perfil dominio.Objeto) *ordenado {
	o := novoOrdenado()
	for _, k := range camposPerfil {
		if v, tem := perfil[k]; tem && v != nil && v != "" {
			o.por(k, v)
		}
	}
	return o
}

func j(v any) string { return stringify(v, " ") }

// Prompt é o que vai ao provedor.
type Prompt struct {
	Sistema     string          `json:"sistema"`
	Mensagem    string          `json:"mensagem"`
	Esquema     json.RawMessage `json:"esquema"`
	NomeEsquema string          `json:"nomeEsquema"`
}

func mensagem(funcao string, e dominio.Objeto) string {
	c := func(k string) string { return dominio.CampoJS(e, k) }
	perfil := j(resumoPerfil(dominio.ObjetoDe(e["perfil"])))
	curriculo := dominio.Texto(dominio.OuV(e["curriculo"], "(não informado)"))
	switch funcao {
	case "triarVaga":
		return "Vaga:\n" + j(e["vaga"]) + "\n\nPerfil:\n" + perfil
	case "responderPergunta":
		partes := []string{"Pergunta: " + c("pergunta"), "Tipo do campo: " + c("tipoCampo")}
		if l := dominio.ListaDe(e["opcoes"]); len(l) > 0 {
			partes = append(partes, "Opções: "+j(l))
		}
		partes = append(partes, "Perfil:\n"+perfil, "Currículo:\n"+curriculo)
		return strings.Join(partes, "\n\n")
	case "revisarAntesDeEnviar":
		return "Formulário:\n" + j(e["formulario"]) + "\n\nPerfil:\n" + perfil + "\n\nCurrículo:\n" + curriculo
	case "classificarEmail":
		vagas := []any{}
		for _, x := range dominio.ListaDe(e["vagas"]) {
			v := dominio.ObjetoDe(x)
			o := novoOrdenado()
			for _, k := range []string{"id", "empresa", "titulo"} {
				if val, tem := v[k]; tem { // undefined some do JSON.stringify
					o.por(k, val)
				}
			}
			vagas = append(vagas, o)
		}
		return "Email:\n" + j(e["email"]) + "\n\nVagas em andamento:\n" + j(vagas)
	case "avaliarEntrevista":
		return "Vaga:\n" + j(e["vaga"]) + "\n\nTranscript/notas:\n" + c("transcript")
	case "escreverMensagem":
		return "Tipo: " + c("tipo") + "\nIdioma: " + c("idioma") + "\n\nVaga:\n" + j(e["vaga"]) + "\n\nPerfil:\n" + perfil
	}
	return ""
}

// MontarPrompt junta regras fixas + instrução da função + esquema (para provedores sem saída estruturada).
func MontarPrompt(funcao string, entrada dominio.Objeto) Prompt {
	f := contratos.Funcoes[funcao]
	return Prompt{
		Sistema:     f.Sistema + "\n\nRegras que não mudam:\n" + contratos.RegrasFixas + "\n\nResponda apenas com um objeto JSON válido neste esquema:\n" + f.saidaCompacta,
		Mensagem:    mensagem(funcao, entrada),
		Esquema:     json.RawMessage(f.saidaCompacta),
		NomeEsquema: funcao,
	}
}
