import { StrictMode } from 'react';
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import Applyer from '../App';
import { Contexto, type App } from '../app/contexto';
import { BuscaDoPerfil } from './BuscaDoPerfil';

afterEach(() => {
  cleanup(); history.replaceState(null, '', '/'); localStorage.clear();
  vi.unstubAllGlobals(); vi.restoreAllMocks();
});

const balanco = { encontradas: 3, novas: 3, grupos: { A: 1, B: 1, C: 1 }, puladas: 0, pendentes: 0, avisos: [] as string[] };
const json = (dados: unknown) => new Response(JSON.stringify(dados), { headers: { 'content-type': 'application/json' } });
const vaga = (grupo: string, empresa: string) => ({
  id: `busca-teste-${grupo}`, etapa: 'descoberta', titulo: 'Analista', empresa, aderencia: grupo, mercado: 'nacional',
  plataforma: 'externo', descobertaEm: '2026-11-10', publicadaEm: new Date().toISOString().slice(0,10),
  radarVersao: 1, radarClasse: grupo === 'A' ? 'forte' : grupo === 'B' ? 'conferir' : 'excluir',
  radarMotivo: `Evidência para ${empresa}.`, motivoClaude: `Parecer para ${empresa}.`, triagemConferir: ['Confirmar local da entrevista.'],
  ...(grupo === 'C' ? { triagemDecisao: 'pular', triagemMotivoPulo: 'Tecnologias fora da stack escolhida.' } : {}),
  fonteBusca: 'Fonte de teste', fonteURL: `https://fonte.example/vagas/${grupo}`, link: `https://empresa.example/oportunidades/${grupo}`,
});

