// Aviso da pasta de dados (jc-mub) com a API interceptada: cada código da lista fechada vira frase fixa; null, código
// desconhecido e 404 não mostram nada; "Entendi" esconde até o app abrir de novo. Só dados fictícios.
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { Contexto, ErroApi, type App } from '../app/contexto';
import { AvisoPasta } from './AvisoPasta';

afterEach(() => { cleanup(); sessionStorage.clear(); });

async function montar(resposta: unknown, erro?: Error) {
  const chamadas: string[] = [];
  const app = { api: async (c: string) => { chamadas.push(c); if (erro) throw erro; return resposta; } } as unknown as App;
  await act(async () => { render(<Contexto.Provider value={app}><AvisoPasta /></Contexto.Provider>); });
  return chamadas;
}

describe('Aviso da pasta de dados', () => {
  it('pasta-antiga-ficou: frase fixa, sem nada da resposta além do código', async () => {
    const chamadas = await montar({ aviso: 'pasta-antiga-ficou', migradaEm: null, caminho: 'C:\\Users\\Usuario Exemplo\\AppData' });
    expect(chamadas).toEqual(['/api/v1/dados/pasta']);
    expect(screen.getByText('A pasta antiga continua no computador')).toBeTruthy();
    expect(screen.getByText(/O app usa só a pasta nova \(Applyer\) e não juntou nada/)).toBeTruthy();
    expect(document.body.textContent).not.toMatch(/Usuario Exemplo|AppData/);
  });

  it('migracao-adiada: diz o que fechar para a troca acontecer na próxima abertura', async () => {
    await montar({ aviso: 'migracao-adiada', migradaEm: null });
    expect(screen.getByText('A pasta de dados ainda não mudou de nome')).toBeTruthy();
    expect(screen.getByText(/Feche o app antigo, a CLI e o assistente \(MCP\)/)).toBeTruthy();
  });

  it.each([
    ['aviso null (com migradaEm, que é só informação)', { aviso: null, migradaEm: '2026-11-09T10:00:00-03:00' }],
    ['código fora da lista', { aviso: '<b>outro</b>', migradaEm: null }],
    ['"toString" (herdado do Object)', { aviso: 'toString', migradaEm: null }],
    ['"constructor" (herdado do Object)', { aviso: 'constructor', migradaEm: null }],
    ['"__proto__"', { aviso: '__proto__', migradaEm: null }],
  ])('%s: nada aparece', async (_n, resposta) => {
    await montar(resposta);
    expect(screen.queryByRole('status')).toBeNull();
  });

  it('servidor sem a rota (404): nada aparece', async () => {
    await montar(null, new ErroApi('não encontrado', 404, { erro: 'x' }));
    expect(screen.queryByRole('status')).toBeNull();
  });

  it('"Entendi" esconde e continua escondido ao voltar para a Hoje nesta abertura', async () => {
    await montar({ aviso: 'pasta-antiga-ficou', migradaEm: null });
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Entendi' })); });
    expect(screen.queryByRole('status')).toBeNull();
    cleanup();
    await montar({ aviso: 'pasta-antiga-ficou', migradaEm: null });
    expect(screen.queryByRole('status')).toBeNull();
  });
});
