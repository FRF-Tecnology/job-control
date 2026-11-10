// Caixa "O que vamos fazer hoje?": com IA conectada, Enter ou uma sugestão cria o pedido e já o entrega para a IA ler
// (aoCriar com o id), sem o aviso "Anotado em Seus pedidos" nem um segundo clique. Sem IA, o pedido só fica anotado.
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, type App } from '../app/contexto';
import { CaixaPedido } from './Hoje';

afterEach(() => { cleanup(); location.hash = ''; });

function montar({ ia = true, falha = false } = {}) {
  const chamadas: { caminho: string; metodo?: string; corpo?: unknown }[] = [];
  const avisos: string[] = [];
  const aoCriar = vi.fn();
  const api = vi.fn((caminho: string, metodo?: string, corpo?: unknown) => {
    chamadas.push({ caminho, metodo, corpo });
    return falha ? Promise.reject(new Error('escreva o pedido')) : Promise.resolve({ id: 'pd-novo-1', status: 'aberto' });
  });
  const app = {
    api, aviso: (t: string) => { avisos.push(t); },
    agir: async (fn: () => Promise<unknown>, mensagem?: string) => {
      try { await fn(); if (mensagem) avisos.push(mensagem); return true; } catch (e) { avisos.push((e as Error).message); return false; }
    },
  } as unknown as App;
  render(<Contexto.Provider value={app}><CaixaPedido sugestoes={['Buscar vagas USD LATAM', 'Quem me respondeu?']} iaConectada={ia} aoCriar={aoCriar} /></Contexto.Provider>);
  return { chamadas, avisos, aoCriar };
}
const caixa = () => screen.getByRole('textbox', { name: 'O que vamos fazer hoje?' }) as HTMLInputElement;
const enviar = async (texto: string) => {
  fireEvent.change(caixa(), { target: { value: texto } });
  await act(async () => { fireEvent.submit(caixa().closest('form')!); });
};
const CRIAR = (texto: string) => ({ caminho: '/api/v1/pedidos', metodo: 'POST', corpo: { texto } });

describe('Caixa de pedido', () => {
  it('com IA, Enter cria o pedido e entrega o id para a IA ler, sem "Anotado"', async () => {
    const { chamadas, avisos, aoCriar } = montar();
    await enviar('  Adiciona a vaga https://vagas.exemplo.com/1  ');
    expect(chamadas).toEqual([CRIAR('Adiciona a vaga https://vagas.exemplo.com/1')]);
    expect(aoCriar).toHaveBeenCalledWith('pd-novo-1');
    expect(avisos).toEqual([]);
    expect(caixa().value).toBe('');
  });

  it('com IA, a sugestão faz o mesmo que o Enter', async () => {
    const { chamadas, aoCriar } = montar();
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Buscar vagas USD LATAM' })); });
    expect(chamadas).toEqual([CRIAR('Buscar vagas USD LATAM')]);
    expect(aoCriar).toHaveBeenCalledWith('pd-novo-1');
  });

  it('sem IA, o pedido só fica anotado', async () => {
    const { chamadas, avisos, aoCriar } = montar({ ia: false });
    await enviar('Buscar vagas novas');
    expect(chamadas).toEqual([CRIAR('Buscar vagas novas')]);
    expect(avisos).toEqual(['Anotado em Seus pedidos, logo abaixo.']);
    expect(aoCriar).not.toHaveBeenCalled();
  });

  it('se o pedido não foi criado, nada vai para a IA e o texto fica na caixa', async () => {
    const { aoCriar } = montar({ falha: true });
    await enviar('Buscar vagas novas');
    expect(aoCriar).not.toHaveBeenCalled();
    expect(caixa().value).toBe('Buscar vagas novas');
  });

  it('o que a tela resolve sozinha continua sendo atalho, sem pedido', async () => {
    const { chamadas, aoCriar } = montar();
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Quem me respondeu?' })); });
    expect(location.hash).toBe('#/quadro');
    expect(chamadas).toEqual([]);
    expect(aoCriar).not.toHaveBeenCalled();
  });
});
