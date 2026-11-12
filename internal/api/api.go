// Package api é a API local /api/v1 da Central (porte de servidor/api.mjs).
// Não depende de HTTP: a janela do app, a CLI e o MCP chamam Chamar(Pedido) e recebem uma Resposta.
// http.go embrulha isso num http.Handler.
package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/FRF-Tecnology/job-control/internal/aovivo"
	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/busca"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/ia"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// Prefixo de todas as rotas.
const Prefixo = "/api/v1"

// VersaoApp é a versão do app (preenchida no build com -ldflags).
var VersaoApp = "0.1.0-dev"

type Objeto = dominio.Objeto

// Pedido é uma chamada à API. Corpo é o JSON já lido (objeto vazio quando não veio corpo).
type Pedido struct {
	Metodo  string
	Caminho string // /api/v1/... com ?consulta
	Corpo   any
	Autor   string // "eu" (tela) ou "claude" (CLI, extensão, MCP)
	// DaJanela: o Handler viu Sec-Fetch-Site same-origin e o Host da janela do app (jc-a2y.6.9). A CLI e o MCP
	// nunca marcam. Só o "Ao vivo" usa.
	DaJanela bool
	// Confirmação da proposta de um pedido (jc-a2y.7.1): os cabeçalhos X-Pedido e X-Impressao, só eles.
	XPedido, XImpressao string
	// doAssistente: o pedido já passou pela lista do ChamarAssistente (jc-a2y.7.4). Só este pacote marca.
	doAssistente bool
}

// Resposta é o que a API devolve. Tipo vazio = JSON.
type Resposta struct {
	Status     int
	Corpo      any
	Tipo       string
	Cabecalhos map[string]string
}

// API guarda as dependências da API local.
type API struct {
	buscaEmCurso   sync.Mutex
	consultarVagas func(context.Context, dominio.Objeto) busca.Coleta
	arm            *armazem.Armazem
	hoje           func() string
	contaID        string
	aleatorio      func() string // só nos testes: deixa os ids previsíveis

	cofre            segredos.Cofre
	provedoresExtras map[string]ia.Provedor // só nos testes: provedor de IA falso
	credenciaisIA    sync.RWMutex           // serializa alterações de rota, vínculos de host e acesso ao cofre
	provaIA          provaConexao           // confirmação da configuração atual, apenas nesta sessão
	detectarCLIs     func(context.Context) []ia.EstadoCLI
	relogio          func() time.Time // só nos testes muda; janela do cache da detecção de CLIs
	cacheCLIs        cacheDeteccao

	// trava: cada pedido segura a leitura; "apagar meus dados" segura a escrita (espera os pedidos em curso e
	// barra os novos). Depois de apagar, apagando fica true e todo pedido recebe 503 até o app fechar.
	trava sync.RWMutex
	// interpretando: uma interpretação de pedido por vez (jc-a2y.7.1); a segunda recebe 409.
	interpretando sync.Mutex
	apagando      bool
	// noMeioDoPedido só existe para o teste segurar um pedido em curso (com a trava de leitura).
	noMeioDoPedido func(caminho string)

	pararEnvio      func()                       // apagar meus dados: para o executor, espera o ciclo e fecha o navegador
	suspenderEnvio  func() func()                // sair das plataformas: nenhum ciclo novo até soltar
	fecharNavegador func()                       // sair das plataformas: fecha o navegador (o perfil não pode estar em uso)
	aoApagar        func()                       // depois de apagar os dados: fecha o app
	envioLigado     bool                         // este build traz o executor (envio.Ligado)
	mostrarEnvio    func(visivel bool)           // "Ver o envio": janela a partir da próxima candidatura
	entrar          func(endereco string) error  // "Entrar nas plataformas": janela de login até a pessoa fechar
	janela          func() (visivel, login bool) // estado da janela do envio
	aoVivo          *aovivo.AoVivo               // "Ao vivo" (jc-a2y.6.9): nil fora do app com envio (404)
	avisoPasta      string                       // aviso da pasta de dados nesta abertura (jc-etv.14)
	// lerFormulario (jc-a2y.7.2): o leitor do executor; nil fora do app com envio (404).
	lerFormulario func(ctx context.Context, endereco string) (dominio.LeituraFormulario, error)
	// canal (jc-a2y.7.4): mcp ou cli numa API do assistente; vazio na do app. Fixado na criação (Opcoes.Canal).
	canal string
	// janelaApp: os ganchos da janela do app de PC; nil fora dele (janela_app.go).
	janelaApp *JanelaApp
}

