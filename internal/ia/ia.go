package ia

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Rota diz qual provedor e modelo atende uma função; Reserva entra se o provedor cair (rede, 5xx, 429).
type Rota struct {
	Provedor string `json:"provedor"`
	Modelo   string `json:"modelo,omitempty"`
	Reserva  *Rota  `json:"reserva,omitempty"`
}

// Config escolhe a rota por função (ou "padrao") e as opções de cada provedor.
type Config struct {
	Rotas      map[string]Rota           `json:"rotas"`
	Provedores map[string]OpcoesProvedor `json:"provedores,omitempty"`
}

// Uso é o registro de cada chamada (para medir custo por candidatura e comparar modelos).
type Uso struct {
	Funcao        string   `json:"funcao"`
	Provedor      string   `json:"provedor"`
	Modelo        string   `json:"modelo"`
	VagaID        string   `json:"vagaId,omitempty"`
	TokensEntrada *int     `json:"tokensEntrada"`
	TokensSaida   *int     `json:"tokensSaida"`
	CustoUSD      *float64 `json:"custoEstimadoUSD"`
	MS            int64    `json:"ms"`
	OK            bool     `json:"ok"`
	Erro          string   `json:"erro,omitempty"`
	Regras        []string `json:"regras,omitempty"`
	Data          string   `json:"data"`
}

// IA executa as funções de negócio. Chave devolve a chave de API de um provedor (cofre do sistema).
type IA struct {
	Config       Config
	Provedores   map[string]Provedor
	Chave        func(provedor string) string
	RegistrarUso func(Uso)
	Agora        func() time.Time
}

// Nova cria a IA com os provedores HTTP e de CLI.
func Nova(cfg Config, chave func(string) string, registrar func(Uso)) *IA {
	cliente := &http.Client{Timeout: tempoLimite}
	return &IA{
		Config: cfg, Chave: chave, RegistrarUso: registrar, Agora: time.Now,
		Provedores: map[string]Provedor{
			"anthropic": anthropic{cliente}, "openai": openai{cliente}, "google": google{cliente},
			"claude-cli": cli{qual: "claude"}, "codex-cli": cli{qual: "codex"}, "gemini-cli": cli{qual: "gemini"},
		},
	}
}

func (a *IA) rota(funcao string) (Rota, bool) {
	if r, ok := a.Config.Rotas[funcao]; ok {
		return r, true
	}
	r, ok := a.Config.Rotas["padrao"]
	return r, ok
}

// Configurada diz se há alguma IA escolhida.
func (a *IA) Configurada() bool { _, ok := a.Config.Rotas["padrao"]; return ok }

func (a *IA) executar(ctx context.Context, funcao string, entrada Objeto, r Rota, p Prompt) (Objeto, error) {
	inicio := a.Agora()
	uso := Uso{Funcao: funcao, Provedor: r.Provedor, Modelo: r.Modelo, Data: inicio.UTC().Format(time.RFC3339)}
	if id, ok := ObjetoDe(entrada["vaga"])["id"].(string); ok {
		uso.VagaID = id
	} else if id, ok := entrada["vagaId"].(string); ok {
		uso.VagaID = id
	}
	defer func() {
		uso.MS = a.Agora().Sub(inicio).Milliseconds()
		if a.RegistrarUso != nil {
			a.RegistrarUso(uso)
		}
	}()
	falhar := func(err error) (Objeto, error) {
		var e *ErroIA
		if errors.As(err, &e) {
			uso.Erro = e.Tipo
			if e.Regra != "" {
				uso.Erro += ":" + e.Regra
			}
		} else {
			uso.Erro = "erro"
		}
		return nil, err
	}
	prov := a.Provedores[r.Provedor]
	if prov == nil {
		return falhar(&ErroIA{Tipo: "config", Mensagem: "provedor desconhecido: " + r.Provedor})
	}
	chave := ""
	if a.Chave != nil {
		chave = a.Chave(r.Provedor)
	}
	resp, err := prov.Chamar(ctx, PedidoProvedor{Modelo: r.Modelo, Chave: chave, Opcoes: a.Config.Provedores[r.Provedor], Prompt: p})
	if err != nil {
		return falhar(err)
	}
	uso.TokensEntrada, uso.TokensSaida = resp.TokensEntrada, resp.TokensSaida
	uso.CustoUSD = CustoEstimado(r.Modelo, resp.TokensEntrada, resp.TokensSaida)
	saida, aplicadas, err := Guardar(funcao, entrada, resp.Texto)
	if err != nil {
		return falhar(err)
	}
	uso.OK, uso.Regras = true, aplicadas
	return saida, nil
}

// Executar roda uma função de IA: valida a entrada, chama o provedor da rota (ou a reserva, se ele cair)
// e passa a saída pela guarda. Bloqueio da guarda não é contornado trocando de modelo.
func (a *IA) Executar(ctx context.Context, funcao string, entrada Objeto) (Objeto, error) {
	if err := ValidarEntrada(funcao, entrada); err != nil {
		return nil, err
	}
	r, ok := a.rota(funcao)
	if !ok {
		return nil, &ErroIA{Tipo: "config", Mensagem: "nenhuma IA conectada: escolha uma em Conectar IA"}
	}
	p := MontarPrompt(funcao, entrada)
	s, err := a.executar(ctx, funcao, entrada, r, p)
	var e *ErroIA
	if err != nil && r.Reserva != nil && errors.As(err, &e) && e.Tipo == "provedor" && e.Reintentavel {
		return a.executar(ctx, funcao, entrada, *r.Reserva, p)
	}
	return s, err
}

// ObjetoDe é um atalho para ler subobjetos.
func ObjetoDe(v any) Objeto { o, _ := v.(map[string]any); return o }
