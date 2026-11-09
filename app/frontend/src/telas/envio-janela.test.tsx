// Ver o envio e entrar nas plataformas (jc-a2y.6.7, contrato 0.4.13) com a API interceptada: as duas ações só pela
// tela, com corpo exato; enquanto a janela de login está aberta, o envio espera; erros em frases fixas; o cartão da
// Gupy/InHire abre pela vaga. Só dados fictícios.
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, ErroApi, type App } from '../app/contexto';
import { PainelEnvio, type EstadoEnvio } from './PainelEnvio';
import { TelaFila } from './Fila';

afterEach(cleanup);

const BASE: EstadoEnvio = { ligado: true, pausado: true, enviandoAgora: null, desfechos: [], janela: 'oculta', login: false };

function montarPainel({ estado = BASE, erro = null as null | Error } = {}) {
  let atual = estado;
  const gravacoes: { caminho: string; corpo: any }[] = [];
  const avisos: { texto: string; erro?: boolean }[] = [];
  const app = {
    estado: { vagas: [] }, aviso: (texto: string, e?: boolean) => { avisos.push({ texto, erro: e }); }, perguntar: async () => 'ok',
    api: vi.fn(async (caminho: string, metodo = 'GET', corpo?: any) => {
      if (metodo === 'GET') return atual;
      gravacoes.push({ caminho, corpo });
      if (erro) throw erro;
      if (caminho.endsWith('/envio/janela')) return (atual = { ...atual, janela: corpo.visivel ? 'visivel' : 'oculta' });
      return (atual = { ...atual, login: true });
    }),
  } as unknown as App;
  render(<Contexto.Provider value={app}><PainelEnvio nome={() => 'Empresa Exemplo — Pessoa Dev React'} motivo={() => 'Outro'} /></Contexto.Provider>);
  return { gravacoes, avisos };
}
const clicar = async (nome: string) => { await act(async () => { fireEvent.click(screen.getByRole('button', { name: nome })); }); };

describe('Ver o envio', () => {
  it('liga a janela com {visivel:true} e diz que vale a partir da próxima candidatura', async () => {
    const { gravacoes, avisos } = montarPainel();
    expect(await screen.findByText(/O envio roda sem janela, para não atrapalhar o uso do PC\./)).toBeTruthy();
    await clicar('Ver o envio');
    expect(gravacoes).toEqual([{ caminho: '/api/v1/envio/janela', corpo: { visivel: true } }]);
    expect(screen.getByText(/a partir da próxima candidatura\. A que está sendo enviada agora termina sem janela\./)).toBeTruthy();
    expect(avisos).toContainEqual({ texto: 'A janela do envio aparece a partir da próxima candidatura.', erro: undefined });
    await clicar('Esconder o envio');
    expect(gravacoes[1]).toEqual({ caminho: '/api/v1/envio/janela', corpo: { visivel: false } });
    expect(screen.getByRole('button', { name: 'Ver o envio' })).toBeTruthy();
  });

  it('se a janela não ficou visível na resposta, não promete que vai aparecer', async () => {
    let atual: EstadoEnvio = BASE;
    const avisos: { texto: string; erro?: boolean }[] = [];
    const app = { estado: { vagas: [] }, aviso: (texto: string, e?: boolean) => { avisos.push({ texto, erro: e }); }, perguntar: async () => 'ok',
      api: vi.fn(async (_c: string, metodo = 'GET') => metodo === 'GET' ? atual : (atual = { ...atual, janela: 'oculta' })) } as unknown as App;
    render(<Contexto.Provider value={app}><PainelEnvio nome={() => 'x'} motivo={() => 'x'} /></Contexto.Provider>);
    await screen.findByRole('button', { name: 'Ver o envio' });
    await clicar('Ver o envio');
    expect(avisos).toEqual([{ texto: 'Não deu para mostrar a janela do envio agora. Tente de novo.', erro: true }]);
    expect(screen.getByRole('button', { name: 'Ver o envio' })).toBeTruthy();
  });

  it('403 (pedido fora da tela) em frase fixa', async () => {
    const { avisos } = montarPainel({ erro: new ErroApi('só a pessoa', 403, { erro: 'só a pessoa' }) });
    await screen.findByRole('button', { name: 'Ver o envio' });
    await clicar('Ver o envio');
    expect(avisos).toContainEqual({ texto: 'Só você pode abrir a janela do envio, por esta tela.', erro: true });
  });
});

