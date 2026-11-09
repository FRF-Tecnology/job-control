package executor

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Páginas fictícias (testdata/paginas): retratos que imitam LinkedIn, Gupy e InHire, com empresas e vagas
// inventadas. Cada uma diz o desfecho esperado do envio.
type paginaFicticia struct {
	Descricao  string    `json:"descricao"`
	Plataforma string    `json:"plataforma"`
	URL        string    `json:"url"`
	Esperado   Desfecho  `json:"esperado"`
	Etapas     []Retrato `json:"etapas"`
}

// paginaDeRetratos é a página do navegador feita de retratos: o detector e o classificador de verdade decidem.
type paginaDeRetratos struct {
	etapas      []Retrato
	i           int
	preenchidos map[string]string
	anexos      int
	enviou      int
}

func (p *paginaDeRetratos) Abrir(ctx context.Context, u string) (string, error) {
	return p.etapas[0].URL, nil
}
func (p *paginaDeRetratos) URLAtual(ctx context.Context) (string, error) {
	return p.etapas[p.i].URL, nil
}
func (p *paginaDeRetratos) Bloqueio(ctx context.Context) (string, error) {
	return DetectarBloqueio(p.etapas[p.i]), nil
}
func (p *paginaDeRetratos) Campos(ctx context.Context) ([]Campo, error) {
	return ClassificarCampos(p.etapas[p.i]), nil
}
func (p *paginaDeRetratos) Preencher(ctx context.Context, c Campo, v string) error {
	p.preenchidos[c.ID] = v
	return nil
}
func (p *paginaDeRetratos) Igual(context.Context, Campo, []string) (bool, error) { return false, nil }
func (p *paginaDeRetratos) Desmarcar(context.Context, Campo) error               { return nil }
func (p *paginaDeRetratos) Anexar(ctx context.Context, c Campo, caminho string) error {
	p.anexos++
	return nil
}
func (p *paginaDeRetratos) Proxima(ctx context.Context) (bool, error) {
	if p.i+1 < len(p.etapas) {
		p.i++
		return true, nil
	}
	return false, nil
}
func (p *paginaDeRetratos) Enviar(ctx context.Context) error              { p.enviou++; return nil }
func (p *paginaDeRetratos) Confirmacao(ctx context.Context) (bool, error) { return true, nil }
func (p *paginaDeRetratos) Fechar() error                                 { return nil }

type navegadorDeRetratos struct{ pag *paginaDeRetratos }

func (n navegadorDeRetratos) NovaPagina(ctx context.Context) (Pagina, error) { return n.pag, nil }

func lerPaginas(t *testing.T) []paginaFicticia {
	t.Helper()
	arquivos, err := filepath.Glob("testdata/paginas/*.json")
	if err != nil || len(arquivos) < 20 {
		t.Fatalf("páginas fictícias: %d (%v)", len(arquivos), err)
	}
	var saida []paginaFicticia
	for _, a := range arquivos {
		b, err := os.ReadFile(a)
		if err != nil {
			t.Fatal(err)
		}
		var p paginaFicticia
		if err := json.Unmarshal(b, &p); err != nil || len(p.Etapas) == 0 {
			t.Fatalf("%s: %v", a, err)
		}
		saida = append(saida, p)
	}
	return saida
}

