// Landing do Applyer: a prévia do app, o botão de baixar, a escolha da IA e as perguntas.
// Sem biblioteca e sem build. Tudo o que aparece já está escrito no index.html; este arquivo só troca
// textos, classes e o que fica escondido, a partir de um estado pequeno (função desenhar).
'use strict';

// Endereço do instalador de cada sistema. PREENCHA ANTES DE PUBLICAR: com o campo vazio, o botão só mostra
// a animação de exemplo e não baixa nada (o site diria "Download iniciado" sem ter o que baixar).
const LINKS = { mac: '', win: '' };

const SISTEMAS = [
  { k: 'mac', nome: 'macOS', grupo: 'pc', acao: 'Baixar para macOS', txt: 'O app completo no seu Mac: busca, Lista do dia, envio com navegador embutido e Quadro.', status: 'Disponível', cor: '#D7C4FF', icone: 'marcas/mac.svg' },
  { k: 'win', nome: 'Windows', grupo: 'pc', acao: 'Baixar para Windows', txt: 'O mesmo app no seu PC, com os seus dados guardados numa pasta do computador.', status: 'Disponível', cor: '#9FE3BC', icone: 'marcas/win.svg' },
  { k: 'linux', nome: 'Linux', grupo: 'pc', acao: 'Avisar quando sair', txt: 'Estamos preparando. Ligue o aviso e mandamos o link assim que sair.', status: 'Em breve', cor: '#F5D27E', icone: 'marcas/linux.svg', breve: true },
  { k: 'ios', nome: 'iPhone', grupo: 'cel', acao: 'Avisar quando sair', txt: 'Em breve você poderá acompanhar o envio, pausar e aprovar pelo celular.', status: 'Em breve', cor: '#A6CDFF', icone: 'marcas/mac.svg', breve: true },
  { k: 'android', nome: 'Android', grupo: 'cel', acao: 'Avisar quando sair', txt: 'Em breve você poderá acompanhar o envio, pausar e aprovar pelo celular.', status: 'Em breve', cor: '#FFC1A6', icone: 'marcas/android.svg', breve: true }
];

const VAGAS = [
  { ini: 'NV', av: 'c0', cargo: 'Analista de Marketing Pleno', empresa: 'Nuvio Varejo', plat: 'Gupy', sal: 'R$ 7–9 mil', pret: 'R$ 8.000 por mês' },
  { ini: 'LP', av: 'c1', cargo: 'Growth Marketing Manager', empresa: 'Lumen Pay', plat: 'Greenhouse', sal: 'US$ 4–5k', pret: 'US$ 4,500 / month', intl: true },
  { ini: 'FL', av: 'c2', cargo: 'Coordenador de Marketing', empresa: 'Ferrous Logística', plat: 'InHire', sal: 'R$ 11 mil', pret: 'R$ 11.000 por mês' },
  { ini: 'HS', av: 'c4', cargo: 'Especialista em CRM', empresa: 'Halcyon Saúde', plat: 'LinkedIn', sal: 'R$ 9,5 mil', pret: 'R$ 9.500 por mês' }
];

const DECK = [
  { ini: 'AS', av: 'c3', cargo: 'Analista de Marketing Digital Sênior', empresa: 'Aster Saúde', local: 'Híbrido · São Paulo', sal: 'R$ 9–11 mil/mês', brl: 'CLT', match: 94, porque: ['Pede mídia paga e CRM, que estão no seu currículo', 'Híbrido, 2 dias no escritório', 'Paga acima da sua pretensão'] },
  { ini: 'PC', av: 'c1', cargo: 'Marketing Manager, LATAM', empresa: 'Polar Commerce', local: 'Remoto · Américas', sal: 'US$ 4–5 mil/mês', brl: '≈ R$ 22–27 mil', match: 89, porque: ['Seu inglês avançado bate com o que pedem', 'Experiência com e-commerce, como você', 'Processo curto, em 3 etapas'] },
  { ini: 'SE', av: 'c4', cargo: 'Coordenador de Conteúdo', empresa: 'Sierra Educação', local: 'Remoto · Brasil', sal: 'R$ 10 mil/mês', brl: 'PJ', match: 87, porque: ['Pede gestão de time pequeno, como no seu último cargo', 'Empresa remota desde o começo', 'Sem teste para fazer em casa'] }
];

