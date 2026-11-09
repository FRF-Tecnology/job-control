package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/curriculos"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// Rotas da fila (jc-a2y.6.2.2). Dados fictícios: vaga, currículo e respostas de teste; nenhum endereço de vaga real.

const marcaConteudo = "Resposta-Ficticia-Marcada-7f3a" // não pode aparecer no log

var pdfFicticio = []byte("%PDF-1.4\n% curriculo ficticio de teste\n")

type filaTeste struct {
	t     *testing.T
	api   *API
	arm   *armazem.Armazem
	pasta string
	srv   *httptest.Server
}

func novaFila(t *testing.T) *filaTeste {
	t.Helper()
	pasta := t.TempDir()
	arm, err := armazem.Abrir(context.Background(), filepath.Join(pasta, "central.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	if err := os.MkdirAll(filepath.Join(pasta, "curriculos"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pasta, "curriculos", "cv-pt.pdf"), pdfFicticio, 0o600); err != nil {
		t.Fatal(err)
	}
	a := Nova(arm, Opcoes{Hoje: func() string { return "2026-11-09" }, Aleatorio: func() string { return "abcd" },
		Cofre: segredos.Memoria{}, ProvedoresIA: map[string]ia.Provedor{},
		DetectarCLIs: func(context.Context) []ia.EstadoCLI { return nil }})
	a.relogio = func() time.Time { return time.Date(2026, 11, 9, 10, 0, 0, 0, time.Local) }
	f := &filaTeste{t: t, api: a, arm: arm, pasta: pasta}
	f.srv = httptest.NewServer(a.Handler())
	t.Cleanup(f.srv.Close)
	// Vaga fictícia aprovada pela pessoa (Triagem).
	f.chamar("POST", "/vagas", Objeto{"id": "li-1", "empresa": "Empresa Teste", "titulo": "Dev Go", "plataforma": "linkedin", "link": "https://www.linkedin.com/jobs/view/9000000001"}, 201)
	f.chamar("POST", "/vagas/li-1/mover", Objeto{"etapa": "aplicar"}, 200)
	return f
}

// chamar passa pelo HTTP de verdade (JSON, mesma origem), como a tela.
func (f *filaTeste) chamar(m, caminho string, corpo any, quer int) Objeto {
	f.t.Helper()
	var r *http.Request
	if m == "GET" {
		r, _ = http.NewRequest(m, f.srv.URL+Prefixo+caminho, nil)
	} else {
		b, _ := json.Marshal(corpo)
		r, _ = http.NewRequest(m, f.srv.URL+Prefixo+caminho, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		f.t.Fatal(err)
	}
	defer resp.Body.Close()
	var saida any
	json.NewDecoder(resp.Body).Decode(&saida)
	if resp.StatusCode != quer {
		f.t.Fatalf("%s %s = %d %v; queria %d", m, caminho, resp.StatusCode, saida, quer)
	}
	o, _ := saida.(Objeto)
	if l, ehLista := saida.([]any); ehLista {
		o = Objeto{"lista": l}
	}
	return o
}

// comoAssistente chama como a CLI/MCP chamam (autor claude), sem passar pelo HTTP.
func (f *filaTeste) comoAssistente(m, caminho string, corpo Objeto) Resposta {
	f.t.Helper()
	r, err := f.api.Chamar(context.Background(), Pedido{Metodo: m, Caminho: Prefixo + caminho, Corpo: corpo, Autor: "claude"})
	if err != nil {
		f.t.Fatal(err)
	}
	return r
}

func (f *filaTeste) versao() int64 {
	v, _ := f.arm.Versao(context.Background())
	return v
}

func conteudoTeste() Objeto {
	return Objeto{
		"curriculo": Objeto{"arquivo": "cv-pt.pdf", "sha256": strings.Repeat("f", 64)}, // sha do cliente: ignorado
		"idioma":    "pt",
		"contato":   Objeto{"nome": "Pessoa Teste", "email": "pessoa@exemplo.com"},
		"pretensao": nil,
		"respostas": []any{Objeto{"pergunta": "Anos de Go?", "resposta": marcaConteudo, "origem": "pessoa"}},
		"mensagem":  nil,
	}
}

func TestFilaCicloPelaAPI(t *testing.T) {
	f := novaFila(t)
	c := f.chamar("POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": conteudoTeste()}, 201)
	soma := sha256.Sum256(pdfFicticio)
	cv := c["conteudo"].(Objeto)["curriculo"].(Objeto)
	if cv["sha256"] != hex.EncodeToString(soma[:]) {
		t.Fatalf("sha256 = %v; queria o do arquivo (o do cliente é ignorado)", cv["sha256"])
	}
	if _, tem := c["tentativas"]; tem {
		t.Fatal("tentativas é interno e não sai na API")
	}
	id := c["id"].(string)
	if !regexp.MustCompile(`^cand-2026-11-09-0001-abcd$`).MatchString(id) || c["estado"] != "preparada" || c["plataforma"] != "linkedin" {
		t.Fatalf("criada = %v", c)
	}
	for _, k := range []string{"aprovacao", "bloqueio", "resultado"} {
		if v, tem := c[k]; !tem || v != nil {
			t.Errorf("%s deveria vir null: %v", k, v)
		}
	}
	if l := f.chamar("GET", "/candidaturas", nil, 200)["lista"].([]any); len(l) != 1 {
		t.Fatalf("lista = %v", l)
	}
	if l := f.chamar("GET", "/candidaturas?estado=aprovada", nil, 200)["lista"].([]any); len(l) != 0 {
		t.Fatalf("filtro = %v", l)
	}
	f.chamar("GET", "/candidaturas?estado=qualquer", nil, 400)
	f.chamar("GET", "/candidaturas/"+id, nil, 200)
	f.chamar("GET", "/candidaturas/cand-nao-existe", nil, 404)

	// Impressão velha: 409 com a atual.
	cf := f.chamar("POST", "/candidaturas/"+id+"/aprovar", Objeto{"impressao": "sha256:v1:" + strings.Repeat("0", 64)}, 409)
	if cf["impressaoAtual"] != c["impressao"] || cf["estado"] != "preparada" {
		t.Fatalf("409 = %v", cf)
	}
	ap := f.chamar("POST", "/candidaturas/"+id+"/aprovar", Objeto{"impressao": c["impressao"]}, 200)
	apr := ap["aprovacao"].(Objeto)
	if ap["estado"] != "aprovada" || apr["validaAte"] != "2026-11-09T23:59:59-03:00" || apr["fuso"] != "America/Sao_Paulo" {
		t.Fatalf("aprovada = %v", ap)
	}
	f.chamar("POST", "/candidaturas/"+id+"/aprovar", Objeto{"impressao": c["impressao"]}, 409)

	// Editar tira a aprovação; versão velha dá 409.
	novo := conteudoTeste()
	novo["idioma"] = "en"
	f.chamar("PUT", "/candidaturas/"+id+"/conteudo", Objeto{"conteudo": novo, "versao": 1}, 409)
	ed := f.chamar("PUT", "/candidaturas/"+id+"/conteudo", Objeto{"conteudo": novo, "versao": ap["versao"]}, 200)
	if ed["estado"] != "preparada" || ed["aprovacao"] != nil || ed["impressao"] == c["impressao"] {
		t.Fatalf("editada = %v", ed)
	}
	f.chamar("POST", "/candidaturas/"+id+"/aprovar", Objeto{"impressao": ed["impressao"]}, 200)
	f.chamar("POST", "/candidaturas/"+id+"/revogar", Objeto{}, 200)
	f.chamar("POST", "/candidaturas/"+id+"/reabrir", Objeto{}, 409)
	f.chamar("POST", "/candidaturas/"+id+"/concluir-manual", Objeto{}, 409)
	ca := f.chamar("POST", "/candidaturas/"+id+"/cancelar", Objeto{}, 200)
	if ca["estado"] != "cancelada" {
		t.Fatalf("cancelada = %v", ca)
	}
	f.chamar("POST", "/candidaturas/"+id+"/cancelar", Objeto{}, 409)
	// Cancelada libera a vaga; a segunda ativa dá 409 com a existente.
	c2 := f.chamar("POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": conteudoTeste()}, 201)
	if d := f.chamar("POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": conteudoTeste()}, 409); d["existente"] != c2["id"] {
		t.Fatalf("duplicata = %v", d)
	}
	f.chamar("POST", "/candidaturas", Objeto{"vagaId": "nao-existe", "conteudo": conteudoTeste()}, 404)
}

