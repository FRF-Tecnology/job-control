// Formulário externo na Fila (jc-a2y.7.2.4) com a API interceptada: "Ler formulário" (corpo {}), perguntas em texto,
// o que veio das respostas padrão, campo só para pendente (sensível nunca), salvar pelo PUT /conteudo com perguntaId,
// aprovar só sem pendente, pré-preenchimento ignorado, Typeform "envie à mão", bloqueio com motivo e 409/429/404 em
// frase fixa. Só dados fictícios; a impressão é a do Go para esta mesma candidatura (impressao.test.ts).
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, ErroApi, type App, type Pergunta } from '../app/contexto';
import { TelaFila } from './Fila';

afterEach(cleanup);

const IMP_FORM = 'sha256:v1:33be9501e5ffff806d5334abce6f820ba9a5d7b3d124ceebefe7e58eb87bd82b';
const URL_FORM = 'https://docs.google.com/forms/d/e/abc/viewform';
const CONTEUDO = { curriculo: { arquivo: 'cv.pdf' }, idioma: 'pt', respostas: [{ pergunta: 'Nível de inglês', resposta: 'Avançado', origem: 'respostasPadrao', perguntaId: 'q3' }] };
const ESTRUTURA = { hash: 'sha256:v1:ab0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcd', urlFinal: URL_FORM,
  perguntas: [{ id: 'q1', texto: 'Qual o seu e-mail?', tipo: 'texto-curto', obrigatoria: true }, { id: 'q3', texto: 'Nível de inglês', tipo: 'escolha-unica', obrigatoria: true, opcoes: ['Básico', 'Avançado'] }] };
const cand = (extra: Record<string, unknown> = {}) => ({
  id: 'cand-f1', vagaId: 'ext-form-1', plataforma: 'formulario', url: URL_FORM, estado: 'preparada', conteudo: CONTEUDO, impressao: IMP_FORM,
  aprovacao: null, bloqueio: null, resultado: null, versao: 3, criadaEm: '2026-11-09T10:00:00-03:00', atualizadaEm: '2026-11-09T10:00:00-03:00',
  estrutura: ESTRUTURA, pendentes: ['q1'], preenchimentoIgnorado: false, ...extra,
});
const LIMITES = { formulario: { limite: 30, usadas: 0, renovaEm: '2026-11-10T00:00:00-03:00' } };

function montar({ candidaturas = [cand()] as any[], erro = null as null | ErroApi, retorno = {} as any } = {}) {
  const gravacoes: { caminho: string; metodo: string; corpo: any }[] = [];
  const avisos: { texto: string; erro?: boolean }[] = [];
  const app = {
    estado: { vagas: [{ id: 'ext-form-1', empresa: 'Empresa Teste', titulo: 'Dev Go', etapa: 'aplicar' }], perfil: {}, curriculos: [], eventos: [], plataformas: [] },
    hoje: '2026-11-09', foco: 'ambos', rota: 'fila', vaga: null, limpar: () => {}, abrirVaga: () => {},
    aviso: (texto: string, e?: boolean) => { avisos.push({ texto, erro: e }); },
    perguntar: async (_p: Pergunta) => 'ok', recarregar: vi.fn(async () => {}),
    api: vi.fn(async (caminho: string, metodo = 'GET', corpo?: any) => {
      if (metodo === 'GET') return caminho.endsWith('/limites') ? LIMITES : caminho.endsWith('/envio') ? { ligado: false, pausado: true, enviandoAgora: null, desfechos: [] } : candidaturas;
      gravacoes.push({ caminho, metodo, corpo });
      if (erro) throw erro;
      return retorno;
    }),
  } as unknown as App;
  render(<Contexto.Provider value={app}><TelaFila /></Contexto.Provider>);
  return { gravacoes, avisos };
}
const cartao = async () => (await screen.findByTestId('cand-f1')) as HTMLElement;
const clicar = async (el: HTMLElement) => { await act(async () => { fireEvent.click(el); }); };
const aprovar = (c: HTMLElement) => within(c).getByRole('button', { name: 'Aprovar este pacote' }) as HTMLButtonElement;

