package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// "Preencher no site da empresa" com a janela falsa. Dados fictícios.

type janelaDePreencher struct {
	pedeCPF  int
	abertas  []string
	host     string
	conteudo Objeto
	perfil   Objeto
	cv       string
	pdf      []byte
}

func (j *janelaDePreencher) abrir(endereco string) error {
	j.abertas = append(j.abertas, endereco)
	return nil
}
func (j *janelaDePreencher) site(context.Context) (string, error) {
	if len(j.abertas) == 0 {
		return "", fmt.Errorf("sem janela")
	}
	return j.host, nil
}
func (j *janelaDePreencher) preencher(_ context.Context, dom string, conteudo, perfil dominio.Objeto, nome string, pdf []byte) (dominio.ResultadoPreenchimento, error) {
	if dom != j.host {
		return dominio.ResultadoPreenchimento{}, &dominio.ErroConflito{Msg: "mudou", Motivo: "site-mudou"}
	}
	j.conteudo, j.perfil, j.cv, j.pdf = conteudo, perfil, nome, pdf
	if conteudo["soCPF"] == true {
		return dominio.ResultadoPreenchimento{Preenchidos: 1}, nil
	}
	if conteudo["cpf"] != nil {
		return dominio.ResultadoPreenchimento{Preenchidos: 5, Anexou: true, Faltando: 1, ComCPF: true}, nil
	}
	return dominio.ResultadoPreenchimento{Preenchidos: 4, Anexou: true, Faltando: 2, PedeCPF: j.pedeCPF}, nil
}

