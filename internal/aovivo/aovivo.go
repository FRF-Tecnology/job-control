// Package aovivo guarda o estado do painel "Ao vivo" (jc-a2y.6.9, docs/propostas/ao-vivo.md): a etapa do envio e o
// último quadro do navegador do envio, só em memória. Nada daqui vai para disco, temp, log, banco ou backup.
//
// Só visualização: não há como clicar, digitar nem controlar o navegador por aqui.
package aovivo

import (
	"regexp"
	"strings"
	"sync"
	"time"
)

// Etapas: lista fechada do contrato (AoVivo.etapa). Nada da página entra aqui.
const (
	Desligado   = "desligado"
	SemEnvio    = "sem-envio"
	Abrindo     = "abrindo"
	Preenchendo = "preenchendo"
	Revisando   = "revisando"
	Enviando    = "enviando"
	Confirmada  = "confirmada"
	Bloqueada   = "bloqueada"
	Falhou      = "falhou"
	Incerta     = "incerta"
	LoginAberto = "login-aberto"
	Pausado     = "pausado"
)

// Limites do quadro (contrato): até 1280×800, imagem até 400 000 caracteres com o prefixo data:, 1 por segundo.
const (
	MaxLargura    = 1280
	MaxAltura     = 800
	MaxImagem     = 400_000
	IntervaloMin  = time.Second
	SemLeituraMax = 10 * time.Second // sem leitura da tela por isso, desliga sozinho (painel fechado)
	FinalDura     = 30 * time.Second // a etapa final fica isso na tela, sem quadro
	prefixoImagem = "data:image/jpeg;base64,"
)

var (
	emAndamento = map[string]bool{Abrindo: true, Preenchendo: true, Revisando: true, Enviando: true}
	finais      = map[string]bool{Confirmada: true, Bloqueada: true, Falhou: true, Incerta: true}
	reBase64    = regexp.MustCompile(`^[A-Za-z0-9+/=]+$`)
	reCand      = regexp.MustCompile(`^cand-[A-Za-z0-9._-]{1,80}$`)
)

// Quadro: o último quadro, pronto para <img src>.
type Quadro struct {
	Imagem  string
	Em      time.Time
	Largura int
	Altura  int
}

// Estado: o que a tela lê.
type Estado struct {
	Ligado      bool
	Etapa       string
	Candidatura string
	Quadro      *Quadro
}

// AoVivo: começa desligado a cada abertura do app.
type AoVivo struct {
	agora func() time.Time

	mu        sync.Mutex
	ligado    bool
	leituraEm time.Time
	etapa     string
	cand      string
	finalEm   time.Time
	quadro    *Quadro
}

// Novo cria o estado; agora é o relógio (time.Now no app).
func Novo(agora func() time.Time) *AoVivo {
	if agora == nil {
		agora = time.Now
	}
	return &AoVivo{agora: agora}
}

// Ligar liga ou desliga. Ligar e desligar apagam o quadro (a etapa fica). Devolve se mudou.
func (a *AoVivo) Ligar(ligado bool) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.vencerSemTrava()
	if a.ligado == ligado {
		if ligado {
			a.leituraEm = a.agora()
		}
		return false
	}
	a.ligado = ligado
	a.leituraEm = a.agora()
	a.quadro = nil
	return true
}

// Ler devolve o estado e conta como leitura da tela (mantém ligado).
func (a *AoVivo) Ler() Estado {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.vencerSemTrava()
	if !a.ligado {
		return Estado{Etapa: Desligado}
	}
	a.leituraEm = a.agora()
	e := Estado{Ligado: true, Etapa: SemEnvio}
	if a.cand != "" {
		e.Etapa, e.Candidatura = a.etapa, a.cand
	}
	if a.quadro != nil {
		q := *a.quadro
		e.Quadro = &q
	}
	return e
}

// Querendo diz se o navegador deve mandar quadros agora: ligado, lido nos últimos 10 s e com envio em andamento.
func (a *AoVivo) Querendo() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.vencerSemTrava()
	return a.ligado && a.cand != "" && emAndamento[a.etapa]
}

// Etapa: o executor conta em que passo está. Etapa fora da lista ou id fora do padrão é ignorado. A etapa fica
// guardada mesmo desligado (só o id e um valor da lista), para o painel aberto no meio de um envio já mostrar o
// passo certo; o quadro, não.
func (a *AoVivo) Etapa(candidatura, etapa string) {
	if !reCand.MatchString(candidatura) || !(emAndamento[etapa] || finais[etapa]) {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.vencerSemTrava()
	if candidatura != a.cand {
		a.quadro = nil // quadro de outra candidatura não fica
	}
	a.cand, a.etapa = candidatura, etapa
	if finais[etapa] {
		a.quadro = nil
		a.finalEm = a.agora()
	}
}

// Quadro: o navegador entrega um quadro (JPEG em base64). Só fica com o ao vivo ligado e envio em andamento, dentro
// dos limites e no máximo 1 por segundo; os outros são descartados.
func (a *AoVivo) Quadro(base64 string, largura, altura int) {
	if base64 == "" || largura <= 0 || altura <= 0 || largura > MaxLargura || altura > MaxAltura ||
		len(prefixoImagem)+len(base64) > MaxImagem || !reBase64.MatchString(base64) {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.vencerSemTrava()
	if !a.ligado || a.cand == "" || !emAndamento[a.etapa] {
		return
	}
	agora := a.agora()
	if a.quadro != nil && agora.Sub(a.quadro.Em) < IntervaloMin {
		return
	}
	var b strings.Builder
	b.Grow(len(prefixoImagem) + len(base64))
	b.WriteString(prefixoImagem)
	b.WriteString(base64)
	a.quadro = &Quadro{Imagem: b.String(), Em: agora, Largura: largura, Altura: altura}
}

// Limpar apaga o quadro e a etapa sem desligar (pausar, login aberto, sair das plataformas, apagar os dados).
func (a *AoVivo) Limpar() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.limparSemTrava()
}

func (a *AoVivo) limparSemTrava() {
	a.quadro, a.cand, a.etapa, a.finalEm = nil, "", "", time.Time{}
}

// vencerSemTrava: sem leitura por 10 s, desliga; etapa final há mais de 30 s, volta a sem envio.
func (a *AoVivo) vencerSemTrava() {
	agora := a.agora()
	if a.ligado && agora.Sub(a.leituraEm) > SemLeituraMax {
		a.ligado = false
		a.quadro = nil
	}
	if finais[a.etapa] && agora.Sub(a.finalEm) > FinalDura {
		a.limparSemTrava()
	}
}
