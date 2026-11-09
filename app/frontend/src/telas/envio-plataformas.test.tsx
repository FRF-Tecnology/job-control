// Estado das plataformas no painel de envio (jc-a2y.6.11.1) com a API interceptada: rótulos e frases fixas por estado
// e motivo, páginas e voltas de hoje, "Retomar <plataforma>" só em pausada/precisa-entrar (com confirmação, {} e 409/403
// em frase fixa), parada até amanhã sem botão, e servidor antigo sem o bloco. Só dados fictícios.
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, ErroApi, type App, type Pergunta } from '../app/contexto';
import { PainelEnvio, type EstadoEnvio, type EstadoPlataforma } from './PainelEnvio';

afterEach(cleanup);

const amanha0h = () => { const d = new Date(); d.setDate(d.getDate() + 1); d.setHours(0, 0, 0, 0); return d.toISOString(); };
const plat = (plataforma: EstadoPlataforma['plataforma'], estado: EstadoPlataforma['estado'], extra: Partial<EstadoPlataforma> = {}): EstadoPlataforma =>
  ({ plataforma, estado, motivo: null, ate: null, voltasHoje: 0, paginasHoje: 0, tetoPaginas: 40, ...extra });
const ESTADO = (plataformas?: EstadoPlataforma[]): EstadoEnvio => ({ ligado: true, pausado: false, enviandoAgora: null, desfechos: [], janela: 'oculta', login: false, plataformas });

function montar(estado: EstadoEnvio, { resposta = 'ok' as string | null, erro = null as null | Error } = {}) {
  let atual = estado;
  const gravacoes: { caminho: string; corpo: unknown }[] = [];
  const perguntas: Pergunta[] = [];
  const avisos: { texto: string; erro?: boolean }[] = [];
  const app: any = {
    estado: { vagas: [] }, aviso: (texto: string, e?: boolean) => { avisos.push({ texto, erro: e }); },
    perguntar: async (p: Pergunta) => { perguntas.push(p); return resposta; },
    api: vi.fn(async (caminho: string, metodo = 'GET', corpo?: unknown) => {
      if (metodo === 'GET') return atual;
      gravacoes.push({ caminho, corpo });
      if (erro) throw erro;
      const p = caminho.split('/')[5];
      return (atual = { ...atual, plataformas: atual.plataformas!.map(x => x.plataforma === p ? { ...x, estado: 'ativa', motivo: null } : x) });
    }),
  };
  render(<Contexto.Provider value={app as App}><PainelEnvio nome={() => 'x'} motivo={() => 'x'} /></Contexto.Provider>);
  return { gravacoes, perguntas, avisos };
}
const cartao = async (nome: string) => (await screen.findByText(nome, { selector: '.plat-topo strong' })).closest('li') as HTMLElement;
const clicar = async (el: HTMLElement) => { await act(async () => { fireEvent.click(el); }); };

