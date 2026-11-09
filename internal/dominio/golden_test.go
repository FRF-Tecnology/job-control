package dominio

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
	_ "time/tzdata" // o fuso de São Paulo precisa existir mesmo no CI
)

// Os casos de testdata/golden.json foram gravados rodando o domínio Node (nucleo/) com
// ferramentas/gerar-golden-dominio.mjs. Cada um precisa dar exatamente a mesma resposta no Go.
// Incluem todos os cenários de testes/nucleo.test.mjs e das regras de testes/emails.test.mjs.

type erroGolden struct {
	Tipo     string   `json:"tipo"`
	Mensagem string   `json:"mensagem"`
	Erros    []string `json:"erros"`
}

type casoGolden struct {
	Nome      string      `json:"nome"`
	Funcao    string      `json:"funcao"`
	Args      []any       `json:"args"`
	Resultado any         `json:"resultado"`
	Erro      *erroGolden `json:"erro"`
}

type arquivoGolden struct {
	Fuso   string       `json:"fuso"`
	Sufixo string       `json:"sufixo"`
	Casos  []casoGolden `json:"casos"`
}

func TestMain(m *testing.M) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		panic(err)
	}
	time.Local = loc
	os.Exit(m.Run())
}

func carregarGolden(t *testing.T) arquivoGolden {
	t.Helper()
	b, err := os.ReadFile("testdata/golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var g arquivoGolden
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatal(err)
	}
	if g.Fuso != "America/Sao_Paulo" {
		t.Fatalf("golden gravado no fuso %q", g.Fuso)
	}
	return g
}

// paraJSON passa um valor por JSON, deixando-o no mesmo formato que o golden.
func paraJSON(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var saida any
	json.Unmarshal(b, &saida)
	return saida
}

func converter[T any](t *testing.T, v any) T {
	t.Helper()
	var saida T
	b, _ := json.Marshal(v)
	if err := json.Unmarshal(b, &saida); err != nil {
		t.Fatalf("converter %T: %v", saida, err)
	}
	return saida
}

func arg(c casoGolden, i int) any {
	if i < len(c.Args) {
		return c.Args[i]
	}
	return nil
}

func executar(t *testing.T, c casoGolden, sufixo string) (any, error) {
	a := func(i int) any { return arg(c, i) }
	o := func(i int) Objeto { return obj(a(i)) }
	vagas := func(i int) []Objeto { return converter[[]Objeto](t, a(i)) }
	estado := func(i int) Estado { return converter[Estado](t, a(i)) }
	s := func(i int) string { return texto(a(i)) }
	ctx := func(i int) Ctx {
		x := o(i)
		dias, _ := numeroDe(x["diasFollowUp"])
		return Ctx{Autor: texto(x["autor"]), ContaID: texto(x["contaId"]), Agora: texto(x["agora"]), DiasFollowUp: int(dias), Texto: texto(x["texto"]), Aleatorio: func() string { return sufixo }}
	}
	com := func(chave string) func(Estado, Objeto, error) (any, error) {
		return func(e Estado, r Objeto, err error) (any, error) {
			if err != nil {
				return nil, err
			}
			return map[string]any{"estado": e, chave: r}, nil
		}
	}
	switch c.Funcao {
	case "normalizar":
		return Normalizar(a(0)), nil
	case "somarDias":
		d, _ := numeroDe(a(1))
		return SomarDias(s(0), int(d)), nil
	case "ehDataISO":
		return EhDataISO(a(0)), nil
	case "mercadoDe":
		return MercadoDe(o(0)), nil
	case "noFoco":
		return NoFoco(o(0), a(1)), nil
	case "foiEnviada":
		return FoiEnviada(o(0)), nil
	case "lerValor":
		if f, ok := LerValor(a(0)); ok {
			return f, nil
		}
		return nil, nil // NaN vira null no JSON
	case "contemDocumento":
		return ContemDocumento(a(0)), nil
	case "validarVaga":
		return ValidarVaga(a(0)), nil
	case "validarPerfil":
		return ValidarPerfil(a(0)), nil
	case "perfilSensivel":
		return PerfilSensivel(a(0)), nil
	case "contemDocumentoEm":
		return ContemDocumentoEm(a(0)), nil
	case "chaveEmpresa":
		return ChaveEmpresa(a(0)), nil
	case "chaveVaga":
		return ChaveVaga(a(0), a(1)), nil
	case "duplicatasPorTitulo":
		return DuplicatasPorTitulo(vagas(0), o(1), a(2)), nil
	case "acharDuplicata":
		return AcharDuplicata(vagas(0), o(1)), nil
	case "pendentesFollowUp":
		return PendentesFollowUp(vagas(0), s(1)), nil
	case "sugerirMensagem":
		return SugerirMensagem(o(0), o(1)), nil
	case "teveRetorno":
		return TeveRetorno(o(0)), nil
	case "resumo":
		return CalcularResumo(vagas(0), s(1)), nil
	case "funil":
		return Funil(vagas(0)), nil
	case "porPlataforma":
		return PorPlataforma(vagas(0)), nil
	case "porSemana":
		return PorSemana(vagas(0)), nil
	case "motivosPulo":
		return MotivosDePulo(vagas(0)), nil
	case "passaPreFiltro":
		return PassaPreFiltro(o(0)), nil
	case "acharVagaDoEmail":
		return AcharVagaDoEmail(vagas(0), o(1), a(2)), nil
	case "transicaoDoEmail":
		return TransicaoDoEmail(s(0), o(1), o(2)), nil
	case "dataDoEmail":
		return DataDoEmail(a(0), s(1)), nil
	case "estadoVazio":
		return EstadoVazio(), nil
	case "adicionarVaga":
		return com("vaga")(AdicionarVaga(estado(0), a(1), ctx(2), verdadeiro(o(3)["forcar"])))
	case "moverVaga":
		return com("vaga")(MoverVaga(estado(0), a(1), a(2), o(3), ctx(4)))
	case "atualizarVaga":
		return com("vaga")(AtualizarVaga(estado(0), a(1), o(2), ctx(3)))
	case "registrarEvento":
		return com("evento")(RegistrarEvento(estado(0), a(1), a(2), a(3), ctx(4)))
	case "followUpFeito":
		return com("vaga")(FollowUpFeito(estado(0), a(1), ctx(2)))
	case "adiarFollowUp":
		d, _ := numeroDe(a(2))
		return com("vaga")(AdiarFollowUp(estado(0), a(1), int(d), ctx(3)))
	case "criarPedido":
		return com("pedido")(CriarPedido(estado(0), a(1), ctx(2)))
	case "concluirPedido":
		return com("pedido")(ConcluirPedido(estado(0), a(1), a(2), ctx(3)))
	case "registrarEmail":
		return com("resultado")(RegistrarEmail(estado(0), obj(o(1)["email"]), obj(o(1)["classificacao"]), ctx(2)))
	case "ligarEmail":
		return com("resultado")(LigarEmail(estado(0), a(1), a(2), ctx(3)))
	case "descartarEmail":
		return com("resultado")(DescartarEmail(estado(0), a(1)))
	case "gerarHistoricoMd":
		return GerarHistoricoMd(estado(0), s(1)), nil
	}
	t.Fatalf("função sem porte no teste: %s", c.Funcao)
	return nil, nil
}

