// "Aprovar todas do grupo A" na Triagem (jc-a2y.31): a pessoa vê a lista e a contagem, confirma,
// e só essas vagas vão para aplicar. Feito pela tela, o autor é "eu". Só dados fictícios.
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, type App, type Pergunta } from '../app/contexto';
import { TelaTriagem } from './Triagem';

afterEach(cleanup);

const vaga = (id: string, aderencia: string, extra: Record<string, unknown> = {}) => ({
  id, aderencia, etapa: 'descoberta', empresa: `Empresa ${id}`, titulo: `Pessoa Dev ${id}`, plataforma: 'linkedin', descobertaEm: '2026-11-08', ...extra,
});

function montar({ vagas, depois = vagas, resposta = 'ok' as string | null, falhar = [] as string[] }: { vagas: any[]; depois?: any[]; resposta?: string | null; falhar?: string[] }) {
  const perguntas: Pergunta[] = [];
  const movidas: { id: string; corpo: any }[] = [];
  const avisos: { texto: string; erro?: boolean }[] = [];
  const api = vi.fn(async (caminho: string, metodo = 'GET', corpo?: any) => {
    if (caminho === '/api/v1/estado') return { vagas: depois };
    const m = caminho.match(/^\/api\/v1\/vagas\/([^/]+)\/mover$/);
    if (m && metodo === 'POST') {
      const id = decodeURIComponent(m[1]);
      if (falhar.includes(id)) throw new Error('vaga não encontrada');
      movidas.push({ id, corpo });
      return {};
    }
    throw new Error(`chamada inesperada ${metodo} ${caminho}`);
  });
  const app: App = {
    estado: { vagas, eventos: [], plataformas: [] }, hoje: '2026-11-09', foco: 'ambos', rota: 'triagem', vaga: null,
    mudarFoco: async () => {}, api, agir: async () => true, mover: vi.fn(async () => true),
    abrirVaga: () => {}, navegar: () => {}, aviso: (texto, erro) => { avisos.push({ texto, erro }); },
    perguntar: async p => { perguntas.push(p); return resposta; },
    recarregar: vi.fn(async () => {}), limpar: () => {},
  };
  render(<Contexto.Provider value={app}><TelaTriagem /></Contexto.Provider>);
  return { app, api, perguntas, movidas, avisos };
}

const clicarAprovarTodas = async () => {
  await act(async () => { fireEvent.click(screen.getByRole('button', { name: /^Aprovar todas do grupo A/ })); });
};

const VAGAS = [vaga('a1', 'A'), vaga('b1', 'B'), vaga('a2', 'A'), vaga('c1', 'C')];

describe('Aprovar todas do grupo A', () => {
  it('mostra a contagem no botão', () => {
    montar({ vagas: VAGAS });
    expect(screen.getByRole('button', { name: 'Aprovar todas do grupo A (2)' })).toBeTruthy();
  });

  it('confirmação lista as vagas A e a contagem; só elas vão para aplicar', async () => {
    const { perguntas, movidas, app, avisos } = montar({ vagas: VAGAS });
    await clicarAprovarTodas();
    expect(perguntas).toHaveLength(1);
    expect(perguntas[0].titulo).toContain('2 vagas');
    expect(perguntas[0].itens).toEqual(['Empresa a1 — Pessoa Dev a1', 'Empresa a2 — Pessoa Dev a2']);
    expect(movidas.map(m => m.id).sort()).toEqual(['a1', 'a2']);
    expect(movidas.every(m => m.corpo.etapa === 'aplicar')).toBe(true);
    expect(movidas.some(m => 'autor' in m.corpo)).toBe(false); // a tela não manda autor: a API grava "eu"
    expect(app.recarregar).toHaveBeenCalledTimes(1);
    expect(avisos).toEqual([{ texto: '2 vagas foram para a Fila.', erro: undefined }]);
  });

  it('cancelar não muda nada', async () => {
    const { movidas, api, avisos } = montar({ vagas: VAGAS, resposta: null });
    await clicarAprovarTodas();
    expect(movidas).toEqual([]);
    expect(api).not.toHaveBeenCalled();
    expect(avisos).toEqual([]);
  });

  it('vaga que mudou depois de abrir a confirmação não volta para aplicar', async () => {
    const depois = [vaga('a1', 'A', { etapa: 'pulada' }), vaga('b1', 'B'), vaga('a2', 'A'), vaga('c1', 'C')];
    const { movidas, avisos } = montar({ vagas: VAGAS, depois });
    await clicarAprovarTodas();
    expect(movidas.map(m => m.id)).toEqual(['a2']);
    expect(avisos[0].texto).toBe('1 vaga foi para a Fila. 1 mudou antes e ficou como estava.');
  });

  it('vaga com empresa ou título trocados depois da confirmação não é aprovada (a pessoa aprovou o que viu)', async () => {
    const depois = [vaga('a1', 'A', { titulo: 'Outra vaga' }), vaga('b1', 'B'), vaga('a2', 'A', { empresa: 'Outra empresa' }), vaga('c1', 'C')];
    const { movidas, avisos } = montar({ vagas: [...VAGAS, vaga('a3', 'A')], depois: [...depois, vaga('a3', 'A')] });
    await clicarAprovarTodas();
    expect(movidas.map(m => m.id)).toEqual(['a3']);
    expect(avisos[0].texto).toBe('1 vaga foi para a Fila. 2 mudaram antes e ficaram como estavam.');
  });

  it('falha numa vaga não para as outras e é avisada', async () => {
    const { movidas, avisos } = montar({ vagas: VAGAS, falhar: ['a1'] });
    await clicarAprovarTodas();
    expect(movidas.map(m => m.id)).toEqual(['a2']);
    expect(avisos).toContainEqual({ texto: 'Não deu para aprovar Empresa a1: vaga não encontrada', erro: true });
  });

  it('com uma vaga A só, o botão não aparece (basta a tecla A)', () => {
    montar({ vagas: [vaga('a1', 'A'), vaga('b1', 'B')] });
    expect(screen.queryByRole('button', { name: /^Aprovar todas do grupo A/ })).toBeNull();
  });
});
