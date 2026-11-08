import { h, dataCurta } from '../util.mjs';

export function telaRodadas(app) {
  const { rodadas, vagas } = app.estado;
  // Também mostra rodadas que só existem nos envios (histórico migrado).
  const porN = new Map();
  for (const v of vagas.filter(v => v.envio?.rodada != null)) {
    const n = v.envio.rodada;
    const g = porN.get(n) ?? { n, data: v.envio.data, enviadas: 0, plataformas: new Set() };
    g.enviadas++;
    g.plataformas.add(v.plataforma);
    if (v.envio.data < g.data) g.data = v.envio.data;
    porN.set(n, g);
  }
  for (const r of rodadas) porN.set(r.n, { ...porN.get(r.n), ...r, plataformas: new Set([...(porN.get(r.n)?.plataformas ?? []), ...(r.fontes ?? [])]) });
  const lista = [...porN.values()].sort((a, b) => b.n - a.n);

  return h('div',
    h('header.cabecalho', h('div', h('h1', 'Rodadas'), h('p', 'Cada vez que o Claude busca e aplica, é uma rodada. Peça uma nova com /rodada.')),
      h('a.botao-fantasma', { href: '/api/v1/historico.md', target: '_blank' }, 'Ver histórico em texto')),
    lista.length ? h('div.tabela-rolagem', h('table',
      h('thead', h('tr', ['Rodada', 'Data', 'Fontes', 'Encontradas', 'Enviadas', 'Puladas', 'Travadas', 'Resumo'].map(t => h('th', { scope: 'col' }, t)))),
      h('tbody', lista.map(r => h('tr', { style: { cursor: 'default' } },
        h('td.num', String(r.n)), h('td', dataCurta(r.data)), h('td', [...r.plataformas].join(', ')),
        h('td.num', r.encontradas ?? '–'), h('td.num', String(r.enviadas ?? 0)), h('td.num', r.puladas ?? '–'), h('td.num', r.bloqueadas ?? '–'),
        h('td', r.resumo ?? '')))))) :
      h('div.convite', h('div', h('strong', 'Nenhuma rodada ainda'), h('p', { style: { margin: 0 } }, 'No Claude Code, dentro desta pasta, digite /rodada.'))));
}
