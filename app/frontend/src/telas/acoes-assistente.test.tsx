// Ações do assistente na tela (jc-a2y.7.4.1): envio e plataforma pausados pelo assistente (MCP ou linha de comando),
// aprovação desfeita pelo assistente e o canal nos eventos. Só exibição, com textos fixos; canal fora da lista vira
// só "assistente". Só dados fictícios.
import { cleanup, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, type App } from '../app/contexto';
import { PainelEnvio, type EstadoEnvio } from './PainelEnvio';
import { TelaFila } from './Fila';
import { nomeAssistente } from '../util';

afterEach(cleanup);

const ENVIO = (extra: Partial<EstadoEnvio>): EstadoEnvio => ({ ligado: true, pausado: true, enviandoAgora: null, desfechos: [], janela: 'oculta', login: false, ...extra });
function painel(estado: EstadoEnvio) {
  const app = { estado: { vagas: [] }, aviso: () => {}, perguntar: async () => 'ok', api: vi.fn(async () => estado) } as unknown as App;
  render(<Contexto.Provider value={app}><PainelEnvio nome={() => 'x'} motivo={() => 'x'} /></Contexto.Provider>);
}

describe('Ações do assistente', () => {
  it('nome do canal: MCP, linha de comando, e fora da lista só "assistente"', () => {
    expect(nomeAssistente('mcp')).toBe('assistente (MCP)');
    expect(nomeAssistente('cli')).toBe('assistente (linha de comando)');
    expect(nomeAssistente('<b>x</b>')).toBe('assistente');
    expect(nomeAssistente('toString')).toBe('assistente');
    expect(nomeAssistente(undefined)).toBe('assistente');
  });

  it.each([
    ['mcp', 'Pausado pelo assistente (MCP).'],
    ['cli', 'Pausado pelo assistente (linha de comando).'],
  ])('envio pausado pelo assistente (%s)', async (canal, frase) => {
    painel(ENVIO({ pausadoPor: { autor: 'claude', canal } }));
    expect(await screen.findByText(frase)).toBeTruthy();
  });

  it('envio pausado pela pessoa ou sem autor: nenhuma frase de assistente', async () => {
    painel(ENVIO({ pausadoPor: { autor: 'eu', canal: 'tela' } }));
    await screen.findByRole('heading', { name: 'Envio pausado' });
    expect(screen.queryByText(/pelo assistente/)).toBeNull();
  });

  it('plataforma pausada pelo assistente (MCP) diz quem pausou', async () => {
    painel(ENVIO({ pausado: false, plataformas: [{ plataforma: 'gupy', estado: 'pausada', motivo: null, pausadaPor: { autor: 'claude', canal: 'mcp' }, paginasHoje: 0, tetoPaginas: 40, voltasHoje: 0 }] }));
    expect(await screen.findByText('Pausada pelo assistente (MCP). Nada desta plataforma sai até você retomar.')).toBeTruthy();
  });

  it('linha do tempo da Hoje: o canal do assistente no evento; sem canal, "Claude" como antes', async () => {
    const vaga = { id: 'v1', empresa: 'Empresa Exemplo', titulo: 'Dev', etapa: 'pulada', plataforma: 'linkedin' };
    const app = {
      estado: { vagas: [vaga], eventos: [
        { id: 'e1', vagaId: 'v1', tipo: 'pulada', data: '2026-11-09', autor: 'claude', canal: 'mcp' },
        { id: 'e2', vagaId: 'v1', tipo: 'nota', data: '2026-11-08', autor: 'claude' },
        { id: 'e3', vagaId: 'v1', tipo: 'descoberta', data: '2026-11-07', autor: 'eu' },
      ], perfil: {}, pedidos: [], emails: [], primeirosPassos: [] },
      hoje: '2026-11-09', foco: 'ambos', mudarFoco: async () => {}, aviso: () => {}, perguntar: async () => 'ok', agir: async () => true, abrirVaga: () => {},
      api: vi.fn(async () => ({ configurada: true })),
    } as unknown as App;
    const { TelaHoje } = await import('./Hoje');
    render(<Contexto.Provider value={app}><TelaHoje /></Contexto.Provider>);
    expect(await screen.findByText('9 nov · assistente (MCP)')).toBeTruthy();
    expect(screen.getByText('8 nov · Claude')).toBeTruthy();
    expect(screen.getByText('7 nov')).toBeTruthy();
  });

  it('aprovação desfeita pelo assistente aparece no cartão da Fila', async () => {
    const cand = { id: 'cand-a', vagaId: 'v1', plataforma: 'linkedin', url: 'https://www.linkedin.com/jobs/view/1', estado: 'preparada',
      conteudo: { curriculo: { arquivo: 'cv.pdf' }, idioma: 'pt', respostas: [] }, impressao: 'sha256:v1:' + '0'.repeat(64),
      aprovacao: null, bloqueio: null, resultado: null, versao: 2, criadaEm: 'x', atualizadaEm: 'x', revogadaPor: { autor: 'claude', canal: 'cli' } };
    const app = {
      estado: { vagas: [{ id: 'v1', empresa: 'Empresa Exemplo', titulo: 'Dev', etapa: 'aplicar' }], perfil: {}, curriculos: [], eventos: [], plataformas: [] },
      hoje: '2026-11-09', aviso: () => {}, perguntar: async () => 'ok', recarregar: async () => {}, limpar: () => {}, abrirVaga: () => {},
      api: vi.fn(async (c: string) => c.endsWith('/limites') ? {} : c.endsWith('/envio') ? ENVIO({}) : [cand]),
    } as unknown as App;
    render(<Contexto.Provider value={app}><TelaFila /></Contexto.Provider>);
    const c = await screen.findByTestId('cand-a');
    expect(within(c).getByText('A aprovação foi desfeita pelo assistente (linha de comando). Revise o pacote e aprove de novo, se quiser.')).toBeTruthy();
  });
});