// Cada página fictícia, do começo ao fim pelo executor: o desfecho bate, e nada é enviado quando bloqueia.
// Juntas, as páginas cobrem os 14 motivos.
func TestPaginasFicticias(t *testing.T) {
	cobertos := map[string]bool{}
	for _, pf := range lerPaginas(t) {
		t.Run(pf.Descricao, func(t *testing.T) {
			pag := &paginaDeRetratos{etapas: pf.Etapas, preenchidos: map[string]string{}}
			s := novoCenario(t)
			s.x.o.Navegador = navegadorDeRetratos{pag}
			d := s.enviar(s.aprovada(pf.Plataforma, pf.URL))
			if d.Estado != pf.Esperado.Estado || d.Motivo != pf.Esperado.Motivo {
				t.Fatalf("desfecho = %s/%s; queria %s/%s", d.Estado, d.Motivo, pf.Esperado.Estado, pf.Esperado.Motivo)
			}
			if d.Estado == dominio.CandBloqueada && pag.enviou != 0 {
				t.Fatal("clicou em enviar numa página que bloqueia")
			}
			if d.Estado == dominio.CandEnviada && (pag.enviou != 1 || pag.anexos == 0) {
				t.Fatalf("enviada sem clique ou sem currículo (cliques %d, anexos %d)", pag.enviou, pag.anexos)
			}
			for id, v := range pag.preenchidos {
				if strings.Contains(id, "cpf") || id == "doc" || id == "ag" || id == "cc" || id == "pw" || id == "raca" {
					t.Fatalf("preencheu campo sensível %s = %q", id, v)
				}
			}
			s.logSemConteudo()
			cobertos[d.Motivo] = true
		})
	}
	// pergunta-nova, site-externo e outro também saem das páginas (M1, M5 e etapas demais).
	var faltam []string
	for m := range dominio.PendenciaDoMotivo {
		if !cobertos[m] {
			faltam = append(faltam, m)
		}
	}
	sort.Strings(faltam)
	if len(faltam) > 0 {
		t.Fatalf("motivos sem página fictícia: %v", faltam)
	}
}

// Página limpa: o texto da descrição ("entre em contato", "time de dados") não bloqueia.
func TestDetectorSemFalsoPositivo(t *testing.T) {
	for _, pf := range lerPaginas(t) {
		if pf.Esperado.Estado != dominio.CandEnviada {
			continue
		}
		for i, r := range pf.Etapas {
			if m := DetectarBloqueio(r); m != "" {
				t.Errorf("%s etapa %d: bloqueou por %s", pf.Descricao, i, m)
			}
		}
	}
}

func TestClassificarCampos(t *testing.T) {
	casos := []struct {
		c    CampoDaPagina
		tipo string
	}{
		{CampoDaPagina{Tipo: "text", Rotulo: "Nome completo"}, CampoNome},
		{CampoDaPagina{Tipo: "text", Autocomplete: "name", Rotulo: "Como devemos te chamar"}, CampoNome},
		{CampoDaPagina{Tipo: "email", Rotulo: "Seu contato"}, CampoEmail},
		{CampoDaPagina{Tipo: "text", Rotulo: "WhatsApp"}, CampoTelefone},
		{CampoDaPagina{Tipo: "text", Rotulo: "Cidade"}, CampoCidade},
		{CampoDaPagina{Tipo: "number", Rotulo: "Pretensão salarial"}, CampoPretensao},
		{CampoDaPagina{Tipo: "textarea", Rotulo: "Carta de apresentação"}, CampoMensagem},
		{CampoDaPagina{Tipo: "file", Rotulo: "Resume / CV"}, CampoCurriculo},
		{CampoDaPagina{Tipo: "file", Rotulo: "Portfólio"}, "documento"},
		{CampoDaPagina{Tipo: "file", Rotulo: "Comprovante de residência"}, "documento"},
		{CampoDaPagina{Tipo: "password", Rotulo: "Código de acesso"}, "senha"},
		// Rótulo sensível vence o tipo do HTML.
		{CampoDaPagina{Tipo: "text", Rotulo: "CPF"}, "cpf"},
		{CampoDaPagina{Tipo: "email", Rotulo: "Senha"}, "senha"},
		{CampoDaPagina{Tipo: "file", Rotulo: "Anexe seu RG"}, "rg"},
		{CampoDaPagina{Tipo: "text", Rotulo: "Documento", Placeholder: "000.000.000-00 (CPF)"}, "cpf"},
		{CampoDaPagina{Tipo: "select", Rotulo: "Banco", Nome: "agencia"}, "dado-bancario"},
		{CampoDaPagina{Tipo: "radio", Rotulo: "Gênero"}, "dado-sensivel"},
		{CampoDaPagina{Tipo: "checkbox", Rotulo: "Aceito os termos"}, "termos"},
		{CampoDaPagina{Tipo: "checkbox", Rotulo: "Seguir a Empresa Teste"}, CampoSeguir}, // jc-a2y.6.5.2: o app só desmarca
		{CampoDaPagina{Tipo: "checkbox", Rotulo: "Quero receber a newsletter"}, CampoOutro},
		{CampoDaPagina{Tipo: "select", Rotulo: "Nível de inglês"}, CampoPergunta},
	}
	for _, k := range casos {
		if got := ClassificarCampos(Retrato{Campos: []CampoDaPagina{k.c}})[0].Tipo; got != k.tipo {
			t.Errorf("%+v = %s; queria %s", k.c, got, k.tipo)
		}
	}
}

