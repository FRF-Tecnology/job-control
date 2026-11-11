package api

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/FRF-Tecnology/job-control/internal/conta"
	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/licenca"
)

// Rotas /conta/* e /assinatura/*: a tela fala só com a API local, e ela chama internal/conta. Só a tela
// (autor "eu"); o assistente (CLI, MCP) recebe 403. Senha e códigos nunca vão para log.

const (
	msgContaSoTela       = "só a pessoa, pela tela do app, mexe na conta e na assinatura"
	msgContaIndisponivel = "a conta não está disponível neste build"
	msgSemInternet       = "Sem conexão com a internet"

	cadenciaAssinarPadrao = 5 * time.Second
	prazoAssinarPadrao    = 10 * time.Minute
)

// ehExcluirEApagar: POST /conta/excluir com apagarDados true. Passa pela trava de escrita (como /dados/apagar),
// fora do roteamento comum, depois de apagar a conta na nuvem.
func ehExcluirEApagar(ped Pedido) bool {
	caminho, _, _ := strings.Cut(ped.Caminho, "?")
	p := strings.TrimPrefix(caminho, Prefixo)
	if strings.ToUpper(ped.Metodo) != "POST" || p != "/conta/excluir" {
		return false
	}
	c, _ := ped.Corpo.(map[string]any)
	v, _ := c["apagarDados"].(bool)
	return v
}

func (a *API) soTelaConta(autor string) error {
	if autor != "eu" || a.canal != "" {
		return &dominio.ErroSoPessoa{Msg: msgContaSoTela}
	}
	if a.conta == nil {
		return falha(503, msgContaIndisponivel)
	}
	return nil
}

// erroDaConta traduz o erro de internal/conta para a resposta da API. O que não é conhecido sobe (500 genérico).
func erroDaConta(err error) error {
	var en *conta.ErroNuvem
	switch {
	case errors.As(err, &en):
		switch en.Status {
		case 400, 401, 403, 404, 409, 429, 503:
		default: // 5xx inesperado ou status que o contrato não prevê: a tela não ganha nada com o texto cru
			return &erroApi{502, Objeto{"erro": "A nuvem não respondeu direito. Tente de novo.", "motivo": "nuvem"}}
		}
		corpo := Objeto{"erro": en.Msg}
		motivo := en.Motivo
		if motivo == "" && en.Status == 403 {
			motivo = "nuvem-recusou" // o 403 da nuvem não pode se passar pelo 403 "só da tela" daqui
		}
		if motivo != "" {
			corpo["motivo"] = motivo
		}
		if en.Versao != "" {
			corpo["versao"] = en.Versao
		}
		return &erroApi{en.Status, corpo}
	case errors.Is(err, conta.ErrSemInternet):
		return &erroApi{503, Objeto{"erro": msgSemInternet, "motivo": conta.MotivoSemInternet}}
	case errors.Is(err, conta.ErrSemConta):
		return &erroApi{401, Objeto{"erro": "Entre na sua conta", "motivo": conta.MotivoSemConta}}
	}
	return err
}

// textos lê os campos de texto do corpo: os obrigatórios não podem faltar nem ser vazios; campo fora da lista dá 400.
func textos(c Objeto, obrigatorios []string, opcionais ...string) (map[string]string, error) {
	todos := append(append([]string{}, obrigatorios...), opcionais...)
	if err := soCampos(c, todos...); err != nil {
		return nil, err
	}
	r := map[string]string{}
	var erros []string
	for _, k := range todos {
		v, tem := c[k]
		s, ehTexto := v.(string)
		switch {
		case tem && !ehTexto:
			erros = append(erros, k+": texto")
		case !tem && contem(obrigatorios, k), tem && s == "" && contem(obrigatorios, k):
			erros = append(erros, k+": obrigatório")
		default:
			r[k] = s
		}
	}
	if len(erros) > 0 {
		return nil, &dominio.ErroValidacao{Erros: erros}
	}
	return r, nil
}

type respostaConta struct {
	conta.Situacao
	Google       string `json:"google"`
	GoogleMotivo string `json:"google_motivo"`
	GoogleVersao string `json:"google_versao"`
	ErroCofre    string `json:"erro_cofre"`
}