// additionalProperties: false de verdade, em todo corpo; nada é gravado.
func TestFilaCorpoEstrito(t *testing.T) {
	f := novaFila(t)
	c := f.chamar("POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": conteudoTeste()}, 201)
	id := c["id"].(string)
	antes := f.versao()
	extra := conteudoTeste()
	extra["cpf"] = "x"
	casos := []struct {
		m, p  string
		corpo Objeto
	}{
		{"POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": conteudoTeste(), "estado": "aprovada"}},
		{"POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": extra}},
		{"POST", "/candidaturas", Objeto{"conteudo": conteudoTeste()}},
		{"POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": "texto"}},
		{"PUT", "/candidaturas/" + id + "/conteudo", Objeto{"conteudo": conteudoTeste(), "versao": 1, "estado": "aprovada"}},
		{"PUT", "/candidaturas/" + id + "/conteudo", Objeto{"conteudo": conteudoTeste(), "versao": "1"}},
		{"PUT", "/candidaturas/" + id + "/conteudo", Objeto{"conteudo": conteudoTeste(), "versao": 1.5}},
		{"POST", "/candidaturas/" + id + "/aprovar", Objeto{"impressao": c["impressao"], "validaAte": "2099-01-01T00:00:00Z"}},
		{"POST", "/candidaturas/" + id + "/aprovar", Objeto{}},
		{"POST", "/candidaturas/" + id + "/revogar", Objeto{"motivo": "x"}},
		{"POST", "/candidaturas/" + id + "/cancelar", Objeto{"x": 1}},
		{"POST", "/candidaturas/" + id + "/reabrir", Objeto{"estado": "preparada"}},
		{"POST", "/candidaturas/" + id + "/concluir-manual", Objeto{"envio": Objeto{}}},
		{"PUT", "/candidaturas/limites/linkedin", Objeto{"limite": 10, "plataforma": "gupy"}},
		{"PUT", "/candidaturas/limites/linkedin", Objeto{"limite": 0}},
		{"PUT", "/candidaturas/limites/linkedin", Objeto{"limite": 41}},
		{"PUT", "/candidaturas/limites/linkedin", Objeto{"limite": 2.5}},
	}
	for _, k := range casos {
		f.chamar(k.m, k.p, k.corpo, 400)
	}
	if v := f.versao(); v != antes {
		t.Fatalf("versão %d → %d: um pedido recusado gravou", antes, v)
	}
	if l, _ := f.arm.LimitesCandidatura(context.Background()); len(l) != 0 {
		t.Fatalf("limite recusado gravou: %v", l)
	}
}

