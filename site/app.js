// Landing do Applyer: a prévia interativa, a seleção de plataformas e os exemplos de conexão da IA.
// Sem biblioteca e sem build. Tudo o que aparece já está escrito no index.html; este arquivo só troca
// textos, classes e o que fica escondido, a partir de um estado pequeno (função desenhar).
'use strict';

// Os instaladores e o cadastro de avisos ainda não estão publicados.
const SISTEMAS = [
  { k: 'mac', acao: 'Baixar para macOS', icone: 'marcas/mac.svg' },
  { k: 'win', acao: 'Baixar para Windows', icone: 'marcas/win.svg' },
  { k: 'linux', acao: 'Avisar quando sair', icone: 'marcas/linux.svg' },
  { k: 'ios', acao: 'Avisar quando sair', icone: 'marcas/mac.svg' },
  { k: 'android', acao: 'Avisar quando sair', icone: 'marcas/android.svg' }
];

const VAGAS = [
  { ini: 'NV', av: 'c0', cargo: 'Analista de Marketing Pleno', empresa: 'Nuvio Varejo', plat: 'Gupy', sal: 'R$ 7–9 mil', pret: 'R$ 8.000 por mês' },
  { ini: 'LP', av: 'c1', cargo: 'Growth Marketing Manager', empresa: 'Lumen Pay', plat: 'InHire', sal: 'US$ 4–5k', pret: 'US$ 4,500 / month', intl: true },
  { ini: 'FL', av: 'c2', cargo: 'Coordenador de Marketing', empresa: 'Ferrous Logística', plat: 'InHire', sal: 'R$ 11 mil', pret: 'R$ 11.000 por mês' },
  { ini: 'HS', av: 'c4', cargo: 'Especialista em CRM', empresa: 'Halcyon Saúde', plat: 'LinkedIn', sal: 'R$ 9,5 mil', pret: 'R$ 9.500 por mês' }
];

const DECK = [
  { ini: 'AS', av: 'c3', cargo: 'Analista de Marketing Digital Sênior', empresa: 'Aster Saúde', local: 'Híbrido · São Paulo', sal: 'R$ 9–11 mil/mês', brl: 'CLT', match: 94, porque: ['Pede mídia paga e CRM, que estão no seu currículo', 'Híbrido, 2 dias no escritório', 'Paga acima da sua pretensão'] },
  { ini: 'PC', av: 'c1', cargo: 'Marketing Manager, LATAM', empresa: 'Polar Commerce', local: 'Remoto · Américas', sal: 'US$ 4–5 mil/mês', brl: '≈ R$ 22–27 mil', match: 89, porque: ['Seu inglês avançado bate com o que pedem', 'Experiência com e-commerce, como você', 'Processo curto, em 3 etapas'] },
  { ini: 'SE', av: 'c4', cargo: 'Coordenador de Conteúdo', empresa: 'Sierra Educação', local: 'Remoto · Brasil', sal: 'R$ 10 mil/mês', brl: 'PJ', match: 87, porque: ['Pede gestão de time pequeno, como no seu último cargo', 'Empresa remota desde o começo', 'Sem teste para fazer em casa'] }
];

