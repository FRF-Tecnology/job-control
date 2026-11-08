import { h, dataCurta, etapaChip, nomePlataforma, contem } from '../util.mjs';
import { ETAPAS, NOMES_ETAPA } from '../../nucleo/modelo.mjs';

const POR_PAGINA = 50;
// Filtros ficam guardados entre redesenhos (e entre visitas, no navegador).
const filtros = (() => { try { return JSON.parse(sessionStorage.getItem('filtros-vagas')) ?? {}; } catch { return {}; } })();
let pagina = 0;
const guardar = () => { try { sessionStorage.setItem('filtros-vagas', JSON.stringify(filtros)); } catch {} };

export function telaVagas(app) {
  const { vagas, plataformas } = app.estado;
  const plataformasUsadas = [...new Set(vagas.map(v => v.plataforma))];

  const lista = vagas
    .filter(v => contem(v, filtros.texto ?? '') && (!filtros.etapa || v.etapa === filtros.etapa) && (!filtros.plataforma || v.plataforma === filtros.plataforma) && (!filtros.aderencia || v.aderencia === filtros.aderencia) && (!filtros.contrato || v.contrato === filtros.contrato))
    .sort((a, b) => (b.envio?.data ?? b.descobertaEm ?? '').localeCompare(a.envio?.data ?? a.descobertaEm ?? ''));
  const paginas = Math.max(1, Math.ceil(lista.length / POR_PAGINA));
  pagina = Math.min(pagina, paginas - 1);
  const visiveis = lista.slice(pagina * POR_PAGINA, (pagina + 1) * POR_PAGINA);

  const mudar = (k, val) => { filtros[k] = val; pagina = 0; guardar(); app.renderizar(); };
  const sel = (k, rotulo, opcoes) => h('select', { 'aria-label': rotulo, id: `filtro-${k}`, on: { change: ev => mudar(k, ev.target.value) } },
    h('option', { value: '' }, rotulo), opcoes.map(([v, t]) => h('option', { value: v, selected: filtros[k] === v }, t)));

  const busca = h('input#busca-vagas', { type: 'search', placeholder: 'Buscar empresa, vaga ou nota', value: filtros.texto ?? '', 'aria-label': 'Buscar' });
  let espera;
  busca.addEventListener('input', () => { clearTimeout(espera); espera = setTimeout(() => mudar('texto', busca.value), 200); });

  return h('div',
    h('header.cabecalho',
      h('div', h('h1', 'Vagas'), h('p', `${lista.length} de ${vagas.length}`)),
      h('div.acoes',
        h('button.botao-fantasma', { type: 'button', on: { click: () => novaVaga(app) } }, 'Adicionar vaga'),
        h('a.botao-fantasma', { href: '/api/v1/exportar.csv' }, 'Exportar CSV'))),
    h('div.filtros',
      busca,
      sel('etapa', 'Todas as etapas', ETAPAS.map(e => [e, NOMES_ETAPA[e]])),
      sel('plataforma', 'Todas as plataformas', plataformasUsadas.map(p => [p, nomePlataforma(p, plataformas)])),
      sel('aderencia', 'Qualquer aderência', [['A', 'Grupo A'], ['B', 'Grupo B'], ['C', 'Grupo C']]),
      sel('contrato', 'Qualquer contrato', [['CLT', 'CLT'], ['PJ', 'PJ'], ['Cooperado', 'Cooperado']])),
    lista.length ? h('div.tabela-rolagem', h('table',
      h('thead', h('tr', ['Data', 'Empresa', 'Vaga', 'Etapa', 'Plataforma', 'Grupo', 'Observação'].map(t => h('th', { scope: 'col' }, t)))),
      h('tbody', visiveis.map(v => h('tr', { tabindex: '0', on: { click: () => app.abrirVaga(v.id), keydown: ev => { if (ev.key === 'Enter') app.abrirVaga(v.id); } } },
        h('td.num', dataCurta(v.envio?.data ?? v.descobertaEm)),
        h('td', h('b', v.empresa)),
        h('td', v.titulo),
        h('td', etapaChip(v.etapa)),
        h('td', nomePlataforma(v.plataforma, plataformas)),
        h('td', v.aderencia ? h(`span.chip.ader-${v.aderencia}`, v.aderencia) : ''),
        h('td.corta', { title: v.pendencia || v.motivo || v.notas || v.motivoClaude || '' }, v.pendencia || v.motivo || v.notas || v.motivoClaude || '')))))) :
      h('div.convite', h('div', h('strong', 'Nada encontrado'), h('p', { style: { margin: 0 } }, 'Limpe os filtros ou busque outro termo.')),
        h('button.botao-fantasma', { type: 'button', on: { click: () => { for (const k of Object.keys(filtros)) delete filtros[k]; guardar(); app.renderizar(); } } }, 'Limpar filtros')),
    paginas > 1 && h('div.paginacao',
      h('button.botao-fantasma', { type: 'button', disabled: pagina === 0, on: { click: () => { pagina--; app.renderizar(); } } }, 'Anterior'),
      h('span.mudo', `Página ${pagina + 1} de ${paginas}`),
      h('button.botao-fantasma', { type: 'button', disabled: pagina >= paginas - 1, on: { click: () => { pagina++; app.renderizar(); } } }, 'Próxima')));
}

async function novaVaga(app) {
  const link = await app.perguntar({ titulo: 'Adicionar vaga', texto: 'Cole o link da vaga. Depois preencha empresa e título.', campo: 'https://…' });
  if (!link) return;
  const empresa = await app.perguntar({ titulo: 'Empresa', campo: 'Nome da empresa' });
  if (!empresa) return;
  const titulo = await app.perguntar({ titulo: 'Título da vaga', campo: 'ex.: Desenvolvedor Java Pleno' });
  if (!titulo) return;
  const id = `ext-${empresa.toLowerCase().normalize('NFD').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')}-${Date.now().toString(36)}`;
  let plataforma = 'externo';
  if (/linkedin\.com/.test(link)) plataforma = 'linkedin';
  else if (/gupy\.io/.test(link)) plataforma = 'gupy';
  else if (/inhire\.app/.test(link)) plataforma = 'inhire';
  const ok = await app.agir(() => app.api('/api/v1/vagas', 'POST', { id, empresa, titulo, link, plataforma }), 'Vaga adicionada em Descoberta');
  if (ok) app.abrirVaga(id);
}
