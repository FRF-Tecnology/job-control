package nuvem

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func (a *amb) getNotas(acesso string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/v1/notas", nil)
	req.RemoteAddr = "192.0.2.10:40000"
	if acesso != "" {
		req.Header.Set("Authorization", "Bearer "+acesso)
	}
	w := httptest.NewRecorder()
	a.s.Handler().ServeHTTP(w, req)
	return w
}

func TestNotasExigeSessao(t *testing.T) {
	a := novoAmb(t)
	if w := a.getNotas(""); w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestNotasListaSoAsDoUsuario(t *testing.T) {
	a := novoAmb(t)
	p := a.criarConfirmado(t, "ana@exemplo.com", "senha-forte-123")
	p2 := a.criarConfirmado(t, "bia@exemplo.com", "senha-forte-123")
	w := a.getNotas(p.Acesso)
	if w.Code != http.StatusOK || w.Body.String() != "[]\n" {
		t.Fatalf("sem notas: status=%d corpo=%q", w.Code, w.Body.String())
	}
	ctx := context.Background()
	var ana, bia string
	_ = a.db.QueryRow(ctx, `select id::text from usuarios where email = 'ana@exemplo.com'`).Scan(&ana)
	_ = a.db.QueryRow(ctx, `select id::text from usuarios where email = 'bia@exemplo.com'`).Scan(&bia)
	for _, q := range [][]any{
		{"in_1", ana, "12", "emitida", "https://nfe.exemplo.com/12.pdf", "2026-10-01T10:00:00Z"},
		{"in_2", ana, "", "pendente", "", "2026-11-01T10:00:00Z"},
		{"in_3", bia, "99", "emitida", "", "2026-11-02T10:00:00Z"},
	} {
		if _, err := a.db.Exec(ctx, `insert into notas (invoice_id, usuario_id, numero, estado, link_pdf, criada_em) values ($1, $2, $3, $4, $5, $6)`, q...); err != nil {
			t.Fatal(err)
		}
	}
	w = a.getNotas(p.Acesso)
	var lista []map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &lista); err != nil || w.Code != 200 {
		t.Fatalf("status=%d err=%v corpo=%s", w.Code, err, w.Body.String())
	}
	if len(lista) != 2 || lista[0]["estado"] != "pendente" || lista[1]["numero"] != "12" ||
		lista[1]["link_pdf"] != "https://nfe.exemplo.com/12.pdf" || lista[1]["criada_em"] != "2026-10-01T10:00:00Z" {
		t.Fatalf("lista = %v", lista)
	}
	if l := a.getNotas(p2.Acesso); l.Body.String() == w.Body.String() {
		t.Fatal("a lista da outra conta veio igual")
	}
}
