// Fila: a conferência da impressão não pisca "Não confere" quando a lista nova chega com outra versão, e o "Detalhes
// técnicos" que a pessoa abriu continua aberto depois da releitura (jc-t82, run 42731298312). Os dois pares pacote →
// impressão são os do Go (impressao.test.ts). Só dados fictícios.
import { act, cleanup, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, type App } from '../app/contexto';
import { TelaFila } from './Fila';

afterEach(cleanup);

const V1 = {
  id: 'cand-a', vagaId: 'ext-exemplo-1', plataforma: 'linkedin', url: 'https://www.linkedin.com/jobs/view/123', estado: 'preparada',
  conteudo: { curriculo: { arquivo: 'curriculo-exemplo.pdf', sha256: 'a'.repeat(64) }, idioma: 'pt', contato: { nome: 'Ana Exemplo', email: 'ana@exemplo.com' },
    pretensao: { valor: 5000.5, moeda: 'BRL', tipo: 'CLT' }, respostas: [{ pergunta: 'Anos com React? "ação"\n', resposta: '3', origem: 'respostasPadrao' }], mensagem: null },
  impressao: 'sha256:v1:2238b4657006a7556b71d4428ea8ca0b71e6158d730a2ed276d3d881f6198129',
  aprovacao: null, bloqueio: null, resultado: null, versao: 1, criadaEm: '2026-11-09T10:00:00-03:00', atualizadaEm: '2026-11-09T10:00:00-03:00',
};
// A mesma candidatura depois de "editada": outro pacote, com a impressão que o Go dá para ele.
const V2 = {
  ...V1, vagaId: 'ext-form-1', plataforma: 'formulario', url: 'https://docs.google.com/forms/d/e/abc/viewform', versao: 2,
  conteudo: { curriculo: { arquivo: 'cv.pdf' }, idioma: 'pt', respostas: [{ pergunta: 'Nível de inglês', resposta: 'Avançado', origem: 'respostasPadrao', perguntaId: 'q3' }] },
  estrutura: { hash: 'sha256:v1:ab0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd', urlFinal: 'https://docs.google.com/forms/d/e/abc/viewform',
    perguntas: [{ id: 'q1', texto: 'Qual o seu e-mail?', tipo: 'texto-curto', obrigatoria: true }, { id: 'q3', texto: 'Nível de inglês', tipo: 'escolha-unica', obrigatoria: true, opcoes: ['Básico', 'Avançado'] }] },
  pendentes: ['q1'], impressao: 'sha256:v1:33be9501e5ffff806d5334abce6f820ba9a5d7b3d124ceebefe7e58eb87bd82b',
};

describe('Fila: conferência da impressão ao reler', () => {
  it('a lista nova não pisca "Não confere" e o "Detalhes técnicos" aberto continua aberto', async () => {
    let atual: any = V1;
    const base = {
      estado: { vagas: [{ id: 'ext-exemplo-1', empresa: 'Empresa Exemplo', titulo: 'Pessoa Dev', etapa: 'aplicar' }], perfil: {}, curriculos: [], eventos: [], plataformas: [] },
      hoje: '2026-11-09', foco: 'ambos', rota: 'fila', vaga: null, limpar: () => {}, abrirVaga: () => {}, aviso: () => {}, perguntar: async () => 'ok', recarregar: async () => {},
      api: vi.fn(async (caminho: string) => caminho.endsWith('/limites') ? {} : caminho.endsWith('/envio') ? { ligado: false, pausado: true, enviandoAgora: null, desfechos: [] } : [atual]),
    };
    const r = render(<Contexto.Provider value={base as unknown as App}><TelaFila /></Contexto.Provider>);
    const cartao = await screen.findByTestId('cand-a');
    await within(cartao).findByText('Conferida nesta tela');
    const det = cartao.querySelector('details.tecnicos') as HTMLDetailsElement;
    det.open = true; // a pessoa abriu

    const viuDiferente: string[] = [];
    const obs = new MutationObserver(() => { if (document.querySelector('.conferida.diferente')) viuDiferente.push('diferente'); });
    obs.observe(document.body, { subtree: true, childList: true, characterData: true });
    // A casca relê (a versão dos dados mudou): a lista nova chega com o pacote editado.
    atual = V2;
    await act(async () => { r.rerender(<Contexto.Provider value={{ ...base, estado: { ...base.estado } } as unknown as App}><TelaFila /></Contexto.Provider>); });
    await within(screen.getByTestId('cand-a')).findByText('Conferida nesta tela');
    obs.disconnect();

    expect(viuDiferente).toEqual([]);
    expect((screen.getByTestId('cand-a').querySelector('details.tecnicos') as HTMLDetailsElement).open).toBe(true);
  });
});
