package api

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// jc-a2y.7.4 (PR 2): a entrada do assistente (MCP e CLI, regra M4) e os requisitos do CyberSec (A1 a A8, decisões do
// PO no #135). Dados fictícios.

const marcaAssistente = "Marca-Ficticia-Assistente-7c2e" // não pode aparecer no log

// assistenteDe: uma API do assistente (canal fixado aqui, como o binário fará) no mesmo banco da fila.
func assistenteDe(f *filaTeste, canal string) *API {
	a := Nova(f.arm, Opcoes{Hoje: func() string { return "2026-11-09" }, Aleatorio: func() string { return "efgh" }, EnvioLigado: true, Canal: canal})
	a.relogio = f.api.relogio
	return a
}

func chamarAssistente(t *testing.T, a *API, m, caminho string, corpo any) Resposta {
	t.Helper()
	if corpo == nil {
		corpo = Objeto{}
	}
	r, err := a.ChamarAssistente(context.Background(), Pedido{Metodo: m, Caminho: Prefixo + caminho, Corpo: corpo, Autor: "eu"})
	if err != nil {
		return Resposta{Status: 500, Corpo: Objeto{"erro": err.Error()}}
	}
	return r
}

// A1.3: a lista do código é a x-assistente do contrato, e nenhuma rota dela é x-so-tela.
func TestListaDoAssistenteEhADoContrato(t *testing.T) {
	b, err := os.ReadFile(caminhoContrato)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Lista []string                        `yaml:"x-assistente"`
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	contrato, codigo := append([]string(nil), doc.Lista...), append([]string(nil), rotasDoAssistente...)
	sort.Strings(contrato)
	sort.Strings(codigo)
	if strings.Join(contrato, "\n") != strings.Join(codigo, "\n") || len(codigo) == 0 {
		t.Fatalf("lista do código:\n%s\n\nx-assistente:\n%s", strings.Join(codigo, "\n"), strings.Join(contrato, "\n"))
	}
	for _, r := range rotasDoAssistente {
		m, caminho, _ := strings.Cut(r, " ")
		no, tem := doc.Paths[caminho][strings.ToLower(m)]
		if !tem {
			t.Errorf("%s não está no contrato", r)
			continue
		}
		var op struct {
			Tela bool `yaml:"x-so-tela"`
		}
		if err := no.Decode(&op); err != nil || op.Tela {
			t.Errorf("%s é x-so-tela (%v)", r, err)
		}
	}
}

