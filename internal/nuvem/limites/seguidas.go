package limites

import (
	"sync"
	"time"
)

// Seguidas conta falhas seguidas por chave: na max-ésima, bloqueia a chave por bloqueio e zera o contador.
// Guarda no máximo capacidade chaves; acima disso descarta primeiro as paradas há mais de bloqueio e,
// se ainda faltar espaço, chaves quaisquer (perder um contador só alivia o bloqueio, nunca prende alguém).
type Seguidas struct {
	mu         sync.Mutex
	max        int
	bloqueio   time.Duration
	capacidade int
	agora      func() time.Time
	porChv     map[string]*seguida
}

type seguida struct {
	falhas int
	ultima time.Time
	ate    time.Time // bloqueada até (zero: livre)
}

// NovaSeguidas cria o contador. Se agora for nil, usa time.Now.
func NovaSeguidas(max int, bloqueio time.Duration, capacidade int, agora func() time.Time) *Seguidas {
	if agora == nil {
		agora = time.Now
	}
	return &Seguidas{max: max, bloqueio: bloqueio, capacidade: capacidade, agora: agora, porChv: map[string]*seguida{}}
}

// Bloqueada diz se a chave está bloqueada agora.
func (s *Seguidas) Bloqueada(chave string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.porChv[chave]
	return ok && s.agora().Before(e.ate)
}

// Falhar registra uma falha e diz se ela bloqueou a chave.
func (s *Seguidas) Falhar(chave string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	agora := s.agora()
	e, ok := s.porChv[chave]
	if !ok {
		s.abrirEspaco(agora)
		e = &seguida{}
		s.porChv[chave] = e
	}
	e.ultima = agora
	e.falhas++
	if e.falhas >= s.max {
		e.falhas = 0
		e.ate = agora.Add(s.bloqueio)
		return true
	}
	return false
}

// Tentar é Bloqueada e Falhar de uma vez só, para quem sabe que a tentativa vai falhar (email sem conta):
// se a chave está bloqueada devolve false; senão conta a falha (bloqueando na max-ésima) e devolve true.
// Tentativas simultâneas não passam de max.
func (s *Seguidas) Tentar(chave string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	agora := s.agora()
	e, ok := s.porChv[chave]
	if ok && agora.Before(e.ate) {
		return false
	}
	if !ok {
		s.abrirEspaco(agora)
		e = &seguida{}
		s.porChv[chave] = e
	}
	e.ultima = agora
	e.falhas++
	if e.falhas >= s.max {
		e.falhas = 0
		e.ate = agora.Add(s.bloqueio)
	}
	return true
}

// Zerar esquece a chave (acerto).
func (s *Seguidas) Zerar(chave string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.porChv, chave)
}

func (s *Seguidas) abrirEspaco(agora time.Time) {
	if len(s.porChv) < s.capacidade {
		return
	}
	for k, e := range s.porChv {
		if !agora.Before(e.ate) && agora.Sub(e.ultima) > s.bloqueio {
			delete(s.porChv, k)
		}
	}
	for k := range s.porChv {
		if len(s.porChv) < s.capacidade {
			break
		}
		delete(s.porChv, k)
	}
}

func (s *Seguidas) tamanho() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.porChv)
}
