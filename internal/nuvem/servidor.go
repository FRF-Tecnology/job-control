package nuvem

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	limiteCorpo        = 64 << 10 // 64 KiB por rota
	limiteCorpoWebhook = 1 << 20  // 1 MiB: o webhook do Stripe pode ser maior
	caminhoWebhook     = "/v1/stripe/webhook"
)

// Dependencias são as peças de fora que o servidor usa; os testes trocam por falsas.
// As tasks seguintes acrescentam e-mail, Google, Stripe e notas.
type Dependencias struct {
	Banco     *pgxpool.Pool
	Agora     func() time.Time
	Aleatorio io.Reader
}

// Servidor atende a API da nuvem.
type Servidor struct {
	cfg Config
	dep Dependencias
	mux *http.ServeMux
}

// Novo monta o servidor com as rotas.
func Novo(cfg Config, dep Dependencias) *Servidor {
	s := &Servidor{cfg: cfg, dep: dep, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /v1/saude", s.saude)
	return s
}

// Handler devolve o http.Handler com o limite de corpo aplicado a todas as rotas.
func (s *Servidor) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limite := int64(limiteCorpo)
		if r.URL.Path == caminhoWebhook {
			limite = limiteCorpoWebhook
		}
		r.Body = http.MaxBytesReader(w, r.Body, limite)
		s.mux.ServeHTTP(w, r)
	})
}

func (s *Servidor) saude(w http.ResponseWriter, _ *http.Request) {
	escreverJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func escreverJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
