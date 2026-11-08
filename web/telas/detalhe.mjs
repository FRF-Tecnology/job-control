import { h, dataCurta, nomeEtapa, etapaChip, nomePlataforma, NOMES_SUB } from '../util.mjs';
import { ETAPAS, MOTIVOS_PULO, MOTIVOS_ENCERRAMENTO, SUB_ETAPAS } from '../../nucleo/modelo.mjs';

const gaveta = () => document.getElementById('gaveta');
const veu = () => document.getElementById('veu');
let aberta = null;

export const detalheAberto = () => !gaveta().hidden;

export function fecharDetalhe() {
  gaveta().hidden = true;
  veu().hidden = true;
  aberta = null;
}

async function moverCom(app, v, etapa) {
  const extras = {};
  if (etapa === 'pulada' || etapa === 'encerrada') {
    const motivo = await app.perguntar({ titulo: etapa === 'pulada' ? 'Por que pular?' : 'Por que encerrar?', opcoes: etapa === 'pulada' ? MOTIVOS_PULO : MOTIVOS_ENCERRAMENTO });
    if (!motivo) return;
    extras.motivo = motivo;
  }
  if (etapa === 'bloqueada') {
    const p = await app.perguntar({ titulo: 'O que falta você fazer?', campo: 'ex.: digitar CPF e enviar', opcoes: ['Digitar CPF e enviar', 'Fazer login no portal', 'Gravar vídeo', 'Fazer teste online'] });
    if (!p) return;
    extras.pendencia = p;
  }
  if (etapa === 'em-processo') {
    const s = await app.perguntar({ titulo: 'Em que etapa está?', opcoes: SUB_ETAPAS.map(x => NOMES_SUB[x]) });
    if (!s) return;
    extras.subEtapa = SUB_ETAPAS.find(x => NOMES_SUB[x] === s);
  }
  app.mover(v.id, etapa, extras, `Movida para ${nomeEtapa(etapa)}`);
}

