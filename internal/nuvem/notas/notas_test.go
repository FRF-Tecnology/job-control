package notas

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/FRF-Tecnology/job-control/internal/nuvem/pagamento"
	"github.com/FRF-Tecnology/job-control/internal/nuvem/pgteste"
)

func TestMain(m *testing.M) { os.Exit(pgteste.Principal(m)) }

const cpfFicticio = "00000000191"

type amb struct {
	db   *pgxpool.Pool
	st   *pagamento.Falso
	em   *Falso
	fila *Fila
	user string
}

func novoAmb(t *testing.T) *amb {
	t.Helper()
	db := pgteste.Banco(t)
	var user string
	if err := db.QueryRow(context.Background(),
		`insert into usuarios (email) values ('ana@exemplo.com') returning id::text`).Scan(&user); err != nil {
		t.Fatal(err)
	}
	st := pagamento.NovoFalso("whsec_x")
	st.DefinirFatura("in_1", pagamento.Fatura{Nome: "Ana Exemplo", Email: "ana@exemplo.com", Documento: cpfFicticio,
		Pais: "BR", ValorCentavos: 2990, Moeda: "BRL",
		Endereco: map[string]string{"linha1": "Rua Exemplo 1", "cidade": "São Paulo", "estado": "SP", "cep": "01000000"}})
	em := &Falso{}
	return &amb{db: db, st: st, em: em, user: user, fila: NovaFila(db, st, em, nil)}
}

func (a *amb) linha(t *testing.T, inv string) (estado, numero, link, erro string, tentativas int) {
	t.Helper()
	err := a.db.QueryRow(context.Background(),
		`select estado, numero, link_pdf, erro, tentativas from notas where invoice_id = $1`, inv).
		Scan(&estado, &numero, &link, &erro, &tentativas)
	if err != nil {
		t.Fatal(err)
	}
	return
}