// Opcoes configura a API.
type Opcoes struct {
	Hoje      func() string // padrão: data local de hoje
	ContaID   string        // padrão: "local"
	Aleatorio func() string // padrão: sufixo aleatório de verdade

	Cofre        segredos.Cofre                       // padrão: cofre do sistema operacional
	ProvedoresIA map[string]ia.Provedor               // só nos testes
	DetectarCLIs func(context.Context) []ia.EstadoCLI // padrão: procura claude, codex e gemini no PATH

	// Ganchos do app (padrão: não fazem nada). PararEnvio e FecharNavegador vêm de envio.Ligar; AoApagar fecha o app.
	PararEnvio      func()
	SuspenderEnvio  func() (soltar func())
	FecharNavegador func()
	AoApagar        func()
	// EnvioLigado: este build traz o executor (envio.Ligado, só o envio_dev). A API não importa o envio.
	EnvioLigado bool
	// Janela do navegador do envio (jc-a2y.6.7), de envio.Ligar: MostrarEnvio, Entrar e Janela.
	MostrarEnvio func(visivel bool)
	Entrar       func(endereco string) error
	Janela       func() (visivel, login bool)
	// AoVivo (jc-a2y.6.9): o último quadro do navegador do envio, em memória. Só o app com o envio passa; sem ele,
	// /envio/ao-vivo dá 404 (build padrão e central-dev).
	AoVivo *aovivo.AoVivo
	// LerFormulario (jc-a2y.7.2): lê a estrutura do formulário no navegador do envio, sem interagir. Só o app com o
	// envio passa; sem ele, POST /candidaturas/{id}/ler-formulario dá 404.
	LerFormulario func(ctx context.Context, endereco string) (dominio.LeituraFormulario, error)
	// AvisoPasta: o aviso da pasta de dados nesta abertura (config.Abertura.Aviso, jc-etv.7): pasta-antiga-ficou,
	// migracao-adiada ou vazio. GET /dados/pasta mostra à tela, sem caminho.
	AvisoPasta string
	// Canal (jc-a2y.7.4, regra M4): mcp ou cli, só nos binários do assistente (o MCP e a CLI). Com ele, a API só
	// atende pelo ChamarAssistente (a lista x-assistente), o autor é sempre claude e o Handler da tela recusa tudo.
	// Fora de mcp e cli, a API do assistente recusa tudo (falha fechada).
	Canal string
	// JanelaApp: os ganchos da janela do app de PC (segundo plano e tamanho padrão). Só o app passa; sem eles,
	// GET /app/janela responde disponivel=false e as rotas que mudam a janela dão 404.
	JanelaApp *JanelaApp
}

// ErroModoSoPessoa: só a pessoa liga o modo direto do perfil (jc-a2y.28).
const ErroModoSoPessoa = "só a pessoa liga o modo direto (aplicar o grupo A sem Triagem)"

// Nova cria a API sobre um armazém.
func Nova(arm *armazem.Armazem, o Opcoes) *API {
	if o.Hoje == nil {
		o.Hoje = func() string { return dominio.HojeISO(time.Now()) }
	}
	if o.ContaID == "" {
		o.ContaID = "local"
	}
	if o.Canal != "" {
		// A API do assistente nunca usa o cofre (A2.4 do CyberSec, jc-a2y.7.4): nem o do sistema, nem um que o
		// binário passe. As rotas /ia/* também estão fora da lista.
		o.Cofre = cofreDoAssistente{}
	} else if o.Cofre == nil {
		// Cada pasta de dados tem as chaves dela no cofre (jc-a2y.65): apagar uma pasta não apaga as de outra.
		o.Cofre = segredos.Sistema{Servico: segredos.ServicoDaPasta(arm.Pasta())}
	}
	// Erro do cofre sempre com texto fixo, venha de onde vier: nada do que foi gravado sai numa resposta (jc-a2y.63).
	o.Cofre = segredos.Seguro{Cofre: o.Cofre}
	if o.Canal != "" {
		// A API do assistente nunca chama uma IA nem roda uma CLI (A2.4 (d) do CyberSec, jc-a2y.7.4).
		o.ProvedoresIA = map[string]ia.Provedor{}
		o.DetectarCLIs = func(context.Context) []ia.EstadoCLI { return nil }
	}
	if o.DetectarCLIs == nil {
		o.DetectarCLIs = ia.DetectarCLIs
	}
	nada := func() {}
	for _, f := range []*func(){&o.PararEnvio, &o.FecharNavegador, &o.AoApagar} {
		if *f == nil {
			*f = nada
		}
	}
	if o.SuspenderEnvio == nil {
		o.SuspenderEnvio = func() func() { return nada }
	}
	if o.MostrarEnvio == nil {
		o.MostrarEnvio = func(bool) {}
	}
	if o.Entrar == nil {
		o.Entrar = func(string) error {
			return &dominio.ErroConflito{Msg: "não deu para abrir o navegador (Chrome ou Edge) neste computador", Motivo: "sem-navegador"}
		}
	}
	if o.Janela == nil {
		o.Janela = func() (bool, bool) { return false, false }
	}
	return &API{arm: arm, hoje: o.Hoje, contaID: o.ContaID, aleatorio: o.Aleatorio, cofre: o.Cofre, provedoresExtras: o.ProvedoresIA, detectarCLIs: o.DetectarCLIs, relogio: time.Now,
		pararEnvio: o.PararEnvio, suspenderEnvio: o.SuspenderEnvio, fecharNavegador: o.FecharNavegador, aoApagar: o.AoApagar, envioLigado: o.EnvioLigado,
		mostrarEnvio: o.MostrarEnvio, entrar: o.Entrar, janela: o.Janela, aoVivo: o.AoVivo, lerFormulario: o.LerFormulario, avisoPasta: o.AvisoPasta, canal: o.Canal, janelaApp: o.JanelaApp, consultarVagas: busca.Consultar}
}

// erroApi vira uma resposta com status e corpo prontos.
type erroApi struct {
	status int
	corpo  Objeto
}

func (e *erroApi) Error() string { return fmt.Sprint(e.corpo["erro"]) }

func falha(status int, msg string) error { return &erroApi{status, Objeto{"erro": msg}} }

var reNaoEncontrado = regexp.MustCompile(`não encontrad[ao]`)

