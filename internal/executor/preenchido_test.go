package executor

import (
	"context"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// jc-a2y.6.5.1 (aprovado pelo PO; regras do CyberSec): (a) sem o botão de enviar, nada foi clicado: bloqueada, não
// incerta; (b) a página só pode trazer preenchidos nome, e-mail, telefone e código do país, e só se iguais ao pacote
// (o app confere sem alterar). Qualquer outra coisa preenchida pela página que não está no pacote bloqueia.

func TestSemBotaoDeEnviarBloqueia(t *testing.T) {
	p := novaPagina()
	p.errEnviar = ErrSemBotaoEnviar
	s := novoCenario(t, p)
	d, err := s.x.Enviar(context.Background(), s.aprovada("linkedin", linkLI))
	if err != nil {
		t.Fatal(err)
	}
	if d.Estado != dominio.CandBloqueada || d.Motivo != "outro" {
		t.Fatalf("sem botão de enviar = %+v; queria bloqueada/outro", d)
	}
}

func TestContatoPreenchidoIgualAoPacote(t *testing.T) {
	p := novaPagina()
	p.campos[1].Preenchido = true // e-mail da conta
	p.campos = append(p.campos, Campo{ID: "ddi", Tipo: CampoDDI, Rotulo: "Código do país", Obrigatorio: true, Preenchido: true})
	p.iguais = map[string]bool{"email": true, "ddi": true}
	s := novoCenario(t, p)
	d, err := s.x.Enviar(context.Background(), s.aprovada("linkedin", linkLI))
	if err != nil {
		t.Fatal(err)
	}
	if d.Estado != dominio.CandEnviada {
		t.Fatalf("desfecho = %+v; queria enviada", d)
	}
	for _, id := range []string{"email", "ddi"} {
		if _, mexeu := p.preenchidos[id]; mexeu {
			t.Errorf("o app alterou %s, que a página já trouxe igual", id)
		}
	}
	if strings.Join(p.conferidos, ",") != "email,ddi" {
		t.Errorf("conferidos = %v", p.conferidos)
	}
	log := strings.Join(s.logs, "\n")
	if !strings.Contains(log, "campo email preenchido pela página, igual ao pacote") || !strings.Contains(log, "campo ddi preenchido pela página") || strings.Contains(log, "Código do país") || strings.Contains(log, "pessoa@") {
		t.Errorf("log = %q", log)
	}
}

func TestPreenchidoPelaPaginaQueBloqueia(t *testing.T) {
	casos := map[string]struct {
		campo  Campo
		iguais map[string]bool
		motivo string
	}{
		"e-mail diferente do pacote":   {Campo{ID: "email2", Tipo: CampoEmail, Rotulo: "E-mail", Obrigatorio: true, Preenchido: true}, nil, "pergunta-nova"},
		"triagem pré-marcada":          {Campo{ID: "mudanca", Tipo: CampoPergunta, Rotulo: "Disponível para mudança?", Preenchido: true}, nil, "pergunta-nova"},
		"triagem obrigatória marcada":  {Campo{ID: "anos2", Tipo: CampoPergunta, Rotulo: "Anos de experiência com Rust", Obrigatorio: true, Preenchido: true}, nil, "pergunta-nova"},
		"termos pré-marcados":          {Campo{ID: "termos", Tipo: "termos", Rotulo: "Li e aceito os termos", Preenchido: true}, nil, "termos"},
		"texto livre preenchido":       {Campo{ID: "porque", Tipo: CampoOutro, Rotulo: "Por que você quer trabalhar aqui?", Preenchido: true}, nil, "pergunta-nova"},
		"currículo anterior escolhido": {Campo{ID: "cvantigo", Tipo: CampoOutro, Rotulo: "Escolha um currículo", Obrigatorio: true, Preenchido: true}, nil, "pergunta-nova"},
		"cpf preenchido":               {Campo{ID: "cpf", Tipo: CampoOutro, Rotulo: "CPF", Obrigatorio: true, Preenchido: true}, nil, "cpf"},
	}
	for nome, c := range casos {
		p := novaPagina()
		p.campos = append(p.campos, c.campo)
		p.iguais = c.iguais
		s := novoCenario(t, p)
		d, err := s.x.Enviar(context.Background(), s.aprovada("linkedin", linkLI))
		if err != nil {
			t.Fatal(err)
		}
		if d.Estado != dominio.CandBloqueada || p.enviou != 0 {
			t.Errorf("%s: desfecho = %+v, envios %d; queria bloqueada sem envio", nome, d, p.enviou)
			continue
		}
		if d.Motivo != c.motivo {
			t.Errorf("%s: motivo = %q; queria %q", nome, d.Motivo, c.motivo)
		}
		if _, mexeu := p.preenchidos[c.campo.ID]; mexeu {
			t.Errorf("%s: o app alterou o campo", nome)
		}
	}
}

// Campo que está no pacote e a página já trouxe preenchido (resposta de candidatura anterior): vai a resposta
// aprovada, nunca a da página.
func TestPreenchidoNoPacoteVaiOAprovado(t *testing.T) {
	p := novaPagina()
	p.campos[3].Preenchido = true // "anos de GO?", respondida no pacote
	s := novoCenario(t, p)
	d, err := s.x.Enviar(context.Background(), s.aprovada("linkedin", linkLI))
	if err != nil {
		t.Fatal(err)
	}
	if d.Estado != dominio.CandEnviada || p.preenchidos["anos"] != marca || len(p.conferidos) != 0 {
		t.Fatalf("desfecho = %+v, anos = %q, conferidos %v", d, p.preenchidos["anos"], p.conferidos)
	}
}

// Valor aprovado fora das opções do campo: bloqueada (a pessoa resolve), nunca outro valor no lugar.
func TestValorForaDasOpcoesBloqueia(t *testing.T) {
	p := novaPagina()
	p.errPorCampo = map[string]error{"anos": ErrSemOpcao}
	s := novoCenario(t, p)
	d, err := s.x.Enviar(context.Background(), s.aprovada("linkedin", linkLI))
	if err != nil {
		t.Fatal(err)
	}
	if d.Estado != dominio.CandBloqueada || d.Motivo != "pergunta-nova" || p.enviou != 0 {
		t.Fatalf("fora das opções = %+v, envios %d", d, p.enviou)
	}
}

func TestDDIDoTelefone(t *testing.T) {
	for tel, quer := range map[string]string{
		"11900000000": "+55", "(11) 90000-0000": "+55", "+55 11 90000-0000": "+55", "+351 912 345 678": "+351",
		"+5511900000000": "", "": "", "+abc 1": "",
	} {
		if got := ddiDoTelefone(tel); got != quer {
			t.Errorf("ddiDoTelefone(%q) = %q; queria %q", tel, got, quer)
		}
	}
}
