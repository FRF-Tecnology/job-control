// Painel de envio (jc-a2y.51) com a API interceptada: desligado, pausado (retomar com confirmação, só a tela), ligado
// (pausar direto), enviando agora, últimos envios e aviso só para desfecho novo. Só dados fictícios.
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, ErroApi, type App, type Pergunta } from '../app/contexto';
import { PainelEnvio, type EstadoEnvio } from './PainelEnvio';

afterEach(cleanup);

const BASE: EstadoEnvio = { ligado: true, pausado: true, enviandoAgora: null, desfechos: [] };
const D1 = { id: 'cand-1', estado: 'enviada' as const, motivo: null, em: '2026-11-09T10:00:00-03:00' };
const D2 = { id: 'cand-2', estado: 'bloqueada' as const, motivo: 'cpf', em: '2026-11-09T10:05:00-03:00' };
const NOMES: Record<string, string> = { 'cand-1': 'Empresa Exemplo — Pessoa Dev React', 'cand-2': 'Outra Exemplo — Dev Go' };

function montar({ estado = BASE, resposta = 'ok' as string | null, erroRetomar = null as null | Error, pausarDa = null as EstadoEnvio | null } = {}) {
  let atual = estado;
  const gravacoes: { caminho: string; corpo: any }[] = [];
  const perguntas: Pergunta[] = [];
  const avisos: { texto: string; erro?: boolean }[] = [];
  const app: any = {
    estado: { vagas: [] },
    aviso: (texto: string, erro?: boolean) => { avisos.push({ texto, erro }); },
    perguntar: async (p: Pergunta) => { perguntas.push(p); return resposta; },
    api: vi.fn(async (caminho: string, metodo = 'GET', corpo?: any) => {
      if (metodo === 'GET') return atual;
      gravacoes.push({ caminho, corpo });
      if (caminho.endsWith('/retomar')) { if (erroRetomar) throw erroRetomar; return (atual = { ...atual, pausado: false }); }
      return (atual = pausarDa ?? { ...atual, pausado: true });
    }),
  };
  const ui = <PainelEnvio nome={id => NOMES[id] ?? 'uma candidatura'} motivo={m => ({ cpf: 'Número fiscal' } as Record<string, string>)[m] ?? 'Outro'} />;
  const r = render(<Contexto.Provider value={app as App}>{ui}</Contexto.Provider>);
  // Simula a /versao mudando: o App troca app.estado e a tela relê.
  const reler = async (novo: EstadoEnvio) => { atual = novo; app.estado = { vagas: [] }; await act(async () => { r.rerender(<Contexto.Provider value={{ ...app } as App}>{ui}</Contexto.Provider>); }); };
  return { gravacoes, perguntas, avisos, reler };
}
const clicar = async (nome: string) => { await act(async () => { fireEvent.click(screen.getByRole('button', { name: nome })); }); };