export function abrirDetalhe(app, id) {
  const v = app.estado.vagas.find(x => x.id === id);
  if (!v) return fecharDetalhe();
  // Não redesenha se a pessoa está digitando nas notas desta vaga.
  if (aberta === id && gaveta().contains(document.activeElement) && document.activeElement.matches('textarea,input')) return;
  const eventos = app.estado.eventos.filter(e => e.vagaId === id).sort((a, b) => b.data.localeCompare(a.data) || b.id.localeCompare(a.id));
  const fechar = () => { location.hash = location.hash.split('/').slice(0, 2).join('/'); };

  const notas = h('textarea#notas-vaga', { placeholder: 'Anotações: recrutador, dúvidas, próximos passos…' });
  notas.value = v.notas ?? '';
  const salvarNotas = async () => {
    const minha = notas.value;
    if ((v.notas ?? '') === minha) return;
    const caminho = `/api/v1/vagas/${encodeURIComponent(id)}`;
    try {
      await app.api(caminho, 'PATCH', { notas: minha, notasAntes: v.notas ?? '' });
      app.aviso('Notas salvas');
      app.sujo = false;
      return app.recarregar();
    } catch (e) {
      if (!/mudaram/.test(e.message)) return app.aviso(e.message, true);
    }
    // Alguém (ex.: o Claude) mudou as notas enquanto você digitava: junte as duas versões.
    const atual = (await app.api('/api/v1/estado')).vagas.find(x => x.id === id)?.notas ?? '';
    const juntas = await app.perguntar({ titulo: 'As notas mudaram enquanto você editava', texto: 'Juntei as duas versões. Ajuste e salve.', campo: 'Notas', valor: `${atual}\n\n${minha}`.trim(), multilinha: true, confirmar: 'Salvar notas' });
    if (juntas == null) { app.aviso('Suas notas não foram salvas', true); return; }
    await app.agir(() => app.api(caminho, 'PATCH', { notas: juntas, notasAntes: atual }), 'Notas salvas');
  };
  notas.addEventListener('blur', salvarNotas);

  const novaNota = h('input#nova-nota', { type: 'text', placeholder: 'Registrar algo (ex.: recrutador respondeu)' });
  const tipoNota = h('select', { 'aria-label': 'Tipo' }, ['nota', 'retorno', 'teste', 'entrevista', 'follow-up'].map(t => h('option', { value: t }, t)));

  const dados = [
    ['Plataforma', nomePlataforma(v.plataforma, app.estado.plataformas)],
    v.aderencia && ['Aderência', `Grupo ${v.aderencia}`],
    v.contrato && ['Contrato', v.contrato],
    v.nivel && ['Nível', v.nivel],
    v.idioma === 'en' && ['Idioma', 'Inglês'],
    ['Encontrada', dataCurta(v.descobertaEm)],
    v.envio?.data && ['Enviada', `${dataCurta(v.envio.data)}${v.envio.rodada ? `, rodada ${v.envio.rodada}` : ''}`],
    v.envio?.pretensao && ['Pretensão', `R$ ${Number(v.envio.pretensao).toLocaleString('pt-BR')}`],
    v.envio?.curriculo && ['Currículo', v.envio.curriculo === 'EN' ? 'Inglês' : 'Português'],
    v.envio?.respostas && ['Respostas', v.envio.respostas],
    v.followUp?.proximo && v.etapa === 'enviada' && ['Follow-up', `${dataCurta(v.followUp.proximo)}${v.followUp.feitos ? ` (${v.followUp.feitos} feito)` : ''}`],
    v.subEtapa && ['Etapa do processo', NOMES_SUB[v.subEtapa]],
    v.motivo && ['Motivo', v.motivo],
    v.pendencia && ['Falta você', v.pendencia],
  ].filter(Boolean);

  gaveta().replaceChildren(...[
    h('div.topo',
      h('div', h('div.mudo', v.empresa), h('h2', { style: { fontSize: '1.5rem', lineHeight: 1.2 } }, v.titulo), h('div', { style: { marginTop: '6px' } }, etapaChip(v.etapa))),
      h('button.botao-fantasma.p', { type: 'button', 'aria-label': 'Fechar detalhe', on: { click: fechar } }, 'Fechar')),
    v.motivoClaude && h('div.ficha', { style: { padding: '12px 14px', boxShadow: 'none', background: 'var(--superficie-2)' } }, h('div', h('b', 'Observação do Claude: '), v.motivoClaude)),
    h('div.acoes',
      v.link && h('a.botao', { href: v.link, target: '_blank', rel: 'noopener' }, 'Abrir vaga'),
      h('label.campo', { style: { minWidth: '200px' } }, h('span', 'Mover para'),
        h('select', { on: { change: ev => { const e = ev.target.value; ev.target.value = ''; if (e) moverCom(app, v, e); } } },
          h('option', { value: '' }, nomeEtapa(v.etapa)),
          ETAPAS.filter(e => e !== v.etapa).map(e => h('option', { value: e }, nomeEtapa(e)))))),
    h('dl', dados.map(([k, val]) => [h('dt', k), h('dd', val)])),
    h('label.campo', h('span', 'Notas'), notas),
    h('section',
      h('h3', { style: { marginBottom: '10px' } }, 'Linha do tempo'),
      h('form.acoes', { style: { marginBottom: '14px', flexWrap: 'nowrap' }, on: { submit: async ev => {
        ev.preventDefault();
        const texto = novaNota.value.trim();
        if (!texto) return;
        const ok = await app.agir(() => app.api(`/api/v1/vagas/${encodeURIComponent(id)}/eventos`, 'POST', { tipo: tipoNota.value, texto }), 'Registrado');
        if (ok) novaNota.value = '';
      } } }, tipoNota, novaNota, h('button.botao-fantasma', { type: 'submit' }, 'Registrar')),
      eventos.length ? h('ol.linha-tempo', eventos.map(e => h(`li.e-${e.tipo}`,
        h('div', h('b', nomeEtapa(e.tipo)), e.texto ? ` — ${e.texto}` : ''),
        h('div.q', `${dataCurta(e.data)} · ${e.autor === 'claude' ? 'Claude' : 'você'}`)))) : h('p.mudo', 'Sem eventos.')),
    h('p.pequeno.mudo', `id: ${v.id}`),
  ].filter(Boolean));
  gaveta().hidden = false;
  veu().hidden = false;
  if (aberta !== id) gaveta().querySelector('button')?.focus();
  aberta = id;
}
