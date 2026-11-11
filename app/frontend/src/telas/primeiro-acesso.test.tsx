import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, ErroApi, chamarApi, type App } from '../app/contexto';
import { PrimeiroAcesso } from './PrimeiroAcesso';

afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

const perfil = {
  cargo: 'Designer', stackPrincipal: ['Pesquisa'], stackSecundaria: ['Figma'],
  filtros: { niveis: ['Pleno'], modelo: ['Remoto'], pularSe: ['Plantão'] },
  foco: 'internacional', ingles: 'Avançado', pretensaoUSD: 3200,
};

function montar(iaPronta: boolean, api = vi.fn().mockResolvedValue({ ok: true })) {
  const concluir = vi.fn();
  const app = { estado: { perfil }, rota: 'hoje', api, limpar: vi.fn(), recarregar: vi.fn(), aviso: vi.fn(), navegar: vi.fn() } as unknown as App;
  const tela = render(<Contexto.Provider value={app}><PrimeiroAcesso iaPronta={iaPronta} alternarTema={() => {}} concluir={concluir} /></Contexto.Provider>);
  return { tela, concluir, app, api };
}

function anexar(arquivo: File) {
  fireEvent.change(screen.getByLabelText('Currículo em PDF'), { target: { files: [arquivo] } });
}
const proximo = () => fireEvent.click(screen.getByRole('button', { name: /Continuar/ }));

describe('Primeiro acesso', () => {
  it('exige um PDF verdadeiro e mantém o arquivo anterior se a substituição falhar', async () => {
    montar(true);
    proximo();
    expect(screen.getByRole('alert').textContent).toMatch(/Escolha seu currículo/);
    anexar(new File(['%PDF-1.4\n%%EOF'], 'Curriculo.pdf', { type: 'application/pdf' }));
    await screen.findByRole('heading', { name: 'Curriculo.pdf' });
    anexar(new File(['uma página HTML'], 'Outro.pdf', { type: 'application/pdf' }));
    await screen.findByText('Esse arquivo não tem o formato de um PDF.');
    anexar(new File(['%PDF-1.4\n1 0 obj'], 'Incompleto.pdf', { type: 'application/pdf' }));
    await screen.findByText(/O PDF está incompleto/);
    expect(screen.getByRole('heading', { name: 'Curriculo.pdf' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Remover' }));
    proximo();
    expect(screen.getByRole('alert').textContent).toMatch(/Escolha seu currículo/);
  });

  it.each([true, false])('envia o PDF e as edições, preservando os campos após um erro de gravação (IA %s)', async iaPronta => {
    const api = vi.fn().mockRejectedValueOnce(new ErroApi('PDF recusado', 400, {})).mockResolvedValueOnce({ ok: true });
    const { concluir } = montar(iaPronta, api);
    const pdf = new File(['%PDF-1.4\n%%EOF'], 'MeuCV.pdf', { type: 'application/pdf' });
    anexar(pdf);
    await screen.findByRole('heading', { name: 'MeuCV.pdf' });
    proximo();
    const cargo = screen.getByLabelText('Cargo ou área de interesse');
    expect((cargo as HTMLInputElement).value).toBe('Designer');
    fireEvent.change(cargo, { target: { value: 'Analista de pesquisa' } });
    fireEvent.change(screen.getByLabelText(/Stack secundária/), { target: { value: 'Figma; SQL, Figma' } });
    proximo();
    fireEvent.click(screen.getByRole('radio', { name: 'Nacional' }));
    expect(screen.queryByLabelText(/Pretensão internacional/)).toBeNull();
    proximo();
    const nome = iaPronta ? /Salvar e buscar vagas/ : /Salvar e conectar IA/;
    expect(screen.getByText('Analista de pesquisa')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: nome }));
    await screen.findByText('PDF recusado');
    expect(concluir).not.toHaveBeenCalled();
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: nome })); });
    await waitFor(() => expect(concluir).toHaveBeenCalledWith(iaPronta));
    const [url, metodo, dados, headers] = api.mock.calls[1];
    expect([url, metodo, headers]).toEqual(['/api/v1/onboarding', 'POST', { 'X-Applyer-Onboarding': '1' }]);
    expect(dados.get('curriculo')).toBe(pdf);
    expect(JSON.parse(dados.get('perfil'))).toEqual({
      cargo: 'Analista de pesquisa', stackPrincipal: ['Pesquisa'], stackSecundaria: ['Figma', 'SQL'],
      filtros: perfil.filtros, foco: 'nacional', ingles: 'Avançado',
    });
  });

  it('preserva o FormData para o navegador gerar o boundary do multipart', async () => {
    const fetch = vi.fn().mockResolvedValue({ ok: true, headers: { get: () => 'application/json' }, json: async () => ({ ok: true }) });
    vi.stubGlobal('fetch', fetch);
    const corpo = new FormData();
    await chamarApi('/api/v1/onboarding', 'POST', corpo, { 'X-Applyer-Onboarding': '1' });
    expect(fetch.mock.calls[0][1]).toEqual({ method: 'POST', headers: { 'X-Applyer-Onboarding': '1' }, body: corpo });
  });
});