const PROVEDORES = [
  { id: 'claude', nome: 'Claude', empresa: 'Anthropic', curto: 'Assinatura via Claude Code', estado: 'No app', detalhe: 'Entre no Claude Code com a conta da sua assinatura. O Applyer usa essa sessão no computador. Se preferir, conecte a API.',
    metodos: [{ t: 'Assinatura do Claude', s: 'Conecte sua conta pelo Claude Code instalado no computador.', k: 'term' }, { t: 'Chave da Anthropic', s: 'Configuração no cofre do sistema.', k: 'chave' }] },
  { id: 'chatgpt', nome: 'ChatGPT', empresa: 'OpenAI', curto: 'API da OpenAI', estado: 'No app', detalhe: 'Use os modelos da OpenAI com sua chave de API. A cobrança da API é separada da assinatura do ChatGPT.',
    metodos: [{ t: 'Assinatura via Codex · prevista', s: 'Está prevista a conexão com a conta ChatGPT usada no Codex.', k: 'term' }, { t: 'Chave da OpenAI', s: 'Disponível no app. Configuração no cofre do sistema.', k: 'chave' }] },
  { id: 'gemini', nome: 'Gemini', empresa: 'Google', curto: 'API do Gemini', estado: 'No app', detalhe: 'Conecte os modelos Gemini com uma chave de API do Google.',
    metodos: [{ t: 'Assinatura Google AI · prevista', s: 'Está prevista a conexão pela conta Google com plano compatível.', k: 'term' }, { t: 'Chave do Google', s: 'Disponível no app. Configuração no cofre do sistema.', k: 'chave' }] },
  { id: 'grok', nome: 'Grok', empresa: 'xAI', curto: 'API da xAI', estado: 'No app', detalhe: 'Use sua chave da xAI e um modelo Grok habilitado na sua conta pela conexão de API compatível com OpenAI.',
    metodos: [{ t: 'Conta Grok · prevista', s: 'Está prevista a opção de entrar com a sua conta Grok.', k: 'term' }, { t: 'Chave da xAI', s: 'Disponível pela conexão de API compatível no app.', k: 'chave' }] },
  { id: 'openrouter', nome: 'OpenRouter', empresa: 'OpenRouter', curto: 'API compatível', estado: 'No app', detalhe: 'No app, configure o endereço de API compatível, a chave do OpenRouter e o modelo.',
    metodos: [{ t: 'Chave do OpenRouter', s: 'O uso é cobrado pelo provedor.', k: 'chave' }] },
  { id: 'local', nome: 'No seu computador', empresa: 'Modelos locais', curto: 'Modelos locais previstos', estado: 'Previsto', detalhe: 'O suporte a modelos que rodam no seu computador está previsto.',
    metodos: [{ t: 'Modelo local', s: 'Integração em desenvolvimento.', k: 'pc' }] }
];
// ---------- estado ----------
const estado = {
  plat: 'win', menu: null,
  aba: 'hoje', triIdx: 0, fora: {}, prov: 'claude',
  passo: 0, vagaIdx: 0, lote: VAGAS.map((_, i) => i), pipFixo: false,
};

const $ = (sel, raiz = document) => raiz.querySelector(sel);
const $$ = (sel, raiz = document) => [...raiz.querySelectorAll(sel)];
// Só mexe na página quando o valor muda: quem está lendo com leitor de tela não ouve repetição,
// e as animações de entrada (digitar, trocar de cartão) tocam uma vez só.
const texto = (el, v) => { const s = String(v); if (el && el.textContent !== s) el.textContent = s; };
const classe = (el, nome, ligada) => { if (el && el.classList.contains(nome) !== ligada) el.classList.toggle(nome, ligada); };
const atributo = (el, nome, v) => { if (el && el.getAttribute(nome) !== String(v)) el.setAttribute(nome, String(v)); };
const escondido = (el, v) => { if (el && el.hidden !== v) el.hidden = v; };
const ligar = (chave, v) => { for (const el of $$(`[data-liga="${chave}"]`)) texto(el, v); };
// Reinicia a transição visual ao trocar de vaga na triagem.
const repetir = el => { if (!el) return; el.style.animation = 'none'; void el.offsetWidth; el.style.animation = ''; };

// ---------- plataforma selecionada ----------
const sistema = k => SISTEMAS.find(s => s.k === k) ?? SISTEMAS[1];
function sistemaDaPessoa() {
  const ua = navigator.userAgent || '';
  if (/iPhone|iPad|iPod/.test(ua)) return 'ios';
  if (/Android/.test(ua)) return 'android';
  if (/Mac/.test(ua)) return 'mac';
  if (/Linux/.test(ua)) return 'linux';
  return 'win';
}

