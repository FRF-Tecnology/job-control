package segredos

import (
	"errors"
	"strings"
	"testing"
)

// jc-a2y.63: o que pode sair do cofre é só o final da chave. Chaves fictícias.

func TestFinalSoOs4Ultimos(t *testing.T) {
	casos := map[string]string{
		"":                              "…",
		"abcd":                          "…", // curta demais: nem o final
		"abcde":                         "…bcde",
		"sk-ant-ficticia-0000-WXYZ":     "…WXYZ",
		"chave-ficticia-com-acento-çãé": "…-çãé",
	}
	for chave, quer := range casos {
		if got := Final(chave); got != quer {
			t.Errorf("Final(%q) = %q; queria %q", chave, got, quer)
		}
	}
	// Nunca o começo (o prefixo diz o provedor e parte da chave), nunca mais que 4 caracteres.
	chave := "sk-ant-ficticia-MARCADOR-1234"
	got := Final(chave)
	if strings.Contains(got, "sk-") || strings.Contains(got, "MARCADOR") || len([]rune(got)) != 5 {
		t.Errorf("Final = %q", got)
	}
}

func TestMemoria(t *testing.T) {
	m := Memoria{}
	if c, err := m.Ler("anthropic"); c != "" || err != nil {
		t.Fatalf("vazio = %q, %v", c, err)
	}
	m.Gravar("anthropic", "chave-ficticia-1")
	if c, _ := m.Ler("anthropic"); c != "chave-ficticia-1" {
		t.Fatalf("ler = %q", c)
	}
	m.Apagar("anthropic")
	if c, _ := m.Ler("anthropic"); c != "" {
		t.Fatalf("depois de apagar = %q", c)
	}
}

// A falha do cofre tem texto fixo: o texto do sistema (que poderia trazer o que foi gravado) não aparece, mas o
// erro original continua acessível para o log registrar o tipo.
func TestErroCofreSemTextoDoSistema(t *testing.T) {
	base := errors.New("detalhe do sistema com sk-ficticia-MARCADOR-9999 não encontrada")
	err := error(&ErroCofre{Acao: "gravar", Err: base})
	if strings.Contains(err.Error(), "MARCADOR") || strings.Contains(err.Error(), "encontrad") {
		t.Errorf("mensagem = %q", err.Error())
	}
	if !errors.Is(err, base) {
		t.Error("o erro original se perdeu (Unwrap)")
	}
}

// O cofre padrão usa o nome de sempre; os testes do cofre do sistema usam outro.
func TestNomeDoServico(t *testing.T) {
	if (Sistema{}).nome() != servico {
		t.Fatalf("padrão = %q", (Sistema{}).nome())
	}
	if (Sistema{Servico: "Applyer-teste-x"}).nome() != "Applyer-teste-x" {
		t.Fatal("o nome de teste não foi usado")
	}
}
