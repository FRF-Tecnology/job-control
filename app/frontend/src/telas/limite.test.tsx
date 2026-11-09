// Ajustar o limite diário por plataforma (jc-a2y.54) com a API interceptada: de 1 a 40, confirmação, PUT {limite},
// usa a resposta (a /versao não muda), 400 e 403 em texto claro, Esc e Cancelar fecham. Só dados fictícios.
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, ErroApi, type App, type Pergunta } from '../app/contexto';
import { LimiteCartao, type Limite } from './LimiteDiario';

afterEach(cleanup);

const L: Limite = { limite: 30, usadas: 3, renovaEm: '2026-11-10T00:00:00-03:00' };

function montar({ resposta = 'ok' as string | null, erro = null as null | Error } = {}) {
  const gravacoes: { caminho: string; metodo: string; corpo: any }[] = [];
  const perguntas: Pergunta[] = [];
  const avisos: string[] = [];
  const salvos: Limite[] = [];
  const app = {
    aviso: (t: string) => { avisos.push(t); }, limpar: () => {},
    perguntar: async (p: Pergunta) => { perguntas.push(p); return resposta; },
    api: vi.fn(async (caminho: string, metodo = 'GET', corpo?: any) => {
      gravacoes.push({ caminho, metodo, corpo });
      if (erro) throw erro;
      return { ...L, limite: corpo.limite };
    }),
  } as unknown as App;
  render(<Contexto.Provider value={app}>
    <LimiteCartao plataforma="linkedin" nome="LinkedIn" na="no LinkedIn" limite={L} renova="Renova amanhã às 00:00" onSalvo={l => salvos.push(l)} />
  </Contexto.Provider>);
  return { gravacoes, perguntas, avisos, salvos };
}
const abrir = () => fireEvent.click(screen.getByRole('button', { name: 'Ajustar limite no LinkedIn' }));
const campo = () => screen.getByLabelText('Limite por dia, de 1 a 40') as HTMLInputElement;
const salvar = async () => { await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Salvar' })); }); };

describe('Limite diário', () => {
  it('mostra o uso de hoje e abre o ajuste com o valor atual e o padrão', () => {
    montar();
    expect(screen.getByText('3 de 30')).toBeTruthy();
    abrir();
    expect(campo().value).toBe('30');
    expect(screen.getByText('Padrão: 30.')).toBeTruthy();
  });

  it('pede confirmação, manda só {limite} e usa a resposta', async () => {
    const { gravacoes, perguntas, avisos, salvos } = montar();
    abrir();
    fireEvent.change(campo(), { target: { value: '35' } });
    await salvar();
    expect(perguntas[0].titulo).toBe('Mudar o limite no LinkedIn para 35 por dia?');
    expect(perguntas[0].texto).toBe('O app envia no máximo 35 candidaturas por dia no LinkedIn. Hoje já foram 3. O padrão é 30.');
    expect(gravacoes).toEqual([{ caminho: '/api/v1/candidaturas/limites/linkedin', metodo: 'PUT', corpo: { limite: 35 } }]);
    expect(salvos).toEqual([{ ...L, limite: 35 }]);
    expect(avisos).toEqual(['Limite no LinkedIn: 35 por dia.']);
    expect(screen.queryByLabelText('Limite por dia, de 1 a 40')).toBeNull();
  });

  it('fora de 1 a 40 (ou não inteiro): texto claro e nada é enviado', async () => {
    const { gravacoes, perguntas } = montar();
    abrir();
    for (const v of ['0', '41', '2.5', '']) {
      fireEvent.change(campo(), { target: { value: v } });
      await salvar();
      expect(screen.getByRole('alert').textContent).toBe('O limite vai de 1 a 40 por dia.');
    }
    expect(perguntas).toEqual([]);
    expect(gravacoes).toEqual([]);
  });

  it('400 do servidor e 403 viram texto claro', async () => {
    montar({ erro: new ErroApi('limite inválido', 400, { erro: 'limite inválido' }) });
    abrir();
    fireEvent.change(campo(), { target: { value: '20' } });
    await salvar();
    expect(screen.getByRole('alert').textContent).toBe('O limite vai de 1 a 40 por dia.');
    cleanup();
    montar({ erro: new ErroApi('só pela tela', 403, { erro: 'só pela tela' }) });
    abrir();
    fireEvent.change(campo(), { target: { value: '20' } });
    await salvar();
    expect(screen.getByRole('alert').textContent).toBe('Só você pode mudar o limite, por esta tela.');
  });

  it('cancelar a confirmação, o botão Cancelar e o Esc não gravam nada', async () => {
    const { gravacoes } = montar({ resposta: null });
    abrir();
    fireEvent.change(campo(), { target: { value: '25' } });
    await salvar();
    expect(gravacoes).toEqual([]);
    fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }));
    expect(screen.queryByLabelText('Limite por dia, de 1 a 40')).toBeNull();
    abrir();
    fireEvent.keyDown(campo(), { key: 'Escape' });
    expect(screen.queryByLabelText('Limite por dia, de 1 a 40')).toBeNull();
    expect(gravacoes).toEqual([]);
  });
});
