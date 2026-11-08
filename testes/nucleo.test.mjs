import { test } from 'node:test';
import assert from 'node:assert/strict';
import { normalizar, somarDias, ETAPAS } from '../nucleo/modelo.mjs';
import { contemDocumento, validarVaga } from '../nucleo/validacao.mjs';
import { chaveVaga, acharDuplicata } from '../nucleo/dedupe.mjs';
import { resumo, funil, porPlataforma, motivosPulo, porSemana } from '../nucleo/funil.mjs';
import { pendentesFollowUp, sugerirMensagem } from '../nucleo/followup.mjs';
import { estadoVazio, adicionarVaga, moverVaga, followUpFeito, adiarFollowUp, atualizarVaga, registrarEvento, ErroValidacao, ErroDuplicata } from '../nucleo/servico.mjs';

const ctx = (agora = '2026-10-29') => ({ autor: 'claude', agora, diasFollowUp: 7 });
const base = (extra = {}) => ({ id: 'li-1', empresa: 'Compass UOL', titulo: 'Desenvolvedor Quarkus', plataforma: 'linkedin', etapa: 'descoberta', ...extra });

// modelo
test('normalizar tira acento e caixa', () => {
  assert.equal(normalizar('  São PAULO  '), 'sao paulo');
});
test('somarDias atravessa o mês', () => {
  assert.equal(somarDias('2026-10-29', 7), '2026-11-05');
});
test('etapas cobrem o funil do plano', () => {
  for (const e of ['descoberta', 'aplicar', 'enviada', 'bloqueada', 'em-processo', 'oferta', 'encerrada', 'pulada']) assert.ok(ETAPAS.includes(e), e);
});

// validação
test('CPF formatado é detectado', () => {
  assert.equal(contemDocumento('meu cpf 123.456.789-09'), true);
});
test('CPF só com números e dígito válido é detectado', () => {
  assert.equal(contemDocumento('52998224725'), true);
});
test('telefone e ids de vaga não são confundidos com CPF', () => {
  assert.equal(contemDocumento('84996617957'), false);
  assert.equal(contemDocumento('4484521064'), false);
});
test('palavra RG seguida de número é detectada', () => {
  assert.equal(contemDocumento('RG: 1234567'), true);
});
test('vaga válida não tem erros', () => {
  assert.deepEqual(validarVaga(base()), []);
});
test('pular sem motivo é recusado', () => {
  assert.ok(validarVaga(base({ etapa: 'pulada' })).some(e => /motivo/i.test(e)));
});
test('bloquear sem pendência é recusado', () => {
  assert.ok(validarVaga(base({ etapa: 'bloqueada' })).some(e => /pend/i.test(e)));
});
test('etapa desconhecida é recusada', () => {
  assert.ok(validarVaga(base({ etapa: 'xyz' })).some(e => /etapa/i.test(e)));
});
test('nota com CPF é recusada', () => {
  assert.ok(validarVaga(base({ notas: 'cpf 123.456.789-09' })).some(e => /CPF|documento/i.test(e)));
});

// dedupe
test('chave ignora "Pessoa Desenvolvedora", acento e pontuação', () => {
  assert.equal(chaveVaga('V.tal', 'Pessoa Desenvolvedora Full Stack - Pleno'), chaveVaga('V.tal', 'Desenvolvedor Full Stack Pleno'));
});
test('acha duplicata por id e por empresa+título', () => {
  const vagas = [base()];
  assert.equal(acharDuplicata(vagas, base({ titulo: 'outra' }))?.id, 'li-1');
  assert.equal(acharDuplicata(vagas, base({ id: 'gupy-9', titulo: 'Desenvolvedor(a) Quarkus' }))?.id, 'li-1');
  assert.equal(acharDuplicata(vagas, base({ id: 'gupy-9', empresa: 'Outra' })), null);
});