const PROVEDORES = [
  { id: 'claude', nome: 'Claude', empresa: 'Anthropic', via: 'Claude Code ou chave', curto: 'Claude Code conectado', estado: 'Conectado', estadoK: 'ok',
    cmd: 'applyer ia conectar claude', saida1: 'Claude Code encontrado neste computador', saida2: 'Usando a sua assinatura. Nenhuma chave salva.',
    metodos: [{ t: 'Sua assinatura', s: 'Claude Code já logado no computador', k: 'term' }, { t: 'Sua chave', s: 'Chave da Anthropic, guardada no cofre do sistema', k: 'chave' }] },
  { id: 'chatgpt', nome: 'ChatGPT', empresa: 'OpenAI', via: 'Codex CLI ou chave', curto: 'Codex conectado', estado: 'Em teste', estadoK: 'teste',
    cmd: 'applyer ia conectar chatgpt', saida1: 'Codex CLI encontrado neste computador', saida2: 'Usando a sua assinatura do ChatGPT.',
    metodos: [{ t: 'Sua assinatura', s: 'Codex CLI já logado no computador', k: 'term' }, { t: 'Sua chave', s: 'Chave da OpenAI, guardada no cofre do sistema', k: 'chave' }] },
  { id: 'gemini', nome: 'Gemini', empresa: 'Google', via: 'Gemini CLI ou chave', curto: 'Gemini CLI conectado', estado: 'Pronto para conectar', estadoK: 'chave',
    cmd: 'applyer ia conectar gemini', saida1: 'Gemini CLI encontrado neste computador', saida2: 'Entre com a sua conta Google para usar.',
    metodos: [{ t: 'Sua assinatura', s: 'Gemini CLI com a sua conta Google', k: 'term' }, { t: 'Sua chave', s: 'Chave do Google AI Studio, no cofre do sistema', k: 'chave' }] },
  { id: 'grok', nome: 'Grok', empresa: 'xAI', via: 'Chave da xAI', curto: 'Chave da xAI', estado: 'Com chave', estadoK: 'chave',
    cmd: 'applyer ia conectar grok --chave', saida1: 'Chave da xAI validada', saida2: 'Guardada no cofre do sistema, nunca em arquivo.',
    metodos: [{ t: 'Sua chave', s: 'Chave da xAI, guardada no cofre do sistema', k: 'chave' }] },
  { id: 'openrouter', nome: 'OpenRouter', empresa: 'Vários modelos', via: 'Uma chave, vários modelos', curto: 'OpenRouter', estado: 'Com chave', estadoK: 'chave',
    cmd: 'applyer ia conectar openrouter --chave', saida1: 'Chave do OpenRouter validada', saida2: 'Escolha o modelo em Perfil e IA.',
    metodos: [{ t: 'Sua chave', s: 'Uma chave para usar vários modelos', k: 'chave' }] },
  { id: 'local', nome: 'No seu computador', empresa: 'Modelos locais', via: 'Ollama, LM Studio · em breve', curto: 'Modelo local', estado: 'Em breve', estadoK: 'breve',
    cmd: 'applyer ia conectar local', saida1: 'Procurando modelos rodando neste computador', saida2: 'Em breve: tudo sem sair da sua máquina.',
    metodos: [{ t: 'Sem internet para a IA', s: 'O modelo roda inteiro no seu computador', k: 'pc' }] }
];
// ---------- estado ----------
const estado = {
  plat: 'win', menu: null, baixando: '', baixou: '', avisos: {},
  aba: 'hoje', triIdx: 0, fora: {}, prov: 'claude',
  passo: 0, vagaIdx: 0, pipFixo: false, aprovado: false, periodo: 'mensal',
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
// Toca de novo uma animação de entrada (o cartão da triagem, o terminal da IA).
const repetir = el => { if (!el) return; el.style.animation = 'none'; void el.offsetWidth; el.style.animation = ''; };

// ---------- tema ----------
function alternarTema() {
  const escuro = !document.documentElement.classList.contains('escuro');
  document.documentElement.classList.toggle('escuro', escuro);
  try { localStorage.setItem('tema', escuro ? 'escuro' : 'claro'); } catch { /* sem armazenamento: vale só agora */ }
}

// ---------- baixar ----------
const IC_BAIXAR = 'M12 4v11M7 10l5 5 5-5M5 20h14';
const IC_OK = 'M5 12l5 5 9-10';
const IC_SINO = 'M6 16V11a6 6 0 0 1 12 0v5l2 2H4zM10 21h4';
const sistema = k => SISTEMAS.find(s => s.k === k) ?? SISTEMAS[1];
const feito = s => (s.breve && estado.avisos[s.k]) || estado.baixou === s.k;
function rotulo(s) {
  if (s.breve) return estado.avisos[s.k] ? 'Avisaremos você' : s.acao;
  if (estado.baixando === s.k) return `Baixando para ${s.nome}…`;
  if (estado.baixou === s.k) return 'Download iniciado';
  return s.acao;
}
let relogioBaixar = 0;
function baixar(k) {
  const s = sistema(k);
  if (s.breve) { estado.avisos[s.k] = !estado.avisos[s.k]; estado.menu = null; return desenhar(); }
  clearTimeout(relogioBaixar);
  Object.assign(estado, { plat: k, baixando: k, baixou: '', menu: null });
  desenhar();
  relogioBaixar = setTimeout(() => {
    Object.assign(estado, { baixando: '', baixou: k });
    desenhar();
    if (LINKS[k]) location.href = LINKS[k];
  }, LINKS[k] ? 900 : 2400);
}
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
  let x = 440, y = 80;
  if (p === 1) { x = 220; y = 20; }
  else if (foco !== undefined) { x = 16 + (foco % 2) * 260 + 150; y = 106 + Math.floor(foco / 2) * 60 + 28; }
  else if (p >= 5) { x = 478; y = 254; }
  $('[data-cursor]', raiz).style.transform = `translate(${x}px, ${y}px)`;
}

