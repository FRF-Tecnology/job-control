// Ferramentas do MCP da Central (T8). Cada uma é fina: monta o pedido e chama a API v1 no mesmo processo,
// então valem as mesmas regras da CLI e da tela (validação, CPF/RG recusado, não duplicar).
// Não existe ferramenta de enviar mensagem: o MCP só lê e grava na Central.
import { PREFIXO } from '../servidor/api.mjs';
import { normalizar, mercadoDe } from '../nucleo/modelo.mjs';

export class ErroFerramenta extends Error {}

const texto = (descricao, extra = {}) => ({ type: 'string', description: descricao, ...extra });
const numero = descricao => ({ type: 'number', description: descricao });
const objeto = (properties, required = [], descricao) => ({ type: 'object', properties, required, additionalProperties: false, ...(descricao ? { description: descricao } : {}) });
const ETAPAS = ['descoberta', 'aplicar', 'enviada', 'bloqueada', 'em-processo', 'oferta', 'encerrada', 'pulada'];
const enc = encodeURIComponent;

// Resumo de uma vaga para listas (menos tokens para o assistente).
const curta = v => ({ id: v.id, empresa: v.empresa, titulo: v.titulo, etapa: v.etapa, subEtapa: v.subEtapa ?? undefined, aderencia: v.aderencia ?? undefined,
  mercado: mercadoDe(v), plataforma: v.plataforma, enviadaEm: v.envio?.data, pendencia: v.pendencia ?? undefined, link: v.link });

