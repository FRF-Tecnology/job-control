//go:build contafalsa

package conta

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/licenca"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// relogio é o relógio da nuvem nos testes; o do PC é ele mais um atraso fixo.
type relogio struct {
	mu sync.Mutex
	t  time.Time
}

func (r *relogio) agora() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.t
}

func (r *relogio) avancar(d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.t = r.t.Add(d)
}

type ambiente struct {
	f     *Falsa
	pub   ed25519.PublicKey
	rel   *relogio
	local func() time.Time
	cofre segredos.Memoria
	pasta string
}

// novoAmbiente: atrasoLocal é quanto o relógio do PC está atrasado em relação ao da nuvem.
func novoAmbiente(t *testing.T, atrasoLocal time.Duration) *ambiente {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	rel := &relogio{t: time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)}
	return &ambiente{f: NovaFalsa(priv, rel.agora), pub: pub, rel: rel,
		local: func() time.Time { return rel.agora().Add(-atrasoLocal) },
		cofre: segredos.Memoria{}, pasta: t.TempDir()}
}

func (a *ambiente) conta(n Nuvem) *Conta {
	if n == nil {
		n = a.f
	}
	c := Nova(n, a.cofre, a.pasta, a.pub, a.local)
	c.dispositivo = func() (string, error) { return dispA, nil }
	return c
}

