// T8: servidor MCP local. Protocolo, cada ferramenta, recusas e uma conversa real por stdio.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { spawn } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { criarArmazemMemoria } from '../nucleo/armazenamento.mjs';
import { criarApi } from '../servidor/api.mjs';
import { criarServidorMcp } from '../mcp/servidor.mjs';
import { respostasParecidas } from '../mcp/ferramentas.mjs';

const PERFIL = { nome: 'Teste', respostasPadrao: [
  { pergunta: 'Anos de experiência com Spring Boot', resposta: '5' },
  { pergunta: 'Anos de experiência com React', resposta: '0' },
  { pergunta: 'Pretensão em USD por mês', resposta: '5000' },
] };

function novo() {
  const armazem = criarArmazemMemoria({ estadoInicial: { perfil: PERFIL } });
  const mcp = criarServidorMcp({ api: criarApi({ armazem, dir: null, hoje: () => '2026-11-07' }) });
  let n = 0;
  const chamar = async (method, params) => mcp.tratar({ jsonrpc: '2.0', id: ++n, method, params });
  const ferramenta = async (name, args) => {
    const r = await chamar('tools/call', { name, arguments: args });
    if (r.error) return r;
    return { erro: Boolean(r.result.isError), texto: r.result.content[0].text, dados: r.result.isError ? null : JSON.parse(r.result.content[0].text) };
  };
  return { armazem, mcp, chamar, ferramenta };
}
const vaga = (id, extra = {}) => ({ id, empresa: `Empresa ${id}`, titulo: 'Backend Java', link: `https://x.com/${id}`, plataforma: 'linkedin', ...extra });

test('protocolo: initialize, ping, lista de ferramentas, método e ferramenta desconhecidos', async () => {
  const { chamar, mcp } = novo();
  const ini = (await chamar('initialize', { protocolVersion: '2025-06-18', capabilities: {}, clientInfo: { name: 't', version: '1' } })).result;
  assert.equal(ini.protocolVersion, '2025-06-18');
  assert.equal(ini.serverInfo.name, 'central-candidaturas');
  assert.match(ini.instructions, /CPF/);
  assert.equal((await chamar('initialize', { protocolVersion: '1999-01-01' })).result.protocolVersion, '2025-06-18', 'versão desconhecida → uma que suportamos');
  assert.deepEqual((await chamar('ping')).result, {});
  const { tools } = (await chamar('tools/list')).result;
  assert.ok(tools.length >= 15);
  for (const t of tools) assert.ok(t.name && t.description && t.inputSchema?.type === 'object', t.name);
  assert.ok(!tools.some(t => /enviar|mensagem|send|email_enviar/.test(t.name)), 'não existe ferramenta de enviar mensagem');
  assert.ok(tools.find(t => t.name === 'vagas_listar').annotations.readOnlyHint);
  assert.equal((await chamar('metodo/inexistente')).error.code, -32601);
  assert.equal((await chamar('tools/call', { name: 'nao_existe', arguments: {} })).error.code, -32602);
  assert.equal(await mcp.tratar({ jsonrpc: '2.0', method: 'notifications/initialized' }), null, 'notificação não tem resposta');
  assert.equal((await mcp.receber('{isto não é json')).at(0).error.code, -32700);
});

test('ferramentas de vaga: adicionar, listar com filtros, detalhe, duplicata, mover, envio, editar, evento', async () => {
  const { ferramenta, armazem } = novo();
  const add = await ferramenta('vaga_adicionar', { vagas: [vaga('a', { idioma: 'en', aderencia: 'A' }), vaga('b'), vaga('a')] });
  assert.deepEqual([add.dados.adicionadas, add.dados.duplicadas.length], [['a', 'b'], 1]);
  const lista = await ferramenta('vagas_listar', { aderencia: 'A' });
  assert.deepEqual(lista.dados.vagas.map(v => [v.id, v.mercado]), [['a', 'internacional']]);
  assert.equal((await ferramenta('vagas_listar', { empresa: 'empresa b' })).dados.total, 1);
  assert.equal((await ferramenta('vaga_detalhe', { id: 'a' })).dados.eventos.length, 1);
  const dup = await ferramenta('vaga_duplicada', { empresa: 'Empresa a', titulo: 'Backend Java', excluir: 'x' });
  assert.deepEqual([dup.dados.duplicada, dup.dados.jaEnviada], [true, false]);
  const envio = await ferramenta('vaga_registrar_envio', { id: 'a', rodada: 3, curriculo: 'EN' });
  assert.deepEqual([envio.dados.etapa, envio.dados.enviadaEm], ['enviada', '2026-11-07']);
  assert.equal((await ferramenta('vaga_duplicada', { empresa: 'Empresa a', titulo: 'Backend Java', excluir: 'x' })).dados.jaEnviada, true);
  assert.equal((await ferramenta('vaga_mover', { id: 'b', etapa: 'pulada', motivo: 'Stack' })).dados.etapa, 'pulada');
  assert.equal((await ferramenta('vaga_editar', { id: 'a', campos: { notas: 'ok', mercado: 'nacional' } })).dados.mercado, 'nacional');
  assert.equal((await ferramenta('evento_registrar', { id: 'a', tipo: 'nota', texto: 'ligaram' })).erro, false);
  assert.equal(armazem.carregar().eventos.filter(e => e.autor === 'claude').length, 5, 'tudo gravado como "claude"');
});

