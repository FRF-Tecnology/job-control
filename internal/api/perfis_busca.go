package api

import (
	"context"
	"crypto/rand"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Uma janela pode continuar mostrando o perfil anterior até a próxima leitura.
// O identificador que ela viu é conferido dentro da mesma transação da edição.
func conferirPerfilBusca(e dominio.Estado, esperado string) error {
	if esperado != "" && esperado != e.PerfilBuscaAtivo() {
		return falha(409, "o perfil de busca mudou. Atualize a tela e tente novamente")
	}
	return nil
}

func (a *API) perfisBusca(ctx context.Context, metodo, caminho, autor string, corpo Objeto) (Resposta, error) {
	if autor != "eu" {
		return Resposta{}, falha(403, "somente a pessoa pode gerenciar os perfis de busca")
	}
	if metodo != "POST" {
		return Resposta{}, falha(404, "rota não encontrada")
	}
	criar := caminho == "/perfis-busca"
	id := strings.TrimPrefix(caminho, "/perfis-busca/")
	if criar {
		id = rand.Text()
	} else {
		var ok bool
		id, ok = strings.CutSuffix(id, "/ativar")
		if !ok || id == "" || strings.Contains(id, "/") {
			return Resposta{}, falha(404, "rota não encontrada")
		}
	}
	err := a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		if err := conferirPerfilBusca(e, dominio.Texto(corpo["perfilOrigem"])); err != nil {
			return e, err
		}
		if criar {
			nome, _ := corpo["nome"].(string)
			modelo, _ := corpo["modelo"].(string)
			return dominio.CriarPerfilBusca(e, id, nome, modelo)
		}
		return dominio.AtivarPerfilBusca(e, id)
	})
	if err != nil {
		return Resposta{}, err
	}
	resultado := Objeto{"perfilBuscaAtivo": id}
	if criar {
		return criado(resultado)
	}
	return ok(resultado)
}
