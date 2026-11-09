// Package executor envia as candidaturas aprovadas (P5, jc-a2y.6.2.3). É código interno, sem rota HTTP: só o app
// chama. Regras da proposta docs/propostas/p5-contrato-fila.md:
//   - M1: tudo o que vai para a página sai do pacote aprovado; campo obrigatório fora dele bloqueia (pergunta-nova);
//   - M2: a entrada em enviando é a conferência atômica do domínio (IniciarEnvio, numa transação); antes de anexar,
//     o currículo é lido de novo e o sha256 tem de bater, senão falhou/curriculo-mudou;
//   - M3: depois do clique em enviar, sem confirmação, a candidatura fica incerta; nunca há nova tentativa;
//   - M5: a página tem de parar no site da plataforma e na mesma vaga, senão bloqueada/site-externo;
//   - "outra" plataforma nunca abre página: vai direto para bloqueada/site-externo;
//   - intervalo com variação entre as ações na página e entre uma candidatura e outra;
//   - M7: o log leva só id, estado, motivo e o tipo do erro.
package executor

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/armazem"
	"github.com/FRF-Tecnology/job-control/internal/curriculos"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
)

// Intervalo: espera sorteada entre Min e Max.
type Intervalo struct{ Min, Max time.Duration }

// Opcoes do executor. Os campos de função só mudam nos testes.
type Opcoes struct {
	Navegador         Navegador
	PastaCurriculos   string
	EntreAcoes        Intervalo // padrão 0,8 s a 2,5 s
	EntreCandidaturas Intervalo // padrão 30 s a 90 s
	Relogio           func() time.Time
	Hoje              func() string
	Esperar           func(ctx context.Context, d time.Duration) error
	Sortear           func() float64 // de 0 a 1
	Log               func(formato string, args ...any)
}

// Executor envia, uma por vez, as candidaturas aprovadas.
type Executor struct {
	arm *armazem.Armazem
	o   Opcoes
	// antesDeIniciar só existe para o teste agir entre "pegou a aprovada" e "entra em enviando".
	antesDeIniciar func()
}

// Desfecho de um envio: o estado final e, se houver, o motivo (bloqueio) ou o código da falha.
type Desfecho struct {
	ID     string
	Estado string
	Motivo string
}

// Autor dos eventos da vaga gravados pelo executor (enviada, bloqueada). Não é a pessoa: não aprova nada.
const autorExecutor = "executor"

// Novo cria o executor sobre o armazém.
func Novo(arm *armazem.Armazem, o Opcoes) *Executor {
	if o.EntreAcoes == (Intervalo{}) {
		o.EntreAcoes = Intervalo{800 * time.Millisecond, 2500 * time.Millisecond}
	}
	if o.EntreCandidaturas == (Intervalo{}) {
		o.EntreCandidaturas = Intervalo{30 * time.Second, 90 * time.Second}
	}
	if o.Relogio == nil {
		o.Relogio = time.Now
	}
	if o.Hoje == nil {
		o.Hoje = func() string { return dominio.HojeISO(time.Now()) }
	}
	if o.Esperar == nil {
		o.Esperar = esperar
	}
	if o.Sortear == nil {
		o.Sortear = rand.Float64
	}
	if o.Log == nil {
		o.Log = log.Printf
	}
	return &Executor{arm: arm, o: o}
}