// O currículo é lido só da pasta de currículos, como arquivo comum de até 20 MB.
func TestFilaCurriculoDoServidor(t *testing.T) {
	f := novaFila(t)
	pastaCV := filepath.Join(f.pasta, "curriculos")
	os.Mkdir(filepath.Join(pastaCV, "pasta.pdf"), 0o700)
	grande, _ := os.Create(filepath.Join(pastaCV, "grande.pdf"))
	grande.Truncate(curriculos.MaxBytes + 1)
	grande.Close()
	os.WriteFile(filepath.Join(f.pasta, "fora.pdf"), pdfFicticio, 0o600)
	for _, nome := range []string{"nao-existe.pdf", "../fora.pdf", `..\fora.pdf`, "pasta.pdf", "grande.pdf", "C:fora.pdf", "cv-pt.pdf:fluxo", "cv-pt.PDF.exe"} {
		c := conteudoTeste()
		c["curriculo"] = Objeto{"arquivo": nome}
		f.chamar("POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": c}, 400)
	}
	if err := os.Symlink(filepath.Join(f.pasta, "fora.pdf"), filepath.Join(pastaCV, "atalho.pdf")); err == nil {
		c := conteudoTeste()
		c["curriculo"] = Objeto{"arquivo": "atalho.pdf"}
		f.chamar("POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": c}, 400)
	} else {
		t.Logf("sem link simbólico nesta máquina (%v); o caso do atalho roda no CI", err)
	}
}