func TestAgendarIdempotente(t *testing.T) {
	a := novoAmb(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := a.fila.Agendar(ctx, "in_1", a.user); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	_ = a.db.QueryRow(ctx, `select count(*) from notas`).Scan(&n)
	if n != 1 {
		t.Fatalf("linhas = %d, queria 1", n)
	}
	if estado, _, _, _, tent := a.linha(t, "in_1"); estado != "pendente" || tent != 0 {
		t.Fatalf("estado=%s tentativas=%d", estado, tent)
	}
}

func TestEmitirGravaNumero(t *testing.T) {
	a := novoAmb(t)
	ctx := context.Background()
	_ = a.fila.Agendar(ctx, "in_1", a.user)
	a.fila.Processar(ctx)
	estado, numero, _, erro, tent := a.linha(t, "in_1")
	if estado != "emitida" || numero == "" || erro != "" || tent != 1 {
		t.Fatalf("estado=%s numero=%q erro=%q tentativas=%d", estado, numero, erro, tent)
	}
	p := a.em.Pedidos()
	if len(p) != 1 || p[0].Documento != cpfFicticio || p[0].Nome != "Ana Exemplo" || p[0].ValorCentavos != 2990 || p[0].UsuarioID != a.user {
		t.Fatalf("pedido = %+v", p)
	}
	// Emitida não é emitida de novo.
	a.fila.Processar(ctx)
	if len(a.em.Pedidos()) != 1 {
		t.Fatal("emitiu duas vezes")
	}
}

func TestFalhaTentaDeNovo(t *testing.T) {
	a := novoAmb(t)
	ctx := context.Background()
	a.em.Falhas = 2
	_ = a.fila.Agendar(ctx, "in_1", a.user)
	for i := 1; i <= 2; i++ {
		a.fila.Processar(ctx)
		estado, _, _, erro, tent := a.linha(t, "in_1")
		if estado != "falhou" || erro == "" || tent != i {
			t.Fatalf("rodada %d: estado=%s erro=%q tentativas=%d", i, estado, erro, tent)
		}
	}
	a.fila.Processar(ctx)
	estado, numero, _, erro, tent := a.linha(t, "in_1")
	if estado != "emitida" || numero == "" || erro != "" || tent != 3 {
		t.Fatalf("estado=%s numero=%q erro=%q tentativas=%d", estado, numero, erro, tent)
	}
}

func TestDesisteDepoisDe24(t *testing.T) {
	a := novoAmb(t)
	ctx := context.Background()
	a.em.Falhas = 1000
	_ = a.fila.Agendar(ctx, "in_1", a.user)
	for i := 0; i < 30; i++ {
		a.fila.Processar(ctx)
	}
	estado, _, _, _, tent := a.linha(t, "in_1")
	if estado != "falhou" || tent != MaxTentativas {
		t.Fatalf("estado=%s tentativas=%d", estado, tent)
	}
	if n := a.em.Chamadas(); n != MaxTentativas {
		t.Fatalf("emissor chamado %d vezes", n)
	}
}

func TestUmaPassadaTentaCadaNotaUmaVez(t *testing.T) {
	a := novoAmb(t)
	ctx := context.Background()
	a.em.Falhas = 1000
	_ = a.fila.Agendar(ctx, "in_1", a.user)
	a.fila.Processar(ctx)
	if n := a.em.Chamadas(); n != 1 {
		t.Fatalf("chamadas = %d, queria 1 por passada", n)
	}
}

func TestFaturaIndisponivelFalhaSemDados(t *testing.T) {
	a := novoAmb(t)
	ctx := context.Background()
	_ = a.fila.Agendar(ctx, "in_x", a.user) // o Stripe falso não conhece in_x
	a.fila.Processar(ctx)
	estado, _, _, erro, tent := a.linha(t, "in_x")
	if estado != "falhou" || tent != 1 || erro == "" {
		t.Fatalf("estado=%s erro=%q tentativas=%d", estado, erro, tent)
	}
}

func TestDuasInstanciasNaoEmitemEmDobro(t *testing.T) {
	a := novoAmb(t)
	ctx := context.Background()
	_ = a.fila.Agendar(ctx, "in_1", a.user)
	outra := NovaFila(a.db, a.st, a.em, nil)
	a.em.Atraso = 300 * time.Millisecond
	fim := make(chan struct{})
	go func() { a.fila.Processar(ctx); close(fim) }()
	time.Sleep(100 * time.Millisecond)
	outra.Processar(ctx)
	<-fim
	if n := a.em.Chamadas(); n != 1 {
		t.Fatalf("emissor chamado %d vezes", n)
	}
}

func TestDocumentoNaoVaiProBanco(t *testing.T) {
	a := novoAmb(t)
	ctx := context.Background()
	a.em.Falhas = 1
	_ = a.fila.Agendar(ctx, "in_1", a.user)
	a.fila.Processar(ctx) // falha
	a.fila.Processar(ctx) // emite
	var achou bool
	err := a.db.QueryRow(ctx, `select exists (select 1 from notas n where n::text like '%' || $1::text || '%')`, cpfFicticio).Scan(&achou)
	if err != nil {
		t.Fatal(err)
	}
	if achou {
		t.Fatal("o CPF apareceu numa coluna de notas")
	}
}

func TestErroDoEmissorNaoGuardaCorpo(t *testing.T) {
	a := novoAmb(t)
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"message":"CPF ` + cpfFicticio + ` inválido de Ana Exemplo"}`))
	}))
	defer srv.Close()
	f := NovaFila(a.db, a.st, &NFEio{Chave: "k", Empresa: "emp", Servico: "1.05", BaseURL: srv.URL}, nil)
	_ = f.Agendar(ctx, "in_1", a.user)
	f.Processar(ctx)
	estado, _, _, erro, _ := a.linha(t, "in_1")
	if estado != "falhou" || !strings.Contains(erro, "400") || strings.Contains(erro, cpfFicticio) || strings.Contains(erro, "Ana") {
		t.Fatalf("estado=%s erro=%q", estado, erro)
	}
}

func TestRodarRoda1VezECadaAgendar(t *testing.T) {
	a := novoAmb(t)
	ctx, parar := context.WithCancel(context.Background())
	feito := make(chan struct{})
	go func() { a.fila.Rodar(ctx); close(feito) }()
	_ = a.fila.Agendar(ctx, "in_1", a.user)
	limite := time.Now().Add(5 * time.Second)
	for time.Now().Before(limite) {
		if estado, _, _, _, _ := a.linha(t, "in_1"); estado == "emitida" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if estado, _, _, _, _ := a.linha(t, "in_1"); estado != "emitida" {
		t.Fatalf("estado = %s depois do Agendar", estado)
	}
	parar()
	select {
	case <-feito:
	case <-time.After(3 * time.Second):
		t.Fatal("Rodar não parou com o contexto")
	}
}

func TestNFEioMontaPedido(t *testing.T) {
	var caminho, metodo, auth, tipo string
	var corpo map[string]any
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		caminho, metodo, auth, tipo = r.URL.Path, r.Method, r.Header.Get("Authorization"), r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &corpo)
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"id":"abc123","number":"77"}`))
	}))
	defer srv.Close()
	e := &NFEio{Chave: "chave-ficticia", Empresa: "emp1", Servico: "1.05", BaseURL: srv.URL}
	numero, _, err := e.Emitir(context.Background(), Pedido{InvoiceID: "in_1", Nome: "Ana Exemplo", Email: "ana@exemplo.com",
		Documento: "000.000.001-91", Pais: "BR", ValorCentavos: 2990, Moeda: "BRL",
		Endereco: map[string]string{"linha1": "Rua Exemplo 1", "cidade": "São Paulo", "estado": "SP", "cep": "01000-000"}})
	if err != nil {
		t.Fatal(err)
	}
	if numero != "77" {
		t.Errorf("numero = %q", numero)
	}
	if metodo != "POST" || caminho != "/v1/companies/emp1/serviceinvoices" || auth != "chave-ficticia" || !strings.HasPrefix(tipo, "application/json") {
		t.Errorf("metodo=%s caminho=%s auth=%s tipo=%s", metodo, caminho, auth, tipo)
	}
	if corpo["cityServiceCode"] != "1.05" || corpo["description"] != "Assinatura mensal Applyer" || corpo["servicesAmount"] != 29.9 {
		t.Errorf("corpo = %v", corpo)
	}
	b, _ := corpo["borrower"].(map[string]any)
	if b["name"] != "Ana Exemplo" || b["email"] != "ana@exemplo.com" || b["federalTaxNumber"] != float64(191) || b["type"] != "NaturalPerson" {
		t.Errorf("tomador = %v", b)
	}
	if n.Load() != 1 {
		t.Errorf("chamadas = %d", n.Load())
	}
}

