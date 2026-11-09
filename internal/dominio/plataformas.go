package dominio

import (
	"sort"
	"time"
)

// Estado do envio por plataforma (jc-a2y.6.11, docs/propostas/envio-resiliente.md). Puro: recebe o estado e devolve
// um novo, sem mexer no original. Fica gravado no banco (armazem, meta "envio-plataformas"), na mesma transação do
// desfecho da candidatura. O "pausar" geral continua acima de tudo.

const (
	PlataformaAtiva         = "ativa"
	PlataformaPausada       = "pausada"        // qualquer autor pausa; só a pessoa retoma
	PlataformaPrecisaEntrar = "precisa-entrar" // a sessão caiu; volta sozinha só depois do login pela janela "Entrar nas plataformas"
	PlataformaParadaHoje    = "parada-hoje"    // captcha ou aviso de conta: nada até 00:00 do dia seguinte
)

// MotivosDaPlataforma: lista fechada (texto nosso, nada da página). Pausa pela pessoa ou pelo assistente: sem motivo.
var MotivosDaPlataforma = map[string]bool{"falhas-seguidas": true, "login": true, "captcha": true, "aviso-conta": true}

// FalhasSeguidasParaPausar: resultados iguais seguidos (erro-tecnico ou site-externo) que pausam a plataforma.
const FalhasSeguidasParaPausar = 2

// EstadoPlataforma de uma plataforma. Seguidas/UltimoCodigo contam os resultados iguais seguidos.
type EstadoPlataforma struct {
	Estado       string `json:"estado"`
	Motivo       string `json:"motivo,omitempty"`
	Ate          string `json:"ate,omitempty"` // só em parada-hoje
	UltimoCodigo string `json:"ultimoCodigo,omitempty"`
	Seguidas     int    `json:"seguidas,omitempty"`
}

// Plataformas: linkedin, gupy e inhire. Sem entrada, a plataforma está ativa.
type Plataformas map[string]EstadoPlataforma

// PlataformasDoEnvio: as que têm envio automático, em ordem.
var PlataformasDoEnvio = []string{"gupy", "inhire", "linkedin"}

func (p Plataformas) clonar() Plataformas {
	n := Plataformas{}
	for k, v := range p {
		n[k] = v
	}
	return n
}

// DaPlataforma devolve o estado em vigor: parada-hoje vencida conta como ativa.
func (p Plataformas) DaPlataforma(plataforma string, agora time.Time) EstadoPlataforma {
	s, tem := p[plataforma]
	if !tem || s.Estado == "" {
		return EstadoPlataforma{Estado: PlataformaAtiva}
	}
	if s.Estado == PlataformaParadaHoje {
		if ate, err := time.Parse(time.RFC3339, s.Ate); err == nil && !agora.Before(ate) {
			return EstadoPlataforma{Estado: PlataformaAtiva}
		}
	}
	return s
}

// Ativa: o executor só pega candidatura de plataforma ativa.
func (p Plataformas) Ativa(plataforma string, agora time.Time) bool {
	return p.DaPlataforma(plataforma, agora).Estado == PlataformaAtiva
}

// RegistrarResultado conta o resultado de uma tentativa da plataforma (código fechado: enviada, erro-tecnico,
// site-externo, login, captcha, aviso-conta ou outro motivo de bloqueio) e devolve o estado novo:
//   - erro-tecnico ou site-externo FalhasSeguidasParaPausar vezes seguidas: pausada/falhas-seguidas;
//   - login: precisa-entrar/login;
//   - captcha ou aviso-conta: parada-hoje até 00:00 do dia seguinte (nada de tentar de novo no mesmo dia);
//   - qualquer outro resultado zera a contagem.
//
// Nunca tira a plataforma de pausada, precisa-entrar ou parada-hoje: só piora (quem retoma é a pessoa).
func (p Plataformas) RegistrarResultado(plataforma, codigo string, agora time.Time) Plataformas {
	n := p.clonar()
	s := n.DaPlataforma(plataforma, agora)
	switch codigo {
	case "erro-tecnico", "site-externo":
		if s.UltimoCodigo == codigo {
			s.Seguidas++
		} else {
			s.UltimoCodigo, s.Seguidas = codigo, 1
		}
		if s.Seguidas >= FalhasSeguidasParaPausar && s.Estado == PlataformaAtiva {
			s.Estado, s.Motivo = PlataformaPausada, "falhas-seguidas"
		}
	case "login":
		s.UltimoCodigo, s.Seguidas = "", 0
		if s.Estado == PlataformaAtiva {
			s.Estado, s.Motivo = PlataformaPrecisaEntrar, "login"
		}
	case "captcha", "aviso-conta":
		s.UltimoCodigo, s.Seguidas = "", 0
		s.Estado, s.Motivo, s.Ate = PlataformaParadaHoje, codigo, RenovaEm(agora).Format(time.RFC3339)
	default:
		s.UltimoCodigo, s.Seguidas = "", 0
	}
	n[plataforma] = s
	return n
}

