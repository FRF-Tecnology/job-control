package curriculos

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	leitorpdf "github.com/ledongthuc/pdf"
)

// pdfDeTeste monta um PDF com uma página. colado=true desenha cada palavra no lugar, sem gravar o espaço (como
// Canva e alguns modelos fazem); colado=false escreve cada linha inteira, com espaços. A fonte tem larguras fixas
// (600/1000 do tamanho) para o leitor saber onde cada letra termina.
func pdfDeTeste(colado bool, linhas ...string) []byte {
	var pagina, b bytes.Buffer
	const tam, largura = 12.0, 7.2 // 600/1000 * 12
	for i, linha := range linhas {
		y := 780 - 18*i
		if !colado {
			fmt.Fprintf(&pagina, "BT /F1 12 Tf 50 %d Td (%s) Tj ET\n", y, linha)
			continue
		}
		// Um bloco por linha; cada palavra é movida para o lugar (Td relativo) e escrita sem o espaço.
		fmt.Fprintf(&pagina, "BT /F1 %.0f Tf 50 %d Td", tam, y)
		anterior := 0
		for i, palavra := range strings.Fields(linha) {
			if i > 0 {
				fmt.Fprintf(&pagina, " %.1f 0 Td", float64(anterior+1)*largura)
			}
			fmt.Fprintf(&pagina, " (%s) Tj", palavra)
			anterior = len(palavra)
		}
		pagina.WriteString(" ET\n")
	}
	larguras := strings.TrimSpace(strings.Repeat("600 ", 95))
	objetos := []string{
		"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Courier /FirstChar 32 /LastChar 126 /Widths [" + larguras + "] >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", pagina.Len(), pagina.String()),
	}
	b.WriteString("%PDF-1.4\n")
	var offsets []int
	for i, obj := range objetos {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objetos)+1)
	for _, n := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", n)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objetos)+1, xref)
	return b.Bytes()
}

var linhasDeCV = []string{
	"Fulana de Tal Exemplo",
	"fulana.exemplo@example.com",
	"Senior Java Developer with nine years of backend experience",
	"Built payment APIs with Spring Boot Kafka and PostgreSQL",
	"Migrated services to Kubernetes on AWS with Terraform",
	"Mentored junior engineers and led code reviews every week",
	"Fluent English and native Portuguese",
}

func TestTextoProfissionalRemontaEspacosDePDFColado(t *testing.T) {
	arquivo := pdfDeTeste(true, linhasDeCV...)
	// O texto simples deste PDF vem colado, como no currículo real.
	r, err := leitorpdf.NewReader(bytes.NewReader(arquivo), int64(len(arquivo)))
	if err != nil {
		t.Fatal(err)
	}
	if simples, _ := r.Page(1).GetPlainText(nil); !colado(simples) {
		t.Fatalf("o PDF de teste devia vir colado: %q", simples)
	}
	texto, err := TextoProfissional(context.Background(), arquivo)
	if err != nil {
		t.Fatal(err)
	}
	for _, trecho := range []string{"Senior Java Developer", "Spring Boot Kafka and PostgreSQL", "Fluent English"} {
		if !strings.Contains(texto, trecho) {
			t.Fatalf("faltou %q no texto remontado:\n%s", trecho, texto)
		}
	}
	// Com as linhas de volta, o filtro de contatos tira o e-mail.
	if strings.Contains(texto, "example.com") {
		t.Fatalf("o contato passou: %q", texto)
	}
}

func TestTextoProfissionalNaoMexeEmPDFComEspacos(t *testing.T) {
	arquivo := pdfDeTeste(false, linhasDeCV...)
	r, _ := leitorpdf.NewReader(bytes.NewReader(arquivo), int64(len(arquivo)))
	simples, _ := r.Page(1).GetPlainText(nil)
	if colado(simples) {
		t.Fatal("PDF com espaços tratado como colado")
	}
	texto, err := TextoProfissional(context.Background(), arquivo)
	if err != nil || !strings.Contains(texto, "Senior Java Developer with nine years") {
		t.Fatalf("%q %v", texto, err)
	}
}

func TestColado(t *testing.T) {
	prosa := strings.Repeat("Senior Java Developer with nine years of backend experience. ", 6)
	for texto, quer := range map[string]bool{
		prosa:                              false,
		strings.ReplaceAll(prosa, " ", ""): true,
		"CurtoSemEspacosMasPequenoDemais":  false, // menos de 200 caracteres: não vale a pena remontar
		"":                                 false,
	} {
		if colado(texto) != quer {
			t.Fatalf("colado(%.30q) = %v", texto, !quer)
		}
	}
}
