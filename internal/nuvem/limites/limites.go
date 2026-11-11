// Pacote limites controla tentativas por chave (ex.: e-mail ou IP) em memória.
package limites

import (
	"sync"
	"time"
)

// limiteLimpeza: acima deste número de chaves, Permitir varre as expiradas.
const limiteLimpeza = 1024

// Janela permite até max eventos por chave dentro de uma janela deslizante.
// Segura para uso concorrente; não usa goroutine de fundo.
type Janela struct {
	mu     sync.Mutex
	max    int
	janela time.Duration
	agora  func() time.Time
	porChv map[string][]time.Time
}

// Nova cria a janela. Se agora for nil, usa time.Now.
func Nova(max int, janela time.Duration, agora func() time.Time) *Janela {
	if agora == nil {
		agora = time.Now
	}
	return &Janela{max: max, janela: janela, agora: agora, porChv: map[string][]time.Time{}}
}

// Permitir registra uma tentativa da chave e diz se ela cabe no limite.
// Tentativas negadas não são registradas.
func (j *Janela) Permitir(chave string) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	agora := j.agora()
	corte := agora.Add(-j.janela)
	if len(j.porChv) > limiteLimpeza {
		j.limpar(corte)
	}
	vivas := podar(j.porChv[chave], corte)
	if len(vivas) >= j.max {
		j.porChv[chave] = vivas
		return false
	}
	j.porChv[chave] = append(vivas, agora)
	return true
}

func (j *Janela) limpar(corte time.Time) {
	for k, ts := range j.porChv {
		if v := podar(ts, corte); len(v) == 0 {
			delete(j.porChv, k)
		} else {
			j.porChv[k] = v
		}
	}
}

// podar descarta instantes anteriores ao corte (a fatia está em ordem).
func podar(ts []time.Time, corte time.Time) []time.Time {
	i := 0
	for i < len(ts) && !ts[i].After(corte) {
		i++
	}
	return ts[i:]
}

func (j *Janela) tamanho() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return len(j.porChv)
}
