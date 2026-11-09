package legado

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func escrever(t *testing.T, dir, nome, conteudo string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, nome)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, nome), []byte(conteudo), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestLerDadosDaCentralAntiga(t *testing.T) {
	dir := t.TempDir()
	escrever(t, dir, "vagas.json", `[{"id":"li-1","empresa":"Acme","titulo":"Dev","etapa":"enviada","envio":{"data":"2026-11-01"}},{"id":"li-2","empresa":"","titulo":"x","etapa":"descoberta"}]`)
	escrever(t, dir, "eventos.json", `[{"id":"ev-1","vagaId":"li-1","tipo":"enviada"}]`)
	escrever(t, dir, "perfil.json", `{"nome":"Ana","foco":"ambos"}`)
	escrever(t, dir, "curriculos/Ana-CV.pdf", "%PDF-1.4")
	escrever(t, dir, "curriculos/notas.txt", "x")

	e, rel, err := Ler(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Vagas) != 2 || len(e.Eventos) != 1 || e.Perfil["nome"] != "Ana" || len(e.Pedidos) != 0 {
		t.Fatalf("estado lido errado: %d vagas, %d eventos, perfil %v", len(e.Vagas), len(e.Eventos), e.Perfil)
	}
	if len(rel.Curriculos) != 1 || rel.Curriculos[0] != "Ana-CV.pdf" {
		t.Errorf("currículos = %v", rel.Curriculos)
	}
	if len(rel.Bloqueios) != 0 || len(rel.Avisos) != 1 || !strings.Contains(rel.Avisos[0], "li-2: campo obrigatório: empresa") {
		t.Errorf("bloqueios %v, avisos %v", rel.Bloqueios, rel.Avisos)
	}

	destino := filepath.Join(t.TempDir(), "curriculos")
	if n, err := CopiarCurriculos(dir, destino, rel.Curriculos); err != nil || n != 1 {
		t.Fatalf("copiar = %d, %v", n, err)
	}
	if n, _ := CopiarCurriculos(dir, destino, rel.Curriculos); n != 0 {
		t.Errorf("copiar de novo = %d; não deveria sobrescrever", n)
	}
}

func TestDocumentoBloqueiaImportacao(t *testing.T) {
	dir := t.TempDir()
	escrever(t, dir, "vagas.json", `[{"id":"li-1","empresa":"Acme","titulo":"Dev","etapa":"descoberta","notas":"cpf 529.982.247-25"}]`)
	escrever(t, dir, "eventos.json", `[{"id":"ev-1","texto":"RG: 1234567"}]`)
	_, rel, err := Ler(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rel.Bloqueios) != 2 {
		t.Fatalf("bloqueios = %v; queria a vaga e o evento", rel.Bloqueios)
	}
}

func TestPastaInexistente(t *testing.T) {
	if _, _, err := Ler(filepath.Join(t.TempDir(), "nao-existe")); err == nil {
		t.Fatal("queria erro para pasta inexistente")
	}
}
