package nuvem

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FRF-Tecnology/job-control/internal/nuvem/codigos"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/contas"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/emails"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/google"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/limites"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/pagamento"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/senhas"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/sessoes"
)

const (
	limiteCorpo        = 64 << 10 // 64 KiB por rota
	limiteCorpoWebhook = 1 << 20  // 1 MiB: o webhook do Stripe pode ser maior
	caminhoWebhook     = "/v1/stripe/webhook"
)

// Dependencias são as peças de fora que o servidor usa; os testes trocam por falsas.
// Notas agenda a nota fiscal de cada fatura paga (Task 10 implementa).
type Dependencias struct {
	Banco     *pgxpool.Pool
	Agora     func() time.Time // nil: time.Now
	Aleatorio io.Reader        // nil: crypto/rand.Reader
	Emails    emails.Enviador
	Google    google.Trocador
	Stripe    pagamento.Stripe
	Notas     AgendadorNotas
	Log       *slog.Logger // nil: slog.Default()
}

// Servidor atende a API da nuvem.
type Servidor struct {
	cfg Config
	dep Dependencias
	mux *http.ServeMux
	log *slog.Logger

	chave    ed25519.PrivateKey
	contas   *contas.Repo
	codigos  *codigos.Repo
	sessoes  *sessoes.Repo
	porIP    *limites.Janela
	porEmail *limites.Janela
	// semConta conta senhas erradas de emails sem conta (ou só Google), com a mesma regra do banco:
	// assim o bloqueio na 11ª tentativa não revela se a conta existe.
	semConta    *limites.Seguidas
	resumoFalso string         // conferido quando o email não tem senha: mesmo custo de quem tem
	envios      sync.WaitGroup // emails em andamento (os testes esperam)
}

// Novo monta o servidor com as rotas. As rotas de conta só existem com banco; a chave vem de LerConfig,
// que já a validou, então chave inválida aqui é erro de programação.
func Novo(cfg Config, dep Dependencias) *Servidor {
	if dep.Agora == nil {
		dep.Agora = time.Now
	}
	if dep.Aleatorio == nil {
		dep.Aleatorio = rand.Reader
	}
	if dep.Log == nil {
		dep.Log = slog.Default()
	}
	s := &Servidor{cfg: cfg, dep: dep, mux: http.NewServeMux(), log: dep.Log}
	s.mux.HandleFunc("GET /v1/saude", s.saude)
	if dep.Banco != nil {
		priv, err := cfg.Chave()
		if err != nil {
			panic(err)
		}
		if dep.Emails == nil {
			panic("nuvem: Dependencias.Emails é obrigatório com banco")
		}
		if dep.Stripe == nil || dep.Notas == nil {
			panic("nuvem: Dependencias.Stripe e Notas são obrigatórios com banco")
		}
		if dep.Google == nil {
			panic("nuvem: Dependencias.Google é obrigatório com banco")
		}
		s.chave = priv
		s.contas = contas.Novo(dep.Banco)
		s.codigos = codigos.Novo(dep.Banco, []byte(cfg.SegredoCodigos), dep.Agora, dep.Aleatorio)
		s.sessoes = sessoes.Novo(dep.Banco, priv, dep.Agora, dep.Aleatorio)
		s.porIP = limites.Nova(limitePorIP, time.Minute, dep.Agora)
		s.porEmail = limites.Nova(limitePorEmail, 15*time.Minute, dep.Agora)
		s.semConta = limites.NovaSeguidas(contas.MaxFalhas, contas.Bloqueio, capacidadeSemConta, dep.Agora)
		if s.resumoFalso, err = senhas.Resumir("applyer-conta-que-nao-existe"); err != nil {
			panic(err)
		}
		s.rotasContas()
		s.rotasLicenca()
		s.rotasPagamento()
		s.rotasNotas()
	}
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

// Esperar aguarda os emails em andamento (ou o fim do ctx). O main chama depois do Shutdown.
func (s *Servidor) Esperar(ctx context.Context) {
	fim := make(chan struct{})
	go func() { s.envios.Wait(); close(fim) }()
	select {
	case <-fim:
	case <-ctx.Done():
	}
}

func (s *Servidor) saude(w http.ResponseWriter, _ *http.Request) {
	escreverJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func escreverJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
