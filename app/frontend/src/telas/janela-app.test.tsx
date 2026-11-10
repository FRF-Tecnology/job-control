// "Janela do app" (contrato 0.15.0) com a API interceptada: só aparece no app de PC; ligar e desligar o segundo plano
// mandam o PUT certo e mostram o que ficou; a recusa do sistema (409) não marca a caixa; "Voltar ao tamanho padrão"
// manda o POST vazio. Nenhum texto do servidor vai para a tela.
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { Contexto, ErroApi, type App } from '../app/contexto';
import { JanelaApp } from './JanelaApp';

afterEach(cleanup);

type Chamada = { rota: string; metodo: string; corpo: unknown };

async function montar(estado: unknown, responder?: (c: Chamada) => unknown) {
  const chamadas: Chamada[] = [];
  const app = {
    api: async (rota: string, metodo = 'GET', corpo?: unknown) => {
      const c = { rota, metodo, corpo };
      chamadas.push(c);
      if (metodo === 'GET') { if (estado instanceof Error) throw estado; return estado; }
      const r = responder?.(c);
      if (r instanceof Error) throw r;
      return r;
    },
  } as unknown as App;
  await act(async () => { render(<Contexto.Provider value={app}><JanelaApp /></Contexto.Provider>); });
  return chamadas;
}
const caixa = () => screen.getByRole('checkbox', { name: 'Continuar aberto em segundo plano' }) as HTMLInputElement;

describe('Janela do app', () => {
  it.each([
    ['fora do app de PC (disponivel false)', { disponivel: false, segundoPlano: false }],
    ['resposta sem os campos', {}],
    ['resposta nula', null],
    ['servidor sem a rota (404)', new ErroApi('não encontrado', 404, { erro: 'x' })],
  ])('%s: a seção não aparece', async (_n, estado) => {
    const chamadas = await montar(estado);
    expect(chamadas).toEqual([{ rota: '/api/v1/app/janela', metodo: 'GET', corpo: undefined }]);
    expect(screen.queryByRole('region', { name: 'Janela do app' })).toBeNull();
  });

  it('no app de PC: mostra a escolha gravada e explica o que muda', async () => {
    await montar({ disponivel: true, segundoPlano: true });
    expect(screen.getByRole('region', { name: 'Janela do app' })).toBeTruthy();
    expect(caixa().checked).toBe(true);
    expect(screen.getByText(/fechar a janela não fecha o app/)).toBeTruthy();
    expect(screen.getByText(/use Sair no menu do ícone/)).toBeTruthy();
  });

  it('ligar: manda o PUT e marca a caixa com o que o app respondeu', async () => {
    const chamadas = await montar({ disponivel: true, segundoPlano: false }, () => ({ disponivel: true, segundoPlano: true }));
    expect(caixa().checked).toBe(false);
    await act(async () => { fireEvent.click(caixa()); });
    expect(chamadas[1]).toEqual({ rota: '/api/v1/app/janela', metodo: 'PUT', corpo: { segundoPlano: true } });
    expect(caixa().checked).toBe(true);
    expect(screen.getByRole('status').textContent).toBe('Pronto. O app continua aberto quando você fechar a janela.');
  });

  it('desligar: manda o PUT e avisa que fechar a janela volta a fechar o app', async () => {
    const chamadas = await montar({ disponivel: true, segundoPlano: true }, () => ({ disponivel: true, segundoPlano: false }));
    await act(async () => { fireEvent.click(caixa()); });
    expect(chamadas[1].corpo).toEqual({ segundoPlano: false });
    expect(caixa().checked).toBe(false);
    expect(screen.getByRole('status').textContent).toBe('Pronto. Fechar a janela volta a fechar o app.');
  });

  it('o sistema não deixou criar o ícone (409): a caixa continua desmarcada e a frase é fixa', async () => {
    await montar({ disponivel: true, segundoPlano: false }, () => new ErroApi('<b>texto do servidor</b>', 409, { erro: '<b>texto do servidor</b>', motivo: 'sem-bandeja' }));
    await act(async () => { fireEvent.click(caixa()); });
    expect(caixa().checked).toBe(false);
    expect(screen.getByRole('alert').textContent).toMatch(/^O sistema não deixou pôr o ícone do Applyer .* O app continua fechando junto com a janela\.$/);
    expect(document.body.textContent).not.toContain('texto do servidor');
    expect(screen.queryByRole('status')).toBeNull();
  });

  it('403 e erro qualquer: frase fixa, nada muda', async () => {
    await montar({ disponivel: true, segundoPlano: false }, () => new ErroApi('x', 403, {}));
    await act(async () => { fireEvent.click(caixa()); });
    expect(screen.getByRole('alert').textContent).toBe('Só você pode mudar isso, por esta tela.');
    cleanup();
    await montar({ disponivel: true, segundoPlano: false }, () => new Error('rede'));
    await act(async () => { fireEvent.click(caixa()); });
    expect(screen.getByRole('alert').textContent).toBe('Não deu para mudar. Tente de novo.');
    expect(caixa().checked).toBe(false);
  });

  it('"Voltar ao tamanho padrão": manda o POST vazio e não mexe no segundo plano', async () => {
    const chamadas = await montar({ disponivel: true, segundoPlano: true }, () => ({ disponivel: true, segundoPlano: true }));
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Voltar ao tamanho padrão' })); });
    expect(chamadas[1]).toEqual({ rota: '/api/v1/app/janela/padrao', metodo: 'POST', corpo: {} });
    expect(caixa().checked).toBe(true);
    expect(screen.getByRole('status').textContent).toBe('Pronto. A janela voltou ao tamanho padrão.');
  });
});
