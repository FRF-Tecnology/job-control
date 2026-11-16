package api

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/caixa"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/backend"
	"github.com/emersion/go-imap/backend/memory"
	"github.com/emersion/go-imap/client"
	"github.com/emersion/go-imap/server"
)

// Senha de app fictícia, com os espaços que o Google mostra; o servidor de teste aceita "password".
const senhaDeAppFicticia = "pass word"

// leitorDeTeste aponta o leitor para um servidor IMAP local (memória do go-imap, usuário "username"). fora=true
// simula o servidor fora do ar.
func leitorDeTeste(t *testing.T, fora bool) caixa.Leitor {
	t.Helper()
	if fora {
		return caixa.Leitor{Discar: func(context.Context, string) (*client.Client, error) { return nil, errors.New("recusou") }}
	}
	s := server.New(comEndereco{memory.New()})
	s.AllowInsecureAuth = true
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go s.Serve(l)
	t.Cleanup(func() { s.Close() })
	return caixa.Leitor{Discar: func(context.Context, string) (*client.Client, error) { return client.Dial(l.Addr().String()) }}
}

// comEndereco: o servidor de memória só conhece "username"; aqui ele entra com um endereço fictício.
type comEndereco struct{ backend.Backend }

func (b comEndereco) Login(c *imap.ConnInfo, usuario, senha string) (backend.User, error) {
	if usuario != "username@example.com" {
		return nil, backend.ErrInvalidCredentials
	}
	return b.Backend.Login(c, "username", senha)
}

func apiComEmail(t *testing.T, fora bool) (*API, segredos.Memoria) {
	t.Helper()
	cofre := segredos.Memoria{}
	a := apiDoContrato(t, cofre)
	a.leitorEmail = leitorDeTeste(t, fora)
	return a, cofre
}

func chamarEmail(t *testing.T, a *API, m string, corpo Objeto, autor string) Resposta {
	t.Helper()
	r, err := a.Chamar(context.Background(), Pedido{Metodo: m, Caminho: Prefixo + "/email/conta", Corpo: corpo, Autor: autor})
	if err != nil {
		t.Fatalf("%s /email/conta: erro inesperado %v", m, err)
	}
	return r
}

func TestConectarEmailGuardaASenhaSoNoCofre(t *testing.T) {
	a, cofre := apiComEmail(t, false)
	ctx := context.Background()
	if r := chamarEmail(t, a, "GET", Objeto{}, "eu"); r.Status != 200 || r.Corpo.(Objeto)["conectada"] != false {
		t.Fatalf("sem conta: %d %v", r.Status, r.Corpo)
	}
	// Endereço de domínio sem servidor conhecido: o servidor vem informado.
	r := chamarEmail(t, a, "PUT", Objeto{"usuario": " username@example.com ", "senha": senhaDeAppFicticia, "servidor": "imap.example.test"}, "eu")
	if r.Status != 200 {
		t.Fatalf("conectar: %d %v", r.Status, r.Corpo)
	}
	b, _ := json.Marshal(r.Corpo)
	if strings.Contains(string(b), "word") {
		t.Fatalf("a senha voltou na resposta: %s", b)
	}
	if cofre[chaveCofreEmail] != "password" {
		t.Fatalf("o cofre devia ter a senha sem espaços, tem %q", cofre[chaveCofreEmail])
	}
	var meta map[string]any
	a.arm.LerMeta(ctx, chaveMetaEmail, &meta)
	if mb, _ := json.Marshal(meta); strings.Contains(string(mb), "word") || meta["usuario"] != "username@example.com" || meta["servidor"] != "imap.example.test" {
		t.Fatalf("meta do e-mail: %s", mb)
	}
	r = chamarEmail(t, a, "GET", Objeto{}, "eu")
	if c := r.Corpo.(Objeto); c["conectada"] != true || c["usuario"] != "username@example.com" {
		t.Fatalf("depois de conectar: %v", c)
	}

	// Desconectar apaga o cofre e o banco; de novo responde igual.
	for range 2 {
		if r := chamarEmail(t, a, "DELETE", Objeto{}, "eu"); r.Status != 200 || r.Corpo.(Objeto)["conectada"] != false {
			t.Fatalf("desconectar: %d %v", r.Status, r.Corpo)
		}
	}
	if _, tem := cofre[chaveCofreEmail]; tem {
		t.Fatal("a senha ficou no cofre")
	}
	if c := chamarEmail(t, a, "GET", Objeto{}, "eu").Corpo.(Objeto); c["conectada"] != false || c["usuario"] != "" {
		t.Fatalf("depois de desconectar: %v", c)
	}
}

func TestConectarEmailNaoGravaQuandoNaoEntra(t *testing.T) {
	casos := []struct {
		nome   string
		fora   bool
		corpo  Objeto
		status int
	}{
		{"senha errada", false, Objeto{"usuario": "username@example.com", "senha": "outra", "servidor": "imap.example.test"}, 400},
		{"servidor fora do ar", true, Objeto{"usuario": "username@example.com", "senha": senhaDeAppFicticia, "servidor": "imap.example.test"}, 502},
		{"domínio sem servidor", false, Objeto{"usuario": "username@example.com", "senha": senhaDeAppFicticia}, 400},
		{"sem endereço", false, Objeto{"usuario": "username", "senha": senhaDeAppFicticia, "servidor": "imap.example.test"}, 400},
		{"sem senha", false, Objeto{"usuario": "username@example.com", "senha": "  ", "servidor": "imap.example.test"}, 400},
		{"servidor com porta", false, Objeto{"usuario": "username@example.com", "senha": senhaDeAppFicticia, "servidor": "imap.example.test:143"}, 400},
		{"campo a mais", false, Objeto{"usuario": "username@example.com", "senha": senhaDeAppFicticia, "porta": 143}, 400},
	}
	for _, c := range casos {
		a, cofre := apiComEmail(t, c.fora)
		r := chamarEmail(t, a, "PUT", c.corpo, "eu")
		if r.Status != c.status {
			t.Fatalf("%s: %d %v; queria %d", c.nome, r.Status, r.Corpo, c.status)
		}
		if b, _ := json.Marshal(r.Corpo); strings.Contains(string(b), "word") {
			t.Fatalf("%s: a senha voltou no erro: %s", c.nome, b)
		}
		if len(cofre) != 0 || chamarEmail(t, a, "GET", Objeto{}, "eu").Corpo.(Objeto)["conectada"] != false {
			t.Fatalf("%s: gravou mesmo sem entrar", c.nome)
		}
	}
}

func TestEmailSoPelaTela(t *testing.T) {
	a, cofre := apiComEmail(t, false)
	for _, m := range []string{"GET", "PUT", "DELETE"} {
		if r := chamarEmail(t, a, m, Objeto{"usuario": "username@example.com", "senha": senhaDeAppFicticia, "servidor": "imap.example.test"}, "claude"); r.Status != 403 {
			t.Fatalf("%s pelo assistente: %d %v", m, r.Status, r.Corpo)
		}
	}
	// Na API do assistente (MCP), nem como "eu".
	mcp := Nova(a.arm, Opcoes{Canal: dominio.CanalMCP, Cofre: cofre})
	mcp.leitorEmail = a.leitorEmail
	if r := chamarEmail(t, mcp, "PUT", Objeto{"usuario": "username@example.com", "senha": senhaDeAppFicticia, "servidor": "imap.example.test"}, "eu"); r.Status != 403 {
		t.Fatalf("PUT pelo MCP: %d %v", r.Status, r.Corpo)
	}
	if len(cofre) != 0 {
		t.Fatal("o assistente gravou a conta")
	}
}
