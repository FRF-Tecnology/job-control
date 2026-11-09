package executor

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/curriculos"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// jc-a2y.7.2, parte 3b: o envio do formulário com página falsa. Nada contra o Google ou o Typeform; dados fictícios.

const formFicticio = "https://docs.google.com/forms/d/e/1FAIpQLSf_ficticio/viewform"

func perguntasDoForm() []dominio.PerguntaFormulario {
	return []dominio.PerguntaFormulario{
		{ID: "entry.1", Texto: "Qual o seu e-mail?", Tipo: "email", Obrigatoria: true},
		{ID: "entry.2", Texto: "Quantos anos de Go você tem?", Tipo: "texto-curto", Obrigatoria: true},
		{ID: "entry.3", Texto: "Nível de inglês", Tipo: "escolha-unica", Obrigatoria: true, Opcoes: []string{"Básico", "Avançado"}},
		{ID: "entry.4", Texto: "Comentários", Tipo: "texto-longo"}, // opcional, sem resposta: fica em branco
	}
}

// paginaForm: a página falsa do formulário (a Pagina de sempre e a PaginaFormulario).
type paginaForm struct {
	*paginaFalsa
	leitura       dominio.LeituraFormulario
	errLer        error
	respondidas   []string
	copia         []bool // CopiaMarcada por chamada (vazio: false)
	errDesmarcar  error
	errResponder  map[string]error
	semBotao      bool
	errEnviarForm error
	registrada    bool
	aoResponder   func()
	aoEnviarForm  func()
	aoDesmarcar   func()

	chamadasCopia int
	leituras      int
	respostas     map[string]string
	ordem         []string
	enviouForm    int
}

func novaPaginaForm() *paginaForm {
	p := &paginaForm{paginaFalsa: novaPagina(), registrada: true, respostas: map[string]string{}}
	p.leitura = dominio.LeituraFormulario{Perguntas: perguntasDoForm(), URLFinal: formFicticio}
	return p
}

func (p *paginaForm) LerEstrutura(ctx context.Context) (dominio.LeituraFormulario, error) {
	p.leituras++
	return p.leitura, p.errLer
}
func (p *paginaForm) RespondidasPelaPagina(ctx context.Context) ([]string, error) {
	return p.respondidas, nil
}
func (p *paginaForm) CopiaMarcada(ctx context.Context) (bool, error) {
	i := p.chamadasCopia
	p.chamadasCopia++
	if i < len(p.copia) {
		return p.copia[i], nil
	}
	return false, nil
}
func (p *paginaForm) DesmarcarCopia(ctx context.Context) error {
	if p.aoDesmarcar != nil {
		p.aoDesmarcar()
	}
	return p.errDesmarcar
}
func (p *paginaForm) Responder(ctx context.Context, q dominio.PerguntaFormulario, v string) error {
	if p.aoResponder != nil {
		p.aoResponder()
	}
	if err := p.errResponder[q.ID]; err != nil {
		return err
	}
	p.respostas[q.ID] = v
	p.ordem = append(p.ordem, q.ID)
	return nil
}
func (p *paginaForm) EnviarFormulario(ctx context.Context) error {
	if p.aoEnviarForm != nil {
		p.aoEnviarForm()
	}
	if p.semBotao {
		return ErrSemBotaoEnviar
	}
	p.enviouForm++
	return p.errEnviarForm
}
func (p *paginaForm) RespostaRegistrada(ctx context.Context) (bool, error) { return p.registrada, nil }

// navegadorDeForms entrega páginas de formulário (ou uma Pagina comum, para o caso sem a interface).
type navegadorDeForms struct {
	paginas []Pagina
	abertas int
}

func (n *navegadorDeForms) EnviaFormulario() bool { return true }

func (n *navegadorDeForms) NovaPagina(ctx context.Context) (Pagina, error) {
	if n.abertas >= len(n.paginas) {
		return nil, errors.New("sem página falsa")
	}
	p := n.paginas[n.abertas]
	n.abertas++
	return p, nil
}

func cenarioDeForms(t *testing.T, paginas ...Pagina) *cenario {
	t.Helper()
	s := novoCenario(t)
	s.x.o.Navegador = &navegadorDeForms{paginas: paginas}
	return s
}