// serviço
test('adicionar grava a vaga e um evento de descoberta', () => {
  const { estado } = adicionarVaga(estadoVazio(), base(), ctx());
  assert.equal(estado.vagas.length, 1);
  assert.equal(estado.vagas[0].descobertaEm, '2026-10-29');
  assert.equal(estado.eventos[0].tipo, 'descoberta');
});
test('adicionar duplicada lança ErroDuplicata', () => {
  const { estado } = adicionarVaga(estadoVazio(), base(), ctx());
  assert.throws(() => adicionarVaga(estado, base({ id: 'x-2' }), ctx()), ErroDuplicata);
});
test('adicionar inválida lança ErroValidacao com a lista de erros', () => {
  assert.throws(() => adicionarVaga(estadoVazio(), base({ empresa: '' }), ctx()), e => e instanceof ErroValidacao && e.erros.length > 0);
});
test('mover para enviada grava envio e agenda follow-up em +7 dias', () => {
  let { estado } = adicionarVaga(estadoVazio(), base(), ctx());
  ({ estado } = moverVaga(estado, 'li-1', 'enviada', { envio: { rodada: 15, curriculo: 'PT', pretensao: 5200 } }, ctx()));
  const v = estado.vagas[0];
  assert.equal(v.etapa, 'enviada');
  assert.equal(v.envio.data, '2026-10-29');
  assert.equal(v.followUp.proximo, '2026-11-05');
  assert.equal(estado.eventos.at(-1).tipo, 'enviada');
});
test('mover para pulada sem motivo não altera o estado', () => {
  const { estado } = adicionarVaga(estadoVazio(), base(), ctx());
  assert.throws(() => moverVaga(estado, 'li-1', 'pulada', {}, ctx()), ErroValidacao);
  assert.equal(estado.vagas[0].etapa, 'descoberta');
});
test('mover vaga inexistente avisa', () => {
  assert.throws(() => moverVaga(estadoVazio(), 'nao-existe', 'aplicar', {}, ctx()), /não encontrada/);
});
test('sair de bloqueada limpa a pendência', () => {
  let { estado } = adicionarVaga(estadoVazio(), base(), ctx());
  ({ estado } = moverVaga(estado, 'li-1', 'bloqueada', { pendencia: 'digitar CPF' }, ctx()));
  ({ estado } = moverVaga(estado, 'li-1', 'enviada', {}, ctx()));
  assert.equal(estado.vagas[0].pendencia, null);
});
test('follow-up feito conta e reagenda; adiar empurra a data', () => {
  let { estado } = adicionarVaga(estadoVazio(), base(), ctx());
  ({ estado } = moverVaga(estado, 'li-1', 'enviada', {}, ctx()));
  ({ estado } = followUpFeito(estado, 'li-1', ctx('2026-11-05')));
  assert.equal(estado.vagas[0].followUp.feitos, 1);
  assert.equal(estado.vagas[0].followUp.proximo, '2026-11-12');
  ({ estado } = adiarFollowUp(estado, 'li-1', 3, ctx('2026-11-12')));
  assert.equal(estado.vagas[0].followUp.proximo, '2026-11-15');
});
test('atualizar só aceita campos editáveis', () => {
  const { estado } = adicionarVaga(estadoVazio(), base(), ctx());
  const { estado: e2 } = atualizarVaga(estado, 'li-1', { notas: 'falar com recrutador', etapa: 'oferta' }, ctx());
  assert.equal(e2.vagas[0].notas, 'falar com recrutador');
  assert.equal(e2.vagas[0].etapa, 'descoberta');
});
test('evento de nota com CPF é recusado', () => {
  const { estado } = adicionarVaga(estadoVazio(), base(), ctx());
  assert.throws(() => registrarEvento(estado, 'li-1', 'nota', 'cpf 123.456.789-09', ctx()), ErroValidacao);
});
test('operações não mutam o estado recebido', () => {
  const { estado } = adicionarVaga(estadoVazio(), base(), ctx());
  const antes = JSON.stringify(estado);
  moverVaga(estado, 'li-1', 'aplicar', {}, ctx());
  assert.equal(JSON.stringify(estado), antes);
});