// Palavras que não ajudam a comparar perguntas de formulário.
const VAZIAS = new Set(['de', 'da', 'do', 'com', 'em', 'a', 'o', 'e', 'voce', 'você', 'sua', 'seu', 'qual', 'quantos', 'the', 'you', 'your', 'how', 'many', 'do', 'have', 'with', 'of', 'in', 'what', 'is', 'are', 'for', 'to', 'work', 'experience', 'experiencia', 'anos', 'years']);
// "React.js" = "React", "Node.js" = "Node"; ".NET" e "C#" ficam inteiros.
const palavras = s => new Set(normalizar(s).replace(/\.js\b/g, '').replace(/[^a-z0-9+#.]+/g, ' ').split(' ')
  .map(p => p.replace(/\.+$/, '')).filter(p => p && !VAZIAS.has(p)));

// Resposta padrão mais parecida com a pergunta do formulário (sobreposição de palavras, termos técnicos pesam mais).
export function respostasParecidas(respostasPadrao = [], pergunta, limite = 3) {
  const alvo = palavras(pergunta);
  if (!alvo.size) return [];
  return respostasPadrao
    .map(r => {
      const p = palavras(r.pergunta);
      const comuns = [...alvo].filter(x => p.has(x));
      const nota = comuns.reduce((a, x) => a + (/^[a-z]+$/.test(x) && x.length <= 3 ? 1 : 2), 0) / (alvo.size + p.size);
      return { pergunta: r.pergunta, resposta: r.resposta, semelhanca: Math.round(nota * 100) / 100 };
    })
    .filter(r => r.semelhanca > 0)
    .sort((a, b) => b.semelhanca - a.semelhanca)
    .slice(0, limite);
}

export const FERRAMENTAS = [
  {
    nome: 'resumo', titulo: 'Resumo do dia',
    descricao: 'Números da Central: vagas por etapa, enviadas hoje e na semana, retornos e follow-ups vencidos.',
    entrada: objeto({}),
    somenteLeitura: true,
    rodar: async ({ chamar }) => chamar('GET', '/resumo'),
  },
  {
    nome: 'vagas_listar', titulo: 'Listar vagas',
    descricao: 'Lista vagas com filtros. Devolve um resumo de cada uma; use vaga_detalhe para ver tudo.',
    entrada: objeto({
      etapa: texto('Etapa', { enum: ETAPAS }),
      empresa: texto('Parte do nome da empresa'),
      aderencia: texto('Aderência', { enum: ['A', 'B', 'C'] }),
      mercado: texto('internacional ou nacional', { enum: ['internacional', 'nacional'] }),
      plataforma: texto('linkedin, gupy, inhire, externo...'),
      desde: texto('Só vagas atualizadas a partir desta data (AAAA-MM-DD)'),
      limite: numero('Máximo de vagas (padrão 50)'),
    }),
    somenteLeitura: true,
    rodar: async ({ chamar }, a) => {
      const q = new URLSearchParams(Object.entries({ etapa: a.etapa, plataforma: a.plataforma }).filter(([, v]) => v));
      const vagas = await chamar('GET', `/vagas${q.size ? `?${q}` : ''}`);
      const lista = vagas.filter(v => (!a.empresa || normalizar(v.empresa).includes(normalizar(a.empresa)))
        && (!a.aderencia || v.aderencia === a.aderencia)
        && (!a.mercado || mercadoDe(v) === a.mercado)
        && (!a.desde || (v.atualizadoEm ?? '') >= a.desde));
      return { total: lista.length, vagas: lista.slice(0, a.limite || 50).map(curta) };
    },
  },
  {
    nome: 'vaga_detalhe', titulo: 'Detalhe da vaga',
    descricao: 'Vaga completa com a linha do tempo (eventos, emails).',
    entrada: objeto({ id: texto('Id da vaga (ex.: li-4479790788)') }, ['id']),
    somenteLeitura: true,
    rodar: async ({ chamar }, a) => chamar('GET', `/vagas/${enc(a.id)}`),
  },
  {
    nome: 'vaga_duplicada', titulo: 'Checar duplicata',
    descricao: 'Antes de aplicar: lista vagas com a mesma empresa e título. Se alguma já foi enviada, não aplique de novo.',
    entrada: objeto({ empresa: texto('Empresa'), titulo: texto('Título da vaga'), excluir: texto('Id da própria vaga, para não contar ela mesma') }, ['empresa', 'titulo']),
    somenteLeitura: true,
    rodar: async ({ chamar }, a) => {
      const ds = await chamar('GET', `/duplicatas?${new URLSearchParams({ empresa: a.empresa, titulo: a.titulo, ...(a.excluir ? { excluir: a.excluir } : {}) })}`);
      return { duplicada: ds.length > 0, jaEnviada: ds.some(d => d.envio?.data), vagas: ds.map(curta) };
    },
  },
  {
    nome: 'vaga_adicionar', titulo: 'Adicionar vagas',
    descricao: 'Adiciona uma ou mais vagas (vão para Descoberta por padrão). Duplicadas e inválidas voltam no resultado, sem derrubar as outras. Informe "mercado" (internacional ou nacional).',
    entrada: objeto({
      vagas: { type: 'array', description: 'Vagas com id, empresa, titulo, link, plataforma, idioma, stack, aderencia, mercado, motivoClaude', items: { type: 'object' } },
      forcar: { type: 'boolean', description: 'Aceita mesmo empresa+título se o id for outro (vaga republicada)' },
    }, ['vagas']),
    rodar: async ({ chamar }, a) => chamar('POST', '/vagas', { lista: a.vagas, forcar: Boolean(a.forcar) }),
  },
  {
    nome: 'vaga_mover', titulo: 'Mover vaga de etapa',
    descricao: 'Muda a etapa. pulada e encerrada pedem motivo; bloqueada pede pendencia (o que falta a pessoa fazer). Para enviada use vaga_registrar_envio.',
    entrada: objeto({
      id: texto('Id da vaga'),
      etapa: texto('Nova etapa', { enum: ETAPAS }),
      motivo: texto('Motivo (pulada: Sênior, Inglês, Diploma, Stack, Presencial/Local, Documento, Duplicada, Vaga fechada, Outro)'),
      pendencia: texto('O que falta a pessoa fazer (bloqueada)'),
      subEtapa: texto('teste, entrevista-rh, entrevista-tecnica ou proposta', { enum: ['teste', 'entrevista-rh', 'entrevista-tecnica', 'proposta'] }),
      texto: texto('Nota curta para a linha do tempo'),
    }, ['id', 'etapa']),
    rodar: async ({ chamar }, a) => {
      if (a.etapa === 'enviada') throw new ErroFerramenta('para marcar como enviada use vaga_registrar_envio');
      const { id, ...corpo } = a;
      return curta(await chamar('POST', `/vagas/${enc(id)}/mover`, corpo));
    },
  },
  {
    nome: 'vaga_registrar_envio', titulo: 'Registrar candidatura enviada',
    descricao: 'Marca a vaga como enviada e agenda o follow-up. Use só depois que a candidatura foi realmente enviada.',
    entrada: objeto({
      id: texto('Id da vaga'),
      rodada: numero('Número da rodada'),
      curriculo: texto('Currículo usado', { enum: ['PT', 'EN'] }),
      pretensao: numero('Pretensão informada (só número)'),
      respostas: texto('Resumo das respostas dadas no formulário'),
      data: texto('Data do envio (AAAA-MM-DD); padrão hoje'),
    }, ['id']),
    rodar: async ({ chamar }, a) => {
      const { id, ...envio } = a;
      return curta(await chamar('POST', `/vagas/${enc(id)}/mover`, { etapa: 'enviada', envio }));
    },
  },
  {
    nome: 'vaga_editar', titulo: 'Editar campos da vaga',
    descricao: 'Muda campos livres: notas, aderencia, mercado, link, nivel, contrato, modelo, stack, idioma, motivoClaude, local, titulo, empresa.',
    entrada: objeto({ id: texto('Id da vaga'), campos: { type: 'object', description: 'Campos a mudar' } }, ['id', 'campos']),
    rodar: async ({ chamar }, a) => curta(await chamar('PATCH', `/vagas/${enc(a.id)}`, a.campos)),
  },
  {
    nome: 'evento_registrar', titulo: 'Registrar evento',
    descricao: 'Adiciona algo à linha do tempo da vaga (retorno, teste, entrevista, nota).',
    entrada: objeto({ id: texto('Id da vaga'), tipo: texto('retorno, teste, entrevista, nota...'), texto: texto('O que aconteceu') }, ['id', 'tipo', 'texto']),
    rodar: async ({ chamar }, a) => chamar('POST', `/vagas/${enc(a.id)}/eventos`, { tipo: a.tipo, texto: a.texto }),
  },
  {
    nome: 'perfil_ler', titulo: 'Ler perfil',
    descricao: 'Perfil da pessoa: pretensões, inglês, stack, filtros, foco, prioridade de vagas e respostas padrão. Leia antes de triar ou aplicar.',
    entrada: objeto({}),
    somenteLeitura: true,
    rodar: async ({ chamar }) => chamar('GET', '/perfil'),
  },
  {
    nome: 'resposta_padrao', titulo: 'Resposta padrão para pergunta de formulário',
    descricao: 'Recebe o texto da pergunta do formulário e devolve as respostas padrão mais parecidas. Se nenhuma servir, não invente: experiência que não está no perfil é 0 / Não.',
    entrada: objeto({ pergunta: texto('Pergunta como aparece no formulário') }, ['pergunta']),
    somenteLeitura: true,
    rodar: async ({ chamar }, a) => {
      const perfil = await chamar('GET', '/perfil');
      const parecidas = respostasParecidas(perfil.respostasPadrao, a.pergunta);
      return { pergunta: a.pergunta, parecidas, aviso: parecidas.length ? undefined : 'Nenhuma resposta padrão parecida. Não invente experiência: numérico 0, sim/não Não.' };
    },
  },
  {
    nome: 'followups_vencidos', titulo: 'Follow-ups vencidos',
    descricao: 'Vagas com follow-up para hoje, com uma mensagem sugerida. A mensagem é só rascunho: enviar exige o "pode mandar" da pessoa.',
    entrada: objeto({}),
    somenteLeitura: true,
    rodar: async ({ chamar }) => (await chamar('GET', '/followups')).map(v => ({ ...curta(v), venceEm: v.followUp?.proximo, mensagemSugerida: v.mensagem })),
  },
  {
    nome: 'followup_registrar', titulo: 'Registrar follow-up',
    descricao: 'Marca o follow-up como feito (a pessoa enviou) ou adia por alguns dias.',
    entrada: objeto({ id: texto('Id da vaga'), acao: texto('feito ou adiar', { enum: ['feito', 'adiar'] }), dias: numero('Dias para adiar (padrão 3)') }, ['id', 'acao']),
    rodar: async ({ chamar }, a) => curta(await chamar('POST', `/vagas/${enc(a.id)}/followup`, { acao: a.acao, dias: a.dias })),
  },
  {
    nome: 'rodada_registrar', titulo: 'Registrar rodada',
    descricao: 'Registra ou atualiza uma rodada (busca + aplicação) com fontes e resumo.',
    entrada: objeto({ n: numero('Número da rodada'), fontes: { type: 'array', items: { type: 'string' } }, resumo: texto('O que aconteceu na rodada') }, ['n']),
    rodar: async ({ chamar }, a) => chamar('PUT', `/rodadas/${enc(String(a.n))}`, a),
  },
  {
    nome: 'pedidos_listar', titulo: 'Pedidos da caixa "O que vamos fazer hoje?"',
    descricao: 'Pedidos que a pessoa digitou na tela Hoje e ainda não foram feitos.',
    entrada: objeto({}),
    somenteLeitura: true,
    rodar: async ({ chamar }) => chamar('GET', '/pedidos?status=aberto'),
  },
  {
    nome: 'pedido_concluir', titulo: 'Concluir pedido',
    descricao: 'Marca um pedido como feito, com uma resposta curta que aparece na tela Hoje.',
    entrada: objeto({ id: texto('Id do pedido'), resposta: texto('Resposta curta (ex.: "4 enviadas, 1 travou no login")') }, ['id']),
    rodar: async ({ chamar }, a) => chamar('POST', `/pedidos/${enc(a.id)}/concluir`, { resposta: a.resposta ?? '' }),
  },
  {
    nome: 'emails_registrar', titulo: 'Registrar emails de processo seletivo',
    descricao: 'Registra emails já classificados: a Central acha a vaga e muda a etapa (recusa → encerrada, convite → entrevista, teste, oferta). Repetido não duplica. Nunca mande o corpo do email, só um resumo.',
    entrada: objeto({
      emails: { type: 'array', items: { type: 'object' }, description: 'Itens { email: { idMensagem, de, assunto, data }, classificacao: { tipo, vagaId, resumo, proximoPasso } }' },
    }, ['emails']),
    rodar: async ({ chamar }, a) => chamar('POST', '/emails', { lista: a.emails }),
  },
  {
    nome: 'emails_sem_vaga', titulo: 'Emails sem vaga',
    descricao: 'Emails que não deu para ligar a uma vaga (a pessoa decide na tela Hoje).',
    entrada: objeto({}),
    somenteLeitura: true,
    rodar: async ({ chamar }) => chamar('GET', '/emails?status=sem-vaga'),
  },
];

// Liga as ferramentas a uma API v1 (criarApi). O autor das gravações é o assistente ("claude").
export function criarFerramentas(api) {
  async function chamar(metodo, caminho, corpo) {
    const r = await api.chamar({ metodo, caminho: PREFIXO + caminho, corpo, autor: 'claude' });
    if (r.status >= 400) throw new ErroFerramenta(r.corpo?.erros?.join('; ') || r.corpo?.erro || `erro ${r.status}`);
    return r.corpo;
  }
  const porNome = new Map(FERRAMENTAS.map(f => [f.nome, f]));
  return {
    listar: () => FERRAMENTAS.map(f => ({
      name: f.nome, title: f.titulo, description: f.descricao, inputSchema: f.entrada,
      annotations: { title: f.titulo, readOnlyHint: Boolean(f.somenteLeitura), destructiveHint: false, openWorldHint: false },
    })),
    existe: nome => porNome.has(nome),
    rodar: (nome, args = {}) => porNome.get(nome).rodar({ chamar }, args ?? {}),
  };
}
