package nuvem

import (
	"net/http"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/nuvem/sessoes"
)

// rotasNotas registra GET /v1/notas: as notas fiscais da conta (mais novas primeiro).
func (s *Servidor) rotasNotas() {
	s.mux.Handle("GET /v1/notas", sessoes.Middleware(s.sessoes, s.notas))
}

type notaJSON struct {
	Numero   string `json:"numero"`
	LinkPDF  string `json:"link_pdf"`
	Estado   string `json:"estado"`
	CriadaEm string `json:"criada_em"`
}

func (s *Servidor) notas(w http.ResponseWriter, r *http.Request, usuarioID string) {
	linhas, err := s.dep.Banco.Query(r.Context(),
		`select numero, link_pdf, estado, criada_em from notas where usuario_id = $1 order by criada_em desc, invoice_id`, usuarioID)
	if err != nil {
		s.interno(w, r, "notas", err)
		return
	}
	defer linhas.Close()
	lista := []notaJSON{}
	for linhas.Next() {
		var n notaJSON
		var criada time.Time
		if err := linhas.Scan(&n.Numero, &n.LinkPDF, &n.Estado, &criada); err != nil {
			s.interno(w, r, "notas", err)
			return
		}
		n.CriadaEm = criada.UTC().Format(time.RFC3339)
		lista = append(lista, n)
	}
	if err := linhas.Err(); err != nil {
		s.interno(w, r, "notas", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	escreverJSON(w, http.StatusOK, lista)
}
