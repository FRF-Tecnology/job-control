package assistente

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/api"
	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// jc-a2y.7.4 (PR 3): a tabela de ferramentas. Dados fictícios.

// argsDeExemplo: argumentos válidos de cada ferramenta (fictícios).
var argsDeExemplo = map[string]map[string]any{
	"vaga_detalhe":        {"id": "v-1"},
	"vaga_duplicada":      {"empresa": "Empresa Teste", "titulo": "Dev Go"},
	"vaga_adicionar":      {"vagas": []any{map[string]any{"id": "v-9", "empresa": "Empresa Nove", "titulo": "Dev Go", "link": "https://exemplo.com/9"}}},
	"vaga_mover":          {"id": "v-1", "etapa": "pulada", "motivo": "Outro"},
	"vaga_editar":         {"id": "v-1", "campos": map[string]any{"notas": "Nota fictícia"}},
	"evento_registrar":    {"id": "v-1", "tipo": "nota", "texto": "Nota fictícia"},
	"resposta_padrao":     {"pergunta": "Quantos anos de Go?"},
	"followup_registrar":  {"id": "v-1", "acao": "adiar", "dias": 3.0},
	"rodada_registrar":    {"n": 1.0, "resumo": "Rodada fictícia"},
	"pedido_concluir":     {"id": "pd-1", "resposta": "Feito"},
	"candidatura_detalhe": {"id": "cd-1"},
	"candidatura_revogar": {"id": "cd-1"},
	"plataforma_pausar":   {"plataforma": "linkedin"},
}

// Cada ferramenta usa só a rota dela, que está na lista do assistente (A8: sem rota livre).
func TestCadaFerramentaUsaASuaRota(t *testing.T) {
	vistos := map[string]bool{}
	for _, f := range Ferramentas {
		if vistos[f.Nome] {
			t.Fatalf("%s repetida", f.Nome)
		}
		vistos[f.Nome] = true
		if f.Entrada["type"] != "object" || f.Entrada["additionalProperties"] != false {
			t.Errorf("%s: inputSchema sem additionalProperties false", f.Nome)
		}
		m, c, _ := strings.Cut(f.Rota, " ")
		if modelo, ok := api.ModeloDoAssistente(m, strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(c, "{id}", "x-1"), "{n}", "1"), "{plataforma}", "linkedin"), "{x}", "x")); !ok || modelo != f.Rota {
			t.Errorf("%s: rota %q fora da lista do assistente", f.Nome, f.Rota)
		}
		chamadas := 0
		falso := func(metodo, caminho string, corpo any) (any, error) {
			chamadas++
			if m, ok := api.ModeloDoAssistente(metodo, caminho); !ok || m != f.Rota {
				t.Errorf("%s chamou %s %s (%q); a rota dela é %s", f.Nome, metodo, caminho, m, f.Rota)
			}
			return map[string]any{}, nil
		}
		args := argsDeExemplo[f.Nome]
		if args == nil {
			args = map[string]any{}
		}
		if p := Conferir(f.Entrada, args); len(p) > 0 {
			t.Fatalf("%s: exemplo inválido: %v", f.Nome, p)
		}
		if _, err := f.rodar(falso, args); err != nil {
			t.Errorf("%s: %v", f.Nome, err)
		}
		if chamadas == 0 {
			t.Errorf("%s não chamou nada", f.Nome)
		}
	}
	for _, proibida := range []string{"vaga_registrar_envio", "emails_registrar", "emails_sem_vaga"} {
		if vistos[proibida] {
			t.Errorf("%s está na tabela", proibida)
		}
	}
	// enviada não é etapa de vaga_mover (decisão do PO no #135).
	f, _ := Achar("vaga_mover")
	if p := Conferir(f.Entrada, map[string]any{"id": "v-1", "etapa": "enviada"}); len(p) == 0 {
		t.Fatal("vaga_mover aceitou enviada")
	}
}

// A8: entrada fora do inputSchema é recusada antes de chamar a API.
func TestConferirEntrada(t *testing.T) {
	f, _ := Achar("vaga_mover")
	for nome, args := range map[string]map[string]any{
		"sem obrigatório": {"etapa": "pulada"},
		"campo a mais":    {"id": "v-1", "etapa": "pulada", "envio": map[string]any{"data": "2026-11-09"}},
		"tipo errado":     {"id": 1.0, "etapa": "pulada"},
		"fora das opções": {"id": "v-1", "etapa": "Enviada"},
	} {
		if p := Conferir(f.Entrada, args); len(p) == 0 {
			t.Errorf("%s: aceitou", nome)
		}
	}
	p, _ := Achar("pedido_concluir")
	if r := Conferir(p.Entrada, map[string]any{"id": "pd-1", "resposta": strings.Repeat("a", 501)}); len(r) == 0 {
		t.Fatal("resposta acima do teto aceita")
	}
}