func esperar(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func (x *Executor) pausa(ctx context.Context, i Intervalo) error {
	d := i.Min
	if i.Max > i.Min {
		d += time.Duration(x.o.Sortear() * float64(i.Max-i.Min))
	}
	return x.o.Esperar(ctx, d)
}

func (x *Executor) ctx(e dominio.Estado) dominio.Ctx {
	dias, _ := dominio.NumeroDe(e.Perfil["diasFollowUp"])
	if dias != dias || dias <= 0 {
		dias = 7
	}
	return dominio.Ctx{Autor: autorExecutor, Agora: x.o.Hoje(), DiasFollowUp: int(dias)}
}

// gravar roda a operação do domínio numa transação. Usa um contexto que não cancela: o desfecho de um envio que já
// começou tem de ficar gravado mesmo se o app estiver fechando.
func (x *Executor) gravar(ctx context.Context, op func(e dominio.Estado, d dominio.Ctx) (dominio.Estado, dominio.Objeto, error)) (dominio.Objeto, error) {
	return x.gravarCom(ctx, x.arm.Gravar, op)
}

// gravarSeLiberado: o mesmo, mas com o envio pausado nada é gravado (armazem.ErrEnvioPausado), na mesma transação.
// É o caminho de toda ação que começa um envio (jc-a2y.51).
func (x *Executor) gravarSeLiberado(ctx context.Context, op func(e dominio.Estado, d dominio.Ctx) (dominio.Estado, dominio.Objeto, error)) (dominio.Objeto, error) {
	return x.gravarCom(ctx, x.arm.GravarSeEnvioLiberado, op)
}

func (x *Executor) gravarCom(ctx context.Context, gravar func(context.Context, func(dominio.Estado) (dominio.Estado, error)) error, op func(e dominio.Estado, d dominio.Ctx) (dominio.Estado, dominio.Objeto, error)) (dominio.Objeto, error) {
	var res dominio.Objeto
	err := gravar(context.WithoutCancel(ctx), func(e dominio.Estado) (dominio.Estado, error) {
		novo, o, err := op(e, x.ctx(e))
		res = o
		return novo, err
	})
	return res, err
}

// Iniciar roda ao abrir o app: enviando vira incerta (M3) e aprovação vencida volta a preparada, numa transação.
func (x *Executor) Iniciar(ctx context.Context) (incertas, vencidas int, err error) {
	agora := x.o.Relogio()
	err = x.arm.Gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		e, incertas = dominio.MarcarIncertas(e, agora)
		e, vencidas = dominio.VencerAprovacoes(e, agora)
		return e, nil
	})
	if err == nil && (incertas > 0 || vencidas > 0) {
		x.o.Log("executor: ao abrir, %d incerta(s) e %d aprovação(ões) vencida(s)", incertas, vencidas)
	}
	// Ao abrir o app, o envio começa sempre PAUSADO (recomendação do CyberSec no #59): quem retomou e fechou o app
	// não volta a enviar sem ninguém na frente. Só a tela retoma.
	if err == nil {
		err = x.arm.GravarEnvioPausado(ctx, true)
	}
	return incertas, vencidas, err
}

// EnviarAprovadas envia as aprovadas, da aprovação mais antiga para a mais nova, com intervalo entre uma e outra.
// Limite atingido numa plataforma pula as outras dela; conflito (mudou no meio) pula só aquela.
func (x *Executor) EnviarAprovadas(ctx context.Context) ([]Desfecho, error) {
	e, err := x.arm.Carregar(ctx)
	if err != nil {
		return nil, err
	}
	var fila []dominio.Objeto
	for _, o := range e.Candidaturas {
		if dominio.Igual(o["estado"], dominio.CandAprovada) {
			fila = append(fila, o)
		}
	}
	sort.SliceStable(fila, func(i, j int) bool {
		return dominio.Texto(dominio.ObjetoDe(fila[i]["aprovacao"])["em"]) < dominio.Texto(dominio.ObjetoDe(fila[j]["aprovacao"])["em"])
	})
	desfechos := []Desfecho{}
	semLimite := map[string]bool{}
	abriuPagina := false
	for _, o := range fila {
		if ctx.Err() != nil || x.arm.EnvioPausado(ctx) { // pausado: para a fila na hora (e de novo na transação)
			break
		}
		plataforma := dominio.Texto(o["plataforma"])
		if semLimite[plataforma] {
			continue
		}
		_, automatico := dominio.LimiteDe(nil, plataforma)
		if automatico && x.limiteAtingido(ctx, plataforma) { // não espera à toa por uma que vai dar 429
			semLimite[plataforma] = true
			x.o.Log("executor: %s pulada (limite diário de %s)", o["id"], plataforma)
			continue
		}
		if automatico && abriuPagina {
			if err := x.pausa(ctx, x.o.EntreCandidaturas); err != nil {
				break
			}
		}
		d, err := x.Enviar(ctx, dominio.Texto(o["id"]))
		var el *dominio.ErroLimite
		var ec *dominio.ErroConflito
		switch {
		case errors.As(err, &el):
			semLimite[plataforma] = true
			x.o.Log("executor: %s pulada (limite diário de %s)", o["id"], plataforma)
			continue
		case errors.As(err, &ec):
			x.o.Log("executor: %s pulada (conflito)", o["id"])
			continue
		case errors.Is(err, armazem.ErrEnvioPausado):
			x.o.Log("executor: envio pausado")
			return desfechos, nil
		case err != nil:
			return desfechos, err
		}
		abriuPagina = abriuPagina || automatico
		desfechos = append(desfechos, d)
	}
	return desfechos, nil
}

