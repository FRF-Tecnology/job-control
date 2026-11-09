// Conectar IA com CLI instalada mas não pronta (jc-a2y.36): ex.: Claude instalado como .cmd pelo npm no
// Windows. A tela mostra "Não pronto" com o aviso fixo do back, sem "Usar". Só dados fictícios.
import { cleanup, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { Contexto, type App } from '../app/contexto';
import { TelaConectarIA } from './ConectarIA';

afterEach(cleanup);

const AVISO = 'esta CLI está instalada como .cmd, e o app não passa o pedido pelo cmd.exe; instale a versão nativa (.exe)';
const NA_TELA = 'Esta CLI está instalada como .cmd, e o app não passa o pedido pelo cmd.exe; instale a versão nativa (.exe)';
const cli = (provedor: string, nome: string, extra: Record<string, unknown>) => ({
  provedor, comando: provedor.replace('-cli', ''), nome, plano: 'Assinatura', instalar: `https://docs.exemplo.com/${provedor}`,
  verificada: false, instalada: true, logada: null, ...extra,
});

function montar(extra: Record<string, unknown> = {}) {
  const estado = {
    configurada: false, chaves: { anthropic: null, openai: null, google: null }, gratis: { disponivel: false },
    uso: { chamadas: 0, falhas: 0, custoEstimadoUSD: 0 },
    clis: [
      cli('claude-cli', 'Claude Code', { verificada: true, pronta: false, aviso: AVISO, instalar: 'https://docs.claude.com/claude-code' }),
      cli('codex-cli', 'Codex', { pronta: true }),
      cli('gemini-cli', 'Gemini CLI', {}), // API antiga, sem "pronta": segue como antes
    ],
    ...extra,
  };
  const app = {
    estado: {}, hoje: '2026-11-09', foco: 'ambos', rota: 'ia', vaga: null,
    api: async (caminho: string) => { if (caminho === '/api/v1/ia') return estado; throw new Error(caminho); },
    aviso: () => {}, limpar: () => {},
  } as unknown as App;
  render(<Contexto.Provider value={app}><TelaConectarIA /></Contexto.Provider>);
}

const item = async (nome: string) => (await screen.findByText(nome)).closest('li') as HTMLElement;

describe('Conectar IA: CLI não pronta', () => {
  it('mostra "Não pronto" com o aviso e o link de instalação, sem "Usar"', async () => {
    montar();
    const claude = await item('Claude Code');
    expect(within(claude).getByText(/Não pronto/)).toBeTruthy();
    expect(within(claude).getByText(NA_TELA)).toBeTruthy();
    expect(within(claude).queryByRole('button', { name: 'Usar' })).toBeNull();
    expect(within(claude).getByRole('link', { name: 'Como instalar' }).getAttribute('href')).toBe('https://docs.claude.com/claude-code');
  });

  it('CLI pronta e CLI da API antiga (sem "pronta") continuam com "Usar"', async () => {
    montar();
    for (const nome of ['Codex', 'Gemini CLI']) {
      const li = await item(nome);
      expect(within(li).getByRole('button', { name: 'Usar' })).toBeTruthy();
      expect(within(li).queryByText(/Não pronto/)).toBeNull();
    }
  });

  it('se a IA em uso não está pronta, o topo avisa e oferece desconectar', async () => {
    montar({ configurada: true, provedor: 'claude-cli' });
    const topo = (await screen.findByText(/Conectada, mas não pronta/)).closest('.convite') as HTMLElement;
    expect(within(topo).getByText(NA_TELA)).toBeTruthy();
    expect(within(topo).getByRole('button', { name: 'Desconectar' })).toBeTruthy();
    expect(within(topo).queryByRole('button', { name: 'Testar conexão' })).toBeNull();
  });
});
