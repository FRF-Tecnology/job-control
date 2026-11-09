// Fila: preparar a candidatura e editar o pacote (P5, jc-a2y.6.3.3). O pacote sai do perfil e da pasta de
// currículos, sem inventar resposta; editar uma aprovada desfaz a aprovação (pede confirmação). Só dados fictícios.
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, ErroApi, type App, type Pergunta } from '../app/contexto';
import { TelaFila } from './Fila';

afterEach(cleanup);

const IMP = 'sha256:v1:2238b4657006a7556b71d4428ea8ca0b71e6158d730a2ed276d3d881f6198129';
const CONTEUDO = {
  curriculo: { arquivo: 'curriculo-exemplo.pdf', sha256: 'a'.repeat(64) }, idioma: 'pt',
  contato: { nome: 'Ana Exemplo', email: 'ana@exemplo.com' }, pretensao: { valor: 5000.5, moeda: 'BRL', tipo: 'CLT' },
  respostas: [{ pergunta: 'Anos com React? "ação"\n', resposta: '3', origem: 'respostasPadrao' }], mensagem: null,
};
const cand = (id: string, estado: string, extra: Record<string, unknown> = {}) => ({
  id: `cand-${id}`, vagaId: 'ext-exemplo-1', plataforma: 'linkedin', url: 'https://www.linkedin.com/jobs/view/123', estado,
  conteudo: CONTEUDO, impressao: IMP, aprovacao: null, bloqueio: null, resultado: null, versao: 4,
  criadaEm: '2026-11-09T10:00:00-03:00', atualizadaEm: '2026-11-09T10:00:00-03:00', ...extra,
});
const LIMITES = { linkedin: { limite: 30, usadas: 0, renovaEm: '2026-11-10T00:00:00-03:00' }, gupy: { limite: 30, usadas: 0, renovaEm: '2026-11-10T00:00:00-03:00' }, inhire: { limite: 30, usadas: 0, renovaEm: '2026-11-10T00:00:00-03:00' } };
const PERFIL = { nome: 'Ana Exemplo', email: 'ana@exemplo.com', pretensaoCLT: 6000, curriculos: { PT: 'curriculos/cv-pt.pdf' }, respostasPadrao: [{ pergunta: 'Anos com React?', resposta: '3' }] };

function montar({ candidaturas = [] as any[], resposta = 'ok' as string | null, erro = null as null | ErroApi } = {}) {
  const gravacoes: { caminho: string; metodo: string; corpo: any }[] = [];
  const perguntas: Pergunta[] = [];
  const avisos: { texto: string; erro?: boolean }[] = [];
  const app = {
    estado: {
      vagas: [
        { id: 'ext-exemplo-1', empresa: 'Empresa Exemplo', titulo: 'Pessoa Dev React', etapa: 'aplicar' },
        { id: 'ext-exemplo-2', empresa: 'Empresa Dois', titulo: 'Pessoa Dev Node', etapa: 'aplicar', contrato: 'CLT' },
        { id: 'ext-exemplo-3', empresa: 'Empresa Três', titulo: 'Fora da fila', etapa: 'descoberta' },
      ],
      perfil: PERFIL, curriculos: [{ nome: 'cv-pt.pdf' }, { nome: 'cv-en.pdf' }], eventos: [], plataformas: [],
    },
    hoje: '2026-11-09', foco: 'ambos', rota: 'fila', vaga: null, limpar: () => {}, abrirVaga: () => {},
    aviso: (texto: string, e?: boolean) => { avisos.push({ texto, erro: e }); },
    perguntar: async (p: Pergunta) => { perguntas.push(p); return resposta; },
    recarregar: vi.fn(async () => {}),
    api: vi.fn(async (caminho: string, metodo = 'GET', corpo?: any) => {
      if (metodo === 'GET') return caminho.endsWith('/limites') ? LIMITES : candidaturas;
      gravacoes.push({ caminho, metodo, corpo });
      if (erro) throw erro;
      return {};
    }),
  } as unknown as App;
  render(<Contexto.Provider value={app}><TelaFila /></Contexto.Provider>);
  return { gravacoes, perguntas, avisos, app };
}
const prontas = async () => (await screen.findByRole('region', { name: 'Prontas para preparar' })) as HTMLElement;
const clicar = async (el: HTMLElement) => { await act(async () => { fireEvent.click(el); }); };
const enviar = async (form: HTMLElement) => { await act(async () => { fireEvent.submit(form); }); };

