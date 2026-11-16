// "Caixa de e-mail" (contrato 0.16.0) com a API interceptada: conectar manda o PUT certo e apaga a senha da tela;
// domínio desconhecido pede o servidor; a recusa (400/502) mostra frase fixa e nenhum texto do servidor;
// desconectar manda o DELETE. Dados fictícios.
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { Contexto, ErroApi, type App } from '../app/contexto';
import { ContaEmail } from './ContaEmail';

afterEach(cleanup);

type Chamada = { rota: string; metodo: string; corpo: unknown };

async function montar(estado: unknown, responder?: (c: Chamada) => unknown) {
  const chamadas: Chamada[] = [];
  const app = {
    api: async (rota: string, metodo = 'GET', corpo?: unknown) => {
      const c = { rota, metodo, corpo };
      chamadas.push(c);
      if (metodo === 'GET') return estado;
      const r = responder?.(c);
      if (r instanceof Error) throw r;
      return r;
    },
  } as unknown as App;
  await act(async () => { render(<Contexto.Provider value={app}><ContaEmail /></Contexto.Provider>); });
  return chamadas;
}
const preencher = (rotulo: string, valor: string) => fireEvent.change(screen.getByLabelText(rotulo), { target: { value: valor } });
const senha = () => screen.getByLabelText('Senha de app') as HTMLInputElement;

describe('Caixa de e-mail', () => {
  it('conecta um Gmail sem pedir servidor e apaga a senha da tela', async () => {
    const chamadas = await montar({ conectada: false, usuario: '', servidor: '' },
      () => ({ conectada: true, usuario: 'fulana@gmail.com', servidor: 'imap.gmail.com' }));
    expect(senha().type).toBe('password');
    preencher('Endereço da caixa', ' fulana@gmail.com ');
    preencher('Senha de app', 'abcd efgh ijkl mnop');
    expect(screen.queryByLabelText('Servidor IMAP')).toBeNull();
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Conectar e-mail' })); });
    expect(chamadas[1]).toEqual({ rota: '/api/v1/email/conta', metodo: 'PUT', corpo: { usuario: 'fulana@gmail.com', senha: 'abcd efgh ijkl mnop' } });
    expect(screen.getByText('Conectado: fulana@gmail.com')).toBeTruthy();
    expect(document.body.innerHTML).not.toContain('abcd efgh');
  });

  it('domínio desconhecido pede o servidor', async () => {
    const chamadas = await montar({ conectada: false }, () => ({ conectada: true, usuario: 'fulana@empresa.example', servidor: 'imap.empresa.example' }));
    preencher('Endereço da caixa', 'fulana@empresa.example');
    preencher('Senha de app', 'segredo-ficticio');
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Conectar e-mail' })); });
    expect(screen.getByRole('alert').textContent).toMatch(/servidor IMAP/);
    expect(chamadas).toHaveLength(1);
    preencher('Servidor IMAP', 'imap.empresa.example');
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Conectar e-mail' })); });
    expect(chamadas[1].corpo).toEqual({ usuario: 'fulana@empresa.example', senha: 'segredo-ficticio', servidor: 'imap.empresa.example' });
  });

  it.each([
    [400, /senha de app/],
    [502, /servidor de e-mail/],
  ])('recusa %s: frase fixa, senha apagada e nada conectado', async (status, frase) => {
    await montar({ conectada: false }, () => new ErroApi('texto do servidor', status, { erro: 'texto do servidor' }));
    preencher('Endereço da caixa', 'fulana@gmail.com');
    preencher('Senha de app', 'errada-ficticia');
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Conectar e-mail' })); });
    expect(screen.getByRole('alert').textContent).toMatch(frase);
    expect(document.body.textContent).not.toContain('texto do servidor');
    expect(senha().value).toBe('');
    expect(screen.queryByText(/Conectado:/)).toBeNull();
  });

  it('desconecta', async () => {
    const chamadas = await montar({ conectada: true, usuario: 'fulana@gmail.com', servidor: 'imap.gmail.com' }, () => ({ conectada: false, usuario: '', servidor: '' }));
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Desconectar' })); });
    expect(chamadas[1]).toEqual({ rota: '/api/v1/email/conta', metodo: 'DELETE', corpo: undefined });
    expect(screen.getByLabelText('Endereço da caixa')).toBeTruthy();
  });
});
