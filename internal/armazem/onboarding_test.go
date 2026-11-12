package armazem

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/curriculos"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

func TestMigracaoDoPrimeiroAcesso(t *testing.T) {
	ctx := context.Background()
	for _, caso := range []struct {
		nome      string
		tx        string
		pdf       bool
		concluido bool
	}{
		{"vazio", "", false, false},
		{"so plataformas de fabrica", `INSERT INTO itens VALUES ('plataformas',0,'{"id":"linkedin"}')`, false, false},
		{"perfil vazio salvo", `INSERT INTO itens VALUES ('perfil',0,'{"nome":"  ","filtros":{},"stackPrincipal":[]}')`, false, false},
		{"perfil preenchido", `INSERT INTO itens VALUES ('perfil',0,'{"cargo":"Designer"}')`, false, true},
		{"vaga existente", `INSERT INTO itens VALUES ('vagas',0,'{"id":"x"}')`, false, true},
		{"ia configurada", `INSERT INTO meta VALUES ('ia','{"rotas":{"padrao":{"provedor":"anthropic"}}}')`, false, true},
		{"curriculo existente", "", true, true},
	} {
		t.Run(caso.nome, func(t *testing.T) {
			pasta := t.TempDir()
			a, err := Abrir(ctx, filepath.Join(pasta, "teste.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer a.Fechar()
			if caso.tx != "" {
				if _, err := a.db.Exec(caso.tx); err != nil {
					t.Fatal(err)
				}
			}
			if caso.pdf {
				if err := os.Mkdir(filepath.Join(pasta, "curriculos"), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(pasta, "curriculos", "anterior.pdf"), []byte("fictício"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := a.PrepararPrimeiraVez(ctx, []byte("[]")); err != nil {
				t.Fatal(err)
			}
			if c, err := a.OnboardingConcluido(ctx); err != nil || c != caso.concluido {
				t.Fatalf("concluído=%t, erro=%v", c, err)
			}
			if _, err := a.db.Exec("DELETE FROM itens"); err != nil {
				t.Fatal(err)
			}
			if _, err := a.PrepararPrimeiraVez(ctx, []byte("[]")); err != nil {
				t.Fatal(err)
			}
			if c, err := a.OnboardingConcluido(ctx); err != nil || c != caso.concluido {
				t.Fatalf("a segunda abertura alterou o marcador: %t %v", c, err)
			}
		})
	}
}

func TestOnboardingDesfazErroERecuperaInterrupcao(t *testing.T) {
	ctx := context.Background()
	pasta := t.TempDir()
	a, err := Abrir(ctx, filepath.Join(pasta, "teste.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Fechar()
	if _, err := a.PrepararPrimeiraVez(ctx, []byte("[]")); err != nil {
		t.Fatal(err)
	}
	pdfs := filepath.Join(pasta, "curriculos")
	pdf := []byte("%PDF-1.4\n%%EOF")
	// Falha no último passo do COMMIT, depois de publicar o arquivo e o perfil.
	if _, err := a.db.Exec("CREATE TRIGGER falha BEFORE UPDATE ON versao BEGIN SELECT RAISE(ABORT, 'teste'); END;"); err != nil {
		t.Fatal(err)
	}
	montar := func(_ dominio.Objeto, nome string) (dominio.Objeto, error) {
		return dominio.Objeto{"cargo": "Designer", "curriculos": dominio.Objeto{"PT": "curriculos/" + nome}}, nil
	}
	if err := a.ConcluirOnboarding(ctx, pdf, montar); err == nil {
		t.Fatal("era esperada a falha do banco")
	}
	itens, err := os.ReadDir(pdfs)
	if err != nil || len(itens) != 0 {
		t.Fatalf("restaram arquivos: %v %v", itens, err)
	}
	e, err := a.Carregar(ctx)
	if err != nil || len(e.Perfil) != 0 {
		t.Fatalf("perfil após rollback: %v %v", e.Perfil, err)
	}
	if c, err := a.OnboardingConcluido(ctx); err != nil || c {
		t.Fatalf("marcador após rollback: %t %v", c, err)
	}
	if _, err := a.db.Exec("DROP TRIGGER falha"); err != nil {
		t.Fatal(err)
	}

	raiz, err := curriculos.AbrirPasta(pasta)
	if err != nil {
		t.Fatal(err)
	}
	defer raiz.Close()
	// Simular encerramento do processo entre Rename e COMMIT, com a pasta
	// de importação e o PDF em disco, mas sem o marcador de conclusão.
	err = a.Transacao(ctx, func(_ *sql.Tx) error {
		_, _, err := curriculos.Importar(raiz, pdf)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := a.prepararOnboarding(ctx); err != nil {
		t.Fatal(err)
	}
	itens, err = os.ReadDir(pdfs)
	if err != nil || len(itens) != 0 {
		t.Fatalf("recuperação deixou arquivos: %v %v", itens, err)
	}
	if err := a.ConcluirOnboarding(ctx, pdf, montar); err != nil {
		t.Fatal(err)
	}
	if err := a.ConcluirOnboarding(ctx, pdf, montar); !errors.Is(err, ErrOnboardingConcluido) {
		t.Fatalf("segunda tentativa: %v", err)
	}
}
