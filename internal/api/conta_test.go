//go:build contafalsa

package api

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/config"
	"github.com/FRF-Tecnology/job-control/internal/conta"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

type ambConta struct {
	t       *testing.T
	f       *conta.Falsa
	c       *conta.Conta
	api     *API
	pasta   string
	abertas chan string
	saiu    chan struct{}
}

func novoAmbConta(t *testing.T, canal string) *ambConta {
	return novoAmbContaCom(t, canal, segredos.Memoria{})
}

func novoAmbContaCom(t *testing.T, canal string, cofre segredos.Cofre) *ambConta {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	a := &ambConta{t: t, f: conta.NovaFalsa(priv, nil), abertas: make(chan string, 8), saiu: make(chan struct{}, 1)}
	a.c = conta.Nova(a.f, cofre, t.TempDir(), pub, nil)
	a.pasta = filepath.Join(t.TempDir(), "Applyer")
	os.MkdirAll(a.pasta, 0o700)
	arm, err := armazem.Abrir(context.Background(), config.CaminhoBanco(a.pasta))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	a.api = Nova(arm, Opcoes{Hoje: func() string { return "2026-11-11" }, Cofre: segredos.Memoria{}, ProvedoresIA: map[string]ia.Provedor{},
		DetectarCLIs: func(context.Context) []ia.EstadoCLI { return nil }, Canal: canal,
		Conta: a.c, AbrirNavegador: func(u string) error { a.abertas <- u; return nil },
		AoApagar: func() { a.saiu <- struct{}{} }})
	a.api.cadenciaAssinar, a.api.prazoAssinar = 20*time.Millisecond, 2*time.Second
	return a
}

func (a *ambConta) chamar(m, caminho string, corpo any, quer int) Objeto {
	a.t.Helper()
	r, err := a.api.Chamar(context.Background(), Pedido{Metodo: m, Caminho: Prefixo + caminho, Corpo: corpo, Autor: "eu"})
	if err != nil {
		a.t.Fatalf("%s %s: erro inesperado %v", m, caminho, err)
	}
	if r.Status != quer {
		a.t.Fatalf("%s %s = %d %v; queria %d", m, caminho, r.Status, r.Corpo, quer)
	}
	o := Objeto{}
	if r.Corpo != nil { // passa pelo JSON, como a tela recebe
		b, _ := json.Marshal(r.Corpo)
		if err := json.Unmarshal(b, &o); err != nil {
			a.t.Fatal(err)
		}
	}
	return o
}

func (a *ambConta) entrar() {
	a.t.Helper()
	a.chamar("POST", "/conta/criar", Objeto{"email": "ana@exemplo.com", "senha": "senha-forte-1", "termos": conta.TermosVersao}, 202)
	a.chamar("POST", "/conta/confirmar", Objeto{"email": "ana@exemplo.com", "codigo": conta.CodigoFalso, "senha": "senha-forte-1"}, 200)
}

func TestContaFluxoPelaAPI(t *testing.T) {
	a := novoAmbConta(t, "")
	if s := a.chamar("GET", "/conta", nil, 200); s["entrada"] != false || s["motivo"] != "sem-conta" {
		t.Fatalf("antes de entrar: %v", s)
	}
	a.chamar("POST", "/conta/criar", Objeto{"email": "ana@exemplo.com", "senha": "senha-forte-1", "termos": conta.TermosVersao}, 202)
	s := a.chamar("POST", "/conta/confirmar", Objeto{"email": "ana@exemplo.com", "codigo": conta.CodigoFalso, "senha": "senha-forte-1"}, 200)
	if s["entrada"] != true {
		t.Fatalf("confirmar: %v", s)
	}
	s = a.chamar("GET", "/conta", nil, 200)
	if s["entrada"] != true || s["estado"] != "teste" || s["email"] != "ana@exemplo.com" || s["motivo"] != "" || s["google"] != "" {
		t.Fatalf("GET /conta: %v", s)
	}
	a.chamar("POST", "/conta/sair", Objeto{}, 204)
	if s := a.chamar("GET", "/conta", nil, 200); s["entrada"] != false {
		t.Fatalf("depois de sair: %v", s)
	}
	s = a.chamar("POST", "/conta/entrar", Objeto{"email": "ana@exemplo.com", "senha": "senha-forte-1"}, 200)
	if s["entrada"] != true || s["estado"] != "teste" {
		t.Fatalf("entrar: %v", s)
	}
	a.chamar("POST", "/conta/recuperar", Objeto{"email": "ana@exemplo.com"}, 202)
	a.chamar("POST", "/conta/nova-senha", Objeto{"email": "ana@exemplo.com", "codigo": conta.CodigoFalso, "senha": "outra-senha-forte"}, 200)
	a.chamar("POST", "/conta/google/cancelar", Objeto{}, 204)
}

