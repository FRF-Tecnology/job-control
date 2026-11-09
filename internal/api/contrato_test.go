package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// O contrato (api/openapi.yaml) e as rotas da API precisam andar juntos. Este arquivo confere:
//   - toda operação do contrato é atendida pela API, sem erro inesperado;
//   - num caminho do contrato, método fora do contrato dá "rota não encontrada";
//   - rotas escritas em api.go e ia.go têm caminho no contrato;
//   - na sessão gravada do Node, o que a API atendeu está no contrato e o que deu "rota não encontrada" não está;
//   - operação marcada x-proposta: true (contrato primeiro, ainda sem implementação) continua sem rota; quando a
//     implementação chegar, este teste falha até a marca sair.
//
// Limites (o que este teste NÃO garante):
//   - a busca de rotas no código só olha api.go, ia.go, candidaturas.go, dados.go e envio.go e só reconhece os formatos de hoje:
//     `p == "/x"`, `case "/x"` e regexp.MustCompile(`^/x...`). Rota nova escrita de outro jeito,
//     ou noutro arquivo, só é pega se cair num caminho que já está no contrato (2ª checagem);
//   - cada operação é chamada com corpo {} e parâmetros de exemplo: o teste prova que a rota existe,
//     não que entradas, respostas e erros batem com os esquemas do contrato.

const caminhoContrato = "../../api/openapi.yaml"

var metodosHTTP = []string{"GET", "PUT", "POST", "PATCH", "DELETE"}

// Valores de exemplo para os parâmetros de caminho (fictícios).
var exemplosParam = map[string]string{"provedor": "anthropic", "n": "1", "plataforma": "linkedin"}

type operacoes map[string]map[string]bool // caminho do contrato → métodos

var reParam = regexp.MustCompile(`\{([^}/]+)\}`)

// lerContrato devolve as operações já atendidas (sem x-proposta).
func lerContrato(t *testing.T) operacoes {
	t.Helper()
	ativas, _ := lerContratoCompleto(t)
	return ativas
}

// lerContratoCompleto lê a seção paths do openapi.yaml com um parser YAML de verdade e separa as operações
// marcadas x-proposta: true (no contrato, ainda sem implementação).
func lerContratoCompleto(t *testing.T) (ativas, propostas operacoes) {
	t.Helper()
	b, err := os.ReadFile(caminhoContrato)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatalf("openapi.yaml inválido: %v", err)
	}
	ativas, propostas = operacoes{}, operacoes{}
	for caminho, itens := range doc.Paths {
		for chave, no := range itens {
			for _, m := range metodosHTTP {
				if strings.ToUpper(chave) != m {
					continue
				}
				var op struct {
					Proposta bool `yaml:"x-proposta"`
				}
				if err := no.Decode(&op); err != nil {
					t.Fatalf("%s %s: %v", m, caminho, err)
				}
				destino := ativas
				if op.Proposta {
					destino = propostas
				}
				if destino[caminho] == nil {
					destino[caminho] = map[string]bool{}
				}
				destino[caminho][m] = true
			}
		}
	}
	return ativas, propostas
}

func concreto(caminho string) string {
	return reParam.ReplaceAllStringFunc(caminho, func(p string) string {
		if v, tem := exemplosParam[p[1:len(p)-1]]; tem {
			return v
		}
		return "exemplo-1"
	})
}

// tem diz se algum caminho do contrato que casa com p tem o método m.
// Parâmetro casa com um segmento: "/vagas/importar" casa com ele mesmo e com "/vagas/{id}".
func (ops operacoes) tem(p, m string) bool {
	partes := strings.Split(p, "/")
	for modelo, metodos := range ops {
		seg := strings.Split(modelo, "/")
		if len(seg) != len(partes) {
			continue
		}
		igual := true
		for i := range seg {
			if !reParam.MatchString(seg[i]) && seg[i] != partes[i] {
				igual = false
				break
			}
		}
		if igual && metodos[m] {
			return true
		}
	}
	return false
}

