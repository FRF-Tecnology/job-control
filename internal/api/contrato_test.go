package api

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// O contrato (api/openapi.yaml) e as rotas da API precisam andar juntos:
//   - toda operação do contrato é atendida pela API;
//   - num caminho do contrato, método fora do contrato dá "rota não encontrada";
//   - toda rota escrita no código (texto fixo ou regex) tem caminho no contrato;
//   - na sessão gravada do Node, o que a API atendeu está no contrato e o que deu "rota não encontrada" não está.

const caminhoContrato = "../../api/openapi.yaml"

var metodosHTTP = []string{"GET", "PUT", "POST", "PATCH", "DELETE"}

// Valores de exemplo para os parâmetros de caminho (fictícios).
var exemplosParam = map[string]string{"provedor": "anthropic", "n": "1"}

type operacoes map[string]map[string]bool // caminho do contrato → métodos

var (
	reLinhaCaminho = regexp.MustCompile(`^  (/[^:\s]*):(.*)$`)
	reLinhaMetodo  = regexp.MustCompile(`^    (get|put|post|patch|delete):`)
	reParam        = regexp.MustCompile(`\{([^}/]+)\}`)
)

// lerContrato lê só a seção paths do openapi.yaml, no formato usado no arquivo:
// "  /caminho: { get: {...}, post: {...} }" ou "  /caminho:" seguido de "    get:".
func lerContrato(t *testing.T) operacoes {
	t.Helper()
	f, err := os.Open(caminhoContrato)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ops := operacoes{}
	emPaths, atual := false, ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := sc.Text()
		if l == "paths:" {
			emPaths = true
			continue
		}
		if !emPaths || strings.TrimSpace(l) == "" || strings.HasPrefix(strings.TrimSpace(l), "#") {
			continue
		}
		if l[0] != ' ' { // próxima seção de topo (components etc.)
			break
		}
		if g := reLinhaCaminho.FindStringSubmatch(l); g != nil {
			atual = g[1]
			if ops[atual] == nil {
				ops[atual] = map[string]bool{}
			}
			for _, m := range metodosEmLinha(g[2]) {
				ops[atual][m] = true
			}
			continue
		}
		if g := reLinhaMetodo.FindStringSubmatch(l); g != nil && atual != "" {
			ops[atual][strings.ToUpper(g[1])] = true
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return ops
}

// metodosEmLinha acha as chaves de método no primeiro nível de "{ get: {...}, put: {...} }", pulando textos entre aspas.
func metodosEmLinha(s string) []string {
	var saida []string
	nivel, aspas, inicio := 0, byte(0), -1
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case aspas != 0:
			if c == aspas {
				aspas = 0
			}
		case c == '\'' || c == '"':
			aspas = c
		case c == '{':
			nivel++
			if nivel == 1 {
				inicio = i + 1
			}
		case c == '}':
			nivel--
		case c == ',' && nivel == 1:
			inicio = i + 1
		case c == ':' && nivel == 1 && inicio >= 0:
			chave := strings.ToUpper(strings.TrimSpace(s[inicio:i]))
			for _, m := range metodosHTTP {
				if chave == m {
					saida = append(saida, m)
				}
			}
			inicio = -1
		}
	}
	return saida
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

func apiDoContrato(t *testing.T) *API {
	t.Helper()
	arm, err := armazem.Abrir(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	// Cofre em memória e sem procurar CLIs: o teste não toca no cofre do sistema nem roda programas.
	return Nova(arm, Opcoes{Hoje: func() string { return "2026-11-08" }, Aleatorio: func() string { return "abcd" },
		Cofre: segredos.Memoria{}, ProvedoresIA: map[string]ia.Provedor{},
		DetectarCLIs: func(context.Context) []ia.EstadoCLI { return nil }})
}

// rotaInexistente: a API não conhece a rota (diferente de "vaga não encontrada", que é rota atendida).
func rotaInexistente(r Resposta) bool {
	c, _ := r.Corpo.(Objeto)
	return r.Status == 404 && c != nil && c["erro"] == "rota não encontrada"
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
	// Guarda contra o leitor quebrar em silêncio: o contrato tem dezenas de operações, e algumas são conhecidas.
	if n < 30 || !ops["/saude"]["GET"] || !ops["/plataformas/{id}"]["PATCH"] || !ops["/ia/chaves/{provedor}"]["DELETE"] {
		t.Fatalf("leitura do contrato suspeita: %d operações, %v", n, ops)
	}
}

func TestContratoContraRotas(t *testing.T) {
	ops := lerContrato(t)
	a := apiDoContrato(t)
	ctx := context.Background()
	for _, modelo := range ordenadas(ops) {
		p := Prefixo + concreto(modelo)
		for _, m := range metodosHTTP {
			r, err := a.Chamar(ctx, Pedido{Metodo: m, Caminho: p, Corpo: Objeto{}})
			atendida := err != nil || !rotaInexistente(r)
			switch {
			case ops[modelo][m] && !atendida:
				t.Errorf("%s %s está no contrato, mas a API responde \"rota não encontrada\"", m, modelo)
			case !ops.tem(concreto(modelo), m) && atendida:
				t.Errorf("%s %s é atendida pela API (status %d), mas não está no contrato", m, modelo, r.Status)
			}
		}
	}
}

// Rotas escritas no código: textos fixos comparados com o caminho e regex de caminho.
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
	for _, arq := range []string{"api.go", "ia.go"} {
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
