import { h, dataCurta, nomePlataforma } from '../util.mjs';
import { resumo, funil, porPlataforma, porSemana, motivosPulo } from '../../nucleo/funil.mjs';

function barras(linhas, cor) {
  const max = Math.max(1, ...linhas.map(l => l.valor));
  return h('div.barras', linhas.map(l => h('div.barra',
    h('span', { title: l.rotulo }, l.rotulo),
    h('span.trilho', { role: 'img', 'aria-label': `${l.rotulo}: ${l.valor}` }, h('i', { style: { width: `${(l.valor / max) * 100}%`, background: l.cor ?? cor } })),
    h('span.v', l.texto ?? String(l.valor)))));
}

export function telaNumeros(app) {
  const { vagas, plataformas } = app.estado;
  const r = resumo(vagas, app.hoje);
  const semanas = porSemana(vagas).slice(-10);
  const maxSemana = Math.max(1, ...semanas.map(s => s.total));
  const plats = porPlataforma(vagas).filter(p => p.enviadas > 0);
  const aderencias = ['A', 'B', 'C'].map(g => {
    const env = vagas.filter(v => v.aderencia === g && v.envio?.data);
    const ret = env.filter(v => ['em-processo', 'oferta'].includes(v.etapa) || (v.etapa === 'encerrada' && v.motivo === 'Reprovada'));
    return { g, env: env.length, ret: ret.length };
  });

  return h('div',
    h('header.cabecalho', h('div', h('h1', 'Números'),
      h('p', `${r.enviadas} enviadas, ${r.retornos} com retorno (${r.taxaRetorno}%). Retorno conta quem avançou no processo ou reprovou: a empresa olhou.`))),
    h('div.paineis',
      h('section.painel', h('h2', 'Funil'), barras(funil(vagas).map((f, i) => ({ rotulo: f.etapa, valor: f.total, cor: ['var(--e-enviada)', 'var(--e-descoberta)', 'var(--e-em-processo)', 'var(--e-oferta)'][i] })))),
      h('section.painel', h('h2', 'Envios por semana'),
        semanas.length ? h('div.colunas-semana', semanas.map(s => h('div', h('span.num', String(s.total)), h('i', { style: { height: `${(s.total / maxSemana) * 100}%` }, title: `${s.total} na semana de ${dataCurta(s.semana)}` }), h('span', dataCurta(s.semana))))) : h('p.mudo', 'Sem envios ainda.')),
      h('section.painel', h('h2', 'Retorno por plataforma'),
        plats.length ? barras(plats.map(p => ({ rotulo: nomePlataforma(p.plataforma, plataformas), valor: p.enviadas, texto: `${p.retornos}/${p.enviadas}` }))) : h('p.mudo', 'Sem envios ainda.'),
        h('p.pequeno.mudo', 'Barra = enviadas. Número = com retorno / enviadas.')),
      h('section.painel', h('h2', 'Retorno por aderência'),
        barras(aderencias.map(a => ({ rotulo: `Grupo ${a.g}`, valor: a.env, texto: a.env ? `${Math.round((a.ret / a.env) * 100)}%` : '–' }))),
        h('p.pequeno.mudo', 'Mostra se as vagas de menor aderência valem o esforço.')),
      h('section.painel', h('h2', 'Por que pulamos'),
        motivosPulo(vagas).length ? barras(motivosPulo(vagas).map(m => ({ rotulo: m.motivo, valor: m.total })), 'var(--e-pulada)') : h('p.mudo', 'Nenhuma vaga pulada.'),
        h('p.pequeno.mudo', 'Se um motivo domina, talvez valha ajustar o perfil (ex.: inglês) ou os filtros de busca.'))));
}