// formularioAprovado: vaga de formulário fictícia, estrutura lida (as respostas vêm das respostasPadrao e do
// contato) e a candidatura aprovada pela pessoa.
func (s *cenario) formularioAprovado(link string) string {
	s.t.Helper()
	s.n++
	vaga := "fv-" + strings.Repeat("x", s.n)
	var id string
	s.gravar(func(e dominio.Estado) (dominio.Estado, error) {
		e.Perfil = dominio.Objeto{"respostasPadrao": []any{
			dominio.Objeto{"pergunta": "Quantos anos de Go você tem?", "resposta": marca},
			dominio.Objeto{"pergunta": "Nível de inglês", "resposta": "Avançado"},
		}}
		e, _, err := dominio.AdicionarVaga(e, dominio.Objeto{"id": vaga, "empresa": "Empresa " + vaga, "titulo": "Dev Go " + vaga, "plataforma": "Site da empresa", "link": link}, eu(s.agora), false)
		if err != nil {
			return e, err
		}
		if e, _, err = dominio.MoverVaga(e, vaga, "aplicar", nil, eu(s.agora)); err != nil {
			return e, err
		}
		conteudo := dominio.Objeto{
			"curriculo": dominio.Objeto{"arquivo": "cv-pt.pdf", "sha256": curriculos.SHA256(pdf)},
			"idioma":    "pt",
			"contato":   dominio.Objeto{"nome": "Pessoa Teste", "email": "pessoa@exemplo.com", "telefone": "11900000000", "cidade": "Cidade Teste"},
			"respostas": []any{},
			"mensagem":  nil,
		}
		e, o, err := dominio.CriarCandidatura(e, vaga, conteudo, s.agora, eu(s.agora))
		if err != nil {
			return e, err
		}
		id = o["id"].(string)
		if e, o, err = dominio.GravarEstruturaFormulario(e, id, perguntasDoForm(), formFicticio, s.agora); err != nil {
			return e, err
		}
		limites, _ := s.arm.LimitesCandidatura(context.Background())
		e, _, err = dominio.AprovarCandidatura(e, id, o["impressao"].(string), s.agora, limites, eu(s.agora))
		return e, err
	})
	return id
}

// O pacote vai, só ele, na ordem das perguntas, com as marcas gravadas antes de cada passo.
func TestFormularioEnviaSoOPacote(t *testing.T) {
	p := novaPaginaForm()
	s := cenarioDeForms(t, p)
	id := s.formularioAprovado(formFicticio)
	var noResponder, noEnviar dominio.Objeto
	p.aoResponder = func() {
		if noResponder == nil {
			noResponder = s.cand(id)
		}
	}
	p.aoEnviarForm = func() { noEnviar = s.cand(id) }
	d := s.enviar(id)
	if d.Estado != dominio.CandEnviada || s.cand(id)["estado"] != dominio.CandEnviada {
		t.Fatalf("desfecho = %+v", d)
	}
	if strings.Join(p.ordem, ",") != "entry.1,entry.2,entry.3" || p.respostas["entry.1"] != "pessoa@exemplo.com" || p.respostas["entry.2"] != marca || p.respostas["entry.3"] != "Avançado" {
		t.Fatalf("respostas = %v %v", p.ordem, p.respostas)
	}
	if p.enviouForm != 1 || p.enviou != 0 {
		t.Fatalf("cliques = %d (form) %d (Pagina.Enviar)", p.enviouForm, p.enviou)
	}
	if dominio.Texto(noResponder["interagiuEm"]) == "" || noResponder["clicouEnviarEm"] != nil {
		t.Fatalf("respondeu sem a marca interagiu (ou com clicou-enviar): %v", noResponder)
	}
	if dominio.Texto(noEnviar["clicouEnviarEm"]) == "" {
		t.Fatal("clicou em enviar sem a marca gravada")
	}
	if !p.fechada {
		t.Fatal("a página ficou aberta")
	}
	// Conta no limite do dia de formulario, e nada muda no estado das plataformas.
	e, _ := s.arm.Carregar(context.Background())
	if n := dominio.UsadasNoDia(e, dominio.PlataformaFormulario, s.agora); n != 1 {
		t.Fatalf("usadas no dia = %d", n)
	}
	if _, tem := s.arm.Plataformas(context.Background())[dominio.PlataformaFormulario]; tem {
		t.Fatal("formulario ganhou estado de plataforma")
	}
	s.logSemConteudo()
	semFormularioNoLog(t, s)
}

