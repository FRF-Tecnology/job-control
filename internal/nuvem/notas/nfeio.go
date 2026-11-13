package notas

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	urlNFEio  = "https://api.nfe.io"
	descricao = "Assinatura mensal Applyer"
)

// NFEio emite pela API de notas de serviço do NFE.io.
type NFEio struct {
	Chave   string // NOTAS_CHAVE: vai no cabeçalho Authorization
	Empresa string // NOTAS_EMPRESA: id da empresa no NFE.io
	Servico string // NOTAS_SERVICO: código do serviço municipal (cityServiceCode)
	BaseURL string // vazio: https://api.nfe.io (os testes usam httptest)
	HTTP    *http.Client
}

func (n *NFEio) Emitir(ctx context.Context, p Pedido) (string, string, error) {
	corpo, err := json.Marshal(corpoNFEio(p, n.Servico))
	if err != nil {
		return "", "", &ErroEmissor{Classe: "pedido incompleto"}
	}
	base := strings.TrimRight(n.BaseURL, "/")
	if base == "" {
		base = urlNFEio
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		base+"/v1/companies/"+url.PathEscape(n.Empresa)+"/serviceinvoices", bytes.NewReader(corpo))
	if err != nil {
		return "", "", &ErroEmissor{Classe: "pedido inválido"}
	}
	req.Header.Set("Authorization", n.Chave)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	cli := n.HTTP
	if cli == nil {
		cli = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := cli.Do(req)
	if err != nil {
		// err (url.Error) citaria a URL; só a classe vai adiante.
		return "", "", &ErroEmissor{Classe: "rede"}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", "", &ErroEmissor{Status: resp.StatusCode} // o corpo da resposta nunca é lido nem guardado
	}
	var r struct {
		ID     string `json:"id"`
		Number any    `json:"number"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&r); err != nil || r.ID == "" {
		return "", "", &ErroEmissor{Classe: "resposta ilegível"}
	}
	// O NFE.io emite de forma assíncrona: o 202 traz o id da nota e o número pode vir só depois.
	// Sem número ainda, guardamos o id do NFE.io no lugar. Link do PDF: não vem nesta resposta.
	numero := r.ID
	switch v := r.Number.(type) {
	case string:
		if v != "" {
			numero = v
		}
	case float64:
		numero = strconv.FormatInt(int64(v), 10)
	}
	return numero, "", nil
}

// corpoNFEio monta o JSON do POST /serviceinvoices. É o único lugar que conhece os nomes dos campos do
// NFE.io (confirmar com a documentação e com o contador: código IBGE da cidade, país em ISO alfa-3 e
// número do endereço ainda não são enviados).
func corpoNFEio(p Pedido, servico string) map[string]any {
	desc := descricao
	if p.Moeda != "" && p.Moeda != "BRL" {
		// Cliente de fora: o valor vai como veio e a moeda original na descrição (regra final: contador).
		desc = fmt.Sprintf("%s (cobrado em %s %.2f)", descricao, p.Moeda, float64(p.ValorCentavos)/100)
	}
	tomador := map[string]any{"name": p.Nome, "email": p.Email}
	if d := soDigitos(p.Documento); d != "" {
		if n, err := strconv.ParseInt(d, 10, 64); err == nil {
			tomador["federalTaxNumber"] = n
			if len(d) > 11 {
				tomador["type"] = "LegalEntity"
			} else {
				tomador["type"] = "NaturalPerson"
			}
		}
	}
	end := map[string]any{"country": pais3(p.Pais)}
	for dest, orig := range map[string]string{"street": "linha1", "additionalInformation": "linha2", "state": "estado"} {
		if v := p.Endereco[orig]; v != "" {
			end[dest] = v
		}
	}
	if v := soDigitos(p.Endereco["cep"]); v != "" {
		end["postalCode"] = v
	}
	if v := p.Endereco["cidade"]; v != "" {
		end["city"] = map[string]any{"name": v}
	}
	tomador["address"] = end
	return map[string]any{
		"cityServiceCode": servico,
		"description":     desc,
		"servicesAmount":  float64(p.ValorCentavos) / 100,
		"borrower":        tomador,
	}
}

func soDigitos(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// pais3 converte o país do Stripe (alfa-2) para o alfa-3 do NFE.io; só o Brasil é mapeado, o resto passa.
func pais3(p string) string {
	if strings.EqualFold(p, "BR") || p == "" {
		return "BRA"
	}
	return strings.ToUpper(p)
}
