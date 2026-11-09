// Lista do dia (jc-a2y.6.13) com a API interceptada: só entra o que confere nesta tela e cabe no limite de hoje; dá
// para tirar itens; um clique aprova a lista, cada item com a impressão calculada aqui; recusas por item em frase
// fixa. Só dados fictícios.
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, ErroApi, type App, type Pergunta } from '../app/contexto';
import { ListaDoDia } from './ListaDoDia';

afterEach(cleanup);

const IMP = (n: number) => 'sha256:v1:' + String(n).repeat(64).slice(0, 64);
const cand = (id: string, plataforma: string, estado: string, n: number, criadaEm = `2026-11-09T10:0${n}:00-03:00`) => ({
  id, plataforma, estado, impressao: IMP(n), criadaEm,
  conteudo: { curriculo: { arquivo: 'curriculo-exemplo.pdf' }, contato: { nome: 'Ana Exemplo' }, pretensao: null, respostas: [{}, {}], mensagem: null },
});
const NOMES: Record<string, string> = { a: 'Empresa A — Dev', b: 'Empresa B — Dev', c: 'Empresa C — Dev', d: 'Empresa D — Dev', e: 'Empresa E — Dev' };

function montar({ candidaturas, calculadas, limites = { linkedin: { limite: 30, usadas: 0 }, gupy: { limite: 30, usadas: 0 } } as any, resposta = 'ok' as string | null, retorno = null as any, erro = null as null | Error }: any) {
  const gravacoes: { caminho: string; corpo: any }[] = [];
  const perguntas: Pergunta[] = [];
  const avisos: { texto: string; erro?: boolean }[] = [];
  const onFeito = vi.fn();
  const app = {
    aviso: (texto: string, e?: boolean) => { avisos.push({ texto, erro: e }); },
    perguntar: async (p: Pergunta) => { perguntas.push(p); return resposta; },
    api: vi.fn(async (caminho: string, _m = 'GET', corpo?: any) => {
      gravacoes.push({ caminho, corpo });
      if (erro) throw erro;
      return retorno ?? { aprovadas: corpo.itens.map((i: any) => i.id), recusadas: [] };
    }),
  } as unknown as App;
  render(<Contexto.Provider value={app}>
    <ListaDoDia candidaturas={candidaturas} calculadas={calculadas} limites={limites} nome={c => NOMES[c.id]} plataforma={p => p === 'gupy' ? 'Gupy' : 'LinkedIn'} onFeito={onFeito} />
  </Contexto.Provider>);
  return { gravacoes, perguntas, avisos, onFeito };
}
const aprovarLista = () => screen.getByRole('button', { name: /^Aprovar a lista do dia/ }) as HTMLButtonElement;
const clicar = async (el: HTMLElement) => { await act(async () => { fireEvent.click(el); }); };

