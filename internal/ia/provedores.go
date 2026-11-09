package ia

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// PedidoProvedor é o que um provedor recebe.
type PedidoProvedor struct {
	Modelo string
	Chave  string
	Opcoes OpcoesProvedor
	Prompt Prompt
}

// OpcoesProvedor: endereço alternativo (proxy, OpenRouter, compatíveis) e limite de tokens.
type OpcoesProvedor struct {
	URL       string `json:"url,omitempty"`
	MaxTokens int    `json:"maxTokens,omitempty"`
}

// RespostaProvedor é o texto devolvido e os tokens usados (nil = desconhecido).
type RespostaProvedor struct {
	Texto         string `json:"texto"`
	TokensEntrada *int   `json:"tokensEntrada"`
	TokensSaida   *int   `json:"tokensSaida"`
}

// Provedor fala com uma IA.
type Provedor interface {
	Nome() string
	Chamar(ctx context.Context, p PedidoProvedor) (RespostaProvedor, error)
}

const tempoLimite = 120 * time.Second

func cortarUTF16(s string, n int) string {
	u := utf16.Encode([]rune(s))
	if len(u) <= n {
		return s
	}
	return string(utf16.Decode(u[:n]))
}

// mensagemHTTP é o texto público e fixo de cada status de erro do provedor.
func mensagemHTTP(status int) string {
	switch {
	case status == 400:
		return "pedido recusado (confira o modelo e as opções)"
	case status == 401:
		return "chave recusada (confira a chave de API)"
	case status == 403:
		return "acesso negado (confira a chave e o plano da conta)"
	case status == 404:
		return "modelo ou endereço não encontrado"
	case status == 408 || status == 504:
		return "o provedor demorou demais"
	case status == 413:
		return "pedido grande demais"
	case status == 429:
		return "limite de uso atingido; tente mais tarde"
	case status >= 500:
		return "o provedor está com problema; tente mais tarde"
	}
	return "o provedor recusou o pedido"
}

var reCodigoErro = regexp.MustCompile(`^[A-Za-z_]{3,48}$`)

// codigoErro tira do corpo de erro só um código seguro: error.code, error.type ou error.status (Anthropic, OpenAI, Google).
// Só letras e "_", e nunca um pedaço da chave.
func codigoErro(bruto []byte, chave string) string {
	var j struct {
		Error map[string]any `json:"error"`
	}
	if json.Unmarshal(bruto, &j) != nil || j.Error == nil {
		return ""
	}
	for _, campo := range []string{"code", "type", "status"} {
		c, _ := j.Error[campo].(string)
		if reCodigoErro.MatchString(c) && !(chave != "" && strings.Contains(chave, c)) {
			return c
		}
	}
	return ""
}

// motivoRede descreve a falha sem repetir o texto dela (que traz o endereço e pode trazer dados do pedido).
func motivoRede(err error) string {
	var dns *net.DNSError
	var cert *tls.CertificateVerificationError
	var autoridade x509.UnknownAuthorityError
	var nome x509.HostnameError
	var invalido x509.CertificateInvalidError
	var ne net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &ne) && ne.Timeout():
		return "tempo esgotado"
	case errors.As(err, &dns):
		return "endereço não encontrado"
	case errors.Is(err, syscall.ECONNREFUSED):
		return "conexão recusada"
	case errors.As(err, &cert), errors.As(err, &autoridade), errors.As(err, &nome), errors.As(err, &invalido):
		return "certificado inválido"
	}
	return "sem conexão com o provedor"
}

