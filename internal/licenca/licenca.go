// Package licenca define a licença assinada que a nuvem emite e o app confere.
// A nuvem assina com a chave privada Ed25519; o app só carrega a pública.
package licenca

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

// Estado é a situação da conta.
type Estado string

const (
	Teste     Estado = "teste"
	Assinante Estado = "assinante"
	Bloqueado Estado = "bloqueado"
)

var (
	// ErrAssinatura: o documento está malformado, alterado ou foi assinado por outra chave.
	ErrAssinatura = errors.New("licença com assinatura inválida")
	// ErrVencida: a licença é autêntica, mas passou de ValidaAte.
	ErrVencida = errors.New("licença vencida")
)

// Licenca é o conteúdo assinado.
type Licenca struct {
	Usuario    string     `json:"usuario"`
	Email      string     `json:"email"`
	Estado     Estado     `json:"estado"`
	TesteFim   *time.Time `json:"teste_fim"`
	PeriodoFim *time.Time `json:"periodo_fim"`
	EmitidaEm  time.Time  `json:"emitida_em"`
	ValidaAte  time.Time  `json:"valida_ate"`
}

// Documento é a licença como trafega: Dados é o JSON da Licenca em base64 e
// Assinatura é a assinatura Ed25519 dos bytes decodificados de Dados, em base64.
type Documento struct {
	Dados      string `json:"dados"`
	Assinatura string `json:"assinatura"`
}

// Assinar serializa e assina a licença.
func Assinar(priv ed25519.PrivateKey, l Licenca) (Documento, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return Documento{}, errors.New("chave privada inválida")
	}
	bruto, err := json.Marshal(l)
	if err != nil {
		return Documento{}, err
	}
	return Documento{
		Dados:      base64.StdEncoding.EncodeToString(bruto),
		Assinatura: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, bruto)),
	}, nil
}

// Conferir valida a assinatura e a validade em agora e devolve a licença.
func Conferir(pub ed25519.PublicKey, d Documento, agora time.Time) (Licenca, error) {
	if len(pub) != ed25519.PublicKeySize {
		return Licenca{}, ErrAssinatura
	}
	bruto, err := base64.StdEncoding.DecodeString(d.Dados)
	if err != nil {
		return Licenca{}, ErrAssinatura
	}
	assinatura, err := base64.StdEncoding.DecodeString(d.Assinatura)
	if err != nil || !ed25519.Verify(pub, bruto, assinatura) {
		return Licenca{}, ErrAssinatura
	}
	var l Licenca
	if err := json.Unmarshal(bruto, &l); err != nil {
		return Licenca{}, ErrAssinatura
	}
	if agora.After(l.ValidaAte) {
		return Licenca{}, ErrVencida
	}
	return l, nil
}

// PodeAgir diz se a conta pode usar os recursos que dependem da nuvem.
func (l Licenca) PodeAgir() bool {
	return l.Estado == Teste || l.Estado == Assinante
}