// ---------- navegador de demonstração ----------
// Sete passos por vaga: intervalo, abrir, três de preencher, conferir, enviada.
const PASSOS = ['Intervalo humano', 'Abrindo a vaga', 'Preenchendo', 'Preenchendo', 'Preenchendo', 'Conferindo', 'Enviada'];
const PREENCHIDOS = [0, 0, 2, 4, 5, 5, 5];
const FOCO = { 2: 1, 3: 3, 4: 4 };
function camposDe(v) {
  return v.intl ? [
    ['Full name', 'Vinícius Prado'], ['Email', 'vini.prado@example.com'], ['Resume', 'Prado_CV_EN.pdf'],
    ['Visa sponsorship?', 'No — contractor in Brazil'], ['Expected compensation', v.pret],
  ] : [
    ['Nome completo', 'Vinícius Prado'], ['E-mail', 'vini.prado@example.com'], ['Currículo', 'Prado_Curriculo.pdf'],
    ['Disponibilidade para início', 'Imediata'], ['Pretensão salarial', v.pret],
  ];
}
function desenharNavegador(raiz) {
  const p = estado.passo, v = VAGAS[estado.vagaIdx], foco = FOCO[p];
  texto($('[data-url]', raiz), `${v.plat.toLowerCase()} · ${v.empresa.toLowerCase().replace(/ /g, '')} · apply`);
  texto($('[data-plat-nome]', raiz), v.plat);
  escondido($('[data-carrega]', raiz), p !== 1);
  const av = $('[data-av]', raiz);
  atributo(av, 'class', `av ${v.av}`); texto(av, v.ini);
  texto($('[data-cargo]', raiz), v.cargo);
  texto($('[data-empresa]', raiz), `${v.empresa} · Apply`);
  camposDe(v).forEach(([rot, val], i) => {
    const campo = $(`[data-campo="${i}"]`, raiz);
    texto($('[data-rot]', campo), rot);
    const ok = i < PREENCHIDOS[p];
    const valor = $('[data-val]', campo);
    if (ok) texto(valor, val);
    escondido(valor, !ok);
    classe(campo, 'foco', i === foco);
  });
  const enviar = $('[data-enviar]', raiz);
  classe(enviar, 'pronto', p === 5); classe(enviar, 'foi', p === 6);
  texto(enviar, p === 6 ? 'Enviada ✓' : 'Submit application');

}

// ---------- desenhar tudo a partir do estado ----------
const NOMES_ABA = { hoje: 'Hoje', triagem: 'Triagem', lista: 'Lista do dia', aovivo: 'Ao vivo', quadro: 'Quadro' };
const ARTE_METODO = { term: 'artes/terminal.svg', chave: 'artes/chave.svg', pc: 'marcas/local.svg' };