// x-so-tela: o assistente recebe 403 e nada é gravado; revogar (reduz o risco) é permitido.
func TestFilaSoATela(t *testing.T) {
	f := novaFila(t)
	c := f.chamar("POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": conteudoTeste()}, 201)
	id := c["id"].(string)
	antes := f.versao()
	semArquivo := conteudoTeste()
	semArquivo["curriculo"] = Objeto{"arquivo": "nao-existe.pdf"}
	casos := []struct {
		m, p  string
		corpo Objeto
	}{
		{"POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": semArquivo}},                      // 403 antes de olhar o arquivo
		{"PUT", "/candidaturas/" + id + "/conteudo", Objeto{"conteudo": conteudoTeste(), "versao": 1.0}}, // número como sai do JSON
		{"POST", "/candidaturas/" + id + "/aprovar", Objeto{"impressao": c["impressao"]}},
		{"POST", "/candidaturas/" + id + "/cancelar", Objeto{}},
		{"POST", "/candidaturas/" + id + "/reabrir", Objeto{}},
		{"POST", "/candidaturas/" + id + "/concluir-manual", Objeto{}},
		{"PUT", "/candidaturas/limites/linkedin", Objeto{"limite": 40.0}},
	}
	for _, k := range casos {
		if r := f.comoAssistente(k.m, k.p, k.corpo); r.Status != 403 {
			t.Errorf("%s %s como assistente = %d %v; queria 403", k.m, k.p, r.Status, r.Corpo)
		}
	}
	if v := f.versao(); v != antes {
		t.Fatalf("o assistente gravou (versão %d → %d)", antes, v)
	}
	// Ler e revogar podem.
	if r := f.comoAssistente("GET", "/candidaturas", nil); r.Status != 200 {
		t.Errorf("listar como assistente = %d", r.Status)
	}
	f.chamar("POST", "/candidaturas/"+id+"/aprovar", Objeto{"impressao": c["impressao"]}, 200)
	if r := f.comoAssistente("POST", "/candidaturas/"+id+"/revogar", Objeto{}); r.Status != 200 {
		t.Errorf("revogar como assistente = %d %v", r.Status, r.Corpo)
	}
}

// Limite diário: leitura, gravação e 429 com {erro, plataforma, limite, usadas, renovaEm}.
func TestFilaLimiteDiario(t *testing.T) {
	f := novaFila(t)
	l := f.chamar("GET", "/candidaturas/limites", nil, 200)
	for _, p := range []string{"linkedin", "gupy", "inhire"} {
		x := l[p].(Objeto)
		if x["limite"] != 30.0 || x["usadas"] != 0.0 || x["renovaEm"] != "2026-11-10T00:00:00-03:00" {
			t.Fatalf("%s = %v", p, x)
		}
	}
	if g := f.chamar("PUT", "/candidaturas/limites/linkedin", Objeto{"limite": 1}, 200); g["limite"] != 1.0 {
		t.Fatalf("gravado = %v", g)
	}
	f.chamar("PUT", "/candidaturas/limites/outra", Objeto{"limite": 1}, 404)
	f.chamar("PUT", "/candidaturas/limites/indeed", Objeto{"limite": 1}, 404)

	// Uma tentativa hoje (o executor, sem rota, registra; aqui o teste faz o papel dele no domínio).
	c := f.chamar("POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": conteudoTeste()}, 201)
	ap := f.chamar("POST", "/candidaturas/"+c["id"].(string)+"/aprovar", Objeto{"impressao": c["impressao"]}, 200)
	agora := f.api.relogio()
	if err := f.arm.Gravar(context.Background(), func(e dominio.Estado) (dominio.Estado, error) {
		v, _ := inteiro(ap["versao"])
		novo, _, err := dominio.IniciarEnvio(e, ap["id"].(string), v, agora, map[string]int{"linkedin": 1})
		if err != nil {
			return e, err
		}
		novo, _, err = dominio.FalharCandidatura(novo, ap["id"].(string), "erro-tecnico", agora)
		return novo, err
	}); err != nil {
		t.Fatal(err)
	}
	f.chamar("POST", "/candidaturas/"+c["id"].(string)+"/reabrir", Objeto{}, 200)
	r := f.chamar("GET", "/candidaturas/"+c["id"].(string), nil, 200)
	if _, tem := r["tentativas"]; tem {
		t.Fatal("tentativas é interno e não sai na API")
	}
	lim := f.chamar("POST", "/candidaturas/"+c["id"].(string)+"/aprovar", Objeto{"impressao": r["impressao"]}, 429)
	if lim["plataforma"] != "linkedin" || lim["limite"] != 1.0 || lim["usadas"] != 1.0 || lim["renovaEm"] != "2026-11-10T00:00:00-03:00" || lim["erro"] == "" {
		t.Fatalf("429 = %v", lim)
	}
	if x := f.chamar("GET", "/candidaturas/limites", nil, 200)["linkedin"].(Objeto); x["usadas"] != 1.0 || x["limite"] != 1.0 {
		t.Fatalf("limites depois = %v", x)
	}
}

