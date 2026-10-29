import { h, dataCurta, nomePlataforma, plural } from '../util.mjs';
import { MOTIVOS_PULO } from '../../nucleo/modelo.mjs';

// Fila: grupo A primeiro, depois B, C e sem grupo; dentro do grupo, as mais recentes.
const ordem = v => ({ A: 0, B: 1, C: 2 }[v.aderencia] ?? 3);
let puladasNestaSessao = new Set();
let ouvinte = null;
let ocupado = false; // evita agir duas vezes na mesma vaga se a pessoa apertar teclas rápido

export function telaTriagem(app) {
  const fila = app.estado.vagas
    .filter(v => v.etapa === 'descoberta' && !puladasNestaSessao.has(v.id))
    .sort((a, b) => ordem(a) - ordem(b) || (b.descobertaEm ?? '').localeCompare(a.descobertaEm ?? ''));
  const total = app.estado.vagas.filter(v => v.etapa === 'descoberta').length;
  const feitas = app.estado.vagas.filter(v => v.etapa !== 'descoberta' && app.estado.eventos.some(e => e.vagaId === v.id && e.autor === 'eu' && (e.tipo === 'aplicar' || e.tipo === 'pulada') && e.data === app.hoje)).length;
  const v = fila[0];
  const grupoA = fila.filter(x => x.aderencia === 'A');

  if (ouvinte) document.removeEventListener('keydown', ouvinte);
  ouvinte = null;

  const cab = h('header.cabecalho',
    h('div', h('h1', 'Triagem'), h('p', total ? `${plural(total, 'vaga espera', 'vagas esperam')} decisão. Hoje você já decidiu ${feitas}.` : 'Nenhuma vaga esperando decisão.')),
    grupoA.length > 1 && h('button.botao-fantasma', { type: 'button', on: { click: async () => {
      const ok = await app.perguntar({ titulo: `Aprovar as ${grupoA.length} vagas do grupo A?`, texto: 'Elas vão para a fila do Claude aplicar.', confirmar: 'Aprovar todas' });
      if (ok !== null) for (const x of grupoA) await app.mover(x.id, 'aplicar', {});
    } } }, `Aprovar todo o grupo A (${grupoA.length})`));

  if (!v) {
    return h('div.triagem', cab, h('div.convite', h('div', h('strong', 'Tudo decidido'),
      h('p', { style: { margin: '4px 0 0' } }, puladasNestaSessao.size ? `${plural(puladasNestaSessao.size, 'vaga ficou', 'vagas ficaram')} para depois.` : 'Peça ao Claude novas vagas com /buscar.')),
      puladasNestaSessao.size ? h('button.botao-fantasma', { type: 'button', on: { click: () => { puladasNestaSessao = new Set(); app.renderizar(); } } }, 'Rever as adiadas') : h('a.botao', { href: '#/quadro' }, 'Ver o quadro')));
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

  const chips = [v.nivel, v.contrato, v.modelo, v.idioma === 'en' ? 'Vaga em inglês' : null, ...(v.stack ?? [])].filter(Boolean);
  const feitosPct = total ? Math.round(((total - fila.length) / total) * 100) : 0;

  return h('div.triagem', cab,
    h('div.progresso', { role: 'progressbar', 'aria-valuenow': feitosPct, 'aria-valuemin': 0, 'aria-valuemax': 100, 'aria-label': 'Progresso da triagem' }, h('i', { style: { width: `${feitosPct}%` } })),
    h('article.ficha', { 'aria-live': 'polite' },
      h('div.acoes', { style: { justifyContent: 'space-between', alignItems: 'center' } },
        h('span.empresa', v.empresa),
        h('span.acoes', v.aderencia && h(`span.chip.ader-${v.aderencia}`, `Grupo ${v.aderencia}`), h('span.chip', nomePlataforma(v.plataforma, app.estado.plataformas)), h('span.chip', `encontrada ${dataCurta(v.descobertaEm)}`))),
      h('h2', v.titulo),
      chips.length > 0 && h('div.acoes', chips.map(c => h('span.chip', c))),
      v.motivoClaude && h('div.motivo', h('b', 'O Claude anotou'), v.motivoClaude),
      v.notas && h('div.motivo', h('b', 'Notas'), v.notas),
      h('div.acoes',
        v.link && h('a.botao-fantasma.p', { href: v.link, target: '_blank', rel: 'noopener' }, 'Ler a vaga completa'),
        h('button.botao-fantasma.p', { type: 'button', on: { click: () => app.abrirVaga(v.id) } }, 'Detalhes')),
      h('div.decisao',
        h('button.botao', { type: 'button', on: { click: aplicar } }, h('kbd', 'A'), 'Aplicar'),
        h('button.botao-fantasma', { type: 'button', on: { click: pular } }, h('kbd', 'P'), 'Pular'),
        h('button.botao-fantasma', { type: 'button', on: { click: depois } }, 'Depois', h('kbd', '→')))),
    h('p.pequeno.mudo', { style: { textAlign: 'center', marginTop: '14px' } }, `${fila.length - 1 > 0 ? `Depois desta, mais ${fila.length - 1}.` : 'É a última.'} Aprovadas vão para a coluna Aplicar; o Claude envia na próxima rodada.`));
}