describe('Painel de envio: plataformas (.6.11)', () => {
  it('ativa mostra páginas e voltas de hoje; no teto, espera até amanhã; sem botão de retomar', async () => {
    montar(ESTADO([plat('gupy', 'ativa', { paginasHoje: 12, voltasHoje: 2 }), plat('inhire', 'ativa', { paginasHoje: 40 }), plat('linkedin', 'ativa', { paginasHoje: 1, voltasHoje: 1 })]));
    const gupy = await cartao('Gupy');
    expect(within(gupy).getByText('Ativa')).toBeTruthy();
    expect(within(gupy).getByText('12 de 40 páginas hoje (2 voltas)')).toBeTruthy();
    expect(within(await cartao('LinkedIn')).getByText('1 de 40 páginas hoje (1 volta)')).toBeTruthy();
    expect(within(await cartao('InHire')).getByText('Chegou ao teto de páginas de hoje: espera até amanhã.')).toBeTruthy();
    expect(screen.queryByRole('button', { name: /^Retomar (Gupy|InHire|LinkedIn)$/ })).toBeNull();
  });

  it('parada até amanhã por aviso da conta: rótulo "Aviso da conta", frase fixa, quando volta, e nenhum botão', async () => {
    montar(ESTADO([plat('linkedin', 'parada-hoje', { motivo: 'aviso-conta', ate: amanha0h() }), plat('gupy', 'parada-hoje', { motivo: 'captcha', ate: amanha0h() })]));
    const li = await cartao('LinkedIn');
    expect(within(li).getByText('Aviso da conta')).toBeTruthy();
    expect(within(li).getByText('A plataforma mostrou um aviso sobre a sua conta: veja você antes de tentar de novo. Volta sozinha amanhã às 00:00.')).toBeTruthy();
    expect(within(li).queryAllByRole('button')).toEqual([]);
    const gupy = await cartao('Gupy');
    expect(within(gupy).getByText('Parada até amanhã')).toBeTruthy();
    expect(within(gupy).getByText('A plataforma pediu um captcha. Volta sozinha amanhã às 00:00.')).toBeTruthy();
  });

  it('pausada por falhas seguidas: "Retomar Gupy" pede confirmação e manda {} só para a rota da plataforma', async () => {
    const { gravacoes, perguntas, avisos } = montar(ESTADO([plat('gupy', 'pausada', { motivo: 'falhas-seguidas' })]));
    const gupy = await cartao('Gupy');
    expect(within(gupy).getByText('Pausada depois de duas falhas seguidas iguais. Confira o que houve antes de retomar.')).toBeTruthy();
    await clicar(within(gupy).getByRole('button', { name: 'Retomar Gupy' }));
    expect(perguntas[0].titulo).toBe('Retomar Gupy?');
    expect(perguntas[0].texto).toMatch(/duas falhas seguidas iguais/);
    expect(gravacoes).toEqual([{ caminho: '/api/v1/envio/plataformas/gupy/retomar', corpo: {} }]);
    expect(avisos).toContainEqual({ texto: 'Gupy retomada.', erro: undefined });
    expect(within(await cartao('Gupy')).getByText('Ativa')).toBeTruthy();
  });

  it('cancelar a confirmação não retoma', async () => {
    const { gravacoes } = montar(ESTADO([plat('inhire', 'pausada')]), { resposta: null });
    await clicar(within(await cartao('InHire')).getByRole('button', { name: 'Retomar InHire' }));
    expect(gravacoes).toEqual([]);
  });

  it('precisa entrar: no LinkedIn, "Entrar no LinkedIn" ao lado de "Retomar"; na Gupy, o caminho é o cartão da candidatura', async () => {
    montar(ESTADO([plat('gupy', 'precisa-entrar', { motivo: 'login' }), plat('linkedin', 'precisa-entrar', { motivo: 'login' })]));
    const li = await cartao('LinkedIn');
    expect(within(li).getByText('Precisa entrar')).toBeTruthy();
    expect(within(li).getByRole('button', { name: 'Entrar no LinkedIn' })).toBeTruthy();
    expect(within(li).getByRole('button', { name: 'Retomar LinkedIn' })).toBeTruthy();
    const gupy = await cartao('Gupy');
    expect(within(gupy).getByText(/Entre pelo cartão de uma candidatura desta plataforma/)).toBeTruthy();
    expect(within(gupy).queryByRole('button', { name: /Entrar/ })).toBeNull();
  });

  it.each([
    [409, 'Gupy já está ativa ou parada até amanhã. A tela foi atualizada.'],
    [403, 'Só você pode retomar, por esta tela.'],
    [500, 'Não deu para retomar. Tente de novo.'],
  ])('retomar com %i vira frase fixa (nunca o texto do servidor)', async (status, frase) => {
    const { avisos } = montar(ESTADO([plat('gupy', 'pausada')]), { erro: new ErroApi('<b>servidor</b>', status, { erro: 'x' }) });
    await clicar(within(await cartao('Gupy')).getByRole('button', { name: 'Retomar Gupy' }));
    expect(avisos).toContainEqual({ texto: frase, erro: true });
  });

  it('plataforma fora da lista (linkedin, gupy, inhire) não ganha "Retomar" nem vira caminho (nota do CyberSec no #114)', async () => {
    const { gravacoes } = montar(ESTADO([plat('../candidaturas/x/aprovar' as any, 'pausada'), plat('gupy', 'ativa')]));
    await cartao('Gupy');
    expect(screen.queryByRole('button', { name: /^Retomar / })).toBeNull();
    expect(gravacoes).toEqual([]);
  });

  it('servidor sem o campo plataformas: o painel não mostra o bloco', async () => {
    montar(ESTADO(undefined));
    await screen.findByRole('heading', { name: 'Envio ligado' });
    expect(screen.queryByRole('heading', { name: 'Plataformas' })).toBeNull();
  });
});