// funil e follow-up
const amostra = () => [
  base({ id: 'a', etapa: 'enviada', plataforma: 'linkedin', envio: { data: '2026-10-28' }, followUp: { proximo: '2026-10-29', feitos: 0 } }),
  base({ id: 'b', empresa: 'B', etapa: 'enviada', plataforma: 'gupy', envio: { data: '2026-10-20' }, followUp: { proximo: '2026-11-30', feitos: 0 } }),
  base({ id: 'c', empresa: 'C', etapa: 'em-processo', plataforma: 'gupy', envio: { data: '2026-10-21' } }),
  base({ id: 'd', empresa: 'D', etapa: 'encerrada', motivo: 'Reprovada', plataforma: 'gupy', envio: { data: '2026-10-22' } }),
  base({ id: 'e', empresa: 'E', etapa: 'pulada', motivo: 'Sênior' }),
  base({ id: 'f', empresa: 'F', etapa: 'bloqueada', pendencia: 'CPF' }),
  base({ id: 'g', empresa: 'G', etapa: 'descoberta' }),
];
test('resumo conta enviadas pelo envio e calcula a taxa de retorno', () => {
  const r = resumo(amostra(), '2026-10-29');
  assert.equal(r.enviadas, 4);
  assert.equal(r.retornos, 2);
  assert.equal(r.taxaRetorno, 50);
  assert.equal(r.bloqueadas, 1);
  assert.equal(r.descoberta, 1);
  assert.equal(r.followUps, 1);
});
test('resumo de lista vazia não divide por zero', () => {
  assert.equal(resumo([], '2026-10-29').taxaRetorno, 0);
});
test('funil começa nas enviadas', () => {
  assert.deepEqual(funil(amostra()).map(f => f.total), [4, 2, 1, 0]);
});
test('por plataforma soma enviadas e retornos', () => {
  const g = porPlataforma(amostra()).find(p => p.plataforma === 'gupy');
  assert.deepEqual([g.enviadas, g.retornos], [3, 2]);
});
test('motivos de pulo agrupam e ordenam', () => {
  assert.deepEqual(motivosPulo(amostra()), [{ motivo: 'Sênior', total: 1 }]);
});
test('por semana agrupa pela segunda-feira', () => {
  assert.deepEqual(porSemana(amostra()), [{ semana: '2026-10-19', total: 3 }, { semana: '2026-10-26', total: 1 }]);
});
test('follow-ups pendentes: só etapa enviada com data vencida', () => {
  assert.deepEqual(pendentesFollowUp(amostra(), '2026-10-29').map(v => v.id), ['a']);
});
test('mensagem de follow-up cita empresa e vaga, em inglês quando pedido', () => {
  const v = base();
  assert.match(sugerirMensagem(v, { nome: 'Aldenor' }), /Compass UOL/);
  assert.match(sugerirMensagem({ ...v, idioma: 'en' }, { nome: 'Aldenor' }), /Hi|Hello/);
});
test('mensagem de follow-up se apresenta só com o perfil da pessoa', () => {
  const v = base();
  const front = { nome: 'Ana', cargo: 'Desenvolvedora front-end React', stackPrincipal: ['React', 'TypeScript'] };
  const pt = sugerirMensagem(v, front);
  assert.match(pt, /Atuo como desenvolvedora front-end React/);
  assert.doesNotMatch(pt, /Java|Spring|Kafka|Obrigado/);
  assert.match(sugerirMensagem({ ...v, idioma: 'en' }, front), /I work with React and TypeScript/);
  // sem cargo nem stack: nenhuma apresentação inventada
  const vazio = sugerirMensagem(v, {});
  assert.doesNotMatch(vazio, /Atuo|Trabalho com|Java/);
  assert.doesNotMatch(sugerirMensagem({ ...v, idioma: 'en' }, {}), /I work/);
});

test('forçar aceita mesmo empresa+título quando o id é outro (vaga republicada)', () => {
  const { estado } = adicionarVaga(estadoVazio(), base(), ctx());
  const { estado: e2 } = adicionarVaga(estado, base({ id: 'inhire-novo' }), ctx(), { forcar: true });
  assert.equal(e2.vagas.length, 2);
});
test('forçar nunca aceita o mesmo id', () => {
  const { estado } = adicionarVaga(estadoVazio(), base(), ctx());
  assert.throws(() => adicionarVaga(estado, base(), ctx(), { forcar: true }), ErroDuplicata);
});

test('evento inicial usa a data real do envio ou da descoberta', () => {
  const { estado } = adicionarVaga(estadoVazio(), base({ etapa: 'enviada', descobertaEm: '2026-10-20', envio: { data: '2026-10-26' } }), ctx());
  assert.equal(estado.eventos[0].data, '2026-10-26');
  assert.equal(estado.eventos[0].tipo, 'enviada');
});
