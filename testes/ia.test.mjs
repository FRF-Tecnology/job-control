import { test } from 'node:test';
import assert from 'node:assert/strict';
import { criarIA, somarUso, concluirPendente, ErroIA, REGRAS, FUNCOES } from '../nucleo/ia/index.mjs';
import { custoEstimado } from '../nucleo/ia/precos.mjs';

// ---------- apoio: fetch falso (sem rede) ----------
const CHAVES = { anthropic: 'sk-ant-TESTE123', openai: 'sk-oa-TESTE123', google: 'g-TESTE123' };
const MODELOS = { anthropic: 'claude-haiku-5-5', openai: 'gpt-teste', google: 'gemini-teste' };
const PROVEDORES = Object.keys(MODELOS);

// Embrulha o que o "modelo disse" no formato de resposta de cada provedor.
function embrulhar(url, saida, [tin, tout] = [100, 20]) {
  const t = typeof saida === 'string' ? saida : JSON.stringify(saida);
  if (url.includes('anthropic')) return { content: [{ type: 'text', text: t }], stop_reason: 'end_turn', usage: { input_tokens: tin, output_tokens: tout } };
  if (url.includes('openai')) return { choices: [{ message: { role: 'assistant', content: t } }], usage: { prompt_tokens: tin, completion_tokens: tout } };
  return { candidates: [{ content: { role: 'model', parts: [{ text: t }] } }], usageMetadata: { promptTokenCount: tin, candidatesTokenCount: tout } };
}
const falha = (status, corpo = '{"error":"x"}') => ({ falha: status, corpo });

// Cada passo: saída do modelo | falha(status) | Error (rede). O último se repete.
function fetchFalso(...passos) {
  const chamadas = [];
  const f = async (url, init) => {
    chamadas.push({ url, headers: init.headers, corpo: JSON.parse(init.body) });
    const p = passos[Math.min(chamadas.length - 1, passos.length - 1)];
    if (p instanceof Error) throw p;
    if (p?.falha) return new Response(p.corpo, { status: p.falha });
    return new Response(JSON.stringify(embrulhar(url, p)), { status: 200 });
  };
  f.chamadas = chamadas;
  return f;
}

const config = (provedor, rotas = {}) => ({
  provedores: Object.fromEntries(PROVEDORES.map(p => [p, { chave: CHAVES[p] }])),
  rotas: { padrao: { provedor, modelo: MODELOS[provedor] }, ...rotas },
});
const criar = (cfg, fetch, usos = []) => criarIA({ config: cfg, fetch, registrarUso: r => usos.push(r), agora: () => '2026-11-07T12:00:00.000Z', env: {} });

const perfil = { cargo: 'Dev backend Java', experienciaAnos: 3, stackPrincipal: ['Java'], respostasPadrao: [{ pergunta: 'Como soube da vaga?', resposta: 'LinkedIn' }] };
const curriculo = 'Java e Spring Boot, 3 anos.';
const vaga = { id: 'li-1', empresa: 'Acme', titulo: 'Dev Java' };
const pergunta = (p, tipoCampo = 'texto', extra = {}) => ({ pergunta: p, tipoCampo, perfil, curriculo, ...extra });
const triagemOk = { decisao: 'aplicar', aderencia: 'A', motivo: 'Stack e nível batem' };

// ---------- provedores montam o pedido certo e leem a resposta ----------
test('anthropic: Messages API com chave, versão e formato json_schema', async () => {
  const f = fetchFalso(triagemOk);
  const r = await criar(config('anthropic'), f).triarVaga({ vaga, perfil });
  assert.deepEqual(r, triagemOk);
  const [c] = f.chamadas;
  assert.equal(c.url, 'https://api.anthropic.com/v1/messages');
  assert.equal(c.headers['x-api-key'], CHAVES.anthropic);
  assert.equal(c.headers['anthropic-version'], '2026-10-21');
  assert.equal(c.corpo.model, 'claude-haiku-5-5');
  assert.match(c.corpo.system, /CPF/);
  assert.deepEqual(c.corpo.output_config.format, { type: 'json_schema', schema: FUNCOES.triarVaga.saida });
  assert.equal(c.corpo.messages[0].role, 'user');
  assert.match(c.corpo.messages[0].content, /Acme/);
  assert.equal('temperature' in c.corpo, false);
});