// limiteAtingido é só leitura, para pular antes de esperar; quem decide é a conferência atômica (IniciarEnvio).
func (x *Executor) limiteAtingido(ctx context.Context, plataforma string) bool {
	e, err := x.arm.Carregar(ctx)
	if err != nil {
		return false
	}
	limites, err := x.arm.LimitesCandidatura(ctx)
	if err != nil {
		return false
	}
	n, _ := dominio.LimiteDe(limites, plataforma)
	return dominio.UsadasNoDia(e, plataforma, x.o.Relogio()) >= n
}

// Enviar envia uma candidatura aprovada. Erro sem desfecho = nada mudou (conflito, limite ou falha ao gravar a
// entrada em enviando). Com desfecho, o estado final já está gravado.
func (x *Executor) Enviar(ctx context.Context, id string) (Desfecho, error) {
	e, err := x.arm.Carregar(ctx)
	if err != nil {
		return Desfecho{}, err
	}
	var c dominio.Objeto
	for _, o := range e.Candidaturas {
		if dominio.Igual(o["id"], id) {
			c = o
		}
	}
	if c == nil {
		return Desfecho{}, &dominio.ErroNaoEncontrado{Msg: "candidatura não encontrada: " + id}
	}
	agora := x.o.Relogio()
	plataforma := dominio.Texto(c["plataforma"])

	// "outra": sem envio automático, nem abre a página.
	if _, automatico := dominio.LimiteDe(nil, plataforma); !automatico {
		if _, err := x.gravarSeLiberado(ctx, func(e dominio.Estado, d dominio.Ctx) (dominio.Estado, dominio.Objeto, error) {
			return dominio.EncaminharExterna(e, id, agora, d)
		}); err != nil {
			return Desfecho{}, err
		}
		x.o.Log("executor: %s bloqueada (site-externo)", id)
		return Desfecho{ID: id, Estado: dominio.CandBloqueada, Motivo: "site-externo"}, nil
	}

	// M2: conferência atômica e consumo da aprovação, numa transação.
	limites, err := x.arm.LimitesCandidatura(ctx)
	if err != nil {
		return Desfecho{}, err
	}
	versao, _ := dominio.NumeroDe(c["versao"])
	if x.antesDeIniciar != nil {
		x.antesDeIniciar()
	}
	if _, err := x.gravarSeLiberado(ctx, func(e dominio.Estado, _ dominio.Ctx) (dominio.Estado, dominio.Objeto, error) {
		return dominio.IniciarEnvio(e, id, int(versao), agora, limites)
	}); err != nil {
		return Desfecho{}, err
	}
	x.o.Log("executor: %s enviando", id)
	return x.concluir(ctx, id, x.preencherEEnviar(ctx, c))
}

// passo é o que o envio decidiu: falhar, bloquear, ficar incerta ou enviada.
type passo struct {
	estado string
	motivo string // bloqueio ou código da falha
	err    error  // só para o tipo no log
}

func falhou(codigo string, err error) passo { return passo{dominio.CandFalhou, codigo, err} }
func bloqueada(motivo string) passo {
	if _, ok := dominio.PendenciaDoMotivo[motivo]; !ok {
		motivo = "outro"
	}
	return passo{estado: dominio.CandBloqueada, motivo: motivo}
}

