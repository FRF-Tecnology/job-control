package api

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// jc-a2y.7.2, parte 2: POST /candidaturas/{id}/ler-formulario. O leitor (Opcoes.LerFormulario) é do executor, na
// parte 3; aqui, um leitor fictício. Sem leitor (build padrão, central-dev): 404. Dados fictícios.

const marcaPergunta = "Pergunta-Ficticia-Marcada-3f1a"

func filaComFormulario(t *testing.T) (*filaTeste, string) {
	t.Helper()
	f := novaFila(t)
	f.chamar("POST", "/vagas", Objeto{"id": "fm-1", "empresa": "Empresa Form", "titulo": "Dev Go", "link": "https://acme.typeform.com/to/AbC123#nome=Pessoa"}, 201)
	f.chamar("POST", "/vagas/fm-1/mover", Objeto{"etapa": "aplicar"}, 200)
	c := f.chamar("POST", "/candidaturas", Objeto{"vagaId": "fm-1", "conteudo": conteudoTeste()}, 201)
	return f, c["id"].(string)
}

func TestLerFormularioSemLeitorEh404(t *testing.T) {
	f, id := filaComFormulario(t)
	f.chamar("POST", "/candidaturas/"+id+"/ler-formulario", Objeto{}, 404)
}

func TestLerFormulario(t *testing.T) {
	var saida bytes.Buffer
	log.SetOutput(&saida)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	f, id := filaComFormulario(t)
	var lido string
	f.api.lerFormulario = func(_ context.Context, endereco string) (dominio.LeituraFormulario, error) {
		lido = endereco
		return dominio.LeituraFormulario{URLFinal: endereco, Perguntas: []dominio.PerguntaFormulario{
			{ID: "p1", Texto: marcaPergunta, Tipo: "texto-longo", Obrigatoria: true},
			{ID: "p2", Texto: "Qual o seu e-mail?", Tipo: "email", Obrigatoria: true},
		}}, nil
	}
	antes := f.versao()
	// Só a tela; corpo estrito: nada é lido nem gravado.
	if r := f.comoAssistente("POST", "/candidaturas/"+id+"/ler-formulario", Objeto{}); r.Status != 403 {
		t.Fatalf("assistente = %d", r.Status)
	}
	f.chamar("POST", "/candidaturas/"+id+"/ler-formulario", Objeto{"url": "https://exemplo.com"}, 400)
	if f.versao() != antes || lido != "" {
		t.Fatal("a recusa leu ou gravou")
	}
	c := f.chamar("POST", "/candidaturas/"+id+"/ler-formulario", Objeto{}, 200)
	if lido != "https://acme.typeform.com/to/AbC123" { // a canônica, sem o pré-preenchimento
		t.Fatalf("leu %q", lido)
	}
	est, _ := c["estrutura"].(map[string]any)
	if est == nil || !strings.HasPrefix(est["hash"].(string), "sha256:v1:") || strings.Join(lista(c["pendentes"]), ",") != "p1" || c["preenchimentoIgnorado"] != true {
		t.Fatalf("candidatura = %v", c)
	}
	if len(dominio.ListaDe(c["leituras"])) != 1 {
		t.Fatalf("leituras = %v", c["leituras"])
	}
	// O log: só números, nunca o texto das perguntas nem a URL.
	if strings.Contains(saida.String(), marcaPergunta) || strings.Contains(saida.String(), "typeform") || !strings.Contains(saida.String(), "formulario: lido (2 perguntas, 1 pendente(s))") {
		t.Fatalf("log = %s", saida.String())
	}
}

