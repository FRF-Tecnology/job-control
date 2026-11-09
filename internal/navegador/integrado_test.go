package navegador

// Regressão permanente do caminho da tela (jc-a2y.50, adotado da revisão integrada do CyberSec, jc-a2y.6.4): a
// pessoa cria e aprova pela API HTTP (Handler, autor "eu"), o assistente tenta pelo autor "claude" (como MCP e CLI na
// P6), e o executor envia com o Chrome de verdade contra o servidor fictício local. Só envia o pacote que a pessoa
// aprovou, uma vez. Só dados fictícios. Pula onde não há Chrome nem Edge, e com -short (navegadorDeTeste).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/api"
	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/executor"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

func TestIntegradoTelaAPIExecutor(t *testing.T) {
	var logGlobal bytes.Buffer
	log.SetOutput(&logGlobal)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	srv := novoServidor(t)
	dados := t.TempDir()
	nav := navegadorDeTeste(t, srv, dados)
	arm, err := armazem.Abrir(context.Background(), filepath.Join(dados, "central.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	pastaCV := filepath.Join(dados, "curriculos")
	os.MkdirAll(pastaCV, 0o700)
	os.WriteFile(filepath.Join(pastaCV, "cv-pt.pdf"), pdf, 0o600)

	var logsExec []string
	x := executor.Novo(arm, executor.Opcoes{
		Navegador: nav, PastaCurriculos: pastaCV,
		Esperar: func(ctx context.Context, d time.Duration) error { return ctx.Err() },
		Log:     func(f string, a ...any) { logsExec = append(logsExec, fmt.Sprintf(f, a...)) },
	})
	a := api.Nova(arm, api.Opcoes{Cofre: segredos.Memoria{}})
	h := a.Handler()

	// tela: HTTP pelo Handler, como o webview (mesma origem, JSON, autor "eu").
	tela := func(metodo, caminho string, corpo any) (int, map[string]any) {
		t.Helper()
		b, _ := json.Marshal(corpo)
		req := httptest.NewRequest(metodo, "http://wails.localhost/api/v1"+caminho, bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		var r map[string]any
		json.Unmarshal(w.Body.Bytes(), &r)
		return w.Code, r
	}
	// assistente: o mesmo contrato, com autor "claude" (como MCP/CLI na P6).
	assistente := func(metodo, caminho string, corpo api.Objeto) int {
		t.Helper()
		r, err := a.Chamar(context.Background(), api.Pedido{Metodo: metodo, Caminho: api.Prefixo + caminho, Corpo: corpo, Autor: "claude"})
		if err != nil {
			t.Fatalf("assistente %s %s: %v", metodo, caminho, err)
		}
		return r.Status
	}
	enviarFila := func(passo string) int {
		t.Helper()
		ctx, cancelar := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancelar()
		antes := len(srv.envios())
		if _, err := x.EnviarAprovadas(ctx); err != nil {
			t.Fatalf("%s: EnviarAprovadas: %v", passo, err)
		}
		return len(srv.envios()) - antes
	}
	impressaoDaTela := func(c map[string]any) string {
		conteudo, _ := c["conteudo"].(map[string]any)
		imp, err := dominio.ImpressaoCandidatura(c["vagaId"].(string), c["plataforma"].(string), c["url"].(string), dominio.Objeto(conteudo))
		if err != nil {
			t.Fatal(err)
		}
		return imp
	}
	ler := func(id string) map[string]any {
		_, c := tela("GET", "/candidaturas/"+id, nil)
		return c
	}

	// Vaga aprovada na Triagem pela pessoa.
	link := "https://www.linkedin.com/jobs/view/9000000001"
	if st, r := tela("POST", "/vagas", map[string]any{"id": "v-int-1", "empresa": "Empresa Ficticia", "titulo": "Dev Go", "plataforma": "linkedin", "link": link}); st != 201 && st != 200 {
		t.Fatalf("criar vaga = %d %v", st, r)
	}
	if st, r := tela("POST", "/vagas/v-int-1/mover", map[string]any{"etapa": "aplicar"}); st != 200 {
		t.Fatalf("mover = %d %v", st, r)
	}
	conteudo := map[string]any{
		"curriculo": map[string]any{"arquivo": "cv-pt.pdf"},
		"idioma":    "pt",
		"contato":   map[string]any{"nome": "Pessoa Teste", "email": "pessoa@exemplo.com", "telefone": "11900000000"},
		"respostas": []any{
			map[string]any{"pergunta": "Anos de experiência com Go", "resposta": "3", "origem": "pessoa"},
			map[string]any{"pergunta": "Experiência com banco de dados?", "resposta": "Sim", "origem": "respostasPadrao"},
			map[string]any{"pergunta": "Mensagem marcada", "resposta": marca, "origem": "pessoa"},
		},
	}

	// 1) O assistente não cria candidatura.
	if st := assistente("POST", "/candidaturas", api.Objeto{"vagaId": "v-int-1", "conteudo": conteudo}); st != 403 {
		t.Errorf("assistente criar = %d; queria 403", st)
	}
	st, c := tela("POST", "/candidaturas", map[string]any{"vagaId": "v-int-1", "conteudo": conteudo})
	if st != 201 {
		t.Fatalf("criar candidatura = %d %v", st, c)
	}
	id := c["id"].(string)

	// 2) Preparada (sem aprovação) não é enviada.
	if n := enviarFila("preparada"); n != 0 {
		t.Errorf("preparada enviou %d", n)
	}
	// 3) O assistente não aprova, nem com a impressão certa.
	if st := assistente("POST", "/candidaturas/"+id+"/aprovar", api.Objeto{"impressao": impressaoDaTela(ler(id))}); st != 403 {
		t.Errorf("assistente aprovar = %d; queria 403", st)
	}
	// 4) Impressão errada: 409.
	if st, _ := tela("POST", "/candidaturas/"+id+"/aprovar", map[string]any{"impressao": "sha256:v1:" + strings.Repeat("0", 64)}); st != 409 {
		t.Errorf("aprovar com impressão errada = %d; queria 409", st)
	}
	// 5) Campo a mais no aprovar: 400.
	if st, _ := tela("POST", "/candidaturas/"+id+"/aprovar", map[string]any{"impressao": impressaoDaTela(ler(id)), "estado": "enviando"}); st != 400 {
		t.Errorf("aprovar com campo a mais = %d; queria 400", st)
	}
	if n := enviarFila("depois das recusas"); n != 0 {
		t.Errorf("recusas enviaram %d", n)
	}
	// 6) Aprovada e depois editada: volta a preparada e não envia.
	if st, r := tela("POST", "/candidaturas/"+id+"/aprovar", map[string]any{"impressao": impressaoDaTela(ler(id))}); st != 200 {
		t.Fatalf("aprovar = %d %v", st, r)
	}
	atual := ler(id)
	conteudo2 := map[string]any{}
	for k, v := range conteudo {
		conteudo2[k] = v
	}
	conteudo2["mensagem"] = "Mensagem editada pela pessoa"
	if st, r := tela("PUT", "/candidaturas/"+id+"/conteudo", map[string]any{"conteudo": conteudo2, "versao": atual["versao"]}); st != 200 || r["estado"] != "preparada" {
		t.Fatalf("editar aprovada = %d %v", st, r["estado"])
	}
	if n := enviarFila("editada"); n != 0 {
		t.Errorf("editada enviou %d", n)
	}
	// 7) Aprovada e revogada (até pelo assistente): não envia.
	if st, _ := tela("POST", "/candidaturas/"+id+"/aprovar", map[string]any{"impressao": impressaoDaTela(ler(id))}); st != 200 {
		t.Fatal("reaprovar falhou")
	}
	if st := assistente("POST", "/candidaturas/"+id+"/revogar", api.Objeto{}); st != 200 {
		t.Errorf("assistente revogar = %d; queria 200", st)
	}
	if n := enviarFila("revogada"); n != 0 {
		t.Errorf("revogada enviou %d", n)
	}
	// 8) Aprovada pela pessoa: envia uma vez, só o pacote.
	if st, _ := tela("POST", "/candidaturas/"+id+"/aprovar", map[string]any{"impressao": impressaoDaTela(ler(id))}); st != 200 {
		t.Fatal("aprovar final falhou")
	}
	// 8a) jc-a2y.51: o envio começa PAUSADO. Pausado, a aprovada não é enviada; o assistente não retoma.
	if st, e := tela("GET", "/envio", nil); st != 200 || e["pausado"] != true {
		t.Fatalf("estado do envio no começo = %d %v", st, e)
	}
	if n := enviarFila("pausado"); n != 0 {
		t.Fatalf("pausado enviou %d", n)
	}
	if st := assistente("POST", "/envio/retomar", api.Objeto{}); st != 403 {
		t.Errorf("assistente retomar = %d; queria 403", st)
	}
	if n := enviarFila("depois do assistente"); n != 0 {
		t.Fatalf("o assistente retomou: enviou %d", n)
	}
	if st, e := tela("POST", "/envio/retomar", map[string]any{}); st != 200 || e["pausado"] != false {
		t.Fatalf("retomar pela tela = %d %v", st, e)
	}
	// 8b) Retomado pela pessoa, a aprovada é enviada uma vez.
	if n := enviarFila("aprovada"); n != 1 {
		t.Fatalf("aprovada enviou %d; queria 1", n)
	}
	// O desfecho aparece no estado do envio, sem conteúdo.
	if st, e := tela("GET", "/envio", nil); st != 200 || len(e["desfechos"].([]any)) != 1 || e["desfechos"].([]any)[0].(map[string]any)["estado"] != "enviada" {
		t.Errorf("desfechos = %d %v", st, e["desfechos"])
	}
	// Só o pacote aprovado chegou ao servidor fictício: os campos da página e o PDF conferido.
	envios := srv.envios()
	ultimo := envios[len(envios)-1]
	for k, v := range map[string]string{"nome": "Pessoa Teste", "email": "pessoa@exemplo.com", "tel": "11900000000", "anos": "3", "bd": "s"} {
		if ultimo.campos.Get(k) != v {
			t.Errorf("campo %s = %q; queria %q", k, ultimo.campos.Get(k), v)
		}
	}
	if ultimo.campos.Get("remoto") != "" || !bytes.Equal(ultimo.arquivo, pdf) {
		t.Errorf("enviou fora do pacote: remoto=%q, arquivo=%q", ultimo.campos.Get("remoto"), ultimo.arquivo)
	}
	if e := ler(id); e["estado"] != "enviada" {
		t.Errorf("estado final = %v", e["estado"])
	}
	_, est := tela("GET", "/estado", nil)
	achou := false
	for _, v := range est["vagas"].([]any) {
		if o := v.(map[string]any); o["id"] == "v-int-1" {
			achou = true
			if o["etapa"] != "enviada" {
				t.Errorf("vaga ficou em %v", o["etapa"])
			}
		}
	}
	if !achou {
		t.Error("a vaga sumiu do /estado") // nota do CyberSec no #55
	}
	// 9) A aprovação foi consumida: rodar de novo não reenvia.
	if n := enviarFila("depois do envio"); n != 0 {
		t.Errorf("reenviou %d", n)
	}
	// 10) Logs sem conteúdo.
	todos := logGlobal.String() + "\n" + strings.Join(logsExec, "\n")
	for _, proibido := range []string{"Pessoa Teste", "pessoa@exemplo.com", "11900000000", marca, "Mensagem editada", link, "9000000001", "cv-pt.pdf", dados, "Empresa Ficticia"} {
		if strings.Contains(todos, proibido) {
			t.Errorf("log contém %q", proibido)
		}
	}
}