// respostaDeErro transforma erros conhecidos em status HTTP; os outros sobem (viram 500).
func respostaDeErro(err error) (Resposta, bool) {
	var ea *erroApi
	var ev *dominio.ErroValidacao
	var ed *dominio.ErroDuplicata
	switch {
	case errors.As(err, &ea):
		return Resposta{Status: ea.status, Corpo: ea.corpo}, true
	case errors.As(err, &ev):
		return Resposta{Status: 400, Corpo: Objeto{"erro": ev.Error(), "erros": ev.Erros}}, true
	case errors.As(err, &ed):
		return Resposta{Status: 409, Corpo: Objeto{"erro": ed.Error(), "existente": ed.Existente}}, true
	}
	if r, ok := respostaDaFila(err); ok {
		return r, true
	}
	if reNaoEncontrado.MatchString(err.Error()) {
		return Resposta{Status: 404, Corpo: Objeto{"erro": err.Error()}}, true
	}
	return Resposta{}, false
}

// Chamar atende um pedido. Erro só sobe quando é inesperado (o HTTP responde 500).
func (a *API) Chamar(ctx context.Context, p Pedido) (Resposta, error) {
	if a.canal != "" && !p.doAssistente { // API do assistente: tudo passa pela lista (jc-a2y.7.4)
		return a.ChamarAssistente(ctx, p)
	}
	if ehPedidoDeApagar(p) {
		return respostaOuErro(a.apagarComTrava(ctx, p))
	}
	a.trava.RLock()
	defer a.trava.RUnlock()
	if a.apagando {
		return respostaFechando(), nil
	}
	if a.noMeioDoPedido != nil {
		a.noMeioDoPedido(p.Caminho)
	}
	return respostaOuErro(a.rotear(ctx, p))
}

// respostaOuErro transforma os erros conhecidos em status; o resto sobe (o HTTP responde 500).
func respostaOuErro(r Resposta, err error) (Resposta, error) {
	if err != nil {
		if resp, ok := respostaDeErro(err); ok {
			return resp, nil
		}
		return Resposta{}, err
	}
	return r, nil
}

func ok(corpo any) (Resposta, error) { return Resposta{Status: 200, Corpo: corpo}, nil }

func criado(corpo any) (Resposta, error) { return Resposta{Status: 201, Corpo: corpo}, nil }

// decodificar imita decodeURIComponent: escape inválido ou UTF-8 quebrado = 404.
func decodificar(s string) (string, error) {
	d, err := url.PathUnescape(s)
	if err != nil || !utf8.ValidString(d) {
		return "", falha(404, "não encontrado")
	}
	return d, nil
}

func ehObjeto(c any) bool { _, ok := c.(map[string]any); return ok }

func (a *API) ctx(e dominio.Estado, autor string) dominio.Ctx {
	dias, _ := dominio.NumeroDe(e.Perfil["diasFollowUp"])
	if dias != dias || dias == 0 { // NaN ou 0 → padrão
		dias = 7
	}
	return dominio.Ctx{Autor: autor, ContaID: a.contaID, Agora: a.hoje(), DiasFollowUp: int(dias), Aleatorio: a.aleatorio, Canal: a.canal}
}

// gravar roda uma operação numa transação (com o backup do dia antes, quando o banco é um arquivo).
func (a *API) gravar(ctx context.Context, fn func(dominio.Estado) (dominio.Estado, error)) error {
	if a.arm.Pasta() != "" {
		if _, err := a.arm.BackupDiario(ctx, a.hoje()); err != nil {
			return err
		}
	}
	return a.arm.Gravar(ctx, fn)
}

// Currículo é um PDF na pasta de currículos.
type Curriculo struct {
	Nome    string `json:"nome"`
	Tamanho int64  `json:"tamanho"`
}

func (a *API) pastaCurriculos() string {
	if a.arm.Pasta() == "" {
		return ""
	}
	return filepath.Join(a.arm.Pasta(), "curriculos")
}

func (a *API) curriculos() []Curriculo {
	saida := []Curriculo{}
	pasta := a.pastaCurriculos()
	if pasta == "" {
		return saida
	}
	itens, err := os.ReadDir(pasta)
	if err != nil {
		return saida
	}
	for _, i := range itens {
		if i.IsDir() || !strings.EqualFold(filepath.Ext(i.Name()), ".pdf") {
			continue
		}
		if info, err := i.Info(); err == nil {
			saida = append(saida, Curriculo{i.Name(), info.Size()})
		}
	}
	sort.Slice(saida, func(i, j int) bool { return saida[i].Nome < saida[j].Nome })
	return saida
}

