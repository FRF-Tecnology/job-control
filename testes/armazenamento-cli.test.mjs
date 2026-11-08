import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, readFileSync, existsSync, writeFileSync, readdirSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { criarArmazem } from '../nucleo/armazenamento.mjs';
import { adicionarVaga } from '../nucleo/servico.mjs';

const raiz = join(dirname(fileURLToPath(import.meta.url)), '..');
const novoDir = () => mkdtempSync(join(tmpdir(), 'central-'));
const cli = (dir, ...args) => spawnSync(process.execPath, [join(raiz, 'ferramentas/cli.mjs'), ...args], { env: { ...process.env, CENTRAL_DADOS: dir, CENTRAL_HOJE: '2026-10-29' }, encoding: 'utf8' });

test('pasta vazia carrega estado vazio', () => {
  const a = criarArmazem(novoDir());
  assert.deepEqual(a.carregar().vagas, []);
});

test('transação grava e relê do disco', () => {
  const dir = novoDir();
  const a = criarArmazem(dir);
  a.transacao(e => adicionarVaga(e, { id: 'x', empresa: 'E', titulo: 'T' }, { autor: 'eu', agora: '2026-10-29' }));
  assert.equal(JSON.parse(readFileSync(join(dir, 'vagas.json'), 'utf8')).length, 1);
  assert.equal(criarArmazem(dir).carregar().vagas[0].id, 'x');
});

test('transação que lança erro não grava nada', () => {
  const dir = novoDir();
  const a = criarArmazem(dir);
  assert.throws(() => a.transacao(() => { throw new Error('falhou'); }));
  assert.equal(existsSync(join(dir, 'vagas.json')), false);
});

test('arquivo corrompido gera erro claro em vez de apagar dados', () => {
  const dir = novoDir();
  writeFileSync(join(dir, 'vagas.json'), '{ quebrado');
  assert.throws(() => criarArmazem(dir).carregar(), /vagas\.json/);
});

test('backup diário copia os arquivos uma vez por dia', () => {
  const dir = novoDir();
  const a = criarArmazem(dir);
  a.transacao(e => adicionarVaga(e, { id: 'x', empresa: 'E', titulo: 'T' }, { autor: 'eu', agora: '2026-10-29' }));
  a.backupDiario('2026-10-29');
  assert.ok(readdirSync(join(dir, 'backups', '2026-10-29')).includes('vagas.json'));
});

test('cli add + mover + resumo', () => {
  const dir = novoDir();
  let r = cli(dir, 'add', '--json', JSON.stringify({ id: 'li-1', empresa: 'Compass UOL', titulo: 'Quarkus', plataforma: 'linkedin' }));
  assert.equal(r.status, 0, r.stderr);
  r = cli(dir, 'mover', 'li-1', 'enviada', '--rodada', '15', '--curriculo', 'PT', '--pretensao', '5200');
  assert.equal(r.status, 0, r.stderr);
  const res = JSON.parse(cli(dir, 'resumo').stdout);
  assert.equal(res.enviadas, 1);
  assert.equal(res.enviadasHoje, 1);
});

test('cli add em lote informa adicionadas, duplicadas e erros sem parar', () => {
  const dir = novoDir();
  const arq = join(dir, 'lote.json');
  writeFileSync(arq, JSON.stringify([
    { id: 'a', empresa: 'A', titulo: 'Java' },
    { id: 'b', empresa: 'A', titulo: 'Java' },
    { id: 'c', empresa: '', titulo: 'X' },
  ]));
  const r = cli(dir, 'add', '--arquivo', arq);
  // processa o lote inteiro, mas sai com 2 para quem chamou perceber a vaga recusada
  assert.equal(r.status, 2, r.stderr);
  assert.match(r.stdout, /adicionadas: 1/);
  assert.match(r.stdout, /duplicadas: 1/);
  assert.match(r.stdout, /com erro: 1/);
});