func TestContaErroDaNuvemRepassado(t *testing.T) {
	a := novoAmbConta(t, "")
	o := a.chamar("POST", "/conta/entrar", Objeto{"email": "ninguem@exemplo.com", "senha": "senha-forte-1"}, 401)
	if o["erro"] != "Email ou senha incorretos" {
		t.Fatalf("erro repassado: %v", o)
	}
	// Termos que mudaram: a nuvem diz o motivo e a versão; reenviar com a versão nova funciona.
	a.entrar()
	a.chamar("POST", "/conta/sair", Objeto{}, 204)
	a.f.MudarTermos("1-novos")
	o = a.chamar("POST", "/conta/entrar", Objeto{"email": "ana@exemplo.com", "senha": "senha-forte-1"}, 409)
	if o["motivo"] != "termos" || o["versao"] != "1-novos" {
		t.Fatalf("termos: %v", o)
	}
	a.chamar("POST", "/conta/entrar", Objeto{"email": "ana@exemplo.com", "senha": "senha-forte-1", "termos": "1-novos"}, 200)
	// Corpo inválido: 400, sem tocar na nuvem.
	a.chamar("POST", "/conta/criar", Objeto{"email": "ana@exemplo.com"}, 400)
	a.chamar("POST", "/conta/entrar", Objeto{"email": "a@b.c", "senha": "x", "tudo": true}, 400)
}

func TestContaSemInternet503(t *testing.T) {
	a := novoAmbConta(t, "")
	a.f.SemInternet(true)
	o := a.chamar("POST", "/conta/entrar", Objeto{"email": "ana@exemplo.com", "senha": "senha-forte-1"}, 503)
	if o["erro"] != "Sem conexão com a internet" || o["motivo"] != "sem-internet" {
		t.Fatalf("sem internet: %v", o)
	}
}

func TestContaSoDaTela(t *testing.T) {
	a := novoAmbConta(t, "")
	rotas := [][2]string{{"GET", "/conta"}, {"POST", "/conta/criar"}, {"POST", "/conta/sair"}, {"POST", "/conta/excluir"},
		{"GET", "/assinatura"}, {"POST", "/assinatura/assinar"}, {"POST", "/assinatura/gerenciar"}}
	for _, r := range rotas {
		resp, err := a.api.Chamar(context.Background(), Pedido{Metodo: r[0], Caminho: Prefixo + r[1], Corpo: Objeto{}, Autor: "claude"})
		if err != nil || resp.Status != 403 {
			t.Errorf("%s %s com autor claude = %d %v", r[0], r[1], resp.Status, err)
		}
	}
	// Excluir com apagarDados também (passa pela trava de escrita).
	if resp, _ := a.api.Chamar(context.Background(), Pedido{Metodo: "POST", Caminho: Prefixo + "/conta/excluir", Autor: "claude",
		Corpo: Objeto{"email": "ana@exemplo.com", "apagarDados": true}}); resp.Status != 403 {
		t.Errorf("excluir+apagar pelo assistente = %d", resp.Status)
	}
	// API do assistente (canal mcp): tudo 403, mesmo com autor eu.
	m := novoAmbConta(t, "mcp")
	for _, r := range rotas {
		resp, err := m.api.Chamar(context.Background(), Pedido{Metodo: r[0], Caminho: Prefixo + r[1], Corpo: Objeto{}, Autor: "eu"})
		if err != nil || resp.Status != 403 {
			t.Errorf("canal mcp %s %s = %d %v", r[0], r[1], resp.Status, err)
		}
	}
	if resp, _ := m.api.Chamar(context.Background(), Pedido{Metodo: "POST", Caminho: Prefixo + "/conta/excluir", Autor: "eu",
		Corpo: Objeto{"email": "ana@exemplo.com", "apagarDados": true}}); resp.Status != 403 {
		t.Errorf("canal mcp excluir+apagar = %d", resp.Status)
	}
}

func TestContaIndisponivelSemConta(t *testing.T) {
	arm, _ := armazem.Abrir(context.Background(), ":memory:")
	t.Cleanup(func() { arm.Fechar() })
	api := Nova(arm, Opcoes{Cofre: segredos.Memoria{}})
	if r, _ := api.Chamar(context.Background(), Pedido{Metodo: "GET", Caminho: Prefixo + "/conta"}); r.Status != 503 {
		t.Fatalf("sem Conta: %d", r.Status)
	}
}

