//go:build !contafalsa

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Sem a tag contafalsa, o central-dev fica sem conta (ruling A11) e sem a rota de teste do relógio.
func TestDevSemContaFalsa(t *testing.T) {
	c, envolver := contaDoDev(t.TempDir())
	if c != nil {
		t.Fatal("conta ligada sem a tag contafalsa")
	}
	h := envolver(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/v1/_teste/avancar", nil))
	if rec.Code != 404 {
		t.Fatalf("rota de teste sem a tag = %d", rec.Code)
	}
}
