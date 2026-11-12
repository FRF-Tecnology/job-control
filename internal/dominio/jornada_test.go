package dominio

import (
	"testing"
	"time"
)

func TestJornadaDevolverCancelaPacoteSemEnviar(t *testing.T) {
	agora := time.Now()
	e := comVaga(t, Estado{}, "v", "outra", "https://vaga.example/v", false)
	e, v, err := IniciarManualRadar(e, "v", conteudoOK(), agora, eu)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = DevolverAoRadar(e, "v", agora, claude); err == nil {
		t.Fatal("assistente devolveu vaga")
	}
	e, v, err = DevolverAoRadar(e, "v", agora, eu)
	if err != nil || v["etapa"] != "descoberta" || v["aplicacaoManual"] != nil || e.Candidaturas[0]["estado"] != CandCancelada || FoiEnviada(v) {
		t.Fatalf("devolução %v %v", v, err)
	}
	n := len(e.Eventos)
	e, _, err = DevolverAoRadar(e, "v", agora, eu)
	if err != nil || len(e.Eventos) != n {
		t.Fatal("repetição alterou o histórico")
	}
}

func TestJornadaQuadroPersisteEtapaNotaHistorico(t *testing.T) {
	e := comVaga(t, Estado{}, "v", "outra", "https://vaga.example/v", false)
	if _, _, err := AtualizarQuadro(e, "v", "Enviada", "", "Conquistada", "", eu); err == nil {
		t.Fatal("incluiu vaga nunca enviada")
	}
	e, _, err := MoverVaga(e, "v", "enviada", nil, eu)
	if err != nil {
		t.Fatal(err)
	}
	n := len(e.Eventos)
	e, v, err := AtualizarQuadro(e, "v", "Enviada", "", "Conquistada", "Contrato assinado", eu)
	if err != nil || v["etapa"] != "encerrada" || v["motivo"] != "Proposta aceita" || v["notas"] != "Contrato assinado" || len(e.Eventos) != n+2 {
		t.Fatalf("resultado %v %v", v, err)
	}
	if _, _, err = AtualizarQuadro(e, "v", "Enviada", "", "Entrevista", "", eu); err == nil {
		t.Fatal("sobrescreveu mudança concorrente")
	}
	e, _, err = AtualizarQuadro(e, "v", "Enviada", "", "Conquistada", "Contrato assinado", eu)
	if err != nil || len(e.Eventos) != n+2 {
		t.Fatal("repetição duplicou histórico")
	}
}