func (x *Executor) preencherEEnviar(ctx context.Context, c dominio.Objeto) passo {
	plataforma, aprovada := dominio.Texto(c["plataforma"]), dominio.Texto(c["url"])
	conteudo := dominio.ObjetoDe(c["conteudo"])
	cv := dominio.ObjetoDe(conteudo["curriculo"])
	nome := dominio.Texto(cv["arquivo"])

	// M2: o currículo no disco tem de ser o aprovado.
	b, err := curriculos.Ler(x.o.PastaCurriculos, nome)
	if err != nil || curriculos.SHA256(b) != dominio.Texto(cv["sha256"]) {
		return falhou("curriculo-mudou", err)
	}
	// Anexa uma cópia do que foi conferido: trocar o original depois daqui não muda o que vai.
	dir, err := os.MkdirTemp("", "central-envio-")
	if err != nil {
		return falhou("erro-tecnico", err)
	}
	defer os.RemoveAll(dir)
	copia := filepath.Join(dir, nome)
	if err := os.WriteFile(copia, b, 0o600); err != nil {
		return falhou("erro-tecnico", err)
	}

	pag, err := x.o.Navegador.NovaPagina(ctx)
	if err != nil {
		return falhou("erro-tecnico", err)
	}
	defer pag.Fechar()

	// M5: a página tem de ficar no site da plataforma e na mesma vaga.
	final, err := pag.Abrir(ctx, aprovada)
	if err != nil {
		return falhou("erro-tecnico", err)
	}
	if !enderecoConfere(plataforma, aprovada, final) {
		return bloqueada("site-externo")
	}

	// Formulário em etapas: em cada uma, confere o endereço e o bloqueio, planeja todos os campos (M1) antes de
	// digitar, preenche, confere de novo e só então avança. Nunca avança com campo sem plano.
	for etapa := 0; ; etapa++ {
		if etapa >= MaxEtapas {
			return bloqueada("outro")
		}
		if p, parou := x.conferirEndereco(ctx, pag, plataforma, aprovada); parou {
			return p
		}
		if p, parou := x.conferirBloqueio(ctx, pag); parou {
			return p
		}
		campos, err := pag.Campos(ctx)
		if err != nil {
			return falhou("erro-tecnico", err)
		}
		plano, motivo := planejar(conteudo, campos)
		if motivo != "" {
			return bloqueada(motivo)
		}
		for _, a := range plano {
			if err := x.pausa(ctx, x.o.EntreAcoes); err != nil {
				return falhou("erro-tecnico", err)
			}
			if a.anexar {
				err = pag.Anexar(ctx, a.campo, copia)
			} else {
				err = pag.Preencher(ctx, a.campo, a.valor)
			}
			if err != nil {
				return falhou("erro-tecnico", err)
			}
		}
		if p, parou := x.conferirBloqueio(ctx, pag); parou {
			return p
		}
		if err := x.pausa(ctx, x.o.EntreAcoes); err != nil {
			return falhou("erro-tecnico", err)
		}
		mais, err := pag.Proxima(ctx)
		if err != nil {
			return falhou("erro-tecnico", err)
		}
		if !mais {
			break
		}
	}

	// Logo antes do clique: ainda no site da plataforma e na mesma vaga.
	if p, parou := x.conferirEndereco(ctx, pag, plataforma, aprovada); parou {
		return p
	}
	// M3: do clique em diante, sem confirmação, fica incerta.
	if ctx.Err() != nil {
		return falhou("erro-tecnico", ctx.Err())
	}
	if err := pag.Enviar(ctx); err != nil {
		return passo{dominio.CandIncerta, "", err}
	}
	if err := x.pausa(context.WithoutCancel(ctx), x.o.EntreAcoes); err != nil {
		return passo{dominio.CandIncerta, "", err}
	}
	confirmou, err := pag.Confirmacao(context.WithoutCancel(ctx))
	if err != nil || !confirmou {
		return passo{dominio.CandIncerta, "", err}
	}
	return passo{estado: dominio.CandEnviada}
}

// MaxEtapas: formulário com mais etapas que isso para em bloqueada/outro (a pessoa confere).
const MaxEtapas = 10

func enderecoConfere(plataforma, aprovada, atual string) bool {
	return dominio.URLDaPlataforma(plataforma, atual) && dominio.MesmaVaga(plataforma, aprovada, atual)
}

func (x *Executor) conferirEndereco(ctx context.Context, pag Pagina, plataforma, aprovada string) (passo, bool) {
	atual, err := pag.URLAtual(ctx)
	if err != nil {
		return falhou("erro-tecnico", err), true
	}
	if !enderecoConfere(plataforma, aprovada, atual) {
		return bloqueada("site-externo"), true
	}
	return passo{}, false
}

type acao struct {
	campo  Campo
	valor  string
	anexar bool
}

// planejar decide o que fazer com cada campo da etapa, antes de digitar qualquer coisa (M1). Um campo que bloqueia
// devolve o motivo e nada é preenchido.
func planejar(conteudo dominio.Objeto, campos []Campo) ([]acao, string) {
	var plano []acao
	for _, campo := range campos {
		valor, motivo := valorDoCampo(conteudo, campo)
		switch {
		case motivo != "":
			return nil, motivo
		case campo.Tipo == CampoCurriculo:
			plano = append(plano, acao{campo: campo, anexar: true})
		case valor != "":
			plano = append(plano, acao{campo: campo, valor: valor})
		}
	}
	return plano, ""
}

func (x *Executor) conferirBloqueio(ctx context.Context, pag Pagina) (passo, bool) {
	if err := x.pausa(ctx, x.o.EntreAcoes); err != nil {
		return falhou("erro-tecnico", err), true
	}
	motivo, err := pag.Bloqueio(ctx)
	if err != nil {
		return falhou("erro-tecnico", err), true
	}
	if motivo != "" {
		return bloqueada(motivo), true
	}
	return passo{}, false
}

