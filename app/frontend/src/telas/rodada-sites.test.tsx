// Rodada dos sites das empresas: escolhe as vagas que vão para o site da empresa; o ▶ reserva, abre, mostra o domínio,
// preenche só depois da conferência e passa para a próxima com "Já enviei" ou "Pular". O app nunca envia. Dados fictícios.
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, ErroApi, type App } from '../app/contexto';
import type { Candidatura } from './Fila';
import { RodadaSites, vagasDaRodadaSites } from './RodadaSites';

afterEach(cleanup);
const cand = (id: string, extra: Partial<Candidatura> = {}): Candidatura => ({
  id: 'c-' + id, vagaId: id, plataforma: 'linkedin', url: 'https://www.linkedin.com/jobs/view/1/', estado: 'preparada', impressao: 'x',
  conteudo: { curriculo: { arquivo: 'cv.pdf' }, idioma: 'pt', contato: null, pretensao: null, respostas: [], mensagem: null },
  aprovacao: null, bloqueio: null, resultado: null, versao: 1, criadaEm: `2026-11-16T10:00:0${id.length}Z`, atualizadaEm: '2026-11-16T10:00:00Z', ...extra,
});
const vaga = (id: string, extra: object = {}) => ({ id, empresa: 'Empresa ' + id, titulo: 'Dev', etapa: 'aplicar', ...extra });
const externo = { estado: 'bloqueada' as const, bloqueio: { motivo: 'site-externo', pendencia: '', em: '2026-11-16T10:00:00Z' } };

describe('vagasDaRodadaSites', () => {
  it('pega as que vão ao site da empresa e deixa o LinkedIn com Candidatura simplificada de fora', () => {
    const vagas = [vaga('ext', { etapa: 'bloqueada' }), vaga('res', { aplicacaoManual: true }), vaga('jobicy'), vaga('li'), vaga('cap'), vaga('env', { envio: { data: '2026-11-15' } }), vaga('sem')];
    const cs = [cand('ext', externo), cand('res', { estado: 'bloqueada', bloqueio: { motivo: 'site-externo', pendencia: '', em: '' } }),
      cand('jobicy', { plataforma: 'outra' }), cand('li'), cand('cap', { estado: 'bloqueada', bloqueio: { motivo: 'captcha', pendencia: '', em: '' } }),
      cand('env', { plataforma: 'outra' })];
    expect(vagasDaRodadaSites(vagas, cs).map(x => x.vaga.id).sort()).toEqual(['ext', 'jobicy', 'res']);
  });
});

function montar(vagas: object[], cs: Candidatura[], api: (rota: string, m?: string, corpo?: any) => Promise<unknown>) {
  const atualizar = vi.fn(async () => {});
  const espia = vi.fn(api);
  const app = { api: espia, aviso: vi.fn(), estado: { vagas } } as unknown as App;
  render(<Contexto.Provider value={app}><RodadaSites candidaturas={cs} atualizar={atualizar} /></Contexto.Provider>);
  return { api: espia, app, atualizar };
}

describe('RodadaSites', () => {
  it('passa pelas vagas: reserva, abre, confere o domínio, preenche, Já enviei, pula, conclui', async () => {
    const vagas = [vaga('a', { etapa: 'bloqueada' }), vaga('b', { aplicacaoManual: true })];
    const cs = [cand('a', externo), cand('b', { ...externo, criadaEm: '2026-11-16T11:00:00Z' })];
    const { api, app } = montar(vagas, cs, async (rota, _m, corpo) => {
      if (rota.endsWith('/preencher') && !corpo?.dominio) return { dominio: rota.includes('/a/') ? 'carreiras.a.com.br' : 'jobs.b.example' };
      if (rota.endsWith('/preencher')) return { preenchidos: 3, anexou: true, faltando: 1, pedeCPF: 1, cpfSalvo: true };
      return {};
    });
    expect(screen.getByText(/Vão nesta rodada: Empresa a — Dev · Empresa b — Dev/)).toBeTruthy();
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Começar rodada (2)' })); });
    const grupo = () => screen.getByRole('group', { name: 'Vaga da rodada' });
    await waitFor(() => expect(within(grupo()).getByText('carreiras.a.com.br')).toBeTruthy());
    expect(api.mock.calls.map(([r, m]) => `${m} ${r}`)).toEqual([
      'POST /api/v1/vagas/a/iniciar-manual', 'POST /api/v1/vagas/a/abrir-para-preencher', 'POST /api/v1/vagas/a/preencher']);
    expect(within(grupo()).getByText(/Vaga 1 de 2/)).toBeTruthy();
    await act(async () => { fireEvent.click(within(grupo()).getByRole('button', { name: 'Preencher' })); });
    expect(api).toHaveBeenLastCalledWith('/api/v1/vagas/a/preencher', 'POST', { dominio: 'carreiras.a.com.br' });
    expect(within(grupo()).getByText(/Preenchi 3 campos e anexei o currículo/)).toBeTruthy();
    await act(async () => { fireEvent.click(within(grupo()).getByRole('button', { name: 'Preencher CPF em carreiras.a.com.br' })); });
    expect(api).toHaveBeenLastCalledWith('/api/v1/vagas/a/preencher', 'POST', { dominio: 'carreiras.a.com.br', cpf: true });
    await act(async () => { fireEvent.click(within(grupo()).getByRole('button', { name: 'Já enviei: próxima' })); });
    expect(api).toHaveBeenCalledWith('/api/v1/vagas/a/confirmar-envio-manual', 'POST', {});
    // a segunda já estava reservada: não reserva de novo
    await waitFor(() => expect(within(grupo()).getByText('jobs.b.example')).toBeTruthy());
    expect(api.mock.calls.some(([r]) => r === '/api/v1/vagas/b/iniciar-manual')).toBe(false);
    await act(async () => { fireEvent.click(within(grupo()).getByRole('button', { name: 'Não é este site: pular' })); });
    expect(api.mock.calls.some(([r]) => r === '/api/v1/vagas/b/confirmar-envio-manual')).toBe(false);
    expect(screen.getAllByText(/Rodada concluída: 1 enviada, 1 pulada/).length).toBeGreaterThan(0);
    expect(app.aviso).toHaveBeenCalledWith('Rodada dos sites concluída: 1 enviada, 1 pulada.');
  });

  it('erro ao abrir: frase fixa, tentar de novo ou pular', async () => {
    const { api } = montar([vaga('a', { aplicacaoManual: true })], [cand('a', externo)], async rota => {
      if (rota.endsWith('/abrir-para-preencher')) throw new ErroApi('texto do servidor', 409, { erro: 'texto do servidor', motivo: 'login-aberto' });
      return {};
    });
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Começar rodada (1)' })); });
    await waitFor(() => expect(screen.getByText(/Já há uma janela do app aberta/)).toBeTruthy());
    expect(document.body.textContent).not.toContain('texto do servidor');
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Tentar de novo' })); });
    expect(api.mock.calls.filter(([r]) => r.endsWith('/abrir-para-preencher'))).toHaveLength(2);
  });

  it('sem vagas para o site da empresa: o bloco não aparece', () => {
    montar([vaga('li')], [cand('li')], async () => ({}));
    expect(screen.queryByRole('region', { name: 'Rodada dos sites das empresas' })).toBeNull();
    expect(document.body.textContent).not.toContain('Rodada dos sites');
  });
});