function desenhar() {
  // Mantém a plataforma selecionada nos dois menus.
  const atual = sistema(estado.plat);
  for (const bs of $$('[data-bs]')) {
    texto($('[data-bs-texto]', bs), atual.acao);
    atributo($('[data-bs-icone]', bs), 'src', atual.icone);
    const aberto = estado.menu === bs;
    atributo($('[data-bs-seta]', bs), 'aria-expanded', aberto);
    escondido($('[data-bs-menu]', bs), !aberto);
    for (const item of $$('[data-plat]', bs)) {
      const on = item.dataset.plat === estado.plat;
      classe(item, 'on', on); atributo(item, 'aria-pressed', on);
    }
  }

  // prévia do app
  const p = estado.passo, v = VAGAS[estado.vagaIdx];
  const nSel = VAGAS.filter((_, i) => !estado.fora[i]).length;
  const enviadas = estado.lote.indexOf(estado.vagaIdx) + (p === 6 ? 1 : 0);
  ligar('abaNome', NOMES_ABA[estado.aba]);
  ligar('nSel', nSel);
  $('[data-aprovar]').disabled = nSel === 0;
  texto($('[data-avancar]'), enviadas === estado.lote.length ? 'Rever simulação' : 'Avançar simulação');
  ligar('enviadasDe', `${enviadas}/${estado.lote.length}`);
  ligar('enviadasHoje', enviadas + 3);
  ligar('pipStatus', `${PASSOS[p]} · ${v.empresa}`);
  for (const b of $$('[data-aba]')) { const on = b.dataset.aba === estado.aba; classe(b, 'on', on); atributo(b, 'aria-pressed', on); }
  for (const t of $$('[data-tela]')) escondido(t, t.dataset.tela !== estado.aba);
  const anel = $('[data-anel]');
  if (anel) anel.style.strokeDashoffset = (251.33 * (1 - Math.min((enviadas + 3) / 30, 1))).toFixed(1);

  const t = DECK[estado.triIdx];
  ligar('triPos', `${estado.triIdx + 1} de ${DECK.length}`);
  const triAv = $('[data-tri-av]');
  atributo(triAv, 'class', `av av-g ${t.av}`); texto(triAv, t.ini);
  ligar('tri.cargo', t.cargo); ligar('tri.onde', `${t.empresa} · ${t.local}`); ligar('tri.match', t.match);
  ligar('tri.sal', t.sal); ligar('tri.brl', t.brl);
  t.porque.forEach((m, i) => ligar(`tri.p${i}`, m));

  VAGAS.forEach((x, i) => {
    const dentro = !estado.fora[i];
    classe($(`[data-linha="${i}"]`), 'fora', !dentro);
    const chave = $(`[data-chave="${i}"]`);
    classe(chave, 'on', dentro); atributo(chave, 'aria-pressed', dentro);
    atributo(chave, 'aria-label', `${dentro ? 'Tirar' : 'Incluir'} ${x.empresa}`);
    const foi = i < estado.vagaIdx || (i === estado.vagaIdx && p === 6);
    const agora = i === estado.vagaIdx && !foi;
    const linha = $(`[data-fila="${i}"]`);
    escondido(linha, !estado.lote.includes(i));
    classe(linha, 'ativo', agora);
    const pill = $('[data-pill]', linha);
    classe(pill, 'ok', foi); classe(pill, 'vai', agora);
    texto(pill, foi ? 'Enviada' : agora ? PASSOS[p] : 'Na fila');
  });
  for (const n of $$('[data-navegador]')) desenharNavegador(n);
  const pip = $('[data-pip]');
  escondido(pip, estado.aba === 'aovivo');
  classe(pip, 'fixo', estado.pipFixo);
  const fixar = $('[data-pip-fixar]');
  atributo(fixar, 'aria-pressed', estado.pipFixo);
  atributo(fixar, 'aria-label', estado.pipFixo ? 'Recolher a prévia do navegador' : 'Ampliar a prévia do navegador');

  // conecte sua IA
  const prov = PROVEDORES.find(x => x.id === estado.prov) ?? PROVEDORES[0];
  for (const b of $$('[data-prov]')) { const on = b.dataset.prov === prov.id; classe(b, 'on', on); atributo(b, 'aria-pressed', on); }
  for (const img of $$('[data-prov-icone]')) atributo(img, 'src', `marcas/${prov.id}.svg`);
  for (const k of ['nome', 'curto', 'empresa', 'estado', 'detalhe']) ligar(`prov.${k}`, prov[k]);
  atributo($('[data-prov-estado]'), 'class', `estado ${prov.estado === 'Previsto' ? 'breve' : 'ok'}`);
  const metodos = $('[data-metodos]');
  if (metodos && metodos.dataset.de !== prov.id) {
    metodos.dataset.de = prov.id;
    metodos.replaceChildren(...prov.metodos.map(m => {
      const linha = document.createElement('div'); linha.className = 'metodo';
      const icone = document.createElement('img');
      icone.className = 'metodo-arte';
      icone.alt = '';
      icone.src = ARTE_METODO[m.k];
      const caixa = document.createElement('div');
      const titulo = document.createElement('b'); titulo.textContent = m.t;
      const sub = document.createElement('span'); sub.textContent = m.s;
      caixa.append(titulo, sub); linha.append(icone, caixa);
      return linha;
    }));
  }

}