func (a *API) estadoDaConta() respostaConta {
	return respostaConta{Situacao: a.conta.Situacao(), Google: a.conta.EstadoGoogle(),
		GoogleMotivo: a.conta.EstadoGoogleMotivo(), GoogleVersao: a.conta.EstadoGoogleVersao(), ErroCofre: a.conta.ErroCofre()}
}

func semConteudo() (Resposta, error) { return Resposta{Status: 204}, nil }

func aceito(mensagem string) (Resposta, error) {
	return Resposta{Status: 202, Corpo: Objeto{"mensagem": mensagem}}, nil
}

func (a *API) rotasConta(ctx context.Context, m, p string, c Objeto, autor string) (Resposta, bool, error) {
	switch {
	case m == "GET" && p == "/conta":
	case m == "POST" && (p == "/conta/criar" || p == "/conta/confirmar" || p == "/conta/entrar" || p == "/conta/recuperar" ||
		p == "/conta/nova-senha" || p == "/conta/google" || p == "/conta/google/cancelar" || p == "/conta/sair" || p == "/conta/excluir"):
	case m == "GET" && p == "/assinatura", m == "POST" && (p == "/assinatura/assinar" || p == "/assinatura/gerenciar"):
	default:
		return Resposta{}, false, nil
	}
	if err := a.soTelaConta(autor); err != nil {
		return Resposta{}, true, err
	}
	r, err := a.executarConta(ctx, p, c)
	if err != nil {
		err = erroDaConta(err)
	}
	return r, true, err
}

func (a *API) executarConta(ctx context.Context, p string, c Objeto) (Resposta, error) {
	switch p {
	case "/conta":
		return ok(a.estadoDaConta())
	case "/conta/criar":
		t, err := textos(c, []string{"email", "senha", "termos"})
		if err != nil {
			return Resposta{}, err
		}
		if err := a.conta.Criar(ctx, t["email"], t["senha"], t["termos"]); err != nil {
			return Resposta{}, err
		}
		return aceito("Enviamos um código de confirmação para o seu email")
	case "/conta/confirmar":
		t, err := textos(c, []string{"email", "codigo", "senha"}, "termos")
		if err != nil {
			return Resposta{}, err
		}
		if err := a.conta.Confirmar(ctx, t["email"], t["codigo"], t["senha"], termosOuPadrao(t["termos"])); err != nil {
			return Resposta{}, err
		}
		return ok(a.conta.Situacao())
	case "/conta/entrar":
		t, err := textos(c, []string{"email", "senha"}, "termos")
		if err != nil {
			return Resposta{}, err
		}
		if err := a.conta.EntrarComSenha(ctx, t["email"], t["senha"], termosOuPadrao(t["termos"])); err != nil {
			return Resposta{}, err
		}
		return ok(a.conta.Situacao())
	case "/conta/recuperar":
		t, err := textos(c, []string{"email"})
		if err != nil {
			return Resposta{}, err
		}
		if err := a.conta.Recuperar(ctx, t["email"]); err != nil {
			return Resposta{}, err
		}
		return aceito("Se o email tem conta, enviamos um código para criar uma senha nova")
	case "/conta/nova-senha":
		t, err := textos(c, []string{"email", "codigo", "senha"})
		if err != nil {
			return Resposta{}, err
		}
		if err := a.conta.NovaSenha(ctx, t["email"], t["codigo"], t["senha"]); err != nil {
			return Resposta{}, err
		}
		return ok(a.conta.Situacao())
	case "/conta/google":
		t, err := textos(c, nil, "termos")
		if err != nil {
			return Resposta{}, err
		}
		return a.iniciarGoogle(ctx, termosOuPadrao(t["termos"]))
	case "/conta/google/cancelar":
		if err := soCampos(c); err != nil {
			return Resposta{}, err
		}
		a.conta.CancelarGoogle()
		return semConteudo()
	case "/conta/sair":
		if err := soCampos(c); err != nil {
			return Resposta{}, err
		}
		if err := a.conta.Sair(ctx); err != nil {
			return Resposta{}, err
		}
		return semConteudo()
	case "/conta/excluir":
		return a.excluirNaNuvem(ctx, c)
	case "/assinatura":
		notas, err := a.conta.Notas(ctx)
		if err != nil {
			return Resposta{}, err
		}
		if notas == nil {
			notas = []conta.Nota{}
		}
		return ok(Objeto{"situacao": a.conta.Situacao(), "notas": notas})
	case "/assinatura/assinar":
		if err := soCampos(c); err != nil {
			return Resposta{}, err
		}
		return a.abrirAssinatura(ctx, a.conta.Assinar, true)
	case "/assinatura/gerenciar":
		if err := soCampos(c); err != nil {
			return Resposta{}, err
		}
		return a.abrirAssinatura(ctx, a.conta.Portal, false)
	}
	return Resposta{}, falha(404, "rota não encontrada")
}

