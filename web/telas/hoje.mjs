import { h, dataLonga, haQuanto, dataCurta, nomeEtapa, NOMES_SUB, plural } from '../util.mjs';
import { resumo } from '../../nucleo/funil.mjs';
import { pendentesFollowUp, sugerirMensagem } from '../../nucleo/followup.mjs';
import { foiEnviada } from '../../nucleo/modelo.mjs';

const ESTEIRA = [
  { etapa: 'descoberta', rotulo: 'para decidir', rota: 'triagem' },
  { etapa: 'aplicar', rotulo: 'na fila do Claude', rota: 'quadro' },
  { etapa: 'enviada', rotulo: 'aguardando retorno', rota: 'quadro' },
  { etapa: 'em-processo', rotulo: 'em processo', rota: 'quadro' },
  { etapa: 'oferta', rotulo: 'ofertas', rota: 'quadro' },
];

const TIPOS_EVENTO = { descoberta: 'Encontrada', aplicar: 'Aprovada para aplicar', enviada: 'Enviada', bloqueada: 'Travou', 'em-processo': 'Em processo', oferta: 'Oferta', encerrada: 'Encerrada', pulada: 'Pulada', 'follow-up': 'Follow-up', nota: 'Nota', retorno: 'Retorno', teste: 'Teste', entrevista: 'Entrevista' };

function linhaVaga(app, v, sub, acoes) {
  return h('li.linha',
    h('button.titulo', { type: 'button', on: { click: () => app.abrirVaga(v.id) } }, `${v.empresa} — ${v.titulo}`),
    h('div.acoes', acoes),
    h('div.sub', sub));
}