describe('Entrar nas plataformas', () => {
  it('LinkedIn: manda {plataforma:"linkedin"}; com a janela aberta, mostra que o envio espera e trava o botão', async () => {
    const { gravacoes, avisos } = montarPainel();
    await screen.findByRole('button', { name: 'Entrar no LinkedIn' });
    await clicar('Entrar no LinkedIn');
    expect(gravacoes).toEqual([{ caminho: '/api/v1/navegador/entrar', corpo: { plataforma: 'linkedin' } }]);
    expect(screen.getByText('Janela de login aberta. O envio espera até você fechá-la; depois, volta sozinho.')).toBeTruthy();
    expect((screen.getByRole('button', { name: 'Entrar no LinkedIn' }) as HTMLButtonElement).disabled).toBe(true);
    expect(avisos).toContainEqual({ texto: 'Janela de login do LinkedIn aberta. O envio espera até você fechá-la.', erro: undefined });
  });

  it('409 com envio em andamento e 409 com login já aberto: frases fixas, sem texto do servidor', async () => {
    const um = montarPainel({ erro: new ErroApi('x', 409, { erro: 'texto do servidor', estado: 'enviando' }) });
    await screen.findByRole('button', { name: 'Entrar no LinkedIn' });
    await clicar('Entrar no LinkedIn');
    expect(um.avisos).toContainEqual({ texto: 'Uma candidatura está sendo enviada agora. Espere terminar e tente de novo.', erro: true });
    cleanup();
    const dois = montarPainel({ erro: new ErroApi('x', 409, { erro: 'texto do servidor' }) });
    await screen.findByRole('button', { name: 'Entrar no LinkedIn' });
    await clicar('Entrar no LinkedIn');
    expect(dois.avisos).toContainEqual({ texto: 'Já há uma janela de login aberta. Feche-a antes de abrir outra.', erro: true });
  });

  it('build sem envio (ligado:false): nenhum dos botões', async () => {
    montarPainel({ estado: { ...BASE, ligado: false } });
    await screen.findByText(/Envio automático desligado/);
    expect(screen.queryByRole('button', { name: 'Ver o envio' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Entrar no LinkedIn' })).toBeNull();
  });
});

describe('Cartão da Gupy: entrar pela vaga', () => {
  const IMP = 'sha256:v1:' + '0'.repeat(64);
  const cand = (id: string, plataforma: string, estado: string) => ({
    id, vagaId: 'ext-exemplo-1', plataforma, url: 'https://empresa.gupy.io/jobs/1', estado, impressao: IMP, aprovacao: null, bloqueio: null, resultado: null, versao: 1,
    conteudo: { curriculo: { arquivo: 'curriculo-exemplo.pdf' }, idioma: 'pt', respostas: [], mensagem: null }, criadaEm: '2026-11-09T10:00:00-03:00', atualizadaEm: '2026-11-09T10:00:00-03:00',
  });
  function montarFila(envio: EstadoEnvio) {
    const gravacoes: { caminho: string; corpo: any }[] = [];
    const app = {
      estado: { vagas: [{ id: 'ext-exemplo-1', empresa: 'Empresa Exemplo', titulo: 'Pessoa Dev React', etapa: 'aplicar' }], perfil: {}, curriculos: [] },
      hoje: '2026-11-09', aviso: () => {}, perguntar: async () => 'ok', recarregar: async () => {}, limpar: () => {},
      api: vi.fn(async (caminho: string, metodo = 'GET', corpo?: any) => {
        if (metodo !== 'GET') { gravacoes.push({ caminho, corpo }); return { ...envio, login: true }; }
        if (caminho.endsWith('/envio')) return envio;
        if (caminho.endsWith('/limites')) return { linkedin: { limite: 30, usadas: 0, renovaEm: '2026-11-10T00:00:00-03:00' }, gupy: { limite: 30, usadas: 0, renovaEm: '2026-11-10T00:00:00-03:00' }, inhire: { limite: 30, usadas: 0, renovaEm: '2026-11-10T00:00:00-03:00' } };
        return [cand('cand-gupy', 'gupy', 'bloqueada'), cand('cand-link', 'linkedin', 'bloqueada')];
      }),
    } as unknown as App;
    render(<Contexto.Provider value={app}><TelaFila /></Contexto.Provider>);
    return { gravacoes };
  }

  it('com o envio no app, a Gupy ganha "Entrar na Gupy por esta vaga", que manda {candidatura}; o LinkedIn não', async () => {
    const { gravacoes } = montarFila(BASE);
    const gupy = await screen.findByTestId('cand-gupy');
    const botao = await within(gupy).findByRole('button', { name: 'Entrar na Gupy por esta vaga' });
    expect(within(await screen.findByTestId('cand-link')).queryByRole('button', { name: /por esta vaga/ })).toBeNull();
    await act(async () => { fireEvent.click(botao); });
    expect(gravacoes).toEqual([{ caminho: '/api/v1/navegador/entrar', corpo: { candidatura: 'cand-gupy' } }]);
  });

  it('sem o envio no app (ligado:false), o cartão não oferece entrar', async () => {
    montarFila({ ...BASE, ligado: false });
    const gupy = await screen.findByTestId('cand-gupy');
    await screen.findByText(/Envio automático desligado/);
    expect(within(gupy).queryByRole('button', { name: /por esta vaga/ })).toBeNull();
  });
});