function iniciarDeslize() {
  const trilho = $('[data-deslize]');
  const valor = $('.deslize-valor', trilho);
  const guia = $('.deslize-guia', trilho);
  const carro = $('.deslize-carro', guia);
  const alca = $('.knob', guia);
  const status = $('.deslize-status', trilho);
  const icone = $('[data-deslize-icone]', trilho);
  const reiniciar = $('[data-deslize-reiniciar]');
  const confirmado = 'Lote aprovado na demonstração';
  let arraste = null, quadro = 0;

  function posicionar(fracao) {
    carro.style.transform = `translate3d(${fracao * 100}%,0,0)`;
    const passo = Math.floor(fracao * 10) * 10;
    if (valor.valueAsNumber !== passo) valor.value = passo;
    atributo(valor, 'aria-valuetext', fracao === 1 ? confirmado : passo + '%');
  }

  function assentar(fracao) {
    const aprovado = fracao === 1;
    classe(trilho, 'on', aprovado);
    posicionar(fracao);
    texto(status, aprovado ? confirmado : '');
    atributo(icone, 'd', aprovado ? 'm4 12 5 5 11-11' : 'M3 13h15M13 7l6 6-6 5');
    escondido(reiniciar, !aprovado);
  }

  function encerrar(aprovado) {
    const id = arraste.id;
    arraste = null;
    cancelAnimationFrame(quadro);
    quadro = 0;
    if (alca.hasPointerCapture(id)) alca.releasePointerCapture(id);
    classe(trilho, 'arrastando', false);
    assentar(aprovado ? 1 : 0);
  }

  alca.addEventListener('pointerdown', evento => {
    if (arraste || !evento.isPrimary || evento.button !== 0 || trilho.classList.contains('on')) return;
    const pista = guia.getBoundingClientRect(), botao = alca.getBoundingClientRect();
    const curso = pista.width - botao.width;
    if (curso <= 0) return;
    evento.preventDefault();
    valor.focus({ preventScroll: true });
    // Se pegar a alça durante a volta, continue do ponto que está sob o dedo.
    const origem = Math.max(0, Math.min(1, (botao.left - pista.left) / curso));
    arraste = { id: evento.pointerId, curso, x: evento.clientX, origem, fracao: origem, pista: trilho.getBoundingClientRect() };
    alca.setPointerCapture(evento.pointerId);
    classe(trilho, 'arrastando', true);
    posicionar(origem);
  });
  function acompanhar(evento) {
    if (arraste?.id !== evento.pointerId) return;
    const { pista, curso, x, origem } = arraste;
    if (evento.clientY < pista.top || evento.clientY > pista.bottom) return encerrar(false);
    const delta = evento.clientX - x;
    arraste.fracao = Math.max(0, Math.min(1, origem + delta / curso));
    if (arraste.fracao > .5 && delta > 0) return encerrar(true);
    if (evento.type === 'pointerup') return encerrar(false);
    // O ponteiro pode mandar várias amostras entre pinturas. Desenhe só a mais recente.
    if (!quadro) quadro = requestAnimationFrame(() => {
      quadro = 0;
      posicionar(arraste.fracao);
    });
  }
  alca.addEventListener('pointermove', acompanhar);
  alca.addEventListener('pointerup', acompanhar);
  function cancelar(evento) {
    if (arraste?.id === evento.pointerId) encerrar(false);
  }
  alca.addEventListener('pointercancel', cancelar);
  alca.addEventListener('lostpointercapture', cancelar);
  function interromper() { if (arraste) encerrar(false); }
  window.addEventListener('blur', interromper);
  window.addEventListener('resize', interromper);
  valor.addEventListener('keydown', evento => {
    if (arraste) {
      evento.preventDefault();
      if (evento.key === 'Escape') encerrar(false);
    }
  });
  valor.addEventListener('input', () => {
    const fracao = valor.valueAsNumber / 100;
    assentar(fracao > .5 ? 1 : fracao);
  });
  reiniciar.addEventListener('click', () => {
    assentar(0);
    valor.focus({ preventScroll: true });
  });
}

function fecharMenu(restaurarFoco = false) {
  const menu = estado.menu;
  if (!menu) return;
  estado.menu = null;
  desenhar();
  if (restaurarFoco) $('[data-bs-seta]', menu).focus();
}