func textoOuNulo(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Versao muda sempre que alguém grava (a tela, a CLI em outro processo) ou um currículo novo aparece.
func (a *API) Versao(ctx context.Context) (string, error) {
	v, err := a.arm.Versao(ctx)
	if err != nil {
		return "", err
	}
	s := strconv.FormatInt(v, 10)
	if a.arm.Pasta() != "" {
		nomes := []string{}
		for _, c := range a.curriculos() {
			nomes = append(nomes, c.Nome)
		}
		s += "|" + strings.Join(nomes, ",")
	}
	return s, nil
}

// Checklist da primeira vez (mostrada na tela Hoje até tudo estar feito).
func (a *API) primeirosPassos(e dominio.Estado) []Objeto {
	pasta := a.pastaCurriculos()
	if pasta == "" {
		pasta = "a pasta de currículos"
	}
	return []Objeto{
		{"id": "perfil", "feito": strings.TrimSpace(dominio.Texto(e.Perfil["nome"])) != "", "titulo": "Preencher o perfil", "texto": "Nome, pretensões, inglês e filtros. O Claude usa em todas as candidaturas.", "rota": "#/perfil"},
		{"id": "curriculo", "feito": len(a.curriculos()) > 0, "titulo": "Colocar o currículo", "texto": "Copie o PDF (em português e, se tiver, em inglês) para " + pasta + ".", "rota": "#/perfil"},
		{"id": "rodada", "feito": len(e.Rodadas) > 0 || len(e.Vagas) > 0, "titulo": "Fazer a primeira rodada", "texto": "Abra o Claude Code nesta pasta e digite /rodada. Ele busca, filtra e mostra as vagas aqui para você decidir.", "rota": "#/plataformas", "botao": "Ver plataformas"},
	}
}

// Campos que a importação de fora ignora: ela só cria vagas novas em Descoberta.
var camposIgnoradosNaImportacao = []string{"etapa", "envio", "followUp", "descobertaEm", "atualizadoEm", "motivo", "pendencia", "subEtapa"}

// adicionarLote adiciona uma lista; cada item falha sozinho (duplicata ou erro) sem derrubar os outros.
func (a *API) adicionarLote(ctx context.Context, lista []any, autor string, forcar, soDescoberta bool) (Objeto, error) {
	adicionadas, duplicadas, erros := []any{}, []any{}, []any{}
	err := a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		c := a.ctx(e, autor)
		for _, item := range lista {
			dados := item
			if soDescoberta {
				o := dominio.Objeto{}
				for k, v := range item.(map[string]any) {
					if !contem(camposIgnoradosNaImportacao, k) {
						o[k] = v
					}
				}
				o["etapa"] = "descoberta"
				dados = o
			}
			id := dominio.ObjetoDe(dados)["id"]
			novo, _, err := dominio.AdicionarVaga(e, dados, c, forcar)
			var dup *dominio.ErroDuplicata
			switch {
			case err == nil:
				e = novo
				adicionadas = append(adicionadas, id)
			case errors.As(err, &dup):
				duplicadas = append(duplicadas, Objeto{"id": id, "existente": dup.Existente["id"], "etapa": dup.Existente["etapa"]})
			default:
				erros = append(erros, Objeto{"id": id, "erro": err.Error()})
			}
		}
		return e, nil
	})
	return Objeto{"adicionadas": adicionadas, "duplicadas": duplicadas, "erros": erros}, err
}

func contem(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// MesclarPerfil: filtros mesclados e respostasPadrao mescladas pela pergunta (sem diferenciar maiúsculas).
func MesclarPerfil(atual, p Objeto) Objeto {
	respostas := append([]any{}, dominio.ListaDe(atual["respostasPadrao"])...)
	for _, r := range dominio.ListaDe(p["respostasPadrao"]) {
		pergunta := strings.ToLower(dominio.CampoJS(dominio.ObjetoDe(r), "pergunta"))
		i := -1
		for j, x := range respostas {
			if strings.ToLower(dominio.CampoJS(dominio.ObjetoDe(x), "pergunta")) == pergunta {
				i = j
				break
			}
		}
		if i >= 0 {
			respostas[i] = r
		} else {
			respostas = append(respostas, r)
		}
	}
	novo := Objeto{}
	for k, v := range atual {
		novo[k] = v
	}
	for k, v := range p {
		novo[k] = v
	}
	if _, mudouFaixa := p["faixaSalarial"]; mudouFaixa {
		delete(novo, "pretensaoUSD")
	}
	filtros := Objeto{}
	for k, v := range dominio.ObjetoDe(atual["filtros"]) {
		filtros[k] = v
	}
	for k, v := range dominio.ObjetoDe(p["filtros"]) {
		filtros[k] = v
	}
	novo["filtros"], novo["respostasPadrao"] = filtros, respostas
	return novo
}

// ErroRespostaSensivel: o /perfil recusa resposta padrão para pergunta de número fiscal, documento, senha ou banco.
const ErroRespostaSensivel = "o perfil não guarda resposta para pergunta de CPF, documento, senha ou banco; apague esta resposta"

// cortarLinha devolve no máximo n letras do texto, numa linha só.
func cortarLinha(s string, n int) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return string(r)
}

func exigirPerfilOk(perfil any) error {
	if erros := dominio.ValidarPerfil(perfil); len(erros) > 0 {
		return &dominio.ErroValidacao{Erros: erros}
	}
	if dominio.PerfilSensivel(perfil) {
		return &dominio.ErroValidacao{Erros: []string{"o perfil não pode guardar CPF, RG, senha ou dados bancários"}}
	}
	return nil
}

var (
	reVagaID       = regexp.MustCompile(`^/vagas/([^/]+)$`)
	reVagaAcao     = regexp.MustCompile(`^/vagas/([^/]+)(/[a-z-]+)?$`)
	rePedido       = regexp.MustCompile(`^/pedidos/([^/]+)/concluir$`)
	reEmail        = regexp.MustCompile(`^/emails/([^/]+)/(ligar|descartar)$`)
	rePlataforma   = regexp.MustCompile(`^/plataformas/([^/]+)$`)
	reRodada       = regexp.MustCompile(`^/rodadas/([^/]+)$`)
	camposPlatText = []string{"nome", "url", "status", "comoAplicar", "limitacoes", "ultimaVarredura"}
)

// consulta lê a parte depois do "?" como o URLSearchParams (último valor vence; escapes ruins ficam como estão).
func consulta(bruta string) map[string]string {
	q := map[string]string{}
	for _, par := range strings.Split(bruta, "&") {
		if par == "" {
			continue
		}
		k, v, _ := strings.Cut(par, "=")
		dec := func(s string) string {
			if d, err := url.QueryUnescape(s); err == nil {
				return d
			}
			return s
		}
		q[dec(k)] = dec(v)
	}
	return q
}

