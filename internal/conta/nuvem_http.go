package conta

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/licenca"
)

const (
	limiteCorpo = 1 << 20 // 1 MiB
	tempoLimite = 15 * time.Second
)

type nuvemHTTP struct {
	base    string
	cliente *http.Client
}

// NovaHTTP cria o cliente da nuvem real. Só aceita https://, exceto loopback
// (http://127.0.0.1 e http://localhost), usado nos testes. cliente nil usa um novo com
// timeout de 15 s; um cliente sem timeout é copiado com 15 s. Nenhum segue redirecionamento.
func NovaHTTP(endereco string, cliente *http.Client) (Nuvem, error) {
	u, err := url.Parse(strings.TrimRight(endereco, "/"))
	if err != nil || u.Host == "" {
		return nil, errors.New("endereço da nuvem inválido")
	}
	loopback := u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost"
	if u.Scheme != "https" && !(u.Scheme == "http" && loopback) {
		return nil, errors.New("o endereço da nuvem precisa ser https://")
	}
	// Cópia: o timeout de 15 s quando falta e nunca seguir redirecionamento (o token não vai para outro endereço;
	// o 3xx volta como ErroNuvem com o status).
	c := http.Client{Timeout: tempoLimite}
	if cliente != nil {
		c = *cliente
		if c.Timeout == 0 {
			c.Timeout = tempoLimite
		}
	}
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	cliente = &c
	return &nuvemHTTP{base: u.String(), cliente: cliente}, nil
}

type corpoErro struct {
	Erro   string `json:"erro"`
	Motivo string `json:"motivo"`
	Versao string `json:"versao"`
}

// chamar faz o pedido; se saida != nil, decodifica nela o corpo de sucesso.
// Nunca registra tokens, senhas nem emails.
func (n *nuvemHTTP) chamar(ctx context.Context, metodo, caminho, acesso string, corpo, saida any) error {
	var leitor io.Reader
	if corpo != nil {
		b, err := json.Marshal(corpo)
		if err != nil {
			return err
		}
		leitor = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, metodo, n.base+caminho, leitor)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if corpo != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if acesso != "" {
		req.Header.Set("Authorization", "Bearer "+acesso)
	}
	resp, err := n.cliente.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return ErrSemInternet
	}
	defer resp.Body.Close()
	bruto, err := io.ReadAll(io.LimitReader(resp.Body, limiteCorpo+1))
	if err != nil {
		return ErrSemInternet
	}
	if len(bruto) > limiteCorpo {
		return &ErroNuvem{Status: resp.StatusCode, Msg: "Resposta grande demais da nuvem"}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var e corpoErro
		_ = json.Unmarshal(bruto, &e)
		msg := e.Erro
		if msg == "" {
			msg = fmt.Sprintf("Erro %d da nuvem", resp.StatusCode)
		}
		return &ErroNuvem{Status: resp.StatusCode, Msg: msg, Motivo: e.Motivo, Versao: e.Versao}
	}
	if saida == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.Unmarshal(bruto, saida); err != nil {
		return &ErroNuvem{Status: resp.StatusCode, Msg: "Resposta inválida da nuvem"}
	}
	return nil
}

func comTermos(m map[string]any, termos string) map[string]any {
	if termos != "" {
		m["termos_versao"] = termos
	}
	return m
}

func (n *nuvemHTTP) par(ctx context.Context, caminho string, corpo any) (Par, error) {
	var p Par
	err := n.chamar(ctx, http.MethodPost, caminho, "", corpo, &p)
	return p, err
}

func (n *nuvemHTTP) Criar(ctx context.Context, email, senha, termos string) error {
	return n.chamar(ctx, http.MethodPost, "/v1/contas", "",
		map[string]any{"email": email, "senha": senha, "termos_versao": termos}, nil)
}

func (n *nuvemHTTP) Confirmar(ctx context.Context, email, codigo, senha, dispositivo, termos string) (Par, error) {
	return n.par(ctx, "/v1/contas/confirmar", comTermos(map[string]any{
		"email": email, "senha": senha, "codigo": codigo, "dispositivo": dispositivo}, termos))
}

func (n *nuvemHTTP) Entrar(ctx context.Context, email, senha, dispositivo, termos string) (Par, error) {
	return n.par(ctx, "/v1/contas/entrar", comTermos(map[string]any{
		"email": email, "senha": senha, "dispositivo": dispositivo}, termos))
}

func (n *nuvemHTTP) Recuperar(ctx context.Context, email string) error {
	return n.chamar(ctx, http.MethodPost, "/v1/contas/recuperar", "", map[string]any{"email": email}, nil)
}

func (n *nuvemHTTP) NovaSenha(ctx context.Context, email, codigo, senha, dispositivo string) (Par, error) {
	return n.par(ctx, "/v1/contas/nova-senha", map[string]any{
		"email": email, "senha": senha, "codigo": codigo, "dispositivo": dispositivo})
}

func (n *nuvemHTTP) Google(ctx context.Context, code, verifier, redirectURI, dispositivo, termos string) (Par, error) {
	return n.par(ctx, "/v1/contas/google", comTermos(map[string]any{
		"code": code, "verifier": verifier, "redirect_uri": redirectURI, "dispositivo": dispositivo}, termos))
}

func (n *nuvemHTTP) Renovar(ctx context.Context, renovacao string) (Par, error) {
	return n.par(ctx, "/v1/sessao/renovar", map[string]any{"renovacao": renovacao})
}

func (n *nuvemHTTP) Sair(ctx context.Context, renovacao string) error {
	return n.chamar(ctx, http.MethodPost, "/v1/sessao/sair", "", map[string]any{"renovacao": renovacao}, nil)
}

func (n *nuvemHTTP) Licenca(ctx context.Context, acesso, dispositivo string) (licenca.Documento, error) {
	var d licenca.Documento
	err := n.chamar(ctx, http.MethodGet, "/v1/licenca?dispositivo="+url.QueryEscape(dispositivo), acesso, nil, &d)
	return d, err
}

func (n *nuvemHTTP) urlStripe(ctx context.Context, caminho, acesso string) (string, error) {
	var r struct {
		URL string `json:"url"`
	}
	if err := n.chamar(ctx, http.MethodPost, caminho, acesso, nil, &r); err != nil {
		return "", err
	}
	return r.URL, nil
}

func (n *nuvemHTTP) Assinar(ctx context.Context, acesso string) (string, error) {
	return n.urlStripe(ctx, "/v1/assinar", acesso)
}

func (n *nuvemHTTP) Portal(ctx context.Context, acesso string) (string, error) {
	return n.urlStripe(ctx, "/v1/portal", acesso)
}

func (n *nuvemHTTP) Notas(ctx context.Context, acesso string) ([]Nota, error) {
	var notas []Nota
	if err := n.chamar(ctx, http.MethodGet, "/v1/notas", acesso, nil, &notas); err != nil {
		return nil, err
	}
	return notas, nil
}

func (n *nuvemHTTP) Excluir(ctx context.Context, acesso, email string) error {
	return n.chamar(ctx, http.MethodDelete, "/v1/contas/eu", acesso, map[string]any{"email": email}, nil)
}

var _ Nuvem = (*nuvemHTTP)(nil)