// PausarPlataforma: qualquer autor (só reduz o risco). Parada-hoje continua parada-hoje.
func (p Plataformas) PausarPlataforma(plataforma string, agora time.Time) (Plataformas, error) {
	if !plataformaDoEnvio(plataforma) {
		return p, &ErroNaoEncontrado{Msg: "plataforma sem envio automático: " + plataforma}
	}
	n := p.clonar()
	s := n.DaPlataforma(plataforma, agora)
	if s.Estado == PlataformaParadaHoje {
		return n, nil
	}
	n[plataforma] = EstadoPlataforma{Estado: PlataformaPausada}
	return n, nil
}

// RetomarPlataforma: só a pessoa (a API confere o autor). De pausada ou precisa-entrar para ativa; parada-hoje e
// ativa dão conflito.
func (p Plataformas) RetomarPlataforma(plataforma string, agora time.Time) (Plataformas, error) {
	if !plataformaDoEnvio(plataforma) {
		return p, &ErroNaoEncontrado{Msg: "plataforma sem envio automático: " + plataforma}
	}
	s := p.DaPlataforma(plataforma, agora)
	switch s.Estado {
	case PlataformaParadaHoje:
		return p, &ErroConflito{Msg: "a plataforma está parada até amanhã (captcha ou aviso de conta)", Estado: s.Estado}
	case PlataformaAtiva:
		return p, &ErroConflito{Msg: "a plataforma já está ativa", Estado: s.Estado}
	}
	n := p.clonar()
	n[plataforma] = EstadoPlataforma{Estado: PlataformaAtiva}
	return n, nil
}

// LiberarDepoisDoLogin: a pessoa fechou a janela "Entrar nas plataformas". Quem estava em precisa-entrar volta a
// ativa, mas só com o envio geral liberado (quem chama confere, na mesma transação). Devolve as liberadas.
func (p Plataformas) LiberarDepoisDoLogin(agora time.Time) (Plataformas, []string) {
	n := p.clonar()
	var liberadas []string
	for _, plataforma := range PlataformasDoEnvio {
		if n.DaPlataforma(plataforma, agora).Estado == PlataformaPrecisaEntrar {
			n[plataforma] = EstadoPlataforma{Estado: PlataformaAtiva}
			liberadas = append(liberadas, plataforma)
		}
	}
	return n, liberadas
}

// Lista para o GET /envio: as três plataformas, em ordem, só com estado, motivo e até (sem contagem interna).
func (p Plataformas) Lista(agora time.Time) []Objeto {
	nomes := append([]string(nil), PlataformasDoEnvio...)
	sort.Strings(nomes)
	saida := make([]Objeto, 0, len(nomes))
	for _, plataforma := range nomes {
		s := p.DaPlataforma(plataforma, agora)
		o := Objeto{"plataforma": plataforma, "estado": s.Estado, "motivo": nil, "ate": nil}
		if s.Motivo != "" {
			o["motivo"] = s.Motivo
		}
		if s.Estado == PlataformaParadaHoje && s.Ate != "" {
			o["ate"] = s.Ate
		}
		saida = append(saida, o)
	}
	return saida
}

func plataformaDoEnvio(plataforma string) bool {
	for _, p := range PlataformasDoEnvio {
		if p == plataforma {
			return true
		}
	}
	return false
}
