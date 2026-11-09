package aovivo

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

// jc-a2y.6.9: o "Ao vivo" guarda só o último quadro, em memória. Dados fictícios.

type relogio struct{ t time.Time }

func (r *relogio) agora() time.Time      { return r.t }
func (r *relogio) andar(d time.Duration) { r.t = r.t.Add(d) }

func novo() (*AoVivo, *relogio) {
	r := &relogio{time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)}
	return Novo(r.agora), r
}

// jpegFalso: base64 fictício. Este pacote não decodifica a imagem; quem confere o JPEG é o navegador.
var jpegFalso = base64.StdEncoding.EncodeToString([]byte("quadro-ficticio"))

func TestComecaDesligadoESemQuadro(t *testing.T) {
	a, _ := novo()
	e := a.Ler()
	if e.Ligado || e.Etapa != Desligado || e.Candidatura != "" || e.Quadro != nil {
		t.Fatalf("estado inicial = %+v", e)
	}
	if a.Querendo() {
		t.Fatal("quer quadros desligado")
	}
	a.Etapa("cand-1", Abrindo)
	a.Quadro(jpegFalso, 800, 600)
	if e := a.Ler(); e.Quadro != nil || e.Etapa != Desligado {
		t.Fatalf("desligado guardou: %+v", e)
	}
}

func TestLigadoGuardaSoOUltimoQuadroDoEnvio(t *testing.T) {
	a, r := novo()
	a.Ligar(true)
	if e := a.Ler(); !e.Ligado || e.Etapa != SemEnvio || e.Quadro != nil {
		t.Fatalf("ligado sem envio = %+v", e)
	}
	if a.Querendo() {
		t.Fatal("quer quadros sem envio")
	}
	// Quadro sem candidatura em envio: descartado.
	a.Quadro(jpegFalso, 800, 600)
	if a.Ler().Quadro != nil {
		t.Fatal("guardou quadro sem envio")
	}
	a.Etapa("cand-1", Abrindo)
	if !a.Querendo() {
		t.Fatal("não quer quadros com envio")
	}
	a.Quadro(jpegFalso, 800, 600)
	e := a.Ler()
	if e.Etapa != Abrindo || e.Candidatura != "cand-1" || e.Quadro == nil || e.Quadro.Largura != 800 || e.Quadro.Altura != 600 {
		t.Fatalf("com envio = %+v", e)
	}
	if !strings.HasPrefix(e.Quadro.Imagem, "data:image/jpeg;base64,") {
		t.Fatalf("imagem = %q", e.Quadro.Imagem[:30])
	}
	// No máximo 1 quadro por segundo: o seguinte, logo depois, é descartado.
	outro := base64.StdEncoding.EncodeToString([]byte("outro-quadro"))
	a.Quadro(outro, 800, 600)
	if a.Ler().Quadro.Imagem != e.Quadro.Imagem {
		t.Fatal("aceitou 2 quadros no mesmo segundo")
	}
	r.andar(time.Second)
	a.Quadro(outro, 800, 600)
	if a.Ler().Quadro.Imagem == e.Quadro.Imagem {
		t.Fatal("não trocou pelo último quadro")
	}
}

func TestQuadroForaDosLimitesEDescartado(t *testing.T) {
	a, _ := novo()
	a.Ligar(true)
	a.Etapa("cand-1", Preenchendo)
	grande := strings.Repeat("A", MaxImagem) // com o prefixo data:, passa do limite do contrato
	for nome, q := range map[string]struct {
		img  string
		l, h int
	}{
		"largo":       {jpegFalso, 1281, 600},
		"alto":        {jpegFalso, 800, 801},
		"vazio":       {"", 800, 600},
		"grande":      {grande, 800, 600},
		"não base64":  {"<script>", 800, 600},
		"sem tamanho": {jpegFalso, 0, 0},
	} {
		a.Quadro(q.img, q.l, q.h)
		if a.Ler().Quadro != nil {
			t.Errorf("%s: guardou", nome)
		}
	}
}

func TestDesligarApagaOQuadro(t *testing.T) {
	a, r := novo()
	a.Ligar(true)
	a.Etapa("cand-1", Enviando)
	a.Quadro(jpegFalso, 800, 600)
	if !a.Ligar(false) {
		t.Fatal("desligar não mudou")
	}
	if a.Ligar(false) {
		t.Fatal("desligar duas vezes mudou")
	}
	e := a.Ler()
	if e.Ligado || e.Etapa != Desligado || e.Quadro != nil || e.Candidatura != "" {
		t.Fatalf("desligado = %+v", e)
	}
	// Ligar de novo não traz o quadro de volta.
	a.Ligar(true)
	r.andar(time.Second)
	if e := a.Ler(); e.Quadro != nil {
		t.Fatalf("religado trouxe o quadro: %+v", e)
	}
}

