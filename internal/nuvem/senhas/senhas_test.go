package senhas

import (
	"errors"
	"strings"
	"testing"
	"time"
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

func TestConferirResumoHostil(t *testing.T) {
	sal := "c2Fsc2Fsc2Fsc2Fsc2E"                      // 16 bytes
	hash := "aGFzaGhhc2hoYXNoaGFzaGhhc2hoYXNoaGFzaGg" // 32 bytes
	monta := func(v, params, s, h string) string {
		return "$argon2id$" + v + "$" + params + "$" + s + "$" + h
	}
	casos := []struct{ nome, resumo string }{
		{"vazio", ""},
		{"memória enorme", monta("v=19", "m=4294967295,t=2,p=1", sal, hash)},
		{"memória acima do teto", monta("v=19", "m=1048577,t=2,p=1", sal, hash)},
		{"passadas enormes", monta("v=19", "m=19456,t=4294967295,p=1", sal, hash)},
		{"passadas acima do teto", monta("v=19", "m=19456,t=11,p=1", sal, hash)},
		{"fios enormes", monta("v=19", "m=19456,t=2,p=255", sal, hash)},
		{"fios acima do teto", monta("v=19", "m=19456,t=2,p=9", sal, hash)},
		{"versão errada", monta("v=16", "m=19456,t=2,p=1", sal, hash)},
		{"versão com lixo", monta("v=19x", "m=19456,t=2,p=1", sal, hash)},
		{"m zero", monta("v=19", "m=0,t=2,p=1", sal, hash)},
		{"t zero", monta("v=19", "m=19456,t=0,p=1", sal, hash)},
		{"p zero", monta("v=19", "m=19456,t=2,p=0", sal, hash)},
		{"não numérico", monta("v=19", "m=abc,t=2,p=1", sal, hash)},
		{"lixo no final", monta("v=19", "m=19456,t=2,p=1x", sal, hash)},
		{"lixo extra", monta("v=19", "m=19456,t=2,p=1,x=3", sal, hash)},
		{"sinal", monta("v=19", "m=+19456,t=2,p=1", sal, hash)},
		{"sal base64 ruim", monta("v=19", "m=19456,t=2,p=1", "!!!", hash)},
		{"hash base64 ruim", monta("v=19", "m=19456,t=2,p=1", sal, "!!!")},
		{"sal curto", monta("v=19", "m=19456,t=2,p=1", "c2Fs", hash)},
		{"hash curto", monta("v=19", "m=19456,t=2,p=1", sal, "aGFzaA")},
		{"hash longo demais", monta("v=19", "m=19456,t=2,p=1", sal, strings.Repeat("QUFB", 30))},
		{"outro algoritmo", "$argon2i$v=19$m=19456,t=2,p=1$" + sal + "$" + hash},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			ini := time.Now()
			ok, err := Conferir(c.resumo, "cavalo bateria grampo")
			if ok || err == nil {
				t.Fatalf("esperado (false, erro), veio (%v, %v)", ok, err)
			}
			if d := time.Since(ini); d > time.Second {
				t.Fatalf("demorou %v para recusar", d)
			}
		})
	}
}

// Sem vaga no semáforo, Resumir e Conferir esperam no máximo a espera e devolvem ErrOcupado.
func TestOcupadoSemVaga(t *testing.T) {
	r, err := Resumir("cavalo bateria grampo")
	if err != nil {
		t.Fatal(err)
	}
	defer TrocarLimite(0, 20*time.Millisecond)()
	inicio := time.Now()
	if _, err := Resumir("cavalo bateria grampo"); !errors.Is(err, ErrOcupado) {
		t.Fatalf("Resumir: %v", err)
	}
	if _, err := Conferir(r, "cavalo bateria grampo"); !errors.Is(err, ErrOcupado) {
		t.Fatalf("Conferir: %v", err)
	}
	if d := time.Since(inicio); d > 2*time.Second {
		t.Fatalf("esperou demais: %v", d)
	}
}

// Com uma vaga, pedidos simultâneos esperam a vez (dentro da espera) e nenhum roda junto com outro.
func TestOcupadoEsperaAVez(t *testing.T) {
	r, err := Resumir("cavalo bateria grampo")
	if err != nil {
		t.Fatal(err)
	}
	defer TrocarLimite(1, 10*time.Second)()
	erros := make(chan error, 4)
	for i := 0; i < 4; i++ {
		go func() {
			ok, err := Conferir(r, "cavalo bateria grampo")
			if err == nil && !ok {
				err = errors.New("senha certa não conferiu")
			}
			erros <- err
		}()
	}
	for i := 0; i < 4; i++ {
		if err := <-erros; err != nil {
			t.Fatal(err)
		}
	}
	if n := emUso(); n != 0 {
		t.Fatalf("vagas não devolvidas: %d", n)
	}
}

func TestCapacidadePadrao(t *testing.T) {
	if c := capacidadePadrao(); c < 2 {
		t.Fatalf("capacidade %d, mínimo 2", c)
	}
}
