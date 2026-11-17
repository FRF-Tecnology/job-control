// Faixa da conta e trava das ações (Task 7): o texto de cada situação, a trava que desabilita as ações das telas
// (menos Perfil, IA e Assinatura) e o 402 de qualquer chamada que faz a faixa aparecer. Só dados fictícios.
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, ID_FAIXA, chamarApi, type App } from '../app/contexto';
import { EVENTO_TRAVA, textoFaixa, useConta, type Situacao } from '../app/conta';
import { FaixaConta } from './FaixaConta';
import { TelaVagas } from './Vagas';
import { PainelEnvio } from './PainelEnvio';
import { TelaTriagem } from './Triagem';
import { TelaPerfil } from './Perfil';

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

const AGORA = new Date('2026-11-11T10:00:00Z');
const depois = (min: number) => new Date(AGORA.getTime() + min * 60_000 + 30_000).toISOString();
const sit = (extra: Partial<Situacao>): Situacao => ({ ativa: true, entrada: true, email: 'ana@exemplo.com', estado: 'teste', teste_fim: depois(60), periodo_fim: null, motivo: '', ...extra });

describe('textoFaixa', () => {
  it('no teste, mostra quanto falta em horas e minutos', () => {
    expect(textoFaixa(sit({ teste_fim: depois(5 * 60 + 12) }), AGORA)).toBe('Seu teste acaba em 5 h 12 min · Assinar');
    expect(textoFaixa(sit({ teste_fim: depois(7) }), AGORA)).toBe('Seu teste acaba em 0 h 7 min · Assinar');
  });
  it('travado: as frases exatas de cada motivo', () => {
    expect(textoFaixa(sit({ estado: 'bloqueado', motivo: 'teste-acabou' }), AGORA)).toBe('Seu teste acabou. Você ainda vê tudo; para buscar e se candidatar, assine.');
    expect(textoFaixa(sit({ estado: 'bloqueado', motivo: 'sem-assinatura' }), AGORA)).toBe('Sua assinatura não está ativa. Você ainda vê tudo; para buscar e se candidatar, assine.');
    expect(textoFaixa(sit({ motivo: 'sem-internet' }), AGORA)).toBe('Sem conexão: dá para ver, mas não para buscar ou se candidatar.');
    expect(textoFaixa(sit({ motivo: 'sem-conta', entrada: true }), AGORA)).toBe('Entre na sua conta para buscar e se candidatar.');
    // motivo desconhecido (ex.: licenca-vencida, só da CLI): como sem assinatura
    expect(textoFaixa(sit({ motivo: 'licenca-vencida' }), AGORA)).toBe('Sua assinatura não está ativa. Você ainda vê tudo; para buscar e se candidatar, assine.');
  });
  it('assinante, conta desligada ou sem situação: sem faixa', () => {
    expect(textoFaixa(sit({ estado: 'assinante', teste_fim: null, periodo_fim: depois(60 * 24 * 20) }), AGORA)).toBeNull();
    expect(textoFaixa({ ativa: false }, AGORA)).toBeNull();
    expect(textoFaixa(null, AGORA)).toBeNull();
  });
});

describe('FaixaConta', () => {
  it('o "Assinar" do teste é um link para a tela da assinatura', () => {
    render(<FaixaConta situacao={sit({ teste_fim: depois(5 * 60 + 12) })} agora={AGORA} reler={async () => {}} />);
    const faixa = screen.getByRole('status');
    expect(faixa.textContent).toBe('Seu teste acaba em 5 h 12 min · Assinar');
    expect(screen.getByRole('link', { name: 'Assinar' }).getAttribute('href')).toBe('#/assinatura');
  });
  it('assinante: nada aparece', () => {
    const { container } = render(<FaixaConta situacao={sit({ estado: 'assinante', teste_fim: null })} agora={AGORA} reler={async () => {}} />);
    expect(container.textContent).toBe('');
  });
});

function appFalso(podeAgir: boolean): App {
  const vagas = [{ id: 'a1', aderencia: 'A', etapa: 'descoberta', empresa: 'Empresa Um', titulo: 'Pessoa Dev', plataforma: 'linkedin', descobertaEm: '2026-11-10' }];
  return {
    estado: { vagas, eventos: [], plataformas: [], perfil: { nome: 'Ana' }, curriculos: [] }, hoje: '2026-11-11', foco: 'ambos', rota: 'triagem', vaga: null,
    mudarFoco: async () => {}, api: vi.fn(async () => ({})), agir: async () => true, mover: vi.fn(async () => true),
    abrirVaga: () => {}, navegar: () => {}, aviso: () => {}, perguntar: async () => null, recarregar: async () => {}, limpar: () => {},
    podeAgir, conta: sit(podeAgir ? {} : { estado: 'bloqueado', motivo: 'teste-acabou' }), releConta: async () => {},
  };
}