test('openai: Chat Completions com Bearer e response_format json_schema', async () => {
  const f = fetchFalso(triagemOk);
  assert.deepEqual(await criar(config('openai'), f).triarVaga({ vaga, perfil }), triagemOk);
  const [c] = f.chamadas;
  assert.equal(c.url, 'https://api.openai.com/v1/chat/completions');
  assert.equal(c.headers.authorization, `Bearer ${CHAVES.openai}`);
  assert.equal(c.corpo.model, 'gpt-teste');
  assert.deepEqual(c.corpo.messages.map(m => m.role), ['system', 'user']);
  assert.equal(c.corpo.response_format.type, 'json_schema');
  assert.equal(c.corpo.response_format.json_schema.name, 'triarVaga');
  assert.deepEqual(c.corpo.response_format.json_schema.schema, FUNCOES.triarVaga.saida);
});

test('google: generateContent com x-goog-api-key e responseJsonSchema; pensamento conta como saída', async () => {
  const usos = [];
  const f = async (url, init) => {
    f.c = { url, headers: init.headers, corpo: JSON.parse(init.body) };
    return new Response(JSON.stringify({
      candidates: [{ content: { parts: [{ text: 'pensando...', thought: true }, { text: JSON.stringify(triagemOk) }] } }],
      usageMetadata: { promptTokenCount: 50, candidatesTokenCount: 10, thoughtsTokenCount: 5 },
    }));
  };
  assert.deepEqual(await criar(config('google'), f, usos).triarVaga({ vaga, perfil }), triagemOk);
  assert.equal(f.c.url, 'https://generativelanguage.googleapis.com/v1beta/models/gemini-teste:generateContent');
  assert.equal(f.c.headers['x-goog-api-key'], CHAVES.google);
  assert.match(f.c.corpo.systemInstruction.parts[0].text, /CPF/);
  assert.equal(f.c.corpo.generationConfig.responseMimeType, 'application/json');
  assert.deepEqual(f.c.corpo.generationConfig.responseJsonSchema, FUNCOES.triarVaga.saida);
  assert.equal(usos[0].tokensEntrada, 50);
  assert.equal(usos[0].tokensSaida, 15);
});

test('chave vem da variável de ambiente; sem chave é erro de configuração', async () => {
  const f = fetchFalso(triagemOk);
  const cfg = { rotas: { padrao: { provedor: 'openai', modelo: 'gpt-teste' } } };
  await criarIA({ config: cfg, fetch: f, env: { OPENAI_API_KEY: 'sk-env' } }).triarVaga({ vaga, perfil });
  assert.equal(f.chamadas[0].headers.authorization, 'Bearer sk-env');
  await assert.rejects(criarIA({ config: cfg, fetch: f, env: {} }).triarVaga({ vaga, perfil }), e => e instanceof ErroIA && e.tipo === 'config');
});

test('a chave nunca aparece na mensagem de erro', async () => {
  for (const p of PROVEDORES) {
    const f = fetchFalso(falha(401, `{"error":"chave invalida: ${CHAVES[p]}"}`));
    await assert.rejects(criar(config(p), f).triarVaga({ vaga, perfil }), e => {
      assert.equal(e.tipo, 'provedor');
      assert.equal(e.message.includes(CHAVES[p]), false, e.message);
      return true;
    });
  }
});

// ---------- roteamento e reserva ----------
test('cada função vai para o provedor da sua rota; o resto usa o padrão', async () => {
  const f = fetchFalso(triagemOk, { texto: 'Olá' });
  const ia = criar(config('anthropic', { triarVaga: { provedor: 'google', modelo: 'gemini-teste' } }), f);
  await ia.triarVaga({ vaga, perfil });
  await ia.escreverMensagem({ tipo: 'follow-up', vaga, perfil, idioma: 'pt' });
  assert.match(f.chamadas[0].url, /generativelanguage/);
  assert.match(f.chamadas[1].url, /anthropic/);
});

