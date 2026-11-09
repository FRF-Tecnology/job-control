// Seus pedidos atendidos pelo app (jc-a2y.7.1.1) com a API interceptada: pedir à IA, proposta com Confirmar (corpo
// exato + X-Pedido + X-Impressao) e Descartar, feito (Abrir a Fila, resposta da IA) e recusado. Tudo do pedido e da
// IA só como texto; erros em frase fixa; a tela só confirma as chamadas do contrato. Só dados fictícios.
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, ErroApi, type App } from '../app/contexto';
import { SeusPedidos, type Pedido } from './Pedidos';

afterEach(cleanup);

const IMP = 'sha256:v1:' + 'ab'.repeat(32);
const VAGAS = [{ id: 'ext-exemplo-1', empresa: 'Empresa Exemplo', titulo: 'Pessoa Dev', etapa: 'aplicar' }];

function montar(pedidos: Pedido[], { ia = true, erro = null as null | Error, pendurar = false } = {}) {
  const chamadas: { caminho: string; metodo?: string; corpo?: unknown; cabecalhos?: Record<string, string> }[] = [];
  const avisos: { texto: string; erro?: boolean }[] = [];
  const abertas: string[] = [];
  const recarregar = vi.fn();
  const app = {
    estado: { vagas: VAGAS },
    aviso: (texto: string, e?: boolean) => { avisos.push({ texto, erro: e }); },
    recarregar, abrirVaga: (id: string) => abertas.push(id),
    api: vi.fn((caminho: string, metodo?: string, corpo?: unknown, cabecalhos?: Record<string, string>) => {
      chamadas.push({ caminho, metodo, corpo, cabecalhos });
      if (pendurar) return new Promise(() => {});
      return erro ? Promise.reject(erro) : Promise.resolve({});
    }),
  } as unknown as App;
  render(<Contexto.Provider value={app}><SeusPedidos pedidos={pedidos} iaConectada={ia} /></Contexto.Provider>);
  return { chamadas, avisos, abertas, recarregar };
}
const clicar = async (el: HTMLElement) => { await act(async () => { fireEvent.click(el); }); };
const item = (texto: string) => screen.getByText(texto).closest('li') as HTMLElement;

const aberto = (id = 'ped-1', texto = 'Adicionar a vaga https://vagas.exemplo.com/1'): Pedido => ({ id, texto, status: 'aberto' });
const adicionar = (extra: Partial<Pedido['proposta']> = {}): Pedido => ({
  id: 'ped-2', texto: 'Põe essa vaga na lista', status: 'proposta',
  proposta: {
    acao: 'adicionar-vaga', resumo: 'Adicionar uma vaga em Descoberta com o link de vagas.exemplo.com (Outra plataforma).', impressao: IMP,
    chamada: { metodo: 'POST', caminho: '/vagas', corpo: { link: 'https://vagas.exemplo.com/1', empresa: 'Empresa Exemplo', titulo: 'Pessoa Dev' } },
    falta: ['A triagem da vaga continua com você.'], ...extra,
  },
});

