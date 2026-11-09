package executor

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// jc-a2y.6.11 (requisitos do CyberSec): envio que se vira sozinho com as falhas. Navegador e páginas falsos, dados
// fictícios.

const linkGupy = "https://empresa-teste.gupy.io/jobs/9000001"

func (s *cenario) plataforma(nome string) dominio.EstadoPlataforma {
	return s.arm.Plataformas(context.Background()).DaPlataforma(nome, s.agora)
}

func (s *cenario) temLog(trecho string) bool {
	return strings.Contains(strings.Join(s.logs, "\n"), trecho)
}

// Item 1: a marca "interagiu" está gravada no banco antes do primeiro preencher e anexar; a "clicou-enviar", antes
// do clique final.
func TestMarcasGravadasAntesDoPasso(t *testing.T) {
	p := novaPagina()
	s := novoCenario(t, p)
	id := s.aprovada("linkedin", linkLI)
	var noPreencher, noAnexar, noEnviar dominio.Objeto
	p.aoPreencher = func() {
		if noPreencher == nil {
			noPreencher = s.cand(id)
		}
	}
	p.aoAnexar = func() { noAnexar = s.cand(id) }
	p.aoEnviar = func() { noEnviar = s.cand(id) }
	if d := s.enviar(id); d.Estado != dominio.CandEnviada {
		t.Fatalf("desfecho = %+v", d)
	}
	if dominio.Texto(noPreencher["interagiuEm"]) == "" || dominio.Texto(noAnexar["interagiuEm"]) == "" {
		t.Fatalf("preencheu/anexou sem a marca gravada: %v / %v", noPreencher["interagiuEm"], noAnexar["interagiuEm"])
	}
	if noPreencher["clicouEnviarEm"] != nil {
		t.Fatal("clicou-enviar gravada antes da hora")
	}
	if dominio.Texto(noEnviar["clicouEnviarEm"]) == "" {
		t.Fatal("clicou em enviar sem a marca gravada")
	}
}

// Item 1: depois da marca, nunca volta (falhou, bloqueada ou incerta, como sempre).
func TestDepoisDeInteragirNuncaVolta(t *testing.T) {
	preencher := novaPagina()
	preencher.errPreencher = errors.New("campo sumiu")
	login := novaPagina()
	login.bloqueios = []string{"", "login"} // a sessão cai depois de preencher
	clique := novaPagina()
	clique.errEnviar = errors.New("clique falhou")
	s := novoCenario(t, preencher, login, clique)
	for _, k := range []struct {
		pagina *paginaFalsa
		estado string
	}{{preencher, dominio.CandFalhou}, {login, dominio.CandBloqueada}, {clique, dominio.CandIncerta}} {
		id := s.aprovada("linkedin", linkLI)
		d := s.enviar(id)
		if d.Estado != k.estado || s.cand(id)["estado"] != k.estado || len(dominio.ListaDe(s.cand(id)["retentativas"])) != 0 {
			t.Fatalf("depois de interagir = %+v; queria %s, sem volta", d, k.estado)
		}
		// A plataforma pode ter mudado (login): retoma para o próximo caso.
		s.arm.MudarPlataformas(context.Background(), func(p dominio.Plataformas, _ bool) (dominio.Plataformas, error) {
			return dominio.Plataformas{}, nil
		})
	}
}

// Item 1 (segunda trava): mesmo que o executor erre e ache que não interagiu, o domínio confere a marca gravada e
// recusa a volta. Teste de mutação: sem a conferência da marca no domínio, este teste reprova.
func TestVoltaComMarcaGravadaReprova(t *testing.T) {
	s := novoCenario(t)
	id := s.aprovada("linkedin", linkLI)
	var versao int
	v, _ := dominio.NumeroDe(s.cand(id)["versao"])
	s.gravar(func(e dominio.Estado) (dominio.Estado, error) {
		e, o, err := dominio.IniciarEnvio(e, id, int(v), s.agora, nil)
		if err != nil {
			return e, err
		}
		e, o, err = dominio.MarcarTentativa(e, id, dominio.MarcaInteragiu, s.agora)
		n, _ := dominio.NumeroDe(o["versao"])
		versao = int(n)
		return e, err
	})
	t1 := &tentativa{x: s.x, id: id, versao: versao} // interagiu=false: o executor "esqueceu"
	d, err := s.x.concluir(context.Background(), "linkedin", t1, falhou("erro-tecnico", errors.New("x")))
	if err != nil {
		t.Fatal(err)
	}
	if d.Estado != dominio.CandFalhou || s.cand(id)["estado"] != dominio.CandFalhou {
		t.Fatalf("voltou com a marca gravada: %+v", d)
	}
}

