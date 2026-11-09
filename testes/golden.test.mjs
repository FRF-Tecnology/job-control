// O domínio em Go (internal/dominio) é conferido contra respostas gravadas do domínio Node.
// Se alguém mudar nucleo/ sem regravar, este teste avisa: rode node ferramentas/gerar-golden-dominio.mjs.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, readFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const raiz = join(dirname(fileURLToPath(import.meta.url)), '..');

for (const [nome, gerador, arquivo, pacote] of [
  ['domínio (nucleo/)', 'gerar-golden-dominio.mjs', 'internal/dominio/testdata/golden.json', './internal/dominio/'],
  ['API (servidor/api.mjs)', 'gerar-golden-api.mjs', 'internal/api/testdata/golden-api.json', './internal/api/'],
]) {
  test(`golden do ${nome} está em dia`, () => {
    const saida = join(mkdtempSync(join(tmpdir(), 'golden-')), 'golden.json');
    const r = spawnSync(process.execPath, [join(raiz, 'ferramentas', gerador)], { env: { ...process.env, GOLDEN_SAIDA: saida }, encoding: 'utf8' });
    assert.equal(r.status, 0, r.stderr);
    const atual = readFileSync(join(raiz, arquivo), 'utf8');
    assert.ok(readFileSync(saida, 'utf8') === atual, `golden desatualizado: rode node ferramentas/${gerador} e confira go test ${pacote}`);
  });
}
