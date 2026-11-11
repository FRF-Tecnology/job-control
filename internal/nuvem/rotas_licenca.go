package nuvem

import (
	"net/http"

	"github.com/FRF-Tecnology/job-control/internal/licenca"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/sessoes"
)

// rotasLicenca registra GET /v1/licenca?dispositivo=<hash>: o app chama a cada 10 min.
func (s *Servidor) rotasLicenca() {
	s.mux.Handle("GET /v1/licenca", sessoes.Middleware(s.sessoes, s.licenca))
}

func (s *Servidor) licenca(w http.ResponseWriter, r *http.Request, usuarioID string) {
	hash := r.URL.Query().Get("dispositivo")
	if !reDispositivo.MatchString(hash) {
		erro(w, http.StatusBadRequest, msgDispositivo)
		return
	}
	ctx := r.Context()
	agora := s.dep.Agora().UTC()
	u, err := s.contas.PorID(ctx, usuarioID)
	if err != nil {
		if !semUsuario(w, err) {
			s.interno(w, r, "usuario", err)
		}
		return
	}
	if u.Confirmado() && u.TesteInicio == nil && !u.Cortesia {
		if err := s.iniciarTeste(ctx, usuarioID, hash, agora); err != nil {
			if semUsuario(w, err) {
				return
			}
			s.interno(w, r, "teste", err)
			return
		}
		if u, err = s.contas.PorID(ctx, usuarioID); err != nil {
			s.interno(w, r, "usuario", err)
			return
		}
	}
	a, err := lerAssinatura(ctx, s.dep.Banco, usuarioID)
	if err != nil {
		s.interno(w, r, "assinatura", err)
		return
	}
	l := licenca.Licenca{
		Usuario:   u.ID,
		Email:     u.Email,
		Estado:    estadoDa(u, a, agora),
		TesteFim:  u.TesteFim,
		EmitidaEm: agora,
		ValidaAte: agora.Add(validadeLic),
	}
	if a != nil {
		l.PeriodoFim = a.PeriodoFim
	}
	doc, err := licenca.Assinar(s.chave, l)
	if err != nil {
		s.interno(w, r, "assinar", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	escreverJSON(w, http.StatusOK, doc)
}
