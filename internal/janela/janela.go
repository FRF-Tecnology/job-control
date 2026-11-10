// Package janela guarda as regras da janela do app, sem depender do Wails: o tamanho padrão, o tamanho que a pessoa
// deixou (lembrado entre aberturas) e o ajuste à tela. Quem aplica na janela de verdade é o app (app/janela.go).
package janela

// Tamanho padrão e mínimo da janela, em pontos da tela (não em pixels físicos).
const (
	LarguraPadrao = 1280
	AlturaPadrao  = 800
	LarguraMinima = 380
	AlturaMinima  = 560
	// Acima disso, o tamanho guardado é tratado como estragado e o app volta ao padrão.
	ladoMaximo = 16384
)

// Chaves da tabela meta.
const (
	// ChaveTamanho guarda o Tamanho que a pessoa deixou.
	ChaveTamanho = "janela"
	// ChaveSegundoPlano guarda true quando a pessoa pediu para o app continuar aberto ao fechar a janela.
	ChaveSegundoPlano = "segundo-plano"
)

// Tamanho é o que o app lembra da janela. A posição não é guardada: a janela sempre abre no centro da tela.
type Tamanho struct {
	Largura    int  `json:"largura"`
	Altura     int  `json:"altura"`
	Maximizada bool `json:"maximizada"`
}

// Padrao é o tamanho de quem abre o app pela primeira vez.
func Padrao() Tamanho { return Tamanho{Largura: LarguraPadrao, Altura: AlturaPadrao} }

// Inicial devolve o tamanho de abertura: o guardado, se existe e faz sentido; senão o padrão. Largura ou altura fora
// dos limites derruba só o tamanho; "maximizada" continua valendo.
func Inicial(guardado *Tamanho) Tamanho {
	t := Padrao()
	if guardado == nil {
		return t
	}
	t.Maximizada = guardado.Maximizada
	if valido(guardado.Largura, LarguraMinima) && valido(guardado.Altura, AlturaMinima) {
		t.Largura, t.Altura = guardado.Largura, guardado.Altura
	}
	return t
}

func valido(lado, minimo int) bool { return lado >= minimo && lado <= ladoMaximo }

// Caber encolhe o tamanho para a área útil da tela (sem a barra de tarefas ou o Dock), sem passar do mínimo da
// janela. mudou diz se algo foi encolhido. Área desconhecida (zero ou negativa) não muda nada.
func Caber(largura, altura, areaLargura, areaAltura int) (l, a int, mudou bool) {
	l, a = largura, altura
	if areaLargura > 0 && l > areaLargura {
		l = max(areaLargura, LarguraMinima)
	}
	if areaAltura > 0 && a > areaAltura {
		a = max(areaAltura, AlturaMinima)
	}
	return l, a, l != largura || a != altura
}

// Lembrar devolve o que gravar depois de uma mudança na janela. Com a janela maximizada, o tamanho anterior (o da
// janela solta) é mantido e só a marca muda. Tamanho fora dos limites (leitura estranha do sistema) não troca o
// anterior.
func Lembrar(anterior Tamanho, largura, altura int, maximizada bool) Tamanho {
	if maximizada {
		anterior.Maximizada = true
		return anterior
	}
	anterior.Maximizada = false
	if valido(largura, LarguraMinima) && valido(altura, AlturaMinima) {
		anterior.Largura, anterior.Altura = largura, altura
	}
	return anterior
}
