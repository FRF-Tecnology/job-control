import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, ErroApi, type App } from '../app/contexto';
import { PrimeiroAcesso } from './PrimeiroAcesso';

afterEach(cleanup);

const perfil = {
  cargo: 'Designer', stackPrincipal: ['Pesquisa'], stackSecundaria: ['Figma'],
  filtros: { niveis: ['Pleno'], modelo: ['Remoto'], pularSe: ['Plantão'] },
  foco: 'internacional', ingles: 'Avançado', pretensaoUSD: 3200,
};
const pdf = (nome: string, conteudo = '%PDF-1.4\n%%EOF') => new File([conteudo], nome, { type: 'application/pdf' });
const clicar = (nome: string | RegExp) => fireEvent.click(screen.getByRole('button', { name: nome }));
const proximo = () => clicar('Continuar');

function montar(base: object = perfil, sugerir = vi.fn().mockResolvedValue({ sugestoes: [] }), salvar = vi.fn().mockResolvedValue({ ok: true })) {
  const concluir = vi.fn();
  const api = vi.fn(async (url: string, _metodo?: string, corpo?: unknown, cabecalhos?: Record<string, string>) => {
    if (url === '/api/v1/ia') return { configurada: true, provedor: 'openai', chaves: { openai: '…1234' }, clis: [] };
    if (url === '/api/v1/onboarding/sugerir') return sugerir(corpo, cabecalhos);
    if (url === '/api/v1/onboarding') return salvar(corpo, cabecalhos);
    return {};
  });
  const app = { estado: { perfil: base }, api, limpar: vi.fn(), recarregar: vi.fn(), aviso: vi.fn() } as unknown as App;
  render(<Contexto.Provider value={app}><PrimeiroAcesso alternarTema={() => {}} concluir={concluir} /></Contexto.Provider>);
  return { concluir, sugerir, salvar };
}

async function conectar() {
  fireEvent.click(await screen.findByRole('button', { name: 'Usar conexão atual' }));
  await screen.findByRole('heading', { name: 'Anexe seu currículo.' });
}
function anexar(arquivo: File) {
  fireEvent.change(screen.getByLabelText('Currículo em PDF'), { target: { files: [arquivo] } });
}
async function escolher(nome: string) {
  anexar(pdf(nome));
  await screen.findByRole('heading', { name: nome });
}
function editar(rotulo: string | RegExp, valor: string) {
  fireEvent.change(screen.getByLabelText(rotulo), { target: { value: valor } });
}