// termosOuPadrao: sem o campo, vale a versão atual dos termos (a nuvem recusa com 409 se ela mudou).
func termosOuPadrao(t string) string {
	if t == "" {
		return conta.TermosVersao
	}
	return t
}

var errNavegador = errors.New("sem navegador")

func erroNavegador() error {
	return &erroApi{503, Objeto{"erro": "Não deu para abrir o navegador neste computador", "motivo": "sem-navegador"}}
}

func (a *API) iniciarGoogle(ctx context.Context, termos string) (Resposta, error) {
	if conta.GoogleClientID == "" {
		return Resposta{}, &erroApi{503, Objeto{"erro": "O login com Google não está disponível neste build", "motivo": "google-indisponivel"}}
	}
	abrir := a.abrirNavegador
	if abrir == nil {
		abrir = func(string) error { return errNavegador }
	}
	switch err := a.conta.IniciarGoogle(ctx, abrir, termos); {
	case err == nil:
		return aceito("Continue no navegador para entrar com o Google")
	case errors.Is(err, conta.ErrGoogleEmAndamento):
		return Resposta{}, &erroApi{409, Objeto{"erro": "Já há um login com Google em andamento", "motivo": "em-andamento"}}
	case errors.Is(err, errNavegador):
		return Resposta{}, erroNavegador()
	default:
		return Resposta{}, err
	}
}

// emailParaExcluir confere o corpo de POST /conta/excluir (antes de qualquer efeito).
func emailParaExcluir(c Objeto) (string, error) {
	if err := soCampos(c, "email", "apagarDados"); err != nil {
		return "", err
	}
	if v, tem := c["apagarDados"]; tem {
		if _, eBool := v.(bool); !eBool {
			return "", erroCampo("apagarDados: true ou false")
		}
	}
	corpo := Objeto{}
	if v, tem := c["email"]; tem {
		corpo["email"] = v
	}
	t, err := textos(corpo, []string{"email"})
	if err != nil {
		return "", err
	}
	return t["email"], nil
}

// avisoExclusao: a conta JÁ foi excluída na nuvem (não se desfaz), mas o que veio depois neste computador falhou.
// A tela mostra o erro e oferece "Apagar meus dados" (POST /dados/apagar), que continua valendo.
func avisoExclusao(erro string) (Resposta, error) {
	return ok(Objeto{"contaExcluida": true, "dadosApagados": false, "erro": erro})
}

// excluirNaNuvem apaga a conta na nuvem e encerra a sessão aqui: 204. Se a nuvem apagou mas a sessão local não saiu,
// 200 com o aviso (avisoExclusao). Com apagarDados, Chamar passa por excluirEApagar.
func (a *API) excluirNaNuvem(ctx context.Context, c Objeto) (Resposta, error) {
	email, err := emailParaExcluir(c)
	if err != nil {
		return Resposta{}, err
	}
	switch err := a.conta.Excluir(ctx, email); {
	case err == nil:
		return semConteudo()
	case errors.As(err, new(*conta.ErroSessaoLocal)):
		return avisoExclusao("A conta foi excluída, mas não deu para encerrar a sessão neste computador. Saia da conta e tente apagar os dados.")
	default:
		return Resposta{}, err
	}
}