for (const [nome, passo] of [['HTTP 500', falha(500)], ['HTTP 429', falha(429)], ['falha de rede', new TypeError('fetch failed')]]) {
  test(`${nome} no provedor principal cai para a reserva`, async () => {
    const usos = [];
    const f = fetchFalso(passo, triagemOk);
    const cfg = config('anthropic', { triarVaga: { provedor: 'anthropic', modelo: 'claude-haiku-5-5', reserva: { provedor: 'openai', modelo: 'gpt-teste' } } });
    assert.deepEqual(await criar(cfg, f, usos).triarVaga({ vaga, perfil }), triagemOk);
    assert.deepEqual(f.chamadas.map(c => new URL(c.url).host), ['api.anthropic.com', 'api.openai.com']);
    assert.deepEqual(usos.map(u => [u.provedor, u.ok]), [['anthropic', false], ['openai', true]]);
    assert.equal(usos[0].erro, 'provedor');
  });
}

test('HTTP 400 e bloqueio da guarda não acionam a reserva', async () => {
  const cfg = config('anthropic', { padrao: { provedor: 'anthropic', modelo: 'claude-haiku-5-5', reserva: { provedor: 'openai', modelo: 'gpt-teste' } } });
  const f400 = fetchFalso(falha(400), triagemOk);
  await assert.rejects(criar(cfg, f400).triarVaga({ vaga, perfil }), e => e.tipo === 'provedor' && e.status === 400);
  assert.equal(f400.chamadas.length, 1);
  const fCpf = fetchFalso({ texto: 'CPF 123.456.789-09' }, { texto: 'ok' });
  await assert.rejects(criar(cfg, fCpf).escreverMensagem({ tipo: 'follow-up', vaga, perfil, idioma: 'pt' }), e => e.regra === 'sem-documento');
  assert.equal(fCpf.chamadas.length, 1);
});

// ---------- medição ----------
test('cada chamada registra provedor, modelo, tokens, custo, tempo e vaga', async () => {
  const usos = [];
  await criar(config('anthropic'), fetchFalso(triagemOk), usos).triarVaga({ vaga, perfil });
  const [u] = usos;
  assert.equal(typeof u.ms, 'number');
  delete u.ms;
  assert.deepEqual(u, {
    funcao: 'triarVaga', provedor: 'anthropic', modelo: 'claude-haiku-5-5', tokensEntrada: 100, tokensSaida: 20,
    custoEstimadoUSD: 0.00002, ok: true, data: '2026-11-07T12:00:00.000Z', vagaId: 'li-1',
  });
  assert.equal(custoEstimado('modelo-que-nao-existe', 100, 20), null);
  const outros = [];
  await criar(config('openai'), fetchFalso(triagemOk), outros).triarVaga({ vaga, perfil });
  assert.equal(outros[0].custoEstimadoUSD, null);
});

test('somarUso agrega por candidatura, função e provedor', () => {
  const r = [
    { funcao: 'triarVaga', provedor: 'anthropic', vagaId: 'a', tokensEntrada: 100, tokensSaida: 10, custoEstimadoUSD: 0.001, ms: 5, ok: true },
    { funcao: 'responderPergunta', provedor: 'openai', vagaId: 'a', tokensEntrada: 50, tokensSaida: 5, custoEstimadoUSD: null, ms: 3, ok: false },
    { funcao: 'triarVaga', provedor: 'anthropic', vagaId: 'b', tokensEntrada: 10, tokensSaida: 1, custoEstimadoUSD: 0.002, ms: 1, ok: true },
  ];
  assert.deepEqual(somarUso(r, 'vagaId').a, { chamadas: 2, falhas: 1, tokensEntrada: 150, tokensSaida: 15, custoEstimadoUSD: 0.001, custoDesconhecido: 1, ms: 8 });
  assert.equal(somarUso(r, 'funcao').triarVaga.custoEstimadoUSD, 0.003);
  assert.deepEqual(Object.keys(somarUso(r, 'provedor')), ['anthropic', 'openai']);
});

