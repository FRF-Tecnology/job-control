// "Ao vivo" (jc-a2y.6.9.1) com a API interceptada e relógio falso: abrir liga, aberto lê a cada segundo, fechar e sair
// desligam, ligado false com o painel aberto religa, a imagem só entra se for data:image/jpeg em base64, a etapa vira
// frase fixa, 404/403 em frase fixa, e com a aba escondida não lê. Só dados fictícios.
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Contexto, ErroApi, type App } from '../app/contexto';
import { AoVivo } from './AoVivo';

const JPEG = 'data:image/jpeg;base64,' + 'QUJD'.repeat(10);
const est = (extra: object = {}) => ({ ligado: true, etapa: 'preenchendo', candidatura: 'cand-1', quadro: { imagem: JPEG, em: '2026-11-09T10:00:00-03:00', largura: 1280, altura: 800 }, ...extra });

beforeEach(() => { vi.useFakeTimers(); });
afterEach(() => { cleanup(); vi.useRealTimers(); Object.defineProperty(document, 'visibilityState', { value: 'visible', configurable: true }); });

function montar({ get = () => est() as any, erroPost = null as null | Error } = {}) {
  const chamadas: { metodo: string; corpo?: unknown }[] = [];
  const app = {
    api: vi.fn(async (_c: string, metodo = 'GET', corpo?: unknown) => {
      chamadas.push({ metodo, corpo });
      if (metodo === 'POST') { if (erroPost && (corpo as any).ligado) throw erroPost; return { ...est(), ligado: (corpo as any).ligado }; }
      const r = get(); if (r instanceof Error) throw r; return r;
    }),
  } as unknown as App;
  const r = render(<Contexto.Provider value={app}><AoVivo nome={id => (id === 'cand-1' ? 'Empresa Exemplo — Pessoa Dev' : 'outra')} /></Contexto.Provider>);
  return { chamadas, r };
}
const abrir = async () => { await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Ver ao vivo' })); }); };
const passar = async (ms: number) => { await act(async () => { await vi.advanceTimersByTimeAsync(ms); }); };

describe('Ao vivo', () => {
  it('abrir liga ({ligado:true}), lê a cada segundo e mostra a etapa e a imagem num <img>', async () => {
    const { chamadas } = montar();
    expect(chamadas).toEqual([]); // fechado: nada liga nem lê
    await abrir();
    expect(chamadas[0]).toEqual({ metodo: 'POST', corpo: { ligado: true } });
    await passar(3000);
    expect(chamadas.filter(c => c.metodo === 'GET').length).toBe(3);
    expect(screen.getByRole('status').textContent).toBe('Preenchendo o formulário…');
    expect(screen.getByText('Empresa Exemplo — Pessoa Dev')).toBeTruthy();
    const img = screen.getByRole('img', { name: 'Tela do envio de Empresa Exemplo — Pessoa Dev' }) as HTMLImageElement;
    expect(img.getAttribute('src')).toBe(JPEG);
    expect(screen.getByRole('button', { name: 'Fechar o ao vivo' }).getAttribute('aria-expanded')).toBe('true');
  });

  it.each([
    ['SVG em data:', 'data:image/svg+xml;base64,PHN2Zz4='],
    ['endereço de fora', 'https://exemplo.com/x.jpg'],
    ['javascript:', 'javascript:alert(1)'],
    ['base64 com caractere fora', 'data:image/jpeg;base64,QUJD"><b>'],
  ])('imagem fora do contrato (%s) não entra no src', async (_n, imagem) => {
    montar({ get: () => est({ quadro: { imagem, em: 'x', largura: 10, altura: 10 } }) });
    await abrir(); await passar(1000);
    expect(screen.queryByRole('img')).toBeNull();
    expect(screen.getByText('A imagem aparece quando uma candidatura estiver sendo enviada.')).toBeTruthy();
  });

  it('fechar desliga ({ligado:false}) e para de ler', async () => {
    const { chamadas } = montar();
    await abrir(); await passar(2000);
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Fechar o ao vivo' })); });
    const n = chamadas.length;
    expect(chamadas[n - 1]).toEqual({ metodo: 'POST', corpo: { ligado: false } });
    await passar(3000);
    expect(chamadas.length).toBe(n);
  });

  it('sair da tela com o painel aberto também desliga', async () => {
    const { chamadas, r } = montar();
    await abrir(); await passar(1000);
    await act(async () => { r.unmount(); });
    expect(chamadas.at(-1)).toEqual({ metodo: 'POST', corpo: { ligado: false } });
  });

  it('o app desligou sozinho (ligado false) com o painel aberto: a tela liga de novo', async () => {
    let n = 0;
    const { chamadas } = montar({ get: () => (++n === 2 ? est({ ligado: false, etapa: 'desligado', quadro: null }) : est()) });
    await abrir(); await passar(2000);
    expect(chamadas.filter(c => c.metodo === 'POST' && (c.corpo as any).ligado).length).toBe(2);
  });

  it('etapas em frase fixa: sem envio, login aberto, pausado, desconhecida', async () => {
    let etapa = 'sem-envio';
    montar({ get: () => est({ etapa, candidatura: null, quadro: null }) });
    await abrir(); await passar(1000);
    expect(screen.getByRole('status').textContent).toBe('Nada sendo enviado agora.');
    etapa = 'login-aberto'; await passar(1000);
    expect(screen.getByRole('status').textContent).toBe('A janela de login está aberta: o envio espera.');
    expect(screen.getByText('Sem imagem enquanto isso.')).toBeTruthy();
    etapa = 'pausado'; await passar(1000);
    expect(screen.getByRole('status').textContent).toBe('O envio está pausado.');
    etapa = '<b>outra</b>'; await passar(1000);
    expect(screen.getByRole('status').textContent).toBe('Nada sendo enviado agora.');
  });

  it.each([
    [404, 'Este app não traz o envio: não há o que ver.'],
    [403, 'O ao vivo só abre na janela do app.'],
  ])('%i ao ligar: frase fixa e o painel fecha', async (status, frase) => {
    montar({ erroPost: new ErroApi('<b>servidor</b>', status, { erro: 'x' }) });
    await abrir(); await passar(0);
    expect(screen.getByRole('alert').textContent).toBe(frase);
    expect(screen.getByRole('button', { name: 'Ver ao vivo' })).toBeTruthy();
  });

  it('com a aba escondida não lê (o app desliga sozinho em 10 s)', async () => {
    const { chamadas } = montar();
    await abrir();
    Object.defineProperty(document, 'visibilityState', { value: 'hidden', configurable: true });
    await passar(3000);
    expect(chamadas.filter(c => c.metodo === 'GET')).toEqual([]);
  });
});
