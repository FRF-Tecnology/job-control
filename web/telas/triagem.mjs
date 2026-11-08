// Triagem: uma vaga por vez. Arraste o cartão (direita aplica, esquerda pula) ou use os botões / teclas A, P, →.
import { h, dataCurta, nomePlataforma, plural } from '../util.mjs';
import { MOTIVOS_PULO, mercadoDe, noFoco } from '../../nucleo/modelo.mjs';

// Fila: grupo A primeiro, depois B, C e sem grupo; dentro do grupo, as mais recentes.
const ordem = v => ({ A: 0, B: 1, C: 2 }[v.aderencia] ?? 3);
const LIMIAR = 110; // px arrastados para decidir
let puladasNestaSessao = new Set();
let ouvinte = null;
let ocupado = false; // evita agir duas vezes na mesma vaga se a pessoa apertar teclas rápido

const svg = d => {
  const el = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  el.setAttribute('viewBox', '0 0 24 24');
  el.setAttribute('aria-hidden', 'true');
  el.innerHTML = d;
  return el;
};

// Arrastar com dedo ou mouse. Devolve a decisão pelo lado em que o cartão foi solto.
function arrastavel(ficha, { aoAplicar, aoPular }) {
  let inicio = null;
  let dx = 0;
  const selos = () => [ficha.querySelector('.selo-gesto.aplicar'), ficha.querySelector('.selo-gesto.pular')];
  const mover = x => {
    ficha.style.transform = `translateX(${x}px) rotate(${x / 18}deg)`;
    const [sim, nao] = selos();
    sim.style.opacity = String(Math.max(0, Math.min(1, x / LIMIAR)));
    nao.style.opacity = String(Math.max(0, Math.min(1, -x / LIMIAR)));
  };
  const voltar = () => { ficha.classList.remove('arrastando'); mover(0); ficha.style.transform = ''; };
  ficha.addEventListener('pointerdown', ev => {
    if (ev.button !== 0 || ev.target.closest('a, button')) return;
    inicio = ev.clientX;
    dx = 0;
    ficha.setPointerCapture(ev.pointerId);
    ficha.classList.add('arrastando');
  });
  ficha.addEventListener('pointermove', ev => { if (inicio != null) { dx = ev.clientX - inicio; mover(dx); } });
  const soltar = async () => {
    if (inicio == null) return;
    inicio = null;
    ficha.classList.remove('arrastando');
    if (dx > LIMIAR) { ficha.style.transform = 'translateX(120vw) rotate(24deg)'; ficha.style.opacity = '0'; await aoAplicar(); }
    else if (dx < -LIMIAR) { voltar(); await aoPular(); }
    else voltar();
  };
  ficha.addEventListener('pointerup', soltar);
  ficha.addEventListener('pointercancel', () => { inicio = null; voltar(); });
}

