package armazem

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"regexp"
	"strings"

	"github.com/FRF-Tecnology/job-control/internal/curriculos"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

const chaveRascunho = "rascunhoPrimeiroAcesso"

var ErrRevisaoRascunho = errors.New("o rascunho mudou em outra janela; recarregue antes de salvar")
var nomeDoRascunho = regexp.MustCompile(`^curriculo-([A-Z2-7]{26,})\.pdf$`)

type DocumentoRascunho struct {
	ArquivoID    string `json:"arquivoId"`
	NomeExibicao string `json:"nomeExibicao"`
	Tamanho      int    `json:"tamanho"`
}

func (d DocumentoRascunho) nomeLocal() string { return "curriculo-" + d.ArquivoID + ".pdf" }

type Rascunho struct {
	Revisao          int64              `json:"revisao"`
	Etapa            int                `json:"etapa"`
	Dados            dominio.Objeto     `json:"dados"`
	Editados         []string           `json:"editados"`
	Curriculo        *DocumentoRascunho `json:"curriculo"`
	TentativaExterna string             `json:"tentativaExterna,omitempty"`
}
type ReferenciaRascunho struct {
	ArquivoID  string
	Revisao    int64
	Assinatura string
}

func lerRascunho(ctx context.Context, tx *sql.Tx) (Rascunho, error) {
	d := Rascunho{Etapa: 1, Dados: dominio.Objeto{}, Editados: []string{}}
	var raw string
	err := tx.QueryRowContext(ctx, "SELECT valor FROM meta WHERE chave=?", chaveRascunho).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return d, nil
	}
	if err != nil {
		return d, err
	}
	err = json.Unmarshal([]byte(raw), &d)
	return d, err
}
func gravarRascunho(ctx context.Context, tx *sql.Tx, d Rascunho) error {
	raw, err := json.Marshal(d)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO meta (chave,valor) VALUES (?,?) ON CONFLICT(chave) DO UPDATE SET valor=excluded.valor", chaveRascunho, string(raw))
	return err
}

func lerRascunhoAberto(ctx context.Context, tx *sql.Tx) (Rascunho, error) {
	marca, err := lerPrimeiroAcesso(ctx, tx)
	if err != nil {
		return Rascunho{}, err
	}
	if marca.Concluido {
		return Rascunho{}, ErrOnboardingConcluido
	}
	return lerRascunho(ctx, tx)
}

func (a *Armazem) LerRascunho(ctx context.Context) (Rascunho, error) {
	var d Rascunho
	if _, err := a.OnboardingConcluido(ctx); err != nil {
		return d, err
	}
	err := a.Transacao(ctx, func(tx *sql.Tx) error {
		var err error
		d, err = lerRascunhoAberto(ctx, tx)
		return err
	})
	return d, err
}

func (a *Armazem) EditarRascunho(ctx context.Context, rev int64, mudar func(*Rascunho) error) (Rascunho, error) {
	var d Rascunho
	if _, err := a.OnboardingConcluido(ctx); err != nil {
		return d, err
	}
	err := a.Transacao(ctx, func(tx *sql.Tx) error {
		var err error
		d, err = lerRascunhoAberto(ctx, tx)
		if err != nil {
			return err
		}
		if d.Revisao != rev {
			return ErrRevisaoRascunho
		}
		if err = mudar(&d); err != nil {
			return err
		}
		d.Revisao++
		return gravarRascunho(ctx, tx, d)
	})
	return d, err
}

func abrirPastaRascunho(pasta string, criar bool) (*os.Root, error) {
	raiz, err := curriculos.AbrirPasta(pasta)
	if err != nil {
		return nil, err
	}
	defer raiz.Close()
	return curriculos.AbrirRascunho(raiz, criar)
}

// Sob a trava de escrita, só o documento referenciado no SQLite é conservado.
// Também cobre interrupção entre troca/exclusão do vínculo e limpeza do disco.
func limparRascunho(pasta string, d Rascunho) error {
	raiz, err := abrirPastaRascunho(pasta, false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer raiz.Close()
	nome := ""
	if d.Curriculo != nil {
		nome = d.Curriculo.nomeLocal()
	}
	if err := curriculos.RecuperarImportacoes(raiz, nome); err != nil {
		return err
	}
	itens, err := fs.ReadDir(raiz.FS(), ".")
	if err != nil {
		return err
	}
	for _, i := range itens {
		if !i.IsDir() && nomeDoRascunho.MatchString(i.Name()) && i.Name() != nome {
			if err := raiz.Remove(i.Name()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *Armazem) TrocarPDFRascunho(ctx context.Context, rev int64, nome string, pdf []byte) (Rascunho, error) {
	d, err := a.EditarRascunho(ctx, rev, func(d *Rascunho) error {
		if pdf != nil {
			raiz, err := abrirPastaRascunho(a.pasta, true)
			if err != nil {
				return err
			}
			defer raiz.Close()
			_, arquivo, err := curriculos.Importar(raiz, pdf)
			if err != nil {
				return err
			}
			d.Curriculo = &DocumentoRascunho{ArquivoID: strings.TrimSuffix(strings.TrimPrefix(arquivo, "curriculo-"), ".pdf"), NomeExibicao: nome, Tamanho: len(pdf)}
		} else {
			d.Curriculo = nil
		}
		d.Etapa = 1
		manter := dominio.Objeto{}
		for _, k := range d.Editados {
			if v, ok := d.Dados[k]; ok {
				manter[k] = v
			}
		}
		d.Dados = manter
		return nil
	})
	if a.pasta != "" {
		reparo := a.Transacao(context.WithoutCancel(ctx), func(tx *sql.Tx) error {
			atual, e := lerRascunho(context.WithoutCancel(ctx), tx)
			if e != nil {
				return e
			}
			return limparRascunho(a.pasta, atual)
		})
		return d, errors.Join(err, reparo)
	}
	return d, err
}

func (a *Armazem) pdfRascunho(ctx context.Context, tx *sql.Tx, ref ReferenciaRascunho) ([]byte, string, error) {
	d, err := lerRascunhoAberto(ctx, tx)
	if err != nil {
		return nil, "", err
	}
	if d.Curriculo == nil || d.Revisao != ref.Revisao || d.Curriculo.ArquivoID != ref.ArquivoID || !nomeDoRascunho.MatchString(d.Curriculo.nomeLocal()) {
		return nil, "", ErrRevisaoRascunho
	}
	raiz, err := abrirPastaRascunho(a.pasta, false)
	if err != nil {
		return nil, "", err
	}
	defer raiz.Close()
	b, err := curriculos.LerDaRaiz(raiz, d.Curriculo.nomeLocal())
	return b, d.Curriculo.NomeExibicao, err
}
func (a *Armazem) LerPDFRascunho(ctx context.Context, ref ReferenciaRascunho) ([]byte, error) {
	var pdf []byte
	err := a.Transacao(ctx, func(tx *sql.Tx) error {
		var err error
		pdf, _, err = a.pdfRascunho(ctx, tx, ref)
		return err
	})
	return pdf, err
}
