package curriculos

import (
	"crypto/rand"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// A raiz prende todas as operações ao diretório do banco, inclusive se outra
// aplicação trocar uma pasta por um link durante a importação.
func AbrirPasta(pastaDados string) (*os.Root, error) {
	if pastaDados == "" {
		return nil, ErrFora
	}
	dados, err := os.OpenRoot(pastaDados)
	if err != nil {
		return nil, err
	}
	defer dados.Close()
	return abrirSubpasta(dados, "curriculos", true)
}

// O rascunho fica fora da lista de PDFs do perfil até a confirmação da pessoa.
func AbrirRascunho(curriculos *os.Root, criar bool) (*os.Root, error) {
	return abrirSubpasta(curriculos, ".rascunho-onboarding", criar)
}

func abrirSubpasta(dados *os.Root, nome string, criar bool) (*os.Root, error) {
	if criar {
		if err := dados.Mkdir(nome, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
	}
	info, err := dados.Lstat(nome)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, ErrFora
	}
	// No Windows, SameFile fixa a identidade de Lstat ao consultá-la pela
	// primeira vez (assim como na leitura de um PDF em Ler).
	os.SameFile(info, info)
	raiz, err := dados.OpenRoot(nome)
	if err != nil {
		return nil, err
	}
	aberta, err := raiz.Stat(".")
	if err != nil || !os.SameFile(info, aberta) {
		raiz.Close()
		return nil, ErrFora
	}
	return raiz, nil
}

const prefixoImportacao = ".applyer-importacao-"

var marcaImportacao = regexp.MustCompile(`^\.applyer-importacao-[A-Z2-7]{26,}$`)

// Uma pasta temporária também registra a importação em andamento. O PDF só
// aparece na lista de currículos depois de escrito, sincronizado e fechado.
// Quem chama segura a transação de escrita do SQLite até guardar a referência.
func Importar(raiz *os.Root, pdf []byte) (marca, nome string, err error) {
	id := rand.Text()
	marca, nome = prefixoImportacao+id, "curriculo-"+id+".pdf"
	if _, err = raiz.Lstat(nome); !errors.Is(err, fs.ErrNotExist) {
		if err == nil {
			err = fs.ErrExist
		}
		return "", "", err
	}
	if err := raiz.Mkdir(marca, 0o700); err != nil {
		return "", "", err
	}
	temporario := filepath.Join(marca, "documento")
	f, err := raiz.OpenFile(temporario, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err == nil {
		_, err = f.Write(pdf)
		if err == nil {
			err = f.Sync()
		}
		err = errors.Join(err, f.Close())
	}
	if err == nil {
		err = raiz.Rename(temporario, nome)
	}
	if err != nil {
		return "", "", errors.Join(err, removerSeExiste(raiz, temporario), removerSeExiste(raiz, marca))
	}
	return marca, nome, nil
}

func removerSeExiste(raiz *os.Root, nome string) error {
	err := raiz.Remove(nome)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// RecuperarImportacoes desfaz uma cópia interrompida antes do COMMIT. Se o
// banco confirmou o PDF, resta só limpar a marca. Roda sob a trava do SQLite.
func RecuperarImportacoes(raiz *os.Root, confirmado string) error {
	itens, err := fs.ReadDir(raiz.FS(), ".")
	if err != nil {
		return err
	}
	for _, item := range itens {
		marca := item.Name()
		if !item.IsDir() || !marcaImportacao.MatchString(marca) {
			continue
		}
		nome := "curriculo-" + strings.TrimPrefix(marca, prefixoImportacao) + ".pdf"
		if nome != confirmado {
			if err := removerSeExiste(raiz, nome); err != nil {
				return err
			}
		}
		if err := removerSeExiste(raiz, filepath.Join(marca, "documento")); err != nil {
			return err
		}
		if err := removerSeExiste(raiz, marca); err != nil {
			return err
		}
	}
	return nil
}
