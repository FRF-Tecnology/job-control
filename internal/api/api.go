// Package api expõe a API local /api/v1 da Central. A janela do app, a CLI,
// o MCP local e as automações falam com ela. As rotas seguem as de servidor/api.mjs.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
)

// VersaoApp é a versão do app (preenchida no build com -ldflags).
var VersaoApp = "0.1.0-dev"

// Fonte é o que a API precisa do armazém. Cresce a cada fase da reescrita.
type Fonte interface {
	Versao(ctx context.Context) (int64, error)
}

// Novo monta o roteador da API.
func Novo(f Fonte) http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /api/v1/saude", func(w http.ResponseWriter, r *http.Request) {
		responder(w, http.StatusOK, map[string]any{"ok": true, "app": VersaoApp})
	})
	m.HandleFunc("GET /api/v1/versao", func(w http.ResponseWriter, r *http.Request) {
		v, err := f.Versao(r.Context())
		if err != nil {
			responder(w, http.StatusInternalServerError, map[string]any{"erro": "não consegui ler a versão dos dados"})
			return
		}
		responder(w, http.StatusOK, map[string]any{"versao": strconv.FormatInt(v, 10)})
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, padrao := m.Handler(r); padrao != "" {
			m.ServeHTTP(w, r)
			return
		}
		// Sem rota: o ServeMux já sabe se é 404 ou 405 (e o cabeçalho Allow); só trocamos o texto por JSON.
		cap := &capturaStatus{ResponseWriter: w}
		m.ServeHTTP(cap, r)
		msg := "rota não encontrada"
		if cap.status == http.StatusMethodNotAllowed {
			msg = "método não permitido nesta rota"
		}
		responder(w, cap.status, map[string]any{"erro": msg})
	})
}

// capturaStatus guarda o status e descarta o corpo em texto do ServeMux.
type capturaStatus struct {
	http.ResponseWriter
	status int
}

func (c *capturaStatus) WriteHeader(s int) { c.status = s }
func (c *capturaStatus) Write(b []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
	}
	return len(b), nil
}

func responder(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(corpo)
}
