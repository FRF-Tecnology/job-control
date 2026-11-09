// Erro inesperado (500) no servidor Node (referência do app Go, jc-a2y.13):
// a resposta é a mensagem fixa e o log só tem método, caminho sem query e tipo do erro.
// Dados fictícios: "Usuario Exemplo" faz o papel do nome do usuário do sistema na pasta de dados.
import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { criarServidor, ERRO_INTERNO, tipoDoErro } from '../servidor/app.mjs';

const pastaFicticia = 'C:\\Users\\Usuario Exemplo\\AppData\\Roaming\\Central de Candidaturas';
const erroComCaminho = () => Object.assign(new Error(`ENOTDIR: not a directory, mkdir '${pastaFicticia}\\backups'`),
  { code: 'ENOTDIR', syscall: 'mkdir', path: `${pastaFicticia}\\backups` });

// Armazém que falha em qualquer chamada, com o caminho da pasta no texto do erro.
const armazemQuebrado = new Proxy({}, { get: (_, nome) => (nome === 'dir' ? undefined : () => { throw erroComCaminho(); }) });

let srv, url;
const linhas = [];
const consoleErrorAntes = console.error;
before(async () => {
  srv = criarServidor({ dir: mkdtempSync(join(tmpdir(), 'central-500-')), armazem: armazemQuebrado, hoje: () => '2026-11-08' });
  await new Promise(r => srv.listen(0, '127.0.0.1', r));
  url = `http://127.0.0.1:${srv.address().port}`;
  console.error = (...a) => linhas.push(a.map(String).join(' '));
});
after(() => { console.error = consoleErrorAntes; srv.close(); });

test('500: resposta fixa e log sem caminho da pasta nem query', async () => {
  const r = await fetch(`${url}/api/v1/duplicatas?empresa=Empresa%20Ficticia%20Secreta&titulo=Vaga%20Secreta`);
  assert.equal(r.status, 500);
  assert.deepEqual(await r.json(), { erro: ERRO_INTERNO });
  const log = linhas.join('\n');
  for (const proibido of ['Usuario Exemplo', 'Central de Candidaturas', 'backups', 'Empresa', 'Secreta', 'empresa=', '?']) {
    assert.ok(!log.includes(proibido), `o log traz "${proibido}": ${log}`);
  }
  assert.match(log, /^api: erro 500 em GET \/api\/v1\/duplicatas \(Error ENOTDIR mkdir\)$/m);
});

test('tipoDoErro: só classe, código e chamada do sistema, seguindo cause', () => {
  assert.equal(tipoDoErro(erroComCaminho()), 'Error ENOTDIR mkdir');
  assert.equal(tipoDoErro(new TypeError('x Usuario Exemplo', { cause: Object.assign(new Error('y'), { code: 'tem espaço' }) })), 'TypeError > Error');
  assert.equal(tipoDoErro('texto solto com Usuario Exemplo'), 'string');
});
