// Fila: aprovar e as outras ações (P5, jc-a2y.6.3.2). Aprovar envia a impressão calculada nesta tela, do mesmo
// objeto mostrado, e só quando ela bate com a do servidor (regra do PO e do CyberSec). Cada ação que muda o rumo da
// candidatura pede confirmação; revogar só reduz o risco e vai direto. Corpo {} nas ações. Só dados fictícios.
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, ErroApi, type App, type Pergunta } from '../app/contexto';
import { TelaFila } from './Fila';

afterEach(cleanup);

const IMP = 'sha256:v1:2238b4657006a7556b71d4428ea8ca0b71e6158d730a2ed276d3d881f6198129'; // a do Go para este pacote
const CONTEUDO = {
  curriculo: { arquivo: 'curriculo-exemplo.pdf', sha256: 'a'.repeat(64) }, idioma: 'pt',
  contato: { nome: 'Ana Exemplo', email: 'ana@exemplo.com' }, pretensao: { valor: 5000.5, moeda: 'BRL', tipo: 'CLT' },
  respostas: [{ pergunta: 'Anos com React? "ação"\n', resposta: '3', origem: 'respostasPadrao' }], mensagem: null,
};
const cand = (id: string, estado: string, extra: Record<string, unknown> = {}) => ({
  id: `cand-${id}`, vagaId: 'ext-exemplo-1', plataforma: 'linkedin', url: 'https://www.linkedin.com/jobs/view/123', estado,
  conteudo: CONTEUDO, impressao: IMP, aprovacao: null, bloqueio: null, resultado: null, versao: 2,
  criadaEm: '2026-11-09T10:00:00-03:00', atualizadaEm: '2026-11-09T10:00:00-03:00', ...extra,
});
const LIMITES = { linkedin: { limite: 30, usadas: 0, renovaEm: '2026-11-10T00:00:00-03:00' }, gupy: { limite: 30, usadas: 0, renovaEm: '2026-11-10T00:00:00-03:00' }, inhire: { limite: 30, usadas: 0, renovaEm: '2026-11-10T00:00:00-03:00' } };

function montar({ candidaturas = [] as any[], resposta = 'ok' as string | null, erro = null as null | ErroApi } = {}) {
  const gravacoes: { caminho: string; corpo: any }[] = [];
  const perguntas: Pergunta[] = [];
  const avisos: { texto: string; erro?: boolean }[] = [];
  const app = {
    estado: { vagas: [{ id: 'ext-exemplo-1', empresa: 'Empresa Exemplo', titulo: 'Pessoa Dev React', etapa: 'aplicar' }], perfil: {}, curriculos: [], eventos: [], plataformas: [] },
    hoje: '2026-11-09', foco: 'ambos', rota: 'fila', vaga: null, limpar: () => {}, abrirVaga: () => {},
    aviso: (texto: string, e?: boolean) => { avisos.push({ texto, erro: e }); },
    perguntar: async (p: Pergunta) => { perguntas.push(p); return resposta; },
    recarregar: vi.fn(async () => {}),
    api: vi.fn(async (caminho: string, metodo = 'GET', corpo?: any) => {
      if (metodo === 'GET') return caminho.endsWith('/limites') ? LIMITES : candidaturas;
      gravacoes.push({ caminho, corpo });
      if (erro) throw erro;
      return {};
    }),
  } as unknown as App;
  render(<Contexto.Provider value={app}><TelaFila /></Contexto.Provider>);
  return { gravacoes, perguntas, avisos };
}
const cartao = async (id: string) => (await screen.findByTestId(`cand-${id}`)) as HTMLElement;
const botao = (c: HTMLElement, nome: string) => within(c).queryByRole('button', { name: nome }) as HTMLButtonElement | null;
const clicar = async (el: HTMLElement) => { await act(async () => { fireEvent.click(el); }); };
const conferida = async (c: HTMLElement) => { await within(c).findByText('Conferida nesta tela'); };

