import { h, dataCurta, nomeEtapa, nomePlataforma, NOMES_SUB } from '../util.mjs';
import { MOTIVOS_PULO, MOTIVOS_ENCERRAMENTO, SUB_ETAPAS } from '../../nucleo/modelo.mjs';

const COLUNAS = ['bloqueada', 'descoberta', 'aplicar', 'enviada', 'em-processo', 'oferta'];
const LIMITE = 40;
const expandidas = new Set();

// Pede o que cada etapa exige antes de mover (motivo, pendência, sub-etapa).
async function extrasPara(app, etapa) {
  if (etapa === 'pulada' || etapa === 'encerrada') {
    const motivo = await app.perguntar({ titulo: etapa === 'pulada' ? 'Por que pular?' : 'Por que encerrar?', opcoes: etapa === 'pulada' ? MOTIVOS_PULO : MOTIVOS_ENCERRAMENTO });
    return motivo ? { motivo } : null;
  }
  if (etapa === 'bloqueada') {
    const pendencia = await app.perguntar({ titulo: 'O que falta você fazer?', campo: 'ex.: digitar CPF e enviar' });
    return pendencia ? { pendencia } : null;
  }
  if (etapa === 'em-processo') {
    const s = await app.perguntar({ titulo: 'Em que etapa está?', opcoes: SUB_ETAPAS.map(x => NOMES_SUB[x]) });
    return s ? { subEtapa: SUB_ETAPAS.find(x => NOMES_SUB[x] === s) } : null;
  }
  return {};
}

export function telaQuadro(app) {
  const { vagas } = app.estado;
  let arrastada = null;

  const soltar = async (etapa, col) => {
    col.classList.remove('sobre');
    const v = vagas.find(x => x.id === arrastada);
    if (!v || v.etapa === etapa) return;
    const extras = await extrasPara(app, etapa);
    if (extras) app.mover(v.id, etapa, extras, `${v.empresa}: ${nomeEtapa(etapa)}`);
  };

  const cartao = v => h(`article.cartao.e-${v.etapa}`, {
    draggable: 'true', tabindex: '0', 'aria-label': `${v.empresa}, ${v.titulo}. Enter abre o detalhe.`,
    on: {
      dragstart: ev => { arrastada = v.id; ev.dataTransfer.effectAllowed = 'move'; ev.currentTarget.classList.add('arrastando'); },
      dragend: ev => ev.currentTarget.classList.remove('arrastando'),
      click: () => app.abrirVaga(v.id),
      keydown: ev => { if (ev.key === 'Enter') app.abrirVaga(v.id); },
    },
  },
  h('div.m', h('span', v.empresa), v.aderencia && h(`span.chip.ader-${v.aderencia}`, v.aderencia)),
  h('div.t', v.titulo),
  h('div.m',
    h('span', nomePlataforma(v.plataforma, app.estado.plataformas)),
    v.envio?.data ? h('span', `enviada ${dataCurta(v.envio.data)}`) : null,
    v.subEtapa ? h('span', NOMES_SUB[v.subEtapa]) : null,
    v.pendencia ? h('span', { style: { color: 'var(--ambar)' } }, v.pendencia) : null));

  const descarte = (etapa, rotulo) => h('div.coluna', {
    style: { minHeight: '64px', justifyContent: 'center', alignItems: 'center', color: 'var(--tinta-2)' },
    on: { dragover: ev => { ev.preventDefault(); ev.currentTarget.classList.add('sobre'); }, dragleave: ev => ev.currentTarget.classList.remove('sobre'), drop: ev => { ev.preventDefault(); soltar(etapa, ev.currentTarget); } },
  }, rotulo);

  return h('div',
    h('header.cabecalho', h('div', h('h1', 'Quadro'), h('p', 'Arraste os cartões entre as colunas. No celular, abra o cartão e use "Mover para".'))),
    h('div.quadro', COLUNAS.map(etapa => {
      const lista = vagas.filter(v => v.etapa === etapa)
        .sort((a, b) => (b.envio?.data ?? b.atualizadoEm ?? '').localeCompare(a.envio?.data ?? a.atualizadoEm ?? ''));
      const mostrar = expandidas.has(etapa) ? lista : lista.slice(0, LIMITE);
      return h(`section.coluna.e-${etapa}`, {
        'aria-label': nomeEtapa(etapa),
        on: { dragover: ev => { ev.preventDefault(); ev.currentTarget.classList.add('sobre'); }, dragleave: ev => { if (!ev.currentTarget.contains(ev.relatedTarget)) ev.currentTarget.classList.remove('sobre'); }, drop: ev => { ev.preventDefault(); soltar(etapa, ev.currentTarget); } },
      },
      h('header', h('span.etapa', { class: `e-${etapa}` }, nomeEtapa(etapa)), h('span.num.mudo', String(lista.length))),
      mostrar.map(cartao),
      lista.length > mostrar.length && h('button.mais', { type: 'button', on: { click: () => { expandidas.add(etapa); app.renderizar(); } } }, `Ver mais ${lista.length - mostrar.length}`),
      !lista.length && h('p.pequeno.mudo', { style: { padding: '6px' } }, 'Vazio'));
    })),
    h('div.acoes', { style: { marginTop: '12px', display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '14px' } },
      descarte('pulada', `Solte aqui para pular (${vagas.filter(v => v.etapa === 'pulada').length})`),
      descarte('encerrada', `Solte aqui para encerrar (${vagas.filter(v => v.etapa === 'encerrada').length})`)));
}