func (a *API) rotear(ctx context.Context, ped Pedido) (Resposta, error) {
	caminho, bruta, _ := strings.Cut(ped.Caminho, "?")
	if !strings.HasPrefix(caminho, Prefixo+"/") {
		return Resposta{}, falha(404, "rota não encontrada")
	}
	p := strings.TrimPrefix(caminho, Prefixo)
	q := consulta(bruta)
	m := strings.ToUpper(ped.Metodo)
	corpo := ped.Corpo
	autor := ped.Autor
	if autor == "" {
		autor = "eu"
	}
	_, ehLista := corpo.([]any)
	if m != "GET" && !ehObjeto(corpo) && !(m == "POST" && (p == "/vagas" || p == "/vagas/importar") && ehLista) {
		return Resposta{}, falha(400, "envie um objeto JSON")
	}
	c := dominio.ObjetoDe(corpo)

	// Confirmação da proposta de um pedido (jc-a2y.7.1): só POST /vagas e /vagas/{id}/mover, só da tela. Vem antes de
	// qualquer outra rota: X-Pedido em outro caminho é recusado.
	if ped.XPedido != "" || ped.XImpressao != "" {
		return a.confirmarPedido(ctx, m, p, corpo, ped.XPedido, ped.XImpressao, autor)
	}
	if strings.HasPrefix(p, "/pedidos/") {
		if r, feito, err := a.rotasPedidosNoApp(ctx, m, p, c, autor); feito {
			return r, err
		}
	}
	if p == "/envio/ao-vivo" {
		return a.rotaAoVivo(ctx, m, c, autor, ped.DaJanela)
	}
	if m == "POST" && p == "/busca" {
		if autor != "eu" {
			return Resposta{}, falha(403, "somente a pessoa inicia uma busca no app")
		}
		return a.buscarETriar(ctx)
	}

	if strings.HasPrefix(p, "/envio") {
		if r, feito, err := a.rotasEnvio(ctx, m, p, q, c, autor); feito {
			return r, err
		}
	}

	if p == "/dados/apagar" || p == "/dados/pasta" || p == "/navegador/sair" || p == "/navegador/entrar" {
		if r, feito, err := a.rotasDados(ctx, m, p, c, autor); feito {
			return r, err
		}
	}

	if p == "/app/janela" || p == "/app/janela/padrao" {
		if r, feito, err := a.rotasJanelaApp(ctx, m, p, c, autor); feito {
			return r, err
		}
	}

	if strings.HasPrefix(p, "/candidaturas") {
		if r, feito, err := a.rotasCandidaturas(ctx, m, p, q, c, autor); feito {
			return r, err
		}
	}

	if strings.HasPrefix(p, "/ia") {
		if r, feito, err := a.rotasIA(ctx, m, p, c); feito {
			return r, err
		}
	}

	if m == "GET" {
		if r, feito, err := a.rotasDeLeitura(ctx, p, q); feito {
			return r, err
		}
	}

	switch {
	case m == "POST" && p == "/vagas":
		if l, ehL := corpo.([]any); ehL {
			r, err := a.adicionarLote(ctx, l, autor, false, false)
			return Resposta{Status: 200, Corpo: r}, err
		}
		if l, ehL := c["lista"].([]any); ehL {
			r, err := a.adicionarLote(ctx, l, autor, dominio.Verdadeiro(c["forcar"]), false)
			return Resposta{Status: 200, Corpo: r}, err
		}
		dados := Objeto{}
		for k, v := range c {
			if k != "forcar" {
				dados[k] = v
			}
		}
		var vaga Objeto
		err := a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
			novo, v, err := dominio.AdicionarVaga(e, dados, a.ctx(e, autor), dominio.Verdadeiro(c["forcar"]))
			vaga = v
			return novo, err
		})
		if err != nil {
			return Resposta{}, err
		}
		return criado(vaga)

	case m == "POST" && p == "/vagas/importar":
		lista := []any{}
		todos := []any{corpo}
		if l, ehL := corpo.([]any); ehL {
			todos = l
		}
		for _, x := range todos {
			if ehObjeto(x) {
				lista = append(lista, x)
			}
		}
		r, err := a.adicionarLote(ctx, lista, autor, false, true)
		return Resposta{Status: 200, Corpo: r}, err

	case p == "/perfil" && (m == "PUT" || m == "PATCH"):
		var novo Objeto
		if err := a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
			novo = c
			if m == "PATCH" {
				novo = MesclarPerfil(e.Perfil, c)
			}
			if autor != "eu" && dominio.Igual(novo["modo"], "direto") && !dominio.Igual(e.Perfil["modo"], "direto") {
				return e, &dominio.ErroValidacao{Erros: []string{ErroModoSoPessoa}}
			}
			if err := exigirPerfilOk(novo); err != nil {
				return e, err
			}
			// Só as respostas padrão alteradas agora recebem a verificação adicional.
			if pergunta := dominio.RespostaPadraoSensivel(c); pergunta != "" {
				return e, &dominio.ErroValidacao{Erros: []string{ErroRespostaSensivel + ": " + cortarLinha(pergunta, 80)}}
			}
			if assinaturaDeBusca(e.Perfil) != assinaturaDeBusca(novo) {
				for i, v := range e.Vagas {
					if v["etapa"] == "descoberta" && v["triagemPerfil"] != nil {
						e.Vagas[i] = aguardandoTriagem(v)
					}
				}
			}
			e.Perfil = novo
			return e, nil
		}); err != nil {
			return Resposta{}, err
		}
		return ok(novo)

	case p == "/onboarding" && m == "POST":
		return a.concluirOnboarding(ctx, autor, c)
	case p == "/onboarding/sugerir" && m == "POST":
		return a.sugerirPeloCurriculo(ctx, autor, c)

	case m == "POST" && p == "/pedidos":
		var ped Objeto
		err := a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
			novo, x, err := dominio.CriarPedido(e, c["texto"], a.ctx(e, autor))
			ped = x
			return novo, err
		})
		if err != nil {
			return Resposta{}, err
		}
		return criado(ped)

	case m == "POST" && rePedido.MatchString(p):
		id, err := decodificar(rePedido.FindStringSubmatch(p)[1])
		if err != nil {
			return Resposta{}, err
		}
		var ped Objeto
		err = a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
			novo, x, err := dominio.ConcluirPedido(e, id, c["resposta"], a.ctx(e, autor))
			ped = x
			return novo, err
		})
		if err != nil {
			return Resposta{}, err
		}
		return ok(ped)

	case m == "POST" && p == "/emails":
		return a.registrarEmails(ctx, c, autor)

	case m == "POST" && reEmail.MatchString(p):
		g := reEmail.FindStringSubmatch(p)
		id, err := decodificar(g[1])
		if err != nil {
			return Resposta{}, err
		}
		var res Objeto
		err = a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
			var novo dominio.Estado
			var err error
			if g[2] == "ligar" {
				novo, res, err = dominio.LigarEmail(e, id, c["vagaId"], a.ctx(e, autor))
			} else {
				novo, res, err = dominio.DescartarEmail(e, id)
			}
			return novo, err
		})
		if err != nil {
			return Resposta{}, err
		}
		return ok(res)

	case m == "POST" && p == "/backup":
		criado, err := a.arm.BackupDiario(ctx, a.hoje())
		if err != nil {
			return Resposta{}, err
		}
		return ok(Objeto{"criado": criado, "data": a.hoje()})

	case rePlataforma.MatchString(p) && (m == "PUT" || m == "PATCH"):
		return a.gravarPlataforma(ctx, rePlataforma.FindStringSubmatch(p)[1], c)

	case reRodada.MatchString(p) && m == "PUT":
		return a.gravarRodada(ctx, reRodada.FindStringSubmatch(p)[1], c)

	case reVagaAcao.MatchString(p):
		return a.rotasDaVaga(ctx, m, reVagaAcao.FindStringSubmatch(p), c, autor)
	}
	return Resposta{}, falha(404, "rota não encontrada")
}

