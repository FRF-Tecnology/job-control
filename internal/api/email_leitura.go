package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"html"
	"log"
	"maps"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/FRF-Tecnology/job-control/internal/busca"
	"github.com/FRF-Tecnology/job-control/internal/caixa"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// "Ler respostas do e-mail" (POST /email/ler): o app lê a caixa conectada (IMAP, só leitura), acha os e-mails de
// processo seletivo, pede à IA o tipo de cada um e registra nas vagas. Regras da proposta do #133 (CyberSec):
//   - e-mail de segurança da conta ou de dinheiro sai antes da IA e não fica guardado (nem o id);
//   - fora do pré-filtro: só o id, sem IA;
//   - para a IA vão só o endereço do remetente, o assunto e um trecho de até 300 caracteres, com números, links e
//     endereços mascarados; das vagas, só as candidatas;
//   - recusa e oferta (e convite de outro domínio) ficam "a-confirmar" (dominio.RegistrarEmailLido);
//   - o log leva só contagens.
//
// Alertas de vaga (LinkedIn e Indeed, remetentes da lista fechada de busca.FonteDoAlerta) não vão para a IA: as vagas
// saem do link (busca.VagasDoAlerta) e entram no Radar como "avaliação pendente"; a próxima busca as avalia. Do
// alerta fica guardado só o id.

const (
	chaveMetaLeitura    = "email-leitura" // {ultimaLeitura: AAAA-MM-DD}
	maxEmailsPorLeitura = 50
	maxIAPorLeitura     = 30
	prazoLeituraEmail   = 8 * time.Minute
	diasPrimeiraLeitura = 30
	tetoTrecho          = 300
	tetoTextoLocal      = 20000 // o texto inteiro só serve para achar a vaga, aqui no computador
	// Alertas: só os da última semana (vaga de alerta mais velha que isso costuma já ter fechado) e um teto de vagas
	// novas por leitura, para o Radar não encher (a IA avalia dez por rodada).
	maxAlertasPorLeitura = 30
	diasDeAlerta         = 7
	maxVagasDeAlerta     = 60
)

var buscaAlertas = caixa.Busca{Remetentes: []string{"jobalerts-noreply@linkedin.com", "jobs-listings@linkedin.com", "indeed.com"}}

// O que procurar no servidor: plataformas de vaga e testes no remetente, palavras de processo no assunto e no corpo.
var buscaRespostas = caixa.Busca{
	Remetentes: []string{"greenhouse", "lever.co", "ashbyhq", "gupy", "inhire", "workable", "smartrecruiters", "myworkday",
		"teamtailor", "recruitee", "bamboohr", "breezy", "jobvite", "taleo", "successfactors", "kenoby", "solides",
		"pandape", "hackerrank", "codility", "coderbyte", "testgorilla", "codesignal", "jobs-noreply@linkedin.com"},
	Assuntos: []string{"application", "applying", "candidatura", "entrevista", "interview", "assessment", "offer",
		"proposta", "processo seletivo", "next step", "unfortunately", "infelizmente"},
	Corpo: []string{"infelizmente", "unfortunately", "processo seletivo"},
}

