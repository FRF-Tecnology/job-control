// jc-a2y.28: a etapa "aplicar" é a aprovação da pessoa na Triagem; o /aplicar envia a partir dela.
// O assistente (MCP e CLI, autor "claude") não pode gravar essa aprovação, nem pôr a vaga em outra etapa ao adicionar,
// nem ligar o modo "direto" do perfil (que aplica o grupo A sem Triagem). Só a pessoa (autor "eu", a tela) pode.
// Dados fictícios.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { criarArmazemMemoria } from '../nucleo/armazenamento.mjs';
import { criarApi, PREFIXO } from '../servidor/api.mjs';
import { criarServidorMcp } from '../mcp/servidor.mjs';
import { filaAplicar } from '../nucleo/servico.mjs';

const VAGA = { id: 'li-9001', empresa: 'Empresa Exemplo', titulo: 'Dev Backend', plataforma: 'linkedin', mercado: 'internacional', aderencia: 'A' };

function novo() {
  const armazem = criarArmazemMemoria({ estadoInicial: { perfil: { nome: 'Pessoa Exemplo', modo: 'revisar' } } });
  const api = criarApi({ armazem, dir: null, hoje: () => '2026-11-09' });
  const mcp = criarServidorMcp({ api });
  let n = 0;
  const ferramenta = async (name, args) => {
    const r = await mcp.tratar({ jsonrpc: '2.0', id: ++n, method: 'tools/call', params: { name, arguments: args } });
    return { erro: Boolean(r.result?.isError), texto: r.result?.content?.[0]?.text ?? '' };
  };
  const chamar = (metodo, caminho, corpo, autor) => api.chamar({ metodo, caminho: PREFIXO + caminho, corpo, autor });
  return { armazem, ferramenta, chamar };
}

test('MCP: vaga_mover para aplicar é recusado e nada muda', async () => {
  const { armazem, ferramenta } = novo();
  assert.equal((await ferramenta('vaga_adicionar', { vagas: [VAGA] })).erro, false);
  const antes = structuredClone(armazem.carregar());
  const r = await ferramenta('vaga_mover', { id: VAGA.id, etapa: 'aplicar' });
  assert.equal(r.erro, true, r.texto);
  assert.match(r.texto, /só a pessoa aprova/);
  assert.deepEqual(armazem.carregar(), antes);
});

test('MCP: vaga_adicionar com etapa diferente de descoberta é recusado e nada é gravado', async () => {
  for (const etapa of ['aplicar', 'enviada', 'bloqueada', 'em-processo', 'oferta', 'pulada', 'encerrada']) {
    const { armazem, ferramenta } = novo();
    const r = await ferramenta('vaga_adicionar', { vagas: [{ ...VAGA, etapa, motivo: 'Outro', pendencia: 'x' }] });
    const e = armazem.carregar();
    assert.equal(e.vagas.length, 0, `${etapa}: ${r.texto}`);
    assert.match(r.texto, /só adiciona vagas em descoberta/, etapa);
  }
});

test('CLI (autor claude): mover para aplicar, adicionar fora de descoberta e ligar o modo direto são recusados', async () => {
  const { armazem, chamar } = novo();
  assert.equal((await chamar('POST', '/vagas', VAGA, 'claude')).status, 201);
  const antes = structuredClone(armazem.carregar());
  assert.equal((await chamar('POST', `/vagas/${VAGA.id}/mover`, { etapa: 'aplicar' }, 'claude')).status, 400);
  assert.equal((await chamar('POST', '/vagas', { ...VAGA, id: 'li-9002', titulo: 'Outra', etapa: 'aplicar' }, 'claude')).status, 400);
  for (const [m, corpo] of [['PATCH', { modo: 'direto' }], ['PUT', { nome: 'Pessoa Exemplo', modo: 'direto' }]]) {
    const r = await chamar(m, '/perfil', corpo, 'claude');
    assert.equal(r.status, 400, `${m} /perfil`);
    assert.match(JSON.stringify(r.corpo), /modo/);
  }
  assert.deepEqual(armazem.carregar(), antes);
  // O resto do perfil o assistente ainda pode ajustar, desde que não mude o modo.
  assert.equal((await chamar('PATCH', '/perfil', { cidade: 'Cidade Exemplo', modo: 'revisar' }, 'claude')).status, 200);
});

