package navegador

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/api"
	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/executor"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// jc-a2y.7.2, parte 3b-2: o envio do formulário com o Chrome de verdade, sem janela, contra um servidor fictício
// local. O DOM é o do formato PÚBLICO do Google Forms, montado por nós (não conferido contra um formulário real,
// decisão do PO). Nada contra o Google de verdade. Dados fictícios.

const tokenEditar = "TOKEN-FICTICIO-EDITAR-7f3a" // o link "Editar sua resposta": nunca no banco nem no log

const dadosEnvio = `[null,["d",[` +
	`[1,"Qual o seu e-mail?",null,0,[[2001,null,1]]],` +
	`[2,"Conte sobre você",null,1,[[2002,null,0]]],` +
	`[3,"Nível de inglês",null,2,[[2003,[["Básico"],["Avançado"]],1]]],` +
	`[4,"Tecnologias",null,4,[[2004,[["Go"],["React"]],0]]],` +
	`[5,"Estado",null,3,[[2005,[["SP"],["RJ"]],1]]],` +
	`[6,"Disponível a partir de",null,9,[[2006,null,0]]]` +
	`]],"/forms","Formulário fictício"]`

// paginaDeEnvio monta o formulário fictício. copia: a caixa da cópia por e-mail já marcada; preenchido: a página
// traz um valor em entry.2002; botao: tem o botão Enviar.
func paginaDeEnvio(id string, copia, preenchido, botao bool) string {
	marcada, valor2002, enviar := "false", "", ""
	if copia {
		marcada = "true"
	}
	if preenchido {
		valor2002 = "texto que a página trouxe"
	}
	if botao {
		enviar = `<div role="button" id="enviar"><span>Enviar</span></div>`
	}
	return `<!doctype html><html><head><meta charset="utf-8"><script>var FB_PUBLIC_LOAD_DATA_ = ` + dadosEnvio + `;</script></head><body>
<form id="f" method="POST" action="/forms/d/e/` + id + `/formResponse">
<div role="list">
 <div role="listitem"><div>Qual o seu e-mail?</div><input type="text"><input type="hidden" name="entry.2001" value=""></div>
 <div role="listitem"><div>Conte sobre você</div><textarea>` + valor2002 + `</textarea><input type="hidden" name="entry.2002" value=""></div>
 <div role="listitem"><div>Nível de inglês</div><div role="radio" data-value="Básico" aria-checked="false">Básico</div><div role="radio" data-value="Avançado" aria-checked="false">Avançado</div><input type="hidden" name="entry.2003" value=""></div>
 <div role="listitem"><div>Tecnologias</div><div role="checkbox" data-answer-value="Go" aria-checked="false">Go</div><div role="checkbox" data-answer-value="React" aria-checked="false">React</div><input type="hidden" name="entry.2004" value=""></div>
 <div role="listitem"><div>Estado</div><div role="listbox"><div role="option" data-value="" aria-selected="true">Escolher</div><div role="option" data-value="SP" aria-selected="false">SP</div><div role="option" data-value="RJ" aria-selected="false">RJ</div></div><input type="hidden" name="entry.2005" value=""></div>
 <div role="listitem"><div>Disponível a partir de</div><input type="date"><input type="hidden" name="entry.2006" value=""></div>
</div>
<div><div role="checkbox" id="copia" aria-label="Enviar uma cópia das minhas respostas para o meu e-mail" aria-checked="` + marcada + `"></div> Enviar uma cópia das minhas respostas</div>
` + enviar + `
<div role="button" id="limpar"><span>Limpar formulário</span></div>
</form>
<script>
const oculto = c => c.querySelector('input[type="hidden"]');
for (const c of document.querySelectorAll('[role="listitem"]')) {
  for (const el of c.querySelectorAll('input:not([type="hidden"]), textarea')) el.addEventListener('input', () => { oculto(c).value = el.value; });
  for (const r of c.querySelectorAll('[role="radio"]')) r.addEventListener('click', () => { for (const o of c.querySelectorAll('[role="radio"]')) o.setAttribute('aria-checked', String(o === r)); oculto(c).value = r.getAttribute('data-value'); });
  for (const r of c.querySelectorAll('[role="checkbox"]')) r.addEventListener('click', () => { r.setAttribute('aria-checked', String(r.getAttribute('aria-checked') !== 'true')); });
  for (const o of c.querySelectorAll('[role="option"]')) o.addEventListener('click', () => { for (const x of c.querySelectorAll('[role="option"]')) x.setAttribute('aria-selected', String(x === o)); oculto(c).value = o.getAttribute('data-value'); });
}
document.getElementById('copia').addEventListener('click', e => { const el = e.currentTarget; el.setAttribute('aria-checked', String(el.getAttribute('aria-checked') !== 'true')); });
document.getElementById('limpar').addEventListener('click', () => { fetch('/limpou', { method: 'POST' }); });
const b = document.getElementById('enviar');
if (b) b.addEventListener('click', () => {
  const f = document.getElementById('f');
  for (const c of document.querySelectorAll('[role="listitem"]')) {
    const marcadas = Array.from(c.querySelectorAll('[role="checkbox"][aria-checked="true"]'));
    if (marcadas.length) { const h = oculto(c); h.remove(); for (const m of marcadas) { const i = document.createElement('input'); i.type = 'hidden'; i.name = h.name; i.value = m.getAttribute('data-answer-value'); f.appendChild(i); } }
  }
  if (document.getElementById('copia').getAttribute('aria-checked') === 'true') { const i = document.createElement('input'); i.type = 'hidden'; i.name = 'sendMeACopy'; i.value = 'on'; f.appendChild(i); }
  f.submit();
});
</script></body></html>`
}

