package main

import (
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// jc-a2y.14 (gosec G114): o servidor do central-dev precisa de tempos limite. Um cliente que manda o cabeçalho
// pela metade (slowloris) não pode prender a conexão para sempre.
func TestServidorFechaCabecalhoLento(t *testing.T) {
	antes := lerCabecalho
	lerCabecalho = 300 * time.Millisecond // encurta só no teste
	defer func() { lerCabecalho = antes }()

	ouvinte, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := novoServidor(ouvinte.Addr().String(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	go srv.Serve(ouvinte)
	defer srv.Close()

	c, err := net.Dial("tcp", ouvinte.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	io.WriteString(c, "GET / HTTP/1.1\r\nHost: localhost\r\n") // cabeçalho sem a linha em branco do fim
	c.SetReadDeadline(time.Now().Add(5 * time.Second))
	inicio := time.Now()
	io.ReadAll(c) // volta quando o servidor fecha a conexão (ou no prazo do teste)
	if d := time.Since(inicio); d > 3*time.Second {
		t.Fatalf("o servidor segurou o cabeçalho lento por %v", d)
	}
}

// Os tempos: cabeçalho e corpo curtos; escrita acima do tempo limite da IA (120 s em internal/ia), para uma
// chamada de IA pelo central-dev não ser cortada no meio.
func TestServidorTemTemposLimite(t *testing.T) {
	s := novoServidor("127.0.0.1:0", http.NotFoundHandler())
	if s.ReadHeaderTimeout <= 0 || s.ReadTimeout <= 0 || s.WriteTimeout <= 0 || s.IdleTimeout <= 0 || s.MaxHeaderBytes <= 0 {
		t.Fatalf("tempos = %+v", s)
	}
	if s.WriteTimeout <= 120*time.Second {
		t.Fatalf("WriteTimeout %v corta uma chamada de IA (tempo limite 120 s)", s.WriteTimeout)
	}
}