test('a pessoa (autor eu) continua aprovando, adicionando em qualquer etapa e escolhendo o modo', async () => {
  const { chamar } = novo();
  assert.equal((await chamar('POST', '/vagas', VAGA, 'eu')).status, 201);
  assert.equal((await chamar('POST', `/vagas/${VAGA.id}/mover`, { etapa: 'aplicar' }, 'eu')).status, 200);
  assert.equal((await chamar('POST', '/vagas', { ...VAGA, id: 'li-9003', titulo: 'Outra', etapa: 'aplicar' }, 'eu')).status, 201);
  assert.equal((await chamar('PATCH', '/perfil', { modo: 'direto' }, 'eu')).status, 200);
});

test('fila do /aplicar: só vagas em aplicar com aprovação (evento aplicar) da pessoa', async () => {
  const { armazem, chamar } = novo();
  for (const id of ['li-a', 'li-b', 'li-c']) await chamar('POST', '/vagas', { ...VAGA, id, titulo: `Dev ${id}` }, 'claude');
  await chamar('POST', '/vagas/li-a/mover', { etapa: 'aplicar' }, 'eu');
  // Dado antigo: vaga posta em aplicar pelo assistente antes desta correção (gravado direto no armazém).
  const e = armazem.carregar();
  e.vagas.find(v => v.id === 'li-b').etapa = 'aplicar';
  e.eventos.push({ id: 'ev-legado', vagaId: 'li-b', tipo: 'aplicar', autor: 'claude', data: '2026-11-08', texto: '' });
  armazem.salvar(e);
  const fila = filaAplicar(armazem.carregar());
  assert.deepEqual(fila.map(v => v.id), ['li-a']);
  // No modo direto, o grupo A em descoberta entra (a pessoa escolheu o modo).
  await chamar('PATCH', '/perfil', { modo: 'direto' }, 'eu');
  assert.deepEqual(filaAplicar(armazem.carregar()).map(v => v.id).sort(), ['li-a', 'li-c']);
  // A vaga li-b (aprovada pelo assistente) nunca entra.
  assert.ok(!filaAplicar(armazem.carregar()).some(v => v.id === 'li-b'));
});

// CLI de verdade (autor "claude"): mover para aplicar sai com erro e a fila do /aplicar continua vazia.
import { spawnSync } from 'node:child_process';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

test('CLI: mover para aplicar é recusado e fila-aplicar só lista o que a pessoa aprovou', () => {
  const raiz = join(dirname(fileURLToPath(import.meta.url)), '..');
  const dir = mkdtempSync(join(tmpdir(), 'central-28-'));
  const cli = (...args) => spawnSync(process.execPath, [join(raiz, 'ferramentas/cli.mjs'), ...args], { env: { ...process.env, CENTRAL_DADOS: dir, CENTRAL_HOJE: '2026-11-09' }, encoding: 'utf8' });
  try {
    assert.equal(cli('add', '--json', JSON.stringify(VAGA)).status, 0);
    const mover = cli('mover', VAGA.id, 'aplicar');
    assert.notEqual(mover.status, 0);
    assert.match(mover.stderr + mover.stdout, /só a pessoa aprova/);
    assert.match(cli('fila-aplicar').stdout, /\(0\)/);
    assert.match(cli('listar', '--etapa', 'descoberta').stdout, new RegExp(VAGA.id));
  } finally { rmSync(dir, { recursive: true, force: true }); }
});