// postarJSON: rede, 5xx e 429 viram erro reintentável (aciona a rota reserva).
// A mensagem de erro é sempre um texto fixo nosso: o corpo do provedor e o texto da falha de rede nunca entram,
// porque podem ecoar a chave (inteira, escapada, codificada ou em partes). Só passa o código curto do provedor.
func postarJSON(ctx context.Context, cliente *http.Client, provedor, endereco, chave string, cabecalhos map[string]string, corpo any) (map[string]any, error) {
	dados, _ := json.Marshal(corpo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endereco, bytes.NewReader(dados))
	if err != nil {
		return nil, &ErroIA{Tipo: "provedor", Mensagem: fmt.Sprintf("%s: endereço inválido", provedor), Provedor: provedor}
	}
	req.Header.Set("content-type", "application/json")
	for k, v := range cabecalhos {
		req.Header.Set(k, v)
	}
	resp, err := cliente.Do(req)
	if err != nil {
		return nil, &ErroIA{Tipo: "provedor", Mensagem: fmt.Sprintf("%s: falha de rede (%s)", provedor, motivoRede(err)), Provedor: provedor, Reintentavel: true}
	}
	defer resp.Body.Close()
	bruto, _ := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		reintentavel := resp.StatusCode == 429 || resp.StatusCode >= 500
		msg := fmt.Sprintf("%s: HTTP %d %s", provedor, resp.StatusCode, mensagemHTTP(resp.StatusCode))
		if c := codigoErro(bruto, chave); c != "" {
			msg += " [" + c + "]"
		}
		return nil, &ErroIA{Tipo: "provedor", Mensagem: msg, Provedor: provedor, Status: resp.StatusCode, Reintentavel: reintentavel}
	}
	var r map[string]any
	if err := json.Unmarshal(bruto, &r); err != nil {
		return nil, &ErroIA{Tipo: "provedor", Mensagem: provedor + ": resposta não é JSON", Provedor: provedor, Reintentavel: true}
	}
	return r, nil
}

func exigirChave(chave, provedor, variavel string) error {
	if chave == "" {
		return &ErroIA{Tipo: "config", Mensagem: fmt.Sprintf("%s: sem chave de API (configure provedores.%s.chave ou %s)", provedor, provedor, variavel), Provedor: provedor}
	}
	return nil
}

func inteiro(v any) *int {
	if f, ok := v.(float64); ok {
		n := int(f)
		return &n
	}
	return nil
}

func recusa(provedor, msg string) error {
	return &ErroIA{Tipo: "provedor", Mensagem: provedor + ": " + msg, Provedor: provedor}
}

func esquemaObjeto(p Prompt) any {
	var e any
	json.Unmarshal(p.Esquema, &e)
	return e
}

// ---------- Anthropic Messages API, saída em JSON via output_config.format ----------

type anthropic struct{ cliente *http.Client }

func (anthropic) Nome() string { return "anthropic" }

func (a anthropic) Chamar(ctx context.Context, p PedidoProvedor) (RespostaProvedor, error) {
	if err := exigirChave(p.Chave, "anthropic", "ANTHROPIC_API_KEY"); err != nil {
		return RespostaProvedor{}, err
	}
	base := dominio.Texto(dominio.OuV(p.Opcoes.URL, "https://api.anthropic.com"))
	max := p.Opcoes.MaxTokens
	if max == 0 {
		max = 16000
	}
	r, err := postarJSON(ctx, a.cliente, "anthropic", base+"/v1/messages", p.Chave,
		map[string]string{"x-api-key": p.Chave, "anthropic-version": "2026-10-21"},
		map[string]any{
			"model": p.Modelo, "max_tokens": max, "system": p.Prompt.Sistema,
			"messages":      []any{map[string]any{"role": "user", "content": p.Prompt.Mensagem}},
			"output_config": map[string]any{"format": map[string]any{"type": "json_schema", "schema": esquemaObjeto(p.Prompt)}},
		})
	if err != nil {
		return RespostaProvedor{}, err
	}
	if r["stop_reason"] == "refusal" {
		return RespostaProvedor{}, recusa("anthropic", "o modelo recusou")
	}
	var b strings.Builder
	for _, x := range dominio.ListaDe(r["content"]) {
		bloco := dominio.ObjetoDe(x)
		if bloco["type"] == "text" {
			b.WriteString(dominio.Texto(bloco["text"]))
		}
	}
	u := dominio.ObjetoDe(r["usage"])
	return RespostaProvedor{Texto: b.String(), TokensEntrada: inteiro(u["input_tokens"]), TokensSaida: inteiro(u["output_tokens"])}, nil
}

// ---------- OpenAI Chat Completions (serve para OpenRouter e compatíveis pela URL) ----------
// strict:false porque os contratos têm campos opcionais; quem garante o formato é a guarda.

type openai struct{ cliente *http.Client }

func (openai) Nome() string { return "openai" }