export function telaTriagem(app) {
  const foco = app.foco;
  const todas = app.estado.vagas.filter(v => v.etapa === 'descoberta');
  const fila = todas
    .filter(v => !puladasNestaSessao.has(v.id) && noFoco(v, foco))
    .sort((a, b) => ordem(a) - ordem(b) || (b.descobertaEm ?? '').localeCompare(a.descobertaEm ?? ''));
  const foraDoFoco = todas.filter(v => !noFoco(v, foco)).length;
  const feitas = app.estado.vagas.filter(v => v.etapa !== 'descoberta' && app.estado.eventos.some(e => e.vagaId === v.id && e.autor === 'eu' && (e.tipo === 'aplicar' || e.tipo === 'pulada') && e.data === app.hoje)).length;
  const v = fila[0];
  const grupoA = fila.filter(x => x.aderencia === 'A');

  if (ouvinte) document.removeEventListener('keydown', ouvinte);
  ouvinte = null;

  const cab = h('header.cabecalho',
    h('div', h('h1', 'Triagem'), h('p', todas.length
      ? `${plural(fila.length, 'vaga espera', 'vagas esperam')} decisão${foco !== 'ambos' ? ` no foco ${foco}` : ''}. Hoje você já decidiu ${feitas}.`
      : 'Nenhuma vaga esperando decisão.')),
    grupoA.length > 1 && h('button.botao-fantasma', { type: 'button', on: { click: async () => {
      const ok = await app.perguntar({ titulo: `Aprovar as ${grupoA.length} vagas do grupo A?`, texto: 'Elas vão para a fila do Claude aplicar.', confirmar: 'Aprovar todas' });
      if (ok !== null) for (const x of grupoA) await app.mover(x.id, 'aplicar', {});
    } } }, `Aprovar todo o grupo A (${grupoA.length})`));

  if (!v) {
    const texto = puladasNestaSessao.size ? `${plural(puladasNestaSessao.size, 'vaga ficou', 'vagas ficaram')} para depois.`
      : foraDoFoco ? `Há ${plural(foraDoFoco, 'vaga', 'vagas')} fora do foco ${foco}.` : 'Peça ao Claude novas vagas com /buscar.';
    return h('div.triagem', cab, h('div.convite', h('div', h('strong', 'Tudo decidido'), h('p', { style: { margin: '4px 0 0' } }, texto)),
      puladasNestaSessao.size ? h('button.botao-fantasma', { type: 'button', on: { click: () => { puladasNestaSessao = new Set(); app.renderizar(); } } }, 'Rever as adiadas')
        : foraDoFoco ? h('button.botao', { type: 'button', on: { click: () => app.mudarFoco('ambos') } }, 'Ver todas')
          : h('a.botao', { href: '#/quadro' }, 'Ver o quadro')));
  }

  const umaVez = fn => async () => { if (ocupado) return; ocupado = true; try { await fn(); } finally { ocupado = false; } };
  const aplicar = umaVez(() => app.mover(v.id, 'aplicar', {}, `${v.empresa}: vai para a fila do Claude`));
  const pular = umaVez(async () => {
    const motivo = await app.perguntar({ titulo: 'Por que pular?', texto: `${v.empresa} — ${v.titulo}`, opcoes: MOTIVOS_PULO });
    if (motivo) await app.mover(v.id, 'pulada', { motivo }, 'Pulada');
  });
  const depois = () => { puladasNestaSessao.add(v.id); app.renderizar(); };

  ouvinte = ev => {
    if (location.hash !== '#/triagem' || document.getElementById('dialogo').open || ev.target.matches('input,textarea,select') || ev.ctrlKey || ev.metaKey || ev.altKey) return;
    const k = ev.key.toLowerCase();
    if (k === 'a') { ev.preventDefault(); aplicar(); }
    else if (k === 'p') { ev.preventDefault(); pular(); }
    else if (k === 'arrowright' || k === 'd') { ev.preventDefault(); depois(); }
  };
  document.addEventListener('keydown', ouvinte);

  const mercado = mercadoDe(v);
  const chips = [v.nivel, v.contrato, v.modelo, v.idioma === 'en' ? 'Vaga em inglês' : null, ...(v.stack ?? [])].filter(Boolean);
  const feitosPct = todas.length ? Math.round(((todas.length - fila.length) / todas.length) * 100) : 0;

  const ficha = h('article.ficha', { 'aria-live': 'polite', 'aria-roledescription': 'cartão de vaga' },
    h('span.selo-gesto.aplicar', { 'aria-hidden': 'true' }, 'Aplicar'),
    h('span.selo-gesto.pular', { 'aria-hidden': 'true' }, 'Pular'),
    h('div.acoes', { style: { justifyContent: 'space-between', alignItems: 'center' } },
      h('span.acoes',
        v.aderencia && h(`span.chip.ader-${v.aderencia}`, `Aderência ${v.aderencia}`),
        h(`span.chip.m-${mercado}`, mercado === 'internacional' ? 'Internacional' : 'Nacional')),
      h('span.chip', `encontrada ${dataCurta(v.descobertaEm)}`)),
    h('div', h('div.empresa', v.empresa), h('h2', { style: { marginTop: '6px' } }, v.titulo)),
    h('div.acoes', h('span.chip.contorno', nomePlataforma(v.plataforma, app.estado.plataformas)), chips.map(c => h('span.chip.contorno', c))),
    v.motivoClaude && h('div.motivo', h('b', svg('<path d="M12 3l1.8 4.5L18 9l-4.2 1.5L12 15l-1.8-4.5L6 9l4.2-1.5z"/>'), 'Por que aparece'), v.motivoClaude),
    v.notas && h('div.motivo', h('b', 'Notas'), v.notas),
    h('div.acoes',
      v.link && h('a.botao-fantasma.p', { href: v.link, target: '_blank', rel: 'noopener' }, 'Ler a vaga completa'),
      h('button.botao-fantasma.p', { type: 'button', on: { click: depois } }, 'Decidir depois')),
    h('p.dica-gesto', { 'aria-hidden': 'true' }, svg('<path d="M5 12h14M15 8l4 4-4 4"/>'), 'Arraste para a direita para aplicar, para a esquerda para pular'));
  arrastavel(ficha, { aoAplicar: aplicar, aoPular: pular });

  return h('div.triagem', cab,
    h('div.progresso', { role: 'progressbar', 'aria-valuenow': feitosPct, 'aria-valuemin': 0, 'aria-valuemax': 100, 'aria-label': 'Progresso da triagem' }, h('i', { style: { '--p': String(feitosPct / 100) } })),
    h('div.pilha', ficha),
    h('div.decisao',
      h('button.redondo', { type: 'button', 'aria-label': 'Pular vaga (tecla P)', on: { click: pular } }, svg('<path d="M7 7l10 10M17 7L7 17"/>')),
      h('button.redondo.menor', { type: 'button', 'aria-label': 'Ver detalhes da vaga', on: { click: () => app.abrirVaga(v.id) } }, svg('<circle cx="12" cy="12" r="8"/><path d="M12 11v5M12 8v.01"/>')),
      h('button.redondo.sim', { type: 'button', 'aria-label': 'Aplicar na vaga (tecla A)', on: { click: aplicar } }, svg('<path d="M6 12.5l4 4L18 8"/>'))),
    h('div.legenda-decisao', { 'aria-hidden': 'true' }, h('span', 'Pular · P'), h('span', 'Detalhes'), h('span', 'Aplicar · A')),
    h('p.pequeno.mudo', { style: { textAlign: 'center', marginTop: '18px' } }, `${fila.length - 1 > 0 ? `Depois desta, mais ${fila.length - 1}.` : 'É a última.'} Aprovadas vão para a fila; o Claude envia na próxima rodada.`));
}
