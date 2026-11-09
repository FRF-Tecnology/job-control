package executor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/curriculos"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Navegador e páginas falsos: nenhuma página real, nenhuma candidatura real. Dados fictícios.

const marca = "Resposta-Ficticia-Marcada-9c1e" // não pode ir para o log

var pdf = []byte("%PDF-1.4\n% curriculo ficticio de teste\n")

type paginaFalsa struct {
	final        string
	errAbrir     error
	bloqueios    []string // um por chamada de Bloqueio
	campos       []Campo
	errPreencher error
	errPorCampo  map[string]error // erro do Preencher só naquele campo (ex.: ErrSemOpcao)
	errEnviar    error
	confirma     bool
	aoPreencher  func()
	etapas       [][]Campo // formulário em etapas (vazio: uma etapa só, com campos)
	urls         []string  // URLAtual por chamada (vazio: final)
	aoAnexar     func()

	chamadasBloqueio int
	etapa            int
	chamadasURL      int
	preenchidos      map[string]string
	anexos           map[string][]byte
	caminhosAnexo    []string
	enviou           int
	fechada          bool
}

func (p *paginaFalsa) Abrir(ctx context.Context, u string) (string, error) {
	if p.final == "" {
		p.final = u
	}
	return p.final, p.errAbrir
}
func (p *paginaFalsa) Bloqueio(ctx context.Context) (string, error) {
	i := p.chamadasBloqueio
	p.chamadasBloqueio++
	if i < len(p.bloqueios) {
		return p.bloqueios[i], nil
	}
	return "", nil
}
func (p *paginaFalsa) Campos(ctx context.Context) ([]Campo, error) {
	if len(p.etapas) > 0 {
		return p.etapas[p.etapa], nil
	}
	return p.campos, nil
}
func (p *paginaFalsa) URLAtual(ctx context.Context) (string, error) {
	i := p.chamadasURL
	p.chamadasURL++
	if i < len(p.urls) {
		return p.urls[i], nil
	}
	return p.final, nil
}
func (p *paginaFalsa) Proxima(ctx context.Context) (bool, error) {
	if p.etapa+1 < len(p.etapas) {
		p.etapa++
		return true, nil
	}
	return false, nil
}
func (p *paginaFalsa) Preencher(ctx context.Context, c Campo, v string) error {
	if p.aoPreencher != nil {
		p.aoPreencher()
	}
	if p.errPreencher != nil {
		return p.errPreencher
	}
	if err := p.errPorCampo[c.ID]; err != nil {
		return err
	}
	p.preenchidos[c.ID] = v
	return nil
}
func (p *paginaFalsa) Anexar(ctx context.Context, c Campo, caminho string) error {
	if p.aoAnexar != nil {
		p.aoAnexar()
	}
	b, err := os.ReadFile(caminho)
	if err != nil {
		return err
	}
	p.anexos[c.ID] = b
	p.caminhosAnexo = append(p.caminhosAnexo, caminho)
	return nil
}
func (p *paginaFalsa) Enviar(ctx context.Context) error {
	p.enviou++
	return p.errEnviar
}
func (p *paginaFalsa) Confirmacao(ctx context.Context) (bool, error) { return p.confirma, nil }
func (p *paginaFalsa) Fechar() error                                 { p.fechada = true; return nil }

type navegadorFalso struct {
	paginas []*paginaFalsa
	abertas int
	ocupado bool // janela de login da pessoa aberta
}

func (n *navegadorFalso) Ocupado() bool { return n.ocupado }

func (n *navegadorFalso) NovaPagina(ctx context.Context) (Pagina, error) {
	if n.abertas >= len(n.paginas) {
		return nil, errors.New("sem página falsa")
	}
	p := n.paginas[n.abertas]
	n.abertas++
	return p, nil
}

func novaPagina() *paginaFalsa {
	return &paginaFalsa{confirma: true, preenchidos: map[string]string{}, anexos: map[string][]byte{},
		campos: []Campo{
			{ID: "nome", Tipo: CampoNome, Obrigatorio: true},
			{ID: "email", Tipo: CampoEmail, Obrigatorio: true},
			{ID: "cv", Tipo: CampoCurriculo, Obrigatorio: true},
			{ID: "anos", Tipo: CampoPergunta, Rotulo: "  anos de GO? *", Obrigatorio: true},
			{ID: "pretensao", Tipo: CampoPretensao},
			{ID: "msg", Tipo: CampoMensagem},                              // opcional e vazia no pacote: fica em branco
			{ID: "como", Tipo: CampoOutro, Rotulo: "Como soube da vaga?"}, // opcional fora do pacote: em branco
		}}
}

