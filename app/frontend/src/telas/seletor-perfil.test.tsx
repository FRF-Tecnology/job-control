import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { Contexto, type App } from '../app/contexto';
import { SeletorPerfil } from './SeletorPerfil';

afterEach(cleanup);

it('mantém o perfil e as edições quando a pessoa cancela a troca', async () => {
  const api = vi.fn();
  const perguntar = vi.fn().mockResolvedValue(null);
  const app = { api, perguntar, estado: { perfilBuscaAtivo: 'java', perfisBusca: [{ id: 'original', nome: 'IA' }, { id: 'java', nome: 'Java' }] } } as unknown as App;
  render(<Contexto.Provider value={app}><div data-perfil-alterado="true"><SeletorPerfil /></div></Contexto.Provider>);
  const seletor = screen.getByRole('combobox', { name: 'Perfil de busca' });
  fireEvent.change(seletor, { target: { value: 'original' } });
  await waitFor(() => expect(perguntar).toHaveBeenCalledWith(expect.objectContaining({ confirmar: 'Descartar alterações' })));
  expect(api).not.toHaveBeenCalled();
  expect(seletor).toHaveProperty('value', 'java');
});

it('cria a busca Java com o nome escolhido e troca a busca pelo identificador persistente', async () => {
  const api = vi.fn().mockResolvedValue({});
  const recarregar = vi.fn().mockResolvedValue(undefined);
  const app = { api, recarregar, estado: { perfilBuscaAtivo: 'original', perfisBusca: [{ id: 'original', nome: 'Design' }, { id: 'ai', nome: 'IA' }] } } as unknown as App;
  render(<Contexto.Provider value={app}><SeletorPerfil /></Contexto.Provider>);
  fireEvent.click(screen.getByRole('button', { name: 'Criar perfil' }));
  fireEvent.change(await screen.findByLabelText('Cargo de interesse'), { target: { value: 'java' } });
  expect(screen.getByLabelText('Nome do perfil')).toHaveProperty('value', 'Java Engineer');
  fireEvent.change(screen.getByLabelText('Nome do perfil'), { target: { value: 'Java no Brasil' } });
  fireEvent.submit(screen.getByLabelText('Nome do perfil').closest('form')!);
  await waitFor(() => expect(recarregar).toHaveBeenCalledTimes(1));
  expect(api).toHaveBeenCalledWith('/api/v1/perfis-busca', 'POST', { nome: 'Java no Brasil', modelo: 'java', perfilOrigem: 'original' });
  fireEvent.change(screen.getByRole('combobox', { name: 'Perfil de busca' }), { target: { value: 'ai' } });
  await waitFor(() => expect(api).toHaveBeenCalledWith('/api/v1/perfis-busca/ai/ativar', 'POST', { perfilOrigem: 'original' }));
});
