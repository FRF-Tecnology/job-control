// Package google troca o código do login com Google (PKCE, retorno em loopback) pela identidade da pessoa.
// O app abre o navegador e recebe o code; só a nuvem tem o segredo do cliente e faz a troca.
package google

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/FRF-Tecnology/job-control/internal/nuvem/contas"
)

const (
	emissorGoogle = "https://accounts.google.com"
	prazo         = 15 * time.Second
)

var (
	// ErrRedirect: o redirect_uri não é o retorno em loopback do app.
	ErrRedirect = errors.New("redirect_uri inválido")
	// ErrGoogle: a troca ou a conferência do id_token falhou. O erro nunca traz o texto do Google.
	ErrGoogle = errors.New("não foi possível entrar com o Google")

	reRetorno = regexp.MustCompile(`^http://127\.0\.0\.1:\d{1,5}/retorno$`)
)

// Identidade é quem entrou: o sub (id estável no Google), o email já normalizado e o nome.
type Identidade struct {
	Sub, Email, Nome string
}

// Trocador troca o code (com o verifier do PKCE) pela identidade.
type Trocador interface {
	Trocar(ctx context.Context, code, verifier, redirectURI string) (Identidade, error)
}

// RedirectValido diz se o endereço é o retorno em loopback do app.
func RedirectValido(uri string) bool { return reRetorno.MatchString(uri) }

// Real fala com o Google. Use sempre por ponteiro: a descoberta do provedor é feita uma vez, na primeira
// troca, e guardada; se falhar, a próxima troca tenta de novo.
type Real struct {
	ClientID, ClientSecret string

	emissor string // vazio: o Google; os testes usam um servidor local
	mu      sync.Mutex
	prov    *oidc.Provider
}

var _ Trocador = (*Real)(nil)

func (r *Real) provedor(ctx context.Context) (*oidc.Provider, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.prov != nil {
		return r.prov, nil
	}
	emissor := r.emissor
	if emissor == "" {
		emissor = emissorGoogle
	}
	p, err := oidc.NewProvider(ctx, emissor)
	if err != nil {
		return nil, err
	}
	r.prov = p
	return p, nil
}

// verificado aceita email_verified como booleano ou como o texto "true" (o Google já mandou dos dois jeitos).
type verificado bool

func (v *verificado) UnmarshalJSON(b []byte) error {
	var x any
	if err := json.Unmarshal(b, &x); err != nil {
		return err
	}
	*v = verificado(x == true || x == "true")
	return nil
}

func (r *Real) Trocar(ctx context.Context, code, verifier, redirectURI string) (Identidade, error) {
	if !RedirectValido(redirectURI) {
		return Identidade{}, ErrRedirect
	}
	ctx, cancelar := context.WithTimeout(ctx, prazo)
	defer cancelar()
	ctx = oidc.ClientContext(ctx, &http.Client{Timeout: prazo})

	prov, err := r.provedor(ctx)
	if err != nil {
		return Identidade{}, fmt.Errorf("%w: descoberta: %v", ErrGoogle, err)
	}
	conf := oauth2.Config{
		ClientID: r.ClientID, ClientSecret: r.ClientSecret, Endpoint: prov.Endpoint(),
		RedirectURL: redirectURI, Scopes: []string{oidc.ScopeOpenID, "email", "profile"},
	}
	tok, err := conf.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		// O corpo da resposta do Google fica de fora; só o código do erro (ex.: invalid_grant) vai adiante.
		var re *oauth2.RetrieveError
		if errors.As(err, &re) {
			status := 0
			if re.Response != nil {
				status = re.Response.StatusCode
			}
			return Identidade{}, fmt.Errorf("%w: troca recusada (%d %s)", ErrGoogle, status, re.ErrorCode)
		}
		return Identidade{}, fmt.Errorf("%w: troca: falha de rede", ErrGoogle)
	}
	bruto, _ := tok.Extra("id_token").(string)
	if bruto == "" {
		return Identidade{}, fmt.Errorf("%w: resposta sem id_token", ErrGoogle)
	}
	// Confere assinatura, emissor, audiência (o nosso ClientID) e validade.
	idt, err := prov.Verifier(&oidc.Config{ClientID: r.ClientID}).Verify(ctx, bruto)
	if err != nil {
		return Identidade{}, fmt.Errorf("%w: id_token: %v", ErrGoogle, err)
	}
	var c struct {
		Email      string     `json:"email"`
		Verificado verificado `json:"email_verified"`
		Nome       string     `json:"name"`
	}
	if err := idt.Claims(&c); err != nil {
		return Identidade{}, fmt.Errorf("%w: claims ilegíveis", ErrGoogle)
	}
	if idt.Subject == "" {
		return Identidade{}, fmt.Errorf("%w: id_token sem sub", ErrGoogle)
	}
	if !c.Verificado {
		return Identidade{}, fmt.Errorf("%w: email não verificado no Google", ErrGoogle)
	}
	email, err := contas.NormalizarEmail(c.Email)
	if err != nil {
		return Identidade{}, fmt.Errorf("%w: email ausente ou inválido", ErrGoogle)
	}
	return Identidade{Sub: idt.Subject, Email: email, Nome: c.Nome}, nil
}
