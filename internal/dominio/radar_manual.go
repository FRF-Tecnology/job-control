package dominio

import "time"

// A candidatura manual fica reservada para a pessoa. Nenhum estado aprovado
// é gravado, nem por um instante: o executor não pode iniciar um envio.
func IniciarManualRadar(e Estado, id string, pacote Objeto, agora time.Time, ctx Ctx) (Estado, Objeto, error) {
	if err := soPessoa(ctx, "iniciar candidatura manual"); err != nil {
		return e, nil, err
	}
	v := vagaDe(e, id)
	if v == nil {
		return e, nil, &ErroNaoEncontrado{"vaga não encontrada"}
	}
	if FoiEnviada(v) || (v["etapa"] != "descoberta" && v["etapa"] != "aplicar") {
		return e, nil, &ErroConflito{Msg: "esta vaga não está disponível no Radar"}
	}
	if !URLDaPlataforma(PlataformaDaVaga(v), texto(v["link"])) {
		return e, nil, validacao("a vaga precisa de um endereço HTTPS válido da plataforma")
	}
	novo := e
	var i int
	var c Candidatura
	for idx, o := range e.Candidaturas {
		if o["vagaId"] != id || o["estado"] == CandCancelada {
			continue
		}
		var err error
		c, err = candidaturaDe(o)
		if err != nil {
			return e, nil, err
		}
		i = idx
		if c.ID == v["candidaturaManualId"] && c.Estado == CandBloqueada && v["aplicacaoManual"] == true {
			return e, v, nil
		}
		linkAtual := texto(v["link"])
		if (c.Estado != CandPreparada && c.Estado != CandAprovada) || (c.URL != linkAtual && !MesmaVaga(c.Plataforma, c.URL, linkAtual)) {
			return e, nil, &ErroConflito{Msg: "já existe uma candidatura para esta vaga; confira a Fila"}
		}
		break
	}
	if v["etapa"] != "aplicar" || !vagaAprovada(novo, id) {
		var err error
		novo, _, err = MoverVaga(novo, id, "aplicar", nil, ctx)
		if err != nil {
			return e, nil, err
		}
	}
	if c.ID == "" {
		var o Objeto
		var err error
		novo, o, err = CriarCandidatura(novo, id, pacote, agora, ctx)
		if err != nil {
			return e, nil, err
		}
		i, c, err = acharCandidatura(novo, texto(o["id"]))
		if err != nil {
			return e, nil, err
		}
	}
	// A reserva disputa a mesma transação da fila. Se o executor já iniciou,
	// o estado enviando foi recusado acima. Se a pessoa chegou primeiro,
	// a autorização antiga é retirada antes de o novo estado ficar visível.
	if c.Estado == CandAprovada {
		c.RevogadaPor = QuemDe(ctx)
	}
	c.Estado, c.Aprovacao, c.ProximaTentativaEm = CandBloqueada, nil, ""
	c.Bloqueio = &Bloqueio{Motivo: "site-externo", Pendencia: "Conclua a candidatura no site da empresa e confirme no Applyer.", Em: agora.Format(time.RFC3339)}
	novo, _ = salvarCandidatura(novo, i, c, agora)
	v = vagaDe(novo, id)
	v["aplicacaoManual"], v["candidaturaManualId"], v["urlCandidaturaManual"] = true, c.ID, c.URL
	return novo, v, nil
}

func ConfirmarManualRadar(e Estado, id string, agora time.Time, ctx Ctx) (Estado, Objeto, error) {
	if err := soPessoa(ctx, "confirmar candidatura manual"); err != nil {
		return e, nil, err
	}
	v := vagaDe(e, id)
	if v == nil {
		return e, nil, &ErroNaoEncontrado{"vaga não encontrada"}
	}
	cid := texto(v["candidaturaManualId"])
	if cid != "" {
		_, c, err := acharCandidatura(e, cid)
		if err == nil && c.VagaID == id {
			if c.Estado == CandEnviada && c.Resultado != nil && c.Resultado.Tipo == "manual" && FoiEnviada(v) {
				return e, v, nil
			}
			if v["aplicacaoManual"] == true && c.Estado == CandBloqueada && v["etapa"] == "aplicar" && !FoiEnviada(v) && c.URL == v["urlCandidaturaManual"] {
				n, _, err := ConcluirManual(e, cid, agora, ctx)
				if err != nil {
					return e, nil, err
				}
				return n, vagaDe(n, id), nil
			}
		}
	}
	return e, nil, &ErroConflito{Msg: "inicie a candidatura no Radar antes de confirmar o envio"}
}

func limparIntencaoRadar(e Estado, c Candidatura, guardarRecibo bool) {
	if v := vagaDe(e, c.VagaID); v != nil && v["candidaturaManualId"] == c.ID {
		delete(v, "aplicacaoManual")
		delete(v, "urlCandidaturaManual")
		if !guardarRecibo {
			delete(v, "candidaturaManualId")
		}
	}
}