func TestLerFormularioRecusasEBloqueios(t *testing.T) {
	f, id := filaComFormulario(t)
	leitura := dominio.LeituraFormulario{}
	var falha error
	f.api.lerFormulario = func(_ context.Context, endereco string) (dominio.LeituraFormulario, error) {
		l := leitura
		if l.URLFinal == "" {
			l.URLFinal = endereco
		}
		return l, falha
	}
	// Login aberto, ou envio em andamento: 409 sem ler.
	f.api.janela = func() (bool, bool) { return false, true }
	f.chamar("POST", "/candidaturas/"+id+"/ler-formulario", Objeto{}, 409)
	f.api.janela = func() (bool, bool) { return false, false }
	// Erro do navegador: 409, a candidatura fica preparada (a leitura conta no teto).
	falha = errors.New("rede")
	f.chamar("POST", "/candidaturas/"+id+"/ler-formulario", Objeto{}, 409)
	if c := f.chamar("GET", "/candidaturas/"+id, nil, 200); c["estado"] != "preparada" {
		t.Fatalf("depois do erro = %v", c["estado"])
	}
	falha = nil
	// Estrutura que o domínio recusa (tipo fora da lista): bloqueada/outro, a pessoa envia.
	leitura = dominio.LeituraFormulario{Perguntas: []dominio.PerguntaFormulario{{ID: "a", Texto: "x", Tipo: "script"}}}
	c := f.chamar("POST", "/candidaturas/"+id+"/ler-formulario", Objeto{}, 200)
	if c["estado"] != "bloqueada" || dominio.ObjetoDe(c["bloqueio"])["motivo"] != "outro" {
		t.Fatalf("estrutura ruim = %v", c)
	}
	// Já bloqueada: 409.
	f.chamar("POST", "/candidaturas/"+id+"/ler-formulario", Objeto{}, 409)

	// Várias etapas (decisão do PO): bloqueada/varias-etapas.
	f2, id2 := filaComFormulario(t)
	f2.api.lerFormulario = func(_ context.Context, endereco string) (dominio.LeituraFormulario, error) {
		return dominio.LeituraFormulario{Bloqueio: "varias-etapas"}, nil
	}
	c = f2.chamar("POST", "/candidaturas/"+id2+"/ler-formulario", Objeto{}, 200)
	if c["estado"] != "bloqueada" || dominio.ObjetoDe(c["bloqueio"])["motivo"] != "varias-etapas" {
		t.Fatalf("várias etapas = %v", c)
	}
	// Bloqueio fora da lista vindo do leitor: outro.
	f3, id3 := filaComFormulario(t)
	f3.api.lerFormulario = func(_ context.Context, endereco string) (dominio.LeituraFormulario, error) {
		return dominio.LeituraFormulario{Bloqueio: "texto da página"}, nil
	}
	c = f3.chamar("POST", "/candidaturas/"+id3+"/ler-formulario", Objeto{}, 200)
	if dominio.ObjetoDe(c["bloqueio"])["motivo"] != "outro" {
		t.Fatalf("bloqueio fora da lista = %v", c["bloqueio"])
	}
	// LinkedIn: não é formulário.
	f3.api.lerFormulario = func(context.Context, string) (dominio.LeituraFormulario, error) {
		t.Fatal("leu candidatura do LinkedIn")
		return dominio.LeituraFormulario{}, nil
	}
	li := f3.chamar("POST", "/candidaturas", Objeto{"vagaId": "li-1", "conteudo": conteudoTeste()}, 201)
	f3.chamar("POST", "/candidaturas/"+li["id"].(string)+"/ler-formulario", Objeto{}, 409)
}

// O teto de 40 páginas abertas no dia: a leitura 41 dá 429 sem abrir nada.
func TestLerFormularioNoTeto(t *testing.T) {
	f, id := filaComFormulario(t)
	abertas := 0
	f.api.lerFormulario = func(context.Context, string) (dominio.LeituraFormulario, error) {
		abertas++
		return dominio.LeituraFormulario{}, errors.New("falha de teste")
	}
	for i := 0; i < dominio.LimiteMaximo; i++ {
		f.chamar("POST", "/candidaturas/"+id+"/ler-formulario", Objeto{}, 409)
	}
	f.chamar("POST", "/candidaturas/"+id+"/ler-formulario", Objeto{}, 429)
	if abertas != dominio.LimiteMaximo {
		t.Fatalf("abriu %d páginas", abertas)
	}
}