func TestNFEioMoedaEstrangeiraVaiNaDescricao(t *testing.T) {
	var corpo map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &corpo)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}))
	defer srv.Close()
	e := &NFEio{Chave: "k", Empresa: "emp1", Servico: "1.05", BaseURL: srv.URL}
	if _, _, err := e.Emitir(context.Background(), Pedido{Nome: "Ana", Pais: "AR", ValorCentavos: 500, Moeda: "USD", Documento: "20123456786"}); err != nil {
		t.Fatal(err)
	}
	d, _ := corpo["description"].(string)
	if !strings.Contains(d, "USD") || !strings.Contains(d, "5.00") || !strings.HasPrefix(d, "Assinatura mensal Applyer") {
		t.Errorf("descricao = %q", d)
	}
}

func TestNFEioErroSemCorpo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte("segredo " + cpfFicticio))
	}))
	defer srv.Close()
	e := &NFEio{Chave: "k", Empresa: "e", Servico: "1", BaseURL: srv.URL}
	_, _, err := e.Emitir(context.Background(), Pedido{Nome: "Ana", Pais: "BR", Documento: cpfFicticio, ValorCentavos: 100, Moeda: "BRL"})
	if err == nil || strings.Contains(err.Error(), cpfFicticio) || !strings.Contains(err.Error(), "500") {
		t.Fatalf("err = %v", err)
	}
}
