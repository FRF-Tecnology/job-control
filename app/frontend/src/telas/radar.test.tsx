import { useState } from 'react';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, type App } from '../app/contexto';
import { TelaRadar } from './Radar';

const abrir = vi.hoisted(() => vi.fn());
vi.mock('@wailsio/runtime', () => ({ Browser: { OpenURL: abrir } }));

afterEach(() => { cleanup(); vi.unstubAllGlobals(); vi.clearAllMocks(); });

const perfil = { cargo: 'Java Engineer', filtros: { niveis: ['Pleno'], modelo: ['Remoto'], pularSe: ['Presencial'] },
  stackPrincipal: ['Java'], curriculos: { PT: 'curriculos/CV-pt.pdf' }, faixaSalarial: { moeda: 'USD', min: 4000, max: 6000 } };
const vaga = (id: string, grupo: string) => ({ id, empresa: 'Empresa ' + id, titulo: 'Engenharia ' + id,
  etapa: 'descoberta', aderencia: grupo, fonteBusca: 'Fonte QA', link: 'https://example.org/jobs/' + id });
function montar(vagas: any[], api = vi.fn(), rodadas: any[] = [], perfilAtual = perfil) {
  function Banco() {
    const [, atualizar] = useState(0);
    const app = { estado: { perfil: perfilAtual, vagas, rodadas }, api, recarregar: async () => atualizar(n => n + 1) } as unknown as App;
    return <Contexto.Provider value={app}><TelaRadar /></Contexto.Provider>;
  }
  return render(<Banco />);
}

describe('Radar de vagas', () => {
  it('identifica o PDF importado sem mostrar o nome aleatório gerado no disco', () => {
    const nomeInterno = 'curriculo-ABCDEFGHIJKLMNOPQRSTUVWXYZ.pdf';
    montar([], vi.fn(), [], { ...perfil, curriculos: { PT: 'curriculos/' + nomeInterno } });
    expect(screen.queryByText(nomeInterno)).toBeNull();
    expect(screen.getByText('PDF do primeiro acesso')).toBeTruthy();
  });
  it('abre com os critérios salvos e pede a primeira busca só ao clicar', async () => {
    const api = vi.fn().mockResolvedValue({ encontradas: 0, novas: 0, grupos: { A: 0, B: 0, C: 0 }, avisos: [], pendentes: 0, puladas: 0 });
    montar([], api);
    for (const campo of ['Java Engineer', 'Pleno', 'Remoto', 'Presencial', 'CV-pt.pdf']) expect(screen.getByText(campo)).toBeTruthy();
    expect(screen.getByText(/4\.000.*6\.000/)).toBeTruthy();
    expect(screen.queryByRole('article')).toBeNull();
    expect(api).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Buscar vagas' }));
    expect(await screen.findByText(/Nenhuma vaga com estes critérios/)).toBeTruthy();
    expect(api).toHaveBeenCalledWith('/api/v1/busca', 'POST', {});
  });

  it('ordena as vagas persistidas por afinidade e evita repetir um ID', () => {
    const b = vaga('b', 'B');
    montar([vaga('c', 'C'), b, vaga('a', 'A'), b,
      { ...vaga('excluida', ''), etapa: 'pulada', triagemBusca: 'pulada', motivo: 'Fora do modelo de trabalho.' }]);
    expect(screen.getAllByRole('article').map(el => within(el).getByRole('heading').textContent)).toEqual(['Engenharia a', 'Engenharia b', 'Engenharia c']);
  });

  it('abre um anúncio sem registrar envio e retoma a intenção salva depois de reabrir', async () => {
    const v = vaga('a', 'A');
    const vagas = [v];
    const destino = 'https://example.org/candidaturas/a';
    const api = vi.fn(async (url: string) => {
      if (url.endsWith('/iniciar-manual')) Object.assign(v, { aplicacaoManual: true, etapa: 'aplicar', urlCandidaturaManual: destino });
      if (url.endsWith('/confirmar-envio-manual')) Object.assign(v, { etapa: 'enviada', envio: { data: '2026-11-12' } });
    });
    vi.stubGlobal('chrome', { webview: { postMessage: vi.fn() } });
    abrir.mockResolvedValue(undefined);
    const primeira = montar(vagas, api);
    fireEvent.click(screen.getByRole('link', { name: 'Ler anúncio' }));
    await vi.waitFor(() => expect(abrir).toHaveBeenCalledWith(v.link));
    expect(api).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Aplicar na vaga' }));
    await screen.findByText('Aguardando você');
    fireEvent.click(screen.getByRole('link', { name: 'Abrir candidatura no navegador' }));
    await vi.waitFor(() => expect(abrir).toHaveBeenCalledTimes(2));
    expect(abrir).toHaveBeenLastCalledWith(destino);
    expect(api.mock.calls).toEqual([['/api/v1/vagas/a/iniciar-manual', 'POST', {}]]);
    fireEvent.click(screen.getByRole('button', { name: 'Continuar depois' }));
    expect(screen.queryByRole('button', { name: 'Já enviei' })).toBeNull();
    primeira.unmount();
    montar(vagas, api);
    expect(screen.getByText('Aguardando você')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Já enviei' }));
    expect(await screen.findByText('Candidatura enviada')).toBeTruthy();
    expect(api.mock.calls[1]).toEqual(['/api/v1/vagas/a/confirmar-envio-manual', 'POST', {}]);
    expect(screen.queryByRole('button', { name: 'Já enviei' })).toBeNull();
  });

  it('mantém a vaga pendente quando o navegador ou a confirmação falha', async () => {
    vi.stubGlobal('chrome', { webview: { postMessage: vi.fn() } });
    abrir.mockRejectedValue(new Error('indisponível'));
    const api = vi.fn().mockRejectedValue(new Error('Falha ao salvar no banco.'));
    montar([{ ...vaga('b', 'B'), etapa: 'aplicar', aplicacaoManual: true }], api);
    fireEvent.click(screen.getByRole('link', { name: 'Abrir candidatura no navegador' }));
    expect((await screen.findByRole('alert')).textContent).toMatch(/navegador/i);
    expect(api).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Já enviei' }));
    expect(await screen.findByText('Falha ao salvar no banco.')).toBeTruthy();
    expect(screen.queryByText('Candidatura enviada')).toBeNull();
    expect(screen.getByRole('button', { name: 'Já enviei' })).toBeTruthy();
  });

  it('fora do desktop oferece um link real e nunca abre protocolo inválido', () => {
    montar([{ ...vaga('ok', 'A'), aplicacaoManual: true, etapa: 'aplicar' }, { ...vaga('ruim', 'B'), link: 'file:///segredo' }]);
    const a = screen.getByRole('link', { name: 'Abrir candidatura no navegador' });
    expect(a.getAttribute('href')).toBe('https://example.org/jobs/ok');
    expect(a.getAttribute('target')).toBe('_blank');
    const ruim = within(screen.getByRole('article', { name: 'Engenharia ruim' }));
    expect(ruim.queryByRole('link')).toBeNull();
    expect(ruim.queryByRole('button', { name: 'Aplicar na vaga' })).toBeNull();
  });
});