func apiDoContrato(t *testing.T, cofre segredos.Cofre) *API {
	t.Helper()
	arm, err := armazem.Abrir(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	// Cofre falso e sem procurar CLIs: o teste não toca no cofre do sistema nem roda programas.
	return Nova(arm, Opcoes{Hoje: func() string { return "2026-11-08" }, Aleatorio: func() string { return "abcd" },
		Cofre: cofre, ProvedoresIA: map[string]ia.Provedor{},
		DetectarCLIs: func(context.Context) []ia.EstadoCLI { return nil }})
}

// rotaInexistente: a API não conhece a rota (diferente de "vaga não encontrada", que é rota atendida).
func rotaInexistente(r Resposta) bool {
	c, _ := r.Corpo.(Objeto)
	return r.Status == 404 && c != nil && c["erro"] == "rota não encontrada"
}

// conferirOperacao chama m no caminho de exemplo do modelo e devolve o problema ("" se bateu com o contrato).
// Erro inesperado (o HTTP responderia 500) é sempre problema: não prova que a rota existe.
func conferirOperacao(ctx context.Context, a *API, ops operacoes, m, modelo string) string {
	p := concreto(modelo)
	r, err := a.Chamar(ctx, Pedido{Metodo: m, Caminho: Prefixo + p, Corpo: Objeto{}})
	if err != nil {
		return fmt.Sprintf("%s %s (chamado como %s): erro inesperado: %v", m, modelo, p, err)
	}
	documentada := ops[modelo][m]
	switch {
	case documentada && rotaInexistente(r):
		return fmt.Sprintf("%s %s está no contrato, mas a API responde \"rota não encontrada\"", m, modelo)
	case !documentada && !rotaInexistente(r) && !ops.tem(p, m):
		return fmt.Sprintf("%s %s é atendida pela API (status %d), mas não está no contrato", m, modelo, r.Status)
	}
	return ""
}

func ordenadas(ops operacoes) []string {
	l := make([]string, 0, len(ops))
	for p := range ops {
		l = append(l, p)
	}
	sort.Strings(l)
	return l
}

func TestContratoLido(t *testing.T) {
	ops := lerContrato(t)
	n := 0
	for _, ms := range ops {
		n += len(ms)
	}
	// Guarda contra a leitura falhar em silêncio: o contrato tem dezenas de operações, e algumas são conhecidas.
	if n < 30 || !ops["/saude"]["GET"] || !ops["/plataformas/{id}"]["PATCH"] || !ops["/ia/chaves/{provedor}"]["DELETE"] {
		t.Fatalf("leitura do contrato suspeita: %d operações, %v", n, ops)
	}
}

func TestContratoContraRotas(t *testing.T) {
	ops := lerContrato(t)
	a := apiDoContrato(t, segredos.Memoria{})
	ctx := context.Background()
	for _, modelo := range ordenadas(ops) {
		for _, m := range metodosHTTP {
			if problema := conferirOperacao(ctx, a, ops, m, modelo); problema != "" {
				t.Error(problema)
			}
		}
	}
}

// Contrato primeiro: toda operação x-proposta ainda não tem rota, e nenhum outro método nesses caminhos tem.
// Quando a implementação chegar, este teste falha até a marca sair do openapi.yaml.
func TestPropostasAindaSemRota(t *testing.T) {
	ativas, propostas := lerContratoCompleto(t)
	if len(propostas) == 0 {
		return
	}
	a := apiDoContrato(t, segredos.Memoria{})
	for _, modelo := range ordenadas(propostas) {
		if ativas[modelo] != nil {
			t.Errorf("%s mistura operações atendidas e propostas; separe antes de implementar", modelo)
		}
		p := concreto(modelo)
		for _, m := range metodosHTTP {
			r, err := a.Chamar(context.Background(), Pedido{Metodo: m, Caminho: Prefixo + p, Corpo: Objeto{}})
			if err != nil || !rotaInexistente(r) {
				t.Errorf("%s %s (chamado como %s) já é atendido (status %d, erro %v): tire x-proposta do contrato", m, modelo, p, r.Status, err)
			}
		}
	}
}

// Cofre que sempre falha: faz uma rota do contrato devolver erro inesperado.
type cofreQuebrado struct{}

var errCofre = errors.New("cofre indisponível (teste)")

func (cofreQuebrado) Ler(string) (string, error)  { return "", errCofre }
func (cofreQuebrado) Gravar(string, string) error { return errCofre }
func (cofreQuebrado) Apagar(string) error         { return errCofre }

// Erro inesperado numa rota do contrato reprova a checagem, com método e rota na mensagem.
func TestErroInesperadoReprovaContrato(t *testing.T) {
	ops := lerContrato(t)
	a := apiDoContrato(t, cofreQuebrado{})
	problema := conferirOperacao(context.Background(), a, ops, "DELETE", "/ia/chaves/{provedor}")
	if !strings.Contains(problema, "DELETE /ia/chaves/{provedor}") || !strings.Contains(problema, "erro inesperado") {
		t.Fatalf("erro do cofre deveria reprovar a operação; problema = %q", problema)
	}
}

// Rotas escritas no código, nos formatos de hoje (ver os limites no topo do arquivo).
var (
	reRotaFixa  = regexp.MustCompile(`(?:p == |case )"(/[^"]*)"`)
	reRotaRegex = regexp.MustCompile("regexp\\.MustCompile\\(`(\\^/[^`]*)`\\)")
)

func TestRotasDoCodigoEstaoNoContrato(t *testing.T) {
	ops := lerContrato(t)
	exemplos := map[string]bool{}
	for _, modelo := range ordenadas(ops) {
		exemplos[concreto(modelo)] = true
	}
	for _, arq := range []string{"api.go", "ia.go", "candidaturas.go", "dados.go", "envio.go"} {
		b, err := os.ReadFile(arq)
		if err != nil {
			t.Fatal(err)
		}
		fonte := string(b)
		achadas := 0
		for _, g := range reRotaFixa.FindAllStringSubmatch(fonte, -1) {
			achadas++
			if ops[g[1]] == nil {
				t.Errorf("%s: rota %q não está no contrato", arq, g[1])
			}
		}
		for _, g := range reRotaRegex.FindAllStringSubmatch(fonte, -1) {
			achadas++
			re := regexp.MustCompile(g[1])
			casou := false
			for e := range exemplos {
				if re.MatchString(e) {
					casou = true
					break
				}
			}
			if !casou {
				t.Errorf("%s: nenhuma rota do contrato casa com %s", arq, g[1])
			}
		}
		if achadas == 0 {
			t.Fatalf("%s: nenhuma rota achada; o padrão do teste ficou velho", arq)
		}
	}
}

func TestSessaoDoNodeBateComContrato(t *testing.T) {
	ops := lerContrato(t)
	b, err := os.ReadFile("testdata/golden-api.json")
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Chamadas []struct {
			Metodo   string `json:"metodo"`
			Caminho  string `json:"caminho"`
			Resposta struct {
				Status int `json:"status"`
				Corpo  any `json:"corpo"`
			} `json:"resposta"`
		} `json:"chamadas"`
	}
	if err := json.Unmarshal(b, &g); err != nil || len(g.Chamadas) == 0 {
		t.Fatalf("golden-api.json: %v (%d chamadas)", err, len(g.Chamadas))
	}
	for _, c := range g.Chamadas {
		p, _, _ := strings.Cut(c.Caminho, "?")
		if !strings.HasPrefix(p, Prefixo+"/") {
			continue // fora de /api/v1: o contrato não cobre
		}
		p = strings.TrimPrefix(p, Prefixo)
		corpo, _ := c.Resposta.Corpo.(map[string]any)
		inexistente := c.Resposta.Status == 404 && corpo != nil && corpo["erro"] == "rota não encontrada"
		noContrato := ops.tem(p, strings.ToUpper(c.Metodo))
		if inexistente && noContrato {
			t.Errorf("%s %s: o Node não conhece, mas está no contrato", c.Metodo, p)
		}
		if !inexistente && !noContrato {
			t.Errorf("%s %s: o Node atende, mas não está no contrato", c.Metodo, p)
		}
	}
}
