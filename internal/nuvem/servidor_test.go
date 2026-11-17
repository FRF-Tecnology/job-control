package nuvem

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSaude(t *testing.T) {
	s := Novo(Config{}, Dependencias{})
	r := httptest.NewRecorder()
	s.Handler().ServeHTTP(r, httptest.NewRequest("GET", "/v1/saude", nil))
	if r.Code != http.StatusOK {
		t.Fatalf("status = %d", r.Code)
	}
	var corpo map[string]bool
	if err := json.Unmarshal(r.Body.Bytes(), &corpo); err != nil || !corpo["ok"] {
		t.Fatalf("corpo = %s (%v)", r.Body.String(), err)
	}
}

func TestCorpoLimitado(t *testing.T) {
	s := Novo(Config{}, Dependencias{})
	s.mux.HandleFunc("POST /v1/teste-corpo", func(w http.ResponseWriter, r *http.Request) {
		var b [1]byte
		for {
			_, err := r.Body.Read(b[:])
			if err != nil {
				if errors.Is(err, io.EOF) {
					w.WriteHeader(200)
				} else {
					w.WriteHeader(413)
				}
				return
			}
		}
	})
	enviar := func(n int) int {
		r := httptest.NewRecorder()
		s.Handler().ServeHTTP(r, httptest.NewRequest("POST", "/v1/teste-corpo", strings.NewReader(strings.Repeat("a", n))))
		return r.Code
	}
	if c := enviar(1024); c != 200 {
		t.Errorf("1 KiB = %d", c)
	}
	if c := enviar(65 * 1024); c != 413 {
		t.Errorf("65 KiB = %d, esperava 413", c)
	}
}