// A1.2: toda operação do contrato fora da lista dá 403 ANTES de ler o banco. A API do assistente roda com o banco já
// fechado: a recusa é 403; uma rota que chegasse a ler daria outro resultado (as da lista provam isso).
func TestForaDaListaRecusaSemTocarNoBanco(t *testing.T) {
	arm, err := armazem.Abrir(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	a := Nova(arm, Opcoes{Hoje: func() string { return "2026-11-09" }, Canal: dominio.CanalMCP, EnvioLigado: true})
	arm.Fechar()
	ativas, propostas := lerContratoCompleto(t)
	todas := map[string]bool{}
	for _, ops := range []operacoes{ativas, propostas} {
		for caminho, metodos := range ops {
			for m := range metodos {
				todas[m+" "+caminho] = true
			}
		}
	}
	permitidas := map[string]bool{}
	for _, r := range rotasDoAssistente {
		permitidas[r] = true
	}
	recusadas := 0
	for op := range todas {
		m, caminho, _ := strings.Cut(op, " ")
		r := chamarAssistente(t, a, m, concreto(caminho), Objeto{})
		switch {
		case !permitidas[op] && r.Status != 403:
			t.Errorf("%s pelo assistente = %d; queria 403", op, r.Status)
		case permitidas[op] && r.Status == 403:
			t.Errorf("%s está na lista e deu 403 (%v)", op, r.Corpo)
		case !permitidas[op]:
			recusadas++
		}
	}
	if recusadas < 20 {
		t.Fatalf("só %d operações recusadas: o contrato foi lido?", recusadas)
	}
}

// A1.1: casamento exato, depois do formato; nada de prefixo, %, .., barra dupla, barra no fim, maiúsculas.
func TestCaminhoForaDoFormato(t *testing.T) {
	f := novaFila(t)
	a := assistenteDe(f, dominio.CanalMCP)
	for _, c := range []struct{ m, caminho string }{
		{"POST", "/vagas/x/../../dados/apagar"},
		{"GET", "/vagas/.."},
		{"GET", "/vagas/."},
		{"GET", "/vagas/%2e%2e"},
		{"GET", "/vagas/li-1%2Fmover"},
		{"POST", "/vagas/li-1/mover/"},
		{"GET", "//vagas"},
		{"GET", "/vagas/"},
		{"GET", "/VAGAS"},
		{"GET", "/Vagas/li-1"},
		{"GET", `/vagas/li-1\..\x`},
		{"get", "/vagas"},
		{"POST", "/candidaturas/li-1/aprovar"},
		{"POST", "/candidaturas/aprovar-lote"},
		{"GET", "/estado"},
		{"POST", "/vagas"},
		{"GET", "/vagas/li-1/eventos"},
	} {
		if r := chamarAssistente(t, a, c.m, c.caminho, Objeto{"etapa": "pulada"}); r.Status != 403 {
			t.Errorf("%s %s = %d; queria 403", c.m, c.caminho, r.Status)
		}
	}
	// O mesmo formato certo passa.
	if r := chamarAssistente(t, a, "GET", "/vagas/li-1", nil); r.Status != 200 {
		t.Fatalf("GET /vagas/li-1 = %d", r.Status)
	}
	if r := chamarAssistente(t, a, "GET", "/vagas?etapa=aplicar&empresa=Empresa%20Teste", nil); r.Status != 200 {
		t.Fatalf("GET /vagas com consulta = %d", r.Status)
	}
}

// Decisão 1 (CyberSec): o autor é sempre claude, o canal é o do binário e fica no evento; nada no pedido muda isso.
// Uma API com canal fora de mcp e cli recusa tudo.
func TestAssistenteNuncaEhAPessoa(t *testing.T) {
	f := novaFila(t)
	a := assistenteDe(f, dominio.CanalMCP)
	if r := chamarAssistente(t, a, "POST", "/vagas/importar", []any{Objeto{"id": "gh-9", "empresa": "Empresa Nova", "titulo": "Dev Go", "link": "https://exemplo.com/vaga/9"}}); r.Status != 200 && r.Status != 201 {
		t.Fatalf("importar = %d %v", r.Status, r.Corpo)
	}
	// Autor "eu" no pedido não vale: mover para aplicar é só da pessoa (400 do domínio).
	if r := chamarAssistente(t, a, "POST", "/vagas/gh-9/mover", Objeto{"etapa": "aplicar"}); r.Status != 400 {
		t.Fatalf("aplicar pelo assistente = %d", r.Status)
	}
	if r := chamarAssistente(t, a, "POST", "/vagas/gh-9/eventos", Objeto{"tipo": "nota", "texto": "Nota fictícia"}); r.Status != 201 {
		t.Fatalf("evento = %d %v", r.Status, r.Corpo)
	}
	v := f.chamar("GET", "/vagas/gh-9", nil, 200)
	achou := 0
	for _, ev := range dominio.ListaDe(v["eventos"]) {
		o := dominio.ObjetoDe(ev)
		if o["autor"] == "eu" {
			t.Fatalf("evento com autor eu pelo assistente: %v", o)
		}
		if o["autor"] == "claude" && o["canal"] == "mcp" {
			achou++
		}
	}
	if achou < 2 {
		t.Fatalf("eventos sem o canal: %v", v["eventos"])
	}
	// A tela continua sem canal nos eventos.
	f.chamar("POST", "/vagas/gh-9/eventos", Objeto{"tipo": "nota", "texto": "Da pessoa"}, 201)
	v = f.chamar("GET", "/vagas/gh-9", nil, 200)
	ultimo := dominio.ObjetoDe(dominio.ListaDe(v["eventos"])[len(dominio.ListaDe(v["eventos"]))-1])
	if ultimo["autor"] != "eu" || ultimo["canal"] != nil {
		t.Fatalf("evento da tela = %v", ultimo)
	}
	for _, canal := range []string{"", "eu", "tela", "MCP", "claude"} {
		b := assistenteDe(f, canal)
		if r := chamarAssistente(t, b, "GET", "/vagas", nil); r.Status != 403 {
			t.Errorf("canal %q = %d; queria 403", canal, r.Status)
		}
	}
}

// A API do assistente só atende pela lista: o Chamar direto passa por ela, e o Handler (a tela) recusa tudo.
func TestAPIDoAssistenteSoPelaLista(t *testing.T) {
	f := novaFila(t)
	a := assistenteDe(f, dominio.CanalCLI)
	c := f.chamar("POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": conteudoTeste()}, 201)
	r, err := a.Chamar(context.Background(), Pedido{Metodo: "POST", Caminho: Prefixo + "/candidaturas/" + c["id"].(string) + "/aprovar", Corpo: Objeto{"impressao": c["impressao"]}, Autor: "eu"})
	if err != nil || r.Status != 403 {
		t.Fatalf("Chamar direto na API do assistente = %d %v", r.Status, err)
	}
	// Uma leitura fora da lista, que o domínio não barraria: só a lista recusa.
	if r, err := a.Chamar(context.Background(), Pedido{Metodo: "GET", Caminho: Prefixo + "/estado", Corpo: Objeto{}, Autor: "eu"}); err != nil || r.Status != 403 {
		t.Fatalf("GET /estado direto na API do assistente = %d %v", r.Status, err)
	}
	srv := httptest.NewServer(a.Handler())
	defer srv.Close()
	req, _ := http.NewRequest("GET", srv.URL+Prefixo+"/vagas", nil)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Fatalf("Handler da API do assistente = %d", resp.StatusCode)
	}
	if e := f.chamar("GET", "/candidaturas/"+c["id"].(string), nil, 200); e["estado"] != "preparada" {
		t.Fatalf("estado = %v", e["estado"])
	}
}

