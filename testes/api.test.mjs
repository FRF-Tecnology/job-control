// T0: a API v1 é a única porta de entrada e não depende de onde os dados moram.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { criarArmazemArquivos, criarArmazemMemoria, criarArmazemPor } from '../nucleo/armazenamento.mjs';
import { criarApi, PREFIXO } from '../servidor/api.mjs';

const hoje = () => '2026-11-07';
const novaApi = armazem => {
  const api = criarApi({ armazem, dir: null, hoje });
  return (metodo, caminho, corpo, autor = 'claude') => api.chamar({ metodo, caminho: PREFIXO + caminho, corpo, autor });
};
const vaga = (id, extra = {}) => ({ id, empresa: `Empresa ${id}`, titulo: 'Backend Java', link: `https://x.com/${id}`, ...extra });

// Roteiro com tudo que a CLI faz. Devolve só o que importa (sem ids aleatórios de evento).
async function roteiro(chamar) {
  const passos = [];
  const anotar = (nome, r) => passos.push([nome, r.status, JSON.stringify(r.corpo, (k, v) => (k === 'id' && String(v).startsWith('ev-') ? 'ev' : v))]);
  anotar('add lote', await chamar('POST', '/vagas', { lista: [vaga('a'), vaga('b'), vaga('a'), { id: 'sem-empresa' }] }));
  anotar('add um', await chamar('POST', '/vagas', vaga('c')));
  anotar('add dup', await chamar('POST', '/vagas', vaga('c')));
  anotar('mover', await chamar('POST', '/vagas/a/mover', { etapa: 'enviada', envio: { rodada: 3, curriculo: 'EN' } }));
  anotar('mover sem motivo', await chamar('POST', '/vagas/b/mover', { etapa: 'pulada' }));
  anotar('editar', await chamar('PATCH', '/vagas/b', { notas: 'ok', aderencia: 'A' }));
  anotar('evento', await chamar('POST', '/vagas/b/eventos', { tipo: 'nota', texto: 'oi' }));
  anotar('followup', await chamar('POST', '/vagas/a/followup', { acao: 'adiar', dias: 2 }));
  anotar('perfil', await chamar('PATCH', '/perfil', { nome: 'V', filtros: { niveis: ['Sênior'] }, respostasPadrao: [{ pergunta: 'P', resposta: 'R' }] }));
  anotar('perfil merge', await chamar('PATCH', '/perfil', { cidade: 'X', respostasPadrao: [{ pergunta: 'p', resposta: 'R2' }] }));
  anotar('rodada', await chamar('PUT', '/rodadas/3', { resumo: 'r3', fontes: ['linkedin'] }));
  anotar('plataforma', await chamar('PATCH', '/plataformas/gupy', { status: 'ok' }));
  anotar('importar', await chamar('POST', '/vagas/importar', [vaga('d', { etapa: 'enviada', envio: { data: '2026-01-01' } })]));
  for (const c of ['/resumo', '/vagas?etapa=enviada', '/vagas/a', '/duplicatas?empresa=Empresa%20b&titulo=Backend%20Java', '/followups', '/perfil', '/rodadas', '/plataformas', '/validar', '/historico.md']) {
    anotar(c, await chamar('GET', c));
  }
  return passos;
}

test('trocar o armazém (arquivos ↔ memória) não muda nenhuma resposta da API', async () => {
  const arquivos = await roteiro(novaApi(criarArmazemArquivos(mkdtempSync(join(tmpdir(), 'central-')), { hoje })));
  const memoria = await roteiro(novaApi(criarArmazemMemoria()));
  assert.ok(arquivos.length > 20);
  for (let i = 0; i < arquivos.length; i++) assert.deepEqual(memoria[i], arquivos[i], `passo ${arquivos[i][0]}`);
});

test('criarArmazemPor escolhe a implementação e recusa tipo desconhecido', () => {
  assert.equal(criarArmazemPor({ tipo: 'memoria' }).tipo, 'memoria');
  assert.equal(criarArmazemPor({ dir: mkdtempSync(join(tmpdir(), 'central-')) }).tipo, 'arquivos');
  assert.throws(() => criarArmazemPor({ tipo: 'postgres' }), /desconhecido/);
});

test('rotas da API v1: status e regras', async () => {
  const chamar = novaApi(criarArmazemMemoria());
  assert.equal((await chamar('POST', '/vagas', vaga('a'))).status, 201);
  assert.equal((await chamar('POST', '/vagas', vaga('a'))).status, 409);
  assert.equal((await chamar('POST', '/vagas/a/mover', { etapa: 'pulada' })).status, 400);
  assert.equal((await chamar('GET', '/vagas/nao-existe')).status, 404);
  assert.equal((await chamar('GET', '/nada')).status, 404);
  assert.equal((await chamar('PATCH', '/perfil', { notas: 'CPF 123.456.789-09' })).status, 400);
  assert.equal((await chamar('PUT', '/rodadas/4', { resumo: 'RG 12.345.678-9' })).status, 400);
  assert.equal((await chamar('POST', '/vagas/a/followup', { acao: 'x' })).status, 400);
  const perfil = (await chamar('PATCH', '/perfil', { nome: 'A', respostasPadrao: [{ pergunta: 'Q', resposta: '1' }] })).corpo;
  const mesclado = (await chamar('PATCH', '/perfil', { respostasPadrao: [{ pergunta: 'q', resposta: '2' }, { pergunta: 'z', resposta: '3' }] })).corpo;
  assert.equal(perfil.nome, mesclado.nome);
  assert.deepEqual(mesclado.respostasPadrao.map(r => r.resposta), ['2', '3']);
  const imp = (await chamar('POST', '/vagas/importar', [vaga('b', { etapa: 'oferta' })])).corpo;
  assert.deepEqual(imp.adicionadas, ['b']);
  assert.equal((await chamar('GET', '/vagas/b')).corpo.etapa, 'descoberta');
});

test('dono dos dados: vaga e evento ganham contaId da sessão, nunca do corpo', async () => {
  const armazem = criarArmazemMemoria();
  const api = criarApi({ armazem, dir: null, hoje, contaId: 'conta-1' });
  await api.chamar({ metodo: 'POST', caminho: `${PREFIXO}/vagas`, corpo: vaga('a', { contaId: 'outra-conta' }), autor: 'eu' });
  const e = armazem.carregar();
  assert.equal(e.vagas[0].contaId, 'conta-1');
  assert.ok(e.eventos.every(ev => ev.contaId === 'conta-1'));
  // Modo local: sem conta informada, é "local".
  const local = criarArmazemMemoria();
  await criarApi({ armazem: local, dir: null, hoje }).chamar({ metodo: 'POST', caminho: `${PREFIXO}/vagas`, corpo: vaga('a') });
  assert.equal(local.carregar().vagas[0].contaId, 'local');
});
