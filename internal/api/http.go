package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"syscall"
)

const limiteCorpo = 2_000_000

// statusClienteDesistiu: o cliente fechou o pedido antes da resposta (o mesmo 499 que o nginx usa). Ninguém lê.
const statusClienteDesistiu = 499

// ErroInterno é a mensagem fixa do 500: nunca leva o texto do erro.
const ErroInterno = "erro interno do app; tente de novo"

// tipoDoErro descreve o erro sem o texto dele: os tipos da cadeia, a operação de arquivo e o código do sistema.
// Ex.: "*fs.PathError mkdir > syscall.Errno 267". Caminho, nome de arquivo e conteúdo ficam de fora.
func tipoDoErro(err error) string {
	var partes []string
	for e := err; e != nil && len(partes) < 6; e = errors.Unwrap(e) {
		parte := fmt.Sprintf("%T", e)
		var pe *fs.PathError
		if errors.As(e, &pe) && e == error(pe) {
			parte += " " + pe.Op
		}
		var errno syscall.Errno
		if errors.As(e, &errno) && e == error(errno) {
			parte += " " + strconv.Itoa(int(errno))
		}
		partes = append(partes, parte)
	}
	return strings.Join(partes, " > ")
}

// Handler embrulha a API em HTTP (porte da parte de API de servidor/app.mjs).
// Rotas antigas /api/x viram /api/v1/x. Gravações exigem JSON e nada vindo de outro site.
func (a *API) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.canal != "" { // a API do assistente nunca serve a tela (jc-a2y.7.4)
			enviar(w, Resposta{Status: 403, Corpo: Objeto{"erro": msgForaDoAssistente}})
			return
		}
		p := r.URL.EscapedPath()
		if strings.HasPrefix(p, "/api/") && !strings.HasPrefix(p, Prefixo+"/") {
			p = Prefixo + p[len("/api"):]
		}
		m := r.Method
		onboarding := m == http.MethodPost && (p == Prefixo+"/onboarding" || p == Prefixo+"/onboarding/sugerir")
		if m != http.MethodGet && m != http.MethodHead && m != http.MethodOptions {
			if s := r.Header.Get("Sec-Fetch-Site"); s != "" && s != "same-origin" && s != "none" {
				enviar(w, Resposta{Status: 403, Corpo: Objeto{"erro": "gravação recusada: pedido veio de outro site"}})
				return
			}
			if onboarding && r.Header.Get("X-Applyer-Onboarding") != "1" {
				enviar(w, Resposta{Status: 403, Corpo: Objeto{"erro": "use a tela de primeiro acesso do Applyer"}})
				return
			}
			if !onboarding && !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
				enviar(w, Resposta{Status: 415, Corpo: Objeto{"erro": "envie JSON (content-type: application/json)"}})
				return
			}
		}
		var corpo any = Objeto{}
		if m != http.MethodGet {
			var c any
			var err error
			if onboarding {
				c, err = lerOnboarding(w, r)
			} else {
				c, err = lerCorpo(r.Body)
			}
			if err != nil {
				if resp, ok := respostaDeErro(err); ok {
					enviar(w, resp)
					return
				}
				enviar(w, Resposta{Status: 500, Corpo: Objeto{"erro": ErroInterno}})
				return
			}
			corpo = c
		}
		caminho := p
		if r.URL.RawQuery != "" {
			caminho += "?" + r.URL.RawQuery
		}
		resp, err := a.Chamar(r.Context(), Pedido{Metodo: m, Caminho: caminho, Corpo: corpo, Autor: "eu",
			XPedido: r.Header.Get("X-Pedido"), XImpressao: r.Header.Get("X-Impressao"), DaJanela: daJanela(r)})
		// Só o cancelamento em si: um erro de verdade (disco, SQLite) que coincide com a saída do cliente continua 500.
		if err != nil && r.Context().Err() != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
			// jc-a2y.35: o cliente desistiu (fechou ou trocou de tela) e o erro é só o cancelamento. Não é erro do
			// app: nada vai para o log, e a gravação em andamento já foi desfeita pela transação.
			enviar(w, Resposta{Status: statusClienteDesistiu, Corpo: Objeto{"erro": "pedido cancelado"}})
			return
		}
		if err != nil {
			// O texto do erro pode ter caminho da pasta (com o nome do usuário) ou dado do pedido: não vai para a
			// resposta nem para o log. O log fica com método, caminho sem query e o tipo do erro.
			log.Printf("api: erro 500 em %s %s (%s)", m, p, tipoDoErro(err))
			resp = Resposta{Status: 500, Corpo: Objeto{"erro": ErroInterno}}
		}
		enviar(w, resp)
	})
}

// lerCorpo lê o JSON do pedido: vazio vira {}, grande demais é 413, inválido é 400.
func lerCorpo(b io.Reader) (any, error) {
	dados, err := io.ReadAll(io.LimitReader(b, limiteCorpo+1))
	if err != nil {
		return nil, falha(400, "JSON inválido")
	}
	if len(dados) > limiteCorpo {
		return nil, falha(413, "corpo grande demais")
	}
	if len(strings.TrimSpace(string(dados))) == 0 {
		return Objeto{}, nil
	}
	var c any
	if err := json.Unmarshal(dados, &c); err != nil {
		return nil, falha(400, "JSON inválido")
	}
	return c, nil
}

func enviar(w http.ResponseWriter, r Resposta) {
	tipo := r.Tipo
	if tipo == "" {
		tipo = "application/json; charset=utf-8"
	}
	w.Header().Set("Content-Type", tipo)
	w.Header().Set("Cache-Control", "no-store")
	for k, v := range r.Cabecalhos {
		w.Header().Set(k, v)
	}
	w.WriteHeader(r.Status)
	if s, ehTexto := r.Corpo.(string); ehTexto && r.Tipo != "" {
		io.WriteString(w, s)
		return
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(r.Corpo); err != nil && !errors.Is(err, io.ErrClosedPipe) {
		log.Printf("api: responder: %s", tipoDoErro(err))
	}
}
