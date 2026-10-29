// Testes que reproduzem os achados da revisão final.
import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync, utimesSync, existsSync, readFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { request } from 'node:http';
import { criarServidor } from '../servidor/app.mjs';
import { criarArmazem } from '../nucleo/armazenamento.mjs';
import { estadoVazio, adicionarVaga, moverVaga, atualizarVaga, followUpFeito, ErroValidacao } from '../nucleo/servico.mjs';
import { contemDocumento, validarVaga, perfilSensivel } from '../nucleo/validacao.mjs';
import { chaveVaga, acharDuplicata } from '../nucleo/dedupe.mjs';

const raiz = join(dirname(fileURLToPath(import.meta.url)), '..');
const novoDir = () => mkdtempSync(join(tmpdir(), 'central-rev-'));
const ctx = { autor: 'eu', agora: '2026-10-29', diasFollowUp: 7 };
const base = (x = {}) => ({ id: 'a', empresa: 'Acme', titulo: 'Dev Java', ...x });
const cli = (dir, ...args) => spawnSync(process.execPath, [join(raiz, 'ferramentas/cli.mjs'), ...args], { env: { ...process.env, CENTRAL_DADOS: dir, CENTRAL_HOJE: '2026-10-29' }, encoding: 'utf8' });

let srv, url, porta, dirSrv;
before(async () => {
  dirSrv = novoDir();
  srv = criarServidor({ dir: dirSrv, hoje: () => '2026-10-29' });
  await new Promise(r => srv.listen(0, '127.0.0.1', r));
  porta = srv.address().port;
  url = `http://127.0.0.1:${porta}`;
  await fetch(url + '/api/vagas', { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify(base()) });
});
after(() => srv.close());

// 1. CSRF
test('POST com text/plain (sem preflight) é recusado', async () => {
  const r = await fetch(url + '/api/vagas', { method: 'POST', headers: { 'content-type': 'text/plain' }, body: JSON.stringify(base({ id: 'csrf' })) });
  assert.equal(r.status, 415);
});
test('POST vindo de outro site (Sec-Fetch-Site cross-site) é recusado', async () => {
  const status = await new Promise(ok => {
    const req = request({ host: '127.0.0.1', port: porta, path: '/api/vagas/a/mover', method: 'POST', headers: { 'content-type': 'application/json', 'sec-fetch-site': 'cross-site' } }, res => { res.resume(); ok(res.statusCode); });
    req.end(JSON.stringify({ etapa: 'aplicar' }));
  });
  assert.equal(status, 403);
});

// 2. mover não aceita campos arbitrários e preserva o envio
test('mover não troca o id nem outros campos fora da lista', () => {
  const { estado } = adicionarVaga(estadoVazio(), base(), ctx);
  const { estado: e2 } = moverVaga(estado, 'a', 'aplicar', { id: 'zz', empresa: 'Outra' }, ctx);
  assert.equal(e2.vagas[0].id, 'a');
  assert.equal(e2.vagas[0].empresa, 'Acme');
});
test('mover de novo para enviada mescla o envio sem perder a data', () => {
  let { estado } = adicionarVaga(estadoVazio(), base(), ctx);
  ({ estado } = moverVaga(estado, 'a', 'enviada', { envio: { pretensao: 5200 } }, ctx));
  ({ estado } = moverVaga(estado, 'a', 'enviada', { envio: { rodada: 16 } }, { ...ctx, agora: '2026-10-30' }));
  assert.deepEqual(estado.vagas[0].envio, { pretensao: 5200, data: '2026-10-29', rodada: 16 });
});

// 3. trava de gravação e notas desatualizadas
test('trava antiga é removida e a gravação segue', () => {
  const dir = novoDir();
  const a = criarArmazem(dir);
  writeFileSync(join(dir, '.trava'), 'x');
  const antigo = (Date.now() - 60_000) / 1000;
  utimesSync(join(dir, '.trava'), antigo, antigo);
  a.transacao(e => adicionarVaga(e, base(), ctx));
  assert.equal(a.carregar().vagas.length, 1);
  assert.equal(existsSync(join(dir, '.trava')), false);
});
test('trava recente de outro processo faz a gravação esperar e falhar com mensagem clara', () => {
  const dir = novoDir();
  const a = criarArmazem(dir, { esperaTravaMs: 300 });
  writeFileSync(join(dir, '.trava'), 'x');
  assert.throws(() => a.transacao(e => adicionarVaga(e, base(), ctx)), /ocupad/);
});
test('salvar notas sobre uma versão antiga é recusado', () => {
  let { estado } = adicionarVaga(estadoVazio(), base({ notas: 'do Claude' }), ctx);
  assert.throws(() => atualizarVaga(estado, 'a', { notas: 'minha', notasAntes: 'antiga' }, ctx), /mudaram/);
  ({ estado } = atualizarVaga(estado, 'a', { notas: 'minha', notasAntes: 'do Claude' }, ctx));
  assert.equal(estado.vagas[0].notas, 'minha');
});

