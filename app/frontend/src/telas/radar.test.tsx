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
function montar(vagas: any[], api = vi.fn(), estadoExtra = {}) {
  function Banco() {
    const [, atualizar] = useState(0);
    const app = { estado: { perfil, vagas, rodadas: [], ...estadoExtra }, api,
      mover: async (id: string, etapa: string, extras?: object) => {
        Object.assign(vagas.find(v => v.id === id), extras, { etapa });
        atualizar(n => n + 1);
        return true;
      }, recarregar: async () => atualizar(n => n + 1) } as unknown as App;
    return <Contexto.Provider value={app}><TelaRadar /></Contexto.Provider>;
  }
  return render(<Banco />);
}

describe('Radar de vagas', () => {
  it('exibe vagas da base anterior mesmo sem fonte, nota ou registro de triagem', () => {
    montar([{ id: 'legado', empresa: 'Oficina', titulo: 'Desenvolvimento', etapa: 'descoberta' },
      { ...vaga('privada', 'A'), perfilBuscaId: 'outro' }]);
    expect(screen.getByRole('button', { name: /Oficina Desenvolvimento/ })).toBeTruthy();
    expect(screen.getByRole('article', { name: 'Desenvolvimento' })).toBeTruthy();
    expect(screen.queryByText('Nenhuma vaga encontrada')).toBeNull();
    expect(screen.queryByText('Empresa privada')).toBeNull();
  });
  it('mostra valores, periodicidade e requisitos do anúncio salvo e usa a marca local quando conhecida', () => {
    montar([{ ...vaga('hf', 'A'), empresa: 'Hugging Face', periodoSalario: 'hourly', moeda: 'USD',
      salarioMin: 42, salarioMax: 48, stack: ['Python'], descricao: 'Desenvolver serviços de inferência.' },
      { ...vaga('outra', 'B'), empresa: 'Oficina' }]);
    const detalhe = within(screen.getByRole('article', { name: 'Engenharia hf' }));
    expect(detalhe.getByText('Salário por hora')).toBeTruthy();
    expect(detalhe.getByText('US$ 42 – US$ 48')).toBeTruthy();
    expect(detalhe.getByText('Python')).toBeTruthy();
    expect(detalhe.getByText('Desenvolver serviços de inferência.')).toBeTruthy();
    const linhaHF = screen.getByRole('button', { name: /Hugging Face Engenharia hf/ });
    const logo = linhaHF.querySelector('img')!;
    expect(logo.getAttribute('src')).toContain('huggingface');
    fireEvent.error(logo);
    expect(within(linhaHF).getByText('HF')).toBeTruthy();
    expect(screen.getByRole('button', { name: /Oficina Engenharia outra/ }).querySelector('img')).toBeNull();
  });
  it('abre com os critérios salvos e pede a primeira busca só ao clicar', async () => {
    const api = vi.fn().mockResolvedValue({ encontradas: 0, novas: 0, grupos: { A: 0, B: 0, C: 0 }, avisos: [], pendentes: 0, puladas: 0 });
    montar([], api);
    expect(screen.getByText('Java Engineer')).toBeTruthy();
    expect(screen.getByText(/Remoto/)).toBeTruthy();
    expect(screen.getByRole('link', { name: 'Ajustar preferências' }).getAttribute('href')).toBe('#/perfil');
    expect(screen.getByRole('heading', { name: 'Nenhuma vaga encontrada' })).toBeTruthy();
    expect(screen.queryByRole('article')).toBeNull();
    expect(api).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Buscar vagas' }));
    expect(await screen.findByText('Resultado da busca')).toBeTruthy();
    expect(api).toHaveBeenCalledWith('/api/v1/busca', 'POST', { perfilBuscaId: 'original' });
  });

  it('oculta no perfil ativo um anúncio já adicionado à lista de outro perfil', () => {
    montar([{ ...vaga('legado', 'B'), etapa: 'aplicar' }, { ...vaga('copia', 'A'), anuncioId: 'legado', perfilBuscaId: 'java' },
      { ...vaga('nova', 'C'), perfilBuscaId: 'java' }], vi.fn(), { perfilBuscaAtivo: 'java' });
    expect(screen.queryByText('Engenharia copia')).toBeNull();
    expect(screen.queryByText('Engenharia legado')).toBeNull();
    expect(screen.getByRole('article', { name: 'Engenharia nova' })).toBeTruthy();
  });

  it('ordena sem duplicatas e abre um detalhe por vez ao selecionar ou filtrar', () => {
    const b = vaga('b', 'B');
    montar([vaga('c', 'C'), b, vaga('a', 'A'), b,
      { ...vaga('excluida', ''), etapa: 'pulada', triagemBusca: 'pulada', motivo: 'Fora do modelo de trabalho.' }]);
    const lista = screen.getByRole('list', { name: 'Vagas para revisão' });
    const linhas = within(lista).getAllByRole('button');
    expect(linhas.map(el => within(el).getByText(/Engenharia/).textContent)).toEqual(['Engenharia a', 'Engenharia b', 'Engenharia c']);
    expect(screen.getAllByRole('article')).toHaveLength(1);
    expect(screen.getByRole('article', { name: 'Engenharia a' })).toBeTruthy();
    fireEvent.click(linhas[1]);
    expect(screen.getByRole('article', { name: 'Engenharia b' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Nota C' }));
    expect(within(lista).getAllByRole('button')).toHaveLength(1);
    expect(screen.getByRole('article', { name: 'Engenharia c' })).toBeTruthy();
  });

  it('atualiza o detalhe por teclado, adiciona à lista, pula e desfaz na última vaga', async () => {
    montar([vaga('a', 'A'), vaga('b', 'B')]);
    const primeira = screen.getByRole('button', { name: /Empresa a Engenharia a/ });
    fireEvent.keyDown(primeira, { key: 'ArrowDown' });
    expect(screen.getByRole('article', { name: 'Engenharia b' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Adicionar à lista' }));
    await screen.findByRole('article', { name: 'Engenharia a' });
    fireEvent.click(screen.getByRole('button', { name: 'Pular' }));
    await screen.findByRole('heading', { name: 'Você revisou todas as vagas' });
    expect(screen.getByRole('link', { name: 'Ver lista' }).getAttribute('href')).toBe('#/lista');
    fireEvent.click(await screen.findByRole('button', { name: 'Desfazer' }));
    await screen.findByRole('article', { name: 'Engenharia a' });
    fireEvent.click(screen.getByRole('button', { name: 'Nota C' }));
    expect(screen.getByRole('heading', { name: 'Nenhuma vaga com nota C' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Ver todas as vagas' }));
    expect(screen.getByRole('article', { name: 'Engenharia a' })).toBeTruthy();
  });

  it('distingue busca em andamento e interrompida e mantém acesso a vagas anteriores', async () => {
    const api = vi.fn().mockRejectedValueOnce(new Error('Fonte indisponível')).mockResolvedValueOnce({
      encontradas: 0, novas: 0, grupos: { A: 0, B: 0, C: 0 }, avisos: [], pendentes: 0, puladas: 0,
    });
    montar([vaga('salva', 'B')], api);
    fireEvent.click(screen.getByRole('button', { name: 'Buscar vagas' }));
    expect(screen.getByText('Buscando vagas para o seu perfil…')).toBeTruthy();
    await screen.findByRole('heading', { name: 'A busca foi interrompida' });
    expect(screen.getByText('Fonte indisponível')).toBeTruthy();
    expect(screen.getByRole('article', { name: 'Engenharia salva' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Tentar novamente' }));
    await screen.findByText('Resultado da busca');
    expect(screen.queryByText('A busca foi interrompida')).toBeNull();
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
    fireEvent.click(screen.getByRole('button', { name: /Empresa ruim Engenharia ruim/ }));
    const ruim = within(screen.getByRole('article', { name: 'Engenharia ruim' }));
    expect(ruim.queryByRole('link')).toBeNull();
    expect(ruim.queryByRole('button', { name: 'Aplicar na vaga' })).toBeNull();
  });
});