func (a *ambiente) parConfirmado(t *testing.T, email string) Par {
	t.Helper()
	ctx := context.Background()
	if err := a.f.Criar(ctx, email, "senha-forte-1", TermosPadrao); err != nil {
		t.Fatal(err)
	}
	p, err := a.f.Confirmar(ctx, email, CodigoFalso, "senha-forte-1", dispA, "")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func (a *ambiente) entrou(t *testing.T, n Nuvem) (*Conta, Par) {
	t.Helper()
	p := a.parConfirmado(t, "ana@exemplo.com")
	c := a.conta(n)
	if err := c.Entrou(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	return c, p
}

func querAgir(t *testing.T, c *Conta, pode bool, motivo string) {
	t.Helper()
	ok, m := c.PodeAgir()
	if ok != pode || m != motivo {
		t.Fatalf("PodeAgir = (%v, %q), queria (%v, %q)", ok, m, pode, motivo)
	}
	if s := c.Situacao(); s.Motivo != motivo {
		t.Fatalf("Situacao.Motivo = %q, queria %q", s.Motivo, motivo)
	}
}

func TestSemContaNaoAge(t *testing.T) {
	a := novoAmbiente(t, 0)
	c := a.conta(nil)
	querAgir(t, c, false, MotivoSemConta)
	if s := c.Situacao(); s.Entrada || s.Email != "" {
		t.Fatalf("sem conta: %+v", s)
	}
	if _, err := c.Acesso(context.Background()); !errors.Is(err, ErrSemConta) {
		t.Fatalf("Acesso sem conta: queria ErrSemConta, veio %v", err)
	}
	if err := c.AtualizarLicenca(context.Background()); !errors.Is(err, ErrSemConta) {
		t.Fatalf("AtualizarLicenca sem conta: queria ErrSemConta, veio %v", err)
	}
}

func TestEntrouPodeAgir(t *testing.T) {
	a := novoAmbiente(t, 0)
	c, p := a.entrou(t, nil)
	querAgir(t, c, true, "")
	s := c.Situacao()
	if !s.Entrada || s.Email != "ana@exemplo.com" || s.Estado != licenca.Teste || s.TesteFim == nil {
		t.Fatalf("situação: %+v", s)
	}
	if a.cofre[chaveRenovacao] != p.Renovacao {
		t.Fatal("a renovação devia estar no cofre")
	}
	if _, err := LerLicencaDoDisco(a.pasta, a.pub, a.local()); err != nil {
		t.Fatalf("licenca.json: %v", err)
	}
	if b, _ := json.Marshal(s); !strings.Contains(string(b), `"teste_fim"`) || !strings.Contains(string(b), `"entrada":true`) {
		t.Fatalf("JSON da situação: %s", b)
	}
}

func TestRenovacaoNoCofreNaoNoDisco(t *testing.T) {
	a := novoAmbiente(t, 0)
	c, p := a.entrou(t, nil)
	a.rel.avancar(15 * time.Minute) // acesso vencido: Acesso renova
	acesso, err := c.Acesso(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if acesso == p.Acesso {
		t.Fatal("devia ter renovado")
	}
	nova := a.cofre[chaveRenovacao]
	if nova == "" || nova == p.Renovacao {
		t.Fatalf("o cofre devia ter a renovação nova, tem %q", nova)
	}
	segredosVistos := []string{p.Acesso, p.Renovacao, acesso, nova}
	err = filepath.WalkDir(a.pasta, func(caminho string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(caminho)
		if err != nil {
			return err
		}
		for _, s := range segredosVistos {
			if strings.Contains(string(b), s) {
				t.Errorf("%s contém um token", caminho)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestTesteAcabaBloqueia(t *testing.T) {
	a := novoAmbiente(t, 0)
	c, _ := a.entrou(t, nil)
	a.rel.avancar(24*time.Hour + time.Second)
	if err := c.AtualizarLicenca(context.Background()); err != nil {
		t.Fatal(err)
	}
	querAgir(t, c, false, MotivoTesteAcabou)
	if s := c.Situacao(); s.Estado != licenca.Bloqueado || !s.Entrada {
		t.Fatalf("situação: %+v", s)
	}
}

func TestAssinaturaCanceladaMotivo(t *testing.T) {
	a := novoAmbiente(t, 0)
	c, _ := a.entrou(t, nil)
	a.f.Assinante("ana@exemplo.com")
	a.rel.avancar(25 * time.Hour) // passou o teste; continua assinante
	if err := c.AtualizarLicenca(context.Background()); err != nil {
		t.Fatal(err)
	}
	querAgir(t, c, true, "")
	a.f.Cancelar("ana@exemplo.com")
	if err := c.AtualizarLicenca(context.Background()); err != nil {
		t.Fatal(err)
	}
	querAgir(t, c, false, MotivoSemAssinatura)
	if s := c.Situacao(); s.PeriodoFim == nil {
		t.Fatalf("periodo_fim devia vir: %+v", s)
	}
}

func TestQuedaEVoltaDaInternet(t *testing.T) {
	a := novoAmbiente(t, 0)
	c, _ := a.entrou(t, nil)
	a.f.SemInternet(true)
	a.rel.avancar(10 * time.Minute)
	if err := c.AtualizarLicenca(context.Background()); !errors.Is(err, ErrSemInternet) {
		t.Fatalf("queria ErrSemInternet, veio %v", err)
	}
	a.rel.avancar(20 * time.Minute) // 30 min desde a emissão
	querAgir(t, c, true, "")
	a.rel.avancar(time.Second)
	querAgir(t, c, false, MotivoSemInternet)
	if s := c.Situacao(); !s.Entrada || s.Email != "ana@exemplo.com" {
		t.Fatalf("sem internet ainda está entrada: %+v", s)
	}
	a.f.SemInternet(false)
	if err := c.AtualizarLicenca(context.Background()); err != nil {
		t.Fatal(err)
	}
	querAgir(t, c, true, "")
}

func TestRelogioDesajustado(t *testing.T) {
	a := novoAmbiente(t, 3*time.Hour) // PC 3 h atrasado
	c, _ := a.entrou(t, nil)
	querAgir(t, c, true, "")
	a.f.SemInternet(true)
	a.rel.avancar(30 * time.Minute)
	querAgir(t, c, true, "")
	if _, err := LerLicencaDoDisco(a.pasta, a.pub, a.local()); err != nil {
		t.Fatalf("disco aos 30 min: %v", err)
	}
	a.rel.avancar(time.Second)
	querAgir(t, c, false, MotivoSemInternet)
	if _, err := LerLicencaDoDisco(a.pasta, a.pub, a.local()); !errors.Is(err, licenca.ErrVencida) {
		t.Fatalf("disco aos 30m1s: queria vencida, veio %v", err)
	}
	// uma Conta nova (app reaberto) lê o desvio guardado
	c2 := a.conta(nil)
	querAgir(t, c2, false, MotivoSemInternet)
}

func TestLicencaEditadaNoDiscoRecusada(t *testing.T) {
	a := novoAmbiente(t, 0)
	c, _ := a.entrou(t, nil)
	a.rel.avancar(24*time.Hour + time.Second)
	if err := c.AtualizarLicenca(context.Background()); err != nil {
		t.Fatal(err)
	}
	querAgir(t, c, false, MotivoTesteAcabou)

	caminho := filepath.Join(a.pasta, arquivoLicencaNome)
	b, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	var arq arquivoLicenca
	if err := json.Unmarshal(b, &arq); err != nil {
		t.Fatal(err)
	}
	dados, _ := base64.StdEncoding.DecodeString(arq.Documento.Dados)
	editado := strings.Replace(string(dados), `"bloqueado"`, `"assinante"`, 1)
	if editado == string(dados) {
		t.Fatal("o teste não editou nada")
	}
	arq.Documento.Dados = base64.StdEncoding.EncodeToString([]byte(editado))
	b, _ = json.Marshal(arq)
	if err := os.WriteFile(caminho, b, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LerLicencaDoDisco(a.pasta, a.pub, a.local()); !errors.Is(err, licenca.ErrAssinatura) {
		t.Fatalf("queria ErrAssinatura, veio %v", err)
	}
	a.f.SemInternet(true)
	c2 := a.conta(nil)
	if ok, _ := c2.PodeAgir(); ok {
		t.Fatal("licença editada não pode liberar")
	}
}

func TestSairApagaTudo(t *testing.T) {
	a := novoAmbiente(t, 0)
	c, p := a.entrou(t, nil)
	if err := c.Sair(context.Background()); err != nil {
		t.Fatal(err)
	}
	querAgir(t, c, false, MotivoSemConta)
	if len(a.cofre) != 0 {
		t.Fatalf("cofre devia estar vazio: %v", a.cofre)
	}
	if _, err := os.Stat(filepath.Join(a.pasta, arquivoLicencaNome)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("licenca.json devia sumir: %v", err)
	}
	if _, err := a.f.Renovar(context.Background(), p.Renovacao); err == nil {
		t.Fatal("a nuvem devia ter encerrado a sessão")
	}
}

func esperar(t *testing.T, cond func() bool) {
	t.Helper()
	limite := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(limite) {
			t.Fatal("tempo esgotado")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestIniciarRenovaEPedeLicenca(t *testing.T) {
	a := novoAmbiente(t, 0)
	p := a.parConfirmado(t, "ana@exemplo.com")
	a.cofre[chaveRenovacao] = p.Renovacao // app reaberto: só a renovação no cofre
	c := a.conta(nil)
	querAgir(t, c, false, MotivoSemInternet)
	if !c.Situacao().Entrada {
		t.Fatal("com renovação no cofre a pessoa está entrada")
	}
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	c.Iniciar(ctx)
	esperar(t, func() bool { ok, _ := c.PodeAgir(); return ok })
}

func TestIniciarComSessaoRevogadaVoltaParaEntrar(t *testing.T) {
	a := novoAmbiente(t, 0)
	c1, _ := a.entrou(t, nil)
	_ = c1
	if err := a.f.Sair(context.Background(), a.cofre[chaveRenovacao]); err != nil { // revogada na nuvem
		t.Fatal(err)
	}
	c := a.conta(nil)
	if !c.Situacao().Entrada {
		t.Fatal("antes de falar com a nuvem ainda parece entrada")
	}
	ctx, cancelar := context.WithCancel(context.Background())
	defer cancelar()
	c.Iniciar(ctx)
	esperar(t, func() bool { return !c.Situacao().Entrada })
	querAgir(t, c, false, MotivoSemConta)
	if len(a.cofre) != 0 {
		t.Fatalf("cofre devia estar vazio: %v", a.cofre)
	}
	if _, err := os.Stat(filepath.Join(a.pasta, arquivoLicencaNome)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("licenca.json devia sumir: %v", err)
	}
}

// contaRenovacoes conta quantos pedidos de renovação chegam à nuvem.
type contaRenovacoes struct {
	*Falsa
	n atomic.Int32
}

func (c *contaRenovacoes) Renovar(ctx context.Context, r string) (Par, error) {
	c.n.Add(1)
	time.Sleep(2 * time.Millisecond) // alarga a janela de corrida
	return c.Falsa.Renovar(ctx, r)
}

func TestRenovacaoNuncaConcorrente(t *testing.T) {
	a := novoAmbiente(t, 0)
	n := &contaRenovacoes{Falsa: a.f}
	c, p := a.entrou(t, n)
	a.rel.avancar(14*time.Minute + 30*time.Second) // falta menos de 1 min para o acesso vencer
	var wg sync.WaitGroup
	var mu sync.Mutex
	vistos := map[string]bool{}
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ac, err := c.Acesso(context.Background())
			if err != nil {
				t.Error(err)
				return
			}
			mu.Lock()
			vistos[ac] = true
			mu.Unlock()
		}()
	}
	wg.Wait()
	if got := n.n.Load(); got != 1 {
		t.Fatalf("Renovar chamado %d vezes, queria 1", got)
	}
	if len(vistos) != 1 || vistos[p.Acesso] {
		t.Fatalf("todos deviam receber o mesmo acesso novo: %v", vistos)
	}
	if ok, _ := c.PodeAgir(); !ok {
		t.Fatal("a sessão devia continuar")
	}
}

// nuvem5xx faz a renovação falhar com 503 da nuvem.
type nuvem5xx struct{ *Falsa }

func (nuvem5xx) Renovar(context.Context, string) (Par, error) {
	return Par{}, &ErroNuvem{Status: 503, Msg: "Fora do ar"}
}

func TestRenovar5xxMantemSessao(t *testing.T) {
	a := novoAmbiente(t, 0)
	c, p := a.entrou(t, nuvem5xx{a.f})
	a.rel.avancar(15 * time.Minute)
	_, err := c.Acesso(context.Background())
	var en *ErroNuvem
	if !errors.As(err, &en) || en.Status != 503 || errors.Is(err, ErrSemConta) {
		t.Fatalf("queria ErroNuvem 503, veio %v", err)
	}
	if a.cofre[chaveRenovacao] != p.Renovacao {
		t.Fatal("o cofre não podia mudar")
	}
	s := c.Situacao()
	if !s.Entrada || s.Motivo == MotivoSemConta {
		t.Fatalf("a sessão devia continuar: %+v", s)
	}
	if _, err := os.Stat(filepath.Join(a.pasta, arquivoLicencaNome)); err != nil {
		t.Fatalf("licenca.json devia ficar: %v", err)
	}
}

func TestIniciarParaNoCancelamento(t *testing.T) {
	a := novoAmbiente(t, 0)
	c, _ := a.entrou(t, nil)
	ctx, cancelar := context.WithCancel(context.Background())
	c.Iniciar(ctx)
	esperar(t, func() bool { return c.laco.Load() == 1 })
	cancelar()
	esperar(t, func() bool { return c.laco.Load() == 0 })
}

// cofreQueFalha deixa ligar e desligar a falha ao gravar.
type cofreQueFalha struct {
	segredos.Cofre
	falha atomic.Bool
}

func (c *cofreQueFalha) Gravar(p, v string) error {
	if c.falha.Load() {
		return errors.New("cofre trancado")
	}
	return c.Cofre.Gravar(p, v)
}

func TestErroCofreFicaVisivel(t *testing.T) {
	a := novoAmbiente(t, 0)
	cf := &cofreQueFalha{Cofre: a.cofre}
	p := a.parConfirmado(t, "ana@exemplo.com")
	c := Nova(a.f, cf, a.pasta, a.pub, a.local)
	c.dispositivo = func() (string, error) { return dispA, nil }
	if err := c.Entrou(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if c.ErroCofre() != "" {
		t.Fatalf("erro à toa: %q", c.ErroCofre())
	}
	cf.falha.Store(true)
	a.rel.avancar(15 * time.Minute)
	acesso, err := c.Acesso(context.Background())
	if err != nil || acesso == p.Acesso {
		t.Fatalf("a sessão devia seguir na memória: %q %v", acesso, err)
	}
	if !strings.Contains(c.ErroCofre(), "cofre trancado") {
		t.Fatalf("ErroCofre = %q", c.ErroCofre())
	}
	querAgir(t, c, true, "")
	cf.falha.Store(false)
	a.rel.avancar(15 * time.Minute)
	if _, err := c.Acesso(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c.ErroCofre() != "" {
		t.Fatalf("devia limpar: %q", c.ErroCofre())
	}
}

func TestErroCofreLimpaAoSairEEntrar(t *testing.T) {
	a := novoAmbiente(t, 0)
	cf := &cofreQueFalha{Cofre: a.cofre}
	p := a.parConfirmado(t, "ana@exemplo.com")
	c := Nova(a.f, cf, a.pasta, a.pub, a.local)
	c.dispositivo = func() (string, error) { return dispA, nil }
	if err := c.Entrou(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	falhar := func() {
		cf.falha.Store(true)
		a.rel.avancar(15 * time.Minute)
		if _, err := c.Acesso(context.Background()); err != nil {
			t.Fatal(err)
		}
		cf.falha.Store(false)
		if c.ErroCofre() == "" {
			t.Fatal("devia ter erro do cofre")
		}
	}
	falhar()
	if err := c.Sair(context.Background()); err != nil {
		t.Fatal(err)
	}
	if c.ErroCofre() != "" {
		t.Fatalf("Sair devia limpar: %q", c.ErroCofre())
	}
	if err := c.Entrou(context.Background(), a.parConfirmado(t, "bia@exemplo.com")); err != nil {
		t.Fatal(err)
	}
	falhar()
	if err := c.Entrou(context.Background(), a.parConfirmado(t, "cris@exemplo.com")); err != nil {
		t.Fatal(err)
	}
	if c.ErroCofre() != "" {
		t.Fatalf("Entrou devia limpar: %q", c.ErroCofre())
	}
}

// nuvemVeCtx guarda o estado do ctx que a renovação recebeu.
type nuvemVeCtx struct {
	*Falsa
	erroCtx error
	chamou  bool
}

func (n *nuvemVeCtx) Renovar(ctx context.Context, r string) (Par, error) {
	n.chamou, n.erroCtx = true, ctx.Err()
	return n.Falsa.Renovar(ctx, r)
}

func TestRenovarSobreviveAoCancelamentoDoChamador(t *testing.T) {
	a := novoAmbiente(t, 0)
	n := &nuvemVeCtx{Falsa: a.f}
	c, p := a.entrou(t, n)
	a.rel.avancar(15 * time.Minute)
	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()
	acesso, err := c.Acesso(ctx)
	if err != nil || acesso == p.Acesso {
		t.Fatalf("a renovação devia concluir: %q %v", acesso, err)
	}
	if !n.chamou || n.erroCtx != nil {
		t.Fatalf("Renovar devia receber ctx vivo: chamou=%v err=%v", n.chamou, n.erroCtx)
	}
	if a.cofre[chaveRenovacao] == p.Renovacao {
		t.Fatal("o token novo devia estar no cofre")
	}
}
