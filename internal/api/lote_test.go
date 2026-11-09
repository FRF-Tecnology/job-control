package api

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"sort"
	"strings"
	"testing"
)

// jc-a2y.6.13: aprovar a lista do dia em lote, pela tela (HTTP de verdade). Dados fictícios.
func TestAprovarLotePelaAPI(t *testing.T) {
	var saida bytes.Buffer
	log.SetOutput(&saida)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	f := novaFila(t)
	f.chamar("POST", "/vagas", Objeto{"id": "li-2", "empresa": "Empresa Dois", "titulo": "Dev Go", "plataforma": "linkedin", "link": "https://www.linkedin.com/jobs/view/9000000002"}, 201)
	f.chamar("POST", "/vagas/li-2/mover", Objeto{"etapa": "aplicar"}, 200)
	c1 := f.chamar("POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": conteudoTeste()}, 201)
	c2 := f.chamar("POST", "/candidaturas", Objeto{"vagaId": "li-2", "conteudo": conteudoTeste()}, 201)
	item := func(c Objeto) Objeto { return Objeto{"id": c["id"], "impressao": c["impressao"]} }
	velha := item(c2)
	velha["impressao"] = "sha256:v1:" + strings.Repeat("0", 64) // o pacote mudou depois que a tela mostrou

	// Assistente: 403, e nada muda.
	antes := f.versao()
	if r := f.comoAssistente("POST", "/candidaturas/aprovar-lote", Objeto{"itens": []any{item(c1)}}); r.Status != 403 {
		t.Fatalf("assistente = %d", r.Status)
	}
	if f.versao() != antes || f.chamar("GET", "/candidaturas/"+c1["id"].(string), nil, 200)["estado"] != "preparada" {
		t.Fatal("a recusa ao assistente aprovou")
	}
	// A API confere o autor antes de olhar os itens (trava própria, além da do domínio).
	if r := f.comoAssistente("POST", "/candidaturas/aprovar-lote", Objeto{"itens": []any{}}); r.Status != 403 {
		t.Fatalf("assistente com lista vazia = %d; queria 403", r.Status)
	}

	// Corpo estrito: nada aprovado.
	for nome, corpo := range map[string]Objeto{
		"campo a mais":      {"itens": []any{item(c1)}, "todos": true},
		"vazio":             {"itens": []any{}},
		"item com campo":    {"itens": []any{Objeto{"id": c1["id"], "impressao": c1["impressao"], "versao": 1}}},
		"impressão fora":    {"itens": []any{Objeto{"id": c1["id"], "impressao": "sha256:v1:xyz"}}},
		"id fora do padrão": {"itens": []any{Objeto{"id": "li-1", "impressao": c1["impressao"]}}},
		"id repetido":       {"itens": []any{item(c1), item(c1)}},
		"itens não é lista": {"itens": "todos"},
		"grande demais":     {"itens": muitos(item(c1), 121)},
	} {
		f.chamar("POST", "/candidaturas/aprovar-lote", corpo, 400)
		if f.versao() != antes {
			t.Fatalf("%s: gravou", nome)
		}
	}

	// A tela: c1 aprovada; c2 recusada (impressão velha) sem afetar c1.
	r := f.chamar("POST", "/candidaturas/aprovar-lote", Objeto{"itens": []any{item(c1), velha}}, 200)
	b, _ := json.Marshal(r)
	if string(b) != `{"aprovadas":["`+c1["id"].(string)+`"],"recusadas":[{"id":"`+c2["id"].(string)+`","motivo":"impressao-mudou"}]}` {
		t.Fatalf("resposta = %s", b)
	}
	if f.chamar("GET", "/candidaturas/"+c1["id"].(string), nil, 200)["estado"] != "aprovada" || f.chamar("GET", "/candidaturas/"+c2["id"].(string), nil, 200)["estado"] != "preparada" {
		t.Fatal("estados depois do lote")
	}
	// Tudo recusado também é 200.
	r = f.chamar("POST", "/candidaturas/aprovar-lote", Objeto{"itens": []any{item(c1)}}, 200)
	if len(r["aprovadas"].([]any)) != 0 || r["recusadas"].([]any)[0].(Objeto)["motivo"] != "estado" {
		t.Fatalf("de novo = %v", r)
	}
	// O log: só números e o autor.
	linhas := []string{}
	for _, l := range strings.Split(saida.String(), "\n") {
		if strings.Contains(l, "fila: lote") {
			linhas = append(linhas, l[strings.Index(l, "fila: "):])
		}
	}
	sort.Strings(linhas)
	if strings.Join(linhas, "|") != "fila: lote aprovado (eu): 0 aprovadas, 1 recusada(s)|fila: lote aprovado (eu): 1 aprovadas, 1 recusada(s)" {
		t.Fatalf("log = %v", linhas)
	}
	if strings.Contains(saida.String(), "cand-") || strings.Contains(saida.String(), marcaConteudo) {
		t.Fatal("o log tem id ou conteúdo")
	}
}

func muitos(o Objeto, n int) []any {
	l := make([]any, n)
	for i := range l {
		l[i] = o
	}
	return l
}