// ---------- desenhar tudo a partir do estado ----------
const NOMES_ABA = { hoje: 'Hoje', triagem: 'Triagem', lista: 'Lista do dia', aovivo: 'Ao vivo', quadro: 'Quadro' };
const IC_METODO = { term: 'M4 17l6-6-6-6M12 19h8', chave: 'M15 7a4 4 0 1 1-3.9 4.9L3 20v-3h3v-3h3l2.1-2.1A4 4 0 0 1 15 7z', pc: 'M3 4h18v12H3zM8 20h8M12 16v4' };
const SVG = 'http://www.w3.org/2000/svg';

function desenhar() {
  // baixar: os dois botões divididos, os menus e os cartões
  const atual = sistema(estado.plat);
  for (const bs of $$('[data-bs]')) {
    const principal = $('[data-bs-principal]', bs);
    texto($('[data-bs-texto]', bs), rotulo(atual));
    atributo($('[data-bs-icone]', bs), 'src', atual.icone);
    classe(principal, 'baixando', estado.baixando === atual.k);
    classe(principal, 'feito', feito(atual));
    const aberto = estado.menu === bs;
    atributo($('[data-bs-seta]', bs), 'aria-expanded', aberto);
    escondido($('[data-bs-menu]', bs), !aberto);
    for (const item of $$('[data-plat]', bs)) {
      const on = item.dataset.plat === estado.plat;
      classe(item, 'on', on); atributo(item, 'aria-pressed', on);
    }
  }
  escondido($('[data-veu-menu]'), !estado.menu);
  for (const cartao of $$('[data-so]')) {
    const s = sistema(cartao.dataset.so);
    texto($('[data-so-texto]', cartao), rotulo(s));
    classe($('.so-acao', cartao), 'feito', feito(s));
    atributo($('[data-so-icone]', cartao), 'd', feito(s) ? IC_OK : s.breve ? IC_SINO : IC_BAIXAR);
    escondido($('[data-so-barra]', cartao), estado.baixando !== s.k);
  }

  // prévia do app
  const p = estado.passo, v = VAGAS[estado.vagaIdx];
  const nSel = VAGAS.filter((_, i) => !estado.fora[i]).length;
  const enviadas = estado.vagaIdx + (p === 6 ? 1 : 0);
  ligar('abaNome', NOMES_ABA[estado.aba]);
  ligar('nSel', nSel);
  ligar('enviadas', enviadas);
  ligar('enviadasDe', `${enviadas}/${VAGAS.length}`);
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
  atributo(fixar, 'aria-label', estado.pipFixo ? 'Soltar o navegador' : 'Fixar o navegador aberto');

  // deslize para aprovar
  const deslize = $('[data-deslize]');
  classe(deslize, 'on', estado.aprovado); atributo(deslize, 'aria-pressed', estado.aprovado);
  escondido($('[data-deslize-espera]'), estado.aprovado); escondido($('[data-deslize-feito]'), !estado.aprovado);
  atributo($('[data-deslize-icone]'), 'd', estado.aprovado ? IC_OK : 'M5 12h14M13 6l6 6-6 6');

  // conecte sua IA
  const prov = PROVEDORES.find(x => x.id === estado.prov) ?? PROVEDORES[0];
  for (const b of $$('[data-prov]')) { const on = b.dataset.prov === prov.id; classe(b, 'on', on); atributo(b, 'aria-pressed', on); }
  for (const img of $$('[data-prov-icone]')) atributo(img, 'src', `marcas/${prov.id}.svg`);
  for (const k of ['nome', 'curto', 'empresa', 'estado', 'cmd', 'saida1', 'saida2']) ligar(`prov.${k}`, prov[k]);
  atributo($('[data-prov-estado]'), 'class', `estado ${prov.estadoK}`);
  const metodos = $('[data-metodos]');
  if (metodos && metodos.dataset.de !== prov.id) {
    metodos.dataset.de = prov.id;
    metodos.replaceChildren(...prov.metodos.map(m => {
      const linha = document.createElement('div'); linha.className = 'metodo';
      const svg = document.createElementNS(SVG, 'svg');
      svg.setAttribute('class', 'ic'); svg.setAttribute('viewBox', '0 0 24 24'); svg.setAttribute('aria-hidden', 'true'); svg.style.color = 'var(--acento)';
      const caminho = document.createElementNS(SVG, 'path'); caminho.setAttribute('d', IC_METODO[m.k]); svg.append(caminho);
      const caixa = document.createElement('div');
      const titulo = document.createElement('b'); titulo.textContent = m.t;
      const sub = document.createElement('span'); sub.textContent = m.s;
      caixa.append(titulo, sub); linha.append(svg, caixa);
      return linha;
    }));
  }

  // preço
  const mensal = estado.periodo === 'mensal';
  ligar('periodo', mensal ? '/mês' : '/trimestre');
  for (const b of $$('[data-periodo]')) { const on = b.dataset.periodo === estado.periodo; classe(b, 'on', on); atributo(b, 'aria-pressed', on); }
  const pol = $('[data-seg-pol]');
  if (pol) pol.style.transform = `translateX(${mensal ? 0 : 140}px)`;
}

