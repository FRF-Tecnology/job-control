import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { criarServidor } from '../servidor/app.mjs';

let srv, url;
before(async () => {
  srv = criarServidor({ dir: mkdtempSync(join(tmpdir(), 'central-srv-')), hoje: () => '2026-10-29' });
  await new Promise(r => srv.listen(0, '127.0.0.1', r));
  url = `http://127.0.0.1:${srv.address().port}`;
});
after(() => srv.close());

const api = (caminho, metodo = 'GET', corpo) =>
  fetch(url + caminho, { method: metodo, headers: { 'content-type': 'application/json' }, body: corpo ? JSON.stringify(corpo) : undefined });

test('estado vazio responde com listas vazias e a data de hoje', async () => {
  const r = await api('/api/estado');
  assert.equal(r.status, 200);
  const e = await r.json();
  assert.deepEqual(e.vagas, []);
  assert.equal(e.hoje, '2026-10-29');
});

test('criar, mover e ver evento pela API', async () => {
  assert.equal((await api('/api/vagas', 'POST', { id: 'x1', empresa: 'Acme', titulo: 'Java Pleno' })).status, 201);
  const r = await api('/api/vagas/x1/mover', 'POST', { etapa: 'aplicar' });
  assert.equal(r.status, 200);
  const e = await (await api('/api/estado')).json();
  assert.equal(e.vagas.find(v => v.id === 'x1').etapa, 'aplicar');
  assert.ok(e.eventos.some(ev => ev.vagaId === 'x1' && ev.tipo === 'aplicar' && ev.autor === 'eu'));
});

test('duplicada responde 409 com a vaga existente', async () => {
  const r = await api('/api/vagas', 'POST', { id: 'x2', empresa: 'Acme', titulo: 'Java Pleno' });
  assert.equal(r.status, 409);
  assert.equal((await r.json()).existente.id, 'x1');
});

test('pular sem motivo responde 400 com a lista de erros', async () => {
  const r = await api('/api/vagas/x1/mover', 'POST', { etapa: 'pulada' });
  assert.equal(r.status, 400);
  assert.ok((await r.json()).erros.some(e => /motivo/.test(e)));
});

test('vaga inexistente responde 404', async () => {
  assert.equal((await api('/api/vagas/nada/mover', 'POST', { etapa: 'aplicar' })).status, 404);
});

test('follow-up adiar e nota pela API', async () => {
  await api('/api/vagas/x1/mover', 'POST', { etapa: 'enviada' });
  assert.equal((await api('/api/vagas/x1/followup', 'POST', { acao: 'adiar', dias: 3 })).status, 200);
  assert.equal((await api('/api/vagas/x1/eventos', 'POST', { tipo: 'nota', texto: 'ligar sexta' })).status, 201);
  const e = await (await api('/api/estado')).json();
  assert.equal(e.vagas.find(v => v.id === 'x1').followUp.proximo, '2026-11-01');
});

test('perfil com CPF é recusado', async () => {
  const r = await api('/api/perfil', 'PUT', { nome: 'A', obs: 'cpf 123.456.789-09' });
  assert.equal(r.status, 400);
});

test('versão muda depois de uma gravação', async () => {
  const v1 = (await (await api('/api/versao')).json()).versao;
  await api('/api/vagas/x1', 'PATCH', { notas: 'nova nota' });
  const v2 = (await (await api('/api/versao')).json()).versao;
  assert.notEqual(v1, v2);
});

test('serve a página e os módulos do núcleo', async () => {
  const r = await fetch(url + '/');
  assert.equal(r.status, 200);
  assert.match(r.headers.get('content-type'), /html/);
  assert.match(await (await fetch(url + '/nucleo/funil.mjs')).text(), /export function resumo/);
});

test('não deixa sair da pasta web nem ler os dados por caminho', async () => {
  for (const p of ['/web/../dados/perfil.json', '/..%2fpackage.json', '/nucleo/../package.json']) {
    assert.equal((await fetch(url + p)).status, 404, p);
  }
});

test('exporta CSV com cabeçalho', async () => {
  const t = await (await fetch(url + '/api/exportar.csv')).text();
  assert.match(t.split('\n')[0], /empresa/);
  assert.match(t, /Acme/);
});

test('importar sem a chave local responde 403', async () => {
  const r = await fetch(url + '/api/importar', { method: 'POST', headers: { 'content-type': 'application/json' }, body: '[]' });
  assert.equal(r.status, 403);
});

test('importar com a chave adiciona em lote e libera CORS', async () => {
  const chave = (await (await api('/api/chave')).json()).chave;
  const pre = await fetch(url + '/api/importar', { method: 'OPTIONS', headers: { origin: 'https://www.linkedin.com', 'access-control-request-method': 'POST', 'access-control-request-private-network': 'true' } });
  assert.equal(pre.status, 204);
  assert.equal(pre.headers.get('access-control-allow-private-network'), 'true');
  const r = await fetch(url + '/api/importar', { method: 'POST', headers: { 'content-type': 'application/json', 'x-central-chave': chave, origin: 'https://www.linkedin.com' }, body: JSON.stringify([{ id: 'imp-1', empresa: 'Nova', titulo: 'Java Jr' }, { id: 'x1', empresa: 'Acme', titulo: 'Java Pleno' }]) });
  assert.equal(r.status, 200);
  assert.equal(r.headers.get('access-control-allow-origin'), '*');
  const res = await r.json();
  assert.deepEqual([res.adicionadas.length, res.duplicadas.length], [1, 1]);
});

test('a chave só é entregue para a própria central (sem CORS)', async () => {
  const r = await api('/api/chave');
  assert.equal(r.headers.get('access-control-allow-origin'), null);
});

test('recusa requisições com Host de fora (proteção contra DNS rebinding)', async () => {
  const { request } = await import('node:http');
  const status = await new Promise(ok => {
    const req = request({ host: '127.0.0.1', port: srv.address().port, path: '/api/chave', headers: { host: 'atacante.exemplo:80' } }, res => { res.resume(); ok(res.statusCode); });
    req.end();
  });
  assert.equal(status, 403);
});