type cenario struct {
	t       *testing.T
	arm     *armazem.Armazem
	pastaCV string
	agora   time.Time
	nav     *navegadorFalso
	x       *Executor
	logs    []string
	esperas []time.Duration
	n       int
}

var sp = func() *time.Location { l, _ := time.LoadLocation("America/Sao_Paulo"); return l }()

func novoCenario(t *testing.T, paginas ...*paginaFalsa) *cenario {
	t.Helper()
	dir := t.TempDir()
	arm, err := armazem.Abrir(context.Background(), filepath.Join(dir, "central.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	// O envio começa pausado (jc-a2y.51); estes testes são do envio liberado.
	if err := arm.GravarEnvioPausado(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	s := &cenario{t: t, arm: arm, pastaCV: filepath.Join(dir, "curriculos"), agora: time.Date(2026, 11, 9, 10, 0, 0, 0, sp), nav: &navegadorFalso{paginas: paginas}}
	os.Mkdir(s.pastaCV, 0o700)
	if err := os.WriteFile(filepath.Join(s.pastaCV, "cv-pt.pdf"), pdf, 0o600); err != nil {
		t.Fatal(err)
	}
	s.x = Novo(arm, Opcoes{
		Navegador: s.nav, PastaCurriculos: s.pastaCV,
		EntreAcoes:        Intervalo{time.Second, 3 * time.Second},
		EntreCandidaturas: Intervalo{time.Minute, 2 * time.Minute},
		Relogio:           func() time.Time { return s.agora },
		Hoje:              func() string { return s.agora.Format("2006-01-02") },
		Esperar:           func(ctx context.Context, d time.Duration) error { s.esperas = append(s.esperas, d); return ctx.Err() },
		Sortear:           func() float64 { return 0.5 },
		Log:               func(f string, a ...any) { s.logs = append(s.logs, fmt.Sprintf(f, a...)) },
	})
	return s
}

var eu = func(agora time.Time) dominio.Ctx { return dominio.Ctx{Autor: "eu", Agora: agora.Format("2006-01-02")} }

func (s *cenario) gravar(f func(e dominio.Estado) (dominio.Estado, error)) {
	s.t.Helper()
	if err := s.arm.Gravar(context.Background(), f); err != nil {
		s.t.Fatal(err)
	}
}

// aprovada cria uma vaga fictícia aprovada pela pessoa e uma candidatura aprovada para ela; devolve o id.
func (s *cenario) aprovada(plataforma, link string) string {
	s.t.Helper()
	s.n++
	vaga := fmt.Sprintf("v-%d", s.n)
	var id string
	s.gravar(func(e dominio.Estado) (dominio.Estado, error) {
		e, _, err := dominio.AdicionarVaga(e, dominio.Objeto{"id": vaga, "empresa": "Empresa " + vaga, "titulo": "Dev Go " + vaga, "plataforma": plataforma, "link": link}, eu(s.agora), false)
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
			"pretensao": dominio.Objeto{"valor": 8500.0, "moeda": "BRL", "tipo": "PJ"},
			"respostas": []any{
				dominio.Objeto{"pergunta": "Anos de Go?", "resposta": marca, "origem": "pessoa"},
				dominio.Objeto{"pergunta": "Anos de experiência com Go", "resposta": "3", "origem": "respostasPadrao"},
				dominio.Objeto{"pergunta": "Experiência com banco de dados?", "resposta": "Sim", "origem": "respostasPadrao"},
			},
			"mensagem": nil,
		}
		e, o, err := dominio.CriarCandidatura(e, vaga, conteudo, s.agora, eu(s.agora))
		if err != nil {
			return e, err
		}
		id = o["id"].(string)
		limites, _ := s.arm.LimitesCandidatura(context.Background())
		e, _, err = dominio.AprovarCandidatura(e, id, o["impressao"].(string), s.agora, limites, eu(s.agora))
		return e, err
	})
	s.agora = s.agora.Add(time.Second) // aprovações em ordem
	return id
}

func (s *cenario) cand(id string) dominio.Objeto {
	s.t.Helper()
	e, err := s.arm.Carregar(context.Background())
	if err != nil {
		s.t.Fatal(err)
	}
	for _, o := range e.Candidaturas {
		if o["id"] == id {
			return o
		}
	}
	s.t.Fatalf("candidatura %s sumiu", id)
	return nil
}

func (s *cenario) etapa(id string) any {
	e, _ := s.arm.Carregar(context.Background())
	for _, v := range e.Vagas {
		if v["id"] == dominio.Texto(s.cand(id)["vagaId"]) {
			return v["etapa"]
		}
	}
	return nil
}

func (s *cenario) enviar(id string) Desfecho {
	s.t.Helper()
	d, err := s.x.Enviar(context.Background(), id)
	if err != nil {
		s.t.Fatalf("Enviar = %v", err)
	}
	return d
}

// M7: depois de cada teste, nada do pacote, da página ou do disco no log.
func (s *cenario) logSemConteudo() {
	s.t.Helper()
	tudo := strings.Join(s.logs, "\n")
	for _, proibido := range []string{marca, "Pessoa Teste", "pessoa@exemplo.com", "8500", "cv-pt.pdf", s.pastaCV, "linkedin.com", "exemplo", "anos de", "Como soube"} {
		if strings.Contains(tudo, proibido) {
			s.t.Errorf("log tem %q:\n%s", proibido, tudo)
		}
	}
}

const linkLI = "https://www.linkedin.com/jobs/view/9000000001"

func TestEnviaSoOPacoteAprovado(t *testing.T) {
	p := novaPagina()
	p.final = linkLI + "/apply?origem=teste"
	s := novoCenario(t, p)
	id := s.aprovada("linkedin", linkLI)
	d := s.enviar(id)
	if d.Estado != dominio.CandEnviada || s.cand(id)["estado"] != "enviada" || s.etapa(id) != "enviada" {
		t.Fatalf("desfecho = %+v, candidatura %v, vaga %v", d, s.cand(id)["estado"], s.etapa(id))
	}
	quer := map[string]string{"nome": "Pessoa Teste", "email": "pessoa@exemplo.com", "anos": marca, "pretensao": "8500"}
	if fmt.Sprint(p.preenchidos) != fmt.Sprint(quer) {
		t.Fatalf("preenchidos = %v; queria %v", p.preenchidos, quer)
	}
	if string(p.anexos["cv"]) != string(pdf) || p.enviou != 1 || !p.fechada {
		t.Fatalf("anexo/envio/fechada = %q %d %v", p.anexos["cv"], p.enviou, p.fechada)
	}
	// A cópia anexada é temporária e some depois do envio; o original fica.
	if _, err := os.Stat(p.caminhosAnexo[0]); !os.IsNotExist(err) || filepath.Dir(p.caminhosAnexo[0]) == s.pastaCV {
		t.Fatalf("cópia do anexo = %s (%v)", p.caminhosAnexo[0], err)
	}
	// Intervalo entre as ações: cada espera no intervalo configurado.
	if len(s.esperas) < 6 {
		t.Fatalf("esperas = %v", s.esperas)
	}
	for _, d := range s.esperas {
		if d < time.Second || d > 3*time.Second {
			t.Fatalf("espera fora do intervalo: %v", d)
		}
	}
	if _, _, err := (func() (Desfecho, int, error) { d, err := s.x.Enviar(context.Background(), id); return d, 0, err })(); err == nil {
		t.Fatal("enviou duas vezes")
	}
	if s.nav.abertas != 1 {
		t.Fatalf("páginas abertas = %d", s.nav.abertas)
	}
	s.logSemConteudo()
}

// "outra" plataforma nunca abre página e não conta no limite.
func TestOutraPlataformaVaiParaSiteExterno(t *testing.T) {
	s := novoCenario(t)
	id := s.aprovada("Site da empresa", "https://carreiras.exemplo.com/vaga/1")
	d := s.enviar(id)
	c := s.cand(id)
	if d.Estado != dominio.CandBloqueada || d.Motivo != "site-externo" || s.nav.abertas != 0 || c["tentativas"] != nil || s.etapa(id) != "bloqueada" {
		t.Fatalf("desfecho = %+v, abertas %d, candidatura %v", d, s.nav.abertas, c)
	}
	s.logSemConteudo()
}

// M2: currículo trocado na pasta depois da aprovação: falhou sem abrir a página. Troca durante o envio não muda o anexo.
func TestCurriculoConferidoAntesDeAnexar(t *testing.T) {
	s := novoCenario(t, novaPagina())
	id := s.aprovada("linkedin", linkLI)
	os.WriteFile(filepath.Join(s.pastaCV, "cv-pt.pdf"), []byte("%PDF outro"), 0o600)
	d := s.enviar(id)
	c := s.cand(id)
	if d.Estado != dominio.CandFalhou || d.Motivo != "curriculo-mudou" || s.nav.abertas != 0 || len(dominio.ListaDe(c["tentativas"])) != 1 {
		t.Fatalf("desfecho = %+v, abertas %d, tentativas %v", d, s.nav.abertas, c["tentativas"])
	}
	if dominio.ObjetoDe(c["resultado"])["detalhe"] != dominio.DetalheDaFalha["curriculo-mudou"] {
		t.Fatalf("resultado = %v", c["resultado"])
	}
	os.Remove(filepath.Join(s.pastaCV, "cv-pt.pdf"))
	s.gravar(func(e dominio.Estado) (dominio.Estado, error) {
		e, _, err := dominio.ReabrirCandidatura(e, id, s.agora, eu(s.agora))
		return e, err
	})

	p := novaPagina()
	s2 := novoCenario(t, p)
	id2 := s2.aprovada("linkedin", linkLI)
	p.aoAnexar = func() { os.WriteFile(filepath.Join(s2.pastaCV, "cv-pt.pdf"), []byte("%PDF trocado no meio"), 0o600) }
	if d := s2.enviar(id2); d.Estado != dominio.CandEnviada || string(p.anexos["cv"]) != string(pdf) {
		t.Fatalf("anexo = %q (%+v)", p.anexos["cv"], d)
	}
	s.logSemConteudo()
}

// M5: a página tem de parar no site da plataforma e na mesma vaga.
func TestEnderecoConfere(t *testing.T) {
	casos := map[string]bool{
		linkLI:            true,
		linkLI + "/apply": true,
		"https://br.linkedin.com/jobs/view/9000000001":         true,
		"https://www.linkedin.com/jobs/view/9000000002":        false, // outra vaga
		"https://www.linkedin.com/jobs/view/90000000011":       false,
		"https://ats.exemplo.io/vaga/1":                        false,
		"https://linkedin.com.exemplo.io/jobs/view/9000000001": false,
		"http://www.linkedin.com/jobs/view/9000000001":         false,
	}
	for final, ok := range casos {
		p := novaPagina()
		p.final = final
		s := novoCenario(t, p)
		d := s.enviar(s.aprovada("linkedin", linkLI))
		if ok && d.Estado != dominio.CandEnviada || !ok && (d.Motivo != "site-externo" || len(p.preenchidos) > 0 || p.enviou > 0) {
			t.Errorf("parou em %s: %+v (preenchidos %v)", final, d, p.preenchidos)
		}
		s.logSemConteudo()
	}
}

// M1 e bloqueios: o que só a pessoa responde bloqueia antes de digitar qualquer coisa.
func TestBloqueiaSemPreencher(t *testing.T) {
	casos := []struct {
		nome   string
		ajuste func(p *paginaFalsa)
		motivo string
	}{
		{"captcha ao abrir", func(p *paginaFalsa) { p.bloqueios = []string{"captcha"} }, "captcha"},
		{"motivo desconhecido", func(p *paginaFalsa) { p.bloqueios = []string{"texto livre da página"} }, "outro"},
		{"pergunta obrigatória nova", func(p *paginaFalsa) {
			p.campos = append(p.campos, Campo{ID: "q", Tipo: CampoPergunta, Rotulo: "Por que você quer trabalhar aqui?", Obrigatorio: true})
		}, "pergunta-nova"},
		{"campo obrigatório desconhecido", func(p *paginaFalsa) {
			p.campos = append(p.campos, Campo{ID: "q", Tipo: CampoOutro, Rotulo: "Link do portfólio", Obrigatorio: true})
		}, "pergunta-nova"},
		{"cpf mesmo opcional", func(p *paginaFalsa) { p.campos = append(p.campos, Campo{ID: "d", Tipo: "cpf"}) }, "cpf"},
		{"senha", func(p *paginaFalsa) { p.campos = append(p.campos, Campo{ID: "s", Tipo: "senha", Obrigatorio: true}) }, "senha"},
		{"dado sensível", func(p *paginaFalsa) { p.campos = append(p.campos, Campo{ID: "r", Tipo: "dado-sensivel"}) }, "dado-sensivel"},
		{"termos", func(p *paginaFalsa) { p.campos = append(p.campos, Campo{ID: "t", Tipo: "termos", Obrigatorio: true}) }, "termos"},
		{"mensagem obrigatória fora do pacote", func(p *paginaFalsa) {
			p.campos = append(p.campos, Campo{ID: "carta", Tipo: CampoMensagem, Obrigatorio: true})
		}, "pergunta-nova"},
	}
	for _, k := range casos {
		p := novaPagina()
		k.ajuste(p)
		s := novoCenario(t, p)
		id := s.aprovada("linkedin", linkLI)
		d := s.enviar(id)
		c := s.cand(id)
		bloqueio := dominio.ObjetoDe(c["bloqueio"])
		if d.Estado != dominio.CandBloqueada || d.Motivo != k.motivo || bloqueio["motivo"] != k.motivo || bloqueio["pendencia"] != dominio.PendenciaDoMotivo[k.motivo] {
			t.Errorf("%s: %+v / %v", k.nome, d, bloqueio)
		}
		if len(p.preenchidos)+len(p.anexos) > 0 || p.enviou > 0 || s.etapa(id) != "bloqueada" {
			t.Errorf("%s: preencheu %v, anexou %d, enviou %d, vaga %v", k.nome, p.preenchidos, len(p.anexos), p.enviou, s.etapa(id))
		}
		s.logSemConteudo()
	}
	// Bloqueio que aparece depois de preencher (ex.: login): para antes do clique.
	p := novaPagina()
	p.bloqueios = []string{"", "login"}
	s := novoCenario(t, p)
	if d := s.enviar(s.aprovada("linkedin", linkLI)); d.Motivo != "login" || p.enviou != 0 {
		t.Fatalf("login depois de preencher = %+v, enviou %d", d, p.enviou)
	}
}

// M3: clicou e não confirmou fica incerta, sem nova tentativa; erro antes do clique é falha técnica.
func TestIncertaEFalha(t *testing.T) {
	semConfirmar := novaPagina()
	semConfirmar.confirma = false
	erroNoClique := novaPagina()
	erroNoClique.errEnviar = errors.New("clique falhou em https://www.linkedin.com/jobs?x=" + marca)
	s := novoCenario(t, semConfirmar, erroNoClique)
	for _, id := range []string{s.aprovada("linkedin", linkLI), s.aprovada("linkedin", "https://www.linkedin.com/jobs/view/9000000002")} {
		if d := s.enviar(id); d.Estado != dominio.CandIncerta || s.cand(id)["estado"] != "incerta" {
			t.Fatalf("%s = %+v", id, d)
		}
		if _, err := s.x.Enviar(context.Background(), id); err == nil {
			t.Fatal("incerta enviada de novo")
		}
	}
	if semConfirmar.enviou != 1 || erroNoClique.enviou != 1 || s.nav.abertas != 2 {
		t.Fatalf("cliques %d/%d, páginas %d", semConfirmar.enviou, erroNoClique.enviou, s.nav.abertas)
	}

	abrir := novaPagina()
	abrir.errAbrir = errors.New("rede")
	preencher := novaPagina()
	preencher.errPreencher = errors.New("campo sumiu")
	s2 := novoCenario(t, abrir, preencher)
	for _, id := range []string{s2.aprovada("linkedin", linkLI), s2.aprovada("linkedin", "https://www.linkedin.com/jobs/view/9000000002")} {
		if d := s2.enviar(id); d.Estado != dominio.CandFalhou || d.Motivo != "erro-tecnico" {
			t.Fatalf("%s = %+v", id, d)
		}
	}
	if preencher.enviou != 0 {
		t.Fatal("clicou depois de erro ao preencher")
	}
	s.logSemConteudo()
	s2.logSemConteudo()
}

// Cancelar (fechar o app) antes do clique: falha técnica gravada, sem clicar.
func TestCanceladoAntesDoClique(t *testing.T) {
	p := novaPagina()
	ctx, cancelar := context.WithCancel(context.Background())
	p.aoPreencher = cancelar
	s := novoCenario(t, p)
	id := s.aprovada("linkedin", linkLI)
	d, err := s.x.Enviar(ctx, id)
	if err != nil || d.Estado != dominio.CandFalhou || p.enviou != 0 || s.cand(id)["estado"] != "falhou" {
		t.Fatalf("cancelado = %+v %v, enviou %d", d, err, p.enviou)
	}
}

// M2 no instante do envio: limite atingido ou aprovação vencida não mudam nada.
func TestRecusaNaEntrada(t *testing.T) {
	s := novoCenario(t, novaPagina(), novaPagina())
	if err := s.arm.GravarLimiteCandidatura(context.Background(), "linkedin", 1); err != nil {
		t.Fatal(err)
	}
	a := s.aprovada("linkedin", linkLI)
	b := s.aprovada("linkedin", "https://www.linkedin.com/jobs/view/9000000002")
	s.enviar(a)
	antes := fmt.Sprint(s.cand(b))
	var el *dominio.ErroLimite
	if _, err := s.x.Enviar(context.Background(), b); !errors.As(err, &el) || fmt.Sprint(s.cand(b)) != antes || s.nav.abertas != 1 {
		t.Fatalf("limite = %v, abertas %d", err, s.nav.abertas)
	}
	s.agora = time.Date(2026, 11, 10, 0, 0, 1, 0, sp) // venceu
	s.arm.GravarLimiteCandidatura(context.Background(), "linkedin", 40)
	var ec *dominio.ErroConflito
	if _, err := s.x.Enviar(context.Background(), b); !errors.As(err, &ec) || fmt.Sprint(s.cand(b)) != antes {
		t.Fatalf("vencida = %v", err)
	}
	if _, err := s.x.Enviar(context.Background(), "cand-nao-existe"); !dominio.EhNaoEncontrado(err) {
		t.Fatalf("inexistente = %v", err)
	}
}

// Ao abrir o app: enviando vira incerta e aprovação vencida volta a preparada.
func TestIniciar(t *testing.T) {
	s := novoCenario(t)
	a := s.aprovada("linkedin", linkLI)
	b := s.aprovada("linkedin", "https://www.linkedin.com/jobs/view/9000000002")
	s.gravar(func(e dominio.Estado) (dominio.Estado, error) {
		v, _ := dominio.NumeroDe(s.cand(a)["versao"])
		e, _, err := dominio.IniciarEnvio(e, a, int(v), s.agora, nil)
		return e, err
	})
	s.agora = time.Date(2026, 11, 10, 8, 0, 0, 0, sp)
	incertas, vencidas, err := s.x.Iniciar(context.Background())
	if err != nil || incertas != 1 || vencidas != 1 || s.cand(a)["estado"] != "incerta" || s.cand(b)["estado"] != "preparada" {
		t.Fatalf("Iniciar = %d %d %v; %v %v", incertas, vencidas, err, s.cand(a)["estado"], s.cand(b)["estado"])
	}
}

// A fila: da aprovação mais antiga para a mais nova, com intervalo entre candidaturas, pulando a plataforma no limite.
func TestEnviarAprovadas(t *testing.T) {
	s := novoCenario(t, novaPagina(), novaPagina())
	s.arm.GravarLimiteCandidatura(context.Background(), "linkedin", 1)
	li1 := s.aprovada("linkedin", linkLI)
	ou := s.aprovada("Site da empresa", "https://carreiras.exemplo.com/1")
	li2 := s.aprovada("linkedin", "https://www.linkedin.com/jobs/view/9000000002")
	gp := s.aprovada("gupy", "https://empresa-teste.gupy.io/jobs/1")
	ds, err := s.x.EnviarAprovadas(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	ordem := []string{}
	for _, d := range ds {
		ordem = append(ordem, d.ID+":"+d.Estado)
	}
	quer := []string{li1 + ":enviada", ou + ":bloqueada", gp + ":enviada"}
	if fmt.Sprint(ordem) != fmt.Sprint(quer) || s.cand(li2)["estado"] != "aprovada" {
		t.Fatalf("ordem = %v; queria %v (li2 %v)", ordem, quer, s.cand(li2)["estado"])
	}
	entre := 0
	for _, d := range s.esperas {
		if d >= time.Minute {
			entre++
		}
	}
	if entre != 1 { // só entre as duas que abriram página (LinkedIn → Gupy)
		t.Fatalf("esperas entre candidaturas = %d (%v)", entre, s.esperas)
	}
	s.logSemConteudo()
}

func TestValorDoCampo(t *testing.T) {
	conteudo := dominio.Objeto{
		"contato":   dominio.Objeto{"nome": "Pessoa Teste"},
		"pretensao": dominio.Objeto{"valor": 12000.5},
		"respostas": []any{dominio.Objeto{"pergunta": "Inglês?", "resposta": "Avançado"}},
		"mensagem":  "Olá",
	}
	casos := []struct {
		c             Campo
		valor, motivo string
	}{
		{Campo{Tipo: CampoNome, Obrigatorio: true}, "Pessoa Teste", ""},
		{Campo{Tipo: CampoEmail, Obrigatorio: true}, "", "pergunta-nova"},
		{Campo{Tipo: CampoEmail}, "", ""},
		{Campo{Tipo: CampoPretensao}, "12001", ""}, // sem ponto decimal
		{Campo{Tipo: CampoMensagem}, "Olá", ""},
		{Campo{Tipo: CampoPergunta, Rotulo: " INGLÊS?  *"}, "Avançado", ""},
		{Campo{Tipo: CampoPergunta, Rotulo: "Inglês fluente?", Obrigatorio: true}, "", "pergunta-nova"},
		{Campo{Tipo: "rg"}, "", "rg"},
		{Campo{Tipo: "dado-bancario"}, "", "dado-bancario"},
		{Campo{Tipo: "documento"}, "", "documento"},
		{Campo{Tipo: "outro", Obrigatorio: true}, "", "pergunta-nova"},
		{Campo{Tipo: "qualquer"}, "", ""},
	}
	for _, k := range casos {
		if v, m := valorDoCampo(conteudo, k.c); v != k.valor || m != k.motivo {
			t.Errorf("%+v = %q %q; queria %q %q", k.c, v, m, k.valor, k.motivo)
		}
	}
}

// jc-a2y.51: pausado, nada é enviado; a conferência é na mesma transação que entraria em enviando.
func TestPausadoNaoEnvia(t *testing.T) {
	s := novoCenario(t, novaPagina(), novaPagina())
	a := s.aprovada("linkedin", linkLI)
	ou := s.aprovada("Site da empresa", "https://carreiras.exemplo.com/1")
	if err := s.arm.GravarEnvioPausado(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	ds, err := s.x.EnviarAprovadas(context.Background())
	if err != nil || len(ds) != 0 || s.nav.abertas != 0 || s.cand(a)["estado"] != "aprovada" || s.cand(ou)["estado"] != "aprovada" {
		t.Fatalf("pausado: desfechos %v (%v), páginas %d, estados %v %v", ds, err, s.nav.abertas, s.cand(a)["estado"], s.cand(ou)["estado"])
	}
	// Mesmo chamando Enviar direto (sem a fila), a transação recusa e nada muda.
	antes := fmt.Sprint(s.cand(a))
	if _, err := s.x.Enviar(context.Background(), a); !errors.Is(err, armazem.ErrEnvioPausado) || fmt.Sprint(s.cand(a)) != antes || s.nav.abertas != 0 {
		t.Fatalf("Enviar pausado = %v", err)
	}
	if _, err := s.x.Enviar(context.Background(), ou); !errors.Is(err, armazem.ErrEnvioPausado) || s.cand(ou)["estado"] != "aprovada" {
		t.Fatalf("outra pausada = %v", err)
	}
	// Retomado, envia.
	s.arm.GravarEnvioPausado(context.Background(), false)
	if ds, err := s.x.EnviarAprovadas(context.Background()); err != nil || len(ds) != 2 {
		t.Fatalf("retomado = %v %v", ds, err)
	}
}

// jc-a2y.6.7: com a janela de login da pessoa aberta, o ciclo espera: nenhuma página abre e nada muda de estado
// (nem a de outra plataforma, que iria para site-externo sem página).
func TestLoginAbertoNaoEnvia(t *testing.T) {
	s := novoCenario(t, novaPagina())
	a := s.aprovada("linkedin", linkLI)
	ou := s.aprovada("Site da empresa", "https://carreiras.exemplo.com/1")
	s.nav.ocupado = true
	ds, err := s.x.EnviarAprovadas(context.Background())
	if err != nil || len(ds) != 0 || s.nav.abertas != 0 || s.cand(a)["estado"] != "aprovada" || s.cand(ou)["estado"] != "aprovada" {
		t.Fatalf("com login aberto: desfechos %v (%v), páginas %d, estados %v %v", ds, err, s.nav.abertas, s.cand(a)["estado"], s.cand(ou)["estado"])
	}
	s.nav.ocupado = false // a pessoa fechou a janela
	if ds, err := s.x.EnviarAprovadas(context.Background()); err != nil || len(ds) != 2 {
		t.Fatalf("depois do login = %v %v", ds, err)
	}
}

// Pausar no meio da fila: a que está em envio termina; a próxima não começa.
func TestPausarNoMeioDaFila(t *testing.T) {
	p1 := novaPagina()
	s := novoCenario(t, p1, novaPagina())
	a := s.aprovada("linkedin", linkLI)
	b := s.aprovada("linkedin", "https://www.linkedin.com/jobs/view/9000000002")
	p1.aoPreencher = func() { s.arm.GravarEnvioPausado(context.Background(), true) } // a pessoa pausa durante o 1º
	ds, err := s.x.EnviarAprovadas(context.Background())
	if err != nil || len(ds) != 1 || s.cand(a)["estado"] != "enviada" || s.cand(b)["estado"] != "aprovada" || s.nav.abertas != 1 {
		t.Fatalf("desfechos %v (%v); a=%v b=%v páginas %d", ds, err, s.cand(a)["estado"], s.cand(b)["estado"], s.nav.abertas)
	}
}

// Pausar entre "pegou a aprovada" e "entrou em enviando" (o pedido do CyberSec no #59): a transação do IniciarEnvio
// lê o pausado e recusa; a candidatura fica aprovada e nenhum envio acontece.
func TestPausarEntrePegarEEntrarEmEnviando(t *testing.T) {
	s := novoCenario(t, novaPagina())
	id := s.aprovada("linkedin", linkLI)
	s.x.antesDeIniciar = func() { s.arm.GravarEnvioPausado(context.Background(), true) }
	ds, err := s.x.EnviarAprovadas(context.Background())
	if err != nil || len(ds) != 0 || s.cand(id)["estado"] != "aprovada" || s.nav.abertas != 0 || dominio.ListaDe(s.cand(id)["tentativas"]) != nil {
		t.Fatalf("desfechos %v (%v), estado %v, páginas %d, tentativas %v", ds, err, s.cand(id)["estado"], s.nav.abertas, s.cand(id)["tentativas"])
	}
}

// Ao abrir o app, o envio volta a ficar pausado, mesmo que a pessoa tenha retomado antes de fechar.
func TestIniciarPausa(t *testing.T) {
	s := novoCenario(t) // o cenário começa com o envio liberado
	if _, _, err := s.x.Iniciar(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !s.arm.EnvioPausado(context.Background()) {
		t.Fatal("Iniciar não pausou o envio")
	}
}

// quebrarBanco põe uma linha estragada na tabela itens: Carregar e Gravar passam a falhar; a meta continua gravável.
func (s *cenario) quebrarBanco() {
	s.t.Helper()
	if err := s.arm.Transacao(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(context.Background(), `INSERT INTO itens (colecao, ordem, dados) VALUES ('vagas', 999, '{estragado')`)
		return err
	}); err != nil {
		s.t.Fatal(err)
	}
}

// Revisão do #61: com a gravação das incertas falhando ao abrir, o envio já fica pausado (vem primeiro).
func TestIniciarPausaMesmoComFalha(t *testing.T) {
	s := novoCenario(t) // começa liberado
	s.quebrarBanco()
	if _, _, err := s.x.Iniciar(context.Background()); err == nil {
		t.Fatal("Iniciar deveria falhar com o banco estragado")
	}
	if !s.arm.EnvioPausado(context.Background()) {
		t.Fatal("a falha no Iniciar deixou o envio liberado")
	}
}

// Rodar com o Iniciar falhando não entra no laço: devolve o erro na hora e nenhuma página abre.
func TestRodarNaoEntraNoLacoSeIniciarFalha(t *testing.T) {
	s := novoCenario(t, novaPagina())
	s.aprovada("linkedin", linkLI)
	s.quebrarBanco()
	ctx, cancelar := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelar()
	var ciclo sync.Mutex
	inicio := time.Now()
	err := s.x.Rodar(ctx, time.Millisecond, &ciclo)
	if err == nil || time.Since(inicio) > time.Second || s.nav.abertas != 0 {
		t.Fatalf("Rodar = %v em %v, páginas %d", err, time.Since(inicio), s.nav.abertas)
	}
	if !ciclo.TryLock() {
		t.Fatal("Rodar deixou o ciclo travado")
	}
}

// Rodar normal: abre pausado e não envia; ao cancelar, sai.
func TestRodarAbrePausado(t *testing.T) {
	s := novoCenario(t, novaPagina())
	id := s.aprovada("linkedin", linkLI)
	ctx, cancelar := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancelar()
	var ciclo sync.Mutex
	if err := s.x.Rodar(ctx, 10*time.Millisecond, &ciclo); err != nil {
		t.Fatal(err)
	}
	if s.cand(id)["estado"] != "aprovada" || s.nav.abertas != 0 || !s.arm.EnvioPausado(context.Background()) {
		t.Fatalf("Rodar enviou sem retomar: %v, páginas %d", s.cand(id)["estado"], s.nav.abertas)
	}
}

// Reabrir o app (decisão do PO no #61): a pessoa retomou e fechou; ao abrir de novo (banco reaberto do disco), o
// estado gravado ainda é "retomado", e o Iniciar o volta a pausado antes de qualquer envio.
func TestReabrirOAppVoltaPausado(t *testing.T) {
	ctx := context.Background()
	s := novoCenario(t, novaPagina()) // o cenário começa retomado
	id := s.aprovada("linkedin", linkLI)
	s.arm.Fechar() // fecha o app
	arm, err := armazem.Abrir(ctx, filepath.Join(filepath.Dir(s.pastaCV), "central.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer arm.Fechar()
	if arm.EnvioPausado(ctx) {
		t.Fatal("o estado gravado devia continuar retomado antes do Iniciar (o teste não estaria provando nada)")
	}
	x := Novo(arm, Opcoes{Navegador: s.nav, PastaCurriculos: s.pastaCV, Relogio: func() time.Time { return s.agora },
		Hoje: func() string { return s.agora.Format("2006-01-02") }, Esperar: func(ctx context.Context, d time.Duration) error { return ctx.Err() },
		Log: func(string, ...any) {}})
	if _, _, err := x.Iniciar(ctx); err != nil {
		t.Fatal(err)
	}
	if !arm.EnvioPausado(ctx) {
		t.Fatal("reabrir o app não voltou a pausar")
	}
	if ds, err := x.EnviarAprovadas(ctx); err != nil || len(ds) != 0 || s.nav.abertas != 0 {
		t.Fatalf("enviou depois de reabrir: %v %v, páginas %d", ds, err, s.nav.abertas)
	}
	e, _ := arm.Carregar(ctx)
	for _, o := range e.Candidaturas {
		if o["id"] == id && o["estado"] != "aprovada" {
			t.Fatalf("estado = %v", o["estado"])
		}
	}
}