// ---------- ações ----------
function aoClicar(ev) {
  const alvo = sel => ev.target.closest(sel);
  let el;
  if ((el = alvo('[data-tema]'))) return alternarTema();
  if ((el = alvo('[data-bs-seta]'))) { const bs = el.closest('[data-bs]'); estado.menu = estado.menu === bs ? null : bs; return desenhar(); }
  if ((el = alvo('[data-bs-principal]'))) return baixar(estado.plat);
  if ((el = alvo('[data-plat]'))) { Object.assign(estado, { plat: el.dataset.plat, menu: null, baixou: '' }); return desenhar(); }
  if (alvo('[data-veu-menu]')) { estado.menu = null; return desenhar(); }
  if ((el = alvo('[data-baixar]'))) return baixar(el.dataset.baixar);
  if ((el = alvo('[data-aba]'))) { estado.aba = el.dataset.aba; return desenhar(); }
  if ((el = alvo('[data-ir]'))) { estado.aba = el.dataset.ir; return desenhar(); }
  if (alvo('[data-aprovar]')) { Object.assign(estado, { aba: 'aovivo', passo: 0, vagaIdx: 0 }); return desenhar(); }
  if (alvo('[data-triagem]')) { estado.triIdx = (estado.triIdx + 1) % DECK.length; desenhar(); return repetir($('[data-vaga-cartao]')); }
  if ((el = alvo('[data-chave]'))) { const i = Number(el.dataset.chave); estado.fora[i] = !estado.fora[i]; return desenhar(); }
  if (alvo('[data-pip-fixar]')) { estado.pipFixo = !estado.pipFixo; return desenhar(); }
  if (alvo('[data-deslize]')) { estado.aprovado = !estado.aprovado; return desenhar(); }
  if ((el = alvo('[data-prov]'))) { if (estado.prov !== el.dataset.prov) { estado.prov = el.dataset.prov; desenhar(); repetir($('[data-term]')); } return; }
  if ((el = alvo('[data-periodo]'))) { estado.periodo = el.dataset.periodo; return desenhar(); }
  if ((el = alvo('[data-faq]'))) {
    const abrir = el.getAttribute('aria-expanded') !== 'true';
    for (const b of $$('[data-faq]')) {
      const on = abrir && b === el;
      b.setAttribute('aria-expanded', String(on)); b.classList.toggle('aberto', on);
      document.getElementById(b.getAttribute('aria-controls')).hidden = !on;
    }
  }
}

function iniciar() {
  estado.plat = sistemaDaPessoa();
  document.addEventListener('click', aoClicar);
  document.addEventListener('keydown', ev => { if (ev.key === 'Escape' && estado.menu) { const seta = $('[data-bs-seta]', estado.menu); estado.menu = null; desenhar(); seta?.focus(); } });
  desenhar();
  // A demonstração do envio anda sozinha. Com "reduzir movimento" ligado, ela fica parada num passo que mostra o formulário.
  if (matchMedia('(prefers-reduced-motion: reduce)').matches) { estado.passo = 4; return desenhar(); }
  setInterval(() => {
    if (document.visibilityState !== 'visible') return;
    estado.passo = (estado.passo + 1) % PASSOS.length;
    if (estado.passo === 0) estado.vagaIdx = (estado.vagaIdx + 1) % VAGAS.length;
    desenhar();
  }, 1100);
}
iniciar();
