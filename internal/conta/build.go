package conta

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// A nuvem do build: o ldflags (-X) da compilação preenche (app/build, a partir de APPLYER_NUVEM_URL e
// APPLYER_NUVEM_PUB). Ruling A11: sem os dois, a conta e a trava ficam desligadas, e o app, a CLI e o MCP funcionam
// como antes, sem conta.
var (
	EnderecoNuvem string
	ChavePublica  string // base64 da chave pública Ed25519 da nuvem (32 bytes)
)

// AvisoDesligada: a linha de log da partida quando o build não tem a nuvem.
const AvisoDesligada = "conta desligada: build sem nuvem configurada"

// MotivoLicencaVencida: a CLI ou o MCP acharam licenca.json vencido ou ilegível. Eles não renovam: só o app aberto.
const MotivoLicencaVencida = "licenca-vencida"

// ChaveDoBuild: a chave pública da nuvem do build. ligada false (sem erro) quando faltam o endereço E a chave; erro
// quando só um dos dois veio (build pela metade: nunca destravado) ou a chave não é base64 de uma chave Ed25519.
func ChaveDoBuild() (pub ed25519.PublicKey, ligada bool, err error) {
	switch {
	case EnderecoNuvem == "" && ChavePublica == "":
		return nil, false, nil
	case ChavePublica == "":
		return nil, false, errors.New("build com a nuvem pela metade: falta ChavePublica")
	case EnderecoNuvem == "":
		return nil, false, errors.New("build com a nuvem pela metade: falta EnderecoNuvem")
	}
	b, err := base64.StdEncoding.DecodeString(ChavePublica)
	if err != nil || len(b) != ed25519.PublicKeySize {
		return nil, false, errors.New("a chave pública da nuvem no build é inválida")
	}
	return ed25519.PublicKey(b), true, nil
}

// DoBuild monta a conta do app sobre a nuvem do build (sem iniciar). nil, nil quando o build não tem a nuvem.
func DoBuild(cofre segredos.Cofre, pasta string) (*Conta, error) {
	pub, ligada, err := ChaveDoBuild()
	if err != nil || !ligada {
		return nil, err
	}
	n, err := NovaHTTP(EnderecoNuvem, nil)
	if err != nil {
		return nil, err
	}
	return Nova(n, cofre, pasta, pub, time.Now), nil
}

// TravaDoDisco é a trava da CLI e do MCP: a cada chamada lê licenca.json da pasta (sem rede, nunca renova). Sem
// arquivo: sem-conta. Vencido, editado ou ilegível: licenca-vencida (abrir o app traz uma nova). Bloqueado:
// sem-assinatura se já houve período pago, senão teste-acabou (como Conta.Situacao).
func TravaDoDisco(pasta string, pub ed25519.PublicKey, agora func() time.Time) func() (bool, string) {
	return func() (bool, string) {
		l, err := LerLicencaDoDisco(pasta, pub, agora())
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return false, MotivoSemConta
		case err != nil:
			return false, MotivoLicencaVencida
		case l.PodeAgir():
			return true, ""
		case l.PeriodoFim != nil:
			return false, MotivoSemAssinatura
		default:
			return false, MotivoTesteAcabou
		}
	}
}

// TravaDoBuild: TravaDoDisco com a chave do build e o relógio do PC. Build sem a nuvem: nil (sem trava, ruling A11),
// a não ser que a pasta já tenha licenca.json (ruling A15, falha fechada): a instalação tem conta e este build não
// consegue conferir, então trava com licenca-vencida.
func TravaDoBuild(pasta string) (func() (bool, string), error) {
	pub, ligada, err := ChaveDoBuild()
	if err != nil {
		return nil, err
	}
	if !ligada {
		if _, err := os.Lstat(filepath.Join(pasta, arquivoLicencaNome)); errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return func() (bool, string) { return false, MotivoLicencaVencida }, nil
	}
	return TravaDoDisco(pasta, pub, time.Now), nil
}