func TestSemLeituraPor10sDesliga(t *testing.T) {
	a, r := novo()
	a.Ligar(true)
	a.Etapa("cand-1", Preenchendo)
	a.Quadro(jpegFalso, 800, 600)
	r.andar(9 * time.Second)
	if !a.Querendo() {
		t.Fatal("desligou antes de 10 s")
	}
	a.Ler() // a leitura renova
	r.andar(9 * time.Second)
	if !a.Querendo() {
		t.Fatal("a leitura não renovou")
	}
	r.andar(2 * time.Second)
	if a.Querendo() {
		t.Fatal("não desligou depois de 10 s sem leitura")
	}
	// Quem lê depois vê desligado e sem quadro: o painel precisa ligar de novo.
	if e := a.Ler(); e.Ligado || e.Quadro != nil {
		t.Fatalf("depois de 10 s = %+v", e)
	}
}

func TestEtapaFinalFica30sSemQuadro(t *testing.T) {
	a, r := novo()
	a.Ligar(true)
	a.Etapa("cand-1", Enviando)
	a.Quadro(jpegFalso, 800, 600)
	a.Etapa("cand-1", Confirmada)
	if a.Querendo() {
		t.Fatal("quer quadros depois de terminar")
	}
	e := a.Ler()
	if e.Etapa != Confirmada || e.Candidatura != "cand-1" || e.Quadro != nil {
		t.Fatalf("logo depois de terminar = %+v", e)
	}
	for i := 0; i < 5; i++ { // a tela lê a cada segundo; aqui, a cada 5 s
		r.andar(5 * time.Second)
		a.Ler()
	}
	r.andar(4 * time.Second) // 29 s
	if e := a.Ler(); e.Etapa != Confirmada {
		t.Fatalf("antes de 30 s = %+v", e)
	}
	r.andar(2 * time.Second)
	if e := a.Ler(); e.Etapa != SemEnvio || e.Candidatura != "" {
		t.Fatalf("depois de 30 s = %+v", e)
	}
}

func TestLimparApagaSemDesligar(t *testing.T) {
	a, _ := novo()
	a.Ligar(true)
	a.Etapa("cand-1", Revisando)
	a.Quadro(jpegFalso, 800, 600)
	a.Limpar()
	e := a.Ler()
	if !e.Ligado || e.Etapa != SemEnvio || e.Quadro != nil || e.Candidatura != "" {
		t.Fatalf("depois de limpar = %+v", e)
	}
}

func TestEtapaForaDaListaEIgnorada(t *testing.T) {
	a, _ := novo()
	a.Ligar(true)
	a.Etapa("cand-1", "texto da página")
	a.Etapa("cand-1", LoginAberto) // só a API decide login-aberto e pausado
	a.Etapa("cand-1", Pausado)
	a.Etapa("li-1", Abrindo) // id fora do padrão
	if e := a.Ler(); e.Etapa != SemEnvio || e.Candidatura != "" {
		t.Fatalf("aceitou etapa ou id fora da lista: %+v", e)
	}
}

// O painel aberto no meio de um envio já mostra o passo certo, mas nenhum quadro de antes de ligar.
func TestLigarNoMeioDoEnvioMostraAEtapa(t *testing.T) {
	a, _ := novo()
	a.Etapa("cand-1", Preenchendo)
	a.Quadro(jpegFalso, 800, 600) // desligado: descartado
	a.Ligar(true)
	e := a.Ler()
	if e.Etapa != Preenchendo || e.Candidatura != "cand-1" || e.Quadro != nil {
		t.Fatalf("ligado no meio = %+v", e)
	}
	if !a.Querendo() {
		t.Fatal("não quer quadros no meio do envio")
	}
}

// jc-a2y.6.11: a candidatura devolvida para aprovada (falha antes de interagir) volta o painel a "sem envio". Só a
// mesma candidatura limpa; sem-envio de outra não mexe na que está em andamento.
func TestDevolvidaVoltaASemEnvio(t *testing.T) {
	a, _ := novo()
	a.Ligar(true)
	a.Etapa("cand-1", Abrindo)
	a.Quadro(jpegFalso, 800, 600)
	a.Etapa("cand-2", SemEnvio) // outra candidatura: nada muda
	if e := a.Ler(); e.Etapa != Abrindo || e.Quadro == nil {
		t.Fatalf("sem-envio de outra mexeu: %+v", e)
	}
	a.Etapa("cand-1", SemEnvio)
	if e := a.Ler(); e.Etapa != SemEnvio || e.Candidatura != "" || e.Quadro != nil || a.Querendo() {
		t.Fatalf("devolvida = %+v", e)
	}
}
