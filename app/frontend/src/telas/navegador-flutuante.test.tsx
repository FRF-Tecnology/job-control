// Navegador do envio flutuante, com a API interceptada e relógio falso. O que importa: fechado, ele não liga nem lê
// nada; só com o mouse (ou o foco) em cima liga o ao vivo e lê a cada segundo; ao sair, desliga e a imagem some; a
// imagem só entra se for data:image/jpeg em base64; 404 não fica tentando; e some quando o painel da Fila está aberto
// ou o app não tem envio. Só dados fictícios.
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ErroApi } from '../app/contexto';
import { EVENTO_PAINEL_AO_VIVO } from './AoVivo';
import { NavegadorFlutuante } from './NavegadorFlutuante';
import { EVENTO_ENVIO } from './PainelEnvio';

const JPEG = 'data:image/jpeg;base64,' + 'QUJD'.repeat(10);
const aoVivo = (extra: object = {}) => ({ ligado: true, etapa: 'preenchendo', candidatura: 'cand-1', quadro: { imagem: JPEG, em: '2026-11-09T10:00:00-03:00', largura: 1280, altura: 800 }, ...extra });
const ENVIO = { ligado: true, pausado: false, login: false, enviandoAgora: { id: 'cand-1', desde: '2026-11-09T10:00:00-03:00' }, desfechos: [] };
const VAGAS = [{ id: 'vaga-1', empresa: 'Empresa Exemplo', titulo: 'Pessoa Dev' }];

beforeEach(() => { vi.useFakeTimers(); localStorage.clear(); });
afterEach(() => { cleanup(); vi.useRealTimers(); });

function montar({ envio = ENVIO as object | null, get = () => aoVivo() as any } = {}) {
  const chamadas: { caminho: string; metodo: string; corpo?: unknown }[] = [];
  const avisos: { texto: string; erro?: boolean }[] = [];
  const api = vi.fn(async (caminho: string, metodo = 'GET', corpo?: unknown) => {
    chamadas.push({ caminho, metodo, corpo });
    if (caminho === '/api/v1/candidaturas') return [{ id: 'cand-1', vagaId: 'vaga-1' }];
    if (caminho === '/api/v1/envio/pausar') return { ...ENVIO, pausado: true };
    if (metodo === 'POST') { const r = get(); if (r instanceof Error && (corpo as any).ligado) throw r; return { ...aoVivo(), ligado: (corpo as any).ligado }; }
    const r = get(); if (r instanceof Error) throw r; return r;
  });
  const r = render(<NavegadorFlutuante api={api} envio={envio as any} vagas={VAGAS} aviso={(texto, erro) => { avisos.push({ texto, erro }); }} />);
  const aoVivoChamadas = () => chamadas.filter(c => c.caminho === '/api/v1/envio/ao-vivo');
  return { chamadas, aoVivoChamadas, avisos, r };
}
const widget = () => screen.getByRole('complementary', { name: 'Navegador do envio' });
const passar = async (ms: number) => { await act(async () => { await vi.advanceTimersByTimeAsync(ms); }); };
const entrar = async () => { await act(async () => { fireEvent.pointerEnter(widget()); }); await passar(300); };
const sair = async () => { await act(async () => { fireEvent.pointerLeave(widget()); }); await passar(500); };