describe('Fila: aprovar', () => {
  it('aprova com confirmação e envia só {impressao}, a calculada nesta tela', async () => {
    const { gravacoes, perguntas, avisos } = montar({ candidaturas: [cand('a', 'preparada')] });
    const c = await cartao('a');
    await conferida(c);
    await clicar(botao(c, 'Aprovar este pacote')!);
    expect(perguntas[0].titulo).toBe('Aprovar e enviar este pacote?');
    expect(gravacoes).toEqual([{ caminho: '/api/v1/candidaturas/cand-a/aprovar', corpo: { impressao: IMP } }]);
    expect(avisos).toContainEqual({ texto: 'Aprovado. O app envia este pacote hoje, uma vez.', erro: undefined });
  });

  it('impressão que não bate: botão desabilitado e nada é enviado', async () => {
    const { gravacoes } = montar({ candidaturas: [cand('b', 'preparada', { impressao: 'sha256:v1:' + '0'.repeat(64) })] });
    const c = await cartao('b');
    await within(c).findByText(/Não confere com o pacote mostrado/);
    const b = botao(c, 'Aprovar este pacote')!;
    expect(b.disabled).toBe(true);
    await clicar(b);
    expect(gravacoes).toEqual([]);
  });

  it('enquanto confere, o botão fica desabilitado', async () => {
    // Segura o cálculo da impressão (digest que não termina) para ver o estado "Conferindo…".
    const digest = vi.spyOn(globalThis.crypto.subtle, 'digest').mockImplementation(() => new Promise<ArrayBuffer>(() => {}));
    try {
      const { gravacoes } = montar({ candidaturas: [cand('a', 'preparada')] });
      const c = await cartao('a');
      expect(within(c).getByText('Conferindo…')).toBeTruthy();
      const b = botao(c, 'Aprovar este pacote')!;
      expect(b.disabled).toBe(true);
      await clicar(b);
      expect(gravacoes).toEqual([]);
    } finally { digest.mockRestore(); }
  });

  it('cancelar a confirmação não aprova', async () => {
    const { gravacoes } = montar({ candidaturas: [cand('a', 'preparada')], resposta: null });
    const c = await cartao('a');
    await conferida(c);
    await clicar(botao(c, 'Aprovar este pacote')!);
    expect(gravacoes).toEqual([]);
  });

  it('409 (o pacote mudou) aparece em texto claro', async () => {
    const { avisos } = montar({ candidaturas: [cand('a', 'preparada')], erro: new ErroApi('o conteúdo mudou; revise de novo', 409, { erro: 'o conteúdo mudou; revise de novo', estado: 'preparada', impressaoAtual: 'sha256:v1:' + '1'.repeat(64) }) });
    const c = await cartao('a');
    await conferida(c);
    await clicar(botao(c, 'Aprovar este pacote')!);
    expect(avisos).toContainEqual({ texto: 'O pacote mudou desde que você abriu. A fila foi atualizada; revise de novo antes de aprovar.', erro: true });
  });

  it('429 (limite do dia) aparece com a plataforma, o uso e quando renova', async () => {
    const { avisos } = montar({ candidaturas: [cand('a', 'preparada')], erro: new ErroApi('limite diário atingido', 429, { erro: 'limite diário atingido', plataforma: 'linkedin', limite: 30, usadas: 30, renovaEm: '2026-11-10T00:00:00-03:00' }) });
    const c = await cartao('a');
    await conferida(c);
    await clicar(botao(c, 'Aprovar este pacote')!);
    expect(avisos).toContainEqual({ texto: 'Limite de hoje no LinkedIn atingido (30 de 30). Renova amanhã às 00:00.', erro: true });
  });
});

describe('Fila: outras ações', () => {
  const aprovada = cand('ap', 'aprovada', { aprovacao: { em: '2026-11-09T10:05:00-03:00', impressao: IMP, validaAte: '2026-11-09T23:59:59-03:00', fuso: 'America/Sao_Paulo' } });

  it('aprovada: "Tirar aprovação" vai direto (só reduz o risco), com corpo {}', async () => {
    const { gravacoes, perguntas } = montar({ candidaturas: [aprovada] });
    await clicar(botao(await cartao('ap'), 'Tirar aprovação')!);
    expect(perguntas).toEqual([]);
    expect(gravacoes).toEqual([{ caminho: '/api/v1/candidaturas/cand-ap/revogar', corpo: {} }]);
  });

  it('cancelar pede confirmação e manda {}', async () => {
    const { gravacoes, perguntas } = montar({ candidaturas: [cand('a', 'preparada')] });
    await clicar(botao(await cartao('a'), 'Cancelar candidatura')!);
    expect(perguntas[0].titulo).toBe('Cancelar esta candidatura?');
    expect(gravacoes).toEqual([{ caminho: '/api/v1/candidaturas/cand-a/cancelar', corpo: {} }]);
  });

  it('reabrir e marcar como enviada pedem confirmação e mandam {}', async () => {
    const bloq = cand('bl', 'bloqueada', { bloqueio: { motivo: 'cpf', pendencia: 'Digitar o CPF e enviar', em: '2026-11-09T11:00:00-03:00' } });
    const { gravacoes, perguntas } = montar({ candidaturas: [bloq] });
    const c = await cartao('bl');
    await clicar(botao(c, 'Marcar como enviada')!);
    await clicar(botao(c, 'Reabrir')!);
    expect(perguntas.map(p => p.titulo)).toEqual(['Você enviou pela página?', 'Reabrir esta candidatura?']);
    expect(gravacoes).toEqual([
      { caminho: '/api/v1/candidaturas/cand-bl/concluir-manual', corpo: {} },
      { caminho: '/api/v1/candidaturas/cand-bl/reabrir', corpo: {} },
    ]);
  });

  it('incerta: confirmação explica a conferência na plataforma e não oferece cancelar', async () => {
    const { perguntas } = montar({ candidaturas: [cand('in', 'incerta')], resposta: null });
    const c = await cartao('in');
    expect(botao(c, 'Cancelar candidatura')).toBeNull();
    await clicar(botao(c, 'Marcar como enviada')!);
    await clicar(botao(c, 'Reabrir')!);
    expect(perguntas[0].titulo).toBe('Você conferiu que foi enviada?');
    expect(perguntas[1].texto).toMatch(/só se você conferiu no LinkedIn que ela não foi enviada/);
  });

  it('cada estado oferece só as ações que o contrato permite', async () => {
    montar({ candidaturas: [cand('p', 'preparada'), aprovada, cand('e', 'enviando'), cand('f', 'enviada'), cand('x', 'falhou', { resultado: { em: '', tipo: 'falhou', detalhe: 'erro' } }), cand('c', 'cancelada')] });
    const acoes = async (id: string) => within(await cartao(id)).queryAllByRole('button').map(b => b.textContent).filter(t => t !== 'Editar pacote');
    expect(await acoes('p')).toEqual(['Aprovar este pacote', 'Cancelar candidatura']);
    expect(await acoes('ap')).toEqual(['Tirar aprovação', 'Cancelar candidatura']);
    expect(await acoes('e')).toEqual([]);
    expect(await acoes('f')).toEqual([]);
    expect(await acoes('x')).toEqual(['Reabrir', 'Cancelar candidatura']);
    expect(await acoes('c')).toEqual([]);
  });
});
