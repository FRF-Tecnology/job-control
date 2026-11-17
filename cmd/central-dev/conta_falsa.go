//go:build contafalsa

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/conta"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// Rotas de teste: só no central-dev com -tags contafalsa (nunca nos binários do app).
// rotaAvancar adianta o relógio da nuvem falsa e o da conta juntos e pede a licença de novo, para o teste de ponta a
// ponta ver o teste de 24 h acabar. rotaPagar faz o papel da pessoa que pagou no Checkout: a Falsa marca a conta como
// assinante e a licença é pedida de novo (o central-dev não abre o navegador).
const (
	rotaAvancar = "/api/v1/_teste/avancar"
	rotaPagar   = "/api/v1/_teste/pagar"
)

// maxHorasAvancar: até 400 dias por chamada.
const maxHorasAvancar = 24 * 400

// contaDoDev: a conta sobre a nuvem falsa, com uma chave gerada agora e um relógio compartilhado que a rota de teste
// adianta. O cofre é de memória (a sessão some ao fechar): o central-dev nunca guarda a conta no cofre do sistema.
func contaDoDev(pasta string) (*conta.Conta, func(http.Handler) http.Handler) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		falhar("chave da nuvem falsa", err)
	}
	var adiante atomic.Int64 // ns
	agora := func() time.Time { return time.Now().Add(time.Duration(adiante.Load())) }
	falsa := conta.NovaFalsa(priv, agora)
	c := conta.Nova(falsa, segredos.Memoria{}, pasta, pub, agora)
	log.Print("conta: nuvem falsa (contafalsa), código de confirmação " + conta.CodigoFalso)
	envolver := func(api http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != rotaAvancar && r.URL.Path != rotaPagar {
				api.ServeHTTP(w, r)
				return
			}
			if r.Method != http.MethodPost {
				responder(w, 405, map[string]any{"erro": "use POST"})
				return
			}
			if r.URL.Path == rotaPagar {
				s := c.Situacao()
				if !s.Entrada || s.Email == "" {
					responder(w, 409, map[string]any{"erro": "entre numa conta antes de pagar"})
					return
				}
				falsa.Assinante(s.Email)
				_ = c.AtualizarLicenca(r.Context())
				responder(w, 200, c.Situacao())
				return
			}
			var corpo struct {
				Horas *float64 `json:"horas"`
			}
			if err := json.NewDecoder(io.LimitReader(r.Body, 1<<10)).Decode(&corpo); err != nil || corpo.Horas == nil ||
				!(*corpo.Horas > 0 && *corpo.Horas <= maxHorasAvancar) {
				responder(w, 400, map[string]any{"erro": "envie {\"horas\": número de 0 a 9600}"})
				return
			}
			adiante.Add(int64(*corpo.Horas * float64(time.Hour)))
			_ = c.AtualizarLicenca(r.Context()) // sem conta ou sem internet: a situação já diz
			responder(w, 200, c.Situacao())
		})
	}
	return c, envolver
}

func responder(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(corpo)
}
