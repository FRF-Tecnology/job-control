package navegador

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/FRF-Tecnology/job-control/internal/dominio"
	"github.com/FRF-Tecnology/job-control/internal/executor"
)

// pagina é uma aba. Tudo o que roda na página é script nosso (constantes deste arquivo), com os valores passados
// como texto JSON: nada que a página devolve é executado. O retrato só lê o DOM.
type pagina struct {
	aba    context.Context
	fechar context.CancelFunc
}

var _ executor.Pagina = (*pagina)(nil)

const (
	tempoAbrir       = 45 * time.Second
	tempoAcao        = 20 * time.Second
	tempoNavegacao   = 15 * time.Second // até a aba sair do about:blank
	tempoConfirmacao = 15 * time.Second
)

// rodar executa a ação na aba, respeitando o cancelamento do executor e um tempo máximo.
func rodar[T any](ctx context.Context, p *pagina, limite time.Duration, acao chromedp.Action[T]) (T, error) {
	c, cancelar := context.WithTimeout(p.aba, limite)
	defer cancelar()
	parar := context.AfterFunc(ctx, cancelar)
	defer parar()
	v, err := chromedp.Run(c, acao)
	if ctx.Err() != nil {
		return v, ctx.Err()
	}
	return v, err
}

func (p *pagina) fazer(ctx context.Context, limite time.Duration, passos ...chromedp.Action[chromedp.Void]) error {
	_, err := rodar(ctx, p, limite, chromedp.Steps(passos...))
	return err
}

// Padrões dos botões (texto normalizado: sem acento, minúsculo). Abrir o formulário e avançar nunca clicam em
// enviar; nada clica em botão que leva para fora ("no site da empresa").
const (
	botaoAbrir   = `^(candidatura simplificada|easy apply|candidatar-se|candidate-se|candidatar|quero me candidatar|inscrever-se|aplicar|aplicar agora|apply|apply now|postularse|postular|postularme|solicitar empleo)$`
	botaoProxima = `^(avancar|proximo|proxima|proxima etapa|continuar|seguinte|next|continue|siguiente|continuar al siguiente paso|revisar|review)\b`
	botaoEnviar  = `^(enviar|enviar candidatura|enviar inscricao|finalizar candidatura|concluir candidatura|submit|submit application|send application|enviar solicitud|enviar postulacion|postularme ahora)\b`
	botaoFora    = `site|website|externo|external|sitio|empresa`
)

var confirmacao = regexp.MustCompile(`candidatura (foi )?(enviada|recebida|realizada)|recebemos (a )?sua candidatura|inscricao (realizada|enviada|confirmada)|obrigad[oa] por se candidatar|application (was )?(sent|submitted|received)|your application (was|has been) (sent|submitted|received)|thank you for applying|solicitud enviada|postulacion (enviada|recibida)|hemos recibido tu (postulacion|solicitud)|gracias por postularte`)

// Abrir navega para a vaga aprovada e, se a página só tem a descrição, clica no botão de candidatura da própria
// plataforma (nunca no de site externo). Devolve onde a página parou.
func (p *pagina) Abrir(ctx context.Context, url string) (string, error) {
	if err := p.fazer(ctx, tempoAbrir, chromedp.Navigate(url), chromedp.WaitReady(chromedp.CSS("body"))); err != nil {
		return "", err
	}
	// A navegação tem de ter saído do about:blank antes de o executor conferir o endereço (P0 da .6.5).
	if err := p.esperarEndereco(ctx); err != nil {
		return "", err
	}
	r, err := p.retrato(ctx)
	if err != nil {
		return "", err
	}
	if len(executor.ClassificarCampos(r)) == 0 {
		if _, err := p.clicar(ctx, botaoAbrir, botaoFora); err != nil {
			return "", err
		}
	}
	return p.URLAtual(ctx)
}