describe('Painel de envio: notas do CyberSec no #60', () => {
  it('leitura falhou: "Pausar envio" continua disponível e manda {}', async () => {
    const gravacoes: any[] = [];
    const app: any = { estado: {}, aviso: () => {}, perguntar: async () => 'ok',
      api: vi.fn(async (caminho: string, metodo = 'GET', corpo?: any) => {
        if (metodo === 'GET') throw new Error('sem rota');
        gravacoes.push({ caminho, corpo }); return { ...BASE, pausado: true };
      }) };
    render(<Contexto.Provider value={app}><PainelEnvio nome={() => 'x'} motivo={() => 'x'} /></Contexto.Provider>);
    await screen.findByText(/Não deu para ler o estado do envio/);
    await clicar('Pausar envio');
    expect(gravacoes).toEqual([{ caminho: '/api/v1/envio/pausar', corpo: {} }]);
    expect(screen.getByRole('heading', { name: 'Envio pausado' })).toBeTruthy();
  });

  it('compara o "em" como instante: desfecho em outro fuso, mais novo, vira aviso', async () => {
    const { avisos, reler } = montar({ estado: { ...BASE, pausado: false, desfechos: [D2] } }); // 10:05 -03:00 = 13:05Z
    await screen.findByRole('heading', { name: 'Envio ligado' });
    const novo = { id: 'cand-1', estado: 'enviada' as const, motivo: null, em: '2026-11-09T09:06:00-04:00' }; // 13:06Z
    await reler({ ...BASE, pausado: false, desfechos: [novo, D2] });
    expect(avisos).toEqual([{ texto: 'Enviada: Empresa Exemplo — Pessoa Dev React.', erro: false }]);
  });

  it('data inválida não quebra: a hora vira "—"', async () => {
    montar({ estado: { ...BASE, enviandoAgora: { id: 'cand-1', desde: 'não é data' }, desfechos: [{ ...D1, em: 'x' }] } });
    await screen.findByRole('heading', { name: 'Envio pausado' });
    expect(screen.getByRole('status').textContent).toBe('Enviando agora: Empresa Exemplo — Pessoa Dev React, desde —');
  });

  it('avisa a casca (evento) a cada estado novo, para a marca "Envio ligado"', async () => {
    const recebidos: any[] = [];
    const ouvir = (e: Event) => recebidos.push((e as CustomEvent).detail);
    window.addEventListener('central:envio', ouvir);
    try {
      montar({ estado: { ...BASE, pausado: true } });
      await screen.findByRole('heading', { name: 'Envio pausado' });
      await clicar('Retomar envio');
      expect(recebidos.map(r => r.pausado)).toEqual([true, false]);
    } finally { window.removeEventListener('central:envio', ouvir); }
  });
});
describe('Painel de envio', () => {
  it('desligado no build: diz que nada é enviado e não mostra botão', async () => {
    montar({ estado: { ...BASE, ligado: false } });
    expect(await screen.findByText('Envio automático desligado neste app: nada é enviado sozinho.')).toBeTruthy();
    expect(screen.queryByRole('button')).toBeNull();
  });

  it('pausado (o começo): "Envio pausado" e "Retomar envio", que pede confirmação e manda {}', async () => {
    const { gravacoes, perguntas, avisos } = montar();
    expect(await screen.findByRole('heading', { name: 'Envio pausado' })).toBeTruthy();
    await clicar('Retomar envio');
    expect(perguntas[0].titulo).toBe('Retomar o envio?');
    expect(gravacoes).toEqual([{ caminho: '/api/v1/envio/retomar', corpo: {} }]);
    expect(screen.getByRole('heading', { name: 'Envio ligado' })).toBeTruthy(); // usa a resposta do POST
    expect(perguntas[0].texto).toMatch(/Sempre que o app abrir, o envio volta pausado\./);
    expect(screen.getByText('Sempre que o app abrir, o envio volta pausado.')).toBeTruthy();
    expect(avisos).toContainEqual({ texto: 'Envio retomado.', erro: undefined });
  });

  it('cancelar a confirmação não retoma', async () => {
    const { gravacoes } = montar({ resposta: null });
    await screen.findByRole('heading', { name: 'Envio pausado' });
    await clicar('Retomar envio');
    expect(gravacoes).toEqual([]);
  });

  it('retomar fora da tela (403) aparece em texto claro', async () => {
    const { avisos } = montar({ erroRetomar: new ErroApi('só pela tela', 403, { erro: 'só pela tela' }) });
    await screen.findByRole('heading', { name: 'Envio pausado' });
    await clicar('Retomar envio');
    expect(avisos).toContainEqual({ texto: 'Só você pode retomar o envio, por esta tela.', erro: true });
  });

  it('ligado: "Pausar envio" vai direto (só reduz o risco), com {}', async () => {
    const { gravacoes, perguntas } = montar({ estado: { ...BASE, pausado: false } });
    await screen.findByRole('heading', { name: 'Envio ligado' });
    await clicar('Pausar envio');
    expect(perguntas).toEqual([]);
    expect(gravacoes).toEqual([{ caminho: '/api/v1/envio/pausar', corpo: {} }]);
    expect(screen.getByRole('heading', { name: 'Envio pausado' })).toBeTruthy();
  });

  it('enviando agora: mostra a vaga e, se pausado, que ela termina antes de parar', async () => {
    montar({ estado: { ...BASE, pausado: true, enviandoAgora: { id: 'cand-1', desde: '2026-11-09T10:00:00-03:00' } } });
    await screen.findByRole('heading', { name: 'Envio pausado' });
    expect(screen.getByRole('status').textContent).toMatch(/^Enviando agora: Empresa Exemplo — Pessoa Dev React, desde \d\d:\d\d$/);
    expect(screen.getByText('A candidatura que está sendo enviada agora termina; nenhuma outra começa até você retomar.')).toBeTruthy();
  });

  it('ligado com envio em curso: junto do Pausar, diz que a candidatura atual termina', async () => {
    const { avisos } = montar({ estado: { ...BASE, pausado: false, enviandoAgora: { id: 'cand-1', desde: '2026-11-09T10:00:00-03:00' } } });
    expect(await screen.findByText('O app envia as aprovadas, uma por vez. Se pausar, a candidatura que está sendo enviada agora termina.')).toBeTruthy();
    await clicar('Pausar envio');
    expect(avisos).toContainEqual({ texto: 'Envio pausado. A candidatura que está sendo enviada agora termina; nenhuma outra sai até você retomar.', erro: undefined });
  });

  it('últimos envios: estado, motivo com o rótulo da fila e a vaga', async () => {
    montar({ estado: { ...BASE, desfechos: [D2, D1] } });
    const lista = (await screen.findByText('Últimos envios (2)')).closest('details')!;
    const itens = within(lista).getAllByRole('listitem').map(li => li.textContent);
    expect(itens[0]).toMatch(/^Bloqueada · Número fiscalOutra Exemplo — Dev Go/);
    expect(itens[1]).toMatch(/^EnviadaEmpresa Exemplo — Pessoa Dev React/);
  });

  it('avisa só o desfecho novo, não os que já estavam na primeira leitura', async () => {
    const { avisos, reler } = montar({ estado: { ...BASE, pausado: false, desfechos: [D1] } });
    await screen.findByRole('heading', { name: 'Envio ligado' });
    expect(avisos).toEqual([]);
    await reler({ ...BASE, pausado: false, desfechos: [D2, D1] });
    expect(avisos).toEqual([{ texto: 'Bloqueada (Número fiscal): Outra Exemplo — Dev Go. Veja na fila o que falta.', erro: true }]);
  });
});
