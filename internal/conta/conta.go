package conta

import (
	"context"
	"crypto/ed25519"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/licenca"
	"github.com/FRF-Tecnology/job-control/internal/segredos"
)

// chaveRenovacao é o nome do token de renovação no cofre do sistema.
const chaveRenovacao = "conta/renovacao"

// Motivos de PodeAgir e de Situacao.Motivo.
const (
	MotivoTesteAcabou   = "teste-acabou"
	MotivoSemAssinatura = "sem-assinatura"
	MotivoSemInternet   = "sem-internet"
	MotivoSemConta      = "sem-conta"
)

const (
	intervaloLicenca = 10 * time.Minute
	intervaloSemRede = time.Minute // sem internet, tenta de novo mais cedo
	margemAcesso     = time.Minute // renova o acesso quando falta menos que isso
)

// ErrSemConta: ninguém entrou, ou a nuvem disse que a sessão não vale mais.
var ErrSemConta = errors.New("sem conta")

// Situacao é o que a tela mostra da conta.
type Situacao struct {
	Entrada    bool           `json:"entrada"`
	Email      string         `json:"email"`
	Estado     licenca.Estado `json:"estado"`
	TesteFim   *time.Time     `json:"teste_fim"`
	PeriodoFim *time.Time     `json:"periodo_fim"`
	Motivo     string         `json:"motivo"`
}

// Conta guarda a sessão (renovação no cofre, acesso só na memória) e a licença (em memória e em licenca.json)
// e decide se o app pode agir. Segura para uso concorrente; Situacao e PodeAgir não fazem I/O.
type Conta struct {
	nuvem       Nuvem
	cofre       segredos.Cofre
	pasta       string
	pub         ed25519.PublicKey
	agora       func() time.Time
	dispositivo func() (string, error)

	// renovMu serializa tudo o que usa ou troca o token de renovação (Renovar, Entrou, Sair, limpeza): ele é de
	// uso único, e duas renovações com o mesmo token revogariam a família inteira.
	renovMu   sync.Mutex
	renovacao string

	arqMu sync.Mutex // serializa as escritas de licenca.json

	dispMu sync.Mutex
	disp   string

	mu        sync.RWMutex
	entrada   bool
	acesso    string
	acessoAte time.Time // pelo relógio da nuvem
	lic       *licenca.Licenca
	desvio    time.Duration // emitida_em − relógio do PC quando a licença chegou
	geracao   uint64        // muda a cada entrada/saída: descarta licença pedida pela sessão anterior
	errCofre  string        // último erro ao gravar a renovação no cofre ("" se está tudo certo)

	laco atomic.Int32 // quantas goroutines de Iniciar estão vivas (para os testes)

	google estadoGoogle // login Google em andamento (google.go)
}

// Nova monta a conta a partir do cofre (renovação) e de licenca.json na pasta. agora nil usa time.Now.
func Nova(n Nuvem, cofre segredos.Cofre, pasta string, pub ed25519.PublicKey, agora func() time.Time) *Conta {
	if agora == nil {
		agora = time.Now
	}
	c := &Conta{nuvem: n, cofre: cofre, pasta: pasta, pub: pub, agora: agora, dispositivo: Dispositivo}
	if r, err := cofre.Ler(chaveRenovacao); err == nil && r != "" {
		c.renovacao, c.entrada = r, true
	}
	if c.entrada {
		if l, desvio, _, err := lerArquivoLicenca(pasta, pub); err == nil {
			c.lic, c.desvio = &l, desvio
		}
	}
	return c
}

// Iniciar renova a sessão e pede a licença sem bloquear; depois repete a cada 10 min até ctx acabar.
func (c *Conta) Iniciar(ctx context.Context) {
	c.laco.Add(1)
	go func() {
		defer c.laco.Add(-1)
		for {
			espera := intervaloLicenca
			if c.Situacao().Entrada {
				if err := c.AtualizarLicenca(ctx); errors.Is(err, ErrSemInternet) {
					espera = intervaloSemRede
				}
			}
			t := time.NewTimer(espera)
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}
		}
	}()
}

