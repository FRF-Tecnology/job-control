import { h, dataCurta } from '../util.mjs';
import { porPlataforma } from '../../nucleo/funil.mjs';

const STATUS = { automatizada: 'Automatizada', manual: 'Manual', 'a-testar': 'A testar', descartada: 'Descartada' };

export function telaPlataformas(app) {
  const { plataformas, vagas } = app.estado;
  const nums = new Map(porPlataforma(vagas).map(p => [p.plataforma, p]));
  const ordem = Object.keys(STATUS);
  const lista = [...plataformas].sort((a, b) => ordem.indexOf(a.status) - ordem.indexOf(b.status) || String(a.nome ?? a.id).localeCompare(String(b.nome ?? b.id)));

  const salvar = (p, patch, msg) => app.agir(() => app.api(`/api/v1/plataformas/${encodeURIComponent(p.id)}`, 'PUT', patch), msg);

  return h('div',
    h('header.cabecalho',
      h('div', h('h1', 'Plataformas'), h('p', 'Onde procurar vagas e como cada uma funciona. Teste uma fonte nova por rodada e anote aqui o que descobrir.')),
      h('button.botao-fantasma', { type: 'button', on: { click: async () => {
        const nome = await app.perguntar({ titulo: 'Nova plataforma', campo: 'Nome (ex.: Remotar)' });
        if (!nome) return;
        const url = await app.perguntar({ titulo: `Endereço de ${nome}`, campo: 'https://…' });
        const id = nome.toLowerCase().normalize('NFD').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
        salvar({ id }, { nome, url: url ?? '', status: 'a-testar', comoAplicar: '', limitacoes: '' }, 'Plataforma adicionada');
      } } }, 'Adicionar plataforma')),
    plataformas.length ? h('div.plataformas', lista.map(p => {
      const n = nums.get(p.id);
      return h(`article.plataforma.s-${p.status}`,
        h('div.acoes', { style: { justifyContent: 'space-between', alignItems: 'baseline' } },
          h('h2', /^https?:\/\//.test(p.url ?? '') ? h('a', { href: p.url, target: '_blank', rel: 'noopener' }, p.nome ?? p.id) : (p.nome ?? p.id)),
          h('select', { 'aria-label': `Situação de ${p.nome}`, style: { width: 'auto', minHeight: '30px', padding: '2px 8px' }, on: { change: ev => salvar(p, { status: ev.target.value }, `${p.nome}: ${STATUS[ev.target.value]}`) } },
            Object.entries(STATUS).map(([k, t]) => h('option', { value: k, selected: p.status === k }, t)))),
        n ? h('p.num', { style: { margin: 0 } }, `${n.achadas} achadas · ${n.enviadas} enviadas · ${n.retornos} retornos`) : h('p.pequeno.mudo', { style: { margin: 0 } }, 'Nenhuma vaga daqui ainda.'),
        p.ultimaVarredura && h('p.pequeno.mudo', { style: { margin: 0 } }, `Última busca: ${dataCurta(p.ultimaVarredura)}`),
        campoTexto(p, 'comoAplicar', 'Como aplicar', salvar),
        campoTexto(p, 'limitacoes', 'Limitações', salvar));
    })) : h('div.convite', h('div', h('strong', 'Nenhuma plataforma cadastrada'), h('p', { style: { margin: 0 } }, 'Copie modelo-dados/plataformas.json para dados/ ou adicione uma aqui.'))));
}

function campoTexto(p, k, rotulo, salvar) {
  const t = h('textarea', { id: `pl-${p.id}-${k}`, style: { minHeight: '58px' }, placeholder: k === 'comoAplicar' ? 'Passo a passo, filtros que funcionam…' : 'Pede conta? CPF? Captcha?' });
  t.value = p[k] ?? '';
  t.addEventListener('blur', () => { if (t.value !== (p[k] ?? '')) salvar(p, { [k]: t.value }, 'Salvo'); });
  return h('label.campo', h('span', rotulo), t);
}
