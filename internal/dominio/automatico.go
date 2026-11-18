package dominio

import "time"

// Modo automático (perfil.modo "direto"): a pessoa liga na tela, e só ela (a API recusa o assistente). Nele, o app
// aprova sozinho o pacote preparado das vagas nota A do Radar, dentro dos limites do dia, e o executor envia. As
// vagas B e C, e as que a pessoa escolheu, esperam a aprovação dela, como no modo Escolher. Formulário com resposta
// faltando não vai. Puro: recebe o estado e devolve o novo, com os ids aprovados.
func AprovarAutomaticas(e Estado, agora time.Time, limites map[string]int) (Estado, []string) {
	if !ModoAutomatico(e) {
		return e, nil
	}
	var candidatas []Candidatura
	for _, o := range e.Candidaturas {
		c, err := candidaturaDe(o)
		if err != nil || c.Estado != CandPreparada {
			continue
		}
		if v := vagaDe(e, c.VagaID); v != nil && notaADoRadar(v) {
			candidatas = append(candidatas, c)
		}
	}
	novo := e
	var aprovadas []string
	for _, c := range candidatas {
		depois, _, err := aprovar(novo, c.ID, c.Impressao, agora, limites, true)
		if err != nil { // limite do dia, resposta faltando ou mudou no meio: fica para a pessoa
			continue
		}
		novo = depois
		aprovadas = append(aprovadas, c.ID)
	}
	return novo, aprovadas
}