// ErroCofre é o erro da última gravação da renovação no cofre ("" quando está tudo certo). Se não for vazio, a
// sessão vale só até o app fechar.
func (c *Conta) ErroCofre() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.errCofre
}

// Situacao devolve o estado atual sem I/O.
func (c *Conta) Situacao() Situacao {
	c.mu.RLock()
	entrada, lic, desvio := c.entrada, c.lic, c.desvio
	c.mu.RUnlock()
	if !entrada {
		return Situacao{Motivo: MotivoSemConta}
	}
	s := Situacao{Entrada: true}
	if lic == nil {
		s.Motivo = MotivoSemInternet
		return s
	}
	s.Email, s.Estado = lic.Email, lic.Estado
	s.TesteFim, s.PeriodoFim = copiarHora(lic.TesteFim), copiarHora(lic.PeriodoFim)
	switch {
	case !vale(*lic, c.agora(), desvio):
		s.Motivo = MotivoSemInternet
	case lic.PodeAgir():
	case lic.PeriodoFim != nil:
		s.Motivo = MotivoSemAssinatura
	default:
		s.Motivo = MotivoTesteAcabou
	}
	return s
}

func copiarHora(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := *t
	return &v
}

// PodeAgir: (true, "") só com licença válida de teste ou assinante; senão (false, motivo).
func (c *Conta) PodeAgir() (bool, string) {
	s := c.Situacao()
	return s.Motivo == "", s.Motivo
}

// Entrou guarda a sessão nova (renovação no cofre, acesso na memória) e pede a licença. O erro da licença
// (ex.: ErrSemInternet) volta, mas a sessão fica guardada.
func (c *Conta) Entrou(ctx context.Context, p Par) error {
	c.renovMu.Lock()
	if err := c.cofre.Gravar(chaveRenovacao, p.Renovacao); err != nil {
		c.renovMu.Unlock()
		return err
	}
	c.renovacao = p.Renovacao
	c.mu.Lock()
	c.entrada, c.acesso, c.acessoAte = true, p.Acesso, p.AcessoAte
	c.errCofre = "" // a gravação acima deu certo
	c.lic = nil     // o desvio fica: é do relógio do PC, não da conta
	c.geracao++
	c.mu.Unlock()
	c.arqMu.Lock()
	errArq := apagarLicenca(c.pasta) // a licença de quem estava antes não vale para esta sessão
	c.arqMu.Unlock()
	c.renovMu.Unlock()
	return errors.Join(errArq, c.AtualizarLicenca(ctx))
}

// Sair encerra a sessão na nuvem (melhor esforço: sem internet, o token vence sozinho) e apaga cofre e
// licenca.json.
func (c *Conta) Sair(ctx context.Context) error {
	c.renovMu.Lock()
	defer c.renovMu.Unlock()
	if c.renovacao != "" {
		_ = c.nuvem.Sair(ctx, c.renovacao)
	}
	return c.limpar()
}

// limpar apaga a sessão e a licença. Chamar com renovMu.
func (c *Conta) limpar() error {
	c.renovacao = ""
	errCofre := c.cofre.Apagar(chaveRenovacao)
	c.mu.Lock()
	c.entrada, c.acesso, c.acessoAte = false, "", time.Time{}
	c.errCofre = ""
	c.lic = nil
	c.geracao++
	c.mu.Unlock()
	c.arqMu.Lock()
	errArq := apagarLicenca(c.pasta)
	c.arqMu.Unlock()
	return errors.Join(errCofre, errArq)
}

// acessoValido diz se o acesso guardado ainda tem mais que a margem de vida (pelo relógio da nuvem estimado).
func (c *Conta) acessoValido() (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ok := c.acesso != "" && c.agora().Add(c.desvio).Add(margemAcesso).Before(c.acessoAte)
	return c.acesso, ok
}

