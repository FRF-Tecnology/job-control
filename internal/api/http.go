package api

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
)

const limiteCorpo = 2_000_000

// Handler embrulha a API em HTTP (porte da parte de API de servidor/app.mjs).
// Rotas antigas /api/x viram /api/v1/x. Gravações exigem JSON e nada vindo de outro site.
func (a *API) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.EscapedPath()
		if strings.HasPrefix(p, "/api/") && !strings.HasPrefix(p, Prefixo+"/") {
			p = Prefixo + p[len("/api"):]
		}
		m := r.Method
		if m != http.MethodGet && m != http.MethodHead && m != http.MethodOptions {
			if s := r.Header.Get("Sec-Fetch-Site"); s != "" && s != "same-origin" && s != "none" {
				enviar(w, Resposta{Status: 403, Corpo: Objeto{"erro": "gravação recusada: pedido veio de outro site"}})
				return
			}
			if !strings.Contains(r.Header.Get("Content-Type"), "application/json") {
				enviar(w, Resposta{Status: 415, Corpo: Objeto{"erro": "envie JSON (content-type: application/json)"}})
				return
			}
		}
		var corpo any = Objeto{}
		if m != http.MethodGet {
			c, err := lerCorpo(r.Body)
			if err != nil {
				if resp, ok := respostaDeErro(err); ok {
					enviar(w, resp)
					return
				}
			}
			corpo = c
		}
		caminho := p
		if r.URL.RawQuery != "" {
			caminho += "?" + r.URL.RawQuery
		}
		resp, err := a.Chamar(r.Context(), Pedido{Metodo: m, Caminho: caminho, Corpo: corpo, Autor: "eu"})
		if err != nil {
			log.Printf("api: %s %s: %v", m, caminho, err)
			resp = Resposta{Status: 500, Corpo: Objeto{"erro": err.Error()}}
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
		log.Printf("api: responder: %v", err)
	}
}