var (
	// Segurança da conta e dinheiro: no remetente, no assunto ou no começo do texto.
	reSensivel = regexp.MustCompile(`(?i)c[oó]digo de (verifica|seguran|acesso|confirma)|verification code|security code|one[- ]time (code|password)|\botp\b|\b2fa\b|two[- ]factor|dois fatores|duas etapas|redefin\w* (a |sua )?senha|alterar (a |sua )?senha|reset (your )?password|password reset|novo (acesso|login)|new (sign[- ]in|login)|alerta de seguran|security alert|\bpix\b|fatura|boleto|invoice|extrato|cart[aã]o de cr[eé]dito|credit card|\bbanco\b|\bbank\b`)
	// Remetentes de conta e de banco (lista fechada).
	reRemetenteSensivel = regexp.MustCompile(`(?i)accounts\.google\.com|@(.+\.)?(nubank|itau|bradesco|santander|caixa|bancointer|inter|c6bank|btgpactual|mercadopago|picpay|paypal|wise)\.`)
	reBancoDeTalentos   = regexp.MustCompile(`(?i)banco de talentos|talent (pool|bank)`)

	reTag       = regexp.MustCompile(`(?s)<[^>]*>`)
	reBlocoHTML = regexp.MustCompile(`(?is)<(script|style|head)[^>]*>.*?</(script|style|head)>`)
	reBrancos   = regexp.MustCompile(`\s+`)
	reLink      = regexp.MustCompile(`(?i)\b(https?://|www\.)\S+`)
	reEndereco  = regexp.MustCompile(`[^\s<>@]+@[^\s<>@]+\.[a-zA-Z]{2,}`)
	reCPF       = regexp.MustCompile(`\b\d{3}\.?\d{3}\.?\d{3}-?\d{2}\b`)
	reCartao    = regexp.MustCompile(`\b(?:\d[ -]?){13,19}\b`)
	reCodigo    = regexp.MustCompile(`\b\d{4,8}\b`)
	reAnguloes  = regexp.MustCompile(`[<>]`)
)

// textoDoEmail: o texto simples do e-mail (ou o HTML sem marcação), numa linha só.
func textoDoEmail(m caixa.Mensagem) string {
	t := m.Texto
	if strings.TrimSpace(t) == "" {
		t = html.UnescapeString(reTag.ReplaceAllString(reBlocoHTML.ReplaceAllString(m.HTML, " "), " "))
	}
	return cortarRunas(strings.TrimSpace(reBrancos.ReplaceAllString(t, " ")), tetoTextoLocal)
}

