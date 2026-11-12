package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// Provedor falso: responde sempre o mesmo JSON e guarda a chave que recebeu.
type provedorFalso struct {
	resposta string
	chave    *string
}

func (provedorFalso) Nome() string { return "anthropic" }
func (f provedorFalso) Chamar(ctx context.Context, p ia.PedidoProvedor) (ia.RespostaProvedor, error) {
	*f.chave = p.Chave
	n := 50
	return ia.RespostaProvedor{Texto: f.resposta, TokensEntrada: &n, TokensSaida: &n}, nil
}

func TestRotasDeIA(t *testing.T) {
	ctx := context.Background()
	arm, err := armazem.Abrir(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer arm.Fechar()
	cofre := segredos.Memoria{}
	var chaveUsada string
	resposta := `{"decisao":"pular","aderencia":"C","motivo":"pede 10 anos"}`
	a := Nova(arm, Opcoes{Hoje: func() string { return "2026-11-08" }, Cofre: cofre,
		ProvedoresIA: map[string]ia.Provedor{"anthropic": provedorFalso{resposta: resposta, chave: &chaveUsada}},
		DetectarCLIs: func(context.Context) []ia.EstadoCLI { return []ia.EstadoCLI{{InfoCLI: ia.CLIs[0], Instalada: true}} }})
	chamar := func(metodo, caminho string, corpo any) (int, map[string]any) {
		t.Helper()
		if corpo == nil {
			corpo = Objeto{}
		}
		r, err := a.Chamar(ctx, Pedido{Metodo: metodo, Caminho: Prefixo + caminho, Corpo: normal(corpo)})
		if err != nil {
			t.Fatalf("%s %s: %v", metodo, caminho, err)
		}
		var m map[string]any
		json.Unmarshal([]byte(emJSON(r.Corpo)), &m)
		return r.Status, m
	}

	// Conta nova: nada conectado; a CLI detectada aparece.
	s, c := chamar("GET", "/ia", nil)
	if s != 200 || c["configurada"] != false || len(c["clis"].([]any)) != 1 || c["chaves"].(map[string]any)["anthropic"] != nil {
		t.Fatalf("estado inicial = %d %v", s, c)
	}
	if s, c := chamar("POST", "/ia/testar", nil); s != 400 || c["tipo"] != "config" {
		t.Fatalf("testar sem IA = %d %v", s, c)
	}

	// Chave: vai para o cofre; a tela só vê o final.
	if s, c := chamar("PUT", "/ia/chaves/anthropic", Objeto{"chave": "  sk-ant-chave-de-teste-1234  "}); s != 200 || c["final"] != "…1234" {
		t.Fatalf("salvar chave = %d %v", s, c)
	}
	if cofre["anthropic"] != "sk-ant-chave-de-teste-1234" {
		t.Fatalf("cofre = %v", cofre)
	}
	if s, _ := chamar("PUT", "/ia/chaves/claude-cli", Objeto{"chave": "xxxxxxxxxxxx"}); s != 400 {
		t.Fatalf("chave para CLI = %d; CLI usa login, não chave", s)
	}
	if s, _ := chamar("PUT", "/ia/chaves/anthropic", Objeto{"chave": "curta"}); s != 400 {
		t.Fatalf("chave curta = %d", s)
	}

	// Escolher provedor: modelo padrão; provedor desconhecido e endereço sem https são recusados.
	if s, c := chamar("PUT", "/ia/config", Objeto{"provedor": "anthropic"}); s != 200 || c["modelo"] != "claude-haiku-5-5" {
		t.Fatalf("config = %d %v", s, c)
	}
	if s, _ := chamar("PUT", "/ia/config", Objeto{"provedor": "skynet"}); s != 400 {
		t.Fatalf("provedor desconhecido = %d", s)
	}
	if s, _ := chamar("PUT", "/ia/config", Objeto{"provedor": "openai", "modelo": "x", "url": "http://inseguro"}); s != 400 {
		t.Fatalf("url sem https = %d", s)
	}
	if s, _ := chamar("PUT", "/ia/config", Objeto{"provedor": "openai"}); s != 400 {
		t.Fatalf("openai sem modelo = %d", s)
	}

	// Testar e opinar sobre uma vaga: a chave do cofre chega ao provedor; a guarda age (pular → motivoPulo).
	if s, c := chamar("POST", "/ia/testar", nil); s != 200 || c["ok"] != true || chaveUsada != "sk-ant-chave-de-teste-1234" {
		t.Fatalf("testar = %d %v (chave %q)", s, c, chaveUsada)
	}
	if _, c := chamar("GET", "/ia/config", nil); c["conexaoVerificada"] != true {
		t.Fatal("teste real da rota não registrado")
	}
	chamar("POST", "/vagas", Objeto{"id": "li-1", "empresa": "Acme", "titulo": "Dev"})
	if s, c := chamar("POST", "/ia/triar/li-1", nil); s != 200 || c["motivoPulo"] != "Outro" {
		t.Fatalf("triar = %d %v", s, c)
	}
	if s, _ := chamar("POST", "/ia/triar/nao-existe", nil); s != 404 {
		t.Fatalf("triar vaga inexistente = %d", s)
	}
	// Saída fora do contrato (mensagem sem "texto") vira 422.
	if s, c := chamar("POST", "/ia/mensagem/li-1", nil); s != 422 || c["tipo"] != "esquema" {
		t.Fatalf("mensagem = %d %v", s, c)
	}

	// Uso medido (sem conteúdo) e desconectar.
	if _, c := chamar("GET", "/ia", nil); c["provedor"] != "anthropic" || c["uso"].(map[string]any)["chamadas"] != float64(3) || c["uso"].(map[string]any)["falhas"] != float64(1) {
		t.Fatalf("estado depois = %v", c)
	}
	chamar("PUT", "/ia/config", Objeto{"provedor": "nenhum"})
	chamar("DELETE", "/ia/chaves/anthropic", nil)
	if _, c := chamar("GET", "/ia", nil); c["configurada"] != false || c["chaves"].(map[string]any)["anthropic"] != nil {
		t.Fatalf("depois de desconectar = %v", c)
	}
}