// Acesso devolve um token de acesso, renovando se faltar menos de 1 min para vencer.
func (c *Conta) Acesso(ctx context.Context) (string, error) {
	if a, ok := c.acessoValido(); ok {
		return a, nil
	}
	a, _ := c.acessoValido()
	return c.renovar(ctx, a)
}

// renovar troca a renovação por um par novo. velho é o acesso que quem chama viu: se outro já renovou enquanto
// esperava a vez, devolve o acesso novo sem chamar a nuvem de novo.
func (c *Conta) renovar(ctx context.Context, velho string) (string, error) {
	c.renovMu.Lock()
	defer c.renovMu.Unlock()
	if a, ok := c.acessoValido(); ok && a != velho {
		return a, nil
	}
	if c.renovacao == "" {
		return "", ErrSemConta
	}
	// Sem o cancelamento de quem chamou: se a nuvem já gastou o token (uso único) e a resposta se perdesse, a
	// próxima tentativa reusaria o token velho e a nuvem revogaria a família inteira.
	rctx, cancelar := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
	defer cancelar()
	p, err := c.nuvem.Renovar(rctx, c.renovacao)
	if err != nil {
		var en *ErroNuvem
		if errors.As(err, &en) && en.Status == 401 {
			return "", errors.Join(ErrSemConta, c.limpar())
		}
		return "", err
	}
	// A renovação nova fica na memória mesmo se o cofre falhar: esta sessão continua; ao reabrir, o cofre
	// teria a antiga (já usada) e a pessoa entra de novo. O erro fica visível em ErroCofre.
	c.renovacao = p.Renovacao
	errCofre := c.cofre.Gravar(chaveRenovacao, p.Renovacao)
	c.mu.Lock()
	c.acesso, c.acessoAte = p.Acesso, p.AcessoAte
	c.errCofre = ""
	if errCofre != nil {
		c.errCofre = errCofre.Error()
	}
	c.mu.Unlock()
	return p.Acesso, nil
}

func (c *Conta) idDispositivo() (string, error) {
	c.dispMu.Lock()
	defer c.dispMu.Unlock()
	if c.disp == "" {
		d, err := c.dispositivo()
		if err != nil {
			return "", err
		}
		c.disp = d
	}
	return c.disp, nil
}

// AtualizarLicenca pede a licença à nuvem agora (ao abrir, a cada 10 min e depois de assinar).
func (c *Conta) AtualizarLicenca(ctx context.Context) error {
	c.mu.RLock()
	g, entrada := c.geracao, c.entrada
	c.mu.RUnlock()
	if !entrada {
		return ErrSemConta
	}
	disp, err := c.idDispositivo()
	if err != nil {
		return err
	}
	a, err := c.Acesso(ctx)
	if err != nil {
		return err
	}
	doc, err := c.nuvem.Licenca(ctx, a, disp)
	var en *ErroNuvem
	if errors.As(err, &en) && en.Status == 401 { // acesso recusado: renova uma vez e tenta de novo
		if a, err = c.renovar(ctx, a); err != nil {
			return err
		}
		doc, err = c.nuvem.Licenca(ctx, a, disp)
	}
	if err != nil {
		return err
	}
	return c.guardarLicenca(doc, g)
}

// guardarLicenca confere a assinatura, calcula o desvio do relógio e guarda na memória e no disco, se a sessão
// ainda for a mesma de quando o pedido saiu.
func (c *Conta) guardarLicenca(doc licenca.Documento, g uint64) error {
	l, err := conferirAssinatura(c.pub, doc)
	if err != nil {
		return err
	}
	desvio := limitarDesvio(l.EmitidaEm.Sub(c.agora()))
	c.arqMu.Lock()
	defer c.arqMu.Unlock()
	c.mu.RLock()
	mesma := c.geracao == g
	c.mu.RUnlock()
	if !mesma {
		return ErrSemConta
	}
	errArq := gravarLicenca(c.pasta, doc, desvio)
	c.mu.Lock()
	if c.geracao == g {
		c.lic, c.desvio = &l, desvio
	}
	c.mu.Unlock()
	return errArq
}
