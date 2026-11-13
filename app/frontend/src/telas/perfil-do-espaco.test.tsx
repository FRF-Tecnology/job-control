import { afterEach, expect, it, vi } from 'vitest';
import { cleanup, render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Contexto, type App } from '../app/contexto';
import { PerfilDoEspaco } from './PerfilDoEspaco';

afterEach(cleanup);
function montar(verificada: boolean) {
  const api = vi.fn(async (path: string, _metodo?: string, _corpo?: unknown) =>
    path === '/api/v1/ia' ? { provedor: 'claude-cli', configurada: true, chaves: {}, clis: [] } : {});
  const app = { estado: { perfilBuscaAtivo: 'java', perfisBusca: [{id:'java',nome:'Java'}], perfil: {
    cargo: 'Java Engineer', paisTrabalho:'BR', cidadeTrabalho:'Recife', filtros:{modelo:['Híbrido']}, curriculos: {PT:'curriculos/interno.pdf'},
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
  expect(screen.getByLabelText('País de trabalho')).toHaveProperty('value','BR');
  expect(screen.getByLabelText('Cidade de trabalho')).toHaveProperty('value','Recife');
  fireEvent.change(screen.getByLabelText('País de trabalho'),{target:{value:'PT'}});
  fireEvent.change(screen.getByLabelText('Cidade de trabalho'),{target:{value:'Porto'}});
  fireEvent.change(screen.getByLabelText('Títulos equivalentes'),{target:{value:'Desenvolvedor Java, Java Developer'}});
  fireEvent.click(screen.getByRole('button',{name:'Salvar perfil'}));
  await waitFor(()=>expect(api).toHaveBeenCalledWith('/api/v1/perfil?perfilBuscaId=java','PATCH',
    expect.objectContaining({cargo:'Java Engineer',titulosEquivalentes:['Desenvolvedor Java','Java Developer'],paisTrabalho:'PT',cidadeTrabalho:'Porto',faixaSalarial:{moeda:'EUR',min:7000,max:9000}})));
});

function montarCom(perfil: object) {
  const api = vi.fn(async (path: string, _metodo?: string, _corpo?: unknown) =>
    path === '/api/v1/ia' ? { provedor: 'claude-cli', configurada: true, chaves: {}, clis: [] } : {});
  const app = { estado: { perfilBuscaAtivo: 'original', perfisBusca: [], perfil: { cargo: 'Java Engineer', ...perfil } }, api,
    limpar: vi.fn(), recarregar: vi.fn(), aviso: vi.fn(), conexaoIA: { nome: 'Claude', verificada: true } } as unknown as App;
  render(<Contexto.Provider value={app}><PerfilDoEspaco /></Contexto.Provider>);
  return api;
}
it('grava e-mail e telefone para o pacote e recusa e-mail inválido', async () => {
  const api = montarCom({ nome: 'Fulano', email: 'antigo@exemplo.com' });
  expect(screen.getByLabelText('E-mail')).toHaveProperty('value', 'antigo@exemplo.com');
  fireEvent.change(screen.getByLabelText('E-mail'), { target: { value: 'sem-arroba' } });
  fireEvent.click(screen.getByRole('button', { name: 'Salvar perfil' }));
  expect((await screen.findByRole('alert')).textContent).toMatch(/e-mail válido/);
  expect(api.mock.calls.some(([, m]) => m === 'PATCH')).toBe(false);
  fireEvent.change(screen.getByLabelText('E-mail'), { target: { value: ' fulano@exemplo.com ' } });
  fireEvent.change(screen.getByLabelText('Telefone'), { target: { value: '(11) 91234-5678' } });
  fireEvent.click(screen.getByRole('button', { name: 'Salvar perfil' }));
  await waitFor(() => expect(api).toHaveBeenCalledWith('/api/v1/perfil?perfilBuscaId=original', 'PATCH',
    expect.objectContaining({ nome: 'Fulano', email: 'fulano@exemplo.com', telefone: '(11) 91234-5678' })));
});
it('sem faixa salva, a moeda segue o mercado até a pessoa escolher outra', () => {
  montarCom({ foco: 'nacional' });
  const moeda = () => (screen.getByLabelText('Moeda') as HTMLSelectElement).value;
  expect(moeda()).toBe('BRL');
  fireEvent.change(screen.getByLabelText('Mercado'), { target: { value: 'internacional' } });
  expect(moeda()).toBe('USD');
  fireEvent.change(screen.getByLabelText('Moeda'), { target: { value: 'EUR' } });
  fireEvent.change(screen.getByLabelText('Mercado'), { target: { value: 'nacional' } });
  expect(moeda()).toBe('EUR');
});
it('com faixa salva, trocar o mercado não mexe na moeda', () => {
  montarCom({ foco: 'internacional', faixaSalarial: { moeda: 'USD', min: 5000 } });
  fireEvent.change(screen.getByLabelText('Mercado'), { target: { value: 'nacional' } });
  expect((screen.getByLabelText('Moeda') as HTMLSelectElement).value).toBe('USD');
});