// concluir grava o desfecho do envio (a candidatura está em enviando).
func (x *Executor) concluir(ctx context.Context, id string, p passo) (Desfecho, error) {
	agora := x.o.Relogio()
	_, err := x.gravar(ctx, func(e dominio.Estado, d dominio.Ctx) (dominio.Estado, dominio.Objeto, error) {
		switch p.estado {
		case dominio.CandEnviada:
			return dominio.MarcarEnviada(e, id, agora, d)
		case dominio.CandBloqueada:
			return dominio.BloquearCandidatura(e, id, p.motivo, agora, d)
		case dominio.CandFalhou:
			return dominio.FalharCandidatura(e, id, p.motivo, agora)
		default:
			return dominio.TornarIncerta(e, id, agora)
		}
	})
	if err != nil {
		// Fica em enviando; ao abrir o app de novo, vira incerta (M3).
		x.o.Log("executor: %s não gravou o desfecho (%T)", id, err)
		return Desfecho{}, err
	}
	switch {
	case p.err != nil:
		x.o.Log("executor: %s %s %s (%s)", id, p.estado, p.motivo, tipoDoErro(p.err))
	case p.motivo != "":
		x.o.Log("executor: %s %s (%s)", id, p.estado, p.motivo)
	default:
		x.o.Log("executor: %s %s", id, p.estado)
	}
	return Desfecho{ID: id, Estado: p.estado, Motivo: p.motivo}, nil
}

// tipoDoErro: só o tipo (M7); o texto do erro pode ter caminho, URL ou texto da página.
func tipoDoErro(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "cancelado"
	case errors.Is(err, context.DeadlineExceeded):
		return "tempo esgotado"
	case errors.Is(err, curriculos.ErrFora), errors.Is(err, curriculos.ErrGrande):
		return "currículo"
	}
	return fmt.Sprintf("%T", err)
}

// valorDoCampo devolve o valor do pacote para o campo (M1) ou o motivo de bloqueio. Valor e motivo vazios: campo
// opcional sem resposta no pacote, que fica em branco.
func valorDoCampo(conteudo dominio.Objeto, c Campo) (valor, motivo string) {
	// Rótulo sensível vence o tipo que o adaptador deu ao campo (senha, CPF, agência/conta...).
	if m := dominio.MotivoDoRotulo(c.Rotulo); m != "" {
		c.Tipo = m
	}
	if c.Tipo == "termos" && !c.Obrigatorio {
		return "", "" // aceite opcional fica sem marcar
	}
	if c.Tipo == CampoCurriculo {
		return "", ""
	}
	contato := dominio.ObjetoDe(conteudo["contato"])
	switch c.Tipo {
	case CampoNome, CampoEmail, CampoTelefone, CampoCidade:
		valor, _ = contato[c.Tipo].(string)
	case CampoPretensao:
		if v, ok := dominio.ObjetoDe(conteudo["pretensao"])["valor"].(float64); ok {
			valor = strconv.FormatFloat(math.Round(v), 'f', 0, 64) // só dígitos, sem ponto decimal (campo numérico)
		}
	case CampoMensagem:
		valor, _ = conteudo["mensagem"].(string)
	case CampoPergunta:
		valor = respostaPara(conteudo, c.Rotulo)
	default:
		// Pedido que só a pessoa responde (cpf, rg, senha, dado-bancario, documento, dado-sensivel, termos...).
		if _, sensivel := dominio.PendenciaDoMotivo[c.Tipo]; sensivel && c.Tipo != "outro" && c.Tipo != "pergunta-nova" {
			return "", c.Tipo
		}
	}
	if strings.TrimSpace(valor) == "" {
		if c.Obrigatorio {
			return "", "pergunta-nova"
		}
		return "", ""
	}
	return valor, ""
}

// respostaPara acha a resposta do pacote cuja pergunta tem o mesmo texto (sem diferença de maiúsculas, espaços e
// marca de obrigatório no fim). Nunca inventa nem aproxima.
func respostaPara(conteudo dominio.Objeto, rotulo string) string {
	quer := normalizarPergunta(rotulo)
	if quer == "" {
		return ""
	}
	for _, r := range dominio.ListaDe(conteudo["respostas"]) {
		o := dominio.ObjetoDe(r)
		if normalizarPergunta(dominio.Texto(o["pergunta"])) == quer {
			v, _ := o["resposta"].(string)
			return v
		}
	}
	return ""
}

func normalizarPergunta(s string) string {
	s = strings.Join(strings.Fields(strings.ToLower(s)), " ")
	return strings.TrimSpace(strings.TrimRight(s, " *:"))
}
