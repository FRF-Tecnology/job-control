// Fase 5: ajustes adiados da revisão final e primeira vez com currículos.
import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, existsSync, readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { criarServidor } from '../servidor/app.mjs';
import { criarArmazem } from '../nucleo/armazenamento.mjs';
import { estadoVazio, adicionarVaga, moverVaga, registrarEvento } from '../nucleo/servico.mjs';
import { validarVaga, validarPerfil } from '../nucleo/validacao.mjs';
import { lerValor } from '../nucleo/modelo.mjs';
import { resumo, funil } from '../nucleo/funil.mjs';

const raiz = join(dirname(fileURLToPath(import.meta.url)), '..');
const novoDir = () => mkdtempSync(join(tmpdir(), 'central-f5-'));
const ctx = (agora = '2026-10-30') => ({ autor: 'eu', agora, diasFollowUp: 7 });
const cli = (dir, ...args) => spawnSync(process.execPath, [join(raiz, 'ferramentas/cli.mjs'), ...args], { env: { ...process.env, CENTRAL_DADOS: dir, CENTRAL_HOJE: '2026-10-30' }, encoding: 'utf8' });

let srv, url, dirSrv;
before(async () => {
  dirSrv = novoDir();
  srv = criarServidor({ dir: dirSrv, hoje: () => '2026-10-30' });
  await new Promise(r => srv.listen(0, '127.0.0.1', r));
  url = `http://127.0.0.1:${srv.address().port}`;
});
after(() => srv.close());
const api = (caminho, metodo = 'GET', corpo) =>
  fetch(url + caminho, { method: metodo, headers: { 'content-type': 'application/json' }, body: corpo === undefined ? undefined : JSON.stringify(corpo) });