test('cli recusa pular sem motivo com código 1', () => {
  const dir = novoDir();
  cli(dir, 'add', '--json', JSON.stringify({ id: 'a', empresa: 'A', titulo: 'Java' }));
  const r = cli(dir, 'mover', 'a', 'pulada');
  assert.equal(r.status, 1);
  assert.match(r.stderr, /motivo/);
});

test('cli dup avisa vaga já existente por empresa+título', () => {
  const dir = novoDir();
  cli(dir, 'add', '--json', JSON.stringify({ id: 'li-9', empresa: 'V.tal', titulo: 'Desenvolvedor Full Stack - Pleno' }));
  const r = cli(dir, 'dup', 'V.tal', 'Pessoa Desenvolvedora Full Stack Pleno');
  assert.match(r.stdout, /li-9/);
});

test('cli exportar-md gera histórico com as enviadas', () => {
  const dir = novoDir();
  cli(dir, 'add', '--json', JSON.stringify({ id: 'a', empresa: 'Acme', titulo: 'Java Pleno', plataforma: 'gupy' }));
  cli(dir, 'mover', 'a', 'enviada', '--rodada', '1');
  const r = cli(dir, 'exportar-md');
  assert.match(r.stdout, /Acme/);
  assert.match(r.stdout, /Rodada 1/);
});

test('cli add --forcar inclui vaga com mesmo título e id diferente', () => {
  const dir = novoDir();
  cli(dir, 'add', '--json', JSON.stringify({ id: 'a', empresa: 'DB1', titulo: 'Java Pleno' }));
  const r = cli(dir, 'add', '--forcar', '--json', JSON.stringify({ id: 'b', empresa: 'DB1', titulo: 'Java Pleno' }));
  assert.match(r.stdout, /adicionadas: 1/);
});

test('primeira vez: copia plataformas do modelo só se ainda não existirem', async () => {
  const { prepararPrimeiraVez } = await import('../nucleo/armazenamento.mjs');
  const dir = novoDir();
  assert.equal(prepararPrimeiraVez(dir, join(raiz, 'modelo-dados')), true);
  assert.ok(criarArmazem(dir).carregar().plataformas.some(p => p.id === 'linkedin'));
  assert.equal(prepararPrimeiraVez(dir, join(raiz, 'modelo-dados')), false);
});

test('cli script preenche marcadores com perfil, chave e lista de empresas', () => {
  const dir = novoDir();
  writeFileSync(join(dir, 'perfil.json'), JSON.stringify({ nome: 'Fulano', email: 'f@x.com', telefone: '(11) 98888-7777', linkedin: 'https://l/in/f', pretensaoCLT: 5000, pretensaoPJ: 7000 }));
  writeFileSync(join(dir, '.chave'), 'abc123');
  const ih = cli(dir, 'script', 'inhire-IH');
  assert.equal(ih.status, 0, ih.stderr);
  assert.match(ih.stdout, /name:"Fulano"/);
  assert.match(ih.stdout, /phone:"11988887777"/);
  assert.doesNotMatch(ih.stdout, /\{\{/);
  const bi = cli(dir, 'script', 'busca-inhire', '--dias', '3');
  assert.match(bi.stdout, /CHAVE = "abc123"/);
  assert.match(bi.stdout, /DESDE = "2026-10-26"/);
  assert.match(bi.stdout, /"db1"/);
  const bl = cli(dir, 'script', 'busca-linkedin', '--palavras', '["java","spring"]');
  assert.match(bl.stdout, /PALAVRAS = \["java","spring"\]/);
});

test('cli add --uri aceita lista codificada (plano B dos scripts de busca)', () => {
  const dir = novoDir();
  const r = cli(dir, 'add', '--uri', encodeURIComponent(JSON.stringify([{ id: 'u1', empresa: 'Uri', titulo: 'Java' }])));
  assert.match(r.stdout, /adicionadas: 1/);
});
