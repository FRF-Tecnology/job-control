import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { Contexto, type App } from '../app/contexto';
import { ConexaoOnboarding } from './ConexaoOnboarding';

afterEach(cleanup);
const vazio = { configurada: false, clis: [], chaves: {} };
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
