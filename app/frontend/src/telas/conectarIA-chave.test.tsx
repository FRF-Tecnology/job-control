// Chave de API presa ao endereço (jc-a2y.40, front do jc-a2y.27). A tela mostra para qual endereço a chave vai,
// grava o endereço antes da chave e avisa quando trocar o endereço apagou a chave salva. Só chaves fictícias.
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { Contexto, type App } from '../app/contexto';
import { TelaConectarIA } from './ConectarIA';

afterEach(cleanup);

type Chamada = { caminho: string; metodo: string; corpo?: any };

function montar({ estado = {}, chaveApagada = false }: { estado?: Record<string, unknown>; chaveApagada?: boolean } = {}) {
  const ia = {
    configurada: false, chaves: { anthropic: null, openai: null, google: null }, clis: [], gratis: { disponivel: false },
    uso: { chamadas: 0, falhas: 0, custoEstimadoUSD: 0 }, ...estado,
  };
  const chamadas: Chamada[] = [];
  const avisos: { texto: string; erro?: boolean }[] = [];
  const app = {
    estado: {}, hoje: '2026-11-09', foco: 'ambos', rota: 'ia', vaga: null, limpar: () => {},
    aviso: (texto: string, erro?: boolean) => { avisos.push({ texto, erro }); },
    api: async (caminho: string, metodo = 'GET', corpo?: any) => {
      if (metodo !== 'GET') chamadas.push({ caminho, metodo, corpo });
      if (caminho === '/api/v1/ia') return ia;
      if (caminho === '/api/v1/ia/config') return { configurada: true, provedor: corpo?.provedor, modelo: corpo?.modelo, chaveApagada };
      if (caminho.startsWith('/api/v1/ia/chaves/')) return { provedor: caminho.split('/').pop(), final: '…9999' };
      throw new Error(caminho);
    },
  } as unknown as App;
  render(<Contexto.Provider value={app}><TelaConectarIA /></Contexto.Provider>);
  return { chamadas, avisos };
}

const formulario = async () => (await screen.findByRole('button', { name: /Salvar e usar/ })).closest('form') as HTMLElement;
const campo = (form: HTMLElement, rotulo: RegExp) => within(form).getByLabelText(rotulo) as HTMLInputElement;

describe('Conectar IA: chave presa ao endereço', () => {
  it('mostra o endereço padrão para onde a chave vai', async () => {
    montar();
    const form = await formulario();
    expect(within(form).getByText(/api\.anthropic\.com/)).toBeTruthy();
    expect(within(form).getByText(/padrão/)).toBeTruthy();
  });

  it('com endereço próprio, mostra só o host digitado', async () => {
    montar();
    const form = await formulario();
    fireEvent.change(campo(form, /Provedor/), { target: { value: 'openai' } });
    fireEvent.change(campo(form, /Endereço/), { target: { value: 'https://usuario:senha@OpenRouter.ai/api/v1' } });
    expect(within(form).getByText('openrouter.ai')).toBeTruthy();
    expect(within(form).queryByText(/senha|usuario|\/api\/v1/)).toBeNull();
  });

  it('grava o endereço antes da chave (senão a chave nova fica presa ao endereço antigo e é apagada)', async () => {
    const { chamadas } = montar();
    const form = await formulario();
    fireEvent.change(campo(form, /Provedor/), { target: { value: 'openai' } });
    fireEvent.change(campo(form, /Chave de API/), { target: { value: 'sk-ficticia-0000-9999' } });
    fireEvent.change(campo(form, /Modelo/), { target: { value: 'modelo-exemplo' } });
    fireEvent.change(campo(form, /Endereço/), { target: { value: 'https://compativel.exemplo.com' } });
    await act(async () => { fireEvent.submit(form); });
    expect(chamadas.map(c => `${c.metodo} ${c.caminho}`)).toEqual(['PUT /api/v1/ia/config', 'PUT /api/v1/ia/chaves/openai']);
    expect(chamadas[0].corpo).toMatchObject({ provedor: 'openai', url: 'https://compativel.exemplo.com' });
  });

  it('chave apagada pela troca de endereço: avisa com o host novo, não diz "conectado" e põe o foco na chave', async () => {
    const { avisos } = montar({ chaveApagada: true, estado: { chaves: { anthropic: null, openai: '…1234', google: null } } });
    const form = await formulario();
    fireEvent.change(campo(form, /Provedor/), { target: { value: 'openai' } });
    fireEvent.change(campo(form, /Endereço/), { target: { value: 'https://outro.exemplo.com' } });
    await act(async () => { fireEvent.submit(form); });
    expect(avisos).toEqual([{ texto: 'A chave salva foi apagada porque o endereço mudou para outro.exemplo.com. Cole a chave de novo.', erro: true }]);
    expect(document.activeElement).toBe(campo(form, /Chave de API/));
  });

  it('chave curta não chama a API', async () => {
    const { chamadas, avisos } = montar();
    const form = await formulario();
    fireEvent.change(campo(form, /Chave de API/), { target: { value: 'curta' } });
    await act(async () => { fireEvent.submit(form); });
    expect(chamadas).toEqual([]);
    expect(avisos).toEqual([{ texto: 'Cole a chave de API inteira.', erro: true }]);
  });

  it('provedor em uso com endereço próprio: o campo vem preenchido e o host aparece', async () => {
    montar({ estado: { configurada: true, provedor: 'openai', modelo: 'modelo-exemplo', url: 'https://compativel.exemplo.com', chaves: { anthropic: null, openai: '…1234', google: null } } });
    const form = await formulario();
    expect(campo(form, /Provedor/).value).toBe('openai');
    expect(campo(form, /Endereço/).value).toBe('https://compativel.exemplo.com');
    expect(within(form).getByText('compativel.exemplo.com')).toBeTruthy();
  });

  it('endereço próprio aparece destacado no formulário e repetido no topo (denuncia endereço trocado)', async () => {
    montar({ estado: { configurada: true, provedor: 'openai', modelo: 'modelo-exemplo', url: 'https://compativel.exemplo.com', chaves: { anthropic: null, openai: '…1234', google: null } } });
    const form = await formulario();
    const destino = within(form).getByText('compativel.exemplo.com').closest('.destino-chave') as HTMLElement;
    expect(destino.className).toContain('proprio');
    expect(destino.textContent).toContain('endereço próprio');
    const topo = (await screen.findByText(/^Configurada:/)).closest('.convite') as HTMLElement;
    expect(within(topo).getByText(/Chave para compativel\.exemplo\.com \(endereço próprio\)/)).toBeTruthy();
  });

  it('endereço padrão no topo aparece como padrão', async () => {
    montar({ estado: { configurada: true, provedor: 'anthropic', modelo: 'claude-haiku-5-5', chaves: { anthropic: '…1234', openai: null, google: null } } });
    const topo = (await screen.findByText(/^Configurada:/)).closest('.convite') as HTMLElement;
    expect(within(topo).getByText(/Chave para api\.anthropic\.com \(padrão\)/)).toBeTruthy();
  });

  it('provedor em uso sem chave salva: o topo avisa que falta a chave, com o host', async () => {
    montar({ estado: { configurada: true, provedor: 'openai', modelo: 'modelo-exemplo', url: 'https://compativel.exemplo.com' } });
    const topo = (await screen.findByText(/Falta a chave/)).closest('.convite') as HTMLElement;
    expect(within(topo).getByText(/compativel\.exemplo\.com/)).toBeTruthy();
    expect(within(topo).queryByRole('button', { name: 'Testar conexão' })).toBeNull();
  });
});