func apiDoAssistente(t *testing.T) (*api.API, *api.API) {
	t.Helper()
	arm, err := armazem.Abrir(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	hoje := func() string { return "2026-11-09" }
	return api.Nova(arm, api.Opcoes{Hoje: hoje, Canal: dominio.CanalMCP, EnvioLigado: true}), api.Nova(arm, api.Opcoes{Hoje: hoje, Aleatorio: func() string { return "abcd" }})
}

func rodar(t *testing.T, a *api.API, nome string, args map[string]any) (any, error) {
	t.Helper()
	f, ok := Achar(nome)
	if !ok {
		t.Fatalf("sem %s", nome)
	}
	return Rodar(context.Background(), a, f, args)
}

// As ferramentas pela entrada do assistente, num banco de verdade: as regras do domínio valem por cima.
func TestFerramentasPelaEntradaDoAssistente(t *testing.T) {
	a, _ := apiDoAssistente(t)
	if _, err := rodar(t, a, "vaga_adicionar", map[string]any{"vagas": []any{
		map[string]any{"id": "v-1", "empresa": "Empresa Um", "titulo": "Dev Go", "link": "https://exemplo.com/1", "etapa": "aplicar"},
		map[string]any{"id": "v-2", "empresa": "Empresa Dois", "titulo": "Dev React", "link": "https://exemplo.com/2"},
	}}); err != nil {
		t.Fatal(err)
	}
	r, err := rodar(t, a, "vagas_listar", map[string]any{"empresa": "empresa um"})
	if err != nil {
		t.Fatal(err)
	}
	o := r.(map[string]any)
	if o["total"] != 1 || dominio.ObjetoDe(dominio.ListaDe(o["vagas"])[0])["etapa"] != "descoberta" {
		t.Fatalf("vagas_listar = %v", o)
	}
	// aplicar é só da pessoa (o domínio recusa); o evento com nome de etapa, a entrada do assistente recusa.
	var ef *ErroFerramenta
	if _, err := rodar(t, a, "vaga_mover", map[string]any{"id": "v-1", "etapa": "aplicar"}); !errors.As(err, &ef) {
		t.Fatalf("aplicar pelo assistente = %v", err)
	}
	if _, err := rodar(t, a, "evento_registrar", map[string]any{"id": "v-1", "tipo": "enviada", "texto": "x"}); !errors.As(err, &ef) {
		t.Fatalf("evento enviada = %v", err)
	}
	for _, id := range []string{"../dados/apagar", "a b", "x%2F", ".oculto"} {
		if _, err := rodar(t, a, "vaga_detalhe", map[string]any{"id": id}); !errors.As(err, &ef) || ef.Msg != "id fora do formato" {
			t.Fatalf("id %q = %v", id, err)
		}
	}
	// Teto de itens no vaga_adicionar (A5.2).
	muitas := []any{}
	for i := 0; i < maxVagasPorCarga+1; i++ {
		muitas = append(muitas, map[string]any{"empresa": "E", "titulo": "T"})
	}
	if _, err := rodar(t, a, "vaga_adicionar", map[string]any{"vagas": muitas}); !errors.As(err, &ef) {
		t.Fatalf("51 vagas = %v", err)
	}
	if _, err := rodar(t, a, "envio_pausar", nil); err != nil {
		t.Fatalf("envio_pausar = %v", err)
	}
	e, err := rodar(t, a, "envio_estado", nil)
	if q := dominio.ObjetoDe(dominio.ObjetoDe(e)["pausadoPor"]); err != nil || q["canal"] != "mcp" {
		t.Fatalf("envio_estado = %v %v", e, err)
	}
	if r, err := rodar(t, a, "resposta_padrao", map[string]any{"pergunta": "Anos de Go?"}); err != nil || dominio.ObjetoDe(r)["aviso"] == nil {
		t.Fatalf("resposta_padrao sem perfil = %v %v", r, err)
	}
}

// A5.1: a resposta tem teto de 64 KB, com corte e aviso, sem quebrar um caractere no meio.
func TestTextoComTeto(t *testing.T) {
	pequeno := Texto(map[string]any{"a": "b"})
	if strings.Contains(pequeno, "cortada") {
		t.Fatal("cortou o pequeno")
	}
	grande := Texto(map[string]any{"texto": strings.Repeat("ação ", 30000)})
	if len(grande) > MaxResposta || !strings.Contains(grande, "cortada em 64 KB") || !strings.HasSuffix(grande, "limite menor)") {
		t.Fatalf("grande = %d bytes", len(grande))
	}
	if !utf8Valido(grande) {
		t.Fatal("cortou um caractere no meio")
	}
	// Caracteres de dois bytes, nas duas paridades: o corte nunca cai no meio de um.
	for _, prefixo := range []string{"", "a"} {
		if g := Texto(map[string]any{"t": prefixo + strings.Repeat("ç", 40000)}); !utf8Valido(g) || len(g) > MaxResposta {
			t.Fatalf("prefixo %q: corte inválido", prefixo)
		}
	}
}

func utf8Valido(s string) bool { return strings.ToValidUTF8(s, "�") == s }

func TestRespostasParecidas(t *testing.T) {
	padrao := []any{
		map[string]any{"pergunta": "Quantos anos de experiência com Go?", "resposta": "3"},
		map[string]any{"pergunta": "Você tem experiência com React.js?", "resposta": "Sim"},
		map[string]any{"pergunta": "Inglês", "resposta": "Avançado"},
	}
	r := RespostasParecidas(padrao, "Years of experience with Go", 3)
	if len(r) == 0 || r[0]["resposta"] != "3" {
		t.Fatalf("Go = %v", r)
	}
	if r := RespostasParecidas(padrao, "Experience with React", 3); len(r) == 0 || r[0]["resposta"] != "Sim" {
		t.Fatalf("React = %v", r)
	}
	if r := RespostasParecidas(padrao, "qual o seu?", 3); len(r) != 0 {
		t.Fatalf("pergunta vazia = %v", r)
	}
}