func apiDePreencher(t *testing.T, j *janelaDePreencher) *API {
	t.Helper()
	ctx := context.Background()
	pasta := t.TempDir()
	arm, err := armazem.Abrir(ctx, filepath.Join(pasta, "dados.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	os.MkdirAll(filepath.Join(pasta, "curriculos"), 0o700)
	os.WriteFile(filepath.Join(pasta, "curriculos", "cv-pt.pdf"), []byte("%PDF-1.4\nCurrículo fictício\n%%EOF\n"), 0o600)
	o := Opcoes{Cofre: segredos.Memoria{}}
	if j != nil {
		o.AbrirParaPreencher, o.SiteParaPreencher, o.PreencherSite = j.abrir, j.site, j.preencher
	}
	a := Nova(arm, o)
	err = a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		e.Perfil = Objeto{"nome": "Fulana de Tal", "email": "fulana@example.com", "telefone": "+55 11 90000-0000",
			"curriculos": Objeto{"PT": "curriculos/cv-pt.pdf"}, "respostasPadrao": []any{Objeto{"pergunta": "Precisa de visto?", "resposta": "Não"}}}
		cx := a.ctx(e, "eu")
		for _, id := range []string{"linkedin-1", "linkedin-2"} {
			var err error
			e, _, err = dominio.AdicionarVaga(e, Objeto{"id": id, "empresa": "Acme", "titulo": "Engenheira " + id,
				"link": "https://www.linkedin.com/jobs/view/" + strings.TrimPrefix(id, "linkedin-") + "000000/", "plataforma": "linkedin"}, cx, false)
			if err != nil {
				return e, err
			}
		}
		// linkedin-1 reservada com "Aplicar na vaga"; linkedin-2 não.
		e, _, err := a.iniciarManualRadar(e, "linkedin-1", cx)
		return e, err
	})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestPreencherSiteDaEmpresa(t *testing.T) {
	j := &janelaDePreencher{host: "carreiras.acme.example"}
	a := apiDePreencher(t, j)

	// Só a pessoa; só vaga reservada; corpo estrito.
	if r := chamar(t, a, "POST", "/vagas/linkedin-1/abrir-para-preencher", Objeto{}, "claude"); r.Status != 403 || len(j.abertas) != 0 {
		t.Fatalf("assistente: %d", r.Status)
	}
	if r := chamar(t, a, "POST", "/vagas/linkedin-2/abrir-para-preencher", Objeto{}, "eu"); r.Status != 409 || !strings.Contains(fmt.Sprint(r.Corpo), "sem-reserva") {
		t.Fatalf("sem reserva: %d %v", r.Status, r.Corpo)
	}
	if r := chamar(t, a, "POST", "/vagas/linkedin-1/preencher", Objeto{"url": "x"}, "eu"); r.Status != 400 {
		t.Fatalf("campo a mais: %d", r.Status)
	}

	// Abre a vaga no endereço reservado.
	if r := chamar(t, a, "POST", "/vagas/linkedin-1/abrir-para-preencher", Objeto{}, "eu"); r.Status != 200 || len(j.abertas) != 1 || j.abertas[0] != "https://www.linkedin.com/jobs/view/1000000/" {
		t.Fatalf("abrir: %d %v %v", r.Status, r.Corpo, j.abertas)
	}
	// Primeiro o domínio, para a pessoa conferir; nada é preenchido.
	r := chamar(t, a, "POST", "/vagas/linkedin-1/preencher", Objeto{}, "eu")
	if r.Status != 200 || r.Corpo.(Objeto)["dominio"] != "carreiras.acme.example" || j.conteudo != nil {
		t.Fatalf("domínio: %d %v", r.Status, r.Corpo)
	}
	// Domínio malformado.
	if r := chamar(t, a, "POST", "/vagas/linkedin-1/preencher", Objeto{"dominio": "https://x/y"}, "eu"); r.Status != 400 {
		t.Fatalf("domínio malformado: %d", r.Status)
	}
	// Outro domínio: a janela recusa.
	if r := chamar(t, a, "POST", "/vagas/linkedin-1/preencher", Objeto{"dominio": "golpe.example"}, "eu"); r.Status != 409 || j.conteudo != nil {
		t.Fatalf("domínio trocado: %d %v", r.Status, r.Corpo)
	}
	// Conferido: preenche com o contato do perfil, as respostas padrão e o currículo conferido.
	r = chamar(t, a, "POST", "/vagas/linkedin-1/preencher", Objeto{"dominio": "Carreiras.Acme.example"}, "eu")
	if r.Status != 200 {
		t.Fatalf("preencher: %d %v", r.Status, r.Corpo)
	}
	if res := r.Corpo.(Objeto); res["preenchidos"] != 4 || res["anexou"] != true || res["faltando"] != 2 || res["pedeCPF"] != nil {
		t.Fatalf("resultado: %+v", res)
	}
	contato := dominio.ObjetoDe(j.conteudo["contato"])
	if contato["email"] != "fulana@example.com" || len(dominio.ListaDe(j.conteudo["respostas"])) != 1 || j.cv != "cv-pt.pdf" || !strings.HasPrefix(string(j.pdf), "%PDF") {
		t.Fatalf("pacote: %v cv %q", j.conteudo, j.cv)
	}
	if j.perfil["nome"] != "Fulana de Tal" {
		t.Fatalf("perfil: %v", j.perfil)
	}

	// O currículo mudou depois da reserva: nada é preenchido.
	j.conteudo = nil
	os.WriteFile(filepath.Join(a.pastaCurriculos(), "cv-pt.pdf"), []byte("%PDF-1.4\noutro\n%%EOF\n"), 0o600)
	if r := chamar(t, a, "POST", "/vagas/linkedin-1/preencher", Objeto{"dominio": "carreiras.acme.example"}, "eu"); r.Status != 409 || !strings.Contains(fmt.Sprint(r.Corpo), "curriculo-mudou") || j.conteudo != nil {
		t.Fatalf("currículo mudou: %d %v", r.Status, r.Corpo)
	}
}

func TestPreencherSemEnvio(t *testing.T) {
	a := apiDePreencher(t, nil)
	for _, rota := range []string{"/vagas/linkedin-1/abrir-para-preencher", "/vagas/linkedin-1/preencher"} {
		if r := chamar(t, a, "POST", rota, Objeto{}, "eu"); r.Status != 404 {
			t.Fatalf("%s sem o envio: %d", rota, r.Status)
		}
	}
}