describe('Fila: formulário externo', () => {
  it('sem leitura: "Ler formulário" manda {} só para ler-formulario; aprovar fica travado', async () => {
    const lido = cand();
    const { gravacoes, avisos } = montar({ candidaturas: [cand({ estrutura: null, pendentes: [], impressao: 'sha256:v1:' + '0'.repeat(64) })], retorno: lido });
    const c = await cartao();
    expect(within(c).getByText('Formulário')).toBeTruthy();
    expect(aprovar(c).disabled).toBe(true);
    await clicar(within(c).getByRole('button', { name: 'Ler formulário' }));
    expect(gravacoes).toEqual([{ caminho: '/api/v1/candidaturas/cand-f1/ler-formulario', metodo: 'POST', corpo: {} }]);
    expect(avisos).toContainEqual({ texto: 'Formulário lido: 2 perguntas, 1 pendente para responder.', erro: false });
  });

  it('lido: perguntas em texto, a resposta padrão com a origem, campo só na pendente e aprovar travado', async () => {
    montar();
    const c = await cartao();
    await within(c).findByText('Conferida nesta tela'); // impressão com a estrutura bate com a do servidor
    expect(within(c).getByRole('heading', { name: 'Perguntas do formulário (2)' })).toBeTruthy();
    expect(within(c).getByText('Falta responder 1 pergunta obrigatória para aprovar.')).toBeTruthy();
    expect(within(c).getByText('Escolha única · obrigatória · opções: Básico, Avançado')).toBeTruthy();
    const form = c.querySelector('.form-ext') as HTMLElement;
    expect(within(form).getByText('Avançado')).toBeTruthy();
    expect(within(form).getByText('Respostas padrão')).toBeTruthy();
    expect(within(c).getAllByRole('textbox', { name: /^Resposta: / })).toHaveLength(1);
    expect(aprovar(c).disabled).toBe(true);
  });

  it('responder a pendente: PUT /conteudo com o pacote inteiro, a resposta com perguntaId e origem pessoa, e a versão lida', async () => {
    const { gravacoes, avisos } = montar();
    const c = await cartao();
    fireEvent.change(within(c).getByRole('textbox', { name: 'Resposta: Qual o seu e-mail?' }), { target: { value: ' pessoa.teste@exemplo.com ' } });
    await clicar(within(c).getByRole('button', { name: 'Salvar resposta' }));
    expect(gravacoes).toEqual([{ caminho: '/api/v1/candidaturas/cand-f1/conteudo', metodo: 'PUT', corpo: {
      versao: 3,
      conteudo: { ...CONTEUDO, respostas: [CONTEUDO.respostas[0], { pergunta: 'Qual o seu e-mail?', resposta: 'pessoa.teste@exemplo.com', origem: 'pessoa', perguntaId: 'q1' }] },
    } }]);
    expect(avisos).toContainEqual({ texto: 'Respostas salvas.', erro: undefined });
  });

  it('pendente de escolha única: só as opções lidas, num select', async () => {
    montar({ candidaturas: [cand({ conteudo: { ...CONTEUDO, respostas: [] }, pendentes: ['q1', 'q3'], impressao: 'x' })] });
    const c = await cartao();
    const sel = within(c).getByRole('combobox', { name: 'Resposta: Nível de inglês' }) as HTMLSelectElement;
    expect([...sel.options].map(o => o.value)).toEqual(['', 'Básico', 'Avançado']);
    expect(within(c).getByText('Faltam responder 2 perguntas obrigatórias para aprovar.')).toBeTruthy();
  });

  it('pergunta sensível pendente nunca ganha campo', async () => {
    const est = { ...ESTRUTURA, perguntas: [{ id: 'q9', texto: 'Qual o seu CPF?', tipo: 'texto-curto', obrigatoria: true }] };
    montar({ candidaturas: [cand({ estrutura: est, pendentes: ['q9'], conteudo: { ...CONTEUDO, respostas: [] }, impressao: 'x' })] });
    const c = await cartao();
    expect(within(c).queryByRole('textbox', { name: /^Resposta: / })).toBeNull();
    expect(within(c).getByText('Pergunta sensível: o app não responde. Envie este formulário à mão.')).toBeTruthy();
    expect(within(c).queryByRole('button', { name: /^Salvar/ })).toBeNull();
  });

  it('texto da página só como texto (sem HTML)', async () => {
    const est = { ...ESTRUTURA, perguntas: [{ id: 'q1', texto: '<b>Qual</b> o seu <img src=x onerror=alert(1)> e-mail?', tipo: 'texto-curto', obrigatoria: false }] };
    montar({ candidaturas: [cand({ estrutura: est, pendentes: [], impressao: 'x' })] });
    const c = await cartao();
    expect(within(c).getByText('<b>Qual</b> o seu <img src=x onerror=alert(1)> e-mail?')).toBeTruthy();
    expect(c.querySelector('.form-ext b, .form-ext img')).toBeNull();
  });

  it('sem pendente e conferida: aprovar libera', async () => {
    const respostas = [...CONTEUDO.respostas];
    montar({ candidaturas: [cand({ pendentes: [] })] });
    const c = await cartao();
    await within(c).findByText('Conferida nesta tela');
    expect(within(c).getByText('Todas as obrigatórias têm resposta.')).toBeTruthy();
    expect(aprovar(c).disabled).toBe(false);
    expect(respostas).toHaveLength(1);
  });

  it('pré-preenchimento ignorado: a frase fixa', async () => {
    montar({ candidaturas: [cand({ preenchimentoIgnorado: true })] });
    expect(within(await cartao()).getByText('O link trazia respostas pré-preenchidas; o app ignorou.')).toBeTruthy();
  });

  it('Typeform: "envie à mão", sem "Ler formulário"; conferir manda o mesmo ler-formulario', async () => {
    const { gravacoes } = montar({ candidaturas: [cand({ url: 'https://exemplo.typeform.com/to/abc123', estrutura: null, pendentes: [], impressao: 'x' })], retorno: cand({ estado: 'bloqueada', estrutura: null, bloqueio: { motivo: 'outro', pendencia: 'O envio parou: confira a página', em: 'x' } }) });
    const c = await cartao();
    expect(within(c).getByText('Typeform: envie à mão.')).toBeTruthy();
    expect(within(c).queryByRole('button', { name: 'Ler formulário' })).toBeNull();
    await clicar(within(c).getByRole('button', { name: 'Conferir e enviar à mão' }));
    expect(gravacoes[0].caminho).toBe('/api/v1/candidaturas/cand-f1/ler-formulario');
  });

  it('bloqueio da leitura aparece com o motivo (várias etapas)', async () => {
    montar({ candidaturas: [cand({ estado: 'bloqueada', estrutura: null, pendentes: [], bloqueio: { motivo: 'varias-etapas', pendencia: 'Formulário em várias etapas: envie à mão', em: '2026-11-09T11:00:00-03:00' } })] });
    const c = await cartao();
    expect(within(c).getByText('Bloqueada · Várias etapas')).toBeTruthy();
    expect(within(c).getByText('Formulário em várias etapas: envie à mão')).toBeTruthy();
  });

  it.each([
    [409, {}, 'Não deu para ler agora: a candidatura não está para revisar, ou há um envio ou uma janela de login aberta. Tente de novo depois.'],
    [429, { plataforma: 'formulario', limite: 40, usadas: 40, renovaEm: '2026-11-10T00:00:00-03:00' }, 'O app já abriu 40 páginas de formulário hoje, o teto do dia. Renova amanhã às 00:00.'],
    [404, {}, 'Este app não traz a leitura de formulários.'],
    [403, {}, 'Só você pode ler o formulário, por esta tela.'],
  ])('ler com %i: frase fixa, nada do servidor', async (status, dados, frase) => {
    const { avisos } = montar({ candidaturas: [cand({ estrutura: null, pendentes: [], impressao: 'x' })], erro: new ErroApi('<b>servidor</b>', status, { erro: '<b>servidor</b>', ...dados }) });
    await clicar(within(await cartao()).getByRole('button', { name: 'Ler formulário' }));
    expect(avisos).toContainEqual({ texto: frase, erro: true });
  });
});
