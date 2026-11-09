package api

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// Rotas de IA (/api/v1/ia/...): conectar a IA da pessoa e usar nas telas.
//   GET    /ia                      o que está conectado, CLIs detectadas, chaves salvas (só o final) e uso
//   GET    /ia/config               só o que está conectado (rápido, para as telas)
//   PUT    /ia/config               {provedor, modelo?, url?}   provedor "nenhum" desconecta
//   PUT    /ia/chaves/{provedor}    {chave}  vai para o cofre do sistema
//   DELETE /ia/chaves/{provedor}
//   POST   /ia/testar               uma triagem de mentira para ver se a IA responde
//   POST   /ia/triar/{vagaId}       opinião da IA sobre a vaga (não muda nada)
//   POST   /ia/mensagem/{vagaId}    rascunho de follow-up (nunca é enviado sozinho)

const chaveConfig = "ia"

// Provedores que a pessoa pode escolher e o modelo padrão de cada um ("" = ela escolhe / padrão da CLI).
var provedoresIA = map[string]string{
	"anthropic": "claude-haiku-5-5", "openai": "", "google": "",
	"claude-cli": "haiku", "codex-cli": "", "gemini-cli": "",
}

// Provedores que usam chave de API (as CLIs usam o login da pessoa).
var comChave = []string{"anthropic", "openai", "google"}

var (
	reIAChave  = regexp.MustCompile(`^/ia/chaves/([a-z-]+)$`)
	reIAVaga   = regexp.MustCompile(`^/ia/(triar|mensagem)/([^/]+)$`)
	reNomeProv = regexp.MustCompile(`^[a-z-]+$`)
)

func (a *API) configIA(ctx context.Context) (ia.Config, error) {
	var cfg ia.Config
	_, err := a.arm.LerMeta(ctx, chaveConfig, &cfg)
	return cfg, err
}

func (a *API) novaIA(ctx context.Context) (*ia.IA, error) {
	cfg, err := a.configIA(ctx)
	if err != nil {
		return nil, err
	}
	x := ia.Nova(cfg, func(p string) string { c, _ := a.cofre.Ler(p); return c }, func(u ia.Uso) {
		// Medir não pode derrubar a resposta.
		_ = a.arm.RegistrarUso(context.Background(), u.Data[:10], u)
	})
	for nome, p := range a.provedoresExtras {
		x.Provedores[nome] = p
	}
	return x, nil
}

// respostaIA traduz erros da IA para a tela: configuração 400, provedor fora 502, guarda/formato 422.
func respostaIA(err error) (Resposta, error) {
	var e *ia.ErroIA
	if !errors.As(err, &e) {
		return Resposta{}, err
	}
	status := 422
	switch e.Tipo {
	case "config", "entrada":
		status = 400
	case "provedor":
		status = 502
	}
	return Resposta{Status: status, Corpo: Objeto{"erro": e.Mensagem, "tipo": e.Tipo, "regra": textoOuNulo(e.Regra)}}, nil
}

func (a *API) rotasIA(ctx context.Context, m, p string, c Objeto) (Resposta, bool, error) {
	switch {
	case m == "GET" && p == "/ia":
		r, err := a.estadoIA(ctx)
		return r, true, err

	case m == "GET" && p == "/ia/config": // rápido: só o que está conectado (sem procurar CLIs)
		cfg, err := a.configIA(ctx)
		if err != nil {
			return Resposta{}, true, err
		}
		r, okR := cfg.Rotas["padrao"]
		if !okR {
			res, err := ok(Objeto{"configurada": false})
			return res, true, err
		}
		res, err := ok(Objeto{"configurada": true, "provedor": r.Provedor, "modelo": r.Modelo})
		return res, true, err

	case m == "PUT" && p == "/ia/config":
		r, err := a.configurarIA(ctx, c)
		return r, true, err

	case reIAChave.MatchString(p) && (m == "PUT" || m == "DELETE"):
		prov := reIAChave.FindStringSubmatch(p)[1]
		if !contem(comChave, prov) {
			return Resposta{}, true, falha(400, "este provedor não usa chave de API")
		}
		if m == "DELETE" {
			if err := a.cofre.Apagar(prov); err != nil {
				return Resposta{}, true, err
			}
			r, err := ok(Objeto{"provedor": prov, "final": nil})
			return r, true, err
		}
		chave, _ := c["chave"].(string)
		if chave = trimEspacos(chave); len(chave) < 10 {
			return Resposta{}, true, &dominio.ErroValidacao{Erros: []string{"cole a chave de API inteira"}}
		}
		if err := a.cofre.Gravar(prov, chave); err != nil {
			return Resposta{}, true, err
		}
		r, err := ok(Objeto{"provedor": prov, "final": segredos.Final(chave)})
		return r, true, err

	case m == "POST" && p == "/ia/testar":
		x, err := a.novaIA(ctx)
		if err != nil {
			return Resposta{}, true, err
		}
		e, err := a.arm.Carregar(ctx)
		if err != nil {
			return Resposta{}, true, err
		}
		inicio := time.Now()
		saida, err := x.Executar(ctx, "triarVaga", Objeto{
			"vaga":   Objeto{"id": "teste", "empresa": "Empresa Exemplo", "titulo": "Vaga de teste", "notas": "Vaga fictícia para testar a conexão."},
			"perfil": perfilOuVazio(e.Perfil),
		})
		if err != nil {
			r, err := respostaIA(err)
			return r, true, err
		}
		r, err := ok(Objeto{"ok": true, "ms": time.Since(inicio).Milliseconds(), "saida": saida})
		return r, true, err

	case m == "POST" && reIAVaga.MatchString(p):
		g := reIAVaga.FindStringSubmatch(p)
		id, err := decodificar(g[2])
		if err != nil {
			return Resposta{}, true, err
		}
		e, err := a.arm.Carregar(ctx)
		if err != nil {
			return Resposta{}, true, err
		}
		var vaga Objeto
		for _, v := range e.Vagas {
			if dominio.Igual(v["id"], id) {
				vaga = v
			}
		}
		if vaga == nil {
			return Resposta{}, true, falha(404, "vaga não encontrada: "+id)
		}
		x, err := a.novaIA(ctx)
		if err != nil {
			return Resposta{}, true, err
		}
		var saida Objeto
		if g[1] == "triar" {
			saida, err = x.Executar(ctx, "triarVaga", Objeto{"vaga": vaga, "perfil": perfilOuVazio(e.Perfil)})
		} else {
			idioma := "pt"
			if vaga["idioma"] == "en" {
				idioma = "en"
			}
			saida, err = x.Executar(ctx, "escreverMensagem", Objeto{"tipo": "follow-up", "vaga": vaga, "perfil": perfilOuVazio(e.Perfil), "idioma": idioma})
		}
		if err != nil {
			r, err := respostaIA(err)
			return r, true, err
		}
		r, err := ok(saida)
		return r, true, err
	}
	return Resposta{}, false, nil
}

