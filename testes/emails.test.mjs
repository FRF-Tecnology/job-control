// T1: emails de processo seletivo mudam a vaga sozinhos, sem duplicar.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname } from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { passaPreFiltro, acharVagaDoEmail, transicaoDoEmail, dataDoEmail } from '../nucleo/emails.mjs';
import { criarArmazemMemoria } from '../nucleo/armazenamento.mjs';
import { criarApi, PREFIXO } from '../servidor/api.mjs';

const enviada = (id, empresa, titulo, extra = {}) => ({ id, empresa, titulo, etapa: 'enviada', plataforma: 'linkedin', descobertaEm: '2026-11-01', atualizadoEm: '2026-11-01', envio: { data: '2026-11-01' }, followUp: { proximo: '2026-11-08', feitos: 0 }, ...extra });
const VAGAS = [
  enviada('li-1', 'Oscilar', 'Senior Software Engineer'),
  enviada('li-2', 'Sezzle', 'Backend Engineer'),
  enviada('li-3', 'Zup Innovation', 'Pessoa Desenvolvedora Backend Java Sênior'),
  enviada('li-4', 'BairesDev', 'Senior Java Developer'),
  enviada('li-5', 'BairesDev', 'Senior AI Engineer'),
  { ...enviada('li-6', 'Nortal', 'Java Developer'), etapa: 'pulada', motivo: 'Stack', envio: undefined, followUp: undefined },
];
const email = (idMensagem, de, assunto, texto = '') => ({ idMensagem, de, assunto, texto, data: 'Fri, 07 Nov 2026 14:00:00 +0000' });

test('pré-filtro: ATS e palavras de processo passam; newsletter não', () => {
  assert.ok(passaPreFiltro(email('1', 'no-reply@greenhouse.io', 'Hello')));
  assert.ok(passaPreFiltro(email('2', 'maria@empresa.com', 'Próximas etapas do processo seletivo')));
  assert.ok(passaPreFiltro(email('3', 'rh@x.com', 'Update', 'Unfortunately we decided to move forward with other candidates')));
  assert.ok(!passaPreFiltro(email('4', 'news@loja.com', 'Promoção de sexta')));
});

test('achar a vaga: empresa no assunto ou no domínio; título desempata; empate ou vaga pulada não casa', () => {
  assert.equal(acharVagaDoEmail(VAGAS, email('a', 'no-reply@greenhouse.io', 'Thank you for applying to Oscilar'))?.id, 'li-1');
  assert.equal(acharVagaDoEmail(VAGAS, email('b', 'Talent <talent@sezzle.com>', 'Your application'))?.id, 'li-2');
  assert.equal(acharVagaDoEmail(VAGAS, email('c', 'recrutamento@zup.com.br', 'Atualização', 'Obrigado pelo interesse na Zup'))?.id, 'li-3');
  assert.equal(acharVagaDoEmail(VAGAS, email('d', 'jobs@bairesdev.com', 'Senior AI Engineer — next steps'))?.id, 'li-5');
  assert.equal(acharVagaDoEmail(VAGAS, email('e', 'jobs@bairesdev.com', 'Your application')), null, 'duas vagas BairesDev sem título no email: empate');
  assert.equal(acharVagaDoEmail(VAGAS, email('f', 'jobs@nortal.com', 'Interview')), null, 'vaga pulada não recebe email');
  assert.equal(acharVagaDoEmail(VAGAS, email('g', 'x@y.com', 'oi'), 'li-4')?.id, 'li-4');
  assert.equal(acharVagaDoEmail(VAGAS, email('h', 'jobs@acme.io', 'Application received')), null);
});

test('transições da tabela do backlog; nunca volta etapa nem mexe em vaga encerrada', () => {
  const v = VAGAS[0];
  assert.equal(transicaoDoEmail('confirmacao', v), null);
  assert.equal(transicaoDoEmail('outro', v), null);
  assert.deepEqual(transicaoDoEmail('rejeicao', v), { etapa: 'encerrada', extras: { motivo: 'Reprovada' } });
  assert.deepEqual(transicaoDoEmail('entrevista', v, { assunto: 'Interview invitation' }), { etapa: 'em-processo', extras: { subEtapa: 'entrevista-rh', pendencia: 'Agendar a entrevista' } });
  assert.equal(transicaoDoEmail('entrevista', v, { assunto: 'Technical interview' }).extras.subEtapa, 'entrevista-tecnica');
  assert.deepEqual(transicaoDoEmail('teste', v), { etapa: 'em-processo', extras: { subEtapa: 'teste', pendencia: 'Fazer o teste' } });
  assert.deepEqual(transicaoDoEmail('oferta', v), { etapa: 'oferta', extras: { subEtapa: 'proposta' } });
  assert.equal(transicaoDoEmail('entrevista', { ...v, etapa: 'oferta' }), null);
  assert.equal(transicaoDoEmail('rejeicao', { ...v, etapa: 'encerrada' }), null);
  assert.equal(dataDoEmail('Fri, 07 Nov 2026 14:00:00 +0000', 'x').length, 10);
  assert.equal(dataDoEmail('2026-11-05T10:00:00Z', 'x'), '2026-11-05');
  assert.equal(dataDoEmail(undefined, '2026-11-07'), '2026-11-07');
});

const novaApi = () => {
  const armazem = criarArmazemMemoria({ estadoInicial: { vagas: structuredClone(VAGAS) } });
  const api = criarApi({ armazem, dir: null, hoje: () => '2026-11-07' });
  const chamar = (metodo, caminho, corpo) => api.chamar({ metodo, caminho: PREFIXO + caminho, corpo, autor: 'claude' });
  return { armazem, chamar, vaga: id => armazem.carregar().vagas.find(v => v.id === id) };
};
const item = (e, tipo, extra = {}) => ({ email: e, classificacao: { tipo, vagaId: null, resumo: `${tipo} de ${e.de}`, ...extra } });