describe('Fila: preparar candidatura', () => {
  it('lista as vagas em Aplicar sem candidatura ativa (a que já tem uma não aparece)', async () => {
    montar({ candidaturas: [cand('a', 'preparada')] });
    const r = await prontas();
    expect(within(r).getByText('Empresa Dois — Pessoa Dev Node')).toBeTruthy();
    expect(within(r).queryByText(/Empresa Exemplo/)).toBeNull();
    expect(within(r).queryByText(/Fora da fila/)).toBeNull();
  });

  it('candidatura cancelada não conta como ativa: a vaga volta a aparecer para preparar', async () => {
    montar({ candidaturas: [cand('a', 'cancelada')] });
    expect(within(await prontas()).getByText('Empresa Exemplo — Pessoa Dev React')).toBeTruthy();
  });

  it('o formulário vem do perfil e o POST leva só o pacote (currículo pelo nome, sem sha256)', async () => {
    const { gravacoes, avisos, app } = montar();
    const r = await prontas();
    const linha = within(r).getByText('Empresa Dois — Pessoa Dev Node').closest('li') as HTMLElement;
    await clicar(within(linha).getByRole('button', { name: 'Preparar candidatura' }));
    const form = within(linha).getByRole('form', { name: 'Pacote da candidatura' });
    expect((within(form).getByLabelText('Currículo') as HTMLSelectElement).value).toBe('cv-pt.pdf');
    expect((within(form).getByLabelText('Nome') as HTMLInputElement).value).toBe('Ana Exemplo');
    await enviar(form);
    expect(gravacoes).toEqual([{ caminho: '/api/v1/candidaturas', metodo: 'POST', corpo: {
      vagaId: 'ext-exemplo-2',
      conteudo: { curriculo: { arquivo: 'cv-pt.pdf' }, idioma: 'pt', contato: { nome: 'Ana Exemplo', email: 'ana@exemplo.com' },
        pretensao: { valor: 6000, moeda: 'BRL', tipo: 'CLT' }, respostas: [{ pergunta: 'Anos com React?', resposta: '3', origem: 'respostasPadrao' }], mensagem: null },
    } }]);
    expect(avisos).toContainEqual({ texto: 'Candidatura preparada. Revise o pacote e aprove.', erro: undefined });
    expect(app.recarregar).toHaveBeenCalled();
  });

  it('currículo que não é o do perfil para o idioma aparece destacado ao lado do campo', async () => {
    montar();
    const linha = within(await prontas()).getByText('Empresa Dois — Pessoa Dev Node').closest('li') as HTMLElement;
    await clicar(within(linha).getByRole('button', { name: 'Preparar candidatura' }));
    const form = within(linha).getByRole('form', { name: 'Pacote da candidatura' });
    expect(within(form).queryByText(/Confira qual vai/)).toBeNull(); // português: o do perfil está na pasta
    fireEvent.change(within(form).getByLabelText('Idioma'), { target: { value: 'en' } });
    expect(within(form).getByText('O perfil não tem currículo para inglês. Confira qual vai.')).toBeTruthy();
  });

  it('409 de duplicata aparece em texto claro', async () => {
    const { avisos } = montar({ erro: new ErroApi('já existe uma candidatura ativa para esta vaga', 409, { erro: 'já existe uma candidatura ativa para esta vaga', existente: 'cand-x' }) });
    const linha = within(await prontas()).getByText('Empresa Dois — Pessoa Dev Node').closest('li') as HTMLElement;
    await clicar(within(linha).getByRole('button', { name: 'Preparar candidatura' }));
    await enviar(within(linha).getByRole('form', { name: 'Pacote da candidatura' }));
    expect(avisos).toContainEqual({ texto: 'Esta vaga já tem uma candidatura ativa ou já foi enviada. Veja na fila.', erro: true });
  });
});