export function telaHoje(app) {
  const { vagas, eventos, perfil } = app.estado;
  const hoje = app.hoje;
  const r = resumo(vagas, hoje);
  const conta = e => vagas.filter(v => v.etapa === e).length;
  const maior = Math.max(1, ...ESTEIRA.map(s => conta(s.etapa)));

  const bloqueadas = vagas.filter(v => v.etapa === 'bloqueada').sort((a, b) => (a.aderencia ?? 'Z').localeCompare(b.aderencia ?? 'Z'));
  const follow = pendentesFollowUp(vagas, hoje);
  const emProcesso = vagas.filter(v => v.etapa === 'em-processo' || v.etapa === 'oferta');
  const recentes = [...eventos].sort((a, b) => b.data.localeCompare(a.data) || b.id.localeCompare(a.id)).slice(0, 10);
  const porId = new Map(vagas.map(v => [v.id, v]));
  const semPerfil = !perfil?.nome;

  return h('div',
    h('header.cabecalho',
      h('div', h('h1', dataLonga(hoje).replace(/^./, c => c.toUpperCase())),
        h('p', `${plural(r.enviadasHoje, 'candidatura enviada', 'candidaturas enviadas')} hoje, ${r.enviadasSemana} na semana. ${r.enviadas} no total, ${r.taxaRetorno}% com retorno.`))),

    semPerfil && h('div.convite', { style: { marginBottom: '24px' } },
      h('div', h('strong', 'Comece pelo seu perfil'), h('p', { style: { margin: '4px 0 0' } }, 'Nome, pretensões, inglês e filtros. O Claude usa esses dados em todas as candidaturas.')),
      h('a.botao', { href: '#/perfil' }, 'Preencher perfil')),

    h('nav.esteira', { 'aria-label': 'Etapas' }, ESTEIRA.map(s => {
      const n = conta(s.etapa);
      return h(`a.e-${s.etapa}`, { href: `#/${s.rota}`, style: { '--peso': String(0.6 + n / maior), borderTopColor: `var(--e-${s.etapa})` } },
        h('span.n', String(n)), h('span.r', s.rotulo));
    })),

    h('div.grade-hoje',
      h('div',
        h('section.bloco.alerta',
          h('header', h('h2', 'Precisa de você'), h('span.num', String(bloqueadas.length))),
          bloqueadas.length ? h('ul.linhas', bloqueadas.map(v => linhaVaga(app, v,
            v.pendencia,
            [v.link && h('a.botao.p', { href: v.link, target: '_blank', rel: 'noopener' }, 'Abrir vaga'),
              h('button.botao-fantasma.p', { type: 'button', on: { click: () => app.mover(v.id, 'enviada', {}, `${v.empresa}: marcada como enviada`) } }, 'Enviei'),
              h('button.botao-fantasma.p', { type: 'button', on: { click: async () => { const m = await app.perguntar({ titulo: 'Desistir desta vaga?', opcoes: ['Documento', 'Outro'], campo: 'Motivo', confirmar: 'Pular vaga' }); if (m) app.mover(v.id, 'pulada', { motivo: m }, 'Vaga pulada'); } } }, 'Desistir')])))
            : h('p.vazio', 'Nada travado. O Claude consegue seguir sozinho.')),

        h('section.bloco',
          h('header', h('h2', 'Follow-ups para hoje'), h('span.num', String(follow.length))),
          follow.length ? h('ul.linhas', follow.map(v => linhaVaga(app, v,
            `Enviada ${haQuanto(v.envio.data, hoje)} pelo ${v.plataforma}${v.followUp.feitos ? `, ${plural(v.followUp.feitos, 'follow-up feito', 'follow-ups feitos')}` : ''}`,
            [h('button.botao.p', { type: 'button', on: { click: async () => {
              const texto = await app.perguntar({ titulo: `Mensagem para ${v.empresa}`, texto: 'Ajuste se quiser e copie. Envie pelo LinkedIn ou e-mail do recrutador.', campo: 'Mensagem', valor: sugerirMensagem(v, perfil), multilinha: true, confirmar: 'Copiar mensagem' });
              if (texto) { try { await navigator.clipboard.writeText(texto); app.aviso('Mensagem copiada'); } catch { app.aviso('Não consegui copiar; selecione o texto manualmente', true); } }
            } } }, 'Mensagem'),
            h('button.botao-fantasma.p', { type: 'button', on: { click: () => app.agir(() => app.api(`/api/vagas/${encodeURIComponent(v.id)}/followup`, 'POST', { acao: 'feito' }), 'Follow-up registrado') } }, 'Feito'),
            h('button.botao-fantasma.p', { type: 'button', on: { click: () => app.agir(() => app.api(`/api/vagas/${encodeURIComponent(v.id)}/followup`, 'POST', { acao: 'adiar', dias: 3 }), 'Adiado 3 dias') } }, '+3 dias')])))
            : h('p.vazio', proximoFollowUp(vagas, hoje))),

        h('section.bloco',
          h('header', h('h2', 'Em andamento'), h('span.num', String(emProcesso.length))),
          emProcesso.length ? h('ul.linhas', emProcesso.map(v => linhaVaga(app, v,
            [NOMES_SUB[v.subEtapa] ?? nomeEtapa(v.etapa), v.notas].filter(Boolean).join(': '),
            [v.link && h('a.botao-fantasma.p', { href: v.link, target: '_blank', rel: 'noopener' }, 'Abrir')])))
            : h('p.vazio', 'Nenhum processo seletivo andando ainda.'))),

      h('div',
        h('section.bloco',
          h('header', h('h2', 'Triagem')),
          r.descoberta
            ? h('div.convite', h('div', h('strong', String(r.descoberta)), h('p', { style: { margin: 0 } }, 'vagas esperando sua decisão')), h('a.botao', { href: '#/triagem' }, 'Começar triagem'))
            : h('p.vazio', 'Nenhuma vaga nova. Peça ao Claude: /buscar'),
          r.aplicar > 0 && h('p.pequeno.mudo', `${plural(r.aplicar, 'vaga aprovada espera', 'vagas aprovadas esperam')} o Claude aplicar (/aplicar).`)),

        h('section.bloco',
          h('header', h('h2', 'Atividade recente')),
          recentes.length ? h('ul.atividade', recentes.map(ev => {
            const v = porId.get(ev.vagaId);
            return h('li', h('time', { datetime: ev.data }, dataCurta(ev.data)),
              h('span', h('b', TIPOS_EVENTO[ev.tipo] ?? ev.tipo), ' ', v ? `${v.empresa} — ${v.titulo}` : ev.vagaId,
                ev.autor === 'claude' ? h('span.mudo', ' (Claude)') : null));
          })) : h('p.vazio', 'Sem atividade ainda.')))));
}

function proximoFollowUp(vagas, hoje) {
  const futuros = vagas.filter(v => v.etapa === 'enviada' && v.followUp?.proximo > hoje).map(v => v.followUp.proximo).sort();
  if (!futuros.length) return vagas.some(foiEnviada) ? 'Nenhum follow-up agendado.' : 'Os follow-ups aparecem 7 dias depois de cada envio.';
  const n = futuros.filter(d => d === futuros[0]).length;
  return `Nada para hoje. Próximos: ${plural(n, 'vaga', 'vagas')} em ${dataCurta(futuros[0])}.`;
}