func tipoDoErro(err error) string {
	var v *ErroValidacao
	var d *ErroDuplicata
	var n *ErroNaoEncontrado
	switch {
	case errors.As(err, &v):
		return "ErroValidacao"
	case errors.As(err, &d):
		return "ErroDuplicata"
	case errors.As(err, &n):
		return "Error"
	}
	return fmt.Sprintf("%T", err)
}

func TestGoldenDoNode(t *testing.T) {
	g := carregarGolden(t)
	for _, c := range g.Casos {
		t.Run(c.Funcao+"/"+c.Nome, func(t *testing.T) {
			antes := paraJSON(t, c.Args)
			r, err := executar(t, c, g.Sufixo)
			if !reflect.DeepEqual(paraJSON(t, c.Args), antes) {
				t.Fatal("a função alterou a entrada")
			}
			if c.Erro != nil {
				if err == nil {
					t.Fatalf("queria erro %s %q; veio sucesso: %s", c.Erro.Tipo, c.Erro.Mensagem, resumir(t, r))
				}
				if tipoDoErro(err) != c.Erro.Tipo || err.Error() != c.Erro.Mensagem {
					t.Fatalf("erro = %s %q; queria %s %q", tipoDoErro(err), err.Error(), c.Erro.Tipo, c.Erro.Mensagem)
				}
				var v *ErroValidacao
				if errors.As(err, &v) && !reflect.DeepEqual(v.Erros, c.Erro.Erros) {
					t.Fatalf("erros = %q; queria %q", v.Erros, c.Erro.Erros)
				}
				return
			}
			if err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if got := paraJSON(t, r); !reflect.DeepEqual(got, c.Resultado) {
				t.Fatalf("diferente do Node:\n%s", diferenca(t, got, c.Resultado, ""))
			}
		})
	}
}

func resumir(t *testing.T, v any) string {
	b, _ := json.Marshal(v)
	if len(b) > 300 {
		b = append(b[:300], "…"...)
	}
	return string(b)
}

// diferenca mostra o primeiro caminho em que Go e Node discordam.
func diferenca(t *testing.T, go_, node any, caminho string) string {
	if reflect.DeepEqual(go_, node) {
		return ""
	}
	gm, ok1 := go_.(map[string]any)
	nm, ok2 := node.(map[string]any)
	if ok1 && ok2 {
		var difs []string
		for k := range nm {
			if d := diferenca(t, gm[k], nm[k], caminho+"."+k); d != "" {
				difs = append(difs, d)
			}
		}
		for k := range gm {
			if _, tem := nm[k]; !tem {
				difs = append(difs, fmt.Sprintf("%s.%s: só no Go = %s", caminho, k, resumir(t, gm[k])))
			}
		}
		return strings.Join(difs, "\n")
	}
	gl, ok1 := go_.([]any)
	nl, ok2 := node.([]any)
	if ok1 && ok2 && len(gl) == len(nl) {
		for i := range nl {
			if d := diferenca(t, gl[i], nl[i], fmt.Sprintf("%s[%d]", caminho, i)); d != "" {
				return d
			}
		}
	}
	return fmt.Sprintf("%s:\n  Go:   %s\n  Node: %s", caminho, resumir(t, go_), resumir(t, node))
}
