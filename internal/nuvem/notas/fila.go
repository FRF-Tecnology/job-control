package notas

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FRF-Tecnology/job-control/internal/nuvem/pagamento"
)

const (
	// MaxTentativas: depois disso a nota fica em "falhou" e a fila desiste (alerta no log).
	MaxTentativas = 24
	// porPassada limita as notas de uma passada, para uma fila grande não prender o servidor.
	porPassada = 20
	intervalo  = time.Hour
)

// Faturas busca os dados da fatura (o Stripe).
type Faturas interface {
	Fatura(ctx context.Context, id string) (pagamento.Fatura, error)
}

// erroFatura: a fatura não pôde ser lida; o texto vem de pagamento.Resumo (tipo, código e status).
type erroFatura string

func (e erroFatura) Error() string { return string(e) }

// Fila é a fila de notas no Postgres (tabela notas). Segura com duas instâncias: cada nota é trabalhada
// sob select ... for update skip locked.
type Fila struct {
	banco   *pgxpool.Pool
	faturas Faturas
	emissor Emissor
	log     *slog.Logger
	gatilho chan struct{}
	// Intervalo entre passadas de Rodar (1 h).
	Intervalo time.Duration
}

// NovaFila monta a fila. log nil usa slog.Default().
func NovaFila(banco *pgxpool.Pool, faturas Faturas, emissor Emissor, log *slog.Logger) *Fila {
	if log == nil {
		log = slog.Default()
	}
	return &Fila{banco: banco, faturas: faturas, emissor: emissor, log: log,
		gatilho: make(chan struct{}, 1), Intervalo: intervalo}
}

// Agendar registra a nota da fatura como pendente. Idempotente por invoiceID (o Stripe reenvia webhooks).
// O erro nunca leva dados pessoais.
func (f *Fila) Agendar(ctx context.Context, invoiceID, usuarioID string) error {
	_, err := f.banco.Exec(ctx,
		`insert into notas (invoice_id, usuario_id, estado) values ($1, $2, 'pendente') on conflict (invoice_id) do nothing`,
		invoiceID, usuarioID)
	if err != nil {
		return fmt.Errorf("notas: gravar agendamento: %w", err)
	}
	select { // acorda o Rodar sem bloquear
	case f.gatilho <- struct{}{}:
	default:
	}
	return nil
}

// Rodar processa a fila ao subir, a cada Intervalo e logo após cada Agendar, até o ctx acabar.
// Só a passada horária refaz notas que falharam; a da subida e a do gatilho tratam só as pendentes,
// para uma pane do emissor não queimar as 24 tentativas em minutos.
func (f *Fila) Rodar(ctx context.Context) {
	tic := time.NewTicker(f.Intervalo)
	defer tic.Stop()
	f.processar(ctx, false) // subida: só pendentes
	for {
		select {
		case <-ctx.Done():
			return
		case <-tic.C:
			f.processar(ctx, true)
		case <-f.gatilho:
			f.processar(ctx, false)
		}
	}
}

// Processar faz a passada horária: até porPassada notas pendentes ou falhas, uma tentativa cada, uma por vez.
func (f *Fila) Processar(ctx context.Context) { f.processar(ctx, true) }

// ProcessarNovas faz a passada do gatilho: só as pendentes (as que falharam esperam a passada horária).
func (f *Fila) ProcessarNovas(ctx context.Context) { f.processar(ctx, false) }

func (f *Fila) processar(ctx context.Context, refazer bool) {
	vistas := []string{}
	for i := 0; i < porPassada && ctx.Err() == nil; i++ {
		id, achou, err := f.proxima(ctx, vistas, refazer)
		if err != nil {
			if ctx.Err() == nil {
				f.log.Error("fila de notas falhou", "erro", fmt.Sprintf("%T", err))
			}
			return
		}
		if !achou {
			return
		}
		vistas = append(vistas, id)
	}
}

// proxima trabalha uma nota dentro de uma transação que segura a linha até gravar o resultado.
func (f *Fila) proxima(ctx context.Context, vistas []string, refazer bool) (string, bool, error) {
	tx, err := f.banco.Begin(ctx)
	if err != nil {
		return "", false, err
	}
	// A gravação final não pode ser cortada pelo desligamento: depois que o emissor aceitou, a linha tem de ficar emitida.
	fim, cancelar := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancelar()
	defer func() { _ = tx.Rollback(fim) }()
	var id, usuario string
	var tentativas int
	err = tx.QueryRow(ctx,
		`select invoice_id, coalesce(usuario_id::text, ''), tentativas from notas
		  where (estado = 'pendente' or ($3 and estado = 'falhou')) and tentativas < $1 and invoice_id <> all($2)
		  order by criada_em, invoice_id limit 1 for update skip locked`, MaxTentativas, vistas, refazer).
		Scan(&id, &usuario, &tentativas)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	numero, link, errEmitir := f.emitir(ctx, id, usuario)
	tentativas++
	if errEmitir == nil {
		_, err = tx.Exec(fim, `update notas set estado = 'emitida', numero = $2, link_pdf = $3, erro = '', tentativas = $4 where invoice_id = $1`,
			id, numero, link, tentativas)
	} else {
		if ctx.Err() != nil { // desligando: a tentativa não conta
			return id, true, ctx.Err()
		}
		msg := descrever(errEmitir)
		_, err = tx.Exec(fim, `update notas set estado = 'falhou', erro = $2, tentativas = $3 where invoice_id = $1`,
			id, msg, tentativas)
		f.log.Warn("nota fiscal falhou", "fatura", id, "tentativa", tentativas, "erro", msg)
		if tentativas >= MaxTentativas {
			f.log.Error("ALERTA: nota fiscal desistida depois de todas as tentativas", "fatura", id)
		}
	}
	if err != nil {
		return "", false, err
	}
	return id, true, tx.Commit(fim)
}

func (f *Fila) emitir(ctx context.Context, invoiceID, usuarioID string) (string, string, error) {
	fat, err := f.faturas.Fatura(ctx, invoiceID)
	if err != nil {
		return "", "", erroFatura("fatura: " + pagamento.Resumo(err))
	}
	return f.emissor.Emitir(ctx, Pedido{InvoiceID: invoiceID, UsuarioID: usuarioID, Nome: fat.Nome,
		Documento: fat.Documento, Email: fat.Email, Pais: fat.Pais, ValorCentavos: fat.ValorCentavos,
		Moeda: fat.Moeda, Endereco: fat.Endereco})
}

// descrever vira o texto de notas.erro: só erros já genéricos (ErroEmissor, erroFatura) ou a classe do erro.
func descrever(err error) string {
	var ee *ErroEmissor
	var ef erroFatura
	switch {
	case errors.As(err, &ee):
		return ee.Error()
	case errors.As(err, &ef):
		return ef.Error()
	case errors.Is(err, context.DeadlineExceeded):
		return "tempo esgotado"
	}
	return fmt.Sprintf("falha do emissor (%T)", err)
}