describe('Lista do dia', () => {
  it('só entram as preparadas que conferem nesta tela; as outras ficam fora, com aviso', () => {
    montar({
      candidaturas: [cand('a', 'linkedin', 'preparada', 1), cand('b', 'linkedin', 'preparada', 2), cand('c', 'linkedin', 'aprovada', 3)],
      calculadas: { a: IMP(1), b: IMP(9) }, // b não confere
    });
    const lista = screen.getByRole('region', { name: 'Lista do dia' });
    expect(within(lista).getAllByRole('checkbox')).toHaveLength(1);
    expect(within(lista).getByText('Empresa A — Dev')).toBeTruthy();
    expect(within(lista).getByText('LinkedIn · curriculo-exemplo.pdf · 2 respostas')).toBeTruthy();
    expect(within(lista).getByText(/1 pacote está fora da lista: ainda conferindo, ou a impressão não bate/)).toBeTruthy();
    expect(aprovarLista().textContent).toBe('Aprovar a lista do dia (1)');
  });

  it('respeita o que resta do limite de hoje (contando as já aprovadas): o resto fica para amanhã', () => {
    montar({
      candidaturas: [cand('a', 'linkedin', 'preparada', 1), cand('b', 'linkedin', 'preparada', 2), cand('c', 'linkedin', 'aprovada', 3), cand('d', 'gupy', 'preparada', 4)],
      calculadas: { a: IMP(1), b: IMP(2), d: IMP(4) },
      limites: { linkedin: { limite: 3, usadas: 1 }, gupy: { limite: 30, usadas: 0 } }, // linkedin: 3 − 1 − 1 aprovada = 1
    });
    const lista = screen.getByRole('region', { name: 'Lista do dia' });
    expect(within(lista).getAllByRole('checkbox').map(c => (c.closest('label') as HTMLElement).querySelector('strong')!.textContent)).toEqual(['Empresa A — Dev', 'Empresa D — Dev']);
    expect(within(lista).getByText('1 pacote fica para amanhã: o limite de hoje da plataforma já está cheio.')).toBeTruthy();
  });

  it('tirar um item: ele não vai; a confirmação lista o que vai e manda cada um com a impressão calculada aqui', async () => {
    const { gravacoes, perguntas, avisos, onFeito } = montar({
      candidaturas: [cand('a', 'linkedin', 'preparada', 1), cand('b', 'gupy', 'preparada', 2)],
      calculadas: { a: IMP(1), b: IMP(2) },
    });
    await clicar(screen.getByRole('checkbox', { name: /Empresa B/ }));
    expect(aprovarLista().textContent).toBe('Aprovar a lista do dia (1)');
    await clicar(aprovarLista());
    expect(perguntas[0].titulo).toBe('Aprovar 1 candidatura hoje?');
    expect(perguntas[0].itens).toEqual(['Empresa A — Dev']);
    expect(gravacoes).toEqual([{ caminho: '/api/v1/candidaturas/aprovar-lote', corpo: { itens: [{ id: 'a', impressao: IMP(1) }] } }]);
    expect(avisos).toContainEqual({ texto: '1 candidatura aprovada. O app envia hoje, uma vez.', erro: undefined });
    expect(onFeito).toHaveBeenCalled();
  });

  it('cancelar a confirmação não aprova; tirar todos trava o botão', async () => {
    const { gravacoes } = montar({ candidaturas: [cand('a', 'linkedin', 'preparada', 1)], calculadas: { a: IMP(1) }, resposta: null });
    await clicar(aprovarLista());
    expect(gravacoes).toEqual([]);
    await clicar(screen.getByRole('checkbox', { name: /Empresa A/ }));
    expect(aprovarLista().disabled).toBe(true);
  });

  it('recusa por item em frase fixa; 403 em frase fixa', async () => {
    const um = montar({
      candidaturas: [cand('a', 'linkedin', 'preparada', 1), cand('b', 'linkedin', 'preparada', 2)],
      calculadas: { a: IMP(1), b: IMP(2) },
      retorno: { aprovadas: ['a'], recusadas: [{ id: 'b', motivo: 'impressao-mudou' }] },
    });
    await clicar(aprovarLista());
    expect(um.avisos).toContainEqual({ texto: 'Empresa B — Dev ficou de fora: o pacote mudou; revise de novo.', erro: true });
    cleanup();
    const dois = montar({ candidaturas: [cand('a', 'linkedin', 'preparada', 1)], calculadas: { a: IMP(1) }, erro: new ErroApi('só pela tela', 403, { erro: 'x' }) });
    await clicar(aprovarLista());
    expect(dois.avisos).toContainEqual({ texto: 'Só você pode aprovar a lista, por esta tela.', erro: true });
  });

  it('motivos do contrato (nao-encontrada, vaga-fora) em frase fixa; itens vão na ordem da tela', async () => {
    const { gravacoes, avisos } = montar({
      candidaturas: [cand('b', 'linkedin', 'preparada', 2, '2026-11-09T10:05:00-03:00'), cand('a', 'linkedin', 'preparada', 1, '2026-11-09T10:01:00-03:00'), cand('c', 'gupy', 'preparada', 3, '2026-11-09T10:09:00-03:00')],
      calculadas: { a: IMP(1), b: IMP(2), c: IMP(3) },
      retorno: { aprovadas: ['a'], recusadas: [{ id: 'b', motivo: 'nao-encontrada' }, { id: 'c', motivo: 'vaga-fora' }] },
    });
    await clicar(aprovarLista());
    expect(gravacoes[0].corpo.itens.map((i: any) => i.id)).toEqual(['a', 'b', 'c']); // da mais antiga para a mais nova, como na tela
    expect(avisos).toContainEqual({ texto: 'Empresa B — Dev ficou de fora: não está mais na fila.', erro: true });
    expect(avisos).toContainEqual({ texto: 'Empresa C — Dev ficou de fora: a vaga saiu de Aplicar ou já foi enviada.', erro: true });
  });
  it('nada preparado e conferido: a seção não aparece', () => {
    montar({ candidaturas: [cand('a', 'linkedin', 'aprovada', 1)], calculadas: {} });
    expect(screen.queryByRole('region', { name: 'Lista do dia' })).toBeNull();
  });
});