func (a *API) rotasDeLeitura(ctx context.Context, p string, q map[string]string) (Resposta, bool, error) {
	if p == "/saude" {
		r, err := ok(Objeto{"ok": true, "app": VersaoApp})
		return r, true, err
	}
	if p == "/versao" {
		v, err := a.Versao(ctx)
		return Resposta{Status: 200, Corpo: Objeto{"versao": v}}, true, err
	}
	if p == "/curriculos" {
		return Resposta{Status: 200, Corpo: Objeto{"pasta": textoOuNulo(a.pastaCurriculos()), "arquivos": a.curriculos()}}, true, nil
	}
	if p == "/estado" {
		// Leia a versão antes dos campos. Uma conclusão simultânea torna a
		// próxima consulta de versão diferente e a tela relê o estado.
		v, err := a.Versao(ctx)
		if err != nil {
			return Resposta{}, true, err
		}
		concluido, err := a.arm.OnboardingConcluido(ctx)
		if err != nil {
			return Resposta{}, true, err
		}
		e, err := a.arm.Carregar(ctx)
		if err != nil {
			return Resposta{}, true, err
		}
		corpo := Objeto{
			"onboardingConcluido": concluido,
			"vagas":               e.Vagas, "eventos": e.Eventos, "plataformas": e.Plataformas, "rodadas": e.Rodadas, "perfil": e.Perfil,
			"pedidos": e.Pedidos, "emails": e.Emails, "hoje": a.hoje(), "versao": v, "curriculos": a.curriculos(),
			"pastaCurriculos": textoOuNulo(a.pastaCurriculos()), "primeirosPassos": a.primeirosPassos(e),
		}
		return Resposta{Status: 200, Corpo: corpo}, true, nil
	}
	e, err := a.arm.Carregar(ctx)
	if err != nil {
		return Resposta{}, true, err
	}
	hoje := a.hoje()
	var corpo any
	switch p {
	case "/resumo":
		corpo = dominio.CalcularResumo(e.Vagas, hoje)
	case "/vagas":
		l := []Objeto{}
		for _, v := range e.Vagas {
			if (q["etapa"] == "" || dominio.Igual(v["etapa"], q["etapa"])) && (q["plataforma"] == "" || dominio.Igual(v["plataforma"], q["plataforma"])) {
				l = append(l, v)
			}
		}
		corpo = l
	case "/duplicatas":
		var excluir any
		if x, tem := q["excluir"]; tem {
			excluir = x
		}
		corpo = dominio.DuplicatasPorTitulo(e.Vagas, Objeto{"empresa": q["empresa"], "titulo": q["titulo"]}, excluir)
	case "/followups":
		l := []Objeto{}
		for _, v := range dominio.PendentesFollowUp(e.Vagas, hoje) {
			x := Objeto{}
			for k, val := range v {
				x[k] = val
			}
			x["mensagem"] = dominio.SugerirMensagem(v, e.Perfil)
			l = append(l, x)
		}
		corpo = l
	case "/perfil":
		corpo = e.Perfil
	case "/plataformas":
		corpo = e.Plataformas
	case "/rodadas":
		corpo = e.Rodadas
	case "/pedidos":
		corpo = filtrar(e.Pedidos, func(x Objeto) bool { return q["status"] == "" || dominio.Igual(x["status"], q["status"]) })
	case "/emails":
		corpo = filtrar(e.Emails, func(x Objeto) bool {
			return (q["status"] == "" || dominio.Igual(x["status"], q["status"])) && (q["vaga"] == "" || dominio.Igual(x["vagaId"], q["vaga"]))
		})
	case "/primeiros-passos":
		corpo = Objeto{"passos": a.primeirosPassos(e)}
	case "/historico.md":
		return Resposta{Status: 200, Corpo: dominio.GerarHistoricoMd(e, hoje), Tipo: "text/markdown; charset=utf-8"}, true, nil
	case "/exportar.csv":
		return Resposta{Status: 200, Corpo: CSV(e.Vagas), Tipo: "text/csv; charset=utf-8",
			Cabecalhos: map[string]string{"content-disposition": `attachment; filename="candidaturas-` + hoje + `.csv"`}}, true, nil
	case "/validar":
		problemas := []string{}
		ids := map[string]bool{}
		for _, v := range e.Vagas {
			id := dominio.CampoJS(v, "id")
			if ids[id] {
				problemas = append(problemas, id+": id repetido")
			}
			ids[id] = true
			for _, erro := range dominio.ValidarVaga(v) {
				problemas = append(problemas, id+": "+erro)
			}
		}
		corpo = Objeto{"ok": len(problemas) == 0, "problemas": problemas, "vagas": len(e.Vagas), "eventos": len(e.Eventos)}
	default:
		g := reVagaID.FindStringSubmatch(p)
		if g == nil {
			return Resposta{}, false, nil
		}
		id, err := decodificar(g[1])
		if err != nil {
			return Resposta{}, true, err
		}
		for _, v := range e.Vagas {
			if dominio.Igual(v["id"], id) {
				x := Objeto{}
				for k, val := range v {
					x[k] = val
				}
				x["eventos"] = filtrar(e.Eventos, func(ev Objeto) bool { return dominio.Igual(ev["vagaId"], id) })
				return Resposta{Status: 200, Corpo: x}, true, nil
			}
		}
		return Resposta{}, true, falha(404, "vaga não encontrada: "+id)
	}
	return Resposta{Status: 200, Corpo: corpo}, true, nil
}

