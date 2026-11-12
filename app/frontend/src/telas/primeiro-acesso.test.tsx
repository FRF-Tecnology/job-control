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
const desabilitado = (nome: string) => (screen.getByRole('button', { name: nome }) as HTMLButtonElement).disabled;

type Arquivo = {arquivoId: string; nomeExibicao: string; tamanho: number};
const rascunhoVazio = () => ({ revisao: 0, dados: {} as Record<string, unknown>, etapa: 1, editados: [] as string[], curriculo: null as Arquivo | null });
function montar(base: object = perfil, sugerir = vi.fn().mockResolvedValue({ sugestoes: [] }), salvar = vi.fn().mockResolvedValue({ ok: true }), rascunho = rascunhoVazio()) {
  const concluir = vi.fn();
  const api = vi.fn(async (url: string, _metodo?: string, corpo?: unknown, cabecalhos?: Record<string, string>) => {
    const c = corpo as any;
    if (url.endsWith('/rascunho') && !_metodo) return structuredClone(rascunho);
    if (url.endsWith('/rascunho') || url.endsWith('/onboarding/curriculo')) {
      if (c.revisao !== rascunho.revisao) throw new ErroApi('A revisão mudou.', 409, {});
      rascunho.revisao++;
      if (_metodo === 'PATCH') {
        Object.assign(rascunho.dados, c.dados); rascunho.etapa = c.etapa ?? rascunho.etapa; rascunho.editados = c.editados ?? rascunho.editados;
      } else if (_metodo === 'PUT') {
        rascunho.curriculo = { arquivoId: 'cv-' + rascunho.revisao, nomeExibicao: c.nomeArquivo, tamanho: atob(c.curriculoBase64).length };
        return { ...rascunho.curriculo, revisao: rascunho.revisao };
      } else if (_metodo === 'DELETE') rascunho.curriculo = null;
      return structuredClone(rascunho);
    }
    if (url === '/api/v1/ia') return { configurada: true, provedor: 'anthropic', chaves: { anthropic: '…1234' }, clis: [] };
    if (url === '/api/v1/ia/testar') return { ok: true, ms: 12 };
    if (url === '/api/v1/onboarding/sugerir') return sugerir(corpo, cabecalhos);
    if (url === '/api/v1/onboarding') return salvar(corpo, cabecalhos);
    return {};
  });
  const app = { estado: { perfil: base }, api, limpar: vi.fn(), recarregar: vi.fn(), aviso: vi.fn() } as unknown as App;
  render(<Contexto.Provider value={app}><PrimeiroAcesso alternarTema={() => {}} concluir={concluir} /></Contexto.Provider>);
  return { concluir, sugerir, salvar, api, rascunho };
}

async function conectar() {
  fireEvent.click(await screen.findByRole('button', { name: 'Usar conexão atual' }));
  await screen.findByText('Claude conectado');
}
async function revisar() {
  clicar('Preencher manualmente');
  await screen.findByRole('heading', {name:'Que vagas você procura?'});
}
async function voltar() {
  clicar('Voltar');
  await screen.findByRole('heading', {name:'Sua IA e seu currículo'});
}
function anexar(arquivo: File) {
  fireEvent.change(screen.getByLabelText('Currículo em PDF'), { target: { files: [arquivo] } });
}
async function escolher(nome: string) {
  await waitFor(() => expect((screen.getByRole('button', {name:/Escolher PDF|Trocar PDF/}) as HTMLButtonElement).disabled).toBe(false));
  anexar(pdf(nome));
  await screen.findByText(nome, { exact: true });
}
function editar(rotulo: string | RegExp, valor: string) {
  fireEvent.change(screen.getByLabelText(rotulo), { target: { value: valor } });
}