// servidorDeEnvio: os formulários fictícios por caminho e o que chegou no formResponse.
type servidorDeEnvio struct {
	*servidorFicticio
	mu       sync.Mutex
	recebido []url.Values
	limpou   int
}

func (s *servidorDeEnvio) respostas() []url.Values {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]url.Values(nil), s.recebido...)
}

func novoServidorDeEnvio(t *testing.T) *servidorDeEnvio {
	t.Helper()
	s := &servidorDeEnvio{}
	paginas := map[string]string{
		"envio":      paginaDeEnvio("1FAIpQLSf_envio", false, false, true),
		"copia":      paginaDeEnvio("1FAIpQLSf_copia", true, false, true),
		"preenchido": paginaDeEnvio("1FAIpQLSf_preenchido", false, true, true),
		"sembotao":   paginaDeEnvio("1FAIpQLSf_sembotao", false, false, false),
	}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := strings.Split(r.Host, ":")[0]
		if host != "docs.google.com" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/limpou" {
			s.mu.Lock()
			s.limpou++
			s.mu.Unlock()
			return
		}
		var nome, fim string
		if _, err := fmt.Sscanf(strings.ReplaceAll(strings.TrimPrefix(r.URL.Path, "/forms/d/e/1FAIpQLSf_"), "/", " "), "%s %s", &nome, &fim); err != nil {
			http.NotFound(w, r)
			return
		}
		switch {
		case r.Method == http.MethodGet && fim == "viewform" && paginas[nome] != "":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, paginas[nome])
		case r.Method == http.MethodPost && fim == "formResponse":
			r.ParseForm()
			s.mu.Lock()
			s.recebido = append(s.recebido, r.PostForm)
			s.mu.Unlock()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<!doctype html><html><body><p>Formulário fictício</p><p>Sua resposta foi registrada.</p><a href="/forms/d/e/1FAIpQLSf_`+nome+`/viewform?edit2=`+tokenEditar+`">Editar sua resposta</a></body></html>`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	s.servidorFicticio = &servidorFicticio{Server: srv}
	return s
}

func abrirForm(t *testing.T, n *Navegador, nome string) *pagina {
	t.Helper()
	pg, err := n.NovaPagina(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Fechar() })
	if _, err := pg.Abrir(context.Background(), "https://docs.google.com/forms/d/e/1FAIpQLSf_"+nome+"/viewform"); err != nil {
		t.Fatal(err)
	}
	return pg.(*pagina)
}

// A página do formulário, passo a passo: relê a estrutura, responde cada tipo pela opção exata, desmarca a cópia,
// clica uma vez em Enviar (nunca em Limpar) e vê a frase de resposta registrada.
func TestPaginaDoFormularioComChrome(t *testing.T) {
	srv := novoServidorDeEnvio(t)
	n := navegadorDeTeste(t, srv.servidorFicticio, t.TempDir())
	ctx := context.Background()

	p := abrirForm(t, n, "envio")
	l, err := p.LerEstrutura(ctx)
	if err != nil || l.Bloqueio != "" || len(l.Perguntas) != 6 || l.URLFinal != "https://docs.google.com/forms/d/e/1FAIpQLSf_envio/viewform" {
		t.Fatalf("estrutura = %+v %v", l, err)
	}
	if ids, err := p.RespondidasPelaPagina(ctx); err != nil || len(ids) != 0 {
		t.Fatalf("respondidas = %v %v", ids, err)
	}
	if marcada, err := p.CopiaMarcada(ctx); err != nil || marcada {
		t.Fatalf("cópia = %v %v", marcada, err)
	}
	pergunta := func(id string) dominio.PerguntaFormulario {
		for _, q := range l.Perguntas {
			if q.ID == id {
				return q
			}
		}
		t.Fatalf("sem %s", id)
		return dominio.PerguntaFormulario{}
	}
	// Opção que não existe: ErrSemOpcao, e nada fica marcado.
	for _, r := range []struct{ id, ruim string }{{"entry.2003", "avançado"}, {"entry.2004", "Rust"}, {"entry.2004", "go"}, {"entry.2005", "MG"}, {"entry.2005", "sp"}} {
		if err := p.Responder(ctx, pergunta(r.id), r.ruim); !errors.Is(err, executor.ErrSemOpcao) {
			t.Errorf("%s com %q = %v", r.id, r.ruim, err)
		}
	}
	// Valor que o campo não aceita (data inválida): erro, nunca "respondeu".
	if err := p.Responder(ctx, pergunta("entry.2006"), "2026-13-45"); err == nil || errors.Is(err, executor.ErrSemOpcao) {
		t.Errorf("data inválida = %v", err)
	}
	if ids, _ := p.RespondidasPelaPagina(ctx); len(ids) != 0 {
		t.Fatalf("opção errada marcou algo: %v", ids)
	}
	for _, r := range []struct{ id, valor string }{{"entry.2001", "pessoa@exemplo.com"}, {"entry.2003", "Avançado"}, {"entry.2004", "Go"}, {"entry.2005", "SP"}, {"entry.2006", "2026-12-01"}} {
		if err := p.Responder(ctx, pergunta(r.id), r.valor); err != nil {
			t.Fatalf("responder %s = %v", r.id, err)
		}
	}
	if ids, _ := p.RespondidasPelaPagina(ctx); strings.Join(ids, ",") != "entry.2001,entry.2003,entry.2004,entry.2005,entry.2006" {
		t.Fatalf("respondidas depois = %v", ids)
	}
	if err := p.Responder(ctx, dominio.PerguntaFormulario{ID: "entry.9999", Tipo: "texto-curto"}, "x"); err == nil || errors.Is(err, executor.ErrSemOpcao) {
		t.Fatalf("pergunta que não existe = %v", err)
	}
	if err := p.Responder(ctx, dominio.PerguntaFormulario{ID: `entry.1"]`, Tipo: "texto-curto"}, "x"); err == nil {
		t.Fatal("id fora do formato foi para a página")
	}
	if err := p.EnviarFormulario(ctx); err != nil {
		t.Fatal(err)
	}
	if ok, err := p.RespostaRegistrada(ctx); !ok || err != nil {
		t.Fatalf("resposta registrada = %v %v", ok, err)
	}
	got := srv.respostas()
	if len(got) != 1 {
		t.Fatalf("envios = %d", len(got))
	}
	quer := url.Values{"entry.2001": {"pessoa@exemplo.com"}, "entry.2002": {""}, "entry.2003": {"Avançado"}, "entry.2004": {"Go"}, "entry.2005": {"SP"}, "entry.2006": {"2026-12-01"}}
	if fmt.Sprint(got[0]) != fmt.Sprint(quer) {
		t.Fatalf("o servidor recebeu %v; queria %v", got[0], quer)
	}
	if srv.limpou != 0 {
		t.Fatal("clicou em Limpar formulário")
	}

	// Cópia por e-mail marcada pela página: desmarca e confere.
	c := abrirForm(t, n, "copia")
	if marcada, _ := c.CopiaMarcada(ctx); !marcada {
		t.Fatal("não viu a cópia marcada")
	}
	if err := c.DesmarcarCopia(ctx); err != nil {
		t.Fatal(err)
	}
	if marcada, _ := c.CopiaMarcada(ctx); marcada {
		t.Fatal("a cópia continuou marcada")
	}
	// Valor que a página trouxe: o id volta, o valor não.
	pr := abrirForm(t, n, "preenchido")
	if ids, err := pr.RespondidasPelaPagina(ctx); err != nil || strings.Join(ids, ",") != "entry.2002" {
		t.Fatalf("preenchido = %v %v", ids, err)
	}
	// Sem o botão Enviar: nada clicado (nunca o Limpar).
	sb := abrirForm(t, n, "sembotao")
	if err := sb.EnviarFormulario(ctx); !errors.Is(err, executor.ErrSemBotaoEnviar) {
		t.Fatalf("sem botão = %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	if len(srv.respostas()) != 1 || srv.limpou != 0 {
		t.Fatalf("sem botão enviou ou limpou (%d, %d)", len(srv.respostas()), srv.limpou)
	}
}

