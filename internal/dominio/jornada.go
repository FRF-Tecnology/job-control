package dominio

import "time"

// A devolução tira a autorização e a vaga da lista numa só gravação.
func DevolverAoRadar(e Estado, id string, agora time.Time, ctx Ctx) (Estado, Objeto, error) {
	if err := soPessoa(ctx, "devolver vaga ao Radar"); err != nil {
		return e, nil, err
	}
	v := vagaDe(e, id)
	if v == nil {
		return e, nil, &ErroNaoEncontrado{"vaga não encontrada"}
	}
	if v["etapa"] == "descoberta" && v["devolvidaAoRadar"] == true {
		return e, v, nil
	}
	if v["etapa"] != "aplicar" || FoiEnviada(v) {
		return e, nil, &ErroConflito{Msg: "a vaga saiu da lista; atualize a tela"}
	}
	novo := e
	for _, c := range e.Candidaturas {
		if c["vagaId"] != id || c["estado"] == CandCancelada {
			continue
		}
		var err error
		novo, _, err = CancelarCandidatura(novo, texto(c["id"]), agora, ctx)
		if err != nil {
			return e, nil, err
		}
	}
	novo, v, err := MoverVaga(novo, id, "descoberta", Objeto{"texto": "Devolvida ao Radar"}, ctx)
	if err != nil {
		return e, nil, err
	}
	v["devolvidaAoRadar"] = true
	return novo, v, nil
}

func colunaQuadro(v Objeto) string {
	switch v["etapa"] {
	case "enviada":
		return "Enviada"
	case "em-processo":
		if v["subEtapa"] == "proposta" {
			return "Proposta"
		}
		return "Entrevista"
	case "oferta":
		return "Proposta"
	case "encerrada":
		if v["motivo"] == "Proposta aceita" {
			return "Conquistada"
		}
	}
	return ""
}

// Etapa, anotação e eventos mudam juntos. O histórico existente e as subetapas de
// processos antigos são preservados quando a pessoa edita só a anotação.
func AtualizarQuadro(e Estado, id, etapaAntes, notasAntes, etapa, notas string, ctx Ctx) (Estado, Objeto, error) {
	if err := soPessoa(ctx, "atualizar candidatura no quadro"); err != nil {
		return e, nil, err
	}
	v := vagaDe(e, id)
	if v == nil {
		return e, nil, &ErroNaoEncontrado{"vaga não encontrada"}
	}
	anterior := colunaQuadro(v)
	if anterior == "" {
		return e, nil, &ErroConflito{Msg: "esta candidatura não está no quadro"}
	}
	if anterior == etapa && texto(v["notas"]) == notas {
		return e, v, nil
	}
	if anterior != etapaAntes || texto(v["notas"]) != notasAntes {
		return e, nil, &ErroConflito{Msg: "a candidatura mudou; reabra o cartão para revisar"}
	}
	novo := e
	if anterior != etapa {
		destino := ""
		extras := Objeto{"texto": "Etapa alterada de " + anterior + " para " + etapa + "."}
		switch etapa {
		case "Enviada":
			destino = "enviada"
		case "Entrevista":
			destino = "em-processo"
			extras["subEtapa"] = nil
		case "Proposta":
			destino = "oferta"
			extras["subEtapa"] = "proposta"
		case "Conquistada":
			destino = "encerrada"
			extras["motivo"] = "Proposta aceita"
		default:
			return e, nil, validacao("etapa do quadro desconhecida")
		}
		var err error
		novo, v, err = MoverVaga(novo, id, destino, extras, ctx)
		if err != nil {
			return e, nil, err
		}
	}
	if notas != notasAntes {
		var err error
		novo, v, err = AtualizarVaga(novo, id, Objeto{"notas": notas, "notasAntes": notasAntes}, ctx)
		if err != nil {
			return e, nil, err
		}
		linha := "Anotação removida."
		if notas != "" {
			linha = notas
		}
		novo, _, err = RegistrarEvento(novo, id, "nota", linha, ctx)
		if err != nil {
			return e, nil, err
		}
	}
	return novo, v, nil
}
