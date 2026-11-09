package navegador

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/executor"
)

// Envio de formulários externos (jc-a2y.7.2, parte 3b-2): a página do formulário para o executor
// (executor.PaginaFormulario). Google Forms pelo formato PÚBLICO do DOM, NÃO conferido contra um formulário real
// (decisão do PO, a mesma do #124): cada pergunta é achada pelo campo oculto "entry.N" dentro do bloco da pergunta
// ([role=listitem]); a opção, pelo rótulo exato (data-value). Tudo o que roda na página é script nosso, com os valores
// em JSON; só códigos fixos voltam, nunca o valor nem o texto da página. O Typeform nem chega aqui (bloqueado na
// leitura).

var _ executor.PaginaFormulario = (*pagina)(nil)

const (
	// botaoEnviarFormulario: o botão final do formulário. "Limpar formulário" nunca.
	botaoEnviarFormulario = `^(enviar|submit|send|enviar respuesta|enviar formulario)$`
	botaoNaoEnviar        = botaoFora + `|limpar|clear|borrar`
	maxRespondidas        = 200
)

// respostaRegistrada: a página final do formulário (a lista fixa de hoje, mais as frases do formulário).
var respostaRegistrada = regexp.MustCompile(`sua resposta foi registrada|suas respostas foram registradas|your response has been recorded|se ha registrado tu respuesta|se registro tu respuesta|thanks for completing this typeform`)

var reEntrada = regexp.MustCompile(`^entry\.[0-9]{1,20}$`)

func (p *pagina) avaliar(ctx context.Context, funcao string, args ...any) (string, error) {
	js, err := chamada(funcao, args...)
	if err != nil {
		return "", err
	}
	return rodar(ctx, p, tempoAcao, chromedp.Evaluate[string](js))
}

// RespondidasPelaPagina: os ids (entry.N) das perguntas que a página já trouxe com valor. O valor não sai da página.
func (p *pagina) RespondidasPelaPagina(ctx context.Context) ([]string, error) {
	bruto, err := p.avaliar(ctx, scriptRespondidas)
	if err != nil {
		return nil, err
	}
	var ids []string
	if len(bruto) > 20_000 || json.Unmarshal([]byte(bruto), &ids) != nil || len(ids) > maxRespondidas {
		return nil, errors.New("navegador: resposta da página fora do formato")
	}
	for _, id := range ids {
		if !reEntrada.MatchString(id) {
			return nil, errors.New("navegador: resposta da página fora do formato")
		}
	}
	return ids, nil
}

// CopiaMarcada: se a caixa "enviar uma cópia das respostas para o meu e-mail" está marcada.
func (p *pagina) CopiaMarcada(ctx context.Context) (bool, error) {
	res, err := p.avaliar(ctx, scriptCopia, false)
	if err != nil {
		return false, err
	}
	switch res {
	case "nenhuma", "desmarcada":
		return false, nil
	case "marcada":
		return true, nil
	}
	return false, fmt.Errorf("navegador: cópia (%s)", res) // só o código fixo
}

// DesmarcarCopia clica na caixa marcada (como a pessoa faria) e confere que ficou desmarcada. Nunca marca.
func (p *pagina) DesmarcarCopia(ctx context.Context) error {
	res, err := p.avaliar(ctx, scriptCopia, true)
	if err != nil {
		return err
	}
	if res != "nenhuma" && res != "desmarcada" {
		return fmt.Errorf("navegador: não desmarcou a cópia (%s)", res)
	}
	return nil
}

// Responder põe o valor na pergunta e confere que ficou. Opção pelo rótulo exato; sem ela, executor.ErrSemOpcao.
func (p *pagina) Responder(ctx context.Context, q dominio.PerguntaFormulario, valor string) error {
	if !reEntrada.MatchString(q.ID) {
		return errors.New("navegador: id de pergunta fora do formato")
	}
	res, err := p.avaliar(ctx, scriptResponder, q.ID, q.Tipo, valor)
	if err != nil {
		return err
	}
	switch res {
	case "ok":
		return nil
	case "sem-opcao":
		return executor.ErrSemOpcao
	}
	return fmt.Errorf("navegador: não respondeu (%s)", res) // só o código fixo, sem valor nem pergunta
}

// EnviarFormulario clica uma vez no botão final ("Enviar"). Sem ele, nada é clicado.
func (p *pagina) EnviarFormulario(ctx context.Context) error {
	clicou, err := p.clicar(ctx, botaoEnviarFormulario, botaoNaoEnviar)
	if err != nil {
		return err
	}
	if !clicou {
		return executor.ErrSemBotaoEnviar
	}
	return nil
}

