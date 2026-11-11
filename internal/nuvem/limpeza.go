package nuvem

import (
	"context"
	"time"
)

const (
	intervaloLimpeza = time.Hour
	prazoPendente    = 7 * 24 * time.Hour // conta nunca confirmada some depois disso
	sobraCodigos     = 24 * time.Hour     // código vencido (e eventos de código) somem depois disso
)

// Limpar apaga as contas nunca confirmadas criadas há mais de 7 dias (com os códigos do email), os códigos
// vencidos há mais de 1 dia e o histórico de envios/erros de código com mais de 1 dia (já fora das janelas).
// Conta pendente não tem sessão, assinatura nem teste (tudo isso exige email confirmado).
func (s *Servidor) Limpar(ctx context.Context) error {
	agora := s.dep.Agora()
	tx, err := s.dep.Banco.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var pendentes int64
	if err := tx.QueryRow(ctx, `
		with apagados as (
			delete from usuarios where email_confirmado_em is null and criado_em < $1 returning email
		), seus_codigos as (
			delete from codigos where email in (select email from apagados)
		)
		select count(*) from apagados`, agora.Add(-prazoPendente)).Scan(&pendentes); err != nil {
		return err
	}
	cods, err := tx.Exec(ctx, `delete from codigos where expira_em < $1`, agora.Add(-sobraCodigos))
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `delete from eventos_codigo where em < $1`, agora.Add(-sobraCodigos)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	s.log.Info("limpeza feita", "pendentes_apagadas", pendentes, "codigos_vencidos", cods.RowsAffected())
	return nil
}

// RodarLimpeza chama Limpar agora e a cada hora, até o ctx acabar (o main cancela no desligamento).
func (s *Servidor) RodarLimpeza(ctx context.Context) {
	t := time.NewTicker(intervaloLimpeza)
	defer t.Stop()
	for {
		if err := s.Limpar(ctx); err != nil && ctx.Err() == nil {
			s.log.Error("falha na limpeza", "erro", err.Error())
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