// Nada do formulário no log: nem o endereço, nem o id, nem as perguntas.
func semFormularioNoLog(t *testing.T, s *cenario) {
	t.Helper()
	tudo := strings.Join(s.logs, "\n")
	for _, proibido := range []string{"docs.google", "forms.gle", "1FAIpQLSf", "entry.", "Avançado", "inglês", "Quantos anos"} {
		if strings.Contains(tudo, proibido) {
			t.Errorf("log tem %q:\n%s", proibido, tudo)
		}
	}
}

// Antes de qualquer marca: estrutura relida diferente, leitura com bloqueio, destino diferente, campo que a página
// já trouxe ou valor fora das opções param, sem responder nada e sem a marca "interagiu".
func TestFormularioBloqueiaAntesDeInteragir(t *testing.T) {
	mudou := novaPaginaForm()
	mudou.leitura.Perguntas[1].Texto = "Quantos anos de Go e Rust você tem?"
	tipoMudou := novaPaginaForm()
	tipoMudou.leitura.Perguntas[2].Opcoes = []string{"Básico", "Intermediário"}
	login := novaPaginaForm()
	login.leitura = dominio.LeituraFormulario{Bloqueio: "login"}
	outroDestino := novaPaginaForm()
	outroDestino.leitura.URLFinal = "https://docs.google.com/forms/d/e/1FAIpQLSf_outro/viewform"
	paginaPreencheu := novaPaginaForm()
	paginaPreencheu.respondidas = []string{"entry.4"}
	abriuOutro := novaPaginaForm()
	abriuOutro.final = "https://docs.google.com/forms/d/e/1FAIpQLSf_outro/viewform"
	contaGoogle := novaPaginaForm()
	contaGoogle.final = "https://accounts.google.com/v3/signin/identifier"
	fora := novaPaginaForm()
	fora.final = "https://pagamentos.exemplo.com/form"
	casos := []struct {
		nome   string
		pagina *paginaForm
		motivo string
		semLer bool // o destino errado para antes de reler a página
	}{
		{"estrutura mudou", mudou, "pergunta-nova", false},
		{"opções mudaram", tipoMudou, "pergunta-nova", false},
		{"login na leitura", login, "login", false},
		{"destino lido diferente", outroDestino, "site-externo", false},
		{"campo que a página trouxe", paginaPreencheu, "pergunta-nova", false},
		{"abriu outro formulário", abriuOutro, "site-externo", true},
		{"login da conta Google", contaGoogle, "login", true},
		{"fora da lista", fora, "site-externo", true},
	}
	paginas := []Pagina{}
	for _, c := range casos {
		paginas = append(paginas, c.pagina)
	}
	s := cenarioDeForms(t, paginas...)
	for _, c := range casos {
		id := s.formularioAprovado(formFicticio)
		d := s.enviar(id)
		got := s.cand(id)
		if d.Estado != dominio.CandBloqueada || d.Motivo != c.motivo || got["estado"] != dominio.CandBloqueada {
			t.Errorf("%s: desfecho = %+v; queria bloqueada/%s", c.nome, d, c.motivo)
		}
		if c.semLer && c.pagina.leituras != 0 {
			t.Errorf("%s: releu a página antes de conferir o destino", c.nome)
		}
		if len(c.pagina.ordem) != 0 || c.pagina.enviouForm != 0 || got["interagiuEm"] != nil {
			t.Errorf("%s: interagiu (%v, %d cliques, marca %v)", c.nome, c.pagina.ordem, c.pagina.enviouForm, got["interagiuEm"])
		}
	}
	// O login do formulário fica bloqueado (sem voltar), e nada pausa os outros formulários.
	if _, tem := s.arm.Plataformas(context.Background())[dominio.PlataformaFormulario]; tem {
		t.Fatal("formulario ganhou estado de plataforma")
	}
	s.logSemConteudo()
	semFormularioNoLog(t, s)
}

// O forms.gle abre o destino canônico guardado na leitura: o endereço confere contra ele.
func TestFormularioPeloLinkCurto(t *testing.T) {
	p := novaPaginaForm()
	p.final = formFicticio // o forms.gle redirecionou
	s := cenarioDeForms(t, p)
	id := s.formularioAprovado("https://forms.gle/AbCficticio1")
	if d := s.enviar(id); d.Estado != dominio.CandEnviada {
		t.Fatalf("forms.gle = %+v", d)
	}
}