// Os cabeçalhos da tela (X-Pedido, X-Impressao) nunca passam: o pedido é remontado sem eles.
func TestCabecalhosDaTelaNaoPassam(t *testing.T) {
	f := novaFila(t)
	a := assistenteDe(f, dominio.CanalMCP)
	r, err := a.ChamarAssistente(context.Background(), Pedido{Metodo: "POST", Caminho: Prefixo + "/vagas/li-1/mover", Corpo: Objeto{"etapa": "pulada", "motivo": "Outro"},
		XPedido: "pd-ficticio", XImpressao: "sha256:v1:" + strings.Repeat("0", 64)})
	if err != nil || r.Status != 200 {
		t.Fatalf("mover com cabeçalhos da tela = %d %v %v", r.Status, r.Corpo, err)
	}
}

// Decisão do PO no #135 e A3: nenhum caminho do assistente chega a enviada; com candidatura ativa, ele não muda o que
// define o envio.
func TestAssistenteNaoRegistraEnvio(t *testing.T) {
	f := novaFila(t)
	a := assistenteDe(f, dominio.CanalMCP)
	chamarAssistente(t, a, "POST", "/vagas/importar", []any{
		Objeto{"id": "ex-1", "empresa": "Empresa Um", "titulo": "Dev Go", "link": "https://exemplo.com/1", "etapa": "enviada", "envio": Objeto{"data": "2026-11-01"}},
	})
	v := f.chamar("GET", "/vagas/ex-1", nil, 200)
	if v["etapa"] != "descoberta" || dominio.FoiEnviada(v) {
		t.Fatalf("importar trouxe envio: %v %v", v["etapa"], v["envio"])
	}
	for _, etapa := range []string{"enviada", " Enviada ", "ENVIADA", "ｅｎｖｉａｄａ"} {
		if r := chamarAssistente(t, a, "POST", "/vagas/ex-1/mover", Objeto{"etapa": etapa}); r.Status != 403 {
			t.Errorf("mover para %q = %d; queria 403", etapa, r.Status)
		}
	}
	if r := chamarAssistente(t, a, "POST", "/vagas/ex-1/mover", Objeto{"etapa": "bloqueada", "pendencia": "Login", "envio": Objeto{"data": "2026-11-09"}}); r.Status != 403 {
		t.Errorf("mover com envio = %d; queria 403", r.Status)
	}
	if r := chamarAssistente(t, a, "POST", "/vagas/ex-1/mover", Objeto{"etapa": "em-processo"}); r.Status != 409 {
		t.Errorf("em-processo sem envio = %d; queria 409", r.Status)
	}
	for _, tipo := range []string{"enviada", "Enviada", "em-processo"} {
		if r := chamarAssistente(t, a, "POST", "/vagas/ex-1/eventos", Objeto{"tipo": tipo, "texto": "x"}); r.Status != 403 {
			t.Errorf("evento %q = %d; queria 403", tipo, r.Status)
		}
	}
	if r := chamarAssistente(t, a, "PATCH", "/vagas/ex-1", Objeto{"etapa": "enviada", "envio": Objeto{"data": "2026-11-09"}, "notas": "Nota fictícia"}); r.Status != 200 {
		t.Fatalf("PATCH = %d %v", r.Status, r.Corpo)
	}
	v = f.chamar("GET", "/vagas/ex-1", nil, 200)
	if v["etapa"] != "descoberta" || dominio.FoiEnviada(v) || v["notas"] != "Nota fictícia" {
		t.Fatalf("PATCH mudou etapa ou envio: %v %v", v["etapa"], v["envio"])
	}
	// Vaga já enviada (pela pessoa): o assistente pode registrar o processo.
	f.chamar("POST", "/vagas/ex-1/mover", Objeto{"etapa": "enviada"}, 200)
	if r := chamarAssistente(t, a, "POST", "/vagas/ex-1/mover", Objeto{"etapa": "em-processo", "subEtapa": "entrevista-rh"}); r.Status != 200 {
		t.Fatalf("em-processo de vaga enviada = %d %v", r.Status, r.Corpo)
	}
	// A3: com candidatura ativa, link, plataforma, empresa e título não mudam; o resto muda.
	f.chamar("POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": conteudoTeste()}, 201)
	for campo, valor := range map[string]any{"link": "https://www.linkedin.com/jobs/view/9000000002", "plataforma": "gupy", "empresa": "Outra", "titulo": "Outro"} {
		if r := chamarAssistente(t, a, "PATCH", "/vagas/li-1", Objeto{campo: valor}); r.Status != 409 {
			t.Errorf("PATCH %s com candidatura ativa = %d; queria 409", campo, r.Status)
		}
	}
	if r := chamarAssistente(t, a, "PATCH", "/vagas/li-1", Objeto{"notas": "ok"}); r.Status != 200 {
		t.Fatalf("PATCH notas com candidatura ativa = %d", r.Status)
	}
	// Sem candidatura: o link novo tem de ser da plataforma da vaga.
	f.chamar("POST", "/vagas", Objeto{"id": "li-2", "empresa": "Empresa Dois", "titulo": "Dev Go", "plataforma": "linkedin", "link": "https://www.linkedin.com/jobs/view/9000000003"}, 201)
	if r := chamarAssistente(t, a, "PATCH", "/vagas/li-2", Objeto{"link": "https://www.linkedin.com.exemplo.com/jobs/view/1", "plataforma": "linkedin"}); r.Status != 400 {
		t.Errorf("link fora da plataforma = %d; queria 400", r.Status)
	}
}