func TestAssinarAbreNavegador(t *testing.T) {
	a := novoAmbConta(t, "")
	a.entrar()
	a.chamar("POST", "/assinatura/assinar", Objeto{}, 202)
	if u := <-a.abertas; u != "https://stripe.exemplo.com/checkout" {
		t.Fatalf("URL aberta: %q", u)
	}
	// O pagamento acontece: o acompanhamento em segundo plano pega a licença nova sozinho.
	a.f.Assinante("ana@exemplo.com")
	espera(t, func() bool { return a.c.Situacao().Estado == "assinante" })
	espera(t, func() bool { return !a.api.acompanhando.Load() }) // parou ao virar assinante

	a.chamar("POST", "/assinatura/gerenciar", Objeto{}, 202)
	if u := <-a.abertas; u != "https://stripe.exemplo.com/portal" {
		t.Fatalf("URL do portal: %q", u)
	}
	o := a.chamar("GET", "/assinatura", nil, 200)
	if _, tem := o["notas"]; !tem || o["situacao"] == nil {
		t.Fatalf("GET /assinatura: %v", o)
	}
}

func TestAssinarUmAcompanhamentoPorVez(t *testing.T) {
	a := novoAmbConta(t, "")
	a.entrar()
	a.api.prazoAssinar = 300 * time.Millisecond
	a.chamar("POST", "/assinatura/assinar", Objeto{}, 202)
	if !a.api.acompanhando.Load() {
		t.Fatal("deveria estar acompanhando")
	}
	a.chamar("POST", "/assinatura/assinar", Objeto{}, 202)
	espera(t, func() bool { return !a.api.acompanhando.Load() }) // o prazo acaba
}

func TestAssinarSemNavegador(t *testing.T) {
	a := novoAmbConta(t, "")
	a.entrar()
	a.api.abrirNavegador = nil
	o := a.chamar("POST", "/assinatura/assinar", Objeto{}, 503)
	if o["motivo"] != "sem-navegador" || a.api.acompanhando.Load() {
		t.Fatalf("sem navegador: %v", o)
	}
}

func TestExcluirComApagarDados(t *testing.T) {
	a := novoAmbConta(t, "")
	a.entrar()
	// Email errado: a nuvem recusa e nada local é apagado.
	a.chamar("POST", "/conta/excluir", Objeto{"email": "outra@exemplo.com", "apagarDados": true}, 400)
	if _, err := os.Stat(config.CaminhoBanco(a.pasta)); err != nil || !a.c.Situacao().Entrada {
		t.Fatalf("a falha da nuvem não pode apagar nada: %v", err)
	}
	r := a.chamar("POST", "/conta/excluir", Objeto{"email": "ana@exemplo.com", "apagarDados": true}, 200)
	if r["completo"] != true || r["fechar"] != true {
		t.Fatalf("apagar: %v", r)
	}
	if a.c.Situacao().Entrada {
		t.Fatal("a sessão local deveria ter saído")
	}
	if _, err := os.Stat(config.CaminhoBanco(a.pasta)); !os.IsNotExist(err) {
		t.Fatalf("o banco deveria ter sido apagado: %v", err)
	}
	select {
	case <-a.saiu:
	case <-time.After(3 * time.Second):
		t.Fatal("o app não fechou")
	}
	// A conta não existe mais na nuvem.
	if _, err := a.f.Entrar(context.Background(), "ana@exemplo.com", "senha-forte-1", dispTeste, conta.TermosVersao); err == nil {
		t.Fatal("a conta deveria ter sido excluída na nuvem")
	}
}

func TestExcluirSemApagarDados(t *testing.T) {
	a := novoAmbConta(t, "")
	a.entrar()
	a.chamar("POST", "/conta/excluir", Objeto{"email": "ana@exemplo.com"}, 204)
	if a.c.Situacao().Entrada {
		t.Fatal("a sessão local deveria ter saído")
	}
	if _, err := os.Stat(config.CaminhoBanco(a.pasta)); err != nil {
		t.Fatalf("sem apagarDados o banco fica: %v", err)
	}
}

