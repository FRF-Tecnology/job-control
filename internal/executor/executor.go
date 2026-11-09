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
	"sync"
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

// gravarSeLiberado: o mesmo, mas com o envio pausado (armazem.ErrEnvioPausado) ou a plataforma fora de ativa
// (armazem.ErrPlataformaParada) nada é gravado, na mesma transação. É o caminho de toda ação que começa um envio
// (jc-a2y.51, jc-a2y.6.11).
func (x *Executor) gravarSeLiberado(ctx context.Context, plataforma string, op func(e dominio.Estado, d dominio.Ctx) (dominio.Estado, dominio.Objeto, error)) (dominio.Objeto, error) {
	agora := x.o.Relogio()
	return x.gravarCom(ctx, func(ctx context.Context, fn func(dominio.Estado) (dominio.Estado, error)) error {
		return x.arm.GravarSeLiberado(ctx, plataforma, agora, fn)
	}, op)
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
	// Ao abrir o app, o envio começa sempre PAUSADO (recomendação do CyberSec no #59), e isso vem PRIMEIRO, sozinho:
	// se a gravação das incertas falhar (banco ocupado pela CLI, disco), o envio já está pausado. Só a tela retoma.
	if err := x.arm.GravarEnvioPausado(ctx, true); err != nil {
		return 0, 0, err
	}
	agora := x.o.Relogio()
	err = x.arm.Gravar(ctx, func(e dominio.Estado) (dominio.Estado, error) {
		e, incertas = dominio.MarcarIncertas(e, agora)
		e, vencidas = dominio.VencerAprovacoes(e, agora)
		return e, nil
	})
	if err == nil && (incertas > 0 || vencidas > 0) {
		x.o.Log("executor: ao abrir, %d incerta(s) e %d aprovação(ões) vencida(s)", incertas, vencidas)
	}
	return incertas, vencidas, err
}

