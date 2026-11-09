package armazem

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// A fila vai para a tabela itens (coleção candidaturas), sobrevive a reabrir o banco e uma gravação com erro não deixa
// nada pela metade. Dados fictícios.
func TestFilaDeCandidaturasNoBanco(t *testing.T) {
	ctx := context.Background()
	caminho := filepath.Join(t.TempDir(), "central.db")
	a, err := Abrir(ctx, caminho)
	if err != nil {
		t.Fatal(err)
	}
	cand := dominio.Objeto{"id": "cand-2026-11-09-0001-abcd", "vagaId": "li-1", "estado": "preparada", "versao": 1.0,
		"conteudo": dominio.Objeto{"idioma": "pt", "respostas": []any{}}}
	if err := a.Gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		e.Candidaturas = append(e.Candidaturas, cand)
		return e, nil
	}); err != nil {
		t.Fatal(err)
	}
	v1, _ := a.Versao(ctx)
	falha := errors.New("no meio")
	err = a.Gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		e.Candidaturas[0]["estado"] = "aprovada"
		return e, falha
	})
	if !errors.Is(err, falha) {
		t.Fatalf("erro = %v", err)
	}
	a.Fechar()

	b, err := Abrir(ctx, caminho)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Fechar()
	e, err := b.Carregar(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Candidaturas) != 1 || !reflect.DeepEqual(e.Candidaturas[0], cand) {
		t.Fatalf("fila depois de reabrir = %v", e.Candidaturas)
	}
	if v, _ := b.Versao(ctx); v != v1 {
		t.Fatalf("versão = %d; a gravação com erro não devia subir (era %d)", v, v1)
	}
}

func TestLimitesDeCandidatura(t *testing.T) {
	ctx := context.Background()
	a, err := Abrir(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Fechar()
	if l, err := a.LimitesCandidatura(ctx); err != nil || len(l) != 0 {
		t.Fatalf("sem nada gravado = %v %v", l, err)
	}
	if err := a.GravarLimiteCandidatura(ctx, "linkedin", 12); err != nil {
		t.Fatal(err)
	}
	if err := a.GravarLimiteCandidatura(ctx, "gupy", 40); err != nil {
		t.Fatal(err)
	}
	var ev *dominio.ErroValidacao
	for _, c := range []struct {
		p string
		n int
	}{{"linkedin", 0}, {"linkedin", 41}, {"outra", 10}, {"indeed", 10}} {
		if err := a.GravarLimiteCandidatura(ctx, c.p, c.n); !errors.As(err, &ev) {
			t.Errorf("%s=%d: erro = %v; queria validação", c.p, c.n, err)
		}
	}
	l, err := a.LimitesCandidatura(ctx)
	if err != nil || !reflect.DeepEqual(l, map[string]int{"linkedin": 12, "gupy": 40}) {
		t.Fatalf("limites = %v %v", l, err)
	}
	// Valor inválido gravado por fora é ignorado na leitura (vale o padrão).
	if err := a.Transacao(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `UPDATE meta SET valor = '{"linkedin":99,"outra":5,"inhire":7}' WHERE chave = ?`, MetaLimitesCandidatura)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if l, _ := a.LimitesCandidatura(ctx); !reflect.DeepEqual(l, map[string]int{"inhire": 7}) {
		t.Fatalf("limites com lixo = %v", l)
	}
}
