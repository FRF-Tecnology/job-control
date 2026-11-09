package dominio

import (
	"strings"
	"testing"
)

// jc-a2y.7.2, parte 3a: a estrutura do Google Forms vem do FB_PUBLIC_LOAD_DATA_ que a própria página carrega. O
// formato aqui é o PÚBLICO (descrito fora do Google), não conferido contra um formulário real (decisão do PO, msg
// 2a16f8ef): os testes usam um JSON fictício nesse formato. Tudo é dado não confiável: tamanho limitado, tipos de
// uma lista, e o resto validado pelo GravarEstruturaFormulario.

// googleFicticio monta um FB_PUBLIC_LOAD_DATA_ fictício: [null, [descrição, [itens...]], "/forms", "Título"].
func googleFicticio(itens string) string {
	return `[null,["Descrição fictícia",[` + itens + `]],"/forms","Formulário fictício"]`
}

func TestEstruturaDoGoogle(t *testing.T) {
	bruto := googleFicticio(`
		[111,"Qual o seu e-mail?",null,0,[[1001,null,1]]],
		[112,"Conte sobre você",null,1,[[1002,null,0]]],
		[113,"Nível de inglês",null,2,[[1003,[["Básico"],["Intermediário"],["Avançado"]],1]]],
		[114,"Tecnologias",null,4,[[1004,[["Go"],["React"]],0]]],
		[115,"Estado",null,3,[[1005,[["SP"],["RJ"]],0]]],
		[116,"Disponível a partir de",null,9,[[1006,null,0]]],
		[117,"Texto solto sem resposta",null,6,null],
		[118,"Uma imagem",null,11,null]`)
	perguntas, bloqueio, err := EstruturaDoGoogle(bruto)
	if err != nil || bloqueio != "" {
		t.Fatalf("leitura = %v %q", err, bloqueio)
	}
	if len(perguntas) != 6 {
		t.Fatalf("perguntas = %d: %+v", len(perguntas), perguntas)
	}
	quer := []struct {
		id, tipo    string
		obrigatoria bool
		opcoes      string
	}{
		{"entry.1001", "texto-curto", true, ""},
		{"entry.1002", "texto-longo", false, ""},
		{"entry.1003", "escolha-unica", true, "Básico,Intermediário,Avançado"},
		{"entry.1004", "multipla-escolha", false, "Go,React"},
		{"entry.1005", "lista", false, "SP,RJ"},
		{"entry.1006", "data", false, ""},
	}
	for i, q := range quer {
		p := perguntas[i]
		if p.ID != q.id || p.Tipo != q.tipo || p.Obrigatoria != q.obrigatoria || strings.Join(p.Opcoes, ",") != q.opcoes {
			t.Errorf("pergunta %d = %+v; queria %+v", i, p, q)
		}
	}
	// A estrutura lida passa no domínio (os mesmos limites do contrato).
	if erros := validarPerguntas(perguntas); len(erros) > 0 {
		t.Fatalf("o domínio recusou a leitura: %v", erros)
	}
}

func TestEstruturaDoGoogleBloqueios(t *testing.T) {
	for nome, c := range map[string]struct {
		itens, quer string
	}{
		"várias seções (decisão do PO)": {`[1,"Nome",null,0,[[10,null,1]]],[2,"Seção 2",null,8,null],[3,"Cidade",null,0,[[11,null,1]]]`, "varias-etapas"},
		"upload de arquivo":             {`[1,"Envie o seu portfólio",null,13,[[10,null,1]]]`, "outro"},
		"grade":                         {`[1,"Avalie",null,7,[[10,[["1"],["2"]],1],[11,[["1"],["2"]],1]]]`, "outro"},
		"tipo desconhecido":             {`[1,"X",null,99,[[10,null,1]]]`, "outro"},
	} {
		_, bloqueio, err := EstruturaDoGoogle(googleFicticio(c.itens))
		if err != nil || bloqueio != c.quer {
			t.Errorf("%s: bloqueio %q, err %v; queria %q", nome, bloqueio, err, c.quer)
		}
	}
	for nome, bruto := range map[string]string{
		"não é JSON":              "<script>alert(1)</script>",
		"sem itens":               `[null,["d",[]]]`,
		"forma errada":            `{"a":1}`,
		"grande demais":           `[null,["` + strings.Repeat("x", MaxEstruturaBruta) + `",[[1,"Nome",null,0,[[10,null,1]]]]]]`, // bom, salvo o tamanho
		"só itens sem resposta":   googleFicticio(`[1,"Texto solto",null,6,null],[2,"Imagem",null,11,null]`),
		"duas entradas num texto": googleFicticio(`[1,"Nome",null,0,[[10,null,1],[11,null,1]]]`),
		"id de entrada zero":      googleFicticio(`[1,"Nome",null,0,[[0,null,1]]]`),
		"rótulo não é texto":      googleFicticio(`[1,"Nível",null,2,[[10,[[5]],1]]]`),
		"título não é texto":      googleFicticio(`[1,{"x":1},null,0,[[10,null,1]]]`),
		"entrada sem id":          googleFicticio(`[1,"Nome",null,0,[[null,null,1]]]`),
		"item sem entradas":       googleFicticio(`[1,"Nome",null,0]`),
		"item curto":              googleFicticio(`[1,"Nome"]`),
		"opção não é lista":       googleFicticio(`[1,"Nome",null,2,[[10,["x"],1]]]`),
	} {
		if _, _, err := EstruturaDoGoogle(bruto); err == nil {
			t.Errorf("%s: aceitou", nome)
		}
	}
}