// O clique nativo continua atendendo ao toque e ao teclado. Com mouse,
// a primeira entrada já abre a lista e dá tempo de cruzar até o painel.
function abrirComMouse(grupo, gatilho, aberto, abrir, fechar) {
  let relogio;
  let abertoPorHover = false;
  const cancelarSaida = () => clearTimeout(relogio);
  gatilho.addEventListener('pointerenter', evento => {
    if (evento.pointerType !== 'mouse' || !matchMedia('(any-hover: hover)').matches) return;
    cancelarSaida();
    if (!aberto()) {
      abertoPorHover = true;
      abrir();
    }
  });
  gatilho.addEventListener('click', evento => {
    if (evento.detail > 0 && abertoPorHover && aberto()) {
      evento.preventDefault();
      evento.stopPropagation();
    }
    abertoPorHover = false;
  });
  grupo.addEventListener('pointerenter', cancelarSaida);
  grupo.addEventListener('pointerleave', evento => {
    if (evento.pointerType !== 'mouse') return;
    cancelarSaida();
    relogio = setTimeout(() => {
      if (grupo.querySelector(':focus-visible')) return;
      if (aberto()) fechar();
      abertoPorHover = false;
    }, 220);
  });
}

function aoClicar(ev) {
  const alvo = sel => ev.target.closest(sel);
  let el;
  if (estado.menu && !estado.menu.contains(ev.target)) fecharMenu();
  if ((el = alvo('a[href^="#faq-r"]'))) {
    const pergunta = document.getElementById(el.hash.slice(1));
    if (pergunta) pergunta.open = true;
    return;
  }
  if ((el = alvo('[data-bs-seta]'))) {
    const bs = el.closest('[data-bs]');
    $('.nav-explorar').open = false;
    estado.menu = estado.menu === bs ? null : bs;
    return desenhar();
  }
  if ((el = alvo('[data-plat]'))) {
    estado.plat = el.dataset.plat;
    return fecharMenu(true);
  }
  if ((el = alvo('[data-aba]'))) { estado.aba = el.dataset.aba; return desenhar(); }
  if ((el = alvo('[data-ir]'))) {
    estado.aba = el.dataset.ir;
    desenhar();
    return $(`[data-aba="${estado.aba}"]`).focus();
  }
  if (alvo('[data-aprovar]')) {
    const lote = VAGAS.map((_, i) => i).filter(i => !estado.fora[i]);
    if (!lote.length) return;
    Object.assign(estado, { aba: 'aovivo', passo: 0, lote, vagaIdx: lote[0] });
    desenhar();
    return $('[data-avancar]').focus();
  }
  if (alvo('[data-avancar]')) {
    estado.passo = (estado.passo + 1) % PASSOS.length;
    if (estado.passo === 0) {
      const proxima = (estado.lote.indexOf(estado.vagaIdx) + 1) % estado.lote.length;
      estado.vagaIdx = estado.lote[proxima];
    }
    return desenhar();
  }
  if ((el = alvo('[data-triagem]'))) { estado.triIdx = (estado.triIdx + (el.dataset.triagem === 'anterior' ? DECK.length - 1 : 1)) % DECK.length; desenhar(); return repetir($('[data-vaga-cartao]')); }
  if ((el = alvo('[data-chave]'))) { const i = Number(el.dataset.chave); estado.fora[i] = !estado.fora[i]; return desenhar(); }
  if (alvo('[data-pip-fixar]')) { estado.pipFixo = !estado.pipFixo; return desenhar(); }
  if ((el = alvo('[data-prov]'))) { if (estado.prov !== el.dataset.prov) { estado.prov = el.dataset.prov; desenhar(); } return; }
}

