package dominio

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// Leitura do Google Forms (jc-a2y.7.2, parte 3a). A estrutura vem do FB_PUBLIC_LOAD_DATA_ que a própria página do
// formulário carrega; o navegador só lê esse valor (JSON.stringify), sem clicar nem digitar. O FORMATO É O PÚBLICO
// (descrito fora do Google) e não foi conferido contra um formulário real (decisão do PO): até termos uma amostra
// real, o que não bater com ele vira erro, e a leitura fica com a pessoa.
//
//	[_, [descrição, [item, ...]], ...]
//	item: [idDoItem, título, descrição, códigoDoTipo, [[idDaEntrada, opções|null, obrigatória(0|1)], ...]]
//	opções: [[rótulo, ...], ...]
//
// Tudo é dado não confiável: o bruto tem teto de tamanho, o tipo vem de uma lista fechada, e as perguntas passam
// depois pelos limites do GravarEstruturaFormulario.

// MaxEstruturaBruta: o maior JSON aceito da estrutura de um formulário (bytes).
const MaxEstruturaBruta = 2_000_000

// tiposGoogle: o código do tipo de item do Google e o tipo do contrato. Os itens sem resposta (texto solto, imagem,
// vídeo) não viram pergunta; os que só a pessoa faz bloqueiam.
var (
	tiposGoogle      = map[int64]string{0: "texto-curto", 1: "texto-longo", 2: "escolha-unica", 3: "lista", 4: "multipla-escolha", 5: "escala", 9: "data", 10: "outro"}
	itensSemResposta = map[int64]bool{6: true, 11: true, 12: true}
	bloqueiosGoogle  = map[int64]string{8: "varias-etapas", 7: "outro", 13: "outro"} // seção, grade, upload
)

var errFormatoGoogle = errors.New("formulário do Google fora do formato conhecido")

// EstruturaDoGoogle lê as perguntas do FB_PUBLIC_LOAD_DATA_ (em JSON). bloqueio: um motivo de PendenciaDoMotivo quando
// o formulário tem o que fica com a pessoa (várias seções, upload, grade, tipo desconhecido); err: fora do formato.
func EstruturaDoGoogle(bruto string) (perguntas []PerguntaFormulario, bloqueio string, err error) {
	if len(bruto) > MaxEstruturaBruta {
		return nil, "", errFormatoGoogle
	}
	dec := json.NewDecoder(strings.NewReader(bruto))
	dec.UseNumber()
	var raiz []any
	if err := dec.Decode(&raiz); err != nil || len(raiz) < 2 {
		return nil, "", errFormatoGoogle
	}
	corpo, ok := raiz[1].([]any)
	if !ok || len(corpo) < 2 {
		return nil, "", errFormatoGoogle
	}
	itens, ok := corpo[1].([]any)
	if !ok || len(itens) == 0 {
		return nil, "", errFormatoGoogle
	}
	for _, x := range itens {
		item, ok := x.([]any)
		if !ok || len(item) < 4 {
			return nil, "", errFormatoGoogle
		}
		tipo, ok := numeroInteiro(item[3])
		if !ok {
			return nil, "", errFormatoGoogle
		}
		if itensSemResposta[tipo] {
			continue
		}
		if m, bloqueia := bloqueiosGoogle[tipo]; bloqueia {
			return nil, m, nil
		}
		nosso, conhecido := tiposGoogle[tipo]
		if !conhecido {
			return nil, "outro", nil
		}
		titulo, ok := item[1].(string)
		if !ok {
			return nil, "", errFormatoGoogle
		}
		if len(item) < 5 {
			return nil, "", errFormatoGoogle
		}
		entradas, ok := item[4].([]any)
		if !ok || len(entradas) != 1 {
			return nil, "", errFormatoGoogle // uma pergunta, uma entrada (várias entradas: grade, que já bloqueou)
		}
		entrada, ok := entradas[0].([]any)
		if !ok || len(entrada) < 3 {
			return nil, "", errFormatoGoogle
		}
		idEntrada, ok := numeroInteiro(entrada[0])
		if !ok || idEntrada <= 0 {
			return nil, "", errFormatoGoogle
		}
		obrigatoria, _ := numeroInteiro(entrada[2])
		p := PerguntaFormulario{ID: "entry." + strconv.FormatInt(idEntrada, 10), Texto: titulo, Tipo: nosso, Obrigatoria: obrigatoria == 1}
		if ops, ok := entrada[1].([]any); ok {
			for _, o := range ops {
				op, ok := o.([]any)
				if !ok || len(op) == 0 {
					return nil, "", errFormatoGoogle
				}
				rotulo, ok := op[0].(string)
				if !ok {
					return nil, "", errFormatoGoogle
				}
				p.Opcoes = append(p.Opcoes, rotulo)
			}
		}
		perguntas = append(perguntas, p)
	}
	if len(perguntas) == 0 {
		return nil, "", errFormatoGoogle
	}
	return perguntas, "", nil
}

func numeroInteiro(v any) (int64, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	i, err := n.Int64()
	return i, err == nil
}