// Formulário em etapas: cada etapa é planejada, conferida e só então avança; o endereço é conferido de novo
// logo antes do clique.
func TestEtapasEEnderecoAntesDoClique(t *testing.T) {
	// Etapa 2 com pergunta obrigatória fora do pacote: a etapa 1 foi preenchida, a 2 não, e nada foi enviado.
	p := novaPagina()
	p.etapas = [][]Campo{
		{{ID: "nome", Tipo: CampoNome, Obrigatorio: true}},
		{{ID: "k8s", Tipo: CampoPergunta, Rotulo: "Anos de Kubernetes?", Obrigatorio: true}},
	}
	s := novoCenario(t, p)
	if d := s.enviar(s.aprovada("linkedin", linkLI)); d.Motivo != "pergunta-nova" || p.enviou != 0 || p.preenchidos["nome"] == "" || p.etapa != 1 {
		t.Fatalf("etapa 2 = %+v, enviou %d, preenchidos %v, etapa %d", d, p.enviou, p.preenchidos, p.etapa)
	}

	// A página sai do site entre o fim do formulário e o clique.
	p = novaPagina()
	p.urls = []string{linkLI + "/apply", "https://ats.exemplo.io/fim"}
	s = novoCenario(t, p)
	if d := s.enviar(s.aprovada("linkedin", linkLI)); d.Motivo != "site-externo" || p.enviou != 0 {
		t.Fatalf("saiu antes do clique = %+v, enviou %d", d, p.enviou)
	}

	// Outra vaga na mesma plataforma, na segunda etapa.
	p = novaPagina()
	p.etapas = [][]Campo{{{ID: "nome", Tipo: CampoNome}}, {{ID: "email", Tipo: CampoEmail}}}
	p.urls = []string{linkLI + "/apply", "https://www.linkedin.com/jobs/view/9000000002/apply"}
	s = novoCenario(t, p)
	if d := s.enviar(s.aprovada("linkedin", linkLI)); d.Motivo != "site-externo" || p.preenchidos["email"] != "" {
		t.Fatalf("outra vaga na etapa 2 = %+v, preenchidos %v", d, p.preenchidos)
	}

	// Rótulo sensível vence o tipo dado pelo adaptador.
	for rotulo, motivo := range map[string]string{"Senha do portal": "senha", "Seu CPF": "cpf", "Agência e conta": "dado-bancario"} {
		p = novaPagina()
		p.campos = append(p.campos, Campo{ID: "x", Tipo: CampoPergunta, Rotulo: rotulo})
		s = novoCenario(t, p)
		if d := s.enviar(s.aprovada("linkedin", linkLI)); d.Motivo != motivo || len(p.preenchidos) > 0 {
			t.Errorf("%q = %+v, preenchidos %v", rotulo, d, p.preenchidos)
		}
	}
	p = novaPagina()
	p.campos = []Campo{{ID: "cv", Tipo: CampoCurriculo, Rotulo: "Anexe seu RG", Obrigatorio: true}}
	s = novoCenario(t, p)
	if d := s.enviar(s.aprovada("linkedin", linkLI)); d.Motivo != "rg" || len(p.anexos) > 0 {
		t.Fatalf("anexo com rótulo de RG = %+v, anexos %d", d, len(p.anexos))
	}

	// Aceite opcional fica sem marcar e a candidatura segue.
	p = novaPagina()
	p.campos = append(p.campos, Campo{ID: "news", Tipo: "termos", Rotulo: "Aceito receber novidades"})
	s = novoCenario(t, p)
	if d := s.enviar(s.aprovada("linkedin", linkLI)); d.Estado != dominio.CandEnviada || p.preenchidos["news"] != "" {
		t.Fatalf("termos opcional = %+v", d)
	}
}