func cortarRunas(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// mascarar troca links, endereços, CPF, cartão e códigos numéricos antes de o texto sair do computador.
func mascarar(s string) string {
	s = reLink.ReplaceAllString(s, "[link]")
	s = reEndereco.ReplaceAllString(s, "[e-mail]")
	s = reCPF.ReplaceAllString(s, "•••")
	s = reCartao.ReplaceAllString(s, "•••")
	return reCodigo.ReplaceAllString(s, "•••")
}

// sensivel: e-mail de segurança da conta ou de dinheiro. Nem chega à IA nem fica guardado.
func sensivel(de, assunto, texto string) bool {
	if reRemetenteSensivel.MatchString(de) {
		return true
	}
	alvo := reBancoDeTalentos.ReplaceAllString(de+"\n"+assunto+"\n"+cortarRunas(texto, 500), " ")
	return reSensivel.MatchString(alvo)
}

// textoDaIA: resumo e próximo passo vêm da IA (não confiável): teto, sem link, sem marcação.
func textoDaIA(v any) any {
	s, ok := v.(string)
	if !ok {
		return v
	}
	s = reAnguloes.ReplaceAllString(reLink.ReplaceAllString(s, "[link]"), "")
	return cortarRunas(strings.TrimSpace(s), tetoTrecho)
}

// idDoEmail: o Message-ID; sem ele, um id estável feito da data, do remetente e do assunto.
func idDoEmail(m caixa.Mensagem) string {
	if id := strings.TrimSpace(m.IDMensagem); id != "" {
		return cortarRunas(id, 250)
	}
	h := sha256.Sum256([]byte(m.Data.UTC().Format(time.RFC3339) + "\n" + m.De + "\n" + m.Assunto))
	return "imap:" + hex.EncodeToString(h[:12])
}

// ResultadoLeitura é a resposta do POST /email/ler: só contagens.
type ResultadoLeitura struct {
	Lidos         int    `json:"lidos"`
	JaLidos       int    `json:"jaLidos"`
	Sensiveis     int    `json:"sensiveis"`
	Ignorados     int    `json:"ignorados"`
	Mudaram       int    `json:"mudaram"`
	AConfirmar    int    `json:"aConfirmar"`
	SemVaga       int    `json:"semVaga"`
	Registrados   int    `json:"registrados"`
	Pendentes     int    `json:"pendentes"`
	Falhas        int    `json:"falhas"`
	Alertas       int    `json:"alertas"`      // alertas de vaga lidos
	VagasNoRadar  int    `json:"vagasNoRadar"` // vagas novas que os alertas puseram no Radar
	UltimaLeitura string `json:"ultimaLeitura"`
}

type leituraEmail struct {
	UltimaLeitura string `json:"ultimaLeitura"`
}

func (a *API) ultimaLeituraEmail(ctx context.Context) string {
	var l leituraEmail
	if _, err := a.arm.LerMeta(ctx, chaveMetaLeitura, &l); err != nil {
		return ""
	}
	return l.UltimaLeitura
}

// contaDeEmail devolve a conta conectada, com a senha do cofre.
func (a *API) contaDeEmail(ctx context.Context) (caixa.Conta, bool, error) {
	var conta contaEmail
	if _, err := a.arm.LerMeta(ctx, chaveMetaEmail, &conta); err != nil {
		return caixa.Conta{}, false, err
	}
	senha, err := a.cofre.Ler(chaveCofreEmail)
	if err != nil || conta.Usuario == "" || senha == "" {
		return caixa.Conta{}, false, err
	}
	return caixa.Conta{Usuario: conta.Usuario, Servidor: conta.Servidor, Senha: senha}, true, nil
}

func (a *API) lerRespostasEmail(ctx context.Context, c Objeto) (Resposta, error) {
	if err := soCampos(c); err != nil {
		return Resposta{}, err
	}
	if !a.lendoEmail.TryLock() {
		return Resposta{}, &dominio.ErroConflito{Msg: "o app já está lendo o e-mail", Motivo: "lendo"}
	}
	defer a.lendoEmail.Unlock()

	conta, conectada, err := a.contaDeEmail(ctx)
	if err != nil {
		return Resposta{}, err
	}
	if !conectada {
		return Resposta{}, &dominio.ErroConflito{Msg: "conecte seu e-mail em Perfil e IA", Motivo: "sem-conta"}
	}
	cfg, err := a.configIA(ctx)
	if err != nil {
		return Resposta{}, err
	}
	if cfg.Rotas["padrao"].Provedor == "" && cfg.Rotas["classificarEmail"].Provedor == "" {
		return Resposta{}, &dominio.ErroConflito{Msg: "conecte uma IA para ler as respostas", Motivo: "sem-ia"}
	}
	motor, err := a.novaIA(ctx)
	if err != nil {
		return Resposta{}, err
	}

	hoje := a.hoje()
	desde, _ := time.Parse("2006-01-02", hoje)
	desde = desde.AddDate(0, 0, -diasPrimeiraLeitura)
	if u, err := time.Parse("2006-01-02", a.ultimaLeituraEmail(ctx)); err == nil {
		desde = u.AddDate(0, 0, -1) // o SINCE do IMAP é por dia; o id não deixa repetir
	}

	prazo, cancelar := context.WithTimeout(ctx, prazoLeituraEmail)
	defer cancelar()
	msgs, err := a.leitorEmail.Buscar(prazo, conta, desde, buscaRespostas, maxEmailsPorLeitura)
	if err == nil {
		desdeAlerta, _ := time.Parse("2006-01-02", hoje)
		if desdeAlerta = desdeAlerta.AddDate(0, 0, -diasDeAlerta); desdeAlerta.Before(desde) {
			desdeAlerta = desde
		}
		var alertas []caixa.Mensagem
		alertas, err = a.leitorEmail.Buscar(prazo, conta, desdeAlerta, buscaAlertas, maxAlertasPorLeitura)
		msgs = juntarMensagens(msgs, alertas)
	}
	switch {
	case errors.Is(err, caixa.ErrSenha):
		return Resposta{}, &dominio.ErroConflito{Msg: "o e-mail recusou a senha de app: conecte de novo em Perfil e IA", Motivo: "senha"}
	case err != nil:
		log.Printf("email: leitura falhou (%s)", tipoDeFalhaEmail(err))
		return Resposta{}, falha(502, "não consegui ler o e-mail agora: confira a internet e tente de novo")
	}
	sort.SliceStable(msgs, func(i, j int) bool { return msgs[i].Data.Before(msgs[j].Data) }) // na ordem em que chegaram

	e, err := a.arm.Carregar(ctx)
	if err != nil {
		return Resposta{}, err
	}
	registrados := map[string]bool{}
	for _, x := range e.Emails {
		registrados[dominio.Texto(x["idMensagem"])] = true
	}

	r := ResultadoLeitura{Lidos: len(msgs)}
	limiteAlerta, _ := time.Parse("2006-01-02", hoje)
	limiteAlerta = limiteAlerta.AddDate(0, 0, -diasDeAlerta)
	chamadas := 0
	for _, m := range msgs {
		id := idDoEmail(m)
		if registrados[id] {
			r.JaLidos++
			continue
		}
		de := caixa.SoEndereco(m.De)
		if fonte := busca.FonteDoAlerta(de); fonte != "" {
			// O corpo de um alerta é uma lista de vagas (e pode citar "banco" como empresa): o filtro de sensíveis
			// olha só o assunto. Nada do alerta vai para a IA.
			if reSensivel.MatchString(m.Assunto) {
				r.Sensiveis++
				continue
			}
			var vagas []Objeto
			if !m.Data.Before(limiteAlerta) && r.VagasNoRadar < maxVagasDeAlerta {
				vagas = busca.VagasDoAlerta(fonte, m.Texto, m.HTML)
			}
			novas := 0
			err := a.gravar(ctx, func(atual dominio.Estado) (dominio.Estado, error) {
				novo, n, err := a.registrarAlerta(atual, id, m.Data, vagas, maxVagasDeAlerta-r.VagasNoRadar)
				novas, e = n, novo
				return novo, err
			})
			if err != nil {
				return Resposta{}, err
			}
			registrados[id] = true
			r.Alertas++
			r.VagasNoRadar += novas
			continue
		}
		texto := textoDoEmail(m)
		if sensivel(de, m.Assunto, texto) {
			r.Sensiveis++
			continue
		}
		email := Objeto{"idMensagem": id, "de": cortarRunas(de, 200), "assunto": cortarRunas(strings.TrimSpace(m.Assunto), 300),
			"data": m.Data.Format(time.RFC3339), "texto": texto}
		cls := Objeto{"tipo": "outro"}
		if dominio.PassaPreFiltro(email) {
			if chamadas >= maxIAPorLeitura || prazo.Err() != nil {
				r.Pendentes++
				continue
			}
			chamadas++
			vagas := []any{}
			for _, v := range dominio.VagasCandidatasDoEmail(e.Vagas, email) {
				vagas = append(vagas, Objeto{"id": v["id"], "empresa": v["empresa"], "titulo": v["titulo"]})
			}
			entrada := Objeto{"email": Objeto{"de": email["de"], "assunto": mascarar(dominio.Texto(email["assunto"])),
				"texto": cortarRunas(mascarar(texto), tetoTrecho)}, "vagas": vagas}
			saida, err := motor.Executar(prazo, "classificarEmail", entrada)
			if err != nil {
				r.Falhas++ // não registra: a próxima leitura tenta de novo
				continue
			}
			cls = Objeto{"tipo": saida["tipo"], "vagaId": saida["vagaId"], "resumo": textoDaIA(saida["resumo"])}
			if p := textoDaIA(saida["proximoPasso"]); p != "" && p != nil {
				cls["proximoPasso"] = p
			}
			// Sugestão para criar a vaga quando ela não está no app (o domínio limpa e só guarda se ficar sem vaga).
			if saida["vagaId"] == nil {
				cls["empresa"], cls["cargo"] = saida["empresa"], saida["cargo"]
			}
		}
		var res Objeto
		err := a.gravar(ctx, func(atual dominio.Estado) (dominio.Estado, error) {
			novo, x, err := dominio.RegistrarEmailLido(atual, email, cls, a.ctx(atual, "eu"))
			res = x
			e = novo
			return novo, err
		})
		var ev *dominio.ErroValidacao
		if errors.As(err, &ev) {
			r.Falhas++ // ex.: documento no assunto; nada guardado
			continue
		}
		if err != nil {
			return Resposta{}, err
		}
		registrados[id] = true
		switch res["status"] {
		case "ignorado":
			r.Ignorados++
			continue
		case "a-confirmar":
			r.AConfirmar++
		case "sem-vaga":
			r.SemVaga++
		}
		if res["transicao"] != nil {
			r.Mudaram++
		}
		r.Registrados++
	}

	r.UltimaLeitura = a.ultimaLeituraEmail(ctx)
	if r.Pendentes == 0 && r.Falhas == 0 {
		if err := a.arm.GravarMeta(ctx, chaveMetaLeitura, leituraEmail{UltimaLeitura: hoje}); err != nil {
			return Resposta{}, err
		}
		r.UltimaLeitura = hoje
	}
	log.Printf("email: lidos %d, ja-lidos %d, sensiveis %d, ignorados %d, registrados %d, mudaram %d, a-confirmar %d, sem-vaga %d, pendentes %d, falhas %d, alertas %d, vagas-no-radar %d",
		r.Lidos, r.JaLidos, r.Sensiveis, r.Ignorados, r.Registrados, r.Mudaram, r.AConfirmar, r.SemVaga, r.Pendentes, r.Falhas, r.Alertas, r.VagasNoRadar)
	return ok(r)
}

// juntarMensagens: as respostas e os alertas, sem repetir o mesmo e-mail.
func juntarMensagens(listas ...[]caixa.Mensagem) []caixa.Mensagem {
	vistos := map[string]bool{}
	var saida []caixa.Mensagem
	for _, l := range listas {
		for _, m := range l {
			if id := idDoEmail(m); !vistos[id] {
				vistos[id] = true
				saida = append(saida, m)
			}
		}
	}
	return saida
}

// registrarAlerta põe no Radar do perfil de busca ativo as vagas do alerta que ainda não estão no app (o mesmo
// anúncio em qualquer etapa, de qualquer perfil, fica de fora) e guarda só o id do e-mail. As vagas entram como
// "avaliação pendente", com a data do alerta como publicação: a próxima busca as avalia como as dos sites.
func (a *API) registrarAlerta(e dominio.Estado, id string, data time.Time, vagas []Objeto, resta int) (dominio.Estado, int, error) {
	ativo := e.PerfilBuscaAtivo()
	cx := a.ctx(e, "claude")
	novas := 0
	for _, bruta := range vagas {
		if novas >= resta {
			break
		}
		dados := maps.Clone(bruta)
		anuncio := dominio.Texto(dados["id"])
		dados["anuncioId"], dados["perfilBuscaId"] = anuncio, ativo
		if ativo != dominio.PerfilOriginal {
			dados["id"] = ativo + "-" + anuncio
		}
		if !data.IsZero() {
			dados["publicadaEm"] = data.Format("2006-01-02")
		}
		dados["triagemBusca"] = "pendente"
		if slices.ContainsFunc(e.Vagas, func(v Objeto) bool { return busca.MesmoAnuncio(v, dados) }) || dominio.AnuncioReservado(e, dados) != nil {
			continue
		}
		novo, _, err := dominio.AdicionarVaga(e, dados, cx, true)
		if err != nil {
			var d *dominio.ErroDuplicata
			var invalida *dominio.ErroValidacao
			if errors.As(err, &d) || errors.As(err, &invalida) {
				continue
			}
			return e, 0, err
		}
		e = novo
		novas++
	}
	// Só o id do alerta: sem remetente, assunto nem texto.
	novo, _, err := dominio.RegistrarEmail(e, Objeto{"idMensagem": id, "data": data.Format(time.RFC3339)}, Objeto{"tipo": "outro"}, a.ctx(e, "eu"))
	return novo, novas, err
}

// tipoDeFalhaEmail: só a classe da falha vai para o log (o texto do erro pode trazer o servidor e o endereço).
func tipoDeFalhaEmail(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "prazo"
	case errors.Is(err, caixa.ErrConexao):
		return "conexao"
	}
	return "outro"
}
