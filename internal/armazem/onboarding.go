package armazem

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io/fs"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/curriculos"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

const chaveOnboarding = "primeiroAcesso"

type primeiroAcesso struct {
	Concluido bool   `json:"concluido"`
	Arquivo   string `json:"arquivo,omitempty"`
}

var ErrOnboardingConcluido = errors.New("o primeiro acesso já foi concluído")

func lerPrimeiroAcesso(ctx context.Context, tx *sql.Tx) (primeiroAcesso, error) {
	var registro primeiroAcesso
	var valor []byte
	if err := tx.QueryRowContext(ctx, "SELECT valor FROM meta WHERE chave = ?", chaveOnboarding).Scan(&valor); err != nil {
		return registro, err
	}
	err := json.Unmarshal(valor, &registro)
	return registro, err
}

// Migração avaliada uma única vez. Plataformas pré-instaladas e a linha do
// perfil vazio são criadas pelo próprio app. Atividade, preferências preenchidas,
// uma IA configurada ou um currículo local indicam uma instalação já em uso.
func (a *Armazem) prepararOnboarding(ctx context.Context) error {
	return a.Transacao(ctx, func(tx *sql.Tx) error {
		registro, err := lerPrimeiroAcesso(ctx, tx)
		existe := err == nil
		if !existe && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if a.pasta != "" {
			temPDF, err := recuperarPasta(a.pasta, registro.Arquivo, !existe)
			if err != nil && !errors.Is(err, curriculos.ErrFora) {
				return err
			}
			registro.Concluido = registro.Concluido || temPDF
		}
		if existe {
			return nil
		}
		if !registro.Concluido {
			err = tx.QueryRowContext(ctx, `SELECT EXISTS(
				SELECT 1 FROM itens WHERE colecao NOT IN ('perfil', 'plataformas')
				UNION ALL SELECT 1 FROM itens p, json_tree(p.dados) j WHERE colecao = 'perfil'
					AND j.atom IS NOT NULL AND trim(CAST(j.atom AS TEXT)) != ''
				UNION ALL SELECT 1 FROM meta WHERE chave = 'ia' AND
					coalesce(json_extract(valor,'$.rotas.padrao.provedor'), '') != ''
			)`).Scan(&registro.Concluido)
			if err != nil {
				return err
			}
		}
		valor, err := json.Marshal(registro)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO meta (chave, valor) VALUES (?, ?)", chaveOnboarding, string(valor))
		return err
	})
}

func recuperarPasta(pasta, confirmado string, verificarLegado bool) (bool, error) {
	raiz, err := curriculos.AbrirPasta(pasta)
	if err != nil {
		return false, err
	}
	defer raiz.Close()
	if err := curriculos.RecuperarImportacoes(raiz, confirmado); err != nil {
		return false, err
	}
	if !verificarLegado {
		return false, nil
	}
	itens, err := fs.ReadDir(raiz.FS(), ".")
	if err != nil {
		return false, err
	}
	for _, item := range itens {
		if item.Type().IsRegular() && strings.HasSuffix(strings.ToLower(item.Name()), ".pdf") {
			return true, nil
		}
	}
	return false, nil
}

// OnboardingConcluido também atende o servidor embutido/testes que abram
// diretamente a API, sem o preparo de primeira execução da janela.
func (a *Armazem) OnboardingConcluido(ctx context.Context) (bool, error) {
	var registro primeiroAcesso
	existe, err := a.LerMeta(ctx, chaveOnboarding, &registro)
	if err == nil && !existe {
		if err = a.prepararOnboarding(ctx); err == nil {
			_, err = a.LerMeta(ctx, chaveOnboarding, &registro)
		}
	}
	return registro.Concluido, err
}

// ConcluirOnboarding publica o PDF e grava perfil, marcador e versão sob a
// mesma trava de escrita entre processos. A referência só existe após COMMIT.
func (a *Armazem) ConcluirOnboarding(ctx context.Context, pdf []byte, montar func(dominio.Objeto, string) (dominio.Objeto, error)) error {
	if _, err := a.OnboardingConcluido(ctx); err != nil {
		return err
	}
	raiz, err := curriculos.AbrirPasta(a.pasta)
	if err != nil {
		return err
	}
	defer raiz.Close()
	var marca string
	err = a.gravarCom(ctx, func(tx *sql.Tx, e dominio.Estado) (dominio.Estado, error) {
		registro, err := lerPrimeiroAcesso(ctx, tx)
		if err != nil {
			return e, err
		}
		if registro.Concluido {
			return e, ErrOnboardingConcluido
		}
		if err := curriculos.RecuperarImportacoes(raiz, ""); err != nil {
			return e, err
		}
		var nome string
		marca, nome, err = curriculos.Importar(raiz, pdf)
		if err != nil {
			return e, err
		}
		e.Perfil, err = montar(e.Perfil, nome)
		if err != nil {
			return e, err
		}
		valor, err := json.Marshal(primeiroAcesso{Concluido: true, Arquivo: nome})
		if err != nil {
			return e, err
		}
		_, err = tx.ExecContext(ctx, "UPDATE meta SET valor = ? WHERE chave = ?", string(valor), chaveOnboarding)
		return e, err
	})
	if marca != "" {
		// Um cancelamento HTTP também desfaz o arquivo. Reconsultar o banco
		// protege tanto uma falha de COMMIT quanto a corrida com outra janela.
		reparo := a.prepararOnboarding(context.WithoutCancel(ctx))
		if err != nil {
			return errors.Join(err, reparo)
		}
		// Se só a limpeza da marca falhar, o perfil e o PDF já estão salvos.
		// A próxima abertura faz a limpeza de novo.
	}
	return err
}