// esperarEndereco espera a aba sair do about:blank (endereço http ou https), até tempoNavegacao. Se não sair,
// segue: o executor vê o about:blank e registra erro técnico, nunca site-externo.
func (p *pagina) esperarEndereco(ctx context.Context) error {
	fim := time.Now().Add(tempoNavegacao)
	for {
		u, err := p.URLAtual(ctx)
		if err != nil {
			return err
		}
		if strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://") || time.Now().After(fim) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (p *pagina) URLAtual(ctx context.Context) (string, error) {
	return rodar(ctx, p, tempoAcao, chromedp.Location())
}

func (p *pagina) retrato(ctx context.Context) (executor.Retrato, error) {
	var r executor.Retrato
	bruto, err := rodar(ctx, p, tempoAcao, chromedp.Evaluate[string](scriptRetrato))
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal([]byte(bruto), &r); err != nil {
		return r, errors.New("navegador: retrato inválido")
	}
	return r, nil
}

func (p *pagina) Bloqueio(ctx context.Context) (string, error) {
	r, err := p.retrato(ctx)
	if err != nil {
		return "", err
	}
	return executor.DetectarBloqueio(r), nil
}

func (p *pagina) Campos(ctx context.Context) ([]executor.Campo, error) {
	r, err := p.retrato(ctx)
	if err != nil {
		return nil, err
	}
	return executor.ClassificarCampos(r), nil
}

// chamada monta "funcao(arg1, arg2)" com os argumentos em JSON (texto, nunca código).
func chamada(funcao string, args ...any) (string, error) {
	partes := make([]string, len(args))
	for i, a := range args {
		b, err := json.Marshal(a)
		if err != nil {
			return "", err
		}
		partes[i] = string(b)
	}
	return "(" + funcao + ")(" + strings.Join(partes, ", ") + ")", nil
}

func (p *pagina) Preencher(ctx context.Context, c executor.Campo, valor string) error {
	js, err := chamada(scriptPreencher, c.ID, valor)
	if err != nil {
		return err
	}
	res, err := rodar(ctx, p, tempoAcao, chromedp.Evaluate[string](js))
	if err != nil {
		return err
	}
	if res == "sem-opcao" {
		return executor.ErrSemOpcao // o executor decide se fica o que a página já marcou
	}
	if res != "ok" {
		return fmt.Errorf("navegador: não preencheu (%s)", res) // só o código fixo, sem valor nem rótulo
	}
	return nil
}

// Igual confere dentro da página se o valor que o campo já tem é o esperado. Só "igual", "diferente" ou "sem-campo"
// voltam: o valor da página nunca sai dela (jc-a2y.6.5.1).
func (p *pagina) Igual(ctx context.Context, c executor.Campo, esperados []string) (bool, error) {
	if len(esperados) == 0 {
		return false, nil
	}
	js, err := chamada(scriptIgual, c.ID, esperados, c.Tipo)
	if err != nil {
		return false, err
	}
	res, err := rodar(ctx, p, tempoAcao, chromedp.Evaluate[string](js))
	if err != nil {
		return false, err
	}
	switch res {
	case "igual":
		return true, nil
	case "diferente":
		return false, nil
	}
	return false, fmt.Errorf("navegador: não conferiu (%s)", res) // só o código fixo
}

// Desmarcar desmarca a caixa (clique, como a pessoa faria, para o formulário saber) e confere que ficou desmarcada.
// Nunca marca: caixa já desmarcada fica como está (jc-a2y.6.5.2).
func (p *pagina) Desmarcar(ctx context.Context, c executor.Campo) error {
	js, err := chamada(scriptDesmarcar, c.ID)
	if err != nil {
		return err
	}
	res, err := rodar(ctx, p, tempoAcao, chromedp.Evaluate[string](js))
	if err != nil {
		return err
	}
	if res != "ok" {
		return fmt.Errorf("navegador: não desmarcou (%s)", res) // só o código fixo
	}
	return nil
}

// Anexar põe no campo de arquivo o caminho que o executor passou (a cópia conferida, #43).
func (p *pagina) Anexar(ctx context.Context, c executor.Campo, caminho string) error {
	return p.fazer(ctx, tempoAcao, chromedp.SetUploadFiles(chromedp.CSS(c.ID), []string{caminho}))
}

// clicar clica no primeiro botão visível e ativo cujo texto casa com padrao e não casa com evitar.
func (p *pagina) clicar(ctx context.Context, padrao, evitar string) (bool, error) {
	js, err := chamada(scriptClicar, padrao, evitar)
	if err != nil {
		return false, err
	}
	clicou, err := rodar(ctx, p, tempoAcao, chromedp.Evaluate[bool](js))
	if err != nil {
		return false, err
	}
	if clicou {
		return true, p.esperar(ctx)
	}
	return false, nil
}

// esperar dá tempo para a página reagir ao clique (troca de etapa ou navegação).
func (p *pagina) esperar(ctx context.Context) error {
	return p.fazer(ctx, tempoAbrir, chromedp.Sleep(700*time.Millisecond), chromedp.WaitReady(chromedp.CSS("body")))
}

// Proxima clica em "avançar" (nunca em enviar). false quando não há próxima etapa.
func (p *pagina) Proxima(ctx context.Context) (bool, error) {
	return p.clicar(ctx, botaoProxima, botaoEnviar+"|"+botaoFora)
}

func (p *pagina) Enviar(ctx context.Context) error {
	clicou, err := p.clicar(ctx, botaoEnviar, botaoFora)
	if err != nil {
		return err
	}
	if !clicou {
		return executor.ErrSemBotaoEnviar // nada foi clicado: o executor bloqueia em vez de deixar incerta
	}
	return nil
}

// Confirmacao espera a página dizer que recebeu a candidatura.
func (p *pagina) Confirmacao(ctx context.Context) (bool, error) {
	fim := time.Now().Add(tempoConfirmacao)
	for {
		r, err := p.retrato(ctx)
		if err == nil && confirmacao.MatchString(dominio.Normalizar(r.Texto)) {
			return true, nil
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

func (p *pagina) Fechar() error {
	p.fechar()
	return nil
}

// scriptRetrato só lê o DOM: texto visível, campos com rótulo e um seletor único, iframes e botões.
const scriptRetrato = `(() => {
  const visivel = el => { const r = el.getBoundingClientRect(); const s = getComputedStyle(el); return s.display !== 'none' && s.visibility !== 'hidden' && (r.width > 0 || r.height > 0); };
  const unico = sel => { try { return document.querySelectorAll(sel).length === 1; } catch (e) { return false; } };
  const limpar = t => (t || '').replace(/\s+/g, ' ').trim().slice(0, 300);
  const seletor = el => {
    if (el.id && unico('#' + CSS.escape(el.id))) return '#' + CSS.escape(el.id);
    const tag = el.tagName.toLowerCase();
    if (el.name) { const s = tag + '[name="' + CSS.escape(el.name) + '"]'; if (unico(s)) return s; }
    const partes = []; let e = el;
    while (e && e.nodeType === 1 && e !== document.documentElement) {
      let i = 1, irmao = e;
      while ((irmao = irmao.previousElementSibling)) { if (irmao.tagName === e.tagName) i++; }
      partes.unshift(e.tagName.toLowerCase() + ':nth-of-type(' + i + ')');
      e = e.parentElement;
    }
    return 'html > ' + partes.join(' > ');
  };
  const rotulo = el => {
    const t = [];
    if (el.labels) for (const l of el.labels) t.push(l.innerText);
    if (el.getAttribute('aria-label')) t.push(el.getAttribute('aria-label'));
    const ref = el.getAttribute('aria-labelledby');
    if (ref) for (const id of ref.split(/\s+/)) { const r = document.getElementById(id); if (r) t.push(r.innerText); }
    if (!t.length) { const f = el.closest('fieldset'); const lg = f && f.querySelector('legend'); if (lg) t.push(lg.innerText); }
    return limpar(t.join(' '));
  };
  // preenchido: a página já trouxe um valor. Só sim ou não: o valor nunca sai do retrato (jc-a2y.6.5.1).
  const preenchido = (el, tipo) => {
    if (tipo === 'checkbox') return el.checked;
    if (tipo === 'file') return !!(el.files && el.files.length);
    if (tipo === 'select') { const o = el.options[el.selectedIndex]; return !!o && !o.disabled && o.value !== ''; }
    return (el.value || '').trim() !== '';
  };
  const campos = []; const grupos = new Set();
  for (const el of document.querySelectorAll('input, select, textarea')) {
    const tipo = (el.tagName === 'INPUT' ? (el.getAttribute('type') || 'text') : el.tagName).toLowerCase();
    if (['hidden', 'submit', 'button', 'reset', 'image'].includes(tipo)) continue;
    if (tipo === 'file' ? !(visivel(el) || (el.parentElement && visivel(el.parentElement))) : !visivel(el)) continue;
    if (tipo === 'radio') {
      if (!el.name || grupos.has(el.name)) continue;
      grupos.add(el.name);
      const f = el.closest('fieldset'); const lg = f && f.querySelector('legend');
      const todos = Array.from(document.getElementsByName(el.name));
      campos.push({ id: 'input[type="radio"][name="' + CSS.escape(el.name) + '"]', tipo, rotulo: lg ? limpar(lg.innerText) : rotulo(el), nome: el.name, placeholder: '', autocomplete: '', obrigatorio: todos.some(r => r.required), preenchido: todos.some(r => r.checked) });
      continue;
    }
    campos.push({ id: seletor(el), tipo, rotulo: rotulo(el), nome: el.name || '', placeholder: el.getAttribute('placeholder') || '', autocomplete: el.getAttribute('autocomplete') || '', obrigatorio: !!el.required || el.getAttribute('aria-required') === 'true', preenchido: preenchido(el, tipo) });
  }
  const quadros = Array.from(document.querySelectorAll('iframe')).map(f => f.getAttribute('src') || '').filter(Boolean).slice(0, 50);
  const botoes = Array.from(document.querySelectorAll('button, input[type=submit], input[type=button], [role=button]')).filter(visivel).map(b => limpar(b.innerText || b.value || b.getAttribute('aria-label'))).filter(Boolean).slice(0, 100);
  return JSON.stringify({ url: location.href, texto: (document.body ? document.body.innerText : '').slice(0, 50000), campos, quadros, botoes });
})()`

// scriptPreencher põe o valor no campo do seletor: texto pelo setter nativo e eventos input/change (formulários
// React), select e rádio pela opção de mesmo texto. Devolve "ok" ou um código fixo.
const scriptPreencher = `(sel, valor) => {
  const norm = s => (s || '').normalize('NFD').replace(/\p{M}+/gu, '').toLowerCase().replace(/\s+/g, ' ').trim();
  const avisar = el => { el.dispatchEvent(new Event('input', { bubbles: true })); el.dispatchEvent(new Event('change', { bubbles: true })); };
  if (sel.startsWith('input[type="radio"]')) {
    const alvo = Array.from(document.querySelectorAll(sel)).find(r => norm((r.labels && r.labels[0] && r.labels[0].innerText) || r.value) === norm(valor));
    if (!alvo) return 'sem-opcao';
    alvo.click();
    return 'ok';
  }
  const el = document.querySelector(sel);
  if (!el) return 'sem-campo';
  if (el.tagName === 'SELECT') {
    let op = Array.from(el.options).find(o => norm(o.text) === norm(valor) || norm(o.value) === norm(valor));
    if (!op && /^\+\d{1,3}$/.test(valor)) { const re = new RegExp('(^|[^0-9])\\+?' + valor.slice(1) + '([^0-9]|$)'); op = Array.from(el.options).find(o => !o.disabled && o.value !== '' && (re.test(o.text) || re.test(o.value))); }
    if (!op) return 'sem-opcao';
    el.value = op.value;
    avisar(el);
    return 'ok';
  }
  const proto = el.tagName === 'TEXTAREA' ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
  const definir = Object.getOwnPropertyDescriptor(proto, 'value').set;
  el.focus();
  definir.call(el, valor);
  avisar(el);
  el.blur();
  return 'ok';
}`

// scriptIgual compara o valor atual do campo com o esperado, por tipo: e-mail e nome sem diferença de caixa e acento,
// telefone pelos dígitos (aceita o código do país na frente), código do país pelo número. Devolve só o resultado.
const scriptDesmarcar = `(sel) => {
  const el = document.querySelector(sel);
  if (!el || el.type !== 'checkbox') return 'sem-campo';
  if (el.checked) el.click();
  return el.checked ? 'continua-marcada' : 'ok';
}`

const scriptIgual = `(sel, esperados, tipo) => {
  const norm = s => (s || '').normalize('NFD').replace(/\p{M}+/gu, '').toLowerCase().replace(/\s+/g, ' ').trim();
  const dig = s => (s || '').replace(/\D+/g, '');
  let atuais = [];
  if (sel.startsWith('input[type="radio"]')) {
    const r = Array.from(document.querySelectorAll(sel)).find(r => r.checked);
    if (r) atuais = [(r.labels && r.labels[0] && r.labels[0].innerText) || '', r.value];
  } else {
    const el = document.querySelector(sel);
    if (!el) return 'sem-campo';
    if (el.tagName === 'SELECT') { const o = el.options[el.selectedIndex]; if (o) atuais = [o.text, o.value]; }
    else atuais = [el.value];
  }
  const igual = v => {
    return esperados.some(esperado => {
      if (tipo === 'telefone') { const e = dig(esperado); return e !== '' && dig(v) === e; }
      if (tipo === 'ddi') { const e = dig(esperado); return e !== '' && new RegExp('(^|[^0-9])\\+?' + e + '([^0-9]|$)').test(v); }
      return norm(v) !== '' && norm(v) === norm(esperado);
    });
  };
  return atuais.some(igual) ? 'igual' : 'diferente';
}`

// scriptClicar clica no primeiro botão visível e ativo cujo texto normalizado casa com padrao e não com evitar.
const scriptClicar = `(padrao, evitar) => {
  const norm = s => (s || '').normalize('NFD').replace(/\p{M}+/gu, '').toLowerCase().replace(/\s+/g, ' ').trim();
  const visivel = el => { const r = el.getBoundingClientRect(); const s = getComputedStyle(el); return s.display !== 'none' && s.visibility !== 'hidden' && (r.width > 0 || r.height > 0); };
  const re = new RegExp(padrao), fora = new RegExp(evitar);
  const botoes = Array.from(document.querySelectorAll('button, input[type=submit], input[type=button], [role=button]'));
  const b = botoes.find(b => {
    if (!visivel(b) || b.disabled || b.getAttribute('aria-disabled') === 'true') return false;
    const t = norm(b.innerText || b.value || b.getAttribute('aria-label'));
    return re.test(t) && !fora.test(t);
  });
  if (!b) return false;
  b.click();
  return true;
}`