test('recusas viram isError para o modelo corrigir: regras, CPF, argumentos, etapa enviada pelo caminho errado', async () => {
  const { ferramenta } = novo();
  await ferramenta('vaga_adicionar', { vagas: [vaga('a')] });
  const semMotivo = await ferramenta('vaga_mover', { id: 'a', etapa: 'pulada' });
  assert.ok(semMotivo.erro && /motivo/.test(semMotivo.texto));
  assert.ok((await ferramenta('vaga_mover', { id: 'a', etapa: 'enviada' })).erro);
  const cpf = await ferramenta('evento_registrar', { id: 'a', tipo: 'nota', texto: 'CPF 529.982.247-25' });
  assert.ok(cpf.erro && /CPF/.test(cpf.texto));
  assert.ok((await ferramenta('vaga_mover', { id: 'a' })).erro, 'falta etapa');
  assert.ok((await ferramenta('vaga_mover', { id: 'a', etapa: 'voando' })).erro, 'etapa fora da lista');
  assert.ok((await ferramenta('vaga_detalhe', { id: 'nao-existe' })).erro);
});

test('perfil, resposta padrão, rotina, pedidos e emails', async () => {
  const { ferramenta } = novo();
  assert.equal((await ferramenta('perfil_ler', {})).dados.nome, 'Teste');
  const r = await ferramenta('resposta_padrao', { pergunta: 'How many years of work experience do you have with Spring Boot?' });
  assert.deepEqual([r.dados.parecidas[0].pergunta, r.dados.parecidas[0].resposta], ['Anos de experiência com Spring Boot', '5']);
  assert.match((await ferramenta('resposta_padrao', { pergunta: 'Você fala alemão?' })).dados.aviso, /Não invente/);
  assert.equal((await ferramenta('resumo', {})).dados.total, 0);
  assert.deepEqual((await ferramenta('followups_vencidos', {})).dados, []);
  assert.equal((await ferramenta('rodada_registrar', { n: 4, resumo: 'r4' })).dados.n, 4);
  assert.deepEqual((await ferramenta('pedidos_listar', {})).dados, []);
  await ferramenta('vaga_adicionar', { vagas: [vaga('z', { empresa: 'Sezzle' })] });
  await ferramenta('vaga_registrar_envio', { id: 'z' });
  const em = await ferramenta('emails_registrar', { emails: [{ email: { idMensagem: 'm1', de: 'talent@sezzle.com', assunto: 'Your application' }, classificacao: { tipo: 'rejeicao', vagaId: null, resumo: 'Recusa' } }] });
  assert.deepEqual([em.dados[0].vagaId, em.dados[0].transicao], ['z', 'encerrada']);
  assert.deepEqual((await ferramenta('emails_sem_vaga', {})).dados, []);
});

test('resposta padrão parecida: termo técnico pesa; nada parecido → lista vazia', () => {
  assert.equal(respostasParecidas(PERFIL.respostasPadrao, 'Years with React.js?')[0].resposta, '0');
  assert.equal(respostasParecidas(PERFIL.respostasPadrao, 'Expected monthly salary in USD')[0].resposta, '5000');
  assert.deepEqual(respostasParecidas(PERFIL.respostasPadrao, 'Qual seu time do coração?'), []);
});

test('stdio: o processo conversa por linhas JSON, grava na pasta de dados e não suja o stdout', async () => {
  const raiz = join(dirname(fileURLToPath(import.meta.url)), '..');
  const dir = mkdtempSync(join(tmpdir(), 'central-mcp-'));
  const p = spawn(process.execPath, [join(raiz, 'mcp/servidor.mjs')], { env: { ...process.env, CENTRAL_DADOS: dir, CENTRAL_HOJE: '2026-11-07' } });
  let saida = '';
  p.stdout.setEncoding('utf8').on('data', d => { saida += d; });
  const linhas = [
    { jsonrpc: '2.0', id: 1, method: 'initialize', params: { protocolVersion: '2025-06-18', capabilities: {}, clientInfo: { name: 't', version: '1' } } },
    { jsonrpc: '2.0', method: 'notifications/initialized' },
    { jsonrpc: '2.0', id: 2, method: 'tools/call', params: { name: 'vaga_adicionar', arguments: { vagas: [vaga('s1')] } } },
    { jsonrpc: '2.0', id: 3, method: 'tools/call', params: { name: 'vagas_listar', arguments: {} } },
  ];
  // Mandado em pedaços quebrados no meio da linha, como pode acontecer num pipe.
  const tudo = linhas.map(l => JSON.stringify(l)).join('\n') + '\n';
  p.stdin.write(tudo.slice(0, 50));
  p.stdin.write(tudo.slice(50));
  p.stdin.end();
  await new Promise(ok => p.on('close', ok));
  const msgs = saida.trim().split('\n').map(l => JSON.parse(l));
  assert.deepEqual(msgs.map(m => m.id), [1, 2, 3], 'uma resposta por pedido, na ordem; nada para a notificação');
  assert.equal(JSON.parse(msgs[2].result.content[0].text).vagas[0].id, 's1');
});
