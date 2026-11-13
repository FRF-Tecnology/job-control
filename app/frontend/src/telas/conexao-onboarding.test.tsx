import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { Contexto, type App } from '../app/contexto';
import { avisoDeDemora, ConexaoOnboarding, msgDemora } from './ConexaoOnboarding';

afterEach(cleanup);
it('uma resposta sem confirmação explícita não libera o perfil', async () => {
  const api = vi.fn(async (u:string) => u.endsWith('/ia') ? {configurada:true,provedor:'anthropic',chaves:{anthropic:'…teste'},clis:[]} : {ok:false});
  const avancar=montar(api);
  fireEvent.click(await screen.findByRole('button',{name:'Usar conexão atual'}));
  await screen.findByRole('alert');
  expect(avancar).not.toHaveBeenCalled();
});
const vazio = { configurada: false, clis: [], chaves: {} };
it('oferece instalação e login e só confirma a assinatura após testar o provedor', async () => {
  const cli = { provedor:'claude-cli', nome:'Claude', instalada:false, pronta:false, logada:false, instalar:'https://docs.claude.com/claude-code' };
  const api = vi.fn(async (url:string) => {
    if (url.endsWith('/ia')) return {...vazio, clis:[{...cli}]};
    if (url.endsWith('/ia/detectar')) { cli.instalada=true;cli.pronta=true;cli.logada=true; return {}; }
    if (url.endsWith('/ia/testar')) return {ok:true,verificacao:'13'};
    return {};
  });
  const avancar=montar(api);
  expect((await screen.findByRole('link',{name:'Instalar Claude Code'})).getAttribute('href')).toBe(cli.instalar);
  expect(screen.getByText('claude auth login')).toBeTruthy();
  fireEvent.click(screen.getByRole('button',{name:'Já entrei · verificar'}));
  await vi.waitFor(()=>expect(avancar).toHaveBeenCalledWith('Claude','13'));
  expect(api.mock.calls.map(c=>c[0]).slice(-3)).toEqual(['/api/v1/ia','/api/v1/ia/config','/api/v1/ia/testar']);
});
function montar(api: App['api']) {
  const avancar = vi.fn();
  const app = { api, limpar: vi.fn() } as unknown as App;
  render(<Contexto.Provider value={app}><ConexaoOnboarding aoConectar={avancar} /></Contexto.Provider>);
  return avancar;
}

it('escolhe entre quatro marcas e confere a chave de Grok antes de liberar o currículo', async () => {
  const api = vi.fn(async (url: string) => url === '/api/v1/ia' ? vazio : { ok: true });
  const avancar = montar(api);
  for (const nome of ['OpenAI', 'Claude', 'Grok', 'Gemini']) expect(await screen.findByRole('button', { name: nome })).toBeTruthy();
  fireEvent.click(screen.getByRole('button', { name: 'Grok' }));
  expect(screen.getByText('api.x.ai')).toBeTruthy();
  fireEvent.change(screen.getByLabelText('Chave de API'), { target: { value: 'valor-ficticio-de-teste' } });
  fireEvent.change(screen.getByLabelText('Modelo'), { target: { value: 'modelo-de-teste' } });
  fireEvent.click(screen.getByRole('button', { name: /Conectar e continuar/ }));
  await vi.waitFor(() => expect(avancar).toHaveBeenCalledOnce());
  expect(api.mock.calls.map(c => c[0])).toEqual(['/api/v1/ia', '/api/v1/ia/config', '/api/v1/ia/chaves/xai', '/api/v1/ia/testar']);
});

it('mantém a pessoa na etapa e permite testar a conexão salva novamente', async () => {
  let tentativas = 0;
  const atual = { ...vazio, configurada: true, provedor: 'openai', modelo: 'teste', chaves: { openai: '…1234' } };
  const api = vi.fn(async (url: string) => {
    if (url === '/api/v1/ia') return atual;
    if (url === '/api/v1/ia/testar' && !tentativas++) throw new Error('Provedor indisponível');
    return { ok: true };
  });
  const avancar = montar(api);
  fireEvent.click(await screen.findByRole('button', { name: /Usar conexão atual/ }));
  expect((await screen.findByRole('alert')).textContent).toContain('Provedor indisponível');
  expect(avancar).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: /Usar conexão atual/ }));
  await vi.waitFor(() => expect(avancar).toHaveBeenCalledOnce());
  expect(api.mock.calls.every(c => ['/api/v1/ia', '/api/v1/ia/testar'].includes(c[0]))).toBe(true);
});

it('explica a espera quando a verificação demora e some quando a IA responde', async () => {
  let responder: (v: unknown) => void = () => {};
  const api = vi.fn((u: string) => u.endsWith('/ia') ? Promise.resolve({ configurada: true, provedor: 'anthropic', chaves: { anthropic: '…teste' }, clis: [] })
    : u.endsWith('/ia/testar') ? new Promise(fim => { responder = fim; }) : Promise.resolve({}));
  const avancar = montar(api as unknown as App['api']);
  const botao = await screen.findByRole('button', { name: 'Usar conexão atual' });
  vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] });
  try {
    fireEvent.click(botao);
    await act(async () => { vi.advanceTimersByTime(avisoDeDemora - 1); });
    expect(screen.queryByText(msgDemora)).toBeNull();
    await act(async () => { vi.advanceTimersByTime(1); });
    expect(screen.getByText(msgDemora)).toBeTruthy();
    await act(async () => { responder({ ok: true, verificacao: '13' }); });
    expect(screen.queryByText(msgDemora)).toBeNull();
  } finally { vi.useRealTimers(); }
  expect(avancar).toHaveBeenCalled();
});
