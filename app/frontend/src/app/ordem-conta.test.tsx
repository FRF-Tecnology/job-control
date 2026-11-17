// Ordem da casca com a conta: a porta de entrada vem antes do primeiro acesso; com a conta desligada (ativa false,
// build sem a nuvem) ou já com entrada, o primeiro acesso aparece como na reescrita-go. Só dados fictícios.
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import App from '../App';

vi.mock('@wailsio/runtime', () => ({ Browser: { OpenURL: vi.fn() } }));
beforeEach(() => { vi.stubGlobal('scrollTo', vi.fn()); });
afterEach(() => { cleanup(); vi.unstubAllGlobals(); location.hash = ''; });

const estado = { versao: '1', hoje: '2026-11-12', onboardingConcluido: false, perfil: {}, vagas: [], eventos: [], plataformas: [], curriculos: [] };

function servidor(conta: object) {
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    const caminho = String(url).split('?')[0];
    const corpo = caminho === '/api/v1/conta' ? conta : caminho === '/api/v1/estado' ? estado
      : caminho === '/api/v1/versao' ? { versao: '1' } : {};
    return new Response(JSON.stringify(corpo), { status: 200, headers: { 'content-type': 'application/json' } });
  }));
}

describe('porta da conta antes do primeiro acesso', () => {
  it('conta ligada e sem entrada: só a porta, sem o primeiro acesso', async () => {
    servidor({ ativa: true, entrada: false, google_disponivel: false });
    render(<App />);
    expect(await screen.findByRole('heading', { name: 'Entre na sua conta' })).toBeTruthy();
    expect(screen.queryByRole('heading', { name: 'Sua IA e seu currículo' })).toBeNull();
  });
  it('conta ligada e com entrada: o primeiro acesso, com a faixa do teste', async () => {
    servidor({ ativa: true, entrada: true, email: 'ana@exemplo.com', estado: 'teste', teste_fim: '2099-01-01T00:00:00Z', motivo: '' });
    render(<App />);
    expect(await screen.findByRole('heading', { name: 'Sua IA e seu currículo' })).toBeTruthy();
    expect(document.querySelector('.faixa-conta')).not.toBeNull();
    expect(screen.queryByRole('heading', { name: 'Entre na sua conta' })).toBeNull();
  });
  it('conta desligada (ativa false): o primeiro acesso como antes, sem porta nem faixa', async () => {
    servidor({ ativa: false });
    render(<App />);
    expect(await screen.findByRole('heading', { name: 'Sua IA e seu currículo' })).toBeTruthy();
    expect(document.querySelector('.faixa-conta')).toBeNull();
    expect(screen.queryByRole('heading', { name: 'Entre na sua conta' })).toBeNull();
  });
});