// A caixa da cópia por e-mail: marcada pela página, o app grava a marca, desmarca e confere antes do clique; erro ao
// desmarcar ou caixa que continua marcada = falhou, sem clicar em enviar.
func TestFormularioCopiaPorEmail(t *testing.T) {
	desmarca := novaPaginaForm()
	desmarca.copia = []bool{true, false}
	erro := novaPaginaForm()
	erro.copia = []bool{true}
	erro.errDesmarcar = errors.New("caixa não respondeu")
	voltou := novaPaginaForm()
	voltou.copia = []bool{true, true}
	s := cenarioDeForms(t, desmarca, erro, voltou)

	id := s.formularioAprovado(formFicticio)
	var noDesmarcar dominio.Objeto
	desmarca.aoDesmarcar = func() { noDesmarcar = s.cand(id) }
	if d := s.enviar(id); d.Estado != dominio.CandEnviada {
		t.Fatalf("cópia desmarcada = %+v", d)
	}
	if dominio.Texto(noDesmarcar["interagiuEm"]) == "" {
		t.Fatal("desmarcou a cópia sem a marca gravada")
	}
	for _, k := range []struct {
		nome string
		p    *paginaForm
	}{{"erro ao desmarcar", erro}, {"continuou marcada", voltou}} {
		nome, p := k.nome, k.p
		id := s.formularioAprovado(formFicticio)
		d := s.enviar(id)
		if d.Estado != dominio.CandFalhou || d.Motivo != "erro-tecnico" || p.enviouForm != 0 {
			t.Errorf("%s = %+v (%d cliques)", nome, d, p.enviouForm)
		}
	}
	s.logSemConteudo()
}

// Do clique em diante: sem botão, bloqueada/outro (nada clicado); clique que falha ou sem a frase de resposta
// registrada, incerta, nunca outra tentativa. Valor fora das opções na página: pergunta-nova.
func TestFormularioCliqueEConfirmacao(t *testing.T) {
	semBotao := novaPaginaForm()
	semBotao.semBotao = true
	falhaClique := novaPaginaForm()
	falhaClique.errEnviarForm = errors.New("clique falhou")
	semFrase := novaPaginaForm()
	semFrase.registrada = false
	semOpcao := novaPaginaForm()
	semOpcao.errResponder = map[string]error{"entry.3": ErrSemOpcao}
	quebrou := novaPaginaForm()
	quebrou.errResponder = map[string]error{"entry.2": errors.New("campo sumiu")}
	casos := []struct {
		nome          string
		pagina        *paginaForm
		estado, extra string
	}{
		{"sem botão", semBotao, dominio.CandBloqueada, "outro"},
		{"clique falhou", falhaClique, dominio.CandIncerta, ""},
		{"sem a frase", semFrase, dominio.CandIncerta, ""},
		{"sem a opção", semOpcao, dominio.CandBloqueada, "pergunta-nova"},
		{"campo sumiu", quebrou, dominio.CandFalhou, "erro-tecnico"},
	}
	paginas := []Pagina{}
	for _, c := range casos {
		paginas = append(paginas, c.pagina)
	}
	s := cenarioDeForms(t, paginas...)
	for _, c := range casos {
		id := s.formularioAprovado(formFicticio)
		d := s.enviar(id)
		if d.Estado != c.estado || d.Motivo != c.extra || len(dominio.ListaDe(s.cand(id)["retentativas"])) != 0 {
			t.Errorf("%s = %+v; queria %s/%s, sem volta", c.nome, d, c.estado, c.extra)
		}
	}
	if semOpcao.respostas["entry.3"] != "" || quebrou.enviouForm != 0 || semOpcao.enviouForm != 0 {
		t.Fatal("outro valor foi no lugar, ou clicou depois de erro")
	}
	s.logSemConteudo()
}

// O endereço é conferido a cada resposta e logo antes do clique.
func TestFormularioEnderecoNoMeio(t *testing.T) {
	saiu := novaPaginaForm()
	saiu.urls = []string{formFicticio, "https://exemplo.com/outra"} // sai depois da primeira resposta
	antes := novaPaginaForm()
	antes.urls = []string{formFicticio, formFicticio, formFicticio, "https://exemplo.com/outra"} // sai antes do clique
	s := cenarioDeForms(t, saiu, antes)
	for _, k := range []struct {
		nome string
		p    *paginaForm
	}{{"no meio", saiu}, {"antes do clique", antes}} {
		nome, p := k.nome, k.p
		id := s.formularioAprovado(formFicticio)
		if d := s.enviar(id); d.Estado != dominio.CandBloqueada || d.Motivo != "site-externo" || p.enviouForm != 0 {
			t.Errorf("%s = %+v (%d cliques)", nome, d, p.enviouForm)
		}
	}
	if len(saiu.ordem) != 1 || len(antes.ordem) != 3 {
		t.Fatalf("respostas = %v / %v", saiu.ordem, antes.ordem)
	}
}