// Itens 2 e 3: erro ao abrir volta para aprovada com a mesma aprovação, fora do limite; a próxima tentativa espera a
// hora gravada, também com o app reaberto; e então envia sem aprovar de novo.
func TestVoltaEsperaAHoraEEnviaDepois(t *testing.T) {
	ruim := novaPagina()
	ruim.errAbrir = errors.New("rede")
	boa := novaPagina()
	s := novoCenario(t, ruim, boa)
	if err := s.arm.GravarLimiteCandidatura(context.Background(), "linkedin", 1); err != nil {
		t.Fatal(err)
	}
	id := s.aprovada("linkedin", linkLI)
	antes := s.cand(id)
	d := s.enviar(id)
	c := s.cand(id)
	if d.Estado != dominio.CandAprovada || c["estado"] != dominio.CandAprovada || len(dominio.ListaDe(c["tentativas"])) != 0 {
		t.Fatalf("erro ao abrir = %+v, %v", d, c)
	}
	ap, apAntes := dominio.ObjetoDe(c["aprovacao"]), dominio.ObjetoDe(antes["aprovacao"])
	if ap["impressao"] != apAntes["impressao"] || ap["validaAte"] != apAntes["validaAte"] || ap["usadaEm"] != nil {
		t.Fatalf("a volta mudou a aprovação: %v -> %v", apAntes, ap)
	}
	if !s.temLog("devolvida (erro-tecnico, 1/3)") {
		t.Errorf("logs = %v", s.logs)
	}

	// App reaberto no meio da espera: começa pausado, a hora marcada continua gravada.
	s.agora = s.agora.Add(2 * time.Minute)
	if _, _, err := s.x.Iniciar(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.cand(id)["estado"] != dominio.CandAprovada || dominio.Texto(s.cand(id)["proximaTentativaEm"]) == "" {
		t.Fatalf("reabrir mudou a candidatura: %v", s.cand(id))
	}
	s.arm.GravarEnvioPausado(context.Background(), false) // a pessoa retoma
	if ds, _ := s.x.EnviarAprovadas(context.Background()); len(ds) != 0 || s.nav.abertas != 1 {
		t.Fatalf("pegou antes da hora: %v, páginas %d", ds, s.nav.abertas)
	}
	s.agora = s.agora.Add(4 * time.Minute)
	ds, err := s.x.EnviarAprovadas(context.Background())
	if err != nil || len(ds) != 1 || ds[0].Estado != dominio.CandEnviada {
		t.Fatalf("depois da hora = %v %v", ds, err)
	}
	// Limite 1: a volta não contou.
	if n := len(dominio.ListaDe(s.cand(id)["tentativas"])); n != 1 {
		t.Errorf("tentativas = %d", n)
	}
	s.logSemConteudo()
}

// Item 4: 2 falhas técnicas seguidas pausam só a plataforma (gravado); as outras seguem; o assistente não retoma.
func TestFalhasSeguidasPausamSoAPlataforma(t *testing.T) {
	r1, r2 := novaPagina(), novaPagina()
	r1.errAbrir, r2.errAbrir = errors.New("rede"), errors.New("rede")
	gupy := novaPagina()
	s := novoCenario(t, r1, r2, gupy)
	li1, li2, li3 := s.aprovada("linkedin", linkLI), s.aprovada("linkedin", "https://www.linkedin.com/jobs/view/9000000002"), s.aprovada("linkedin", "https://www.linkedin.com/jobs/view/9000000003")
	gp := s.aprovada("gupy", linkGupy)
	ds, err := s.x.EnviarAprovadas(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st := s.plataforma("linkedin"); st.Estado != dominio.PlataformaPausada || st.Motivo != "falhas-seguidas" {
		t.Fatalf("linkedin = %+v", st)
	}
	if s.cand(li3)["estado"] != dominio.CandAprovada || s.cand(gp)["estado"] != dominio.CandEnviada || len(ds) != 3 {
		t.Fatalf("li3 %v, gupy %v, desfechos %v", s.cand(li3)["estado"], s.cand(gp)["estado"], ds)
	}
	_ = li1
	_ = li2
	if !s.temLog("envio: plataforma linkedin pausada (falhas-seguidas)") {
		t.Errorf("logs = %v", s.logs)
	}
	// Nem com a hora passada a fila pega a plataforma pausada.
	s.agora = s.agora.Add(3 * time.Hour)
	if ds, _ := s.x.EnviarAprovadas(context.Background()); len(ds) != 0 {
		t.Fatalf("enviou com a plataforma pausada: %v", ds)
	}
	// A conferência que vale é a da transação (a da fila é só leitura).
	if _, err := s.x.Enviar(context.Background(), li3); !errors.Is(err, armazem.ErrPlataformaParada) || s.cand(li3)["estado"] != dominio.CandAprovada {
		t.Fatalf("Enviar com a plataforma pausada = %v", err)
	}
	// Só a pessoa retoma (a API confere o autor; aqui, o domínio): ativa de novo.
	if _, err := s.arm.MudarPlataformas(context.Background(), func(p dominio.Plataformas, _ bool) (dominio.Plataformas, error) {
		return p.RetomarPlataforma("linkedin", s.agora)
	}); err != nil || s.plataforma("linkedin").Estado != dominio.PlataformaAtiva {
		t.Fatalf("retomar = %v, %+v", err, s.plataforma("linkedin"))
	}
	s.logSemConteudo()
}

// Itens 4 e 5: sessão caída antes de interagir devolve e deixa a plataforma em precisa-entrar. Só volta sozinha
// depois do login pela janela "Entrar nas plataformas" e com o envio geral liberado.
func TestPrecisaEntrar(t *testing.T) {
	p := novaPagina()
	p.bloqueios = []string{"login"}
	s := novoCenario(t, p)
	id := s.aprovada("linkedin", linkLI)
	if d := s.enviar(id); d.Estado != dominio.CandAprovada || d.Motivo != "login" {
		t.Fatalf("login antes de interagir = %+v", d)
	}
	if st := s.plataforma("linkedin"); st.Estado != dominio.PlataformaPrecisaEntrar || st.Motivo != "login" {
		t.Fatalf("linkedin = %+v", st)
	}
	// Envio geral pausado: fechar o login não retoma.
	s.arm.GravarEnvioPausado(context.Background(), true)
	s.x.LiberarDepoisDoLogin(context.Background())
	if s.plataforma("linkedin").Estado != dominio.PlataformaPrecisaEntrar {
		t.Fatal("retomou com o envio geral pausado")
	}
	s.arm.GravarEnvioPausado(context.Background(), false)
	s.x.LiberarDepoisDoLogin(context.Background())
	if s.plataforma("linkedin").Estado != dominio.PlataformaAtiva || !s.temLog("envio: plataforma linkedin ativa (login)") {
		t.Fatalf("depois do login = %+v; logs %v", s.plataforma("linkedin"), s.logs)
	}
}

// Item 6: captcha e aviso de conta bloqueiam e param a plataforma até amanhã, sem nova tentativa; retomar dá 409.
func TestCaptchaEAvisoParamAteAmanha(t *testing.T) {
	for _, motivo := range []string{"captcha", "aviso-conta"} {
		p := novaPagina()
		p.bloqueios = []string{motivo}
		s := novoCenario(t, p)
		id := s.aprovada("linkedin", linkLI)
		outra := s.aprovada("linkedin", "https://www.linkedin.com/jobs/view/9000000002")
		ds, err := s.x.EnviarAprovadas(context.Background())
		if err != nil || len(ds) != 1 || ds[0].Estado != dominio.CandBloqueada || ds[0].Motivo != motivo {
			t.Fatalf("%s: %v %v", motivo, ds, err)
		}
		st := s.plataforma("linkedin")
		if st.Estado != dominio.PlataformaParadaHoje || st.Motivo != motivo || st.Ate != "2026-11-10T00:00:00-03:00" {
			t.Fatalf("%s: linkedin = %+v", motivo, st)
		}
		if s.cand(outra)["estado"] != dominio.CandAprovada || s.nav.abertas != 1 {
			t.Fatalf("%s: tentou outra no mesmo dia", motivo)
		}
		if _, err := s.arm.MudarPlataformas(context.Background(), func(p dominio.Plataformas, _ bool) (dominio.Plataformas, error) {
			return p.RetomarPlataforma("linkedin", s.agora)
		}); err == nil {
			t.Fatalf("%s: retomou no mesmo dia", motivo)
		}
		if s.cand(id)["estado"] != dominio.CandBloqueada {
			t.Fatalf("%s: a bloqueada voltou", motivo)
		}
		// 00:00 do dia seguinte: ativa de novo.
		s.agora = time.Date(2026, 11, 10, 0, 0, 0, 0, sp)
		if s.plataforma("linkedin").Estado != dominio.PlataformaAtiva {
			t.Fatalf("%s: não renovou à meia-noite", motivo)
		}
	}
}

// Item 9: 3 voltas e, na 4ª falha antes de interagir, falhou (com a pessoa retomando a plataforma entre elas).
func TestTresVoltasDepoisFalhou(t *testing.T) {
	var paginas []*paginaFalsa
	for i := 0; i < 4; i++ {
		p := novaPagina()
		p.errAbrir = errors.New("rede")
		paginas = append(paginas, p)
	}
	s := novoCenario(t, paginas...)
	id := s.aprovada("linkedin", linkLI)
	for i, quer := range []string{dominio.CandAprovada, dominio.CandAprovada, dominio.CandAprovada, dominio.CandFalhou} {
		d := s.enviar(id)
		if d.Estado != quer || d.Motivo != "erro-tecnico" {
			t.Fatalf("tentativa %d = %+v; queria %s", i+1, d, quer)
		}
		s.agora = s.agora.Add(3 * time.Hour)
		s.arm.MudarPlataformas(context.Background(), func(p dominio.Plataformas, _ bool) (dominio.Plataformas, error) {
			return dominio.Plataformas{}, nil
		})
	}
	if n := len(dominio.ListaDe(s.cand(id)["tentativas"])); n != 1 {
		t.Errorf("tentativas no limite = %d; queria 1 (só a última)", n)
	}
}

// Item 7 (trava do executor): incerta não é caso de volta, nem sem marca gravada (a do domínio é a segunda trava).
func TestIncertaNaoEhCasoDeVolta(t *testing.T) {
	s := novoCenario(t)
	id := s.aprovada("linkedin", linkLI)
	v, _ := dominio.NumeroDe(s.cand(id)["versao"])
	var versao int
	s.gravar(func(e dominio.Estado) (dominio.Estado, error) {
		e, o, err := dominio.IniciarEnvio(e, id, int(v), s.agora, nil)
		n, _ := dominio.NumeroDe(o["versao"])
		versao = int(n)
		return e, err
	})
	d, err := s.x.concluir(context.Background(), "linkedin", &tentativa{x: s.x, id: id, versao: versao}, passo{estado: dominio.CandIncerta})
	if err != nil || d.Estado != dominio.CandIncerta || s.cand(id)["estado"] != dominio.CandIncerta {
		t.Fatalf("incerta = %+v %v", d, err)
	}
}

// Item 7: incerta nunca volta sozinha, nem ao abrir o app.
func TestIncertaNuncaVolta(t *testing.T) {
	p := novaPagina()
	p.confirma = false
	s := novoCenario(t, p)
	id := s.aprovada("linkedin", linkLI)
	if d := s.enviar(id); d.Estado != dominio.CandIncerta {
		t.Fatalf("desfecho = %+v", d)
	}
	s.agora = s.agora.Add(3 * time.Hour)
	s.x.Iniciar(context.Background())
	s.arm.GravarEnvioPausado(context.Background(), false)
	if ds, _ := s.x.EnviarAprovadas(context.Background()); len(ds) != 0 || s.cand(id)["estado"] != dominio.CandIncerta {
		t.Fatalf("incerta voltou: %v", ds)
	}
}
