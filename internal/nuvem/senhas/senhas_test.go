package senhas

import (
	"errors"
	"strings"
	"testing"
)

func TestValidar(t *testing.T) {
	casos := []struct {
		nome string
		in   string
		want error
	}{
		{"curta", "123456789", ErrCurta},
		{"curta com acentos conta runas", "áéíóúâêô", ErrCurta},
		{"dez runas acentuadas", "áéíóúâêôãõ", nil},
		{"longa", strings.Repeat("a", 129), ErrLonga},
		{"limite 128", strings.Repeat("a", 128), nil},
		{"comum", "1234567890", ErrComum},
		{"comum em maiúsculas", "QWERTYUIOP", ErrComum},
		{"boa", "cavalo bateria grampo", nil},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := Validar(c.in); !errors.Is(got, c.want) {
				t.Fatalf("Validar(%q) = %v, esperado %v", c.in, got, c.want)
			}
		})
	}
}

func TestResumirEConferir(t *testing.T) {
	resumo, err := Resumir("cavalo bateria grampo")
	if err != nil {
		t.Fatal(err)
	}
	ok, err := Conferir(resumo, "cavalo bateria grampo")
	if err != nil || !ok {
		t.Fatalf("Conferir = %v, %v", ok, err)
	}
	outro, _ := Resumir("cavalo bateria grampo")
	if outro == resumo {
		t.Fatal("o sal deve variar entre resumos")
	}
}

func TestConferirSenhaErrada(t *testing.T) {
	resumo, _ := Resumir("cavalo bateria grampo")
	ok, err := Conferir(resumo, "cavalo bateria grampos")
	if err != nil || ok {
		t.Fatalf("Conferir = %v, %v; esperado false, nil", ok, err)
	}
	if _, err := Conferir("lixo", "x"); err == nil {
		t.Fatal("resumo inválido deveria dar erro")
	}
}

func TestResumoTemParametros(t *testing.T) {
	resumo, err := Resumir("cavalo bateria grampo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(resumo, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("prefixo inesperado: %s", resumo)
	}
	if n := len(strings.Split(resumo, "$")); n != 6 {
		t.Fatalf("partes = %d, esperado 6", n)
	}
}
