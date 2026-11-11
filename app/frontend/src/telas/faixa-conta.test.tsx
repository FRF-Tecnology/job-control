// Faixa da conta e trava das ações (Task 7): o texto de cada situação, a trava que desabilita as ações das telas
// (menos Perfil, IA e Assinatura) e o 402 de qualquer chamada que faz a faixa aparecer. Só dados fictícios.
import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, chamarApi, type App } from '../app/contexto';
import { EVENTO_TRAVA, textoFaixa, useConta, type Situacao } from '../app/conta';
import { FaixaConta, TravaDaTela } from './FaixaConta';
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

describe('TravaDaTela', () => {
  const botoes = () => screen.getAllByRole('button') as HTMLButtonElement[];
  it('sem poder agir, os botões da Triagem ficam desabilitados', () => {
    render(<Contexto.Provider value={appFalso(false)}><TravaDaTela rota="triagem" ativa podeAgir={false}><TelaTriagem /></TravaDaTela></Contexto.Provider>);
    expect(botoes().length).toBeGreaterThan(0);
    expect(botoes().every(b => b.matches(':disabled'))).toBe(true);
  });
  it('podendo agir, nada fica desabilitado pela trava', () => {
    render(<Contexto.Provider value={appFalso(true)}><TravaDaTela rota="triagem" ativa podeAgir><TelaTriagem /></TravaDaTela></Contexto.Provider>);
    expect(botoes().some(b => b.matches(':disabled'))).toBe(false);
  });
  it('o Perfil continua liberado mesmo travado', () => {
    render(<Contexto.Provider value={appFalso(false)}><TravaDaTela rota="perfil" ativa podeAgir={false}><TelaPerfil /></TravaDaTela></Contexto.Provider>);
    expect(screen.getAllByRole('button', { name: 'Salvar perfil' }).every(b => !b.matches(':disabled'))).toBe(true);
  });
  it('conta desligada no build: nenhuma moldura em volta da tela (como antes)', () => {
    const { container } = render(<TravaDaTela rota="triagem" ativa={false} podeAgir><p>tela</p></TravaDaTela>);
    expect(container.querySelector('fieldset')).toBeNull();
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