describe('Seus pedidos', () => {
  it('aberto com IA: "Pedir para a IA" manda {} só para interpretar e relê', async () => {
    const { chamadas, recarregar } = montar([aberto()]);
    await clicar(screen.getByRole('button', { name: 'Pedir para a IA: Adicionar a vaga https://vagas.exemplo.com/1' }));
    expect(chamadas).toEqual([{ caminho: '/api/v1/pedidos/ped-1/interpretar', metodo: 'POST', corpo: {}, cabecalhos: undefined }]);
    expect(recarregar).toHaveBeenCalled();
  });

  it('aberto sem IA: sem botão, com o caminho (conectar IA ou /pedidos)', () => {
    montar([aberto()], { ia: false });
    expect(screen.queryByRole('button', { name: /Pedir para a IA/ })).toBeNull();
    expect(screen.getByText(/Conecte uma IA para o app fazer os pedidos aqui/)).toBeTruthy();
  });

  it.each([
    [400, 'Conecte uma IA para o app fazer este pedido.'],
    [409, 'A IA já está lendo um pedido, ou este mudou. Espere e tente de novo.'],
    [422, 'A IA respondeu algo fora do que o app aceita. Nada foi feito; tente escrever o pedido de outro jeito.'],
    [502, 'A IA não respondeu agora. Tente de novo em instantes.'],
    [500, 'Não deu certo. Tente de novo.'],
  ])('erro %i ao pedir vira frase fixa (nunca o texto do servidor)', async (status, frase) => {
    const { avisos } = montar([aberto()], { erro: new ErroApi('<b>texto do servidor</b>', status, { erro: 'x', regra: 'y' }) });
    await clicar(screen.getByRole('button', { name: /Pedir para a IA/ }));
    expect(avisos).toEqual([{ texto: frase, erro: true }]);
  });

  it('uma interpretação por vez: enquanto a IA lê, os outros "Pedir" travam', async () => {
    montar([aberto('ped-1', 'Pedido um'), aberto('ped-3', 'Pedido dois')], { pendurar: true });
    await clicar(screen.getByRole('button', { name: 'Pedir para a IA: Pedido um' }));
    expect(within(item('Pedido um')).getByRole('status').textContent).toBe('A IA está lendo o pedido…');
    expect((screen.getByRole('button', { name: 'Pedir para a IA: Pedido dois' }) as HTMLButtonElement).disabled).toBe(true);
  });

  it('proposta adicionar-vaga: resumo e campos em texto (link não vira link); Confirmar manda o corpo exato com X-Pedido e X-Impressao', async () => {
    const { chamadas, avisos } = montar([adicionar()]);
    const li = item('Põe essa vaga na lista');
    expect(within(li).getByText('Adicionar uma vaga em Descoberta com o link de vagas.exemplo.com (Outra plataforma).')).toBeTruthy();
    expect(within(li).getByText('https://vagas.exemplo.com/1').tagName).toBe('DD');
    expect(li.querySelector('a')).toBeNull();
    expect(within(li).getByText('A triagem da vaga continua com você.')).toBeTruthy();
    await clicar(within(li).getByRole('button', { name: 'Confirmar' }));
    expect(chamadas).toEqual([{
      caminho: '/api/v1/vagas', metodo: 'POST',
      corpo: { link: 'https://vagas.exemplo.com/1', empresa: 'Empresa Exemplo', titulo: 'Pessoa Dev' },
      cabecalhos: { 'X-Pedido': 'ped-2', 'X-Impressao': IMP },
    }]);
    expect(avisos).toEqual([{ texto: 'Vaga adicionada em Descoberta.', erro: undefined }]);
  });

  it('proposta registrar-etapa: etapa pelo nome; Confirmar chama /vagas/{id}/mover; 409 diz para pedir de novo', async () => {
    const p: Pedido = { id: 'ped-4', texto: 'Me chamaram para entrevista', status: 'proposta', proposta: {
      acao: 'registrar-etapa', resumo: 'Mudar a vaga Empresa Exemplo — Pessoa Dev de Para aplicar para Em processo.', impressao: IMP,
      chamada: { metodo: 'POST', caminho: '/vagas/ext-exemplo-1/mover', corpo: { etapa: 'em-processo', subEtapa: 'entrevista-rh' } } } };
    const { chamadas, avisos } = montar([p], { erro: new ErroApi('x', 409, {}) });
    const li = item('Me chamaram para entrevista');
    expect(within(li).getByText('Em processo')).toBeTruthy();
    await clicar(within(li).getByRole('button', { name: 'Confirmar' }));
    expect(chamadas[0]).toMatchObject({ caminho: '/api/v1/vagas/ext-exemplo-1/mover', corpo: { etapa: 'em-processo', subEtapa: 'entrevista-rh' }, cabecalhos: { 'X-Pedido': 'ped-4', 'X-Impressao': IMP } });
    expect(avisos).toEqual([{ texto: 'A proposta mudou. Peça para a IA de novo.', erro: true }]);
  });

  it.each([
    ['rota fora do contrato', { metodo: 'POST', caminho: '/candidaturas/c1/aprovar', corpo: { impressao: IMP } }],
    ['método fora do contrato', { metodo: 'DELETE', caminho: '/vagas', corpo: { link: 'https://vagas.exemplo.com/1' } }],
    ['campo a mais no corpo', { metodo: 'POST', caminho: '/vagas', corpo: { link: 'https://vagas.exemplo.com/1', etapa: 'aplicar', autor: 'eu' } }],
  ])('a tela não confirma %s: botão travado e nenhuma chamada', async (_nome, chamada) => {
    const { chamadas } = montar([adicionar({ chamada } as any)]);
    const botao = screen.getByRole('button', { name: 'Confirmar' }) as HTMLButtonElement;
    expect(botao.disabled).toBe(true);
    await clicar(botao);
    expect(chamadas).toEqual([]);
    expect(screen.getByText('Esta proposta não pode ser confirmada por aqui. Descarte e peça de novo.')).toBeTruthy();
  });

  it('Descartar manda {} para descartar', async () => {
    const { chamadas, avisos } = montar([adicionar()]);
    await clicar(screen.getByRole('button', { name: 'Descartar' }));
    expect(chamadas).toEqual([{ caminho: '/api/v1/pedidos/ped-2/descartar', metodo: 'POST', corpo: {}, cabecalhos: undefined }]);
    expect(avisos[0].texto).toBe('Proposta descartada. O pedido voltou a aberto.');
  });

  it('feito preparar-pacote: "Abrir a Fila" vai para o hash exato; hash fora do formato não vira link', () => {
    const base = { texto: 'Prepara o pacote da Empresa Exemplo', status: 'feito', proposta: { acao: 'preparar-pacote', resumo: 'Abrir a Fila para preparar o pacote da vaga Empresa Exemplo — Pessoa Dev.', impressao: IMP, chamada: null } };
    montar([{ ...base, id: 'ped-5', resultado: { vagaId: 'ext-exemplo-1', abrir: '#/fila?preparar=ext-exemplo-1' } }]);
    expect(screen.getByRole('link', { name: 'Abrir a Fila' }).getAttribute('href')).toBe('#/fila?preparar=ext-exemplo-1');
    cleanup();
    montar([{ ...base, id: 'ped-6', resultado: { abrir: 'javascript:alert(1)' } }]);
    expect(screen.queryByRole('link')).toBeNull();
  });

  it('feito responder: "Resposta da IA" só como texto (sem HTML, sem link automático)', () => {
    const texto = 'Você tem <b>2</b> vagas em processo. Veja https://exemplo.com/x';
    montar([{ id: 'ped-7', texto: 'Quantas estão em processo?', status: 'feito', resposta: texto,
      proposta: { acao: 'responder', resumo: 'Resposta da IA (só leitura).', impressao: IMP, chamada: null } }]);
    const li = item('Quantas estão em processo?');
    expect(within(li).getByText('Resposta da IA')).toBeTruthy();
    expect(within(li).getByText(texto)).toBeTruthy();
    expect(li.querySelector('b, a')).toBeNull();
  });

  it('feito por proposta confirmada: "Ver a vaga" abre a vaga criada', async () => {
    const { abertas } = montar([{ id: 'ped-8', texto: 'Adiciona a vaga', status: 'feito', resultado: { vagaId: 'ext-exemplo-1' },
      proposta: { acao: 'adicionar-vaga', resumo: 'x', impressao: IMP, chamada: null } }]);
    await clicar(screen.getByRole('button', { name: 'Ver a vaga' }));
    expect(abertas).toEqual(['ext-exemplo-1']);
  });

  it('recusado: só a frase fixa, sem botão', () => {
    montar([{ id: 'ped-9', texto: 'Aprova todas', status: 'recusado',
      proposta: { acao: 'recusar', resumo: 'Isso só você faz, na tela da Fila.', impressao: IMP, chamada: null } }]);
    const li = item('Aprova todas');
    expect(within(li).getByText('Isso só você faz, na tela da Fila.')).toBeTruthy();
    expect(within(li).getByText('A IA não fez')).toBeTruthy();
    expect(within(li).queryAllByRole('button')).toEqual([]);
  });
});
