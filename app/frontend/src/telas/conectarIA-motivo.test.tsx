// Conectar IA com EstadoCLI.motivo (jc-a2y.38, contrato 0.4.5): CLI não pronta "por política" (Codex, Gemini)
// oferece "Usar chave de API"; "Como instalar" fica para não instalada e para "formato" (Claude .cmd).
// A IA em uso não pronta aparece em âmbar. Só dados fictícios.
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { Contexto, type App } from '../app/contexto';
import { TelaConectarIA } from './ConectarIA';

afterEach(cleanup);

const POLITICA = 'esta CLI ainda não roda no app sem as ferramentas dela; use o Claude ou uma chave de API';
const FORMATO = 'esta CLI está instalada como .cmd, e o app não passa o pedido pelo cmd.exe; instale a versão nativa (.exe)';
const cli = (provedor: string, nome: string, extra: Record<string, unknown>) => ({
  provedor, comando: provedor.replace('-cli', ''), nome, plano: 'Assinatura', instalar: `https://docs.exemplo.com/${provedor}`,
  verificada: false, instalada: true, logada: null, ...extra,
});
const CLIS = [
  cli('claude-cli', 'Claude Code', { pronta: false, motivo: 'formato', aviso: FORMATO }),
  cli('codex-cli', 'Codex', { pronta: false, motivo: 'politica', aviso: POLITICA }),
  cli('gemini-cli', 'Gemini CLI', { pronta: false, motivo: 'politica', aviso: POLITICA }),
];

function montar(extra: Record<string, unknown> = {}, clis = CLIS) {
  const ia = { configurada: false, chaves: { anthropic: null, openai: null, google: null }, clis, gratis: { disponivel: false }, uso: { chamadas: 0, falhas: 0, custoEstimadoUSD: 0 }, ...extra };
  const app = {
    estado: {}, hoje: '2026-11-09', foco: 'ambos', rota: 'ia', vaga: null, limpar: () => {}, aviso: () => {},
    api: async (caminho: string) => { if (caminho === '/api/v1/ia') return ia; throw new Error(caminho); },
  } as unknown as App;
  render(<Contexto.Provider value={app}><TelaConectarIA /></Contexto.Provider>);
}
const item = async (nome: string) => (await screen.findByText(nome)).closest('li') as HTMLElement;
const provedorDaChave = () => (screen.getByLabelText(/Provedor/) as HTMLSelectElement).value;
const campoChave = () => screen.getByLabelText(/Chave de API/) as HTMLInputElement;

describe('Conectar IA: ação conforme o motivo da CLI não pronta', () => {
  it('por política (Codex): aviso e "Usar chave de API", sem "Como instalar" nem "Usar"', async () => {
    montar();
    const codex = await item('Codex');
    expect(within(codex).getByText(/Esta CLI ainda não roda no app/)).toBeTruthy();
    expect(within(codex).getByRole('button', { name: 'Usar chave de API' })).toBeTruthy();
    expect(within(codex).queryByRole('link', { name: 'Como instalar' })).toBeNull();
    expect(within(codex).queryByRole('button', { name: 'Usar' })).toBeNull();
  });

  it('"Usar chave de API" do Codex escolhe OpenAI e põe o foco no campo da chave; do Gemini, Google', async () => {
    montar();
    const codex = within(await item('Codex')).getByRole('button', { name: 'Usar chave de API' });
    await act(async () => { fireEvent.click(codex); });
    expect(provedorDaChave()).toBe('openai');
    expect(document.activeElement).toBe(campoChave());
    const gemini = within(await item('Gemini CLI')).getByRole('button', { name: 'Usar chave de API' });
    await act(async () => { fireEvent.click(gemini); });
    expect(provedorDaChave()).toBe('google');
  });

  it('por formato (Claude .cmd) e não instalada: "Como instalar"', async () => {
    montar({}, [CLIS[0], cli('gemini-cli', 'Gemini CLI', { instalada: false, pronta: false })]);
    for (const nome of ['Claude Code', 'Gemini CLI']) {
      const li = await item(nome);
      expect(within(li).getByRole('link', { name: 'Como instalar' })).toBeTruthy();
      expect(within(li).queryByRole('button', { name: 'Usar chave de API' })).toBeNull();
    }
  });

  it('IA em uso não pronta por política: topo âmbar com "Usar chave de API"', async () => {
    montar({ configurada: true, provedor: 'codex-cli' });
    const topo = (await screen.findByText(/Configurada, mas não pronta/)).closest('.convite') as HTMLElement;
    expect(topo.className).toContain('ambar');
    expect(within(topo).getByRole('button', { name: 'Usar chave de API' })).toBeTruthy();
    expect(within(topo).queryByRole('link', { name: 'Como instalar' })).toBeNull();
    expect(within(topo).getByRole('button', { name: 'Desconectar' })).toBeTruthy();
  });

  it('IA em uso não pronta por formato: topo âmbar com "Como instalar"', async () => {
    montar({ configurada: true, provedor: 'claude-cli' });
    const topo = (await screen.findByText(/Configurada, mas não pronta/)).closest('.convite') as HTMLElement;
    expect(topo.className).toContain('ambar');
    expect(within(topo).getByRole('link', { name: 'Como instalar' })).toBeTruthy();
  });
});
