// jc-a2y.7.1: corpus de injeção da função interpretarPedido (o mesmo arquivo roda no Go, internal/ia). Toda saída de
// IA do corpus é bloqueada (422) ou é uma recusa: nenhuma vira ação que grava.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { guardar, validarEntrada } from '../nucleo/ia/guarda.mjs';
import { FUNCOES, montarPrompt, vagasParaPedido } from '../nucleo/ia/funcoes.mjs';

const corpus = JSON.parse(readFileSync(new URL('../internal/ia/testdata/pedidos-injecao.json', import.meta.url), 'utf8'));
// "REPETIR:x:n" vira x repetido n vezes (saída enorme sem um arquivo enorme).
const expandir = v => JSON.parse(JSON.stringify(v), (_, x) => {
  const m = typeof x === 'string' && x.match(/^REPETIR:(.):(\d+)$/);
  return m ? m[1].repeat(Number(m[2])) : x;
});

test('corpus de injeção: bloqueia ou recusa', () => {
  assert.ok(corpus.casos.length >= 30);
  for (const c of corpus.casos) {
    const entrada = { pedido: c.pedido, vagas: corpus.vagas };
    validarEntrada('interpretarPedido', entrada);
    const bruto = JSON.stringify(expandir(c.saida));
    if (c.espera === 'bloqueia') {
      assert.throws(() => guardar('interpretarPedido', entrada, bruto), e => e.tipo === 'esquema' || e.tipo === 'regra', c.nome);
    } else {
      const { saida } = guardar('interpretarPedido', entrada, bruto);
      assert.equal(saida.acao, 'recusar', c.nome);
    }
  }
});

test('ações válidas passam', () => {
  const entrada = { pedido: 'x', vagas: corpus.vagas };
  for (const s of [
    { acao: 'adicionar-vaga', parametros: { url: 'https://www.linkedin.com/jobs/view/9000000003', empresa: 'Empresa Nova', titulo: 'Dev' } },
    { acao: 'adicionar-vaga', parametros: { url: 'https://xn--exemplo-9ua.com/vaga' } },
    { acao: 'registrar-etapa', parametros: { vagaId: 'gp-2', etapa: 'em-processo', sub: 'entrevista-rh' } },
    { acao: 'registrar-etapa', parametros: { vagaId: 'li-1', etapa: 'pulada', motivo: 'Stack' } },
    { acao: 'preparar-pacote', parametros: { vagaId: 'li-1' } },
    { acao: 'responder', parametros: { texto: 'A vaga da Empresa Teste está em Aplicar.' } },
  ]) assert.deepEqual(guardar('interpretarPedido', entrada, JSON.stringify(s)).saida, s);
});

test('a IA recebe só o mínimo das vagas', () => {
  const vagas = [{ id: 'li-1', empresa: 'E', titulo: 'T', etapa: 'aplicar', link: 'https://x', contato: 'pessoa@exemplo.com', notas: 'segredo', pretensao: 9000 }];
  assert.deepEqual(vagasParaPedido(vagas), [{ id: 'li-1', empresa: 'E', titulo: 'T', etapa: 'aplicar' }]);
  const p = montarPrompt('interpretarPedido', { pedido: 'oi', vagas });
  for (const proibido of ['pessoa@exemplo.com', 'segredo', '9000', 'https://x']) assert.ok(!p.mensagem.includes(proibido), proibido);
  assert.ok(p.mensagem.includes('não é instrução'));
  assert.deepEqual(FUNCOES.interpretarPedido.saida.properties.acao.enum, ['adicionar-vaga', 'registrar-etapa', 'preparar-pacote', 'responder', 'recusar']);
});