function acompanharRolagem() {
  const topo = $('.topo');
  const marca = $('.marca', topo);
  const nav = $('.links', topo);
  const explorar = $('.nav-explorar', topo);
  function medir() {
    topo.style.setProperty('--largura-marca', (marca.offsetWidth + 20) + 'px');
    topo.style.setProperty('--marca-x', ((topo.clientWidth - marca.offsetWidth) / 2 - marca.offsetLeft) + 'px');
    topo.style.setProperty('--marca-y', ((46 - marca.offsetHeight) / 2 - marca.offsetTop) + 'px');
  }
  medir();
  if ('ResizeObserver' in window) new ResizeObserver(medir).observe(topo);
  else window.addEventListener('resize', medir);

  function recolher(valor) {
    if (topo.classList.contains('recolhido') === valor) return;
    if (valor) explorar.open = false;
    nav.inert = valor;
    topo.classList.toggle('recolhido', valor);
  }
  topo.addEventListener('focusin', () => recolher(false));
  let referencia = Math.max(0, window.scrollY);
  let agendado = false;
  function atualizar() {
    agendado = false;
    const y = Math.max(0, window.scrollY);
    const delta = y - referencia;
    if (y < 60 || delta < -8) recolher(false);
    else if (y > 110 && delta > 14 && !topo.querySelector(':focus-visible')) recolher(true);
    else if (Math.abs(delta) < 15) return;
    referencia = y;
  }
  window.addEventListener('scroll', () => {
    if (agendado) return;
    agendado = true;
    requestAnimationFrame(atualizar);
  }, { passive:true });
}

function iniciar() {
  iniciarDeslize();
  estado.plat = sistemaDaPessoa();
  document.addEventListener('click', aoClicar);
  const explorar = $('.nav-explorar');
  abrirComMouse(explorar, $('summary', explorar), () => explorar.open,
    () => { fecharMenu(); explorar.open = true; },
    () => { explorar.open = false; });
  for (const bs of $$('[data-bs]')) {
    abrirComMouse(bs, $('[data-bs-seta]', bs), () => estado.menu === bs,
      () => { explorar.open = false; estado.menu = bs; desenhar(); },
      () => fecharMenu());
  }
  document.addEventListener('click', evento => {
    if (!explorar.contains(evento.target) || evento.target.closest('a')) explorar.open = false;
  });
  explorar.addEventListener('focusout', evento => {
    if (!explorar.contains(evento.relatedTarget)) explorar.open = false;
  });
  document.addEventListener('keydown', evento => {
    if (evento.key === 'Escape' && explorar.open) {
      const devolverFoco = explorar.contains(document.activeElement);
      explorar.open = false;
      if (devolverFoco) $('summary', explorar).focus();
    }
  });
  document.addEventListener('focusin', ev => {
    if (estado.menu && !estado.menu.contains(ev.target)) fecharMenu();
  });
  document.addEventListener('keydown', ev => {
    if (ev.key === 'Escape' && estado.menu) fecharMenu(estado.menu.contains(document.activeElement));
  });
  const abrirPergunta = () => {
    const pergunta = document.getElementById(location.hash.slice(1));
    if (pergunta?.matches('details.faq-item')) pergunta.open = true;
  };
  window.addEventListener('hashchange', abrirPergunta);
  abrirPergunta();
  desenhar();
  document.documentElement.classList.add('js');
  acompanharRolagem();
}
iniciar();

// O gesto responde à interação; a animação ambiente só roda nos cartões visíveis.
function iniciarCenasDaRotina() {
  const secao = $('#como');
  const pausa = $('[data-pausar-rotina]', secao);
  const cartoes = $$('.hora', secao);
  pausa.addEventListener('click', () => {
    const pausada = secao.classList.toggle('rotina-pausada');
    texto(pausa, pausada ? 'Retomar animações' : 'Pausar animações');
  });
  const acompanharAba = () => secao.classList.toggle('rotina-sem-visibilidade', document.hidden);
  acompanharAba();
  document.addEventListener('visibilitychange', acompanharAba);
  if (!('IntersectionObserver' in window)) {
    for (const cartao of cartoes) cartao.classList.add('em-tela');
    return;
  }
  const cenas = new IntersectionObserver(entradas => {
    for (const entrada of entradas) {
      entrada.target.classList.toggle('em-tela', entrada.isIntersecting);
      if (entrada.isIntersecting) entrada.target.classList.add('em-vista');
    }
  });
  for (const cartao of cartoes) {
    cartao.classList.add('prepara-luz');
    cenas.observe(cartao);
  }
}
iniciarCenasDaRotina();
