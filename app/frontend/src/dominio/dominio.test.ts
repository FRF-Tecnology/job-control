// As funções de domínio que as telas usam são o mesmo código de nucleo/ (Node). Aqui elas são
// conferidas contra as respostas gravadas do Node (internal/dominio/testdata/golden.json),
// o mesmo arquivo que o domínio em Go usa.
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import * as modelo from './modelo';
import * as funil from './funil';
import * as followup from './followup';

const golden = JSON.parse(readFileSync(resolve(process.cwd(), '../../internal/dominio/testdata/golden.json'), 'utf8'));
const F: Record<string, (...a: any[]) => any> = { ...modelo, ...funil, ...followup } as any;
const usadas = ['normalizar', 'somarDias', 'ehDataISO', 'mercadoDe', 'noFoco', 'foiEnviada', 'lerValor', 'teveRetorno', 'resumo', 'funil', 'porPlataforma', 'porSemana', 'motivosPulo', 'pendentesFollowUp', 'sugerirMensagem'];

describe('domínio das telas = domínio Node', () => {
  const casos = golden.casos.filter((c: any) => usadas.includes(c.funcao));
  it('tem casos para todas as funções usadas', () => {
    for (const f of usadas) expect(casos.some((c: any) => c.funcao === f), f).toBe(true);
  });
  for (const c of casos) {
    it(`${c.funcao}: ${c.nome}`, () => {
      const r = JSON.parse(JSON.stringify(F[c.funcao](...structuredClone(c.args)) ?? null));
      expect(r).toEqual(c.resultado);
    });
  }
});
