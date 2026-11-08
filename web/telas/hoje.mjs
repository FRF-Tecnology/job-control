// Hoje: uma caixa "O que vamos fazer hoje?", o seletor de foco e só o que pede ação.
import { h, dataLonga, haQuanto, dataCurta, nomeEtapa, NOMES_SUB, plural } from '../util.mjs';
import { pendentesFollowUp, sugerirMensagem } from '../../nucleo/followup.mjs';
import { foiEnviada, normalizar, noFoco } from '../../nucleo/modelo.mjs';

const FOCOS = [['internacional', 'Internacional'], ['nacional', 'Nacional'], ['ambos', 'Os dois']];
const TIPOS_EVENTO = { descoberta: 'Encontrada', aplicar: 'Aprovada para aplicar', enviada: 'Enviada', bloqueada: 'Travou', 'em-processo': 'Em processo', oferta: 'Oferta', encerrada: 'Encerrada', pulada: 'Pulada', 'follow-up': 'Follow-up', nota: 'Nota', retorno: 'Retorno', teste: 'Teste', entrevista: 'Entrevista' };
const FUNIL = [
  ['descoberta', 'Descoberta', 'triagem'], ['aplicar', 'Para aplicar', 'quadro'], ['enviada', 'Enviadas', 'quadro'],
  ['em-processo', 'Em processo', 'quadro'], ['oferta', 'Oferta', 'quadro'],
];

// Pedidos que a própria tela resolve na hora; o resto vira pedido para o assistente (/pedidos).
const ATALHOS = [
  [/\btri(ar|agem)\b|decidir|vagas novas/, '#/triagem'],
  [/respond|retorno|entrevista|processo|oferta/, '#/quadro'],
  [/n[uú]meros|taxa|funil|semana|m[eé]tric/, '#/numeros'],
  [/perfil|pretens[aã]o|curr[ií]culo/, '#/perfil'],
  [/plataforma|fonte/, '#/plataformas'],
];

const svg = d => {
  const el = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  el.setAttribute('viewBox', '0 0 24 24');
  el.setAttribute('aria-hidden', 'true');
  el.innerHTML = d;
  return el;
};

function caixaPedido(app, sugestoes) {
  const entrada = h('input#pedido', { type: 'text', autocomplete: 'off', placeholder: sugestoes[0] ?? 'Peça qualquer coisa', 'aria-label': 'O que vamos fazer hoje?' });
  async function enviar(texto) {
    const t = texto.trim();
    if (!t) return entrada.focus();
    const atalho = ATALHOS.find(([re]) => re.test(normalizar(t)));
    if (atalho && !/\b(busca|aplica|envia|manda|escreve)/.test(normalizar(t))) { location.hash = atalho[1]; return; }
    const ok = await app.agir(() => app.api('/api/v1/pedidos', 'POST', { texto: t }), 'Anotado. No Claude Code, digite /pedidos para ele fazer.');
    if (ok) entrada.value = '';
  }
  return [
    h('form.caixa-pedido', { role: 'search', on: { submit: ev => { ev.preventDefault(); enviar(entrada.value); } } },
      entrada,
      h('button', { type: 'submit', 'aria-label': 'Enviar pedido' }, svg('<path d="M12 19V5M5 12l7-7 7 7"/>'))),
    h('div.sugestoes', { 'aria-label': 'Sugestões' }, sugestoes.map(s => h('button', { type: 'button', on: { click: () => enviar(s) } }, s))),
  ];
}

function sugestoesPara(foco, nAplicar, nDescoberta) {
  const buscar = { internacional: 'Buscar vagas USD LATAM', nacional: 'Buscar vagas CLT/PJ remotas', ambos: 'Buscar vagas novas' }[foco];
  const qual = { internacional: ' internacionais', nacional: ' nacionais', ambos: '' }[foco];
  return [
    nAplicar ? `Aplicar nas ${nAplicar}${qual} aprovadas` : buscar,
    nAplicar ? buscar : null,
    nDescoberta ? `Triar ${plural(nDescoberta, 'vaga nova', 'vagas novas')}` : null,
    'Quem me respondeu?',
  ].filter(Boolean);
}