// Página sem a interface do formulário: erro técnico antes de abrir qualquer coisa, e a candidatura volta para
// aprovada (nada foi feito na página). Leitura que falha: o mesmo.
func TestFormularioSemAPaginaDoFormulario(t *testing.T) {
	comum := novaPagina()
	lerFalha := novaPaginaForm()
	lerFalha.errLer = errors.New("script")
	s := cenarioDeForms(t, comum, lerFalha)
	for _, k := range []struct {
		nome  string
		abriu func() bool
	}{{"sem a interface", func() bool { return comum.fechada }}, {"leitura falhou", func() bool { return lerFalha.fechada }}} {
		nome, abriu := k.nome, k.abriu
		id := s.formularioAprovado(formFicticio)
		d := s.enviar(id)
		if d.Estado != dominio.CandAprovada || s.cand(id)["interagiuEm"] != nil || !abriu() {
			t.Errorf("%s = %+v (marca %v)", nome, d, s.cand(id)["interagiuEm"])
		}
	}
	if comum.enviou != 0 || len(comum.preenchidos) != 0 {
		t.Fatal("a página comum foi usada para enviar o formulário")
	}
}

// Sem o destino guardado na leitura, nem abre página: bloqueada/outro.
func TestFormularioSemDestinoGuardado(t *testing.T) {
	p := novaPaginaForm()
	s := cenarioDeForms(t, p)
	id := s.formularioAprovado(formFicticio)
	c := s.cand(id)
	delete(c, "estrutura")
	if r := s.x.preencherFormulario(context.Background(), c, &tentativa{x: s.x, id: id}); r.estado != dominio.CandBloqueada || r.motivo != "outro" {
		t.Fatalf("sem destino = %+v", r)
	}
	if s.x.o.Navegador.(*navegadorDeForms).abertas != 0 {
		t.Fatal("abriu página sem o destino guardado")
	}
}

// PO, na 3b-1: com um navegador que não envia formulário (o de verdade até a 3b-2), o formulário aprovado não vira
// laço: nem entra em enviando, fica aprovada, sem tentativa, sem volta e sem página (nada no limite nem no teto). As
// outras plataformas da fila seguem.
func TestFormularioSemNavegadorDeFormularioEspera(t *testing.T) {
	li := novaPagina()
	s := novoCenario(t, li) // navegadorFalso: não implementa NavegadorDeFormulario
	form := s.formularioAprovado(formFicticio)
	outra := s.aprovada("linkedin", linkLI)
	for i := 0; i < 3; i++ { // ciclos seguidos da fila
		if _, err := s.x.EnviarAprovadas(context.Background()); err != nil {
			t.Fatal(err)
		}
		s.agora = s.agora.Add(3 * time.Hour)
	}
	c := s.cand(form)
	if c["estado"] != dominio.CandAprovada || len(dominio.ListaDe(c["tentativas"])) != 0 || len(dominio.ListaDe(c["retentativas"])) != 0 || c["proximaTentativaEm"] != nil {
		t.Fatalf("formulário = %v %v %v %v", c["estado"], c["tentativas"], c["retentativas"], c["proximaTentativaEm"])
	}
	e, _ := s.arm.Carregar(context.Background())
	if n := dominio.AberturasNoDia(e, dominio.PlataformaFormulario, s.agora); n != 0 {
		t.Fatalf("aberturas de formulário = %d", n)
	}
	if s.nav.abertas != 1 || s.cand(outra)["estado"] != dominio.CandEnviada {
		t.Fatalf("páginas abertas = %d; linkedin = %v", s.nav.abertas, s.cand(outra)["estado"])
	}
	if d, err := s.x.Enviar(context.Background(), form); !errors.Is(err, ErrSemEnvioDeFormulario) || d != (Desfecho{}) {
		t.Fatalf("Enviar direto = %+v %v", d, err)
	}
	s.logSemConteudo()
	semFormularioNoLog(t, s)
}