// A4 e a condição do CyberSec no #135: quem revogou e quem pausou vêm do servidor (canal do binário), nunca do corpo.
func TestQuemRevogouEPausou(t *testing.T) {
	f := novaFila(t)
	a := assistenteDe(f, dominio.CanalMCP)
	c := f.chamar("POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": conteudoTeste()}, 201)
	id := c["id"].(string)
	f.chamar("POST", "/candidaturas/"+id+"/aprovar", Objeto{"impressao": c["impressao"]}, 200)
	if r := chamarAssistente(t, a, "POST", "/candidaturas/"+id+"/revogar", Objeto{"revogadaPor": Objeto{"autor": "eu", "canal": "tela"}}); r.Status != 400 {
		t.Fatalf("revogar com revogadaPor no corpo = %d; queria 400", r.Status)
	}
	if r := chamarAssistente(t, a, "POST", "/candidaturas/"+id+"/revogar", nil); r.Status != 200 {
		t.Fatalf("revogar pelo assistente = %d %v", r.Status, r.Corpo)
	}
	if q := dominio.ObjetoDe(f.chamar("GET", "/candidaturas/"+id, nil, 200)["revogadaPor"]); q["autor"] != "claude" || q["canal"] != "mcp" {
		t.Fatalf("revogadaPor = %v", q)
	}
	c = f.chamar("GET", "/candidaturas/"+id, nil, 200)
	f.chamar("POST", "/candidaturas/"+id+"/aprovar", Objeto{"impressao": c["impressao"]}, 200)
	f.chamar("POST", "/candidaturas/"+id+"/revogar", Objeto{}, 200)
	if q := dominio.ObjetoDe(f.chamar("GET", "/candidaturas/"+id, nil, 200)["revogadaPor"]); q["autor"] != "eu" || q["canal"] != "tela" {
		t.Fatalf("revogadaPor da tela = %v", q)
	}

	// Pausar o envio: corpo com pausadoPor dá 400; pelo assistente, claude/mcp; retomar (tela) apaga.
	if r := chamarAssistente(t, a, "POST", "/envio/pausar", Objeto{"pausadoPor": Objeto{"autor": "eu", "canal": "tela"}}); r.Status != 400 {
		t.Fatalf("pausar com pausadoPor no corpo = %d; queria 400", r.Status)
	}
	r := chamarAssistente(t, a, "POST", "/envio/pausar", nil)
	if q := dominio.ObjetoDe(dominio.ObjetoDe(r.Corpo)["pausadoPor"]); r.Status != 200 || q["autor"] != "claude" || q["canal"] != "mcp" {
		t.Fatalf("pausar pelo assistente = %d %v", r.Status, r.Corpo)
	}
	if e := f.chamar("POST", "/envio/retomar", Objeto{}, 200); e["pausadoPor"] != nil || e["pausado"] != false {
		t.Fatalf("retomar = %v %v", e["pausado"], e["pausadoPor"])
	}
	if e := f.chamar("POST", "/envio/pausar", Objeto{}, 200); dominio.ObjetoDe(e["pausadoPor"])["canal"] != "tela" {
		t.Fatalf("pausar pela tela = %v", e["pausadoPor"])
	}
	// Pausar uma plataforma pelo assistente.
	if r := chamarAssistente(t, a, "POST", "/envio/plataformas/linkedin/pausar", Objeto{"pausadaPor": Objeto{"autor": "eu"}}); r.Status != 400 {
		t.Fatalf("pausar plataforma com pausadaPor no corpo = %d; queria 400", r.Status)
	}
	r = chamarAssistente(t, a, "POST", "/envio/plataformas/linkedin/pausar", nil)
	if r.Status != 200 {
		t.Fatalf("pausar plataforma = %d %v", r.Status, r.Corpo)
	}
	for _, p := range dominio.ListaDe(dominio.ObjetoDe(r.Corpo)["plataformas"]) {
		o := dominio.ObjetoDe(p)
		if o["plataforma"] == "linkedin" {
			if q := dominio.ObjetoDe(o["pausadaPor"]); o["estado"] != "pausada" || q["autor"] != "claude" || q["canal"] != "mcp" {
				t.Fatalf("linkedin = %v", o)
			}
		} else if o["pausadaPor"] != nil {
			t.Fatalf("%v ganhou pausadaPor", o)
		}
	}
	// Retomar é só da tela.
	if r := chamarAssistente(t, a, "POST", "/envio/retomar", nil); r.Status != 403 {
		t.Fatalf("retomar pelo assistente = %d", r.Status)
	}
}

