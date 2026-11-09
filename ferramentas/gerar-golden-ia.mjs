// Grava as respostas da IA do Node (nucleo/ia) para os testes da IA em Go (internal/ia):
//   - internal/ia/funcoes.json: os contratos (esquemas de entrada e saída) e as instruções de cada função;
//   - internal/ia/testdata/golden-ia.json: prompts montados, guarda aplicada e pedidos de cada provedor.
// Uso: node ferramentas/gerar-golden-ia.mjs   (GOLDEN_SAIDA troca o arquivo do golden)
process.env.TZ = 'America/Sao_Paulo';
import { writeFileSync, mkdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { FUNCOES, montarPrompt } from '../nucleo/ia/funcoes.mjs';
import { guardar, validarEntrada, REGRAS } from '../nucleo/ia/guarda.mjs';
import { custoEstimado } from '../nucleo/ia/precos.mjs';
import { carregarConjunto, montarEntrada } from '../nucleo/ia/avaliacao.mjs';
import * as anthropic from '../nucleo/ia/provedores/anthropic.mjs';
import * as openai from '../nucleo/ia/provedores/openai.mjs';
import * as google from '../nucleo/ia/provedores/google.mjs';

const raiz = join(dirname(fileURLToPath(import.meta.url)), '..');
const json = v => (v === undefined ? null : JSON.parse(JSON.stringify(v)));
// Chaves em ordem alfabética, em qualquer profundidade (o Go grava objetos assim).
const ordenar = v => Array.isArray(v) ? v.map(ordenar)
  : v && typeof v === 'object' ? Object.fromEntries(Object.keys(v).sort().map(k => [k, ordenar(v[k])])) : v;
const erroDe = e => ({ tipo: e.tipo ?? e.name, mensagem: e.message, regra: e.regra ?? null, erros: e.erros ?? null });

// ---------- contratos (o Go embute este arquivo) ----------
const contratos = Object.fromEntries(Object.entries(FUNCOES).map(([nome, f]) => [nome, { entrada: f.entrada, saida: f.saida, sistema: f.sistema }]));
// GOLDEN_FUNCOES troca o destino do funcoes.json (o teste de golden em dia usa).
const destinoFuncoes = process.env.GOLDEN_FUNCOES || join(raiz, 'internal/ia/funcoes.json');
writeFileSync(destinoFuncoes, JSON.stringify({ regrasFixas: montarPrompt('triarVaga', { vaga: {}, perfil: {} }).sistema.split('Regras que não mudam:\n')[1].split('\n\nResponda')[0], funcoes: contratos }, null, 1) + '\n');

const conj = carregarConjunto(join(raiz, 'avaliacao'));
const prompts = [];
const guarda = [];
const entradas = [];

// ---------- prompts e entradas inválidas ----------
for (const c of conj.casos) {
  const entrada = ordenar(montarEntrada(c, conj));
  try {
    validarEntrada(c.funcao, entrada);
    prompts.push({ id: c.id, funcao: c.funcao, entrada, prompt: montarPrompt(c.funcao, entrada) });
  } catch (e) {
    entradas.push({ funcao: c.funcao, entrada, erro: erroDe(e) });
  }
}
for (const [funcao, entrada] of [
  ['triarVaga', { vaga: {} }], ['triarVaga', { vaga: 'x', perfil: {} }], ['responderPergunta', { pergunta: 'Anos?', tipoCampo: 'opcao', perfil: {} }],
  ['responderPergunta', { pergunta: 'Anos?', tipoCampo: 'data', perfil: {} }], ['classificarEmail', { email: { de: 'a', assunto: 'b' }, vagas: [] }],
  ['revisarAntesDeEnviar', { formulario: [{ valor: 'x' }], perfil: {} }], ['funcaoQueNaoExiste', {}], ['avaliarEntrevista', { transcript: 3, vaga: {} }],
]) {
  try { validarEntrada(funcao, entrada); entradas.push({ funcao, entrada, erro: null }); } catch (e) { entradas.push({ funcao, entrada, erro: erroDe(e) }); }
}

// ---------- guarda: respostas de propósito certas e erradas ----------
const respostas = {
  triarVaga: [
    { decisao: 'aplicar', aderencia: 'A', motivo: 'bate' }, { decisao: 'pular', aderencia: 'C', motivo: 'sênior' },
    { decisao: 'pular', aderencia: 'C', motivo: 'stack', motivoPulo: 'Stack' }, { decisao: 'talvez', aderencia: 'A', motivo: 'x' },
    { decisao: 'aplicar', aderencia: 'A' }, { decisao: 'aplicar', aderencia: 'A', motivo: 'x', extra: 1 },
    { decisao: 'perguntar', aderencia: 'B', motivo: 'pede CPF 529.982.247-25' }, '```json\n{"decisao":"aplicar","aderencia":"B","motivo":"ok"}\n```',
    'isso não é json', '[]', 'null', { decisao: 'pular', aderencia: 'Z', motivo: 3 },
  ],
  responderPergunta: [
    { resposta: '5', fonte: 'nenhuma', confianca: 0.5 }, { resposta: '3', fonte: 'curriculo', confianca: 0.9 },
    { resposta: 'R$ 5.000', fonte: 'perfil', confianca: 0.8 }, { resposta: 'cinco anos', fonte: 'perfil', confianca: 0.8 },
    { resposta: 'Sim', fonte: 'curriculo', confianca: 0.7 }, { resposta: 'talvez', fonte: 'perfil', confianca: 0.4 },
    { resposta: 'yes', fonte: 'respostasPadrao', confianca: 1 }, { resposta: '', fonte: 'nenhuma', confianca: 1, parar: true, motivoParar: 'dado sensível' },
    { resposta: '123.456.789-09', fonte: 'perfil', confianca: 1 }, { resposta: 'x', fonte: 'perfil', confianca: 1.5 },
    { resposta: 'Tenho muita experiência', fonte: 'nenhuma', confianca: 0.9 }, { resposta: '2-3', fonte: 'perfil', confianca: 0.5 },
  ],
  revisarAntesDeEnviar: [
    { ok: true, problemas: [] }, { ok: true, problemas: [{ pergunta: 'Anos de Java', valorAtual: '10', sugerido: '3', motivo: 'currículo diz 3' }] },
    { ok: false, problemas: [] }, { ok: false, problemas: [{ pergunta: 'x', valorAtual: '1' }] },
  ],
  classificarEmail: [
    { tipo: 'rejeicao', vagaId: null, resumo: 'recusa' }, { tipo: 'entrevista', vagaId: 'nao-existe', resumo: 'convite', proximoPasso: 'agendar' },
    { tipo: 'oferta', resumo: 'x' }, { tipo: 'spam', vagaId: null, resumo: 'x' },
  ],
  avaliarEntrevista: [
    { pontosFortes: ['clareza'], melhorar: ['exemplos'], perguntasFeitas: ['sobre você'], nota: 4 },
    { pontosFortes: [], melhorar: [], perguntasFeitas: [], nota: 7 }, { pontosFortes: [], melhorar: [], perguntasFeitas: [], nota: 3.5 },
  ],
  escreverMensagem: [
    { assunto: 'Follow-up', texto: 'Olá! Queria saber do processo.' }, { texto: 'Hi! Any update?' },
    { texto: 'Meu CPF é 529.982.247-25' }, { assunto: 3, texto: 'x' },
  ],
};
const entradaExtra = {
  avaliarEntrevista: [ordenar({ transcript: 'Falei sobre Java e Spring.', vaga: { id: 'li-1', empresa: 'Acme', titulo: 'Dev' } })],
};
const entradasGuarda = []; // cada entrada uma vez; os casos apontam pelo índice
const ref = e => { let i = entradasGuarda.indexOf(e); if (i < 0) i = entradasGuarda.push(e) - 1; return i; };
const porFuncao = {};
for (const p of prompts) (porFuncao[p.funcao] ??= []).push(p.entrada);
for (const [funcao, lista] of Object.entries(entradaExtra)) (porFuncao[funcao] ??= []).push(...lista);
for (const [funcao, saidas] of Object.entries(respostas)) {
  for (const entrada of porFuncao[funcao] ?? []) {
    for (const s of saidas) {
      const bruto = typeof s === 'string' ? s : JSON.stringify(s);
      try { guarda.push({ funcao, entrada: ref(entrada), bruto, resultado: json(guardar(funcao, entrada, bruto)) }); } catch (e) { guarda.push({ funcao, entrada: ref(entrada), bruto, erro: erroDe(e) }); }
    }
  }
}

// ---------- provedores: o pedido que cada um monta e como lê a resposta ----------
const provedores = [];
const p0 = montarPrompt('triarVaga', prompts[0].entrada);
const respostasProvedor = {
  anthropic: [{ content: [{ type: 'text', text: '{"a":1}' }, { type: 'tool_use' }, { type: 'text', text: '' }], stop_reason: 'end_turn', usage: { input_tokens: 120, output_tokens: 30 } },
    { stop_reason: 'refusal', content: [] }],
  openai: [{ choices: [{ message: { content: '{"a":1}' } }], usage: { prompt_tokens: 90, completion_tokens: 12 } }, { choices: [{ message: { refusal: 'não' } }] }, { choices: [] }],
  google: [{ candidates: [{ content: { parts: [{ text: 'pensando', thought: true }, { text: '{"a":1}' }] } }], usageMetadata: { promptTokenCount: 50, candidatesTokenCount: 7, thoughtsTokenCount: 20 } },
    { promptFeedback: { blockReason: 'SAFETY' } }],
};
for (const [nome, mod] of Object.entries({ anthropic, openai, google })) {
  for (const [i, resposta] of respostasProvedor[nome].entries()) {
    for (const opcoes of [{}, { url: 'https://proxy.exemplo', maxTokens: 900 }]) {
      let pedido;
      const fetch = async (url, init) => { pedido = { url, headers: init.headers, corpo: JSON.parse(init.body) }; return new Response(JSON.stringify(resposta), { status: 200 }); };
      const caso = { provedor: nome, modelo: `modelo-${nome}`, chave: `chave-${nome}`, opcoes, prompt: p0, resposta };
      try {
        const r = await mod.chamar({ fetch, chave: caso.chave, modelo: caso.modelo, opcoes, ...p0 });
        provedores.push({ ...caso, pedido, resultado: r });
      } catch (e) {
        provedores.push({ ...caso, pedido, erro: erroDe(e) });
      }
      if (i > 0) break; // pedidos iguais; só a leitura muda
    }
  }
}
// Erros HTTP: 429/5xx reintentáveis, 4xx não; a chave nunca aparece na mensagem.
const errosHttp = [];
for (const [status, corpo] of [[429, 'devagar'], [500, 'quebrou chave-anthropic'], [400, '{"error":"modelo inválido"}'], [401, 'chave-anthropic inválida']]) {
  const fetch = async () => new Response(corpo, { status });
  try { await anthropic.chamar({ fetch, chave: 'chave-anthropic', modelo: 'm', ...p0 }); } catch (e) { errosHttp.push({ status, corpo, erro: { ...erroDe(e), reintentavel: e.reintentavel ?? null } }); }
}
try { await anthropic.chamar({ fetch: async () => new Response('x'), chave: '', modelo: 'm', ...p0 }); } catch (e) { errosHttp.push({ status: 0, corpo: 'sem chave', erro: { ...erroDe(e), reintentavel: e.reintentavel ?? null } }); }
// Corpos de erro no formato de cada provedor, ecoando uma chave fictícia de várias formas (jc-a2y.18).
// A mensagem só pode ter o texto fixo do status: nenhum valor do corpo entra.
const chaveEco = 'chave-FICTICIA-0123/abc+XYZ=fim9876';
const ecos = [chaveEco, chaveEco.replaceAll('/', '\\/'), encodeURIComponent(chaveEco), chaveEco.slice(0, 14) + '...' + chaveEco.slice(-4), chaveEco.slice(-20)];
for (const [status, corpo] of [
  [401, JSON.stringify({ type: 'error', error: { type: 'authentication_error', message: `rejected: ${ecos[1]}` } })],
  [404, JSON.stringify({ error: { message: `model x not found (${ecos[2]})`, type: 'invalid_request_error', code: 'model_not_found' } })],
  [403, JSON.stringify({ error: { code: 403, message: `blocked: ${ecos[3]}`, status: 'PERMISSION_DENIED' } })],
  [400, JSON.stringify({ error: { type: 'FICTICIA', message: ecos[4] } })],
  [503, `<html>${ecos[0]}</html>`],
  [408, ''],
  [418, JSON.stringify({ error: { code: 'codigo com espaço', message: 'x' } })],
]) {
  const fetch = async () => new Response(corpo, { status });
  try { await anthropic.chamar({ fetch, chave: chaveEco, modelo: 'm', ...p0 }); } catch (e) { errosHttp.push({ status, corpo, chave: chaveEco, erro: { ...erroDe(e), reintentavel: e.reintentavel ?? null } }); }
}

const custos = [['claude-haiku-5-5', 1000, 200], ['claude-opus-5-5', 1_000_000, 0], ['gpt-x', 10, 10], ['claude-sonnet-5-5', null, 3]].map(([m, a, b]) => ({ modelo: m, entrada: a, saida: b, custo: custoEstimado(m, a, b) }));

const destino = process.env.GOLDEN_SAIDA || join(raiz, 'internal/ia/testdata/golden-ia.json');
mkdirSync(dirname(destino), { recursive: true });
writeFileSync(destino, JSON.stringify({ regras: REGRAS.map(r => ({ id: r.id, funcoes: r.funcoes, descricao: r.descricao })), prompts, entradas, entradasGuarda, guarda, provedores, errosHttp, custos }, null, 1) + '\n');
console.log(`${prompts.length} prompts, ${entradas.length} entradas, ${guarda.length} casos de guarda, ${provedores.length + errosHttp.length} de provedor → ${destino}`);