// Rodar é o laço do executor no app (build envio_dev): Iniciar ao abrir e, de intervalo em intervalo,
// EnviarAprovadas. Cada ciclo segura o ciclo (sync.Locker), que "sair das plataformas" usa para suspender. Se o
// Iniciar falhar, não entra no laço: devolve o erro e nada é enviado até o app abrir de novo.
func (x *Executor) Rodar(ctx context.Context, intervalo time.Duration, ciclo sync.Locker) error {
	ciclo.Lock()
	_, _, err := x.Iniciar(ctx)
	ciclo.Unlock()
	if err != nil {
		x.o.Log("executor: ao abrir (%s); envio desligado até abrir o app de novo", tipoDoErro(err))
		return err
	}
	t := time.NewTicker(intervalo)
	defer t.Stop()
	for {
		ciclo.Lock()
		if ctx.Err() == nil {
			if _, err := x.EnviarAprovadas(ctx); err != nil && ctx.Err() == nil {
				x.o.Log("executor: fila (%s)", tipoDoErro(err))
			}
		}
		ciclo.Unlock()
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

// EnviarAprovadas envia as aprovadas, da aprovação mais antiga para a mais nova, com intervalo entre uma e outra.
// Limite atingido numa plataforma pula as outras dela; conflito (mudou no meio) pula só aquela.
func (x *Executor) EnviarAprovadas(ctx context.Context) ([]Desfecho, error) {
	// A pessoa está com a janela de login aberta ("Entrar nas plataformas"): o ciclo espera, sem pegar candidatura
	// nem abrir página, e nada muda de estado (jc-a2y.6.7).
	if o, ok := x.o.Navegador.(interface{ Ocupado() bool }); ok && o.Ocupado() {
		return []Desfecho{}, nil
	}
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
		agora := x.o.Relogio()
		if s := x.arm.Plataformas(ctx).DaPlataforma(plataforma, agora); s.Estado != dominio.PlataformaAtiva {
			semLimite[plataforma] = true
			x.o.Log("executor: %s pulada (plataforma %s %s)", o["id"], plataforma, s.Estado)
			continue
		}
		if quando, err := time.Parse(time.RFC3339, dominio.Texto(o["proximaTentativaEm"])); err == nil && agora.Before(quando) {
			continue // devolvida (jc-a2y.6.11): espera a hora marcada, gravada no banco
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
		case errors.Is(err, armazem.ErrPlataformaParada):
			semLimite[plataforma] = true
			x.o.Log("executor: %s pulada (plataforma %s parada)", o["id"], plataforma)
			continue
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
		if _, err := x.gravarSeLiberado(ctx, plataforma, func(e dominio.Estado, d dominio.Ctx) (dominio.Estado, dominio.Objeto, error) {
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
	o, err := x.gravarSeLiberado(ctx, plataforma, func(e dominio.Estado, _ dominio.Ctx) (dominio.Estado, dominio.Objeto, error) {
		return dominio.IniciarEnvio(e, id, int(versao), agora, limites)
	})
	if err != nil {
		return Desfecho{}, err
	}
	x.o.Log("executor: %s enviando", id)
	t := &tentativa{x: x, id: id}
	if v, ok := dominio.NumeroDe(o["versao"]); ok {
		t.versao = int(v)
	}
	return x.concluir(ctx, plataforma, t, x.preencherEEnviar(ctx, c, t))
}

// tentativa guarda o que o executor já fez na página (jc-a2y.6.11). A marca é gravada no banco ANTES do passo; se a
// gravação falhar, o passo não acontece (falhou), e a candidatura também não volta sozinha.
type tentativa struct {
	x         *Executor
	id        string
	versao    int  // a lida ao entrar em enviando
	interagiu bool // pediu para gravar a marca (gravou ou não): daqui em diante, nunca volta sozinha
}

func (t *tentativa) marcar(ctx context.Context, marca string) error {
	if t.interagiu && marca == dominio.MarcaInteragiu {
		return nil
	}
	t.interagiu = true
	_, err := t.x.gravar(ctx, func(e dominio.Estado, _ dominio.Ctx) (dominio.Estado, dominio.Objeto, error) {
		return dominio.MarcarTentativa(e, t.id, marca, t.x.o.Relogio())
	})
	return err
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

func (x *Executor) preencherEEnviar(ctx context.Context, c dominio.Objeto, t *tentativa) passo {
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
			if !a.conferir { // conferir só lê a página
				if err := t.marcar(ctx, dominio.MarcaInteragiu); err != nil {
					return falhou("erro-tecnico", err)
				}
			}
			if a.desmarcar {
				if err := pag.Desmarcar(ctx, a.campo); err != nil {
					return falhou("erro-tecnico", err) // sem a certeza de que desmarcou, não envia
				}
				x.o.Log("executor: caixa %s desmarcada", a.campo.Tipo) // só o tipo
				continue
			}
			if a.conferir {
				// Contato que a página já trouxe: só passa se for igual ao do pacote, e o app não mexe (jc-a2y.6.5.1).
				igual, err := pag.Igual(ctx, a.campo, esperados(a.campo.Tipo, a.valor))
				if err != nil {
					return falhou("erro-tecnico", err)
				}
				if !igual {
					return bloqueada("pergunta-nova")
				}
				x.o.Log("executor: campo %s preenchido pela página, igual ao pacote", a.campo.Tipo) // só o tipo
				continue
			}
			if a.anexar {
				err = pag.Anexar(ctx, a.campo, copia)
			} else {
				err = pag.Preencher(ctx, a.campo, a.valor)
			}
			if err != nil {
				// O valor aprovado não é uma das opções do campo: a pessoa resolve; outro valor nunca vai no lugar.
				if errors.Is(err, ErrSemOpcao) {
					return bloqueada("pergunta-nova")
				}
				return falhou("erro-tecnico", err)
			}
		}
		if p, parou := x.conferirBloqueio(ctx, pag); parou {
			return p
		}
		if err := x.pausa(ctx, x.o.EntreAcoes); err != nil {
			return falhou("erro-tecnico", err)
		}
		if err := t.marcar(ctx, dominio.MarcaInteragiu); err != nil { // avançar é interagir
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
	if err := t.marcar(ctx, dominio.MarcaClicouEnviar); err != nil {
		return falhou("erro-tecnico", err)
	}
	if err := pag.Enviar(ctx); err != nil {
		// Sem o botão final, nada foi clicado: bloqueada (a pessoa confere), não incerta (jc-a2y.6.5.1).
		if errors.Is(err, ErrSemBotaoEnviar) {
			return bloqueada("outro")
		}
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
	campo     Campo
	valor     string
	anexar    bool
	conferir  bool // contato que a página já trouxe: só confere se é igual ao pacote, sem alterar
	desmarcar bool // "Seguir <empresa>" que a página trouxe marcada: o app desmarca (jc-a2y.6.5.2)
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
		case campo.Tipo == CampoSeguir:
			if campo.Preenchido {
				plano = append(plano, acao{campo: campo, desmarcar: true})
			}
		case campo.Preenchido && valor != "" && conferivel(campo.Tipo):
			plano = append(plano, acao{campo: campo, valor: valor, conferir: true})
		case campo.Tipo == CampoCurriculo:
			plano = append(plano, acao{campo: campo, anexar: true})
		case valor != "":
			plano = append(plano, acao{campo: campo, valor: valor})
		}
	}
	return plano, ""
}

// conferivel: os únicos campos que a página pode trazer preenchidos e ir assim, se iguais ao pacote (revisão do
// CyberSec, jc-a2y.6.5.1): nome, e-mail, telefone e código do país.
func conferivel(tipo string) bool {
	return tipo == CampoNome || tipo == CampoEmail || tipo == CampoTelefone || tipo == CampoDDI
}

// esperados: os valores que contam como iguais ao do pacote. Telefone: os mesmos dígitos, ou o código do país
// seguido do número nacional, nada mais (nota do CyberSec no #89). Os outros tipos: o próprio valor.
func esperados(tipo, valor string) []string {
	if tipo != CampoTelefone {
		return []string{valor}
	}
	digitos := func(s string) string {
		var b strings.Builder
		for _, r := range s {
			if r >= '0' && r <= '9' {
				b.WriteRune(r)
			}
		}
		return b.String()
	}
	todos := digitos(valor)
	ddi := digitos(ddiDoTelefone(valor))
	if todos == "" {
		return nil
	}
	if strings.HasPrefix(strings.TrimSpace(valor), "+") {
		if ddi == "" || !strings.HasPrefix(todos, ddi) {
			return []string{todos}
		}
		return []string{todos, strings.TrimPrefix(todos, ddi)} // com e sem o código do país
	}
	return []string{todos, ddi + todos}
}

// ddiDoTelefone: o código do país do telefone do pacote. "+55 11 9..." dá "+55". Sem "+", o número é nacional e o
// perfil é do Brasil: "+55". Com "+" e sem separador, não dá para saber: vazio (a pessoa resolve).
func ddiDoTelefone(tel string) string {
	tel = strings.TrimSpace(tel)
	if tel == "" {
		return ""
	}
	if !strings.HasPrefix(tel, "+") {
		return "+55"
	}
	fim := strings.IndexAny(tel[1:], " -.(")
	if fim <= 0 || fim > 3 {
		return ""
	}
	for _, r := range tel[1 : 1+fim] {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return tel[:1+fim]
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

// concluir grava o desfecho do envio (a candidatura está em enviando) e o efeito dele na plataforma, na mesma
// transação (jc-a2y.6.11). Falha técnica ou sessão caída ANTES de interagir (sem marca) devolve a candidatura para
// aprovada (dominio.DevolverAprovada, que confere a marca gravada, a versão e a impressão); se o domínio recusar,
// vale o desfecho de sempre. Incerta nunca volta.
func (x *Executor) concluir(ctx context.Context, plataforma string, t *tentativa, p passo) (Desfecho, error) {
	id, agora := t.id, x.o.Relogio()
	volta := ""
	if !t.interagiu {
		switch {
		case p.estado == dominio.CandFalhou && p.motivo == "erro-tecnico":
			volta = "erro-tecnico"
		case p.estado == dominio.CandBloqueada && p.motivo == "login":
			volta = "login"
		}
	}
	var final dominio.Objeto
	var antes, depois dominio.EstadoPlataforma
	devolveu := false
	err := x.arm.GravarComPlataformas(context.WithoutCancel(ctx), func(e dominio.Estado, pl dominio.Plataformas) (dominio.Estado, dominio.Plataformas, error) {
		antes, devolveu = pl.DaPlataforma(plataforma, agora), false
		if volta != "" {
			novo, o, err := dominio.DevolverAprovada(e, id, t.versao, volta, agora)
			var ec *dominio.ErroConflito
			switch {
			case err == nil:
				final, devolveu = o, true
				pl = pl.RegistrarResultado(plataforma, volta, agora)
				depois = pl.DaPlataforma(plataforma, agora)
				return novo, pl, nil
			case !errors.As(err, &ec):
				return e, pl, err
			}
			// Recusou (marca gravada, versão ou impressão): segue o desfecho de sempre.
		}
		var novo dominio.Estado
		var o dominio.Objeto
		var err error
		switch p.estado {
		case dominio.CandEnviada:
			novo, o, err = dominio.MarcarEnviada(e, id, agora, x.ctx(e))
		case dominio.CandBloqueada:
			novo, o, err = dominio.BloquearCandidatura(e, id, p.motivo, agora, x.ctx(e))
		case dominio.CandFalhou:
			novo, o, err = dominio.FalharCandidatura(e, id, p.motivo, agora)
		default:
			novo, o, err = dominio.TornarIncerta(e, id, agora)
		}
		if err != nil {
			return e, pl, err
		}
		final = o
		pl = pl.RegistrarResultado(plataforma, codigoDoPasso(p), agora)
		depois = pl.DaPlataforma(plataforma, agora)
		return novo, pl, nil
	})
	if err != nil {
		// Fica em enviando; ao abrir o app de novo, vira incerta (M3).
		x.o.Log("executor: %s não gravou o desfecho (%T)", id, err)
		return Desfecho{}, err
	}
	if depois.Estado != antes.Estado && depois.Estado != dominio.PlataformaAtiva {
		x.o.Log("envio: plataforma %s %s (%s)", plataforma, depois.Estado, depois.Motivo) // só códigos
	}
	estado, motivo := dominio.Texto(final["estado"]), p.motivo
	if devolveu {
		motivo = volta
		if estado == dominio.CandFalhou { // a 4ª volta: falhou/erro-tecnico, como antes
			motivo = "erro-tecnico"
		} else {
			x.o.Log("executor: %s devolvida (%s, %d/%d) %s", id, volta, len(dominio.ListaDe(final["retentativas"])), dominio.MaxRetentativas, estado)
			if p.err != nil {
				x.o.Log("executor: %s falha antes de interagir (%s)", id, tipoDoErro(p.err))
			}
			return Desfecho{ID: id, Estado: estado, Motivo: motivo}, nil
		}
	}
	switch {
	case p.err != nil:
		x.o.Log("executor: %s %s %s (%s)", id, estado, motivo, tipoDoErro(p.err))
	case motivo != "":
		x.o.Log("executor: %s %s (%s)", id, estado, motivo)
	default:
		x.o.Log("executor: %s %s", id, estado)
	}
	return Desfecho{ID: id, Estado: estado, Motivo: motivo}, nil
}

// codigoDoPasso: o código fechado do resultado, para o estado da plataforma.
func codigoDoPasso(p passo) string {
	switch p.estado {
	case dominio.CandEnviada:
		return "enviada"
	case dominio.CandIncerta:
		return "incerta"
	}
	return p.motivo
}

// LiberarDepoisDoLogin: a pessoa fechou a janela "Entrar nas plataformas". As plataformas em precisa-entrar voltam
// a ativa, só com o envio geral liberado (conferido na mesma transação); com ele pausado, ficam como estão.
func (x *Executor) LiberarDepoisDoLogin(ctx context.Context) {
	agora := x.o.Relogio()
	var liberadas []string
	_, err := x.arm.MudarPlataformas(ctx, func(p dominio.Plataformas, pausado bool) (dominio.Plataformas, error) {
		if pausado {
			return p, nil
		}
		n, l := p.LiberarDepoisDoLogin(agora)
		liberadas = l
		return n, nil
	})
	if err != nil {
		x.o.Log("envio: plataformas depois do login (%s)", tipoDoErro(err))
		return
	}
	for _, p := range liberadas {
		x.o.Log("envio: plataforma %s ativa (login)", p)
	}
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
	if c.Tipo == "termos" && c.Preenchido {
		return "", "termos" // aceite já marcado pela página: só a pessoa aceita (jc-a2y.6.5.1)
	}
	if c.Tipo == "termos" && !c.Obrigatorio {
		return "", "" // aceite opcional fica sem marcar
	}
	if c.Tipo == CampoCurriculo {
		return "", ""
	}
	// "Seguir <empresa>": opcional, fica desmarcada (o planejar desmarca a que veio marcada); obrigatória, a pessoa
	// decide (aprovado pelo CyberSec no #89).
	if c.Tipo == CampoSeguir {
		if c.Obrigatorio {
			return "", "pergunta-nova"
		}
		return "", ""
	}
	contato := dominio.ObjetoDe(conteudo["contato"])
	switch c.Tipo {
	case CampoNome, CampoEmail, CampoTelefone, CampoCidade:
		valor, _ = contato[c.Tipo].(string)
	case CampoDDI:
		tel, _ := contato[CampoTelefone].(string)
		valor = ddiDoTelefone(tel)
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
		// A página trouxe algo que não está no pacote (resposta de candidatura anterior, texto, opção marcada,
		// currículo antigo): obrigatório ou não, nunca vai sem a pessoa aprovar (revisão do CyberSec, jc-a2y.6.5.1).
		if c.Obrigatorio || c.Preenchido {
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