// O envio é do executor interno: nenhum código de rota chama as funções dele, e nenhuma rota de envio existe.
func TestFilaSemRotaDeEnvio(t *testing.T) {
	proibidas := regexp.MustCompile(`dominio\.(IniciarEnvio|MarcarEnviada|BloquearCandidatura|FalharCandidatura|MarcarIncertas|VencerAprovacoes|EncaminharExterna|TornarIncerta)\b`)
	var arquivos []string
	for _, padrao := range []string{"*.go", "../../cmd/*/*.go", "../../app/*.go"} {
		m, _ := filepath.Glob(padrao)
		arquivos = append(arquivos, m...)
	}
	vistos := 0
	for _, arq := range arquivos {
		if strings.HasSuffix(arq, "_test.go") {
			continue
		}
		b, err := os.ReadFile(arq)
		if err != nil {
			t.Fatal(err)
		}
		vistos++
		if achou := proibidas.FindString(string(b)); achou != "" {
			t.Errorf("%s chama %s: o envio não pode ter caminho pela API, CLI ou app", arq, achou)
		}
	}
	if vistos < 5 {
		t.Fatalf("só %d arquivos conferidos; o padrão ficou velho", vistos)
	}
	f := novaFila(t)
	c := f.chamar("POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": conteudoTeste()}, 201)
	for _, acao := range []string{"enviar", "enviando", "enviada", "bloquear", "falhar", "incerta"} {
		for _, m := range []string{"POST", "PUT", "PATCH"} {
			if r := f.chamar(m, "/candidaturas/"+c["id"].(string)+"/"+acao, Objeto{}, 404); r["erro"] != "rota não encontrada" {
				t.Errorf("%s .../%s = %v", m, acao, r)
			}
		}
	}
	// Nem PATCH na candidatura muda o estado.
	f.chamar("PATCH", "/candidaturas/"+c["id"].(string), Objeto{"estado": "enviada"}, 404)
}

// M7: nada do conteúdo vai para o log, nem num 500 no meio de uma gravação da fila.
func TestFilaLogSemConteudo(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	f := novaFila(t)
	c := f.chamar("POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": conteudoTeste()}, 201)
	f.chamar("POST", "/candidaturas/"+c["id"].(string)+"/aprovar", Objeto{"impressao": "errada"}, 409)
	ruim := conteudoTeste()
	ruim["extra"] = marcaConteudo
	f.chamar("POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": ruim}, 400)
	f.arm.Fechar() // banco fora: a próxima gravação dá 500
	f.chamar("POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": conteudoTeste()}, 500)
	saida := buf.String()
	if !strings.Contains(saida, "erro 500 em POST /api/v1/candidaturas") {
		t.Fatalf("o 500 deveria ir para o log (só método, caminho e tipo): %q", saida)
	}
	for _, proibido := range []string{marcaConteudo, "Pessoa Teste", "pessoa@exemplo.com", "cv-pt.pdf", f.pasta} {
		if strings.Contains(saida, proibido) {
			t.Errorf("log tem %q: %q", proibido, saida)
		}
	}
}
