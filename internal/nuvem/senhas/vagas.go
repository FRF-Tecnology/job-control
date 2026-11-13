package senhas

import (
	"errors"
	"runtime"
	"sync"
	"time"
)

// Cada resumo Argon2id usa 19 MiB: sem teto, pedidos sem login em massa esgotam a memória do servidor.
// Um semáforo global limita os resumos simultâneos; quem não acha vaga espera até espera e desiste.

// ErrOcupado: todas as vagas de resumo estavam ocupadas durante a espera (a rota responde 503).
var ErrOcupado = errors.New("resumos de senha demais ao mesmo tempo")

const esperaPadrao = 2 * time.Second

var (
	vagasMu sync.Mutex
	vagas   = make(chan struct{}, capacidadePadrao())
	espera  = esperaPadrao
)

// capacidadePadrao: 2 por CPU, no mínimo 2.
func capacidadePadrao() int {
	return max(2, 2*runtime.NumCPU())
}

// ocupar pega uma vaga (esperando até espera) e devolve a função que a solta.
func ocupar() (func(), error) {
	vagasMu.Lock()
	c, e := vagas, espera
	vagasMu.Unlock()
	select {
	case c <- struct{}{}:
		return func() { <-c }, nil
	default:
	}
	t := time.NewTimer(e)
	defer t.Stop()
	select {
	case c <- struct{}{}:
		return func() { <-c }, nil
	case <-t.C:
		return nil, ErrOcupado
	}
}

// TrocarLimite troca a capacidade e a espera do semáforo (só para testes) e devolve a função que restaura
// os valores anteriores. Quem já tem vaga devolve no semáforo antigo.
func TrocarLimite(capacidade int, novaEspera time.Duration) (restaurar func()) {
	vagasMu.Lock()
	defer vagasMu.Unlock()
	antigas, antigaEspera := vagas, espera
	vagas, espera = make(chan struct{}, capacidade), novaEspera
	return func() {
		vagasMu.Lock()
		defer vagasMu.Unlock()
		vagas, espera = antigas, antigaEspera
	}
}

// emUso diz quantas vagas estão ocupadas agora (testes).
func emUso() int {
	vagasMu.Lock()
	defer vagasMu.Unlock()
	return len(vagas)
}