describe('Primeiro acesso', () => {
  it('reabre a etapa, os campos e o mesmo PDF após verificar a conexão novamente', async () => {
    const { rascunho } = montar();
    await escolher('Curriculo original.pdf'); await conectar(); await revisar();
    await screen.findByRole('heading', {name: 'Que vagas você procura?'});
    editar('Cargo ou área de interesse','Java remoto');
    await waitFor(() => expect(rascunho.dados.cargo).toBe('Java remoto'));
    expect(rascunho.etapa).toBe(2);
    cleanup(); montar(perfil, undefined, undefined, rascunho);
    await screen.findByText('Curriculo original.pdf');
    expect(screen.queryByText('Claude conectado')).toBeNull();
    await conectar();
    expect(await screen.findByDisplayValue('Java remoto')).toBeTruthy();
  });
  it('conserva o PDF anterior quando falta espaço e permite repetir a troca', async () => {
    const { api } = montar();
    await escolher('Anterior.pdf');
    api.mockRejectedValueOnce(new Error('Disco cheio.'));
    anexar(pdf('Novo.pdf'));
    await screen.findByRole('button', {name:'Tentar salvar novamente'});
    expect(screen.getByText('Anterior.pdf')).toBeTruthy();
    clicar('Tentar salvar novamente');
    await screen.findByText('Novo.pdf');
    expect(screen.queryByText('Anterior.pdf')).toBeNull();
  });
  it('mantém a edição local em conflito até a pessoa escolher a versão da outra janela', async () => {
    const {rascunho} = montar();
    await escolher('A.pdf'); await conectar(); await revisar();
    rascunho.revisao++; rascunho.dados.cargo='Cargo da outra janela';
    editar('Cargo ou área de interesse','Cargo local');
    await screen.findByRole('button',{name:/Descartar edições locais/});
    expect(screen.getByDisplayValue('Cargo local')).toBeTruthy();
    clicar(/Descartar edições locais/);
    await screen.findByText('A.pdf');
    await conectar();
    expect(await screen.findByDisplayValue('Cargo da outra janela')).toBeTruthy();
  });
  it('aceita selecionar o PDF primeiro, mas exige a conexão para abrir a revisão', async () => {
    montar();
    await escolher('Perfil.pdf');
    expect(desabilitado('Preencher com IA')).toBe(true);
    expect(desabilitado('Preencher manualmente')).toBe(true);
    await conectar();
    expect(desabilitado('Preencher com IA')).toBe(false);
    await revisar();
    expect(screen.getByRole('heading', { name: 'Que vagas você procura?' })).toHaveProperty('tabIndex', -1);
    expect(screen.getByLabelText('Cargo ou área de interesse')).toBeTruthy();
    expect(screen.getByLabelText('Seu nível de inglês')).toBeTruthy();
    await voltar(); clicar('Alterar conexão');
    expect(desabilitado('Preencher manualmente')).toBe(true);
    expect(screen.getByText('Perfil.pdf')).toBeTruthy();
  });

  it('exige um PDF completo e mantém o arquivo anterior se a substituição falhar', async () => {
    montar();
    await conectar();
    expect(desabilitado('Preencher com IA')).toBe(true);
    await escolher('Curriculo.pdf');
    anexar(pdf('Outro.pdf', 'uma página HTML'));
    await screen.findByText('Esse arquivo não tem o formato de um PDF.');
    anexar(pdf('Incompleto.pdf', '%PDF-1.4\n1 0 obj'));
    await screen.findByText(/O PDF está incompleto/);
    expect(screen.getByText('Curriculo.pdf')).toBeTruthy();
    clicar('Remover');
    expect(desabilitado('Preencher com IA')).toBe(true);
  });

  it.each(['USD', 'BRL', 'EUR'])('salva a faixa mensal na moeda %s e o PDF intacto; mantém rascunho após um erro', async moeda => {
    const salvar = vi.fn().mockRejectedValueOnce(new ErroApi('PDF recusado', 400, {})).mockResolvedValueOnce({ ok: true });
    const { concluir, sugerir } = montar(perfil, undefined, salvar);
    await conectar(); await escolher('MeuCV.pdf');
    await revisar();
    editar('Cargo ou área de interesse', 'Analista de pesquisa'); editar(/Stack secundária/, 'Figma; SQL, Figma');
    fireEvent.click(screen.getByRole('radio', { name: 'Nacional' }));
    expect((screen.getByLabelText('Mínimo') as HTMLInputElement).value).toBe('3200');
    editar('Moeda', moeda); editar('Mínimo', '4000'); editar('Máximo', '3000'); clicar('Concluir perfil');
    expect(screen.getByRole('alert').textContent).toMatch(/maior ou igual/);
    editar('Máximo', '6000');
    clicar('Concluir perfil'); await screen.findByText('PDF recusado');
    expect(concluir).not.toHaveBeenCalled();
    clicar('Concluir perfil'); await waitFor(() => expect(concluir).toHaveBeenCalledWith(false));
    expect(sugerir).not.toHaveBeenCalled();
    const [corpo, cabecalhos] = salvar.mock.calls[1];
    expect(cabecalhos).toEqual({ 'X-Applyer-Onboarding': '1' });
    expect(corpo.arquivoId).toMatch(/^cv-/);
    expect(corpo.curriculoBase64).toBeUndefined();
    expect(corpo.perfil).toEqual({
      cargo: 'Analista de pesquisa', stackPrincipal: ['Pesquisa'], stackSecundaria: ['Figma', 'SQL'],
      filtros: perfil.filtros, foco: 'nacional', ingles: 'Avançado', faixaSalarial: { min: 4000, max: 6000, moeda },
    });
  });

  it('só pede a primeira busca ao concluir com Salvar e buscar vagas', async () => {
    const { concluir } = montar();
    await conectar(); await escolher('Busca.pdf');
    await revisar();
    clicar('Salvar e buscar vagas');
    await waitFor(() => expect(concluir).toHaveBeenCalledWith(true));
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
    editar('Cargo ou área de interesse', 'Analista de BI');
    expect((screen.getByLabelText('Seu nível de inglês') as HTMLSelectElement).value).toBe('Avançado');
    expect((screen.getByLabelText('Mínimo') as HTMLInputElement).value).toBe('');
    expect((screen.getByLabelText('Remoto') as HTMLInputElement).checked).toBe(false);
    await voltar(); await escolher('B.pdf'); clicar('Preencher com IA');
    await screen.findByDisplayValue('SQL');
    expect(screen.getByDisplayValue('Analista de BI')).toBeTruthy();
    expect((screen.getByLabelText('Pleno') as HTMLInputElement).checked).toBe(false);
    expect(sugerir).toHaveBeenCalledTimes(2);
  });

  it('permite preenchimento manual após falha de leitura da IA', async () => {
    montar({}, vi.fn().mockRejectedValue(new Error('O PDF não contém texto selecionável.')));
    await conectar(); await escolher('Digitalizado.pdf'); clicar('Preencher com IA');
    await screen.findByText('O PDF não contém texto selecionável.');
    await revisar();
    expect(screen.getByRole('heading', { name: 'Que vagas você procura?' })).toBeTruthy();
  });

  it('retira a faixa salarial anterior quando a pessoa deixa os dois limites em branco', async () => {
    const { salvar } = montar();
    await conectar(); await escolher('Perfil.pdf');
    await revisar();
    editar('Mínimo', '');
    clicar('Concluir perfil');
    await waitFor(() => expect(salvar).toHaveBeenCalled());
    expect(salvar.mock.calls[0][0].perfil.faixaSalarial).toBeNull();
  });
});
