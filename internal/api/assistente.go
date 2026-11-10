package api

import (
	"context"
	"errors"
	"log"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/text/unicode/norm"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// A entrada do assistente (jc-a2y.7.4, docs/propostas/mcp-cli.md, regra M4). O MCP e a CLI criam uma API com
// Opcoes.Canal (mcp ou cli) e chamam por aqui. Lista de permissão, negado por padrão: método e rota fora da lista dão
// 403 antes de ler ou gravar qualquer coisa. O casamento é exato, por modelo de rota, sem prefixo. O pedido é
// remontado com autor claude e sem os cabeçalhos da tela. As regras do domínio continuam valendo por cima.

// rotasDoAssistente: a lista x-assistente do api/openapi.yaml (o teste confere que é a mesma).
var rotasDoAssistente = []string{
	"GET /resumo",
	"GET /vagas",
	"GET /vagas/{id}",
	"GET /duplicatas",
	"POST /vagas/importar",
	"POST /vagas/{id}/mover",
	"PATCH /vagas/{id}",
	"POST /vagas/{id}/eventos",
	"GET /followups",
	"POST /vagas/{id}/followup",
	"PUT /rodadas/{n}",
	"GET /perfil",
	"GET /pedidos",
	"POST /pedidos/{id}/concluir",
	"GET /candidaturas",
	"GET /candidaturas/{id}",
	"GET /candidaturas/limites",
	"POST /candidaturas/{id}/revogar",
	"GET /envio",
	"POST /envio/pausar",
	"POST /envio/plataformas/{plataforma}/pausar",
}

const msgForaDoAssistente = "o assistente não pode fazer isso: só a pessoa, pela tela"

// reSegmento: um parâmetro de rota (id de vaga, de candidatura, de pedido, número da rodada, plataforma). Nunca começa
// com ponto (nem "." nem ".."), e nada de barra, %, espaço ou caractere de controle.
var reSegmento = regexp.MustCompile(`^[A-Za-z0-9_~:@-][A-Za-z0-9._~:@-]{0,199}$`)

// rotaDoAssistente devolve o modelo da lista que casa exatamente com o método e o caminho (sem o prefixo e sem a
// consulta), ou ok false. Se dois casam (/candidaturas/limites e /candidaturas/{id}), vale o de menos parâmetros.
func rotaDoAssistente(metodo, caminho string) (string, bool) {
	partes := strings.Split(caminho, "/")
	melhor, params := "", -1
	for _, r := range rotasDoAssistente {
		m, modelo, _ := strings.Cut(r, " ")
		if m != metodo {
			continue
		}
		mod := strings.Split(modelo, "/")
		if len(mod) != len(partes) {
			continue
		}
		casou, n := true, 0
		for i := range mod {
			if strings.HasPrefix(mod[i], "{") {
				casou = casou && reSegmento.MatchString(partes[i])
				n++
			} else {
				casou = casou && mod[i] == partes[i]
			}
		}
		if casou && (params < 0 || n < params) {
			melhor, params = r, n
		}
	}
	return melhor, params >= 0
}

// normalizado: para comparar etapa e tipo vindos do assistente (maiúsculas, espaços, formas Unicode compatíveis).
func normalizado(v any) string {
	s, _ := v.(string)
	return strings.ToLower(strings.TrimSpace(norm.NFKC.String(s)))
}

// corpoRecusado: o que o assistente nunca manda, mesmo numa rota da lista. Só o executor e a pessoa (concluir-manual)
// registram envio (decisão do PO no #135): mover para enviada, mandar o campo envio ou registrar um evento com o nome
// de uma etapa dão 403.
func corpoRecusado(modelo string, corpo any) bool {
	c := dominio.ObjetoDe(corpo)
	switch modelo {
	case "POST /vagas/{id}/mover":
		if _, tem := c["envio"]; tem {
			return true
		}
		return normalizado(c["etapa"]) == "enviada"
	case "POST /vagas/{id}/eventos":
		t := normalizado(c["tipo"])
		for _, etapa := range dominio.Etapas {
			if t == etapa {
				return true
			}
		}
	}
	return false
}

// ChamarAssistente atende um pedido do assistente pela lista. Recusa (403) antes de ler ou gravar: API sem canal de
// assistente, caminho com %, barra invertida, barra dupla ou barra no fim, método ou rota fora da lista, e o corpo que
// registraria um envio. O log leva só o canal, o modelo da rota e o status (nunca ids, argumentos nem texto).
func (a *API) ChamarAssistente(ctx context.Context, p Pedido) (Resposta, error) {
	recusa := Resposta{Status: 403, Corpo: Objeto{"erro": msgForaDoAssistente}}
	if !dominio.CanalDoAssistente(a.canal) {
		log.Printf("assistente: recusado (API sem canal de assistente)")
		return recusa, nil
	}
	caminho, bruta, _ := strings.Cut(p.Caminho, "?")
	if !strings.HasPrefix(caminho, Prefixo+"/") || strings.ContainsAny(caminho, "%\\") || strings.Contains(caminho, "//") || strings.HasSuffix(caminho, "/") {
		log.Printf("assistente: %s recusado (caminho fora do formato)", a.canal)
		return recusa, nil
	}
	modelo, ok := rotaDoAssistente(p.Metodo, strings.TrimPrefix(caminho, Prefixo))
	if !ok {
		log.Printf("assistente: %s recusado (fora da lista)", a.canal)
		return recusa, nil
	}
	if !consultaAceita(modelo, bruta) {
		log.Printf("assistente: %s %s recusado (consulta fora da lista)", a.canal, modelo)
		return recusa, nil
	}
	if corpoRecusado(modelo, p.Corpo) {
		log.Printf("assistente: %s %s recusado (registro de envio)", a.canal, modelo)
		return recusa, nil
	}
	// Remontado: autor claude, sem os cabeçalhos da tela (X-Pedido, X-Impressao) e sem marca de janela.
	r, err := a.Chamar(ctx, Pedido{Metodo: p.Metodo, Caminho: p.Caminho, Corpo: p.Corpo, Autor: "claude", doAssistente: true})
	if err == nil {
		log.Printf("assistente: %s %s %d", a.canal, modelo, r.Status)
	} else {
		log.Printf("assistente: %s %s erro (%T)", a.canal, modelo, err)
	}
	return r, err
}

// cofreDoAssistente: o cofre da API do assistente. Recusa tudo, sem tocar no cofre do sistema.
type cofreDoAssistente struct{}

var errCofreDoAssistente = errors.New("o assistente não usa o cofre")

func (cofreDoAssistente) Ler(string) (string, error)  { return "", errCofreDoAssistente }
func (cofreDoAssistente) Gravar(string, string) error { return errCofreDoAssistente }
func (cofreDoAssistente) Apagar(string) error         { return errCofreDoAssistente }

// chavesDaConsulta: as chaves de ?consulta que cada rota da lista lê (baixa do CyberSec no #136). Rota fora daqui não
// aceita consulta nenhuma; chave repetida ou desconhecida dá 403.
var chavesDaConsulta = map[string][]string{
	"GET /vagas":        {"etapa", "plataforma"},
	"GET /duplicatas":   {"empresa", "titulo", "excluir"},
	"GET /pedidos":      {"status"},
	"GET /candidaturas": {"estado"},
	"GET /envio":        {"desde"},
}

func consultaAceita(modelo, bruta string) bool {
	if bruta == "" {
		return true
	}
	valores, err := url.ParseQuery(bruta)
	if err != nil {
		return false
	}
	aceitas := chavesDaConsulta[modelo]
	for k, v := range valores {
		if len(v) != 1 || !slices.Contains(aceitas, k) {
			return false
		}
	}
	return true
}

// ModeloDoAssistente: o modelo da lista que casa com o método e o caminho (relativo ao Prefixo, com ou sem consulta).
// A tabela de ferramentas (internal/assistente) confere por ele que cada ferramenta usa só a rota dela.
func ModeloDoAssistente(metodo, caminho string) (string, bool) {
	c, _, _ := strings.Cut(caminho, "?")
	return rotaDoAssistente(metodo, c)
}
