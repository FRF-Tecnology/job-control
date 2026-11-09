// Tela Fila (P5, jc-a2y.6.3.1): só leitura. Mostra cada estado com o que ele pede, o pacote exato, a impressão
// conferida nesta tela e os limites por plataforma. Nada grava. Só dados fictícios.
import { cleanup, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, type App } from '../app/contexto';
import { TelaFila } from './Fila';

afterEach(cleanup);

const IMPRESSAO_DO_GO = 'sha256:v1:2238b4657006a7556b71d4428ea8ca0b71e6158d730a2ed276d3d881f6198129';
const CONTEUDO = {
  curriculo: { arquivo: 'curriculo-exemplo.pdf', sha256: 'a'.repeat(64) }, idioma: 'pt',
  contato: { nome: 'Ana Exemplo', email: 'ana@exemplo.com' },
  pretensao: { valor: 5000.5, moeda: 'BRL', tipo: 'CLT' },
  respostas: [{ pergunta: 'Anos com React? "ação"\n', resposta: '3', origem: 'respostasPadrao' }],
  mensagem: null,
};
const cand = (id: string, estado: string, extra: Record<string, unknown> = {}) => ({
  id: `cand-${id}`, vagaId: 'ext-exemplo-1', plataforma: 'linkedin', url: 'https://www.linkedin.com/jobs/view/123', estado,
  conteudo: CONTEUDO, impressao: IMPRESSAO_DO_GO, aprovacao: null, bloqueio: null, resultado: null, versao: 1,
  criadaEm: '2026-11-09T10:00:00-03:00', atualizadaEm: '2026-11-09T10:00:00-03:00', ...extra,
});
const LIMITES = {
  linkedin: { limite: 30, usadas: 3, renovaEm: '2026-11-10T00:00:00-03:00' },
  gupy: { limite: 30, usadas: 0, renovaEm: '2026-11-10T00:00:00-03:00' },
  inhire: { limite: 30, usadas: 0, renovaEm: '2026-11-10T00:00:00-03:00' },
};
const TODAS = [
  cand('prep', 'preparada'),
  cand('apro', 'aprovada', { aprovacao: { em: '2026-11-09T10:05:00-03:00', impressao: IMPRESSAO_DO_GO, validaAte: '2026-11-09T23:59:59-03:00', fuso: 'America/Sao_Paulo' } }),
  cand('envi', 'enviando'),
  cand('feit', 'enviada', { resultado: { em: '2026-11-09T11:00:00-03:00', tipo: 'enviada', detalhe: 'enviada pela plataforma' } }),
  cand('bloq', 'bloqueada', { bloqueio: { motivo: 'cpf', pendencia: 'Digitar o CPF e enviar', em: '2026-11-09T11:00:00-03:00' } }),
  cand('falh', 'falhou', { resultado: { em: '2026-11-09T11:00:00-03:00', tipo: 'falhou', detalhe: 'currículo mudou' } }),
  cand('ince', 'incerta'),
  cand('canc', 'cancelada'),
];

function montar({ candidaturas = TODAS as any[], falhar = false } = {}) {
  const chamadas: string[] = [];
  const app = {
    estado: { vagas: [{ id: 'ext-exemplo-1', empresa: 'Empresa Exemplo', titulo: 'Pessoa Dev React', etapa: 'aplicar' }], eventos: [], plataformas: [] },
    hoje: '2026-11-09', foco: 'ambos', rota: 'fila', vaga: null, aviso: vi.fn(), limpar: () => {}, abrirVaga: () => {},
    api: vi.fn(async (caminho: string, metodo = 'GET') => {
      chamadas.push(`${metodo} ${caminho}`);
      if (falhar) throw new Error('sem conexão com os dados');
      if (caminho === '/api/v1/candidaturas') return candidaturas;
      if (caminho === '/api/v1/candidaturas/limites') return LIMITES;
      throw new Error(caminho);
    }),
  } as unknown as App;
  render(<Contexto.Provider value={app}><TelaFila /></Contexto.Provider>);
  return { chamadas };
}
const cartao = async (id: string) => (await screen.findByTestId(`cand-${id}`)) as HTMLElement;