func (o openai) Chamar(ctx context.Context, p PedidoProvedor) (RespostaProvedor, error) {
	if err := exigirChave(p.Chave, "openai", "OPENAI_API_KEY"); err != nil {
		return RespostaProvedor{}, err
	}
	base := dominio.Texto(dominio.OuV(p.Opcoes.URL, "https://api.openai.com"))
	r, err := postarJSON(ctx, o.cliente, "openai", base+"/v1/chat/completions", p.Chave,
		map[string]string{"authorization": "Bearer " + p.Chave},
		map[string]any{
			"model":           p.Modelo,
			"messages":        []any{map[string]any{"role": "system", "content": p.Prompt.Sistema}, map[string]any{"role": "user", "content": p.Prompt.Mensagem}},
			"response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": p.Prompt.NomeEsquema, "schema": esquemaObjeto(p.Prompt), "strict": false}},
		})
	if err != nil {
		return RespostaProvedor{}, err
	}
	var msg Objeto
	if l := dominio.ListaDe(r["choices"]); len(l) > 0 {
		msg = dominio.ObjetoDe(dominio.ObjetoDe(l[0])["message"])
	}
	if dominio.Verdadeiro(msg["refusal"]) {
		return RespostaProvedor{}, recusa("openai", "o modelo recusou")
	}
	u := dominio.ObjetoDe(r["usage"])
	return RespostaProvedor{Texto: dominio.Texto(msg["content"]), TokensEntrada: inteiro(u["prompt_tokens"]), TokensSaida: inteiro(u["completion_tokens"])}, nil
}

// ---------- Google Gemini generateContent, JSON via responseJsonSchema ----------

type google struct{ cliente *http.Client }

func (google) Nome() string { return "google" }

func (g google) Chamar(ctx context.Context, p PedidoProvedor) (RespostaProvedor, error) {
	if err := exigirChave(p.Chave, "google", "GEMINI_API_KEY"); err != nil {
		return RespostaProvedor{}, err
	}
	base := dominio.Texto(dominio.OuV(p.Opcoes.URL, "https://generativelanguage.googleapis.com"))
	r, err := postarJSON(ctx, g.cliente, "google", base+"/v1beta/models/"+url.PathEscape(p.Modelo)+":generateContent", p.Chave,
		map[string]string{"x-goog-api-key": p.Chave},
		map[string]any{
			"systemInstruction": map[string]any{"parts": []any{map[string]any{"text": p.Prompt.Sistema}}},
			"contents":          []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": p.Prompt.Mensagem}}}},
			"generationConfig":  map[string]any{"responseMimeType": "application/json", "responseJsonSchema": esquemaObjeto(p.Prompt)},
		})
	if err != nil {
		return RespostaProvedor{}, err
	}
	if motivo := dominio.ObjetoDe(r["promptFeedback"])["blockReason"]; dominio.Verdadeiro(motivo) {
		return RespostaProvedor{}, recusa("google", "bloqueado ("+dominio.JSString(motivo)+")")
	}
	var b strings.Builder
	if l := dominio.ListaDe(r["candidates"]); len(l) > 0 {
		for _, x := range dominio.ListaDe(dominio.ObjetoDe(dominio.ObjetoDe(l[0])["content"])["parts"]) {
			parte := dominio.ObjetoDe(x)
			if t, ok := parte["text"].(string); ok && !dominio.Verdadeiro(parte["thought"]) {
				b.WriteString(t)
			}
		}
	}
	u := dominio.ObjetoDe(r["usageMetadata"])
	saida := inteiro(u["candidatesTokenCount"])
	if saida != nil { // pensamento é cobrado como saída
		if p := inteiro(u["thoughtsTokenCount"]); p != nil {
			*saida += *p
		}
	}
	return RespostaProvedor{Texto: b.String(), TokensEntrada: inteiro(u["promptTokenCount"]), TokensSaida: saida}, nil
}

// Precos em USD por 1 milhão de tokens. Modelo fora da tabela: custo desconhecido (nil).
var Precos = map[string][2]float64{
	"claude-opus-5-5":   {4, 20},
	"claude-sonnet-5-5": {2, 10},
	"claude-haiku-5-5":  {0.1, 0.5},
	"claude-haiku-4-5":  {1, 5},
}

// CustoEstimado em USD (nil quando o modelo ou os tokens são desconhecidos).
func CustoEstimado(modelo string, entrada, saida *int) *float64 {
	p, ok := Precos[modelo]
	if !ok || entrada == nil || saida == nil {
		return nil
	}
	c := (float64(*entrada)*p[0] + float64(*saida)*p[1]) / 1e6
	c = float64(int64(c*1e6+0.5)) / 1e6
	return &c
}