// Ruling A13: só as ações de escrita travam (desabilitadas, com a frase da faixa); ver continua livre.
describe('TestTravaDesabilitaSoEscrita', () => {
  const FRASE = 'Seu teste acabou. Você ainda vê tudo; para buscar e se candidatar, assine.';
  const botao = (nome: string) => screen.getByRole('button', { name: nome }) as HTMLButtonElement;
  it('Triagem: decidir fica desabilitado com a frase; ver o detalhe não', () => {
    render(<Contexto.Provider value={appFalso(false)}><TelaTriagem /></Contexto.Provider>);
    for (const nome of ['Pular vaga (tecla P)', 'Aplicar na vaga (tecla A)']) {
      expect(botao(nome).disabled).toBe(true);
      expect(botao(nome).title).toBe(FRASE);
      expect(botao(nome).getAttribute('aria-describedby')).toBe(ID_FAIXA);
    }
    expect(botao('Ver detalhes da vaga').disabled).toBe(false);
    expect(botao('Decidir depois').disabled).toBe(false);
  });
  it('Triagem: a tecla A não decide, só avisa', async () => {
    const app = appFalso(false);
    const avisos: string[] = [];
    app.aviso = t => { avisos.push(t); };
    location.hash = '#/triagem';
    render(<Contexto.Provider value={app}><TelaTriagem /></Contexto.Provider>);
    await act(async () => { fireEvent.keyDown(document.body, { key: 'a' }); });
    expect(app.mover).not.toHaveBeenCalled();
    expect(avisos).toEqual([FRASE]);
    location.hash = '';
  });
  it('podendo agir, a Triagem fica como antes', () => {
    render(<Contexto.Provider value={appFalso(true)}><TelaTriagem /></Contexto.Provider>);
    expect(botao('Aplicar na vaga (tecla A)').disabled).toBe(false);
    expect(botao('Aplicar na vaga (tecla A)').title).toBe('');
  });
  it('Vagas: os filtros continuam; adicionar vaga não', () => {
    render(<Contexto.Provider value={appFalso(false)}><TelaVagas /></Contexto.Provider>);
    expect(screen.getAllByRole('combobox').length).toBeGreaterThan(0);
    expect(screen.getAllByRole('combobox').every(s => !(s as HTMLSelectElement).disabled)).toBe(true);
    expect(botao('Adicionar vaga').disabled).toBe(true);
  });
  it('Fila: Pausar envio continua (A9); Retomar não', async () => {
    const app = appFalso(false);
    let envio = { ligado: true, pausado: false, enviandoAgora: null, desfechos: [] };
    app.api = vi.fn(async (_c: string, metodo = 'GET') => { if (metodo === 'GET') return envio; return (envio = { ...envio, pausado: true }); });
    render(<Contexto.Provider value={app}><PainelEnvio nome={() => 'x'} motivo={() => 'x'} /></Contexto.Provider>);
    expect((await screen.findByRole('button', { name: 'Pausar envio' }) as HTMLButtonElement).disabled).toBe(false);
    await act(async () => { fireEvent.click(botao('Pausar envio')); });
    expect((await screen.findByRole('button', { name: 'Retomar envio' }) as HTMLButtonElement).disabled).toBe(true);
  });
  it('Perfil: salvar o perfil trava (o servidor bloqueia PUT /perfil); a seção Conta não', () => {
    render(<Contexto.Provider value={appFalso(false)}><TelaPerfil /></Contexto.Provider>);
    expect(screen.getAllByRole('button', { name: 'Salvar perfil' }).every(b => (b as HTMLButtonElement).disabled)).toBe(true);
    expect(botao('Sair da conta').disabled).toBe(false);
    expect((screen.getByLabelText('Nome completo') as HTMLInputElement).disabled).toBe(false);
  });
});

describe('402 de qualquer chamada', () => {
  function Casca() {
    const { situacao } = useConta(chamarApi);
    return <FaixaConta situacao={situacao} agora={AGORA} reler={async () => {}} />;
  }
  it('dispara applyer:trava e a faixa de bloqueio aparece', async () => {
    let travado = false;
    const fetch = vi.fn(async (caminho: string, init?: RequestInit) => {
      const json = (status: number, corpo: unknown) => new Response(JSON.stringify(corpo), { status, headers: { 'content-type': 'application/json' } });
      if (caminho === '/api/v1/conta') return json(200, travado ? sit({ estado: 'bloqueado', motivo: 'teste-acabou' }) : sit({ teste_fim: depois(30) }));
      if (caminho === '/api/v1/pedidos' && init?.method === 'POST') {
        travado = true;
        return json(402, { erro: 'Seu teste acabou. Você ainda vê tudo; para buscar e se candidatar, assine.', motivo: 'teste-acabou' });
      }
      return json(404, { erro: 'rota' });
    });
    vi.stubGlobal('fetch', fetch);
    const eventos: string[] = [];
    const ouvir = (ev: Event) => eventos.push((ev as CustomEvent).detail);
    window.addEventListener(EVENTO_TRAVA, ouvir);
    render(<Casca />);
    await waitFor(() => expect(screen.getByRole('status').textContent).toBe('Seu teste acaba em 0 h 30 min · Assinar'));
    await act(async () => { await chamarApi('/api/v1/pedidos', 'POST', { texto: 'buscar' }).catch(() => {}); });
    window.removeEventListener(EVENTO_TRAVA, ouvir);
    expect(eventos).toEqual(['teste-acabou']);
    await waitFor(() => expect(screen.getByRole('status').textContent).toContain('Seu teste acabou. Você ainda vê tudo; para buscar e se candidatar, assine.'));
    expect(fetch.mock.calls.filter(c => c[0] === '/api/v1/conta').length).toBeGreaterThanOrEqual(2);
  });
});