describe('Navegador do envio flutuante', () => {
  it('sem envio neste app (ou antes de saber), não aparece', () => {
    montar({ envio: { ligado: false } });
    expect(screen.queryByRole('complementary')).toBeNull();
    cleanup();
    montar({ envio: null });
    expect(screen.queryByRole('complementary')).toBeNull();
  });

  it('começa encaixado na lateral e, fechado, não liga nem lê o ao vivo', async () => {
    const { chamadas } = montar();
    expect(widget().className).toContain('encaixado');
    expect(screen.getByRole('button', { name: 'Abrir o navegador do envio' }).getAttribute('aria-expanded')).toBe('false');
    await passar(5000);
    expect(chamadas).toEqual([]);
    expect(screen.queryByRole('img')).toBeNull();
  });

  it('o mouse só de passagem não liga; parado em cima, liga, lê a cada segundo e mostra a etapa e a imagem', async () => {
    const { aoVivoChamadas } = montar();
    await act(async () => { fireEvent.pointerEnter(widget()); });
    await passar(100);
    await act(async () => { fireEvent.pointerLeave(widget()); });
    await passar(1000);
    expect(aoVivoChamadas()).toEqual([]);

    await entrar();
    expect(aoVivoChamadas()[0]).toMatchObject({ metodo: 'POST', corpo: { ligado: true } });
    await passar(3000);
    expect(aoVivoChamadas().filter(c => c.metodo === 'GET').length).toBe(3);
    expect(screen.getByRole('status').textContent).toBe('Preenchendo o formulário…');
    const img = screen.getByRole('img', { name: 'Tela do envio de Empresa Exemplo — Pessoa Dev' }) as HTMLImageElement;
    expect(img.getAttribute('src')).toBe(JPEG);
  });

  it('ao sair, desliga ({ligado:false}), para de ler e a imagem não fica na tela', async () => {
    const { aoVivoChamadas } = montar();
    await entrar(); await passar(2000);
    await sair();
    expect(aoVivoChamadas().at(-1)).toMatchObject({ metodo: 'POST', corpo: { ligado: false } });
    const n = aoVivoChamadas().length;
    await passar(5000);
    expect(aoVivoChamadas().length).toBe(n);
    expect(screen.queryByRole('img')).toBeNull();
  });

  it.each([
    ['SVG em data:', 'data:image/svg+xml;base64,PHN2Zz4='],
    ['endereço de fora', 'https://exemplo.com/x.jpg'],
    ['javascript:', 'javascript:alert(1)'],
    ['base64 com caractere fora', 'data:image/jpeg;base64,QUJD"><b>'],
  ])('imagem fora do contrato (%s) não entra no src', async (_n, imagem) => {
    montar({ get: () => aoVivo({ quadro: { imagem, em: 'x', largura: 10, altura: 10 } }) });
    await entrar(); await passar(1000);
    expect(screen.queryByRole('img')).toBeNull();
    expect(screen.getByText('A imagem aparece quando uma candidatura estiver sendo enviada.')).toBeTruthy();
  });

  it('o foco do teclado abre, e o botão da aba mantém aberto mesmo sem o mouse', async () => {
    const { aoVivoChamadas } = montar();
    const aba = screen.getByRole('button', { name: 'Abrir o navegador do envio' });
    await act(async () => { fireEvent.focus(aba); }); await passar(10);
    expect(aoVivoChamadas()[0]).toMatchObject({ metodo: 'POST', corpo: { ligado: true } });
    await act(async () => { fireEvent.click(aba); });
    await act(async () => { fireEvent.blur(aba, { relatedTarget: document.body }); }); await passar(1000);
    expect(screen.getByRole('button', { name: 'Fechar o navegador do envio' }).getAttribute('aria-expanded')).toBe('true');
    expect(aoVivoChamadas().at(-1)?.metodo).toBe('GET'); // continua lendo
  });

  it('404 (app sem o ao vivo): diz em frase fixa e não tenta de novo a cada passada do mouse', async () => {
    const { aoVivoChamadas } = montar({ get: () => new ErroApi('não há', 404, {}) });
    await entrar(); await passar(1000);
    expect(screen.queryByRole('img')).toBeNull();
    const n = aoVivoChamadas().length;
    await sair(); await entrar(); await passar(3000);
    expect(aoVivoChamadas().length).toBe(n);
  });

  it('com o "ao vivo" da Fila aberto, some (e volta quando o painel fecha)', async () => {
    const { aoVivoChamadas } = montar();
    await entrar(); await passar(1000);
    await act(async () => { window.dispatchEvent(new CustomEvent(EVENTO_PAINEL_AO_VIVO, { detail: true })); });
    expect(screen.queryByRole('complementary')).toBeNull();
    expect(aoVivoChamadas().at(-1)).toMatchObject({ metodo: 'POST', corpo: { ligado: false } });
    await act(async () => { window.dispatchEvent(new CustomEvent(EVENTO_PAINEL_AO_VIVO, { detail: false })); });
    expect(widget()).toBeTruthy();
  });

  it('pausar chama POST /envio/pausar, avisa a casca pelo evento e mostra o aviso', async () => {
    const { chamadas, avisos } = montar();
    const eventos: any[] = [];
    const ouvir = (ev: Event) => eventos.push((ev as CustomEvent).detail);
    window.addEventListener(EVENTO_ENVIO, ouvir);
    await entrar();
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Pausar envio' })); });
    window.removeEventListener(EVENTO_ENVIO, ouvir);
    expect(chamadas.some(c => c.caminho === '/api/v1/envio/pausar' && c.metodo === 'POST')).toBe(true);
    expect(eventos[0]).toMatchObject({ pausado: true });
    expect(avisos[0].texto).toContain('Envio pausado');
    expect(screen.queryByRole('button', { name: 'Retomar envio' })).toBeNull(); // retomar é só pela Fila, com confirmação
  });

  it('com o envio pausado não oferece pausar; soltar da lateral vira janelinha e o lugar fica salvo', async () => {
    montar({ envio: { ...ENVIO, pausado: true } });
    await entrar();
    expect(screen.queryByRole('button', { name: 'Pausar envio' })).toBeNull();
    expect(screen.getByRole('link', { name: 'Abrir a Fila' }).getAttribute('href')).toBe('#/fila');
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Soltar o navegador da lateral' })); });
    expect(widget().className).toContain('solto');
    expect(JSON.parse(localStorage.getItem('navegador-envio') ?? '{}').modo).toBe('solto');
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Minimizar o navegador na lateral' })); });
    expect(widget().className).toContain('encaixado');
  });

  it('lugar salvo estranho no armazenamento não quebra: começa encaixado', () => {
    localStorage.setItem('navegador-envio', '{"modo":"voando","r":"x"}');
    montar();
    expect(widget().className).toContain('encaixado');
  });
});