// Ponta a ponta, pelo caminho da tela: a pessoa cria a candidatura, lê o formulário, responde o que ficou pendente e
// aprova; o executor envia com o Chrome de verdade. O servidor fictício recebe só o pacote, uma vez, e o link de
// editar nunca vai para o banco nem para o log.
func TestIntegradoFormulario(t *testing.T) {
	var logGlobal bytes.Buffer
	log.SetOutput(&logGlobal)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	srv := novoServidorDeEnvio(t)
	dados := t.TempDir()
	nav := navegadorDeTeste(t, srv.servidorFicticio, dados)
	arm, err := armazem.Abrir(context.Background(), filepath.Join(dados, "central.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	pastaCV := filepath.Join(dados, "curriculos")
	os.MkdirAll(pastaCV, 0o700)
	os.WriteFile(filepath.Join(pastaCV, "cv-pt.pdf"), pdf, 0o600)
	if err := arm.Gravar(context.Background(), func(e dominio.Estado) (dominio.Estado, error) {
		e.Perfil = dominio.Objeto{"respostasPadrao": []any{
			dominio.Objeto{"pergunta": "Nível de inglês", "resposta": "Avançado"},
			dominio.Objeto{"pergunta": "Tecnologias", "resposta": "Go"},
			dominio.Objeto{"pergunta": "Estado", "resposta": "SP"},
		}}
		return e, nil
	}); err != nil {
		t.Fatal(err)
	}

	var logsExec []string
	x := executor.Novo(arm, executor.Opcoes{
		Navegador: nav, PastaCurriculos: pastaCV,
		Esperar: func(ctx context.Context, d time.Duration) error { return ctx.Err() },
		Log:     func(f string, a ...any) { logsExec = append(logsExec, fmt.Sprintf(f, a...)) },
	})
	a := api.Nova(arm, api.Opcoes{Cofre: segredos.Memoria{}, LerFormulario: nav.LerFormulario})
	h := a.Handler()
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

	link := "https://docs.google.com/forms/d/e/1FAIpQLSf_envio/viewform?entry.2002=Pre+preenchido"
	if st, r := tela("POST", "/vagas", map[string]any{"id": "v-form-1", "empresa": "Empresa Ficticia", "titulo": "Dev Go", "plataforma": "Site da empresa", "link": link}); st != 201 && st != 200 {
		t.Fatalf("criar vaga = %d %v", st, r)
	}
	if st, r := tela("POST", "/vagas/v-form-1/mover", map[string]any{"etapa": "aplicar"}); st != 200 {
		t.Fatalf("mover = %d %v", st, r)
	}
	conteudo := map[string]any{
		"curriculo": map[string]any{"arquivo": "cv-pt.pdf"},
		"idioma":    "pt",
		"contato":   map[string]any{"nome": "Pessoa Teste", "email": "pessoa@exemplo.com"},
		"respostas": []any{},
	}
	st, c := tela("POST", "/candidaturas", map[string]any{"vagaId": "v-form-1", "conteudo": conteudo})
	if st != 201 || c["plataforma"] != "formulario" {
		t.Fatalf("criar candidatura = %d %v", st, c)
	}
	id := c["id"].(string)
	st, c = tela("POST", "/candidaturas/"+id+"/ler-formulario", map[string]any{})
	if st != 200 || fmt.Sprint(c["pendentes"]) != "[entry.2001]" {
		t.Fatalf("ler-formulario = %d %v", st, c)
	}
	// A pessoa responde a pendente (a pergunta de e-mail é texto curto no formato do Google) e aprova.
	cont := c["conteudo"].(map[string]any)
	cont["respostas"] = append(cont["respostas"].([]any), map[string]any{"pergunta": "Qual o seu e-mail?", "resposta": "pessoa@exemplo.com", "origem": "pessoa", "perguntaId": "entry.2001"})
	if st, r := tela("PUT", "/candidaturas/"+id+"/conteudo", map[string]any{"conteudo": cont, "versao": c["versao"]}); st != 200 || r["pendentes"] != nil {
		t.Fatalf("responder = %d %v", st, r)
	}
	_, c = tela("GET", "/candidaturas/"+id, nil)
	if st, r := tela("POST", "/candidaturas/"+id+"/aprovar", map[string]any{"impressao": c["impressao"]}); st != 200 {
		t.Fatalf("aprovar = %d %v", st, r)
	}
	if st, _ := tela("POST", "/envio/retomar", map[string]any{}); st != 200 {
		t.Fatal("retomar")
	}
	ctx, cancelar := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancelar()
	if _, err := x.EnviarAprovadas(ctx); err != nil {
		t.Fatal(err)
	}
	_, c = tela("GET", "/candidaturas/"+id, nil)
	if c["estado"] != "enviada" {
		t.Fatalf("estado = %v (log: %v)", c["estado"], logsExec)
	}
	got := srv.respostas()
	if len(got) != 1 {
		t.Fatalf("envios = %d", len(got))
	}
	quer := url.Values{"entry.2001": {"pessoa@exemplo.com"}, "entry.2002": {""}, "entry.2003": {"Avançado"}, "entry.2004": {"Go"}, "entry.2005": {"SP"}, "entry.2006": {""}}
	if fmt.Sprint(got[0]) != fmt.Sprint(quer) {
		t.Fatalf("o servidor recebeu %v; queria só o pacote %v", got[0], quer)
	}
	// Rodar de novo não reenvia.
	if _, err := x.EnviarAprovadas(ctx); err != nil || len(srv.respostas()) != 1 {
		t.Fatalf("reenviou (%d) %v", len(srv.respostas()), err)
	}
	// Nada do formulário nem do pacote no log; o link de editar em lugar nenhum.
	todos := logGlobal.String() + "\n" + strings.Join(logsExec, "\n")
	for _, proibido := range []string{"pessoa@exemplo.com", "Pessoa Teste", "Avançado", "1FAIpQLSf", "docs.google", "entry.", tokenEditar, "Pre+preenchido", "Qual o seu"} {
		if strings.Contains(todos, proibido) {
			t.Errorf("log contém %q", proibido)
		}
	}
	_, est := tela("GET", "/estado", nil)
	b, _ := json.Marshal(est)
	if strings.Contains(string(b), tokenEditar) || strings.Contains(string(b), "edit2") {
		t.Fatal("o link de editar foi guardado")
	}
	chaves := []string{}
	for k := range got[0] {
		chaves = append(chaves, k)
	}
	sort.Strings(chaves)
	if strings.Contains(strings.Join(chaves, ","), "sendMeACopy") || strings.Contains(strings.Join(chaves, ","), "emailAddress") {
		t.Fatalf("enviou a cópia ou o e-mail da conta: %v", chaves)
	}
}