describe('Primeiro acesso', () => {
  it('começa pela conexão e só apresenta o PDF depois de confirmar o provedor', async () => {
    montar();
    await screen.findByRole('button', { name: 'Usar conexão atual' });
    expect(screen.getByRole('heading', { name: 'Conecte sua IA.' })).toBeTruthy();
    expect(screen.queryByLabelText('Currículo em PDF')).toBeNull();
    expect((screen.getByRole('button', { name: /Currículo Seu documento/ }) as HTMLButtonElement).disabled).toBe(true);
    await conectar();
  });

  it('exige um PDF completo e mantém o arquivo anterior se a substituição falhar', async () => {
    montar();
    await conectar();
    clicar('Preencher com IA');
    expect(screen.getByRole('alert').textContent).toMatch(/Escolha seu currículo/);
    await escolher('Curriculo.pdf');
    anexar(pdf('Outro.pdf', 'uma página HTML'));
    await screen.findByText('Esse arquivo não tem o formato de um PDF.');
    anexar(pdf('Incompleto.pdf', '%PDF-1.4\n1 0 obj'));
    await screen.findByText(/O PDF está incompleto/);
    expect(screen.getByRole('heading', { name: 'Curriculo.pdf' })).toBeTruthy();
    clicar('Remover');
    clicar('Preencher com IA');
    expect(screen.getByRole('alert').textContent).toMatch(/Escolha seu currículo/);
  });

  it.each(['USD', 'BRL', 'EUR'])('salva a faixa mensal na moeda %s e o PDF intacto; mantém rascunho após um erro', async moeda => {
    const salvar = vi.fn().mockRejectedValueOnce(new ErroApi('PDF recusado', 400, {})).mockResolvedValueOnce({ ok: true });
    const { concluir, sugerir } = montar(perfil, undefined, salvar);
    await conectar(); await escolher('MeuCV.pdf');
    clicar('Preencher manualmente');
    editar('Cargo ou área de interesse', 'Analista de pesquisa'); editar(/Stack secundária/, 'Figma; SQL, Figma');
    proximo();
    fireEvent.click(screen.getByRole('radio', { name: 'Nacional' }));
    expect((screen.getByLabelText('Mínimo') as HTMLInputElement).value).toBe('3200');
    editar('Moeda', moeda); editar('Mínimo', '4000'); editar('Máximo', '3000'); proximo();
    expect(screen.getByRole('alert').textContent).toMatch(/maior ou igual/);
    editar('Máximo', '6000'); proximo();
    expect(screen.getByText(new RegExp('por mês \\(' + moeda + '\\)'))).toBeTruthy();
    clicar('Salvar e buscar vagas'); await screen.findByText('PDF recusado');
    expect(concluir).not.toHaveBeenCalled();
    clicar('Salvar e buscar vagas'); await waitFor(() => expect(concluir).toHaveBeenCalledWith(true));
    expect(sugerir).not.toHaveBeenCalled();
    const [corpo, cabecalhos] = salvar.mock.calls[1];
    expect(cabecalhos).toEqual({ 'X-Applyer-Onboarding': '1' });
    expect(atob(corpo.curriculoBase64)).toBe('%PDF-1.4\n%%EOF');
    expect(corpo.perfil).toEqual({
      cargo: 'Analista de pesquisa', stackPrincipal: ['Pesquisa'], stackSecundaria: ['Figma', 'SQL'],
      filtros: perfil.filtros, foco: 'nacional', ingles: 'Avançado', faixaSalarial: { min: 4000, max: 6000, moeda },
    });
  });

  it('preenche pelos trechos do PDF, deixa as preferências livres e preserva edições se o PDF mudar', async () => {
    const sugerir = vi.fn().mockResolvedValueOnce({ sugestoes: [
      { campo: 'cargo', valor: 'Analista', evidencia: 'Analista na equipe de dados' },
      { campo: 'niveis', valor: 'Pleno', evidencia: 'Analista Pleno' },
      { campo: 'stackPrincipal', valor: 'Python', evidencia: 'Automação com Python' },
      { campo: 'ingles', valor: 'Avançado', evidencia: 'Inglês avançado' },
    ] }).mockResolvedValueOnce({ sugestoes: [{ campo: 'stackPrincipal', valor: 'SQL', evidencia: 'Consultas com SQL' }] });
    montar({}, sugerir);
    await conectar(); await escolher('A.pdf');
    expect(sugerir).not.toHaveBeenCalled();
    clicar('Preencher com IA');
    await screen.findByDisplayValue('Analista');
    expect(screen.getByText('“Automação com Python”')).toBeTruthy();
    expect((screen.getByLabelText('Pleno') as HTMLInputElement).checked).toBe(true);
    expect((screen.getByLabelText(/Pular quando/) as HTMLInputElement).value).toBe('');
    editar('Cargo ou área de interesse', 'Analista de BI'); proximo();
    expect((screen.getByLabelText('Seu nível de inglês') as HTMLSelectElement).value).toBe('Avançado');
    expect((screen.getByLabelText('Mínimo') as HTMLInputElement).value).toBe('');
    expect((screen.getByLabelText('Remoto') as HTMLInputElement).checked).toBe(false);
    clicar('← Voltar'); clicar('← Voltar'); await escolher('B.pdf'); clicar('Preencher com IA');
    await screen.findByDisplayValue('SQL');
    expect(screen.getByDisplayValue('Analista de BI')).toBeTruthy();
    expect((screen.getByLabelText('Pleno') as HTMLInputElement).checked).toBe(false);
    expect(sugerir).toHaveBeenCalledTimes(2);
  });

  it('permite preenchimento manual após falha de leitura da IA', async () => {
    montar({}, vi.fn().mockRejectedValue(new Error('O PDF não contém texto selecionável.')));
    await conectar(); await escolher('Digitalizado.pdf'); clicar('Preencher com IA');
    await screen.findByText('O PDF não contém texto selecionável.');
    clicar('Preencher manualmente');
    expect(screen.getByRole('heading', { name: 'Quais vagas interessam?' })).toBeTruthy();
  });
});