// excluirEApagar: confere tudo o que dá antes de tocar na nuvem (corpo e as mesmas condições de /dados/apagar: sem
// envio em andamento e pasta de dados de verdade; nesses casos nada é feito). Depois apaga a conta na nuvem (trava de
// leitura) e, só se deu certo, os dados locais pelo mesmo caminho de POST /dados/apagar (trava de escrita, que fecha
// o app). Se a nuvem apagou e o resto falhou, responde avisoExclusao.
func (a *API) excluirEApagar(ctx context.Context, ped Pedido) (Resposta, error) {
	autor := ped.Autor
	if autor == "" {
		autor = "eu"
	}
	if err := a.soTelaConta(autor); err != nil {
		return Resposta{}, err
	}
	c := Objeto(ped.Corpo.(map[string]any))
	email, err := emailParaExcluir(c)
	if err != nil {
		return Resposta{}, err
	}
	if err := validarApagar(Objeto{"confirmacao": FraseApagar}, "eu"); err != nil {
		return Resposta{}, err
	}
	if a.arm.Pasta() == "" {
		return Resposta{}, erroCampo("banco em memória: não há pasta de dados para apagar")
	}
	if err := a.semEnvioEmAndamento(ctx, "há uma candidatura sendo enviada agora: nada foi excluído nem apagado; tente de novo quando o envio terminar"); err != nil {
		return Resposta{}, err
	}
	a.trava.RLock()
	if a.apagando {
		a.trava.RUnlock()
		return respostaFechando(), nil
	}
	err = a.conta.Excluir(ctx, email)
	a.trava.RUnlock()
	var local *conta.ErroSessaoLocal
	switch {
	case errors.As(err, &local):
		return avisoExclusao("A conta foi excluída, mas não deu para encerrar a sessão neste computador. Seus dados continuam aqui; você pode apagá-los em \"Apagar meus dados\".")
	case err != nil:
		return Resposta{}, erroDaConta(err)
	}
	r, err := a.apagarComTrava(ctx, Pedido{Metodo: "POST", Caminho: Prefixo + "/dados/apagar", Autor: "eu",
		Corpo: Objeto{"confirmacao": FraseApagar}})
	if err != nil {
		msg := "A conta foi excluída, mas não deu para apagar os dados neste computador. Use \"Apagar meus dados\" para tentar de novo."
		if resp, conhecido := respostaDeErro(err); conhecido && resp.Status == 409 {
			msg = "A conta foi excluída, mas há uma candidatura sendo enviada e os dados não foram apagados. Use \"Apagar meus dados\" quando o envio terminar."
		}
		return avisoExclusao(msg)
	}
	return r, nil
}

// abrirAssinatura pede o endereço à nuvem e abre no navegador. Com acompanhar, passa a renovar a licença até a
// assinatura aparecer.
func (a *API) abrirAssinatura(ctx context.Context, pedir func(context.Context) (string, error), acompanhar bool) (Resposta, error) {
	url, err := pedir(ctx)
	if err != nil {
		return Resposta{}, err
	}
	if a.abrirNavegador == nil || a.abrirNavegador(url) != nil {
		return Resposta{}, erroNavegador()
	}
	if acompanhar {
		a.acompanharAssinatura()
		return aceito("Conclua o pagamento no navegador; o app confere sozinho")
	}
	return aceito("Gerencie a assinatura no navegador")
}

// acompanharAssinatura renova a licença a cada 5 s por até 10 min, e para antes quando vira assinante ou a pessoa
// sai da conta. Só uma por vez.
func (a *API) acompanharAssinatura() {
	if !a.acompanhando.CompareAndSwap(false, true) {
		return
	}
	cadencia, prazo := a.cadenciaAssinar, a.prazoAssinar
	if cadencia <= 0 {
		cadencia = cadenciaAssinarPadrao
	}
	if prazo <= 0 {
		prazo = prazoAssinarPadrao
	}
	go func() {
		defer a.acompanhando.Store(false)
		fim := time.NewTimer(prazo)
		tic := time.NewTicker(cadencia)
		defer fim.Stop()
		defer tic.Stop()
		for {
			select {
			case <-fim.C:
				return
			case <-tic.C:
			}
			ctx, cancelar := context.WithTimeout(context.Background(), 30*time.Second)
			_ = a.conta.AtualizarLicenca(ctx) // sem internet ou sem conta: tenta de novo no próximo tique
			cancelar()
			if s := a.conta.Situacao(); !s.Entrada || s.Estado == licenca.Assinante {
				return
			}
		}
	}()
}