// A6: o log leva só o canal, o modelo da rota e o status; nunca ids, argumentos nem texto.
func TestLogDoAssistenteSemConteudo(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	f := novaFila(t)
	a := assistenteDe(f, dominio.CanalMCP)
	chamarAssistente(t, a, "POST", "/vagas/importar", []any{Objeto{"id": "mk-1", "empresa": marcaAssistente, "titulo": marcaAssistente, "link": "https://exemplo.com/" + marcaAssistente}})
	chamarAssistente(t, a, "GET", "/vagas/mk-1", nil)
	chamarAssistente(t, a, "POST", "/vagas/mk-1/eventos", Objeto{"tipo": "nota", "texto": marcaAssistente})
	chamarAssistente(t, a, "POST", "/vagas/"+marcaAssistente+"/mover", Objeto{"etapa": "enviada"})
	chamarAssistente(t, a, "GET", "/"+marcaAssistente, nil)
	chamarAssistente(t, a, "GET", "/vagas?empresa="+marcaAssistente, nil)
	tudo := buf.String()
	if !strings.Contains(tudo, "assistente: mcp") {
		t.Fatalf("o log não registrou o assistente:\n%s", tudo)
	}
	for _, proibido := range []string{marcaAssistente, "mk-1", "exemplo.com"} {
		if strings.Contains(tudo, proibido) {
			t.Errorf("log tem %q:\n%s", proibido, tudo)
		}
	}
}

// A2.4: a API do assistente nunca usa o cofre (nem o do sistema, nem um que o binário passe).
func TestAssistenteSemCofre(t *testing.T) {
	f := novaFila(t)
	a := Nova(f.arm, Opcoes{Canal: dominio.CanalMCP, Cofre: cofreQueNaoPodeSerUsado{t}})
	if _, ok := a.cofre.(segredos.Seguro); !ok {
		t.Fatalf("cofre = %T", a.cofre)
	}
	if _, err := a.cofre.Ler("anthropic"); err == nil {
		t.Fatal("o cofre do assistente leu")
	}
	if err := a.cofre.Gravar("anthropic", "sk-ficticia"); err == nil {
		t.Fatal("o cofre do assistente gravou")
	}
}

type cofreQueNaoPodeSerUsado struct{ t *testing.T }

func (c cofreQueNaoPodeSerUsado) Ler(string) (string, error) {
	c.t.Error("a API do assistente usou o cofre passado")
	return "", nil
}
func (c cofreQueNaoPodeSerUsado) Gravar(string, string) error {
	c.t.Error("a API do assistente usou o cofre passado")
	return nil
}
func (c cofreQueNaoPodeSerUsado) Apagar(string) error {
	c.t.Error("a API do assistente usou o cofre passado")
	return nil
}
