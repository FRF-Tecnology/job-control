// Rodada do LinkedIn: o ▶ aprova os pacotes prontos (com a impressão desta tela) e retoma o envio e a plataforma;
// pacote com carta ou resposta fora do perfil fica de fora; pausar; sessão caída pede login. Dados fictícios.
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, type App } from '../app/contexto';
import type { Candidatura } from './Fila';
import { planoDaRodada, RodadaLinkedIn } from './RodadaLinkedIn';

afterEach(cleanup);
const HOJE = '2026-11-16';
const cand = (id: string, extra: Partial<Candidatura> = {}): Candidatura => ({
  id, vagaId: 'v-' + id, plataforma: 'linkedin', url: 'https://www.linkedin.com/jobs/view/1/', estado: 'preparada', impressao: 'imp-' + id,
  conteudo: { curriculo: { arquivo: 'cv.pdf' }, idioma: 'pt', contato: null, pretensao: null, respostas: [], mensagem: null },
  aprovacao: null, bloqueio: null, resultado: null, versao: 1, criadaEm: `${HOJE}T10:0${id.length}:00Z`, atualizadaEm: `${HOJE}T10:00:00Z`, ...extra,
});
const calculadas = (cs: Candidatura[]) => Object.fromEntries(cs.map(c => [c.id, c.impressao]));
const envio = (extra: object = {}) => ({ ligado: true, pausado: true, enviandoAgora: null, desfechos: [], login: false,
  plataformas: [{ plataforma: 'linkedin', estado: 'ativa' }], ...extra });

describe('planoDaRodada', () => {
  it('pega as prontas do LinkedIn, deixa de fora as que a pessoa revê e respeita o limite de hoje', () => {
    const cs = [
      cand('a'), cand('b'), cand('c'),
      cand('carta', { conteudo: { ...cand('x').conteudo, mensagem: 'Olá' } }),
      cand('nova', { conteudo: { ...cand('x').conteudo, respostas: [{ pergunta: 'Visto?', resposta: 'Não', origem: 'pessoa' }] } }),
      cand('gupy', { plataforma: 'gupy' }),
      cand('fila', { estado: 'aprovada' }),
      cand('ok', { estado: 'enviada', resultado: { em: `${HOJE}T09:00:00Z`, tipo: 'enviada' } }),
      cand('ontem', { estado: 'enviada', resultado: { em: '2026-11-15T09:00:00Z', tipo: 'enviada' } }),
      cand('bloq', { estado: 'bloqueada', bloqueio: { motivo: 'site-externo', pendencia: '', em: `${HOJE}T09:30:00Z` } }),
      cand('conf'),
    ];
    const calc = calculadas(cs);
    delete calc.conf; // ainda conferindo
    calc.c = 'outra'; // a impressão não bate
    const p = planoDaRodada(cs, calc, { limite: 3, usadas: 0, renovaEm: '' }, HOJE);
    expect(p.prontas.map(c => c.id)).toEqual(['a', 'b']); // limite 3 − 1 aprovada na fila = 2
    expect(p.ficam).toBe(0);
    expect(p.revisar).toBe(3); // carta, resposta nova e a impressão que não bate
    expect(p.conferindo).toBe(1);
    expect(p.naFila.map(c => c.id)).toEqual(['fila']);
    expect([p.enviadasHoje, p.paraVoce, p.resta]).toEqual([1, 1, 2]);
    expect(planoDaRodada([cand('a'), cand('b')], calculadas([cand('a'), cand('b')]), { limite: 1, usadas: 0, renovaEm: '' }, HOJE).ficam).toBe(1);
  });
});

function montar(cs: Candidatura[], estadoEnvio: object, respostas: Record<string, unknown> = {}) {
  const atualizar = vi.fn(async () => {});
  const api = vi.fn(async (rota: string, metodo = 'GET', _corpo?: unknown) => {
    const chave = `${metodo} ${rota}`;
    if (chave in respostas) return respostas[chave];
    if (chave === 'GET /api/v1/envio') return estadoEnvio;
    return {};
  });
  const app = { api, aviso: vi.fn(), hoje: HOJE, estado: {} } as unknown as App;
  render(<Contexto.Provider value={app}><RodadaLinkedIn candidaturas={cs} calculadas={calculadas(cs)}
    limites={{ linkedin: { limite: 30, usadas: 0, renovaEm: '' } }} nome={c => `Empresa ${c.id}`} atualizar={atualizar} /></Contexto.Provider>);
  return { api, app, atualizar };
}

