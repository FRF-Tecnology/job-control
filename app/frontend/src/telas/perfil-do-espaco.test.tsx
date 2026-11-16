import { afterEach, expect, it, vi } from 'vitest';
import { act, cleanup, render, screen, fireEvent, waitFor } from '@testing-library/react';
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
it('dados de contato: grava cidade, LinkedIn e GitHub e recusa link de outro site', async () => {
  const api = montarCom({ nome: 'Fulana Exemplo' });
  expect(screen.getByRole('heading', { name: 'Dados de contato' })).toBeTruthy();
  expect(screen.getByLabelText('Nome')).toHaveProperty('value', 'Fulana');
  expect(screen.getByLabelText('Sobrenome')).toHaveProperty('value', 'Exemplo');
  fireEvent.change(screen.getByLabelText('Cidade'), { target: { value: 'Recife' } });
  fireEvent.change(screen.getByLabelText('LinkedIn'), { target: { value: 'https://golpe.example/linkedin.com' } });
  fireEvent.click(screen.getByRole('button', { name: 'Salvar perfil' }));
  expect((await screen.findByRole('alert')).textContent).toMatch(/perfil no LinkedIn/);
  expect(api.mock.calls.some(([, m]) => m === 'PATCH')).toBe(false);
  fireEvent.change(screen.getByLabelText('LinkedIn'), { target: { value: ' linkedin.com/in/fulana ' } });
  fireEvent.change(screen.getByLabelText('GitHub'), { target: { value: 'github.com' } });
  fireEvent.click(screen.getByRole('button', { name: 'Salvar perfil' }));
  expect((await screen.findByRole('alert')).textContent).toMatch(/perfil no GitHub/);
  fireEvent.change(screen.getByLabelText('GitHub'), { target: { value: 'https://github.com/fulana' } });
  fireEvent.click(screen.getByRole('button', { name: 'Salvar perfil' }));
  await waitFor(() => expect(api).toHaveBeenCalledWith('/api/v1/perfil?perfilBuscaId=original', 'PATCH',
    expect.objectContaining({ nome: 'Fulana Exemplo', primeiroNome: 'Fulana', sobrenome: 'Exemplo', cidade: 'Recife', paisResidencia: 'BR',
      linkedin: 'linkedin.com/in/fulana', github: 'https://github.com/fulana' })));
});
it('nome e sobrenome separados; estado da lista e cidades do estado; perfil antigo é separado', async () => {
  const api = montarCom({ nome: 'Maria Clara Souza', cidade: 'Recife, PE' });
  // perfil antigo: primeira palavra e o resto; a sigla no fim da cidade vira o estado
  expect(screen.getByLabelText('Nome')).toHaveProperty('value', 'Maria');
  expect(screen.getByLabelText('Sobrenome')).toHaveProperty('value', 'Clara Souza');
  expect(screen.getByLabelText('Cidade')).toHaveProperty('value', 'Recife');
  const estado = await waitFor(() => { const e = screen.getByLabelText('Estado') as HTMLSelectElement; expect(e.options.length).toBe(28); return e; });
  expect(estado.value).toBe('PE');
  expect([...estado.options].find(o => o.value === 'PE')?.text).toBe('Pernambuco');
  const lista = document.getElementById('cidades-do-estado')!;
  expect([...lista.querySelectorAll('option')].some(o => o.value === 'Olinda')).toBe(true);
  expect([...lista.querySelectorAll('option')].some(o => o.value === 'Campinas')).toBe(false);
  fireEvent.change(screen.getByLabelText('Nome'), { target: { value: 'Maria Clara' } });
  fireEvent.change(screen.getByLabelText('Sobrenome'), { target: { value: 'Souza' } });
  fireEvent.change(estado, { target: { value: 'SP' } });
  expect([...document.getElementById('cidades-do-estado')!.querySelectorAll('option')].some(o => o.value === 'Campinas')).toBe(true);
  fireEvent.change(screen.getByLabelText('Cidade'), { target: { value: 'Campinas' } });
  fireEvent.click(screen.getByRole('button', { name: 'Salvar perfil' }));
  await waitFor(() => expect(api).toHaveBeenCalledWith('/api/v1/perfil?perfilBuscaId=original', 'PATCH',
    expect.objectContaining({ nome: 'Maria Clara Souza', primeiroNome: 'Maria Clara', sobrenome: 'Souza', cidade: 'Campinas', estado: 'SP', paisResidencia: 'BR' })));
});
it('fora do Brasil o estado é texto livre e não há lista de cidades', () => {
  montarCom({ paisResidencia: 'PT', estado: 'Lisboa', cidade: 'Lisboa' });
  expect(screen.getByLabelText('Estado ou província')).toHaveProperty('value', 'Lisboa');
  expect(document.getElementById('cidades-do-estado')).toBeNull();
});
it('telefone: código do país num seletor e o número à parte; número antigo com "+" é separado', async () => {
  const api = montarCom({ telefone: '+351 912 345 678', paisResidencia: 'BR' });
  const ddi = screen.getByLabelText('Código do país do telefone') as HTMLSelectElement;
  expect(ddi.value).toBe('PT');
  expect(ddi.options[ddi.selectedIndex].text).toBe('+351 Portugal');
  expect(screen.getByLabelText('Telefone')).toHaveProperty('value', '912 345 678');
  fireEvent.change(ddi, { target: { value: 'BR' } });
  fireEvent.change(screen.getByLabelText('Telefone'), { target: { value: '34 99999-0000' } });
  fireEvent.click(screen.getByRole('button', { name: 'Salvar perfil' }));
  await waitFor(() => expect(api).toHaveBeenCalledWith('/api/v1/perfil?perfilBuscaId=original', 'PATCH',
    expect.objectContaining({ ddi: '+55', telefone: '34 99999-0000' })));
});
it('telefone sem código guardado: o país onde mora (e o +1 dos Estados Unidos não vira Canadá)', () => {
  montarCom({ telefone: '415 555 0000', paisResidencia: 'US' });
  expect((screen.getByLabelText('Código do país do telefone') as HTMLSelectElement).value).toBe('US');
  cleanup();
  montarCom({ telefone: '415 555 0000', ddi: '+1', paisResidencia: 'US' });
  expect((screen.getByLabelText('Código do país do telefone') as HTMLSelectElement).value).toBe('US');
});
it('CPF: rota e botão próprios, confere os dígitos, mostra só a máscara e nunca vai com o perfil', async () => {
  const cpf = '529.982.247-25'; // gerado para teste
  const api = vi.fn(async (path: string, metodo?: string, _corpo?: unknown) => {
    if (path === '/api/v1/ia') return { provedor: 'claude-cli', configurada: true, chaves: {}, clis: [] };
    if (path === '/api/v1/perfil/cpf') return metodo === 'PUT' ? { salvo: true, mascarado: '***.982.247-**' } : metodo === 'DELETE' ? { salvo: false } : { salvo: false };
    return {};
  });
  const app = { estado: { perfilBuscaAtivo: 'original', perfisBusca: [], perfil: { cargo: 'Java Engineer' } }, api,
    limpar: vi.fn(), recarregar: vi.fn(), aviso: vi.fn(), conexaoIA: { nome: 'Claude', verificada: true } } as unknown as App;
  render(<Contexto.Provider value={app}><PerfilDoEspaco /></Contexto.Provider>);
  const campo = await screen.findByLabelText('CPF');
  fireEvent.change(campo, { target: { value: '529.982.247-24' } });
  fireEvent.click(screen.getByRole('button', { name: 'Salvar CPF' }));
  expect((await screen.findByRole('alert')).textContent).toMatch(/11 números/);
  expect(api.mock.calls.some(([, m]) => m === 'PUT')).toBe(false);
  fireEvent.change(campo, { target: { value: cpf } });
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Salvar CPF' })); });
  expect(api).toHaveBeenCalledWith('/api/v1/perfil/cpf', 'PUT', { cpf });
  expect(screen.getByText('***.982.247-**')).toBeTruthy();
  expect(screen.queryByLabelText('CPF')).toBeNull(); // o número digitado saiu da tela
  // Salvar o perfil não leva o CPF.
  fireEvent.click(screen.getByRole('button', { name: 'Salvar perfil' }));
  await waitFor(() => expect(api.mock.calls.some(([p, m]) => p.startsWith('/api/v1/perfil?') && m === 'PATCH')).toBe(true));
  expect(JSON.stringify(api.mock.calls.filter(([p]) => p.startsWith('/api/v1/perfil?')))).not.toMatch(/982/);
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Apagar' })); });
  expect(api).toHaveBeenLastCalledWith('/api/v1/perfil/cpf', 'DELETE');
  expect(screen.getByLabelText('CPF')).toBeTruthy();
});
it('CPF salvo: a caixinha liga o preenchimento sem perguntar', async () => {
  const api = vi.fn(async (path: string, metodo?: string, corpo?: any) => {
    if (path === '/api/v1/ia') return { provedor: 'claude-cli', configurada: true, chaves: {}, clis: [] };
    if (path === '/api/v1/perfil/cpf') return { salvo: true, mascarado: '***.982.247-**', automatico: metodo === 'PATCH' ? corpo.automatico : false };
    return {};
  });
  const app = { estado: { perfilBuscaAtivo: 'original', perfisBusca: [], perfil: { cargo: 'Java Engineer' } }, api,
    limpar: vi.fn(), recarregar: vi.fn(), aviso: vi.fn(), conexaoIA: { nome: 'Claude', verificada: true } } as unknown as App;
  render(<Contexto.Provider value={app}><PerfilDoEspaco /></Contexto.Provider>);
  const caixa = await screen.findByRole('checkbox', { name: 'Preencher o CPF sem perguntar' }) as HTMLInputElement;
  expect(caixa.checked).toBe(false);
  expect(screen.getByText(/pergunta antes, a cada site/)).toBeTruthy();
  await act(async () => { fireEvent.click(caixa); });
  expect(api).toHaveBeenLastCalledWith('/api/v1/perfil/cpf', 'PATCH', { automatico: true });
  expect((screen.getByRole('checkbox', { name: 'Preencher o CPF sem perguntar' }) as HTMLInputElement).checked).toBe(true);
  expect(screen.getByText(/junto com o resto, depois que você confere o site/)).toBeTruthy();
});
