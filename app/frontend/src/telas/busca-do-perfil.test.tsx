import { StrictMode } from 'react';
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
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
  plataforma: 'externo', descobertaEm: '2026-11-10', motivoClaude: `Parecer para ${empresa}.`, triagemConferir: ['Confirmar local da entrevista.'],
  ...(grupo === 'C' ? { triagemDecisao: 'pular', triagemMotivoPulo: 'Tecnologias fora da stack escolhida.' } : {}),
  fonteBusca: 'Fonte de teste', fonteURL: `https://fonte.example/vagas/${grupo}`, link: `https://empresa.example/oportunidades/${grupo}`,
});

describe('Busca pelo perfil', () => {
  it('Salvar e buscar abre a Triagem e a mesma rodada sobrevive ao StrictMode e à troca de tela', async () => {
    vi.stubGlobal('matchMedia', () => ({ matches: true }));
    vi.spyOn(window, 'scrollTo').mockImplementation(() => {});
    const perfil = { cargo: 'Analista', stackPrincipal: ['Dados'], foco: 'ambos', ingles: 'Básico', filtros: { niveis: ['Pleno'], modelo: ['Remoto'] } };
    const estado = { onboardingConcluido: false, versao: '1', hoje: '2026-11-11', perfil, vagas: [] as ReturnType<typeof vaga>[],
      eventos: [], curriculos: [], plataformas: [], rodadas: [], pedidos: [], emails: [] };
    let concluirBusca!: () => void;
    const aberta = new Promise<void>(resolver => { concluirBusca = resolver; });
    const iniciadas: string[] = [];
    const fetch = vi.fn(async (url: string, init: RequestInit) => {
      switch (url) {
        case '/api/v1/estado': return json(estado);
        case '/api/v1/versao': return json({ versao: estado.versao });
        case '/api/v1/ia':
        case '/api/v1/ia/config': return json({ configurada: true, provedor: 'openai', chaves: { openai: '…1234' }, clis: [] });
        case '/api/v1/onboarding':
          expect(JSON.parse(init.body as string).curriculoBase64).toBe(btoa('%PDF-1.4\n%%EOF'));
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
    await screen.findByRole('heading', { name: 'Anexe seu currículo.' });
    fireEvent.change(screen.getByLabelText('Currículo em PDF'), { target: { files: [new File(['%PDF-1.4\n%%EOF'], 'Teste.pdf')] } });
    await screen.findByRole('heading', { name: 'Teste.pdf' });
    fireEvent.click(screen.getByRole('button', { name: 'Preencher manualmente' }));
    for (let i = 0; i < 2; i++) fireEvent.click(screen.getByRole('button', { name: /Continuar/ }));
    fireEvent.click(await screen.findByRole('button', { name: /Salvar e buscar vagas/ }));
    await screen.findByRole('button', { name: /Buscando e avaliando/ });
    expect(location.hash).toBe('#/triagem');
    expect(iniciadas).toHaveLength(1);
    act(() => { location.hash = '#/quadro'; });
    await screen.findByRole('heading', { name: 'Quadro' });
    act(() => { location.hash = '#/triagem?buscar=1'; });
    await screen.findByRole('button', { name: /Buscando e avaliando/ });
    expect(iniciadas).toHaveLength(1);
    await act(async () => { concluirBusca(); });
    await screen.findByText('Resultado da busca');
    expect(screen.getByText('Grupo A · 1')).toBeTruthy();
    for (const g of ['A', 'B', 'C']) {
      expect(screen.getByText(`Empresa ${g}`)).toBeTruthy();
      expect(screen.getByText(`Parecer para Empresa ${g}.`)).toBeTruthy();
      expect(screen.getByText('Confirmar local da entrevista.')).toBeTruthy();
      if (g === 'C') expect(screen.getByText('Tecnologias fora da stack escolhida.')).toBeTruthy();
      expect(screen.getByRole('link', { name: 'Fonte: Fonte de teste' }).getAttribute('href')).toBe(`https://fonte.example/vagas/${g}`);
      if (g !== 'C') fireEvent.click(screen.getByRole('button', { name: 'Decidir depois' }));
    }
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