// ---------- assistente local, entrada e contrato ----------
test('assistente-local devolve pedido pendente e a resposta dele passa pela guarda', async () => {
  const usos = [];
  const ia = criarIA({ config: {}, fetch: () => assert.fail('não deve chamar rede'), registrarUso: r => usos.push(r) });
  const p = await ia.responderPergunta(pergunta('Quantos anos de experiência com Kotlin?', 'numero'));
  assert.equal(p.pendente, true);
  assert.equal(p.funcao, 'responderPergunta');
  assert.match(p.prompt.sistema, /Nunca invente experiência/);
  assert.equal(usos[0].provedor, 'assistente-local');
  assert.equal(ia.concluir(p, '{"resposta":"4","fonte":"nenhuma","confianca":0.5}').resposta, '0');
  assert.throws(() => concluirPendente(p, 'não é json'), e => e.tipo === 'json');
});

test('entrada inválida, JSON inválido e saída fora do contrato viram ErroIA', async () => {
  const ia = criar(config('anthropic'), fetchFalso('isto não é json'));
  await assert.rejects(ia.responderPergunta({ tipoCampo: 'texto', perfil }), e => e.tipo === 'entrada');
  await assert.rejects(ia.responderPergunta(pergunta('Nível?', 'opcao')), e => e.tipo === 'entrada');
  await assert.rejects(ia.triarVaga({ vaga, perfil }), e => e.tipo === 'json');
  const ia2 = criar(config('anthropic'), fetchFalso({ decisao: 'talvez', aderencia: 'A', motivo: 'x', extra: 1 }));
  await assert.rejects(ia2.triarVaga({ vaga, perfil }), e => e.tipo === 'esquema' && e.erros.length === 2);
  const ia3 = criar(config('anthropic'), fetchFalso({ pontosFortes: [], melhorar: [], perguntasFeitas: [], nota: 9 }));
  await assert.rejects(ia3.avaliarEntrevista({ transcript: 'x', vaga }), e => e.tipo === 'esquema');
});