describe('Fila: editar o pacote', () => {
  const editar = async (id: string) => {
    const c = await screen.findByTestId(`cand-${id}`);
    await clicar(within(c).getByRole('button', { name: 'Editar pacote' }));
    return within(c).getByRole('form', { name: 'Pacote da candidatura' });
  };

  it('preparada: PUT com o pacote e a versão lida', async () => {
    const { gravacoes, perguntas } = montar({ candidaturas: [cand('a', 'preparada')] });
    const form = await editar('a');
    fireEvent.change(within(form).getByLabelText('Mensagem'), { target: { value: 'Olá, tenho interesse.' } });
    await enviar(form);
    expect(perguntas).toEqual([]);
    expect(gravacoes).toHaveLength(1);
    expect(gravacoes[0].caminho).toBe('/api/v1/candidaturas/cand-a/conteudo');
    expect(gravacoes[0].metodo).toBe('PUT');
    expect(gravacoes[0].corpo.versao).toBe(4);
    expect(gravacoes[0].corpo.conteudo.mensagem).toBe('Olá, tenho interesse.');
    expect(gravacoes[0].corpo.conteudo.curriculo).toEqual({ arquivo: 'curriculo-exemplo.pdf' });
  });

  it('aprovada: pede confirmação porque desfaz a aprovação; cancelar não grava', async () => {
    const aprovada = cand('b', 'aprovada', { aprovacao: { em: '2026-11-09T10:05:00-03:00', impressao: IMP, validaAte: '2026-11-09T23:59:59-03:00', fuso: 'America/Sao_Paulo' } });
    const { gravacoes, perguntas } = montar({ candidaturas: [aprovada], resposta: null });
    await enviar(await editar('b'));
    expect(perguntas[0].titulo).toBe('Editar desfaz a aprovação');
    expect(gravacoes).toEqual([]);
  });

  it('aprovada confirmada: grava', async () => {
    const aprovada = cand('b', 'aprovada', { aprovacao: { em: '2026-11-09T10:05:00-03:00', impressao: IMP, validaAte: '2026-11-09T23:59:59-03:00', fuso: 'America/Sao_Paulo' } });
    const { gravacoes } = montar({ candidaturas: [aprovada], resposta: 'ok' });
    await enviar(await editar('b'));
    expect(gravacoes.map(g => g.metodo)).toEqual(['PUT']);
  });

  it('versão velha (409) aparece em texto claro', async () => {
    const { avisos } = montar({ candidaturas: [cand('a', 'preparada')], erro: new ErroApi('a candidatura mudou; recarregue', 409, { erro: 'a candidatura mudou; recarregue', estado: 'preparada' }) });
    await enviar(await editar('a'));
    expect(avisos).toContainEqual({ texto: 'O pacote mudou enquanto você editava. A fila foi atualizada; abra e edite de novo.', erro: true });
  });

  it('pergunta sensível digitada mostra o erro ao lado da resposta e bloqueia salvar', async () => {
    const { gravacoes } = montar({ candidaturas: [cand('a', 'preparada')] });
    const form = await editar('a');
    fireEvent.change(within(form).getByLabelText('Pergunta 1'), { target: { value: 'Chave PIX para pagamento' } });
    expect(within(form).getByText('Só você responde isso na página (dado bancário). Remova esta resposta.')).toBeTruthy();
    expect((within(form).getByRole('button', { name: 'Salvar pacote' }) as HTMLButtonElement).disabled).toBe(true);
    await enviar(form);
    expect(gravacoes).toEqual([]);
  });

  it('resposta da IA aparece marcada e, editada, passa a ser sua', async () => {
    const comIA = cand('c', 'preparada', { conteudo: { ...CONTEUDO, respostas: [{ pergunta: 'Por que esta vaga?', resposta: 'Texto sugerido', origem: 'ia' }] } });
    const { gravacoes } = montar({ candidaturas: [comIA] });
    const form = await editar('c');
    expect(within(form).getByText('IA')).toBeTruthy();
    fireEvent.change(within(form).getByLabelText('Resposta 1'), { target: { value: 'Texto revisado por mim' } });
    expect(within(form).getByText('Você')).toBeTruthy();
    await enviar(form);
    expect(gravacoes[0].corpo.conteudo.respostas).toEqual([{ pergunta: 'Por que esta vaga?', resposta: 'Texto revisado por mim', origem: 'pessoa' }]);
  });
});