describe('chamarApi', () => {
  it('204 sem corpo (sair, cancelar o Google, excluir) não quebra mesmo com content-type json', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status: 204, headers: { 'content-type': 'application/json' } })));
    await expect(chamarApi('/api/v1/conta/sair', 'POST', {})).resolves.toBe('');
  });
});

// "Tentar de novo" (ruling A14): chama POST /conta/atualizar (pede a licença à nuvem), não só relê GET /conta.
describe('Tentar de novo', () => {
  function Casca({ api }: { api: (c: string, m?: string, corpo?: unknown) => Promise<any> }) {
    const { situacao, atualizar } = useConta(api);
    return <FaixaConta situacao={situacao} agora={AGORA} reler={atualizar} />;
  }
  it('pede a licença de novo e mostra a situação que voltou', async () => {
    const api = vi.fn(async (caminho: string, metodo?: string) => {
      if (caminho === '/api/v1/conta/atualizar' && metodo === 'POST') return sit({ teste_fim: depois(30) });
      return sit({ motivo: 'sem-internet' });
    });
    render(<Casca api={api} />);
    const botao = await screen.findByRole('button', { name: 'Tentar de novo' });
    await act(async () => { fireEvent.click(botao); });
    expect(api).toHaveBeenCalledWith('/api/v1/conta/atualizar', 'POST', {});
    await waitFor(() => expect(screen.getByRole('status').textContent).toBe('Seu teste acaba em 0 h 30 min · Assinar'));
  });
});

// Ao abrir, a primeira conferência com a nuvem ainda não acabou (verificando): texto neutro no lugar de "Sem conexão",
// e a casca relê a cada 2 s até acabar; com sem internet, a cada 15 s; senão, 60 s.
describe('Conferindo a conta ao abrir', () => {
  it('textoFaixa: neutro enquanto verificando, sem o botão de tentar de novo', () => {
    expect(textoFaixa(sit({ motivo: 'sem-internet', verificando: true }), AGORA)).toBe('Conferindo sua conta…');
    render(<FaixaConta situacao={sit({ motivo: 'sem-internet', verificando: true })} agora={AGORA} reler={async () => {}} />);
    const faixa = screen.getByRole('status');
    expect(faixa.textContent).toBe('Conferindo sua conta…');
    expect(faixa.className).not.toContain('travada');
    expect(screen.queryByRole('button', { name: 'Tentar de novo' })).toBeNull();
  });
  it('relê a cada 2 s enquanto verificando, a cada 15 s sem internet e a cada 60 s depois', async () => {
    vi.useFakeTimers();
    try {
      let resposta: Situacao = sit({ motivo: 'sem-internet', verificando: true });
      const api = vi.fn(async () => resposta);
      function Casca() { const { situacao } = useConta(api); return <FaixaConta situacao={situacao} agora={AGORA} reler={async () => {}} />; }
      render(<Casca />);
      await act(async () => { await vi.advanceTimersByTimeAsync(0); });
      expect(api).toHaveBeenCalledTimes(1);
      await act(async () => { await vi.advanceTimersByTimeAsync(2000); });
      expect(api).toHaveBeenCalledTimes(2);
      resposta = sit({ motivo: 'sem-internet' });
      await act(async () => { await vi.advanceTimersByTimeAsync(2000); });
      expect(api).toHaveBeenCalledTimes(3);
      expect(screen.getByRole('status').textContent).toContain('Sem conexão');
      await act(async () => { await vi.advanceTimersByTimeAsync(14_000); });
      expect(api).toHaveBeenCalledTimes(3);
      resposta = sit({});
      await act(async () => { await vi.advanceTimersByTimeAsync(1000); });
      expect(api).toHaveBeenCalledTimes(4);
      await act(async () => { await vi.advanceTimersByTimeAsync(59_000); });
      expect(api).toHaveBeenCalledTimes(4);
      await act(async () => { await vi.advanceTimersByTimeAsync(1000); });
      expect(api).toHaveBeenCalledTimes(5);
    } finally {
      vi.useRealTimers();
    }
  });
});