// Título curto da pendência: primeira frase, sem links. Tudo que pede CPF vira um grupo só.
export function resumoPendencia(texto) {
  const limpo = String(texto ?? '').replace(/https?:\/\/\S+/g, '').replace(/\s+/g, ' ').trim();
  if (/\bcpf\b/i.test(limpo)) return 'Digitar o CPF (o resto do formulário já está pronto)';
  return limpo.split(/(?<=\.)\s| — /)[0].replace(/[.:,;]+$/, '').trim() || 'Sem pendência anotada';
}

// Bloqueadas com a mesma pendência viram uma linha só ("Gupy exige login — 2 vagas").
function agruparPendencias(bloqueadas) {
  const grupos = new Map();
  for (const v of bloqueadas) {
    const texto = resumoPendencia(v.pendencia);
    const k = normalizar(texto);
    if (!grupos.has(k)) grupos.set(k, { texto, vagas: [] });
    grupos.get(k).vagas.push(v);
  }
  return [...grupos.values()].sort((a, b) => b.vagas.length - a.vagas.length);
}

function acoesBloqueada(app, v) {
  return [
    v.link && h('a.botao.p', { href: v.link, target: '_blank', rel: 'noopener' }, 'Abrir vaga'),
    h('button.botao-fantasma.p', { type: 'button', on: { click: () => app.mover(v.id, 'enviada', {}, `${v.empresa}: marcada como enviada`) } }, 'Enviei'),
    h('button.botao-fantasma.p', { type: 'button', on: { click: async () => { const m = await app.perguntar({ titulo: 'Desistir desta vaga?', opcoes: ['Documento', 'Outro'], campo: 'Motivo', confirmar: 'Pular vaga' }); if (m) app.mover(v.id, 'pulada', { motivo: m }, 'Vaga pulada'); } } }, 'Desistir'),
  ];
}

const linkVaga = (app, v) => h('button.link-vaga', { type: 'button', on: { click: () => app.abrirVaga(v.id) } }, `${v.empresa} — ${v.titulo}`);