// ---------- aceite da T0: as regras barram a saída de QUALQUER provedor ----------
const msg = { tipo: 'follow-up', vaga, perfil, idioma: 'pt' };
const CASOS = [
  { regra: 'sem-documento', nome: 'CPF no texto da mensagem', funcao: 'escreverMensagem', entrada: msg, modelo: { texto: 'Olá! Meu CPF é 123.456.789-09.' }, bloqueia: true },
  { regra: 'sem-documento', nome: 'CPF numa resposta de formulário', funcao: 'responderPergunta', entrada: pergunta('Algo a acrescentar?'), modelo: { resposta: 'Meu documento: 52998224725', fonte: 'perfil', confianca: 0.9 }, bloqueia: true },
  { regra: 'nunca-inventar', nome: '5 anos de React inventados (fonte nenhuma)', funcao: 'responderPergunta', entrada: pergunta('Quantos anos de experiência com React?', 'numero'), modelo: { resposta: '5', fonte: 'nenhuma', confianca: 0.8 }, espera: { resposta: '0', fonte: 'nenhuma' } },
  { regra: 'nunca-inventar', nome: 'fonte "curriculo" sem currículo', funcao: 'responderPergunta', entrada: pergunta('Do you have experience with React?', 'sim-nao', { curriculo: '' }), modelo: { resposta: 'Yes', fonte: 'curriculo', confianca: 0.9 }, espera: { resposta: 'No' } },
  { regra: 'nunca-inventar', nome: 'opção conservadora da lista', funcao: 'responderPergunta', entrada: pergunta('Experiência com Kubernetes', 'opcao', { opcoes: ['Nenhuma', '1-2 anos', '3+ anos'] }), modelo: { resposta: '3+ anos', fonte: 'nenhuma', confianca: 0.6 }, espera: { resposta: 'Nenhuma' } },
  { regra: 'parar-dado-sensivel', nome: 'pergunta de senha respondida', funcao: 'responderPergunta', entrada: pergunta('Crie uma senha para acessar o portal'), modelo: { resposta: 'Senha@123', fonte: 'perfil', confianca: 0.9 }, espera: { parar: true, resposta: '' } },
  { regra: 'parar-dado-sensivel', nome: 'pergunta de CPF respondida', funcao: 'responderPergunta', entrada: pergunta('Informe seu CPF', 'numero'), modelo: { resposta: '52998224725', fonte: 'perfil', confianca: 0.9 }, espera: { parar: true, resposta: '' } },
  { regra: 'opcao-da-lista', nome: 'opção fora da lista', funcao: 'responderPergunta', entrada: pergunta('Nível de inglês', 'opcao', { opcoes: ['Básico', 'Intermediário', 'Avançado'] }), modelo: { resposta: 'Fluente', fonte: 'perfil', confianca: 0.7 }, bloqueia: true },
  { regra: 'opcao-da-lista', nome: 'opção com caixa/acento diferente volta exata', funcao: 'responderPergunta', entrada: pergunta('Nível de inglês', 'opcao', { opcoes: ['Básico', 'Intermediário', 'Avançado'] }), modelo: { resposta: 'intermediario', fonte: 'perfil', confianca: 0.7 }, espera: { resposta: 'Intermediário' } },
  { regra: 'numero-so-digitos', nome: 'número por extenso', funcao: 'responderPergunta', entrada: pergunta('Pretensão salarial CLT', 'numero'), modelo: { resposta: 'cinco mil reais', fonte: 'perfil', confianca: 0.6 }, bloqueia: true },
  { regra: 'numero-so-digitos', nome: '"R$ 5.000" vira dígitos', funcao: 'responderPergunta', entrada: pergunta('Pretensão salarial CLT', 'numero'), modelo: { resposta: 'R$ 5.000', fonte: 'perfil', confianca: 0.6 }, espera: { resposta: '5000' } },
  { regra: 'sim-nao-valido', nome: 'sim/não com "Talvez"', funcao: 'responderPergunta', entrada: pergunta('Pode começar imediatamente?', 'sim-nao'), modelo: { resposta: 'Talvez', fonte: 'perfil', confianca: 0.5 }, bloqueia: true },
  { regra: 'pulo-com-motivo', nome: 'pular sem motivo', funcao: 'triarVaga', entrada: { vaga, perfil }, modelo: { decisao: 'pular', aderencia: 'C', motivo: 'Vaga sênior' }, espera: { motivoPulo: 'Outro' } },
  { regra: 'revisao-coerente', nome: 'ok com problemas', funcao: 'revisarAntesDeEnviar', entrada: { formulario: [{ pergunta: 'Anos com React', valor: '3' }], perfil, curriculo }, modelo: { ok: true, problemas: [{ pergunta: 'Anos com React', valorAtual: '3', sugerido: '0', motivo: 'Não está no currículo' }] }, espera: { ok: false } },
  { regra: 'vaga-conhecida', nome: 'vagaId inventado', funcao: 'classificarEmail', entrada: { email: { de: 'rh@acme.com', assunto: 'Convite', texto: 'Vamos conversar?' }, vagas: [vaga] }, modelo: { tipo: 'entrevista', vagaId: 'inventada-99', resumo: 'Convite' }, espera: { vagaId: null } },
  { regra: 'mensagem-rascunho', nome: 'mensagem sai como rascunho', funcao: 'escreverMensagem', entrada: msg, modelo: { assunto: 'Follow-up', texto: 'Olá, gostaria de saber do processo.' }, espera: { rascunho: true } },
];

test('toda regra da guarda tem caso de teste', () => {
  const cobertas = new Set(CASOS.map(c => c.regra));
  for (const r of REGRAS) assert.ok(cobertas.has(r.id), `regra sem caso: ${r.id}`);
});

for (const p of PROVEDORES) {
  test(`regras barram/corrigem a saída do provedor ${p}`, async () => {
    for (const c of CASOS) {
      const usos = [];
      const chamada = criar(config(p), fetchFalso(c.modelo), usos)[c.funcao](c.entrada);
      if (c.bloqueia) {
        await assert.rejects(chamada, e => e instanceof ErroIA && e.tipo === 'regra' && e.regra === c.regra, `${p}: ${c.nome}`);
        assert.equal(usos[0].ok, false);
        assert.equal(usos[0].erro, `regra:${c.regra}`);
      } else {
        const r = await chamada;
        for (const [k, v] of Object.entries(c.espera)) assert.deepEqual(r[k], v, `${p}: ${c.nome} (${k})`);
        assert.ok(usos[0].regras.includes(c.regra), `${p}: ${c.nome} registra a regra`);
      }
    }
  });
}