func filtrar(l []Objeto, f func(Objeto) bool) []Objeto {
	saida := []Objeto{}
	for _, x := range l {
		if f(x) {
			saida = append(saida, x)
		}
	}
	return saida
}

// Emails: { email, classificacao } ou { lista: [...] }. Cada item falha sozinho; repetido não duplica.
func (a *API) registrarEmails(ctx context.Context, c Objeto, autor string) (Resposta, error) {
	itens, lote := c["lista"].([]any)
	if !lote {
		itens = []any{c}
	}
	resultados := []Objeto{}
	err := a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		for _, item := range itens {
			o := dominio.ObjetoDe(item)
			novo, r, err := dominio.RegistrarEmail(e, dominio.ObjetoDe(o["email"]), dominio.ObjetoDe(o["classificacao"]), a.ctx(e, autor))
			var ev *dominio.ErroValidacao
			if err != nil {
				if !errors.As(err, &ev) {
					return e, err
				}
				resultados = append(resultados, Objeto{"idMensagem": dominio.Ou(dominio.ObjetoDe(o["email"])["idMensagem"], nil), "status": "erro", "erro": err.Error()})
				continue
			}
			e = novo
			resultados = append(resultados, r)
		}
		return e, nil
	})
	if err != nil {
		return Resposta{}, err
	}
	if lote {
		return ok(resultados)
	}
	status := 200
	if resultados[0]["status"] == "erro" {
		status = 400
	}
	return Resposta{Status: status, Corpo: resultados[0]}, nil
}

func (a *API) gravarPlataforma(ctx context.Context, bruto string, c Objeto) (Resposta, error) {
	id, err := decodificar(bruto)
	if err != nil {
		return Resposta{}, err
	}
	erros := []string{}
	for _, k := range camposPlatText {
		if v := c[k]; v != nil {
			if _, ehTexto := v.(string); !ehTexto {
				erros = append(erros, k+" deve ser texto")
			}
		}
	}
	if len(erros) > 0 {
		return Resposta{}, &dominio.ErroValidacao{Erros: erros}
	}
	if dominio.ContemDocumentoEm(c) {
		return Resposta{}, &dominio.ErroValidacao{Erros: []string{"parece conter CPF/RG — documentos não são guardados na central"}}
	}
	err = a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		nova := Objeto{"nome": id}
		pos := -1
		for i, x := range e.Plataformas {
			if dominio.Igual(x["id"], id) {
				nova, pos = Objeto{}, i
				for k, v := range x {
					nova[k] = v
				}
				break
			}
		}
		for k, v := range c {
			nova[k] = v
		}
		nova["id"] = id
		if pos >= 0 {
			e.Plataformas[pos] = nova
		} else {
			e.Plataformas = append(e.Plataformas, nova)
		}
		return e, nil
	})
	if err != nil {
		return Resposta{}, err
	}
	return ok(Objeto{"ok": true, "id": id})
}

func (a *API) gravarRodada(ctx context.Context, bruto string, c Objeto) (Resposta, error) {
	s, err := decodificar(bruto)
	if err != nil {
		return Resposta{}, err
	}
	n, finito := dominio.NumeroDe(s)
	if !finito || n-n != 0 {
		return Resposta{}, &dominio.ErroValidacao{Erros: []string{"número da rodada inválido"}}
	}
	if dominio.ContemDocumentoEm(c) {
		return Resposta{}, &dominio.ErroValidacao{Erros: []string{"parece conter CPF/RG — documentos não são guardados na central"}}
	}
	var final Objeto
	err = a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		nova := Objeto{"data": a.hoje()}
		rodadas := []Objeto{}
		achou := false
		for _, x := range e.Rodadas {
			if !dominio.Igual(x["n"], n) {
				rodadas = append(rodadas, x)
				continue
			}
			if !achou { // como o find do Node: só a primeira com o mesmo número entra na mescla
				for k, v := range x {
					nova[k] = v
				}
				achou = true
			}
		}
		for k, v := range c {
			nova[k] = v
		}
		nova["n"] = n
		rodadas = append(rodadas, nova)
		sort.SliceStable(rodadas, func(i, j int) bool {
			a, _ := dominio.NumeroDe(rodadas[i]["n"])
			b, _ := dominio.NumeroDe(rodadas[j]["n"])
			return a < b
		})
		e.Rodadas = rodadas
		final = nova
		return e, nil
	})
	if err != nil {
		return Resposta{}, err
	}
	return ok(final)
}

