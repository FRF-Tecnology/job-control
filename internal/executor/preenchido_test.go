package executor

import (
	"context"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// jc-a2y.6.5.1 (aprovado pelo PO antes da .6.5): (a) sem o botão de enviar, nada foi clicado: bloqueada, não incerta;
// (b) obrigatório que a página já trouxe preenchido conta como respondido, sem o app alterar, e o log só conta.

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

func TestObrigatorioPreenchidoPelaPagina(t *testing.T) {
	// Código do país: obrigatório, fora do pacote, já marcado pela página.
	p := novaPagina()
	p.campos = append(p.campos, Campo{ID: "pais", Tipo: CampoOutro, Rotulo: "Código do país", Obrigatorio: true, Preenchido: true})
	s := novoCenario(t, p)
	d, err := s.x.Enviar(context.Background(), s.aprovada("linkedin", linkLI))
	if err != nil {
		t.Fatal(err)
	}
	if d.Estado != dominio.CandEnviada {
		t.Fatalf("desfecho = %+v; queria enviada", d)
	}
	if _, mexeu := p.preenchidos["pais"]; mexeu {
		t.Error("o app alterou o campo que a página já tinha preenchido")
	}
	log := strings.Join(s.logs, "\n")
	if !strings.Contains(log, "1 campo(s) obrigatório(s) preenchido(s) pela página") || strings.Contains(log, "Código do país") {
		t.Errorf("log = %q", log)
	}

	// Continuam bloqueando: não preenchido, texto longo, rótulo sensível.
	for nome, c := range map[string]Campo{
		"vazio":       {ID: "x", Tipo: CampoOutro, Rotulo: "Código do país", Obrigatorio: true},
		"texto longo": {ID: "x", Tipo: CampoPergunta, Rotulo: "Carta de apresentação", Obrigatorio: true, Preenchido: true, Livre: true},
		"cpf":         {ID: "x", Tipo: CampoOutro, Rotulo: "CPF", Obrigatorio: true, Preenchido: true},
		"senha":       {ID: "x", Tipo: "senha", Rotulo: "Senha", Obrigatorio: true, Preenchido: true},
	} {
		p := novaPagina()
		p.campos = append(p.campos, c)
		s := novoCenario(t, p)
		d, err := s.x.Enviar(context.Background(), s.aprovada("linkedin", linkLI))
		if err != nil {
			t.Fatal(err)
		}
		if d.Estado != dominio.CandBloqueada || p.enviou != 0 {
			t.Errorf("%s: desfecho = %+v, envios %d; queria bloqueada sem envio", nome, d, p.enviou)
		}
	}
}

// Select de contato já marcado pela página, sem a opção do pacote (código do país classificado como telefone, e-mail
// da conta): fica o da página. Pergunta com resposta aprovada fora das opções continua falhando, sem envio.
func TestSelectDeContatoJaMarcado(t *testing.T) {
	p := novaPagina()
	p.campos = append(p.campos, Campo{ID: "ddi", Tipo: CampoTelefone, Rotulo: "Código do país do telefone", Obrigatorio: true, Preenchido: true})
	p.errPorCampo = map[string]error{"ddi": ErrSemOpcao, "email": ErrSemOpcao}
	p.campos[1].Preenchido = true // e-mail: select com o e-mail da conta já marcado
	s := novoCenario(t, p)
	d, err := s.x.Enviar(context.Background(), s.aprovada("linkedin", linkLI))
	if err != nil {
		t.Fatal(err)
	}
	if d.Estado != dominio.CandEnviada || !strings.Contains(strings.Join(s.logs, "\n"), "2 campo(s) obrigatório(s) preenchido(s) pela página") {
		t.Fatalf("contato já marcado: desfecho = %+v, logs %v", d, s.logs)
	}

	for nome, c := range map[string]Campo{
		"contato não marcado": {ID: "ddi", Tipo: CampoTelefone, Rotulo: "Código do país do telefone", Obrigatorio: true},
		"pergunta":            {ID: "ddi", Tipo: CampoPergunta, Rotulo: "Anos de experiência com Go", Obrigatorio: true, Preenchido: true},
	} {
		p := novaPagina()
		p.campos = append(p.campos, c)
		p.errPorCampo = map[string]error{"ddi": ErrSemOpcao}
		s := novoCenario(t, p)
		d, err := s.x.Enviar(context.Background(), s.aprovada("linkedin", linkLI))
		if err != nil {
			t.Fatal(err)
		}
		if d.Estado != dominio.CandFalhou || p.enviou != 0 {
			t.Errorf("%s: desfecho = %+v, envios %d; queria falhou sem envio", nome, d, p.enviou)
		}
	}
}