test('API: cada tipo muda a vaga certa, registra na linha do tempo e só guarda o resumo', async () => {
  const { armazem, chamar, vaga } = novaApi();
  const rs = (await chamar('POST', '/emails', { lista: [
    item(email('m1', 'no-reply@greenhouse.io', 'Thank you for applying to Oscilar', 'corpo longo que não deve ficar guardado'), 'confirmacao'),
    item(email('m2', 'talent@sezzle.com', 'Your application to Sezzle'), 'rejeicao'),
    item(email('m3', 'recrutamento@zup.com.br', 'Convite para entrevista técnica na Zup'), 'entrevista'),
    item(email('m4', 'no-reply@hackerrank.com', 'BairesDev invited you to a test for Senior AI Engineer'), 'teste'),
    item(email('m5', 'people@bairesdev.com', 'Offer letter – Senior Java Developer'), 'oferta'),
  ] })).corpo;
  assert.deepEqual(rs.map(r => [r.vagaId, r.transicao]), [['li-1', null], ['li-2', 'encerrada'], ['li-3', 'em-processo'], ['li-5', 'em-processo'], ['li-4', 'oferta']]);
  assert.equal(vaga('li-1').etapa, 'enviada');
  assert.deepEqual([vaga('li-2').etapa, vaga('li-2').motivo], ['encerrada', 'Reprovada']);
  assert.deepEqual([vaga('li-3').subEtapa, vaga('li-3').pendencia], ['entrevista-tecnica', 'Agendar a entrevista']);
  assert.equal(vaga('li-5').subEtapa, 'teste');
  assert.equal(vaga('li-4').etapa, 'oferta');
  const e = armazem.carregar();
  assert.equal(e.eventos.filter(x => x.tipo === 'email').length, 5);
  assert.ok(!JSON.stringify(e.emails).includes('corpo longo'), 'o corpo do email não fica na central');
});

test('API: rodar duas vezes não duplica nada', async () => {
  const { armazem, chamar } = novaApi();
  const lista = [item(email('m2', 'talent@sezzle.com', 'Your application to Sezzle'), 'rejeicao')];
  await chamar('POST', '/emails', { lista });
  const antes = armazem.carregar();
  const [r] = (await chamar('POST', '/emails', { lista })).corpo;
  assert.equal(r.status, 'duplicado');
  const depois = armazem.carregar();
  assert.deepEqual([depois.emails.length, depois.eventos.length], [antes.emails.length, antes.eventos.length]);
});

test('API: sem vaga → lista; ligar aplica a mudança; descartar tira; newsletter é ignorada; CPF é recusado', async () => {
  const { chamar, vaga } = novaApi();
  const [semVaga, news] = (await chamar('POST', '/emails', { lista: [
    item(email('m6', 'jobs@bairesdev.com', 'Interview invitation'), 'entrevista'),
    item(email('m7', 'news@loja.com', 'Promoção'), 'outro'),
  ] })).corpo;
  assert.equal(semVaga.status, 'sem-vaga');
  assert.equal(news.status, 'ignorado');
  assert.equal((await chamar('GET', '/emails?status=sem-vaga')).corpo.length, 1);
  const ligado = (await chamar('POST', '/emails/m6/ligar', { vagaId: 'li-4' })).corpo;
  assert.deepEqual([ligado.vagaId, ligado.transicao, vaga('li-4').etapa], ['li-4', 'em-processo', 'em-processo']);
  assert.equal((await chamar('POST', '/emails/m6/ligar', { vagaId: 'li-5' })).status, 400, 'já ligado');
  assert.equal((await chamar('GET', '/emails?status=sem-vaga')).corpo.length, 0);
  await chamar('POST', '/emails', item(email('m8', 'jobs@acme.io', 'Application received'), 'confirmacao'));
  assert.equal((await chamar('POST', '/emails/m8/descartar', {})).corpo.status, 'descartado');
  assert.equal((await chamar('POST', '/emails/nao-existe/descartar', {})).status, 404);
  const cpf = await chamar('POST', '/emails', item(email('m9', 'rh@x.com', 'Documentos'), 'outro', { resumo: 'pediu o CPF 529.982.247-25' }));
  assert.equal(cpf.status, 400);
  assert.equal((await chamar('POST', '/emails', { email: { de: 'x' }, classificacao: { tipo: 'outro' } })).status, 400, 'sem idMensagem');
});

test('CLI: email --arquivo duas vezes; a segunda diz "já registrado"', () => {
  const raiz = join(dirname(fileURLToPath(import.meta.url)), '..');
  const dir = mkdtempSync(join(tmpdir(), 'central-'));
  writeFileSync(join(dir, 'vagas.json'), JSON.stringify([VAGAS[1]]));
  const arq = join(dir, 'lote.json');
  writeFileSync(arq, JSON.stringify([item(email('m2', 'talent@sezzle.com', 'Your application to Sezzle'), 'rejeicao')]));
  const cli = (...args) => spawnSync(process.execPath, [join(raiz, 'ferramentas/cli.mjs'), ...args], { env: { ...process.env, CENTRAL_DADOS: dir, CENTRAL_HOJE: '2026-11-07' }, encoding: 'utf8' });
  const r1 = cli('email', '--arquivo', arq);
  assert.match(r1.stdout, /ligado \| m2 \| li-2 → encerrada/);
  const r2 = cli('email', '--arquivo', arq);
  assert.match(r2.stdout, /já registrado \| m2/);
  assert.match(cli('emails').stdout, /m2 \| 2026-11-07 \| ligado \| rejeicao \| li-2/);
});
