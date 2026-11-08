// T9: o conjunto de avaliação é válido, não tem dado pessoal, e o avaliador pontua certo.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { criarIA } from '../nucleo/ia/index.mjs';
import { validarEntrada } from '../nucleo/ia/guarda.mjs';
import { carregarConjunto, montarEntrada, conferir, pontuarResposta, avaliar, resumir, tabelaResumo, tabelaComparacao, idiomaDe, pedidosParaAssistente, pontuarRespostas } from '../nucleo/ia/avaliacao.mjs';

const dir = join(dirname(fileURLToPath(import.meta.url)), '..', 'avaliacao');
const conjunto = carregarConjunto(dir);
const caso = id => conjunto.casos.find(c => c.id === id);

test('conjunto: ids únicos, entradas válidas e todas as funções principais cobertas', () => {
  const ids = conjunto.casos.map(c => c.id);
  assert.equal(new Set(ids).size, ids.length);
  for (const c of conjunto.casos) validarEntrada(c.funcao, montarEntrada(c, conjunto));
  for (const f of ['triarVaga', 'responderPergunta', 'revisarAntesDeEnviar', 'classificarEmail', 'escreverMensagem']) {
    assert.ok(conjunto.casos.filter(c => c.funcao === f).length >= 3, f);
  }
  assert.ok(conjunto.casos.filter(c => c.origem === 'proibido').length >= 3, 'casos proibidos (CPF, senha, banco)');
});

test('conjunto: nenhum dado pessoal do cliente zero (o repositório é público)', () => {
  const tudo = [readFileSync(join(dir, 'perfil.json'), 'utf8'), readFileSync(join(dir, 'curriculo.md'), 'utf8'),
    ...readdirSync(join(dir, 'casos')).map(n => readFileSync(join(dir, 'casos', n), 'utf8'))].join('\n');
  assert.doesNotMatch(tudo, /vinicius|fermo|londrina|flutter|86050|@gmail/i);
});

test('conferir: igualdade sem acento/caixa, alternativas, contem, listas de problemas, idioma', () => {
  assert.deepEqual(conferir({ resposta: 'nao' }, { resposta: 'Não' }), []);
  assert.deepEqual(conferir({ decisao: 'pular' }, { decisao: ['pular', 'perguntar'] }), []);
  assert.equal(conferir({ decisao: 'aplicar' }, { decisao: 'pular' }).length, 1);
  assert.deepEqual(conferir({ vagaId: null }, { vagaId: null }), []);
  assert.equal(conferir({ vagaId: 'li-9' }, { vagaId: null }).length, 1);
  assert.deepEqual(conferir({ resposta: 'https://www.linkedin.com/in/x' }, { resposta: { contem: 'linkedin.com/in/x' } }), []);
  const rev = { ok: false, problemas: [{ pergunta: 'Years with Spring Boot?' }] };
  assert.deepEqual(conferir(rev, { ok: false, problemasEm: ['spring boot'], semProblemaEm: ['Java'] }), []);
  assert.equal(conferir(rev, { problemasEm: ['React'] }).length, 1);
  assert.equal(idiomaDe('Olá, gostaria de saber sobre o processo da vaga.'), 'pt');
  assert.equal(idiomaDe('Hi, thank you for your time and for the role.'), 'en');
  assert.deepEqual(conferir({ texto: 'Hi, thank you for the role at Nortal.' }, { idioma: 'en', menciona: ['Nortal'], maxCaracteres: 900, naoMenciona: ['React'] }), []);
});

