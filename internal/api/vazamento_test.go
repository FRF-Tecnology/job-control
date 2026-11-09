package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// Chaves 100% fictícias. A primeira tem "/", "+" e "=" para as formas escapada e codificada ficarem diferentes da exata;
// a sentinela tem trechos só de letras e "_", como numa chave base64url.
const (
	segredoFicticioAPI   = "FICTICIAqwerty0123456789/abcdefXYZ+fim=9876"       // gitleaks:allow
	sentinelaFicticiaAPI = "sk-ant-api03-SENTINELAvazaKEYfragmento_ABC-123xyz" // gitleaks:allow
)

const msg401 = ": HTTP 401 chave recusada (confira a chave de API)"

// O provedor de verdade do app, mas apontado para um servidor falso local (a URL do app exige https).
type provedorRedirecionado struct {
	real ia.Provedor
	url  string
}

func (p provedorRedirecionado) Nome() string { return p.real.Nome() }
func (p provedorRedirecionado) Chamar(ctx context.Context, r ia.PedidoProvedor) (ia.RespostaProvedor, error) {
	r.Opcoes.URL = p.url
	return p.real.Chamar(ctx, r)
}

// capturarLog manda o log do app (pacote log) para um buffer até o fim do teste e devolve uma função que lê o que foi escrito.
// Um marcador prova que a captura está ligada.
func capturarLog(t *testing.T) func() string {
	t.Helper()
	var registro bytes.Buffer
	saidaAntes, flagsAntes := log.Writer(), log.Flags()
	log.SetOutput(&registro)
	t.Cleanup(func() { log.SetOutput(saidaAntes); log.SetFlags(flagsAntes) })
	return func() string {
		log.Print("marcador-do-teste")
		texto := registro.String()
		if !strings.Contains(texto, "marcador-do-teste") {
			t.Fatalf("a captura do log não funcionou: %q", texto)
		}
		return texto
	}
}

type resposta502 struct {
	rota, corpo, erro string
	status            int
	tipo              any
}

