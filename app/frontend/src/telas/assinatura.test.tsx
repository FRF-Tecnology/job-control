// Tela da assinatura (Task 7): a situação, Assinar (abre o Checkout e confere sozinho), Gerenciar e as notas.
// Só dados fictícios.
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, ErroApi, type App } from '../app/contexto';
import type { Situacao } from '../app/conta';
import { TelaAssinatura } from './Assinatura';

afterEach(cleanup);

const TESTE: Situacao = { ativa: true, entrada: true, email: 'ana@exemplo.com', estado: 'teste', teste_fim: '2026-11-12T10:00:00Z', periodo_fim: null, motivo: '' };
const ASSINANTE: Situacao = { ...TESTE, estado: 'assinante', teste_fim: null, periodo_fim: '2026-12-11T10:00:00Z' };

function montar(conta: Situacao, { notas = [] as any[], falhaGet = null as Error | null } = {}) {
  const chamadas: { rota: string; corpo: any }[] = [];
  const releConta = vi.fn(async () => {});
  const app = {
    estado: {}, conta, podeAgir: !conta.motivo, releConta, aviso: () => {},
    api: vi.fn(async (caminho: string, metodo = 'GET', corpo?: any) => {
      chamadas.push({ rota: `${metodo} ${caminho}`, corpo });
      if (caminho === '/api/v1/assinatura') { if (falhaGet) throw falhaGet; return { situacao: conta, notas }; }
      return { mensagem: metodo === 'POST' && caminho.endsWith('/assinar') ? 'Conclua o pagamento no navegador; o app confere sozinho' : 'Gerencie a assinatura no navegador' };
    }),
  } as unknown as App;
  render(<Contexto.Provider value={app}><TelaAssinatura /></Contexto.Provider>);
  return { chamadas, releConta };
}
const clicar = async (nome: string) => { await act(async () => { fireEvent.click(screen.getByRole('button', { name: nome })); }); };

describe('Assinatura', () => {
  it('no teste: Assinar abre o Checkout e passa a conferir a conta', async () => {
    const { chamadas, releConta } = montar(TESTE);
    expect(screen.getByRole('heading', { name: 'Assinatura' })).toBeTruthy();
    expect(screen.getByText(/Teste grátis/)).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Gerenciar assinatura' })).toBeNull();
    await clicar('Assinar');
    expect(chamadas.some(c => c.rota === 'POST /api/v1/assinatura/assinar' && JSON.stringify(c.corpo) === '{}')).toBe(true);
    expect(screen.getByText('Conclua o pagamento no navegador; o app confere sozinho')).toBeTruthy();
    expect(releConta).toHaveBeenCalled();
  });

  it('assinante: Gerenciar e as notas fiscais', async () => {
    const { chamadas } = montar(ASSINANTE, { notas: [{ numero: '42', link_pdf: 'https://exemplo.com/nota-42.pdf', estado: 'emitida', criada_em: '2026-11-11T10:00:00Z' }] });
    await waitFor(() => expect(screen.getByRole('link', { name: /Nota 42/ }).getAttribute('href')).toBe('https://exemplo.com/nota-42.pdf'));
    expect(screen.queryByRole('button', { name: 'Assinar' })).toBeNull();
    await clicar('Gerenciar assinatura');
    expect(chamadas.some(c => c.rota === 'POST /api/v1/assinatura/gerenciar')).toBe(true);
  });

  it('sem internet: as notas não aparecem, mas a tela explica', async () => {
    montar(TESTE, { falhaGet: new ErroApi('Sem conexão com a internet', 503, { erro: 'Sem conexão com a internet', motivo: 'sem-internet' }) });
    await waitFor(() => expect(screen.getByRole('alert').textContent).toContain('Sem conexão com a internet'));
    expect(screen.getByRole('button', { name: 'Assinar' })).toBeTruthy();
  });
});
