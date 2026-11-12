package dominio

import (
	"testing"
	"time"
)

func TestListaReservaAprovadaSemDisputarComEnvio(t *testing.T) {
	agora := time.Now()
	e := comVaga(t, Estado{}, "manual", "linkedin", linkLI("2"), true)
	e, c := aprovada(t, e, "manual", agora, nil)
	emEnvio, _, err := IniciarEnvio(e, c.ID, c.Versao, agora, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := IniciarManualRadar(emEnvio, "manual", nil, agora, eu); err == nil {
		t.Fatal("tomou um envio em curso")
	}
	reservado, v, err := IniciarManualRadar(e, "manual", nil, agora, eu)
	if err != nil {
		t.Fatalf("não reservou a aprovada: %v", err)
	}
	lido := estadoDe(t, reservado, c.ID)
	if v["aplicacaoManual"] != true || lido.Aprovacao != nil || lido.Estado != CandBloqueada || lido.Impressao != c.Impressao {
		t.Fatalf("pacote após reserva %v", lido)
	}
	if _, _, err := IniciarEnvio(reservado, c.ID, c.Versao, agora, nil); err == nil {
		t.Fatal("executor tomou o pacote reservado")
	}
	final, _, err := ConfirmarManualRadar(reservado, "manual", agora, eu)
	if err != nil || len(final.Candidaturas) != 1 || estadoDe(t, final, c.ID).Estado != CandEnviada {
		t.Fatalf("confirmação %v", err)
	}
}

func TestRadarManualExigePessoaEConfirmacao(t *testing.T) {
	e := comVaga(t, Estado{}, "manual", "outra", "https://vagas.example/teste", false)
	agora := time.Now()
	if _, _, err := IniciarManualRadar(e, "manual", conteudoOK(), agora, claude); err == nil {
		t.Fatal("assistente iniciou candidatura")
	}
	if _, _, err := ConfirmarManualRadar(e, "manual", agora, eu); err == nil {
		t.Fatal("confirmou sem iniciar")
	}
	e, v, err := IniciarManualRadar(e, "manual", conteudoOK(), agora, eu)
	if err != nil || v["aplicacaoManual"] != true || FoiEnviada(v) || len(e.Candidaturas) != 1 {
		t.Fatalf("intenção: %v %v", v, err)
	}
	n := len(e.Eventos)
	removida, _, err := MoverVaga(e, "manual", "encerrada", Objeto{"motivo": "Reprovada"}, eu)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ConfirmarManualRadar(removida, "manual", agora, eu); err == nil {
		t.Fatal("ressuscitou uma vaga encerrada em outra janela")
	}
	e, _, err = IniciarManualRadar(e, "manual", nil, agora, eu)
	if err != nil || len(e.Eventos) != n || len(e.Candidaturas) != 1 {
		t.Fatalf("reabertura: %v", err)
	}
	if _, _, err := ConfirmarManualRadar(e, "manual", agora, claude); err == nil {
		t.Fatal("assistente confirmou")
	}
	e, v, err = ConfirmarManualRadar(e, "manual", agora, eu)
	if err != nil || !FoiEnviada(v) || len(e.Eventos) != n+1 || e.Candidaturas[0]["estado"] != CandEnviada {
		t.Fatalf("confirmação: %v %v", v, err)
	}
	e, _, err = ConfirmarManualRadar(e, "manual", agora, eu)
	if err != nil || len(e.Eventos) != n+1 {
		t.Fatalf("confirmação duplicada: %v", err)
	}
}