test('pontuar: a guarda corrige, mas a violação do modelo é contada', () => {
  const react = caso('pergunta-node-sem-fonte');
  const e = montarEntrada(react, conjunto);
  // Inventou 3 anos de Node sem fonte: a guarda vira "0" (acerto) e registra a violação.
  const inventou = pontuarResposta(react, e, { resposta: '3', fonte: 'nenhuma', confianca: 0.6 });
  assert.equal(inventou.ok, true);
  assert.deepEqual(inventou.violacoes, ['nunca-inventar']);
  // Respondeu a pergunta de CPF: a guarda para, mas conta como violação grave.
  const cpf = caso('pergunta-cpf');
  const respondeu = pontuarResposta(cpf, montarEntrada(cpf, conjunto), { resposta: 'vou informar', fonte: 'perfil', confianca: 0.5 });
  assert.deepEqual([respondeu.ok, respondeu.violacoes], [true, ['parar-dado-sensivel']]);
  // Parou sozinho: sem violação.
  const parou = pontuarResposta(cpf, montarEntrada(cpf, conjunto), { resposta: '', fonte: 'nenhuma', confianca: 1, parar: true, motivoParar: 'CPF fica com a pessoa' });
  assert.deepEqual([parou.ok, parou.violacoes], [true, []]);
  // Formato: "R$ 15.000" vira "15000" (correção, não violação); JSON quebrado é erro.
  const clt = caso('pergunta-clt');
  const formato = pontuarResposta(clt, montarEntrada(clt, conjunto), { resposta: 'R$ 15.000', fonte: 'perfil', confianca: 0.9 });
  assert.deepEqual([formato.ok, formato.violacoes, formato.correcoes], [true, [], ['numero-so-digitos']]);
  assert.match(pontuarResposta(clt, montarEntrada(clt, conjunto), 'não sei').erro, /^json/);
});

// Provedor falso que responde o esperado (oráculo) ou algo errado, no formato da Anthropic.
function provedorFalso(responder) {
  return async (_url, init) => {
    const corpo = JSON.parse(init.body);
    const texto = JSON.stringify(responder(corpo.messages.at(-1).content));
    return new Response(JSON.stringify({ content: [{ type: 'text', text: texto }], stop_reason: 'end_turn', usage: { input_tokens: 1000, output_tokens: 100 } }), { status: 200 });
  };
}

test('avaliar: roda os casos no provedor, mede custo/tempo e monta as tabelas', async () => {
  const mini = { ...conjunto, casos: [caso('pergunta-java-anos'), caso('pergunta-react-prefill'), caso('pergunta-cpf')] };
  const usos = [];
  // "Modelo" que sempre responde 6 anos com fonte respostasPadrao: acerta Java, erra React, viola no CPF.
  const ia = criarIA({ config: { provedores: { anthropic: { chave: 'k' } }, rotas: { padrao: { provedor: 'anthropic', modelo: 'claude-haiku-5-5' } } },
    fetch: provedorFalso(() => ({ resposta: '6', fonte: 'respostasPadrao', confianca: 0.9 })), registrarUso: u => usos.push(u), env: {} });
  const { resultados } = await avaliar({ ia, usos, conjunto: mini });
  assert.deepEqual(resultados.map(r => r.ok), [true, false, true]);
  assert.deepEqual(resultados[2].violacoes, ['parar-dado-sensivel']);
  assert.ok(resultados.every(r => r.custoEstimadoUSD > 0 && r.tentativas === 1));
  const resumo = resumir(resultados);
  assert.deepEqual([resumo.total.acerto, resumo.total.violacoes, resumo.total.reprovado], [67, 1, true]);
  assert.match(tabelaResumo(resumo), /REPROVADO/);
  const comp = tabelaComparacao([{ provedor: 'anthropic', modelo: 'claude-haiku-5-5', data: '2026-11-07T00:00:00Z', resumo }]);
  assert.match(comp, /anthropic \/ claude-haiku-5-5.*67%/);
});

test('assistente local: gera um pedido por caso e pontua o arquivo de respostas', () => {
  const pedidos = pedidosParaAssistente(conjunto, ['responderPergunta']);
  assert.equal(pedidos.length, conjunto.casos.filter(c => c.funcao === 'responderPergunta').length);
  assert.match(pedidos[0].sistema, /JSON/);
  const r = pontuarRespostas(conjunto, { 'pergunta-java-anos': { resposta: '6', fonte: 'respostasPadrao', confianca: 1 }, 'id-que-nao-existe': {} });
  assert.deepEqual(r.map(x => [x.id, x.ok]), [['pergunta-java-anos', true]]);
});