describe('Fila: só leitura', () => {
  it('mostra os 8 estados, cada um com o que pede', async () => {
    montar();
    expect(within(await cartao('prep')).getByText('Para revisar')).toBeTruthy();
    expect(within(await cartao('apro')).getByText(/Aprovada até 23:59/)).toBeTruthy();
    expect(within(await cartao('envi')).getByText('Enviando')).toBeTruthy();
    expect(within(await cartao('feit')).getByText('Enviada')).toBeTruthy();
    const bloq = await cartao('bloq');
    expect(within(bloq).getByText('Bloqueada · CPF')).toBeTruthy();
    expect(within(bloq).getByText('Digitar o CPF e enviar')).toBeTruthy();
    expect(within(await cartao('falh')).getByText(/currículo mudou/)).toBeTruthy();
    expect(within(await cartao('ince')).getByText(/Confira no LinkedIn/)).toBeTruthy();
    expect(within(await cartao('canc')).getByText('Cancelada')).toBeTruthy();
  });

  it('o que pede a pessoa vem primeiro: bloqueada, incerta e falhou antes das outras', async () => {
    montar();
    await cartao('prep');
    const ordem = [...document.querySelectorAll('[data-testid^="cand-"]')].map(e => e.getAttribute('data-testid'));
    expect(ordem.slice(0, 3)).toEqual(['cand-bloq', 'cand-ince', 'cand-falh']);
  });

  it('mostra o pacote exato que vai para a vaga', async () => {
    montar({ candidaturas: [cand('prep', 'preparada')] });
    const c = await cartao('prep');
    expect(within(c).getByText('Empresa Exemplo — Pessoa Dev React')).toBeTruthy();
    expect(within(c).getByText('curriculo-exemplo.pdf')).toBeTruthy();
    expect(within(c).getByText('Ana Exemplo · ana@exemplo.com')).toBeTruthy();
    expect(within(c).getByText(/R\$\s?5\.000,50 · CLT/)).toBeTruthy();
    expect(within(c).getByText(/Anos com React\?/)).toBeTruthy();
    expect(within(c).getByText('3')).toBeTruthy();
    expect(within(c).getByText('Respostas padrão')).toBeTruthy();
    expect(within(c).getByText('Sem mensagem')).toBeTruthy();
  });

  it('confere a impressão nesta tela e avisa quando não bate com a do servidor', async () => {
    montar({ candidaturas: [cand('ok', 'preparada'), cand('ruim', 'preparada', { impressao: 'sha256:v1:' + '0'.repeat(64) })] });
    const ok = await cartao('ok');
    expect(within(ok).getByText(IMPRESSAO_DO_GO)).toBeTruthy();
    expect(await within(ok).findByText('Conferida nesta tela')).toBeTruthy();
    const ruim = await cartao('ruim');
    expect(await within(ruim).findByText(/Não confere com o pacote mostrado/)).toBeTruthy();
  });

  it('mostra os limites por plataforma e quando renovam', async () => {
    montar();
    const limites = await screen.findByRole('region', { name: 'Limite de hoje' });
    expect(within(limites).getByText('LinkedIn')).toBeTruthy();
    expect(within(limites).getByText('3 de 30')).toBeTruthy();
    expect(within(limites).getAllByText(/Renova amanhã às 00:00/).length).toBeGreaterThan(0);
  });

  it('fila vazia e erro de leitura têm estado próprio', async () => {
    montar({ candidaturas: [] });
    expect(await screen.findByText('Nenhuma candidatura na fila')).toBeTruthy();
    cleanup();
    montar({ falhar: true });
    expect(await screen.findByText('Não deu para ler a fila')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Tentar de novo' })).toBeTruthy();
  });

  it('só lê: nenhuma chamada grava', async () => {
    const { chamadas } = montar();
    await cartao('prep');
    expect(chamadas.every(c => c.startsWith('GET '))).toBe(true);
  });
});
