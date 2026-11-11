package conta

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/licenca"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// trocarBuild põe valores nas variáveis do build (como o ldflags -X faria) e devolve no fim do teste.
func trocarBuild(t *testing.T, endereco, chave string) {
	t.Helper()
	e, c := EnderecoNuvem, ChavePublica
	EnderecoNuvem, ChavePublica = endereco, chave
	t.Cleanup(func() { EnderecoNuvem, ChavePublica = e, c })
}

// Ruling A11: sem endereço E chave no build, a conta fica desligada (sem erro); chave errada é erro.
func TestChaveDoBuild(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	b64 := base64.StdEncoding.EncodeToString(pub)
	for _, c := range []struct {
		nome, endereco, chave string
		ligada, erro          bool
	}{
		{"vazio", "", "", false, false},
		{"só endereço", "https://nuvem.exemplo.com", "", false, false},
		{"só chave", "", b64, false, false},
		{"as duas", "https://nuvem.exemplo.com", b64, true, false},
		{"chave não é base64", "https://nuvem.exemplo.com", "%%%", false, true},
		{"chave curta", "https://nuvem.exemplo.com", base64.StdEncoding.EncodeToString([]byte("curta")), false, true},
	} {
		trocarBuild(t, c.endereco, c.chave)
		got, ligada, err := ChaveDoBuild()
		if ligada != c.ligada || (err != nil) != c.erro {
			t.Errorf("%s: ligada %v erro %v", c.nome, ligada, err)
		}
		if c.ligada && !got.Equal(pub) {
			t.Errorf("%s: chave diferente", c.nome)
		}
	}
}

func TestDoBuild(t *testing.T) {
	trocarBuild(t, "", "")
	if c, err := DoBuild(segredos.Memoria{}, t.TempDir()); c != nil || err != nil {
		t.Fatalf("sem nuvem: %v %v", c, err)
	}
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	trocarBuild(t, "https://nuvem.exemplo.com", base64.StdEncoding.EncodeToString(pub))
	c, err := DoBuild(segredos.Memoria{}, t.TempDir())
	if err != nil || c == nil {
		t.Fatalf("com nuvem: %v %v", c, err)
	}
	if pode, motivo := c.PodeAgir(); pode || motivo != MotivoSemConta {
		t.Fatalf("conta nova = %v %q", pode, motivo)
	}
	trocarBuild(t, "http://nuvem.exemplo.com", base64.StdEncoding.EncodeToString(pub)) // sem https
	if _, err := DoBuild(segredos.Memoria{}, t.TempDir()); err == nil {
		t.Fatal("aceitou endereço sem https")
	}
}

// A trava da CLI e do MCP lê licenca.json a cada chamada, sem rede, e nunca renova.
func TestTravaDoDisco(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	agora := emitidaTeste.Add(10 * time.Minute)
	gravar := func(t *testing.T, pasta string, l licenca.Licenca) {
		t.Helper()
		if l.EmitidaEm.IsZero() {
			l.EmitidaEm, l.ValidaAte = emitidaTeste, emitidaTeste.Add(30*time.Minute)
		}
		d, err := licenca.Assinar(priv, l)
		if err != nil {
			t.Fatal(err)
		}
		if err := GravarLicenca(pasta, d); err != nil {
			t.Fatal(err)
		}
	}
	fim := emitidaTeste.Add(-time.Hour)
	for _, c := range []struct {
		nome   string
		lic    *licenca.Licenca
		pode   bool
		motivo string
	}{
		{"sem arquivo", nil, false, MotivoSemConta},
		{"teste", &licenca.Licenca{Estado: licenca.Teste}, true, ""},
		{"assinante", &licenca.Licenca{Estado: licenca.Assinante}, true, ""},
		{"teste acabou", &licenca.Licenca{Estado: licenca.Bloqueado, TesteFim: &fim}, false, MotivoTesteAcabou},
		{"assinatura acabou", &licenca.Licenca{Estado: licenca.Bloqueado, PeriodoFim: &fim}, false, MotivoSemAssinatura},
		{"vencida", &licenca.Licenca{Estado: licenca.Assinante, EmitidaEm: emitidaTeste.Add(-time.Hour),
			ValidaAte: emitidaTeste.Add(-30 * time.Minute)}, false, MotivoLicencaVencida},
	} {
		pasta := t.TempDir()
		if c.lic != nil {
			gravar(t, pasta, *c.lic)
		}
		pode, motivo := TravaDoDisco(pasta, pub, func() time.Time { return agora })()
		if pode != c.pode || motivo != c.motivo {
			t.Errorf("%s = %v %q; queria %v %q", c.nome, pode, motivo, c.pode, c.motivo)
		}
	}
	// Arquivo editado (assinatura não confere): trava como vencida (só o app traz uma licença nova).
	pasta := t.TempDir()
	if err := os.WriteFile(filepath.Join(pasta, "licenca.json"), []byte(`{"documento":{"dados":"e30=","assinatura":"AA=="}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if pode, motivo := TravaDoDisco(pasta, pub, func() time.Time { return agora })(); pode || motivo != MotivoLicencaVencida {
		t.Errorf("editado = %v %q", pode, motivo)
	}
	// A cada chamada: a licença que chega depois já vale, sem montar a trava de novo.
	pasta = t.TempDir()
	trava := TravaDoDisco(pasta, pub, func() time.Time { return agora })
	if pode, _ := trava(); pode {
		t.Fatal("sem arquivo deixou agir")
	}
	gravar(t, pasta, licenca.Licenca{Estado: licenca.Teste})
	if pode, motivo := trava(); !pode {
		t.Fatalf("depois de gravar = %q", motivo)
	}
}

func TestTravaDoBuild(t *testing.T) {
	trocarBuild(t, "", "")
	if trava, err := TravaDoBuild(t.TempDir()); trava != nil || err != nil {
		t.Fatalf("sem nuvem: trava %v erro %v", trava != nil, err)
	}
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	trocarBuild(t, "https://nuvem.exemplo.com", base64.StdEncoding.EncodeToString(pub))
	trava, err := TravaDoBuild(t.TempDir())
	if err != nil || trava == nil {
		t.Fatalf("com nuvem: %v", err)
	}
	if pode, motivo := trava(); pode || motivo != MotivoSemConta {
		t.Fatalf("sem arquivo = %v %q", pode, motivo)
	}
}