// RespostaRegistrada espera a frase de resposta registrada. A URL e o link "Editar sua resposta" ficam na página.
func (p *pagina) RespostaRegistrada(ctx context.Context) (bool, error) {
	fim := time.Now().Add(tempoConfirmacao)
	for {
		r, err := p.retrato(ctx)
		if err == nil {
			t := dominio.Normalizar(r.Texto)
			if respostaRegistrada.MatchString(t) || confirmacao.MatchString(t) {
				return true, nil
			}
		}
		if time.Now().After(fim) {
			return false, err
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// caixaDaPergunta (trecho comum dos scripts): o bloco da pergunta pelo campo oculto entry.N.
const caixaDaPergunta = `const caixaDe = id => { const o = Array.from(document.querySelectorAll('input[type="hidden"]')).find(i => i.name === id); return o ? [o, o.closest('[role="listitem"]')] : [null, null]; };`

// scriptRespondidas: os entry.N com valor (campo oculto, texto, opção ou caixa marcada). Só os ids voltam.
const scriptRespondidas = `() => {
  const ids = [];
  for (const o of Array.from(document.querySelectorAll('input[type="hidden"]'))) {
    if (!/^entry\.[0-9]{1,20}$/.test(o.name || '')) continue;
    const caixa = o.closest('[role="listitem"]');
    let tem = (o.value || '') !== '';
    if (caixa) {
      if (caixa.querySelector('[role="radio"][aria-checked="true"], [role="checkbox"][aria-checked="true"], [role="option"][aria-selected="true"]:not([data-value=""]), input:checked')) tem = true;
      for (const c of Array.from(caixa.querySelectorAll('input:not([type="hidden"]):not([type="radio"]):not([type="checkbox"]), textarea'))) if ((c.value || '') !== '') tem = true;
    }
    if (tem && !ids.includes(o.name)) ids.push(o.name);
  }
  return JSON.stringify(ids.slice(0, 201));
}`

// scriptCopia acha a caixa "enviar uma cópia para o meu e-mail" (fora das perguntas) pelo texto e diz se está marcada.
// desmarcar: clica se estiver marcada e diz como ficou. Nunca marca.
const scriptCopia = `(desmarcar) => {
  const norm = s => (s || '').normalize('NFD').replace(/\p{M}+/gu, '').toLowerCase().replace(/\s+/g, ' ').trim();
  const re = /(enviar|envie|envia|send|enviarme|enviame|receber|receive|recibir).{0,40}(copia|copy)/;
  const ehPergunta = el => { const c = el.closest('[role="listitem"]'); return !!(c && Array.from(c.querySelectorAll('input[type="hidden"]')).some(i => /^entry\./.test(i.name || ''))); };
  const rotulo = el => norm([el.getAttribute('aria-label'), el.labels && el.labels[0] && el.labels[0].innerText, el.parentElement && el.parentElement.innerText].filter(Boolean).join(' '));
  const marcada = el => el.getAttribute('role') === 'checkbox' ? el.getAttribute('aria-checked') === 'true' : !!el.checked;
  const caixa = Array.from(document.querySelectorAll('[role="checkbox"], input[type="checkbox"]')).find(el => !ehPergunta(el) && re.test(rotulo(el)));
  if (!caixa) return 'nenhuma';
  if (desmarcar && marcada(caixa)) caixa.click();
  return marcada(caixa) ? 'marcada' : 'desmarcada';
}`

// scriptResponder põe o valor na pergunta entry.N e confere: texto e data pelo valor do campo; escolha, escala e
// caixas pela opção de rótulo exato (data-value); lista pela opção de rótulo exato dentro da própria pergunta.
const scriptResponder = `(id, tipo, valor) => {
  ` + caixaDaPergunta + `
  const [oculto, caixa] = caixaDe(id);
  if (!caixa) return 'sem-campo';
  const avisar = el => { el.dispatchEvent(new Event('input', { bubbles: true })); el.dispatchEvent(new Event('change', { bubbles: true })); };
  const marcado = el => el.getAttribute('aria-checked') === 'true';
  if (tipo === 'escolha-unica' || tipo === 'escala' || tipo === 'multipla-escolha') {
    const papel = tipo === 'multipla-escolha' ? 'checkbox' : 'radio';
    const atributo = papel === 'checkbox' ? 'data-answer-value' : 'data-value';
    const alvo = Array.from(caixa.querySelectorAll('[role="' + papel + '"]')).find(r => r.getAttribute(atributo) === valor);
    if (!alvo) return 'sem-opcao';
    if (!marcado(alvo)) alvo.click();
    return marcado(alvo) ? 'ok' : 'nao-ficou';
  }
  if (tipo === 'lista') {
    const lista = caixa.querySelector('[role="listbox"]');
    if (!lista) return 'sem-campo';
    const op = Array.from(caixa.querySelectorAll('[role="option"]')).find(o => o.getAttribute('data-value') === valor);
    if (!op) return 'sem-opcao';
    lista.click();
    op.click();
    return oculto.value === valor ? 'ok' : 'nao-ficou';
  }
  const seletor = tipo === 'texto-longo' ? 'textarea' : (tipo === 'data' ? 'input[type="date"]' : 'input[type="text"], input[type="email"], input[type="tel"], input[type="number"], input[type="url"], input:not([type])');
  const campo = caixa.querySelector(seletor);
  if (!campo) return 'sem-campo';
  const proto = campo.tagName === 'TEXTAREA' ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
  campo.focus();
  Object.getOwnPropertyDescriptor(proto, 'value').set.call(campo, valor);
  avisar(campo);
  campo.blur();
  return campo.value === valor ? 'ok' : 'nao-ficou';
}`

var _ executor.NavegadorDeFormulario = (*Navegador)(nil)

// EnviaFormulario: este navegador envia formulário (as páginas são executor.PaginaFormulario). Sem isso, o executor
// deixa o formulário aprovado esperando, sem tentativa.
func (n *Navegador) EnviaFormulario() bool { return true }