export function telaHoje(app) {
  const { vagas, eventos, perfil } = app.estado;
  const hoje = app.hoje;
  const foco = app.foco;
  const doFoco = vagas.filter(v => noFoco(v, foco));
  const conta = etapa => doFoco.filter(v => (etapa === 'enviada' ? foiEnviada(v) : v.etapa === etapa)).length;

  const enviadas = vagas.filter(foiEnviada);
  const enviadasHoje = enviadas.filter(v => v.envio.data === hoje).length;
  const inicioSemana = (() => { const d = new Date(`${hoje}T12:00`); d.setDate(d.getDate() - ((d.getDay() + 6) % 7)); return d.toISOString().slice(0, 10); })();
  const naSemana = enviadas.filter(v => v.envio.data >= inicioSemana).length;
  const limite = Number(perfil?.limiteLinkedinDia) || 40;
  const linkedinHoje = enviadas.filter(v => v.plataforma === 'linkedin' && v.envio.data === hoje).length;
  const nAplicar = conta('aplicar');
  const nDescoberta = conta('descoberta');

  const bloqueadas = vagas.filter(v => v.etapa === 'bloqueada');
  const grupos = agruparPendencias(bloqueadas);
  const follow = pendentesFollowUp(vagas, hoje);
  const emProcesso = vagas.filter(v => v.etapa === 'em-processo' || v.etapa === 'oferta');
  const recentes = [...eventos].sort((a, b) => b.data.localeCompare(a.data) || b.id.localeCompare(a.id)).slice(0, 8);
  const porId = new Map(vagas.map(v => [v.id, v]));
  const passos = app.estado.primeirosPassos ?? [];
  const faltam = passos.filter(p => !p.feito).length;
  const pedidos = (app.estado.pedidos ?? []).filter(p => p.status === 'aberto' || p.concluidoEm === hoje).reverse();
  const nome = String(perfil?.nome ?? '').trim().split(/\s+/)[0];
  const maiorFunil = Math.max(1, ...FUNIL.map(([e]) => conta(e)));

  return h('div',
    h('section.ola', { 'aria-labelledby': 'ola-titulo' },
      h('div.ola-topo',
        h('span.data', dataLonga(hoje).replace(/^./, c => c.toUpperCase())),
        h('div.seletor', { role: 'group', 'aria-label': 'Foco da busca' }, FOCOS.map(([id, rotulo]) =>
          h('button', { type: 'button', 'aria-pressed': String(foco === id), on: { click: () => app.mudarFoco(id) } }, rotulo)))),
      h('h1#ola-titulo', nome ? `Olá, ${nome}.` : 'Olá.'),
      h('p.pergunta', 'O que vamos fazer hoje?'),
      caixaPedido(app, sugestoesPara(foco, nAplicar, nDescoberta))),

    h('div.cartoes-num',
      h('div.cartao-num', h('span.n', String(enviadasHoje)), h('span.r', 'enviadas hoje')),
      h('a.cartao-num.escuro', { href: '#/quadro' }, h('span.n', String(nAplicar)), h('span.r', foco === 'ambos' ? 'para aplicar' : `para aplicar · ${foco}`)),
      h('div.cartao-num', h('span.n', String(Math.max(0, limite - linkedinHoje))), h('span.r', 'restantes no LinkedIn hoje')),
      h('div.cartao-num', h('span.n', String(naSemana)), h('span.r', 'enviadas na semana'))),

    faltam > 0 && h('section.primeiros', { 'aria-labelledby': 'primeiros-titulo' },
      h('div', h('h2#primeiros-titulo', 'Primeiros passos'),
        h('p.pequeno', `${passos.length - faltam} de ${passos.length} prontos. Some daqui quando terminar.`)),
      h('ol', passos.map(p => h(`li.${p.feito ? 'feito' : 'falta'}`,
        h('span.marca-passo', { 'aria-hidden': 'true' }, p.feito ? '✓' : ''),
        h('div', h('strong', p.titulo), h('span.estado-passo', p.feito ? ' — pronto' : ' — falta'), !p.feito && h('p.pequeno', p.texto)),
        !p.feito && h('a.botao-fantasma.p', { href: p.rota }, p.botao ?? 'Abrir'))))),

    pedidos.length > 0 && h('section.bloco',
      h('header', h('h2', 'Seus pedidos'), h('span.num', String(pedidos.filter(p => p.status === 'aberto').length))),
      h('ul.lista-cartao', pedidos.map(p => h(`li.pedido${p.status === 'feito' ? '.feito' : ''}`,
        h('strong', p.texto),
        h('span.estado', p.status === 'feito' ? 'Feito' : 'Esperando o Claude'),
        p.resposta ? h('p', p.resposta) : p.status === 'aberto' && h('p', 'No Claude Code, digite /pedidos.'))))),

    h('div.grade-hoje',
      h('div',
        h('section.bloco',
          h('header', h('h2', 'Precisa de você'), h('span.num', String(bloqueadas.length))),
          grupos.length ? h('ul.lista-cartao', grupos.map(g => h('li.item-acao',
            h('span.selo-n.ambar', String(g.vagas.length)),
            h('div', h('strong', g.texto), h('span.sub', g.vagas.length === 1 ? `${g.vagas[0].empresa} — ${g.vagas[0].titulo}` : `${plural(g.vagas.length, 'vaga', 'vagas')}: ${[...new Set(g.vagas.map(v => v.empresa))].slice(0, 3).join(', ')}${new Set(g.vagas.map(v => v.empresa)).size > 3 ? '…' : ''}`)),
            g.vagas.length === 1
              ? h('div.acoes', acoesBloqueada(app, g.vagas[0]))
              : h('details', h('summary', `Ver as ${g.vagas.length} vagas`),
                h('ul.vagas-grupo', g.vagas.map(v => h('li', linkVaga(app, v), h('span.acoes', acoesBloqueada(app, v)))))))))
            : h('p.vazio', 'Nada travado. O Claude consegue seguir sozinho.')),

        h('section.bloco',
          h('header', h('h2', 'Follow-ups para hoje'), h('span.num', String(follow.length))),
          follow.length ? h('ul.linhas', follow.map(v => h('li.linha', linkVaga(app, v),
            h('div.acoes',
              h('button.botao.p', { type: 'button', on: { click: async () => {
                const texto = await app.perguntar({ titulo: `Mensagem para ${v.empresa}`, texto: 'Ajuste se quiser e copie. Envie pelo LinkedIn ou e-mail do recrutador.', campo: 'Mensagem', valor: sugerirMensagem(v, perfil), multilinha: true, confirmar: 'Copiar mensagem' });
                if (texto) { try { await navigator.clipboard.writeText(texto); app.aviso('Mensagem copiada'); } catch { app.aviso('Não consegui copiar; selecione o texto manualmente', true); } }
              } } }, 'Mensagem'),
              h('button.botao-fantasma.p', { type: 'button', on: { click: () => app.agir(() => app.api(`/api/v1/vagas/${encodeURIComponent(v.id)}/followup`, 'POST', { acao: 'feito' }), 'Follow-up registrado') } }, 'Feito'),
              h('button.botao-fantasma.p', { type: 'button', on: { click: () => app.agir(() => app.api(`/api/v1/vagas/${encodeURIComponent(v.id)}/followup`, 'POST', { acao: 'adiar', dias: 3 }), 'Adiado 3 dias') } }, '+3 dias')),
            h('div.sub', `Enviada ${haQuanto(v.envio.data, hoje)} pelo ${v.plataforma}${v.followUp.feitos ? `, ${plural(v.followUp.feitos, 'follow-up feito', 'follow-ups feitos')}` : ''}`))))
            : h('p.vazio', proximoFollowUp(vagas, hoje))),

        h('section.bloco',
          h('header', h('h2', 'Em andamento'), h('span.num', String(emProcesso.length))),
          emProcesso.length ? h('ul.linhas', emProcesso.map(v => h('li.linha', linkVaga(app, v),
            h('div.acoes', v.link && h('a.botao-fantasma.p', { href: v.link, target: '_blank', rel: 'noopener' }, 'Abrir')),
            h('div.sub', [NOMES_SUB[v.subEtapa] ?? nomeEtapa(v.etapa), v.notas].filter(Boolean).join(': ')))))
            : h('p.vazio', 'Nenhum processo seletivo andando ainda. Quando alguém responder, aparece aqui.'))),

      h('div',
        nDescoberta > 0 && h('section.bloco',
          h('div.convite.azul', h('div', h('strong', String(nDescoberta)), h('p', { style: { margin: 0 } }, plural(nDescoberta, 'vaga nova para decidir', 'vagas novas para decidir').replace(/^\d+ /, ''))),
            h('a.botao', { href: '#/triagem' }, 'Triar agora'))),

        h('section.bloco',
          h('header', h('h2', 'Funil'), h('span.num', foco === 'ambos' ? '' : foco)),
          h('nav.funil-barras', { 'aria-label': 'Funil por etapa' }, FUNIL.map(([e, rotulo, rota]) => {
            const n = conta(e);
            return h(`a.e-${e}`, { href: `#/${rota}` }, h('span', rotulo),
              h('span.trilho', { 'aria-hidden': 'true' }, h('i', { style: { '--p': String(n / maiorFunil) } })), h('span.v', String(n)));
          }))),

        h('section.bloco',
          h('header', h('h2', 'Linha do tempo')),
          recentes.length ? h('ul.atividade', recentes.map(ev => {
            const v = porId.get(ev.vagaId);
            return h(`li.e-${ev.tipo}`, h('time', { datetime: ev.data }, `${dataCurta(ev.data)}${ev.autor === 'claude' ? ' · Claude' : ''}`),
              h('span', h('b', TIPOS_EVENTO[ev.tipo] ?? ev.tipo), ' ', v ? `${v.empresa} — ${v.titulo}` : ev.vagaId));
          })) : h('p.vazio', 'Sem atividade ainda.')))),

    h('p.tranquilo', svg('<path d="M12 3l7 3v6c0 4.5-3 7.5-7 9-4-1.5-7-4.5-7-9V6z"/><path d="M9 12l2 2 4-4"/>'),
      'Senhas, CPF e logins ficam sempre com você. A Central nunca guarda documentos e nada é enviado em seu nome sem você aprovar.'));
}

function proximoFollowUp(vagas, hoje) {
  const futuros = vagas.filter(v => v.etapa === 'enviada' && v.followUp?.proximo > hoje).map(v => v.followUp.proximo).sort();
  if (!futuros.length) return vagas.some(foiEnviada) ? 'Nenhum follow-up agendado.' : 'Os follow-ups aparecem 7 dias depois de cada envio.';
  const n = futuros.filter(d => d === futuros[0]).length;
  return `Nada para hoje. Próximos: ${plural(n, 'vaga', 'vagas')} em ${dataCurta(futuros[0])}.`;
}
