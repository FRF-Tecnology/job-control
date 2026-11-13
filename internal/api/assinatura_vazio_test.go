package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// O perfil como o primeiro acesso grava: sem titulosEquivalentes nem cidadeTrabalho.
func perfilDoPrimeiroAcesso() Objeto {
	return Objeto{"cargo": "Desenvolvedor Backend", "paisTrabalho": "BR", "foco": "nacional", "ingles": "Intermediário",
		"stackPrincipal": []any{"Java", "Go"}, "filtros": Objeto{"niveis": []any{"Pleno"}, "modelo": []any{"Remoto"}, "pularSe": []any{}}}
}

func comCampos(p Objeto, extra Objeto) Objeto {
	novo := Objeto{}
	for k, v := range p {
		novo[k] = v
	}
	for k, v := range extra {
		novo[k] = v
	}
	return novo
}

func TestAssinaturaTrataVazioComoAusente(t *testing.T) {
	base := perfilDoPrimeiroAcesso()
	original := assinaturaDeBusca(base)
	// O Perfil e IA grava os campos vazios assim.
	for _, vazio := range []Objeto{
		{"titulosEquivalentes": []any{}, "cidadeTrabalho": ""},
		{"titulosEquivalentes": []string{}, "cidadeTrabalho": "   "},
		{"contratos": Objeto{}, "formacao": nil},
	} {
		if assinaturaDeBusca(comCampos(base, vazio)) != original {
			t.Fatalf("campo vazio mudou a assinatura: %v", vazio)
		}
	}
	// Mudança de verdade continua invalidando as notas.
	for _, mudanca := range []Objeto{
		{"titulosEquivalentes": []any{"Java Developer"}},
		{"cidadeTrabalho": "Recife"},
		{"experienciaAnos": 0}, // zero não é vazio
	} {
		if assinaturaDeBusca(comCampos(base, mudanca)) == original {
			t.Fatalf("mudança não invalidou as notas: %v", mudanca)
		}
	}
}

// Como a assinatura era calculada antes de tratar o vazio. Perfis sem campo vazio têm de dar a mesma assinatura,
// para ninguém perder as notas da IA ao atualizar o app.
func assinaturaAntiga(perfil Objeto) string {
	dados := Objeto{}
	for _, campo := range []string{"cargo", "titulosEquivalentes", "paisTrabalho", "cidadeTrabalho", "foco", "filtros", "contratos", "stackPrincipal", "stackSecundaria", "ingles", "pretensaoUSD", "pretensaoCLT", "pretensaoPJ", "experienciaAnos", "formacao", "cidade"} {
		dados[campo] = perfil[campo]
	}
	if faixa := perfil["faixaSalarial"]; faixa != nil {
		dados["faixaSalarial"] = faixa
	}
	b, _ := json.Marshal(dados)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

func TestAssinaturaSemCampoVazioContinuaIgualAAntiga(t *testing.T) {
	for _, p := range []Objeto{
		perfilDoPrimeiroAcesso(),
		comCampos(perfilDoPrimeiroAcesso(), Objeto{"titulosEquivalentes": []any{"Java Developer"}, "cidadeTrabalho": "Recife", "experienciaAnos": 5.0}),
		comCampos(perfilDoPrimeiroAcesso(), Objeto{"faixaSalarial": Objeto{"moeda": "BRL", "min": 8000.0}}),
	} {
		if assinaturaDeBusca(p) != assinaturaAntiga(p) {
			t.Fatalf("a assinatura de um perfil sem campo vazio mudou: %v", p)
		}
	}
}

// O bug: salvar o Perfil e IA pela primeira vez, sem mudar nada, mandava todas as vagas avaliadas de volta à triagem.
func TestSalvarPerfilSemMudarNadaMantemAsNotas(t *testing.T) {
	ctx := context.Background()
	arm, err := armazem.Abrir(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { arm.Fechar() })
	a := Nova(arm, Opcoes{Cofre: segredos.Memoria{}})
	perfil := perfilDoPrimeiroAcesso()
	if err := arm.Gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		e.Perfil = perfil
		e.Vagas = append(e.Vagas, Objeto{"id": "avaliada", "titulo": "Backend", "empresa": "Exemplo", "etapa": "descoberta",
			"aderencia": "B", "triagemBusca": "classificada", "triagemPerfil": assinaturaDeBusca(perfil)})
		return e, nil
	}); err != nil {
		t.Fatal(err)
	}
	salvar := func(corpo Objeto) {
		t.Helper()
		r, err := a.Chamar(ctx, Pedido{Metodo: "PATCH", Caminho: Prefixo + "/perfil?perfilBuscaId=original", Corpo: corpo, Autor: "eu"})
		if err != nil || r.Status != 200 {
			t.Fatalf("PATCH /perfil: %v %v", r, err)
		}
	}
	nota := func() any {
		t.Helper()
		e, err := arm.Carregar(ctx)
		if err != nil {
			t.Fatal(err)
		}
		return e.Vagas[0]["aderencia"]
	}

	// O que o Perfil e IA manda sem a pessoa mudar nada (mais e-mail e telefone, que não são critério de busca).
	salvar(Objeto{"cargo": "Desenvolvedor Backend", "titulosEquivalentes": []any{}, "cidadeTrabalho": "", "email": "f@exemplo.com", "telefone": "11 90000"})
	if nota() != "B" {
		t.Fatalf("salvar sem mudar o critério jogou fora a nota: %v", nota())
	}
	// Mudar o cargo continua mandando a vaga de volta para a triagem.
	salvar(Objeto{"cargo": "Engenheiro de Dados"})
	if nota() == "B" {
		t.Fatal("mudar o cargo não invalidou a nota")
	}
}
