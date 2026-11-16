package api

import (
	"context"
	"errors"
	"log"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/triagem"
)

// "Ler a descrição da vaga" (POST /vagas/{id}/ler-descricao, só da tela). Uma vaga do Radar que chegou sem descrição
// (os alertas do LinkedIn trazem só título, empresa e local) é aberta no navegador do envio, com a sessão da pessoa,
// SEM INTERAGIR, e a descrição vai para a vaga. A IA avalia a vaga de novo na hora (a mesma triagem da busca).
//   - Só a pessoa, um clique por vaga (o produto é copiloto: nada abre o LinkedIn sozinho).
//   - Só vaga do Radar (descoberta), do LinkedIn, com o endereço da vaga na plataforma; a página tem de parar na
//     mesma vaga (senão, 409 com o motivo).
//   - O texto da página é de terceiro: até 6000 caracteres (cortado no fim de uma palavra), sem <>, com os
//     parágrafos. Não vai para o log.

const tetoDescricao = 6000

var msgsDescricao = map[string]string{
	dominio.EnderecoLogin: "o LinkedIn pediu login: entre nas plataformas pelo painel de Envio e tente de novo",
	"sem-descricao":       "a página da vaga abriu, mas não achei a descrição",
}

func limparDescricao(s string) string {
	return dominio.TextoLongo(s, tetoDescricao)
}

// ajustarNota: as regras da busca para a nota da IA (A só com evidência forte; pular é C).
func ajustarNota(v Objeto, nota triagem.Resultado) triagem.Resultado {
	if nota.Aderencia == "A" && v["radarClasse"] != "forte" {
		nota.Aderencia = "B"
	}
	if nota.Decisao == "pular" && nota.Aderencia != "" {
		nota.Aderencia = "C"
	}
	return nota
}