// CSV
test('CSV neutraliza fórmulas (=, +, -, @) para o Excel não executar', async () => {
  await api('/api/vagas', 'POST', { id: 'csv-1', empresa: '=HYPERLINK("http://x")', titulo: '+Dev', notas: '@soma' });
  const txt = await (await api('/api/exportar.csv')).text();
  assert.match(txt, /"'=HYPERLINK/);
  assert.match(txt, /"'\+Dev"/);
  assert.match(txt, /"'@soma"/);
});

// Pretensão em formato brasileiro
test('lerValor entende 8.500, 8500, 8.500,50 e R$ 8.500', () => {
  assert.equal(lerValor('8.500'), 8500);
  assert.equal(lerValor('8500'), 8500);
  assert.equal(lerValor('8.500,50'), 8500.5);
  assert.equal(lerValor('R$ 8.500'), 8500);
  assert.equal(lerValor('8500.5'), 8500.5);
  assert.ok(Number.isNaN(lerValor('abc')));
});

test('cli mover --pretensao 8.500 grava 8500', () => {
  const dir = novoDir();
  cli(dir, 'add', '--json', JSON.stringify({ id: 'p1', empresa: 'E', titulo: 'T' }));
  const r = cli(dir, 'mover', 'p1', 'enviada', '--pretensao', '8.500');
  assert.equal(r.status, 0, r.stderr);
  assert.equal(criarArmazem(dir).carregar().vagas[0].envio.pretensao, 8500);
});

// CLI: valor que começa com "--"
test('cli aceita --opcao=valor, inclusive valor começando com "--"', () => {
  const dir = novoDir();
  cli(dir, 'add', '--json', JSON.stringify({ id: 'o1', empresa: 'E', titulo: 'T' }));
  const r = cli(dir, 'mover', 'o1', 'pulada', '--motivo=--Outro');
  assert.equal(r.status, 0, r.stderr);
  assert.equal(criarArmazem(dir).carregar().vagas[0].motivo, '--Outro');
});

// Backup diário a cada gravação, não só ao abrir o servidor
test('primeira gravação do dia cria o backup do dia', () => {
  const dir = novoDir();
  const a = criarArmazem(dir, { hoje: () => '2026-10-30' });
  a.transacao(e => adicionarVaga(e, { id: 'b1', empresa: 'E', titulo: 'T' }, ctx()));
  assert.ok(existsSync(join(dir, 'backups', '2026-10-30')));
  const a2 = criarArmazem(dir, { hoje: () => '2026-10-31' });
  a2.transacao(e => adicionarVaga(e, { id: 'b2', empresa: 'E2', titulo: 'T2' }, ctx('2026-10-31')));
  // o backup do dia 31 guarda o estado de antes da gravação (só b1)
  assert.equal(JSON.parse(readFileSync(join(dir, 'backups', '2026-10-31', 'vagas.json'), 'utf8')).length, 1);
});

// Ids de evento ordenáveis
test('ids de evento do mesmo dia ficam em ordem de texto (999 antes de 1000)', () => {
  let e = estadoVazio();
  ({ estado: e } = adicionarVaga(e, { id: 'v', empresa: 'E', titulo: 'T' }, ctx()));
  e.eventos = Array.from({ length: 998 }, (_, i) => ({ id: `velho-${i}`, vagaId: 'v', tipo: 'nota', data: '2026-10-01' }));
  let a, b;
  ({ estado: e, evento: a } = registrarEvento(e, 'v', 'nota', 'a', ctx()));
  ({ estado: e, evento: b } = registrarEvento(e, 'v', 'nota', 'b', ctx()));
  assert.ok(a.id < b.id, `${a.id} deveria vir antes de ${b.id}`);
});

// Em processo sem envio
test('mover direto para em-processo registra o envio, e Hoje e funil batem', () => {
  let e = estadoVazio();
  ({ estado: e } = adicionarVaga(e, { id: 'r1', empresa: 'Recrutadora', titulo: 'Java' }, ctx()));
  ({ estado: e } = moverVaga(e, 'r1', 'em-processo', { subEtapa: 'entrevista-rh' }, ctx()));
  assert.equal(e.vagas[0].envio.data, '2026-10-30');
  assert.equal(resumo(e.vagas, '2026-10-30').emProcesso, funil(e.vagas).find(f => f.etapa === 'Em processo').total);
});

// Tipos
test('validarVaga recusa campos de texto com outro tipo e subetapa desconhecida', () => {
  const v = { id: 'x', empresa: 'E', titulo: 'T', etapa: 'descoberta' };
  assert.deepEqual(validarVaga(v), []);
  assert.ok(validarVaga({ ...v, notas: {} }).some(x => /notas/.test(x)));
  assert.ok(validarVaga({ ...v, empresa: ['a'] }).some(x => /empresa/.test(x)));
  assert.ok(validarVaga({ ...v, stack: 'java' }).some(x => /stack/.test(x)));
  assert.ok(validarVaga({ ...v, etapa: 'em-processo', envio: { data: '2026-10-30' }, subEtapa: 7 }).some(x => /sub/.test(x)));
  assert.deepEqual(validarVaga({ ...v, notas: null, stack: ['Java'], subEtapa: null }), []);
});

test('validarPerfil recusa respostasPadrao fora do formato', () => {
  assert.deepEqual(validarPerfil({ nome: 'A', respostasPadrao: [{ pergunta: 'p', resposta: 'r' }] }), []);
  assert.ok(validarPerfil({ respostasPadrao: 'x' }).length);
  assert.ok(validarPerfil({ respostasPadrao: [null] }).length);
  assert.ok(validarPerfil({ nome: {} }).length);
  assert.ok(validarPerfil({ filtros: { niveis: 'Pleno' } }).length);
});

test('corpos malformados respondem 400, nunca 500', async () => {
  await api('/api/vagas', 'POST', { id: 'm1', empresa: 'E', titulo: 'T' });
  const casos = [
    ['PUT', '/api/perfil', { respostasPadrao: 'x' }],
    ['PUT', '/api/perfil', { respostasPadrao: [null] }],
    ['PATCH', '/api/vagas/m1', { notas: {} }],
    ['POST', '/api/vagas/m1/eventos', { tipo: {}, texto: 'a' }],
    ['POST', '/api/vagas/m1/eventos', { texto: {} }],
    ['PUT', '/api/plataformas/p', { nome: 5 }],
  ];
  for (const [m, p, b] of casos) assert.equal((await api(p, m, b)).status, 400, `${m} ${p} ${JSON.stringify(b)}`);
  assert.equal((await api('/api/vagas/%E0%A4%A/mover', 'POST', { etapa: 'pulada' })).status, 404);
});

// Primeira vez: currículos
test('GET /api/curriculos lista os PDFs de dados/curriculos e diz o que falta', async () => {
  let r = await (await api('/api/curriculos')).json();
  assert.deepEqual(r.arquivos, []);
  mkdirSync(join(dirSrv, 'curriculos'), { recursive: true });
  writeFileSync(join(dirSrv, 'curriculos', 'meu-cv.pdf'), '%PDF-1.4');
  writeFileSync(join(dirSrv, 'curriculos', 'leia.txt'), 'x');
  r = await (await api('/api/curriculos')).json();
  assert.deepEqual(r.arquivos.map(a => a.nome), ['meu-cv.pdf']);
  assert.ok(r.pasta.endsWith('curriculos'));
});

test('primeiros passos: perfil, currículo e plataformas', async () => {
  const dir = novoDir();
  const s = criarServidor({ dir, hoje: () => '2026-10-30' });
  await new Promise(r => s.listen(0, '127.0.0.1', r));
  const u = `http://127.0.0.1:${s.address().port}`;
  const passos = async () => (await (await fetch(u + '/api/primeiros-passos')).json()).passos;
  try {
    let p = await passos();
    assert.deepEqual(p.map(x => [x.id, x.feito]), [['perfil', false], ['curriculo', false], ['rodada', false]]);
    await fetch(u + '/api/perfil', { method: 'PUT', headers: { 'content-type': 'application/json' }, body: JSON.stringify({ nome: 'Fulana', pretensaoCLT: 5000 }) });
    mkdirSync(join(dir, 'curriculos'), { recursive: true });
    writeFileSync(join(dir, 'curriculos', 'cv.pdf'), '%PDF');
    p = await passos();
    assert.deepEqual(p.map(x => x.feito), [true, true, false]);
  } finally { s.close(); }
});