// chamarRotasDeIA salva a chave pelo HTTP, conecta o provedor (que responde 401 com corpoErro) e chama as 3 rotas de IA.
func chamarRotasDeIA(t *testing.T, prov, chave, corpoErro string) []resposta502 {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		io.WriteString(w, corpoErro)
	}))
	defer srv.Close()
	arm, err := armazem.Abrir(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer arm.Fechar()
	real := ia.Nova(ia.Config{}, nil, nil).Provedores[prov]
	h := Nova(arm, Opcoes{Hoje: func() string { return "2026-11-08" }, Aleatorio: func() string { return "abcd" },
		Cofre:        segredos.Memoria{},
		ProvedoresIA: map[string]ia.Provedor{prov: provedorRedirecionado{real, srv.URL}},
		DetectarCLIs: func(context.Context) []ia.EstadoCLI { return nil }}).Handler()
	chamar := func(m, p string, corpo any) (int, string) {
		t.Helper()
		b, _ := json.Marshal(corpo)
		req := httptest.NewRequest(m, "http://127.0.0.1"+Prefixo+p, strings.NewReader(string(b)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		bruto, _ := io.ReadAll(w.Result().Body)
		return w.Code, string(bruto)
	}
	if st, c := chamar("PUT", "/ia/chaves/"+prov, Objeto{"chave": chave}); st != 200 || strings.Contains(c, chave) {
		t.Fatalf("salvar chave = %d %s", st, c)
	}
	if st, c := chamar("PUT", "/ia/config", Objeto{"provedor": prov, "modelo": "modelo-exemplo"}); st != 200 {
		t.Fatalf("config = %d %s", st, c)
	}
	chamar("POST", "/vagas", Objeto{"id": "vaga-exemplo", "empresa": "Empresa Exemplo", "titulo": "Vaga de teste"})
	var saida []resposta502
	for _, rota := range []string{"/ia/testar", "/ia/triar/vaga-exemplo", "/ia/mensagem/vaga-exemplo"} {
		st, corpo := chamar("POST", rota, Objeto{})
		var dec map[string]any
		json.Unmarshal([]byte(corpo), &dec)
		erro, _ := dec["erro"].(string)
		saida = append(saida, resposta502{rota: rota, corpo: corpo, erro: erro, status: st, tipo: dec["tipo"]})
	}
	return saida
}

// trechoDe devolve o primeiro trecho de 4 caracteres de alguma forma que aparece no texto ("" se nenhum).
func trechoDe(texto string, formas []string, ignorarCaixa bool) string {
	if ignorarCaixa {
		texto = strings.ToLower(texto)
	}
	for _, f := range formas {
		if ignorarCaixa {
			f = strings.ToLower(f)
		}
		for i := 0; i+4 <= len(f); i++ {
			if strings.Contains(texto, f[i:i+4]) {
				return f[i : i+4]
			}
		}
	}
	return ""
}

func conferir502(t *testing.T, prov string, rs []resposta502, formas []string, ignorarCaixa bool) {
	t.Helper()
	for _, r := range rs {
		// Corpo cru (bytes da resposta) e texto já decodificado.
		for _, texto := range []string{r.corpo, r.erro} {
			if v := trechoDe(texto, formas, ignorarCaixa); v != "" {
				t.Fatalf("%s: a resposta traz um trecho da chave (%q): %s", r.rota, v, r.corpo)
			}
		}
		if r.status != 502 || r.tipo != "provedor" || r.erro != prov+msg401 {
			t.Fatalf("%s = %d %s", r.rota, r.status, r.corpo)
		}
	}
}

// Ponta a ponta pelo HTTP: o provedor ecoa a chave na mensagem de erro, em 5 formas; nem a resposta nem o log trazem trecho dela.
func TestErro502NaoDevolveAChave(t *testing.T) {
	s := segredoFicticioAPI
	formas := []string{s, strings.ReplaceAll(s, "/", `\/`), url.QueryEscape(s), s[:16] + "..." + s[len(s)-4:], s[len(s)-20:]}
	lerLog := capturarLog(t)
	for _, prov := range []string{"anthropic", "openai", "google"} {
		for i, eco := range formas {
			t.Run(fmt.Sprint(prov, "/forma-", i), func(t *testing.T) {
				corpo := fmt.Sprintf(`{"error":{"type":"authentication_error","message":"Incorrect API key provided: %s"}}`, eco)
				conferir502(t, prov, chamarRotasDeIA(t, prov, s, corpo), formas, false)
			})
		}
	}
	if v := trechoDe(lerLog(), formas, false); v != "" {
		t.Fatalf("o log traz um trecho da chave (%q)", v)
	}
}

// Ponta a ponta pelo HTTP: trechos derivados da sentinela (caixa trocada, prefixo, sufixo, "_" no lugar de "-")
// em error.code, error.type e error.status; nem a resposta nem o log trazem 4+ letras da sentinela, em qualquer caixa.
func TestErro502SemValorDoCorpo(t *testing.T) {
	run := "SENTINELAvazaKEYfragmento_ABC"
	ecos := []string{run, strings.ToLower(run), strings.ToUpper(run), "x" + run, "err_" + run, run + "_err",
		strings.ToLower(run[:12]) + "_code", "sk_ant_api"}
	letras := regexp.MustCompile(`[A-Za-z]{4,}`).FindAllString(sentinelaFicticiaAPI, -1)
	lerLog := capturarLog(t)
	for _, prov := range []string{"anthropic", "openai", "google"} {
		for _, campo := range []string{"code", "type", "status"} {
			for i, eco := range ecos {
				t.Run(fmt.Sprint(prov, "/", campo, "/eco-", i), func(t *testing.T) {
					corpo := fmt.Sprintf(`{"error":{%q:%q,"message":"x"}}`, campo, eco)
					rs := chamarRotasDeIA(t, prov, sentinelaFicticiaAPI, corpo)
					conferir502(t, prov, rs, letras, true)
					for _, r := range rs {
						if strings.Contains(strings.ToLower(r.corpo), "sk_ant") {
							t.Fatalf("%s: a resposta traz o começo da chave: %s", r.rota, r.corpo)
						}
					}
				})
			}
		}
	}
	texto := lerLog()
	if v := trechoDe(texto, letras, true); v != "" || strings.Contains(strings.ToLower(texto), "sk_ant") {
		t.Fatalf("o log traz letras da chave (%q)", v)
	}
}
