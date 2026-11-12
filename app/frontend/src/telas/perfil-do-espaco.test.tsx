import { afterEach, expect, it, vi } from 'vitest';
import { cleanup, render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Contexto, type App } from '../app/contexto';
import { PerfilDoEspaco } from './PerfilDoEspaco';

afterEach(cleanup);
function montar(verificada: boolean) {
  const api = vi.fn(async (path: string, _metodo?: string, _corpo?: unknown) =>
    path === '/api/v1/ia' ? { provedor: 'claude-cli', configurada: true, chaves: {}, clis: [] } : {});
  const app = { estado: { perfilBuscaAtivo: 'java', perfisBusca: [{id:'java',nome:'Java'}], perfil: {
    cargo: 'Java Engineer', curriculos: {PT:'curriculos/interno.pdf'},
    curriculoExibicao: {arquivo:'curriculos/interno.pdf',nome:'Meu currículo.pdf'},
    faixaSalarial: {moeda:'USD',min:4000,max:6000} } }, api, limpar:vi.fn(), recarregar:vi.fn(), aviso:vi.fn(),
    conexaoIA: {nome:'Claude', verificada} } as unknown as App;
  render(<Contexto.Provider value={app}><PerfilDoEspaco /></Contexto.Provider>);
  return api;
}
it('mostra a verificação da sessão sem cobrar nova chamada ao abrir Perfil', async () => {
  const api=montar(true);
  expect(await screen.findByText('Claude conectado')).toBeTruthy();
  expect(screen.getByText('Meu currículo.pdf')).toBeTruthy();
  expect(api.mock.calls.some(([p])=>p.includes('/ia/testar'))).toBe(false);
  fireEvent.click(screen.getByRole('button',{name:'Alterar conexão'}));
  expect(await screen.findByRole('group',{name:'Escolha seu provedor de IA'})).toBeTruthy();
});
it('distingue configuração de confirmação e grava a faixa no perfil selecionado', async () => {
  const api=montar(false);
  await screen.findByRole('group',{name:'Escolha seu provedor de IA'});
  expect(screen.queryByText('Claude conectado')).toBeNull();
  fireEvent.change(screen.getByLabelText('Moeda'),{target:{value:'EUR'}});
  fireEvent.change(screen.getByLabelText('Mínimo'),{target:{value:'7000'}});
  fireEvent.click(screen.getByRole('button',{name:'Salvar perfil'}));
  expect(await screen.findByRole('alert')).toBeTruthy();
  expect(api.mock.calls.some(([,m])=>m==='PATCH')).toBe(false);
  fireEvent.change(screen.getByLabelText('Máximo'),{target:{value:'9000'}});
  fireEvent.click(screen.getByRole('button',{name:'Salvar perfil'}));
  await waitFor(()=>expect(api).toHaveBeenCalledWith('/api/v1/perfil?perfilBuscaId=java','PATCH',
    expect.objectContaining({cargo:'Java Engineer',faixaSalarial:{moeda:'EUR',min:7000,max:9000}})));
});