func (a *API) lerDescricaoDaVaga(ctx context.Context, id string, c Objeto, autor string) (Resposta, error) {
	if a.lerDescricao == nil {
		return Resposta{}, falha(404, msgSemEnvio)
	}
	if autor != "eu" {
		return Resposta{}, &dominio.ErroSoPessoa{Msg: "só a pessoa, pela tela, abre a vaga para ler a descrição"}
	}
	if err := soCampos(c); err != nil {
		return Resposta{}, err
	}
	if a.janela != nil {
		if _, login := a.janela(); login {
			return Resposta{}, &dominio.ErroConflito{Msg: "feche a janela de login antes de ler a vaga", Motivo: "login-aberto"}
		}
	}
	if err := a.semEnvioEmAndamento(ctx, "há uma candidatura sendo enviada agora; espere terminar"); err != nil {
		return Resposta{}, err
	}
	e, err := a.arm.Carregar(ctx)
	if err != nil {
		return Resposta{}, err
	}
	var vaga Objeto
	for _, v := range e.Vagas {
		if v["id"] == id {
			vaga = v
		}
	}
	if vaga == nil {
		return Resposta{}, falha(404, "vaga não encontrada")
	}
	link := dominio.Texto(vaga["link"])
	plataforma := dominio.PlataformaDaVaga(vaga)
	if vaga["etapa"] != "descoberta" {
		return Resposta{}, &dominio.ErroConflito{Msg: "a vaga não está mais no Radar", Motivo: "fora-do-radar"}
	}
	if plataforma != "linkedin" || !dominio.URLDaPlataforma(plataforma, link) {
		return Resposta{}, &dominio.ErroConflito{Msg: "por enquanto, só vagas do LinkedIn", Motivo: "plataforma"}
	}

	// Fora da transação: o navegador pode levar segundos.
	texto, bloqueio, err := a.lerDescricao(ctx, plataforma, link)
	if err != nil {
		var ec *dominio.ErroConflito
		if errors.As(err, &ec) {
			return Resposta{}, err // envio em andamento
		}
		log.Printf("radar: leitura da descrição falhou (%s)", tipoDoErro(err)) // só o tipo
		return Resposta{}, &dominio.ErroConflito{Msg: "não deu para abrir a vaga agora; tente de novo", Motivo: "falhou"}
	}
	descricao := limparDescricao(texto)
	if bloqueio == "" && descricao == "" {
		bloqueio = "sem-descricao"
	}
	if bloqueio != "" {
		log.Printf("radar: descrição não lida (%s)", bloqueio) // só a categoria
		msg, conhecido := msgsDescricao[bloqueio]
		if !conhecido {
			msg, bloqueio = "a página não abriu na vaga (ela pode ter saído do ar)", "fora"
		}
		return Resposta{}, &dominio.ErroConflito{Msg: msg, Motivo: bloqueio}
	}

	var gravada Objeto
	var assinatura string
	err = a.gravar(ctx, func(atual dominio.Estado) (dominio.Estado, error) {
		for i, v := range atual.Vagas {
			if v["id"] != id {
				continue
			}
			if v["etapa"] != "descoberta" || dominio.Texto(v["link"]) != link {
				return atual, &dominio.ErroConflito{Msg: "a vaga mudou enquanto a página abria; tente de novo", Motivo: "vaga-mudou"}
			}
			nova := aguardandoTriagem(v)
			nova["descricao"] = descricao
			classe, motivo := triagem.QualificarRadar(atual.Perfil, nova, a.relogio())
			nova["radarClasse"], nova["radarMotivo"] = classe, motivo
			if problemas := dominio.ValidarVaga(nova); len(problemas) > 0 {
				return atual, &dominio.ErroValidacao{Erros: problemas}
			}
			atual.Vagas[i] = nova
			gravada, assinatura = nova, assinaturaDeBusca(atual.Perfil)
			return atual, nil
		}
		return atual, falha(404, "vaga não encontrada")
	})
	if err != nil {
		return Resposta{}, err
	}
	log.Printf("radar: descrição lida (%d caracteres)", len([]rune(descricao)))
	avaliada := a.avaliarVagaDoRadar(ctx, gravada, assinatura)
	if e, err := a.arm.Carregar(ctx); err == nil { // a vaga como ficou depois da nota
		for _, v := range e.Vagas {
			if v["id"] == id {
				gravada = v
			}
		}
	}
	return ok(Objeto{"vaga": gravada, "avaliada": avaliada})
}

// avaliarVagaDoRadar: a triagem da busca para uma vaga só. Sem IA, com uma busca em andamento ou com falha da IA, a
// vaga fica pendente (a próxima busca avalia). Devolve se a nota foi gravada.
func (a *API) avaliarVagaDoRadar(ctx context.Context, v Objeto, assinatura string) bool {
	if !a.buscaEmCurso.TryLock() {
		return false
	}
	defer a.buscaEmCurso.Unlock()
	e, err := a.arm.Carregar(ctx)
	if err != nil {
		return false
	}
	var nota triagem.Resultado
	if v["radarClasse"] == "excluir" {
		motivo := dominio.Texto(v["radarMotivo"])
		nota = triagem.Resultado{Estado: "pulada", Decisao: "pular", Motivo: motivo, MotivoPulo: motivo}
	} else {
		cfg, err := a.configIA(ctx)
		if err != nil || (cfg.Rotas["padrao"].Provedor == "" && cfg.Rotas["triarVaga"].Provedor == "") {
			return false
		}
		motor, err := a.novaIA(ctx)
		if err != nil {
			return false
		}
		porVaga, cancelar := context.WithTimeout(ctx, prazoPorVaga)
		nota, err = triagem.AvaliarRadar(porVaga, motor, e.Perfil, v)
		cancelar()
		if err != nil {
			log.Printf("radar: a IA não avaliou a vaga (%s)", tipoDoErro(err))
			return false
		}
		nota = ajustarNota(v, nota)
	}
	aplicado, err := a.salvarParecer(ctx, v, assinatura, nota)
	return err == nil && aplicado
}