// 4. dados sensíveis
test('RG com texto entre a palavra e o número é detectado', () => {
  assert.equal(contemDocumento('meu rg é 12.345.678-9'), true);
});
test('perfil com pergunta de RG/CPF/senha ou campo senha é recusado', () => {
  assert.ok(perfilSensivel({ respostasPadrao: [{ pergunta: 'Qual seu RG?', resposta: '12.345.678-9' }] }));
  assert.ok(perfilSensivel({ senha: 'abc' }));
  assert.equal(perfilSensivel({ nome: 'A', respostasPadrao: [{ pergunta: 'PcD?', resposta: 'Não' }] }), false);
});
test('texto de follow-up com CPF é recusado', () => {
  let { estado } = adicionarVaga(estadoVazio(), base(), ctx);
  ({ estado } = moverVaga(estado, 'a', 'enviada', {}, ctx));
  assert.throws(() => followUpFeito(estado, 'a', { ...ctx, texto: 'CPF 529.982.247-25' }), ErroValidacao);
});
test('plataforma com CPF é recusada pela API', async () => {
  const r = await fetch(url + '/api/plataformas/x', { method: 'PUT', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ nome: 'X', limitacoes: 'cpf 529.982.247-25' }) });
  assert.equal(r.status, 400);
});

// 5. dup
test('cli dup ignora a própria vaga com --excluir e lista todas as outras', () => {
  const dir = novoDir();
  cli(dir, 'add', '--json', JSON.stringify(base({ id: 'g1' })));
  cli(dir, 'add', '--json', JSON.stringify(base({ id: 'g2', forcar: true })));
  const r = cli(dir, 'dup', 'Acme', 'Dev Java', '--excluir', 'g1');
  assert.match(r.stdout, /g2/);
  assert.doesNotMatch(r.stdout, /g1 \|/);
  assert.match(cli(dir, 'dup', 'Nada', 'Dev Java').stdout, /^nova/);
});

// 6. dedupe
test('dedupe ignora sufixos de empresa e back-end/backend', () => {
  assert.equal(chaveVaga('DB1 Group', 'Desenvolvedor Back-end Java Pleno'), chaveVaga('DB1', 'Desenvolvedor(a) Backend Java Pleno'));
  assert.equal(chaveVaga('DB1 Global Software', 'Backend Java Pleno'), chaveVaga('DB1', 'Backend Java Pleno'));
});
test('empresa desconhecida nunca vira duplicata', () => {
  assert.equal(acharDuplicata([base({ id: 'x', empresa: '?' })], base({ id: 'y', empresa: '?' })), null);
});
test('forcar vale por item dentro do lote', () => {
  const dir = novoDir();
  const arq = join(dir, 'l.json');
  writeFileSync(arq, JSON.stringify([base({ id: 'g1' }), base({ id: 'g2' }), base({ id: 'g3', forcar: true })]));
  const r = cli(dir, 'add', '--arquivo', arq);
  assert.match(r.stdout, /adicionadas: 2/);
  assert.match(r.stdout, /duplicadas: 1/);
});

// 8. porta real
test('cli script usa a porta gravada pelo servidor em dados/.porta', () => {
  const dir = novoDir();
  writeFileSync(join(dir, '.porta'), '4325');
  assert.match(cli(dir, 'script', 'busca-gupy').stdout, /localhost:4325/);
});

// 10. importação só cria Descoberta
test('importar força etapa descoberta, descarta envio e ignora item nulo', async () => {
  const chave = readFileSync(join(dirSrv, '.chave'), 'utf8').trim();
  const r = await fetch(url + '/api/importar', { method: 'POST', headers: { 'content-type': 'application/json', 'x-central-chave': chave }, body: JSON.stringify([{ id: 'i9', empresa: 'Imp', titulo: 'Java', etapa: 'enviada', envio: { data: '2026-01-01' } }, null]) });
  assert.equal(r.status, 200);
  const e = await (await fetch(url + '/api/estado')).json();
  const v = e.vagas.find(x => x.id === 'i9');
  assert.equal(v.etapa, 'descoberta');
  assert.equal(v.envio, undefined);
});
test('cli chave --nova troca a chave', () => {
  const dir = novoDir();
  writeFileSync(join(dir, '.chave'), 'velha');
  cli(dir, 'chave', '--nova');
  assert.notEqual(readFileSync(join(dir, '.chave'), 'utf8').trim(), 'velha');
});

// menores incluídos
test('link só aceita http(s)', () => {
  assert.ok(validarVaga(base({ etapa: 'descoberta', link: 'javascript:alert(1)' })).some(e => /link/.test(e)));
  assert.deepEqual(validarVaga(base({ etapa: 'descoberta', link: 'https://x.com/v' })), []);
});
test('marcadores com apóstrofo não quebram o script', () => {
  const dir = novoDir();
  writeFileSync(join(dir, 'perfil.json'), JSON.stringify({ nome: "Ana D'Ávila" }));
  const js = cli(dir, 'script', 'inhire-IH').stdout;
  assert.match(js, /name:"Ana D'Ávila"/);
});
