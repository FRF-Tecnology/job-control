import { h } from '../util.mjs';

// Campos simples do perfil: [chave, rótulo, tipo, dica]
const CAMPOS = [
  ['Contato', [['nome', 'Nome completo', 'text'], ['email', 'E-mail', 'email'], ['telefone', 'Telefone', 'text'], ['cidade', 'Cidade', 'text'], ['linkedin', 'LinkedIn', 'url'], ['github', 'GitHub', 'url']]],
  ['Profissional', [['cargo', 'Como você se apresenta', 'text'], ['experienciaAnos', 'Anos de experiência', 'number'], ['ingles', 'Inglês', 'text', 'Básico, Conversacional, Avançado…'], ['formacao', 'Formação', 'text'], ['inicio', 'Disponibilidade para começar', 'text']]],
  ['Remuneração', [['pretensaoCLT', 'Pretensão CLT (R$/mês)', 'number'], ['pretensaoPJ', 'Pretensão PJ (R$/mês)', 'number'], ['salarioAtual', 'Salário atual', 'text', 'deixe em branco se preferir']]],
];

export function telaPerfil(app) {
  const perfil = structuredClone(app.estado.perfil ?? {});
  perfil.filtros ??= {};
  perfil.respostasPadrao ??= [];

  const campo = (k, rotulo, tipo, dica) => {
    const i = h('input', { id: `perfil-${k}`, type: tipo, placeholder: dica ?? '' });
    i.value = perfil[k] ?? '';
    i.addEventListener('input', () => { perfil[k] = tipo === 'number' ? (i.value === '' ? '' : Number(i.value)) : i.value; });
    return h('label.campo', h('span', rotulo), i);
  };
  const lista = (titulo, valores, aoMudar, dica) => {
    const t = h('textarea', { style: { minHeight: '64px' }, placeholder: dica });
    t.value = (valores ?? []).join('\n');
    t.addEventListener('input', () => aoMudar(t.value.split('\n').map(s => s.trim()).filter(Boolean)));
    return h('label.campo', h('span', titulo), t);
  };

  const respostas = h('div.respostas');
  const desenharRespostas = () => respostas.replaceChildren(...perfil.respostasPadrao.map((r, i) => {
    const p = h('input', { type: 'text', 'aria-label': 'Pergunta', value: r.pergunta });
    const resp = h('input', { type: 'text', 'aria-label': 'Resposta', value: r.resposta });
    p.addEventListener('input', () => { r.pergunta = p.value; });
    resp.addEventListener('input', () => { r.resposta = resp.value; });
    return h('div.par', p, resp, h('button.botao-fantasma.p', { type: 'button', 'aria-label': `Remover ${r.pergunta}`, on: { click: () => { perfil.respostasPadrao.splice(i, 1); desenharRespostas(); } } }, 'Remover'));
  }));
  desenharRespostas();

  const modo = valor => h('label', { style: { display: 'flex', gap: '8px', alignItems: 'start' } },
    h('input', { type: 'radio', name: 'modo', value: valor, checked: (perfil.modo ?? 'revisar') === valor, on: { change: () => { perfil.modo = valor; } } }),
    h('span', valor === 'revisar' ? h('span', h('b', 'Revisar antes'), ' — o Claude só aplica o que você aprovou na triagem.') : h('span', h('b', 'Aplicar direto'), ' — o Claude aplica sozinho as vagas do grupo A e deixa B e C para a triagem.')));

  const dias = h('input', { id: 'perfil-dias', type: 'number', min: 1, max: 60, value: perfil.diasFollowUp ?? 7 });
  dias.addEventListener('input', () => { perfil.diasFollowUp = Number(dias.value) || 7; });

  return h('form.formulario', { on: { submit: async ev => {
    ev.preventDefault();
    await app.agir(() => app.api('/api/perfil', 'PUT', perfil), 'Perfil salvo');
  } } },
  h('header.cabecalho', h('div', h('h1', 'Perfil e regras'), h('p', 'O Claude lê esta página antes de cada rodada. CPF, RG e senhas nunca ficam aqui.')),
    h('button.botao', { type: 'submit' }, 'Salvar perfil')),
  CAMPOS.map(([grupo, campos]) => h('fieldset', h('legend', grupo), h('div.duas', campos.map(c => campo(...c))))),
  h('fieldset', h('legend', 'Busca e filtros'),
    h('div.duas',
      lista('Níveis aceitos', perfil.filtros.niveis, v => { perfil.filtros.niveis = v; }, 'Júnior\nPleno'),
      lista('Modelo de trabalho', perfil.filtros.modelo, v => { perfil.filtros.modelo = v; }, 'Remoto'),
      lista('Stack principal', perfil.stackPrincipal, v => { perfil.stackPrincipal = v; }, 'Java\nSpring Boot'),
      lista('Stack secundária', perfil.stackSecundaria, v => { perfil.stackSecundaria = v; }, 'React\nNode.js')),
    lista('Pular quando', perfil.filtros.pularSe, v => { perfil.filtros.pularSe = v; }, 'Sênior\nGraduação concluída obrigatória')),
  h('fieldset', h('legend', 'Como o Claude trabalha'), modo('revisar'), modo('direto'),
    h('label.campo', { style: { maxWidth: '260px' } }, h('span', 'Follow-up depois de quantos dias'), dias)),
  h('fieldset', h('legend', 'Respostas padrão dos formulários'),
    h('p.pequeno.mudo', { style: { margin: 0 } }, 'Respostas honestas que o Claude repete nos formulários. Se algo não está aqui nem no currículo, ele responde "não" em vez de inventar.'),
    respostas,
    h('div', h('button.botao-fantasma', { type: 'button', on: { click: () => { perfil.respostasPadrao.push({ pergunta: '', resposta: '' }); desenharRespostas(); respostas.lastElementChild?.querySelector('input')?.focus(); } } }, 'Adicionar resposta'))),
  h('div', h('button.botao', { type: 'submit' }, 'Salvar perfil')));
}
