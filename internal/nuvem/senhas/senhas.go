// Pacote senhas valida e guarda senhas de contas (Argon2id, formato PHC).
package senhas

import (
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

const (
	minRunas = 10
	maxRunas = 128

	memoriaKiB = 19456 // 19 MiB
	passadas   = 2
	fios       = 1
	tamSal     = 16
	tamHash    = 32
)

var (
	ErrCurta = errors.New("senha curta: mínimo de 10 caracteres")
	ErrLonga = errors.New("senha longa: máximo de 128 caracteres")
	ErrComum = errors.New("senha comum demais")
)

// comuns.txt: SecLists, Passwords/Common-Credentials/10k-most-common.txt
// (danielmiessler/SecLists, licença MIT).
//
//go:embed comuns.txt
var comunsBruto string

var (
	comunsUmaVez sync.Once
	comuns       map[string]struct{}
)

func carregarComuns() {
	comuns = make(map[string]struct{}, 10100)
	for _, l := range strings.Split(comunsBruto, "\n") {
		l = strings.ToLower(strings.TrimSpace(l))
		if l != "" {
			comuns[l] = struct{}{}
		}
	}
}

// Validar confere tamanho (em runas) e recusa senhas da lista de comuns.
func Validar(s string) error {
	n := utf8.RuneCountInString(s)
	if n < minRunas {
		return ErrCurta
	}
	if n > maxRunas {
		return ErrLonga
	}
	comunsUmaVez.Do(carregarComuns)
	if _, ok := comuns[strings.ToLower(s)]; ok {
		return ErrComum
	}
	return nil
}

// Resumir gera o resumo Argon2id no formato PHC, com sal aleatório.
func Resumir(s string) (string, error) {
	sal := make([]byte, tamSal)
	if _, err := rand.Read(sal); err != nil {
		return "", fmt.Errorf("gerar sal: %w", err)
	}
	h := argon2.IDKey([]byte(s), sal, passadas, memoriaKiB, fios, tamHash)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memoriaKiB, passadas, fios,
		base64.RawStdEncoding.EncodeToString(sal),
		base64.RawStdEncoding.EncodeToString(h)), nil
}

// Conferir compara a senha com o resumo em tempo constante, usando os
// parâmetros gravados no próprio resumo.
func Conferir(resumo, s string) (bool, error) {
	p := strings.Split(resumo, "$")
	if len(p) != 6 || p[0] != "" || p[1] != "argon2id" {
		return false, errors.New("resumo de senha inválido")
	}
	var versao int
	if _, err := fmt.Sscanf(p[2], "v=%d", &versao); err != nil || versao != argon2.Version {
		return false, errors.New("versão do resumo não suportada")
	}
	var m, t uint32
	var par uint8
	if _, err := fmt.Sscanf(p[3], "m=%d,t=%d,p=%d", &m, &t, &par); err != nil || m == 0 || t == 0 || par == 0 {
		return false, errors.New("parâmetros do resumo inválidos")
	}
	sal, err := base64.RawStdEncoding.DecodeString(p[4])
	if err != nil {
		return false, errors.New("sal do resumo inválido")
	}
	esperado, err := base64.RawStdEncoding.DecodeString(p[5])
	if err != nil || len(esperado) == 0 {
		return false, errors.New("hash do resumo inválido")
	}
	h := argon2.IDKey([]byte(s), sal, t, m, par, uint32(len(esperado)))
	return subtle.ConstantTimeCompare(h, esperado) == 1, nil
}
