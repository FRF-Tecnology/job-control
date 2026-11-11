//go:build contafalsa

package conta

import (
	"context"
	"errors"
	"testing"
)

const retornoLocal = "http://127.0.0.1:1/retorno"

// criar cria e confirma a conta com a senha "senha-forte-1" e devolve a sessão.
func criar(t *testing.T, f *Falsa, email, disp string) Par {
	t.Helper()
	ctx := context.Background()
	if err := f.Criar(ctx, email, "senha-forte-1", TermosPadrao); err != nil {
		t.Fatal(err)
	}
	p, err := f.Confirmar(ctx, email, CodigoFalso, "senha-forte-1", disp, "")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func querTermos(t *testing.T, err error, versao string) {
	t.Helper()
	var e *ErroNuvem
	if !errors.As(err, &e) || e.Status != 409 || e.Motivo != "termos" || e.Versao != versao || e.Msg != "Os termos mudaram" {
		t.Fatalf("queria 409 termos %q, veio %#v", versao, err)
	}
}

func TestFalsaTermosMudaram(t *testing.T) {
	f, _ := novaFalsaDeTeste(t)
	ctx := context.Background()
	criar(t, f, "ana@exemplo.com", dispA)
	f.MudarTermos("2026-12")
	_, err := f.Entrar(ctx, "ana@exemplo.com", "senha-forte-1", dispA, "")
	querTermos(t, err, "2026-12")
	_, err = f.Entrar(ctx, "ana@exemplo.com", "senha-forte-1", dispA, TermosPadrao)
	querTermos(t, err, "2026-12")
	if _, err := f.Entrar(ctx, "ana@exemplo.com", "senha-forte-1", dispA, "2026-12"); err != nil {
		t.Fatal(err)
	}
	// aceitou: não pede de novo
	if _, err := f.Entrar(ctx, "ana@exemplo.com", "senha-forte-1", dispA, ""); err != nil {
		t.Fatal(err)
	}
	// Criar com versão diferente da atual: 400
	err = f.Criar(ctx, "bia@exemplo.com", "senha-forte-1", TermosPadrao)
	var e *ErroNuvem
	if !errors.As(err, &e) || e.Status != 400 || e.Motivo != "termos" || e.Versao != "2026-12" {
		t.Fatalf("criar: %#v", err)
	}
	// Confirmar depois de mudar os termos: 409; com a atual, abre a sessão
	if err := f.Criar(ctx, "bia@exemplo.com", "senha-forte-1", "2026-12"); err != nil {
		t.Fatal(err)
	}
	f.MudarTermos("2027-01")
	_, err = f.Confirmar(ctx, "bia@exemplo.com", CodigoFalso, "senha-forte-1", dispB, "")
	querTermos(t, err, "2027-01")
	if _, err := f.Confirmar(ctx, "bia@exemplo.com", CodigoFalso, "senha-forte-1", dispB, "2027-01"); err != nil {
		t.Fatal(err)
	}
}

func TestFalsaDispositivoInvalido(t *testing.T) {
	f, _ := novaFalsaDeTeste(t)
	ctx := context.Background()
	p := criar(t, f, "ana@exemplo.com", dispA)
	for _, d := range []string{"", "abc", "A" + dispA[1:], dispA + "0"} {
		var e *ErroNuvem
		if _, err := f.Entrar(ctx, "ana@exemplo.com", "senha-forte-1", d, ""); !errors.As(err, &e) || e.Status != 400 {
			t.Fatalf("entrar %q: %v", d, err)
		}
		if _, err := f.Licenca(ctx, p.Acesso, d); !errors.As(err, &e) || e.Status != 400 {
			t.Fatalf("licenca %q: %v", d, err)
		}
		if _, err := f.Google(ctx, "x@exemplo.com", "v", retornoLocal, d, TermosPadrao); !errors.As(err, &e) || e.Status != 400 {
			t.Fatalf("google %q: %v", d, err)
		}
		if _, err := f.NovaSenha(ctx, "ana@exemplo.com", CodigoFalso, "outra-senha-1", d); !errors.As(err, &e) || e.Status != 400 {
			t.Fatalf("nova senha %q: %v", d, err)
		}
	}
	if err := f.Criar(ctx, "bia@exemplo.com", "senha-forte-1", TermosPadrao); err != nil {
		t.Fatal(err)
	}
	var e *ErroNuvem
	if _, err := f.Confirmar(ctx, "bia@exemplo.com", CodigoFalso, "senha-forte-1", "abc", ""); !errors.As(err, &e) || e.Status != 400 {
		t.Fatalf("confirmar: %v", err)
	}
}

func TestFalsaSenhaContaRunes(t *testing.T) {
	f, _ := novaFalsaDeTeste(t)
	ctx := context.Background()
	// 10 runes (mais de 10 bytes): vale
	if err := f.Criar(ctx, "ana@exemplo.com", "ççççççççç1", TermosPadrao); err != nil {
		t.Fatal(err)
	}
	if err := f.Criar(ctx, "bia@exemplo.com", "ççççççççç", TermosPadrao); err == nil {
		t.Fatal("9 runes devia falhar")
	}
}

func TestFalsaNovaSenhaRevogaSessoes(t *testing.T) {
	f, _ := novaFalsaDeTeste(t)
	ctx := context.Background()
	p := criar(t, f, "ana@exemplo.com", dispA)
	if err := f.Recuperar(ctx, "ana@exemplo.com"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.NovaSenha(ctx, "ana@exemplo.com", CodigoFalso, "curta", dispA); err == nil {
		t.Fatal("senha curta devia falhar")
	}
	p2, err := f.NovaSenha(ctx, "ana@exemplo.com", CodigoFalso, "outra-senha-1", dispA)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Renovar(ctx, p.Renovacao); err == nil {
		t.Fatal("a renovação antiga devia parar de valer")
	}
	if _, err := f.Renovar(ctx, p2.Renovacao); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Entrar(ctx, "ana@exemplo.com", "outra-senha-1", dispA, ""); err != nil {
		t.Fatal(err)
	}
}

func TestFalsaGoogleContaNovaPedeTermos(t *testing.T) {
	f, _ := novaFalsaDeTeste(t)
	ctx := context.Background()
	_, err := f.Google(ctx, "ana@exemplo.com", "v", retornoLocal, dispA, "")
	querTermos(t, err, TermosPadrao)
	p, err := f.Google(ctx, "ana@exemplo.com", "v", retornoLocal, dispA, TermosPadrao)
	if err != nil || p.Acesso == "" {
		t.Fatalf("google: %v", err)
	}
	// conta que já aceitou entra sem termos
	if _, err := f.Google(ctx, "ana@exemplo.com", "v", retornoLocal, dispA, ""); err != nil {
		t.Fatal(err)
	}
}
