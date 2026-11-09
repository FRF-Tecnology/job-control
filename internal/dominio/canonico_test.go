package dominio

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// Exemplos da RFC 8785 (JCS): números no formato do ECMAScript, strings com escape mínimo e chaves em ordem UTF-16.
func TestJSONCanonicoRFC8785(t *testing.T) {
	// Seção 3.2.2 (exemplo completo).
	entrada := `{"numbers":[333333333.33333329,1E30,4.50,2e-3,0.000000000000000000000000001],"string":"\u20ac$\u000F\u000aA'\u0042\u0022\u005c\\\"\/","literals":[null,true,false]}`
	quer := `{"literals":[null,true,false],"numbers":[333333333.3333333,1e+30,4.5,0.002,1e-27],"string":"€$\u000f\nA'B\"\\\\\"/"}`
	var v any
	if err := json.Unmarshal([]byte(entrada), &v); err != nil {
		t.Fatal(err)
	}
	got, err := JSONCanonico(v)
	if err != nil || string(got) != quer {
		t.Fatalf("canônico =\n%s\nqueria\n%s (%v)", got, quer, err)
	}
	// Seção 3.2.3 (ordem das chaves por unidades UTF-16).
	ordem := `{"€":"Euro Sign","\r":"Carriage Return","דּ":"Hebrew Letter Dalet With Dagesh","1":"One","😀":"Emoji: Grinning Face","\u0080":"Control","ö":"Latin Small Letter O With Diaeresis"}`
	json.Unmarshal([]byte(ordem), &v)
	got, _ = JSONCanonico(v)
	chaves := []string{"\r", "1", "\u0080", "ö", "€", "\U0001F600", "דּ"}
	pos := -1
	for _, k := range chaves {
		b, _ := json.Marshal(k)
		_ = b
		i := strings.Index(string(got), `"`+strings.ReplaceAll(k, "\r", `\r`)+`":`)
		if i <= pos {
			t.Fatalf("ordem errada em %q: %s", k, got)
		}
		pos = i
	}
}

// Números (Apêndice B da RFC 8785, os que cabem em float64).
func TestNumeroES(t *testing.T) {
	casos := map[float64]string{
		0: "0", math.Copysign(0, -1): "0", 1: "1", -1: "-1", 4.5: "4.5", 0.002: "0.002", 1e30: "1e+30", 1e-27: "1e-27",
		5e-324: "5e-324", 1.7976931348623157e308: "1.7976931348623157e+308", 9007199254740992: "9007199254740992",
		1e21: "1e+21", 1e20: "100000000000000000000", 0.000001: "0.000001", 1e-7: "1e-7", 333333333.3333333: "333333333.3333333",
		-0.5: "-0.5", 5000: "5000", 123.456: "123.456",
	}
	for f, quer := range casos {
		if got, err := numeroES(f); err != nil || got != quer {
			t.Errorf("numeroES(%v) = %q (%v); queria %q", f, got, err, quer)
		}
	}
	for _, f := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, err := numeroES(f); err == nil {
			t.Errorf("numeroES(%v) deveria falhar", f)
		}
	}
}

// A impressão muda com qualquer mudança no pacote e não depende da ordem das chaves.
func TestImpressaoCandidatura(t *testing.T) {
	c1 := Objeto{"idioma": "pt", "curriculo": Objeto{"arquivo": "cv.pdf", "sha256": strings.Repeat("a", 64)}, "respostas": []any{}}
	c2 := Objeto{"respostas": []any{}, "curriculo": Objeto{"sha256": strings.Repeat("a", 64), "arquivo": "cv.pdf"}, "idioma": "pt"}
	a, err := ImpressaoCandidatura("li-1", "linkedin", "https://www.linkedin.com/jobs/view/1", c1)
	b, _ := ImpressaoCandidatura("li-1", "linkedin", "https://www.linkedin.com/jobs/view/1", c2)
	if err != nil || a != b || !strings.HasPrefix(a, "sha256:v1:") || len(a) != len("sha256:v1:")+64 {
		t.Fatalf("impressões %q e %q (%v)", a, b, err)
	}
	c3 := Objeto{"idioma": "en", "curriculo": c1["curriculo"], "respostas": []any{}}
	if d, _ := ImpressaoCandidatura("li-1", "linkedin", "https://www.linkedin.com/jobs/view/1", c3); d == a {
		t.Fatal("mudar o conteúdo não mudou a impressão")
	}
	if d, _ := ImpressaoCandidatura("li-2", "linkedin", "https://www.linkedin.com/jobs/view/1", c1); d == a {
		t.Fatal("mudar a vaga não mudou a impressão")
	}
}