describe('RodadaLinkedIn', () => {
  it('▶ aprova as prontas com a impressão da tela, retoma o envio e o LinkedIn pausado', async () => {
    const cs = [cand('a'), cand('b'), cand('carta', { conteudo: { ...cand('x').conteudo, mensagem: 'Olá' } })];
    const pausada = envio({ plataformas: [{ plataforma: 'linkedin', estado: 'pausada' }] });
    const retomado = envio({ pausado: false, plataformas: [{ plataforma: 'linkedin', estado: 'pausada' }] });
    const { api, atualizar } = montar(cs, pausada, {
      'POST /api/v1/candidaturas/aprovar-lote': { aprovadas: ['a', 'b'], recusadas: [] },
      'POST /api/v1/envio/retomar': retomado,
      'POST /api/v1/envio/plataformas/linkedin/retomar': envio({ pausado: false }),
    });
    const botao = await screen.findByRole('button', { name: 'Começar rodada (2)' });
    expect(screen.getByText(/Vão nesta rodada: Empresa a · Empresa b\./)).toBeTruthy();
    expect(screen.getByText(/1 pacote tem carta ou resposta fora do perfil/)).toBeTruthy();
    await act(async () => { fireEvent.click(botao); });
    const posts = api.mock.calls.filter(([, m]) => m === 'POST');
    expect(posts).toEqual([
      ['/api/v1/candidaturas/aprovar-lote', 'POST', { itens: [{ id: 'a', impressao: 'imp-a' }, { id: 'b', impressao: 'imp-b' }] }],
      ['/api/v1/envio/retomar', 'POST', {}],
      ['/api/v1/envio/plataformas/linkedin/retomar', 'POST', {}],
    ]);
    expect(atualizar).toHaveBeenCalled();
  });

  it('rodando: mostra o que está indo e pausa', async () => {
    const cs = [cand('a', { estado: 'aprovada' }), cand('b', { estado: 'aprovada' })];
    const { api } = montar(cs, envio({ pausado: false, enviandoAgora: { id: 'a', desde: `${HOJE}T10:00:00Z` } }), {
      'POST /api/v1/envio/pausar': envio({ pausado: true }),
    });
    expect((await screen.findByText(/Enviando agora:/)).textContent).toMatch(/Enviando agora: Empresa a/);
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Pausar' })); });
    expect(api).toHaveBeenCalledWith('/api/v1/envio/pausar', 'POST', {});
    expect(screen.getByRole('button', { name: 'Retomar rodada' })).toBeTruthy();
  });

  it('sessão do LinkedIn caída: o botão principal é entrar', async () => {
    const { api } = montar([cand('a')], envio({ plataformas: [{ plataforma: 'linkedin', estado: 'precisa-entrar' }] }), {
      'POST /api/v1/navegador/entrar': envio({ login: true }),
    });
    expect(await screen.findByText(/A sessão do LinkedIn caiu/)).toBeTruthy();
    expect(screen.queryByRole('button', { name: /Começar rodada/ })).toBeNull();
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Entrar no LinkedIn' })); });
    expect(api).toHaveBeenCalledWith('/api/v1/navegador/entrar', 'POST', { plataforma: 'linkedin' });
  });

  it('lista vazia: botão Começar desligado e o caminho pelo Radar', async () => {
    montar([], envio());
    expect(await screen.findByText(/Adicione vagas do LinkedIn no Radar/)).toBeTruthy();
    expect((screen.getByRole('button', { name: 'Começar rodada' }) as HTMLButtonElement).disabled).toBe(true);
  });
  it('janela de login aberta: explica que é o navegador do app e, quando ela fecha, o botão volta sozinho', async () => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
    try {
      let estado: object = envio({ login: true });
      const atualizar = vi.fn(async () => {});
      const api = vi.fn(async (rota: string) => rota === '/api/v1/envio' ? estado : {});
      const app = { api, aviso: vi.fn(), hoje: HOJE, estado: {} } as unknown as App;
      render(<Contexto.Provider value={app}><RodadaLinkedIn candidaturas={[cand('a')]} calculadas={calculadas([cand('a')])}
        limites={{ linkedin: { limite: 30, usadas: 0, renovaEm: '' } }} nome={c => c.id} atualizar={atualizar} /></Contexto.Provider>);
      expect(await screen.findByText(/navegador separado do seu/)).toBeTruthy();
      expect((screen.getByRole('button', { name: 'Começar rodada (1)' }) as HTMLButtonElement).disabled).toBe(true);
      estado = envio(); // a pessoa fechou a janela
      await act(async () => { vi.advanceTimersByTime(2100); });
      await act(async () => {});
      expect((screen.getByRole('button', { name: 'Começar rodada (1)' }) as HTMLButtonElement).disabled).toBe(false);
    } finally { vi.useRealTimers(); }
  });
  it('sem o envio neste build: não começa e diz o caminho', async () => {
    montar([cand('a')], envio({ ligado: false }));
    expect(await screen.findByText(/não envia sozinha/)).toBeTruthy();
    expect((screen.getByRole('button', { name: 'Começar rodada (1)' }) as HTMLButtonElement).disabled).toBe(true);
  });
});