describe('Busca pelo perfil', () => {
  it('concluir abre o Radar e uma busca explícita sobrevive ao StrictMode e à rota antiga da Triagem', async () => {
    vi.stubGlobal('matchMedia', () => ({ matches: true }));
    vi.spyOn(window, 'scrollTo').mockImplementation(() => {});
    const perfil = { cargo: 'Analista', stackPrincipal: ['Dados'], foco: 'ambos', ingles: 'Básico', filtros: { niveis: ['Pleno'], modelo: ['Remoto'] } };
    const estado = { onboardingConcluido: false, versao: '1', hoje: '2026-11-11', perfil, vagas: [] as ReturnType<typeof vaga>[],
      eventos: [], curriculos: [], plataformas: [], rodadas: [], pedidos: [], emails: [] };
    let concluirBusca!: () => void;
    const aberta = new Promise<void>(resolver => { concluirBusca = resolver; });
    const iniciadas: string[] = [];
    let revisao = 0;
    const fetch = vi.fn(async (url: string, init: RequestInit) => {
      switch (url) {
        case '/api/v1/estado': return json(estado);
        case '/api/v1/versao': return json({ versao: estado.versao });
        case '/api/v1/ia':
        case '/api/v1/ia/config': return json({ configurada: true, provedor: 'openai', chaves: { openai: '…1234' }, clis: [] });
        case '/api/v1/ia/testar': return json({ ok: true, verificacao: '3' });
        case '/api/v1/onboarding/rascunho':
          if (init.method === 'PATCH') revisao++;
          return json({ revisao, etapa: 1, dados: {}, editados: [], curriculo: null });
        case '/api/v1/onboarding/curriculo':
          expect(JSON.parse(init.body as string).curriculoBase64).toBe(btoa('%PDF-1.4\n%%EOF'));
          return json({ revisao: ++revisao, arquivoId: 'cv-qa', nomeExibicao: 'Teste.pdf', tamanho: 14 });
        case '/api/v1/onboarding':
          expect(JSON.parse(init.body as string)).toMatchObject({ arquivoId: 'cv-qa', revisao });
          estado.onboardingConcluido = true; estado.versao = '2';
          return json({ ok: true, onboardingConcluido: true });
        case '/api/v1/busca':
          expect(init.method).toBe('POST'); iniciadas.push(url);
          await aberta;
          estado.vagas = [vaga('C', 'Empresa C'), vaga('B', 'Empresa B'), vaga('A', 'Empresa A')];
          estado.versao = '3';
          return json(balanco);
        default: return json({});
      }
    });
    vi.stubGlobal('fetch', fetch);
    render(<StrictMode><Applyer /></StrictMode>);
    fireEvent.click(await screen.findByRole('button', { name: 'Usar conexão atual' }));
    fireEvent.change(screen.getByLabelText('Currículo em PDF'), { target: { files: [new File(['%PDF-1.4\n%%EOF'], 'Teste.pdf')] } });
    await screen.findByText('Teste.pdf');
    fireEvent.click(screen.getByRole('button', { name: 'Preencher manualmente' }));
    fireEvent.click(await screen.findByRole('button', { name: 'Concluir perfil' }));
    await screen.findByRole('heading', { name: 'Radar de vagas' });
    expect(iniciadas).toHaveLength(0);
    fireEvent.click(screen.getByRole('button', { name: 'Buscar vagas' }));
    await screen.findByRole('button', { name: 'Buscando…' });
    expect(location.hash).toBe('#/radar');
    expect(iniciadas).toHaveLength(1);
    act(() => { location.hash = '#/quadro'; });
    await screen.findByRole('heading', { name: 'Quadro de candidaturas' });
    act(() => { location.hash = '#/triagem?buscar=1'; });
    await waitFor(() => expect(location.hash).toBe('#/radar'));
    await screen.findByRole('button', { name: 'Buscando…' });
    expect(iniciadas).toHaveLength(1);
    await act(async () => { concluirBusca(); });
    fireEvent.click(await screen.findByText('Resultado da busca'));
    expect(screen.getByText('Grupo A · 1')).toBeTruthy();
    ['A', 'B', 'C'].forEach((g,i) => {
      fireEvent.click(screen.getByRole('button',{name:['Recomendadas','A conferir','Histórico'][i]}));
      const lista = screen.getByRole('list', { name: 'Vagas para revisão' });
      fireEvent.click(within(lista).getByRole('button', { name: new RegExp(`Empresa ${g}`) }));
      const cartao = within(screen.getByRole('article', { name: 'Analista' }));
      expect(cartao.getByText(`Empresa ${g}`)).toBeTruthy();
      expect(cartao.getByText(`Parecer para Empresa ${g}.`)).toBeTruthy();
      expect(cartao.getByText(`Evidência para Empresa ${g}.`)).toBeTruthy();
      expect(cartao.getByText('Confirmar local da entrevista.')).toBeTruthy();
      if (g === 'C') expect(cartao.getByText(/Tecnologias fora da stack escolhida/)).toBeTruthy();
      expect(cartao.getByRole('link', { name: 'Fonte: Fonte de teste' }).getAttribute('href')).toBe(`https://fonte.example/vagas/${g}`);
    });
    expect(iniciadas).toHaveLength(1);
  });

  it('permite tentar novamente após erro e retomar uma rodada com avaliações pendentes', async () => {
    const api = vi.fn().mockRejectedValueOnce(new Error('A fonte está indisponível.'))
      .mockResolvedValueOnce({ ...balanco, grupos: { A: 1, B: 0, C: 0 }, pendentes: 2, avisos: ['A IA não concluiu a rodada.'] })
      .mockResolvedValueOnce({ ...balanco, novas: 0 });
    const app = { api, recarregar: vi.fn().mockResolvedValue(undefined) } as unknown as App;
    render(<Contexto.Provider value={app}><BuscaDoPerfil /></Contexto.Provider>);
    fireEvent.click(screen.getByRole('button', { name: 'Buscar vagas com meu perfil' }));
    await screen.findByText('A fonte está indisponível.');
    fireEvent.click(screen.getByRole('button', { name: 'Tentar buscar novamente' }));
    await screen.findByText('A IA não concluiu a rodada.');
    expect(screen.getByText('2 pendentes')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Retomar avaliação das vagas' }));
    await screen.findByText('0 pendentes');
    await waitFor(() => expect(app.recarregar).toHaveBeenCalledTimes(2));
    expect(api).toHaveBeenCalledTimes(3);
  });
});