func (a *API) rotasDaVaga(ctx context.Context, m string, g []string, c Objeto, autor string) (Resposta, error) {
	id, err := decodificar(g[1])
	if err != nil {
		return Resposta{}, err
	}
	acao := g[2]
	var vaga Objeto
	executar := func(op func(e dominio.Estado, cx dominio.Ctx) (dominio.Estado, Objeto, error)) (Objeto, error) {
		err := a.gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
			novo, v, err := op(e, a.ctx(e, autor))
			vaga = v
			return novo, err
		})
		return vaga, err
	}
	switch {
	case m == "POST" && (acao == "/iniciar-manual" || acao == "/confirmar-envio-manual"):
		if err := soCampos(c); err != nil {
			return Resposta{}, err
		}
		if autor != "eu" {
			return Resposta{}, falha(403, "somente a pessoa pode iniciar ou confirmar a candidatura")
		}
		v, err := executar(func(e dominio.Estado, cx dominio.Ctx) (dominio.Estado, Objeto, error) {
			if acao == "/confirmar-envio-manual" {
				return dominio.ConfirmarManualRadar(e, id, a.relogio(), cx)
			}
			return a.iniciarManualRadar(e, id, cx)
		})
		if err != nil {
			return Resposta{}, err
		}
		return ok(v)
	case m == "PATCH" && acao == "":
		v, err := executar(func(e dominio.Estado, cx dominio.Ctx) (dominio.Estado, Objeto, error) {
			return dominio.AtualizarVaga(e, id, c, cx)
		})
		if err != nil {
			return Resposta{}, err
		}
		return ok(v)
	case m == "POST" && acao == "/mover":
		extras := Objeto{}
		for k, v := range c {
			if k != "etapa" {
				extras[k] = v
			}
		}
		v, err := executar(func(e dominio.Estado, cx dominio.Ctx) (dominio.Estado, Objeto, error) {
			return dominio.MoverVaga(e, id, c["etapa"], extras, cx)
		})
		if err != nil {
			return Resposta{}, err
		}
		return ok(v)
	case m == "POST" && acao == "/eventos":
		v, err := executar(func(e dominio.Estado, cx dominio.Ctx) (dominio.Estado, Objeto, error) {
			return dominio.RegistrarEvento(e, id, dominio.OuV(c["tipo"], "nota"), c["texto"], cx)
		})
		if err != nil {
			return Resposta{}, err
		}
		return criado(v)
	case m == "POST" && acao == "/followup":
		switch c["acao"] {
		case "feito":
			v, err := executar(func(e dominio.Estado, cx dominio.Ctx) (dominio.Estado, Objeto, error) {
				cx.Texto = dominio.Texto(c["texto"])
				return dominio.FollowUpFeito(e, id, cx)
			})
			if err != nil {
				return Resposta{}, err
			}
			return ok(v)
		case "adiar":
			dias, ok2 := dominio.NumeroDe(c["dias"])
			if !ok2 || dias == 0 {
				dias = 3
			}
			v, err := executar(func(e dominio.Estado, cx dominio.Ctx) (dominio.Estado, Objeto, error) {
				return dominio.AdiarFollowUp(e, id, int(dias), cx)
			})
			if err != nil {
				return Resposta{}, err
			}
			return ok(v)
		}
		return Resposta{}, falha(400, `acao deve ser "feito" ou "adiar"`)
	}
	return Resposta{}, falha(404, "rota não encontrada")
}

// CSV exporta as vagas (separador ";" e BOM para o Excel abrir com acento).
func CSV(vagas []Objeto) string {
	cols := []string{"id", "empresa", "titulo", "plataforma", "etapa", "subEtapa", "aderencia", "contrato", "motivo", "pendencia", "dataEnvio", "rodada", "pretensao", "proximoFollowUp", "link", "notas"}
	valor := func(v Objeto, c string) any {
		var calc any
		switch c {
		case "dataEnvio":
			calc = dominio.ObjetoDe(v["envio"])["data"]
		case "rodada":
			calc = dominio.ObjetoDe(v["envio"])["rodada"]
		case "pretensao":
			calc = dominio.ObjetoDe(v["envio"])["pretensao"]
		case "proximoFollowUp":
			calc = dominio.ObjetoDe(v["followUp"])["proximo"]
		}
		return dominio.Ou(calc, dominio.Ou(v[c], ""))
	}
	// Apóstrofo na frente de =, +, -, @ impede o Excel de tratar o texto como fórmula.
	esc := func(x any) string {
		s := dominio.JSString(x)
		if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
			s = "'" + s
		}
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	linhas := []string{strings.Join(cols, ";")}
	for _, v := range vagas {
		campos := make([]string, len(cols))
		for i, c := range cols {
			campos[i] = esc(valor(v, c))
		}
		linhas = append(linhas, strings.Join(campos, ";"))
	}
	return string(rune(0xFEFF)) + strings.Join(linhas, "\n")
}