func perfilOuVazio(p Objeto) Objeto {
	if p == nil {
		return Objeto{}
	}
	return p
}

func trimEspacos(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\n' || s[0] == '\t' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\n' || s[len(s)-1] == '\t' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

func (a *API) configurarIA(ctx context.Context, c Objeto) (Resposta, error) {
	prov, _ := c["provedor"].(string)
	if prov == "nenhum" {
		if err := a.arm.GravarMeta(ctx, chaveConfig, ia.Config{}); err != nil {
			return Resposta{}, err
		}
		return ok(Objeto{"configurada": false})
	}
	padrao, conhecido := provedoresIA[prov]
	if !conhecido || !reNomeProv.MatchString(prov) {
		return Resposta{}, &dominio.ErroValidacao{Erros: []string{"provedor desconhecido"}}
	}
	if msg, fora := ia.CLIIndisponivel(prov); fora { // jc-a2y.26: Codex e Gemini ainda sem travas de ferramentas
		return Resposta{}, &dominio.ErroValidacao{Erros: []string{msg}}
	}
	modelo, _ := c["modelo"].(string)
	if modelo = trimEspacos(modelo); modelo == "" {
		modelo = padrao
	}
	if !ia.ModeloSeguro(modelo) { // jc-a2y.43: o modelo vira argumento da CLI
		return Resposta{}, &dominio.ErroValidacao{Erros: []string{"o nome do modelo só pode ter letras, números, espaço e . , _ : / \\ = + @ ~ -"}}
	}
	if modelo == "" && !contem([]string{"codex-cli", "gemini-cli"}, prov) {
		return Resposta{}, &dominio.ErroValidacao{Erros: []string{"informe o modelo (ex.: o nome do modelo na sua conta)"}}
	}
	cfg := ia.Config{Rotas: map[string]ia.Rota{"padrao": {Provedor: prov, Modelo: modelo}}}
	if u, _ := c["url"].(string); trimEspacos(u) != "" {
		if !reHTTPS.MatchString(u) {
			return Resposta{}, &dominio.ErroValidacao{Erros: []string{"o endereço precisa começar com https://"}}
		}
		cfg.Provedores = map[string]ia.OpcoesProvedor{prov: {URL: trimEspacos(u)}}
	}
	if err := a.arm.GravarMeta(ctx, chaveConfig, cfg); err != nil {
		return Resposta{}, err
	}
	return ok(Objeto{"configurada": true, "provedor": prov, "modelo": modelo})
}

var reHTTPS = regexp.MustCompile(`^https://`)

func (a *API) estadoIA(ctx context.Context) (Resposta, error) {
	cfg, err := a.configIA(ctx)
	if err != nil {
		return Resposta{}, err
	}
	chaves := Objeto{}
	for _, p := range comChave {
		c, _ := a.cofre.Ler(p)
		if c == "" {
			chaves[p] = nil
		} else {
			chaves[p] = segredos.Final(c)
		}
	}
	corpo := Objeto{"configurada": false, "chaves": chaves, "clis": a.detectarCLIs(ctx), "gratis": Objeto{"disponivel": false}}
	if r, okR := cfg.Rotas["padrao"]; okR {
		corpo["configurada"], corpo["provedor"], corpo["modelo"] = true, r.Provedor, r.Modelo
		if o, tem := cfg.Provedores[r.Provedor]; tem && o.URL != "" {
			corpo["url"] = o.URL
		}
	}
	// Uso dos últimos 30 dias (sem conteúdo: só contagem, tokens e custo estimado).
	usos, err := a.arm.UsosDesde(ctx, time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	if err != nil {
		return Resposta{}, err
	}
	resumo := Objeto{"chamadas": 0, "falhas": 0, "custoEstimadoUSD": 0.0}
	chamadas, falhas, custo := 0, 0, 0.0
	for _, bruto := range usos {
		var u ia.Uso
		if json.Unmarshal(bruto, &u) != nil {
			continue
		}
		chamadas++
		if !u.OK {
			falhas++
		}
		if u.CustoUSD != nil {
			custo += *u.CustoUSD
		}
	}
	resumo["chamadas"], resumo["falhas"], resumo["custoEstimadoUSD"] = chamadas, falhas, float64(int64(custo*1e4+0.5))/1e4
	corpo["uso"] = resumo
	return ok(corpo)
}