const dispTeste = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func espera(t *testing.T, cond func() bool) {
	t.Helper()
	fim := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(fim) {
			t.Fatal("tempo esgotado esperando a condição")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (a *ambConta) contaExisteNaNuvem() bool {
	_, err := a.f.Entrar(context.Background(), "ana@exemplo.com", "senha-forte-1", dispTeste, conta.TermosVersao)
	return err == nil
}

func TestExcluirComEnvioEmAndamentoNaoTocaNaNuvem(t *testing.T) {
	a := novoAmbConta(t, "")
	a.entrar()
	a.chamar("POST", "/vagas", Objeto{"id": "li-1", "empresa": "Empresa Teste", "titulo": "Dev Go"}, 201)
	semear := a.api.arm
	if err := semear.Gravar(context.Background(), func(e dominio.Estado) (dominio.Estado, error) {
		e.Candidaturas = append(e.Candidaturas, Objeto{"id": "c-1", "vagaId": "li-1", "estado": dominio.CandEnviando})
		return e, nil
	}); err != nil {
		t.Fatal(err)
	}
	o := a.chamar("POST", "/conta/excluir", Objeto{"email": "ana@exemplo.com", "apagarDados": true}, 409)
	if o["estado"] != "enviando" {
		t.Fatalf("409: %v", o)
	}
	if !a.c.Situacao().Entrada {
		t.Fatal("a sessão local deveria estar intacta")
	}
	if !a.contaExisteNaNuvem() {
		t.Fatal("a conta não pode ter sido excluída na nuvem")
	}
	if _, err := os.Stat(config.CaminhoBanco(a.pasta)); err != nil {
		t.Fatalf("o banco deveria estar intacto: %v", err)
	}
}

// cofre que não consegue apagar: a sessão local não sai depois de a nuvem excluir a conta.
type cofreSemApagar struct{ segredos.Memoria }

func (cofreSemApagar) Apagar(string) error { return errors.New("cofre travado (teste)") }

func TestExcluirNuvemOkApagarFalhaAvisa(t *testing.T) {
	a := novoAmbContaCom(t, "", cofreSemApagar{segredos.Memoria{}})
	a.entrar()
	o := a.chamar("POST", "/conta/excluir", Objeto{"email": "ana@exemplo.com", "apagarDados": true}, 200)
	if o["contaExcluida"] != true || o["dadosApagados"] != false || o["erro"] == "" || o["erro"] == nil {
		t.Fatalf("aviso: %v", o)
	}
	if a.contaExisteNaNuvem() {
		t.Fatal("a conta deveria ter sido excluída na nuvem")
	}
	// Os dados ficaram, e POST /dados/apagar continua valendo para tentar de novo.
	if _, err := os.Stat(config.CaminhoBanco(a.pasta)); err != nil {
		t.Fatalf("os dados deveriam ter ficado: %v", err)
	}
	r := a.chamar("POST", "/dados/apagar", Objeto{"confirmacao": FraseApagar}, 200)
	if r["fechar"] != true {
		t.Fatalf("apagar depois do aviso: %v", r)
	}
	// Sem apagarDados o aviso é o mesmo.
	b := novoAmbContaCom(t, "", cofreSemApagar{segredos.Memoria{}})
	b.entrar()
	if o := b.chamar("POST", "/conta/excluir", Objeto{"email": "ana@exemplo.com"}, 200); o["contaExcluida"] != true {
		t.Fatalf("aviso sem apagarDados: %v", o)
	}
}

func TestExcluirSemEmailEErrosDaNuvem(t *testing.T) {
	a := novoAmbConta(t, "")
	a.entrar()
	for _, corpo := range []Objeto{{}, {"apagarDados": true}} {
		r, _ := a.api.Chamar(context.Background(), Pedido{Metodo: "POST", Caminho: Prefixo + "/conta/excluir", Corpo: corpo, Autor: "eu"})
		if r.Status != 400 || !strings.Contains(fmt.Sprint(r.Corpo), "email: obrigatório") {
			t.Fatalf("sem email: %d %v", r.Status, r.Corpo)
		}
	}
	for _, c := range []struct {
		en   *conta.ErroNuvem
		quer int
		mot  string
	}{
		{&conta.ErroNuvem{Status: 403, Msg: "Recusado"}, 403, "nuvem-recusou"},
		{&conta.ErroNuvem{Status: 429, Msg: "Devagar", Motivo: "limite"}, 429, "limite"},
		{&conta.ErroNuvem{Status: 500, Msg: "pg: falha interna"}, 502, "nuvem"},
		{&conta.ErroNuvem{Status: 418, Msg: "xícara"}, 502, "nuvem"},
	} {
		err := erroDaConta(c.en)
		r, _ := respostaDeErro(err)
		corpo, _ := r.Corpo.(Objeto)
		if r.Status != c.quer || corpo["motivo"] != c.mot {
			t.Errorf("%d: %d %v", c.en.Status, r.Status, r.Corpo)
		}
		if c.quer == 502 && corpo["erro"] != "A nuvem não respondeu direito. Tente de novo." {
			t.Errorf("502 vazou texto: %v", corpo)
		}
	}
}
