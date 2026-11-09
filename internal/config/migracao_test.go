package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Applyer, PR A (jc-etv.7, docs/propostas/applyer-nomes-internos.md, itens 1 e 2): a pasta de dados passa de
// "Central de Candidaturas" para "Applyer", com rename ao abrir, e APPLYER_PASTA substitui CENTRAL_PASTA (aceita por
// duas versões). Só pastas de teste.

func semVariaveis(t *testing.T) {
	t.Helper()
	t.Setenv("APPLYER_PASTA", "")
	t.Setenv("CENTRAL_PASTA", "")
}

func base(t *testing.T) string {
	t.Helper()
	b, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func criarCom(t *testing.T, pasta, arquivo string) {
	t.Helper()
	if err := os.MkdirAll(pasta, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pasta, arquivo), []byte("ficticio"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func existe(p string) bool { _, err := os.Lstat(p); return err == nil }

func TestPastaNovaQuandoNaoHaNenhuma(t *testing.T) {
	casaFalsa(t)
	semVariaveis(t)
	a, err := AbrirPastaDados()
	if err != nil {
		t.Fatal(err)
	}
	nova := filepath.Join(base(t), "Applyer")
	if !MesmaPasta(a.Caminho, nova) || a.Migrou || a.Aviso != "" || !existe(nova) || existe(filepath.Join(base(t), "Central de Candidaturas")) {
		t.Fatalf("nenhuma = %+v", a)
	}
}

func TestPastaAntigaEhRenomeadaInteira(t *testing.T) {
	casaFalsa(t)
	semVariaveis(t)
	antiga, nova := filepath.Join(base(t), "Central de Candidaturas"), filepath.Join(base(t), "Applyer")
	criarCom(t, antiga, "central.db")
	criarCom(t, filepath.Join(antiga, "navegador", "Default"), "Cookies")
	a, err := AbrirPastaDados()
	if err != nil {
		t.Fatal(err)
	}
	if !MesmaPasta(a.Caminho, nova) || !a.Migrou || a.Aviso != "" {
		t.Fatalf("antiga = %+v", a)
	}
	if existe(antiga) || !existe(filepath.Join(nova, "central.db")) || !existe(filepath.Join(nova, "navegador", "Default", "Cookies")) {
		t.Fatal("o rename não levou tudo, ou deixou a antiga")
	}
	// Abrir de novo: já migrada, nada muda.
	if b, err := AbrirPastaDados(); err != nil || b.Migrou || b.Aviso != "" || !MesmaPasta(b.Caminho, nova) {
		t.Fatalf("de novo = %+v %v", b, err)
	}
}

func TestAsDuasPastasUsaANovaEAvisa(t *testing.T) {
	casaFalsa(t)
	semVariaveis(t)
	antiga, nova := filepath.Join(base(t), "Central de Candidaturas"), filepath.Join(base(t), "Applyer")
	criarCom(t, antiga, "central.db")
	criarCom(t, nova, "central.db")
	a, err := AbrirPastaDados()
	if err != nil {
		t.Fatal(err)
	}
	if !MesmaPasta(a.Caminho, nova) || a.Migrou || a.Aviso != AvisoAntigaFicou {
		t.Fatalf("as duas = %+v", a)
	}
	// Nada é juntado nem apagado sozinho.
	if !existe(filepath.Join(antiga, "central.db")) {
		t.Fatal("mexeu na antiga")
	}
}

func TestRenameRecusadoUsaAAntigaNestaVez(t *testing.T) {
	casaFalsa(t)
	semVariaveis(t)
	antiga, nova := filepath.Join(base(t), "Central de Candidaturas"), filepath.Join(base(t), "Applyer")
	criarCom(t, antiga, "central.db")
	antes := renomear
	t.Cleanup(func() { renomear = antes })
	renomear = func(string, string) error { return errors.New("arquivo em uso (teste)") }
	a, err := AbrirPastaDados()
	if err != nil {
		t.Fatal(err)
	}
	if !MesmaPasta(a.Caminho, antiga) || a.Migrou || a.Aviso != AvisoMigracaoAdiada {
		t.Fatalf("recusado = %+v", a)
	}
	if existe(nova) {
		t.Fatal("criou a nova pela metade")
	}
	// Na próxima abertura, com o rename liberado, migra.
	renomear = antes
	if b, err := AbrirPastaDados(); err != nil || !b.Migrou || !MesmaPasta(b.Caminho, nova) {
		t.Fatalf("na próxima = %+v %v", b, err)
	}
}

func TestAntigaQueNaoEPastaNaoMigra(t *testing.T) {
	casaFalsa(t)
	semVariaveis(t)
	antiga := filepath.Join(base(t), "Central de Candidaturas")
	if err := os.MkdirAll(base(t), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(antiga, []byte("não é pasta"), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := AbrirPastaDados()
	if err != nil || a.Migrou || !MesmaPasta(a.Caminho, filepath.Join(base(t), "Applyer")) {
		t.Fatalf("arquivo no lugar da antiga = %+v %v", a, err)
	}
	if b, _ := os.ReadFile(antiga); string(b) != "não é pasta" {
		t.Fatal("mexeu no arquivo")
	}
}

func TestVariaveisDaPasta(t *testing.T) {
	casa := casaFalsa(t)
	um, dois := filepath.Join(casa, "teste-um"), filepath.Join(casa, "teste-dois")
	for _, c := range []struct {
		nome            string
		applyer, antiga string
		quer, origem    string
		falha           bool
	}{
		{"só APPLYER_PASTA", um, "", um, "pasta: APPLYER_PASTA", false},
		{"só CENTRAL_PASTA", "", um, um, "pasta: CENTRAL_PASTA (obsoleta)", false},
		{"as duas iguais", um, um + string(filepath.Separator), um, "pasta: APPLYER_PASTA", false},
		{"as duas diferentes", um, dois, "", "pasta: APPLYER_PASTA e CENTRAL_PASTA diferentes", true},
	} {
		t.Setenv("APPLYER_PASTA", c.applyer)
		t.Setenv("CENTRAL_PASTA", c.antiga)
		a, err := AbrirPastaDados()
		if c.falha {
			if !errors.Is(err, ErrDuasPastas) || existe(um) && existe(dois) {
				t.Errorf("%s: %+v %v", c.nome, a, err)
			}
		} else if err != nil || !MesmaPasta(a.Caminho, c.quer) || a.Migrou {
			t.Errorf("%s: %+v %v", c.nome, a, err)
		}
		if got := OrigemPasta(); got != c.origem {
			t.Errorf("%s: origem %q; queria %q", c.nome, got, c.origem)
		}
	}
	// Com variável, a pasta padrão nunca é migrada.
	t.Setenv("APPLYER_PASTA", um)
	t.Setenv("CENTRAL_PASTA", "")
	criarCom(t, filepath.Join(base(t), "Central de Candidaturas"), "central.db")
	if _, err := AbrirPastaDados(); err != nil || !existe(filepath.Join(base(t), "Central de Candidaturas", "central.db")) {
		t.Fatal("migrou a padrão com APPLYER_PASTA")
	}
}

// O cofre trata a pasta antiga e a nova como a pasta padrão: com o rename adiado, as chaves continuam achadas.
func TestEhPastaPadraoAceitaAsDuas(t *testing.T) {
	casaFalsa(t)
	semVariaveis(t)
	for _, p := range []string{filepath.Join(base(t), "Applyer"), filepath.Join(base(t), "Central de Candidaturas")} {
		if !EhPastaPadrao(p) {
			t.Errorf("%s não é padrão", filepath.Base(p))
		}
	}
	if EhPastaPadrao(filepath.Join(base(t), "Outra")) {
		t.Error("outra pasta é padrão")
	}
}

// Os marcadores escondem as duas pastas padrão e as duas variáveis.
func TestOcultarCaminhosComAsDuasPastas(t *testing.T) {
	casa := casaFalsa(t)
	t.Setenv("APPLYER_PASTA", filepath.Join(casa, "pasta-applyer"))
	t.Setenv("CENTRAL_PASTA", "")
	for _, c := range []struct{ entrada, marcador string }{
		{filepath.Join(casa, "pasta-applyer", "central.db"), "<APPLYER_PASTA>"},
		{filepath.Join(base(t), "Applyer", "central.db"), "<pasta padrão>"},
		{filepath.Join(base(t), "Central de Candidaturas", "central.db"), "<pasta padrão>"},
	} {
		got := OcultarCaminhos("erro em " + c.entrada)
		if got != "erro em "+c.marcador+string(filepath.Separator)+"central.db" {
			t.Errorf("%q", got)
		}
	}
}
