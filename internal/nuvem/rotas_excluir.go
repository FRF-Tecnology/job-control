package nuvem

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/FRF-Tecnology/job-control/internal/nuvem/contas"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/pagamento"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/sessoes"
)

const (
	msgConfirmeEmail = "Confirme digitando o email da conta"
	msgCancelar      = "Não deu para cancelar a assinatura agora. Tente de novo."
)

// rotasExcluir registra DELETE /v1/contas/eu (LGPD: a pessoa apaga a própria conta pelo app).
func (s *Servidor) rotasExcluir() {
	s.mux.Handle("DELETE /v1/contas/eu", s.limitarIP(sessoes.Middleware(s.sessoes, s.excluirConta).ServeHTTP))
}

// semUsuario responde 401 quando o token ainda confere mas a conta já foi apagada (vale até 15 min).
func semUsuario(w http.ResponseWriter, err error) bool {
	if errors.Is(err, contas.ErrNaoEncontrado) {
		erro(w, http.StatusUnauthorized, msgSessao)
		return true
	}
	return false
}

// excluirConta apaga a conta. O Stripe vem primeiro: se não der para cancelar a assinatura, nada é apagado
// (senão a pessoa seguiria sendo cobrada sem conta). Depois, uma transação só apaga tudo.
// notas fica como está (obrigação fiscal) e computadores_teste só perde o dono: apagar a conta e criar
// outra não rende outro teste no mesmo computador.
func (s *Servidor) excluirConta(w http.ResponseWriter, r *http.Request, usuarioID string) {
	var p pedido
	if !lerPedido(w, r, &p) {
		return
	}
	ctx := r.Context()
	u, err := s.contas.PorID(ctx, usuarioID)
	if err != nil {
		if !semUsuario(w, err) {
			s.interno(w, r, "usuario", err)
		}
		return
	}
	if email, err := contas.NormalizarEmail(p.Email); err != nil || email != u.Email {
		erro(w, http.StatusBadRequest, msgConfirmeEmail)
		return
	}
	a, err := s.assinaturaParaCancelar(ctx, u.ID)
	if err != nil {
		s.interno(w, r, "assinatura", err)
		return
	}
	if a != "" {
		if err := s.dep.Stripe.Cancelar(ctx, a); err != nil {
			s.log.Error("falha no Stripe", "rota", r.URL.Path, "onde", "cancelar", "usuario", u.ID, "erro", pagamento.Resumo(err))
			erro(w, http.StatusBadGateway, msgCancelar)
			return
		}
	}
	if err := s.apagarConta(ctx, u.ID, u.Email); err != nil {
		s.interno(w, r, "excluir", err)
		return
	}
	s.log.Info("conta excluída", "usuario", u.ID)
	w.WriteHeader(http.StatusNoContent)
}

// assinaturaParaCancelar devolve o id da assinatura no Stripe se ela ainda pode cobrar ("" se não).
func (s *Servidor) assinaturaParaCancelar(ctx context.Context, usuarioID string) (string, error) {
	var id, estado string
	err := s.dep.Banco.QueryRow(ctx,
		`select stripe_assinatura, estado from assinaturas where usuario_id = $1::uuid`, usuarioID).Scan(&id, &estado)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	switch estado {
	case "active", "trialing", "past_due", "incomplete", "unpaid":
		return id, nil
	}
	return "", nil
}

func (s *Servidor) apagarConta(ctx context.Context, usuarioID, email string) error {
	tx, err := s.dep.Banco.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	// Trava a linha: uma renovação ou um teste em andamento espera (ou falha) em vez de recriar linhas filhas.
	if _, err := tx.Exec(ctx, `select 1 from usuarios where id = $1::uuid for update`, usuarioID); err != nil {
		return err
	}
	for _, c := range []struct{ sql, arg string }{
		{`delete from codigos where email = $1`, email},
		{`delete from sessoes where usuario_id = $1::uuid`, usuarioID},
		{`delete from assinaturas where usuario_id = $1::uuid`, usuarioID},
		{`update computadores_teste set usuario_id = null where usuario_id = $1::uuid`, usuarioID},
		{`delete from usuarios where id = $1::uuid`, usuarioID},
	} {
		if _, err := tx.Exec(ctx, c.sql, c.arg); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
