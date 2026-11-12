// Porta de entrada da conta (Task 7): criar e confirmar com o código, entrar, os motivos da nuvem (confirmar,
// termos, entrar-com-senha), o Google pelo navegador e a seção Conta do Perfil (sair, excluir). Só dados fictícios.
import { act, cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, ErroApi, type App } from '../app/contexto';
import { TERMOS_VERSAO } from '../app/conta';
import { SecaoConta, TelaConta } from './Conta';

afterEach(() => { cleanup(); vi.useRealTimers(); });

type Resp = (corpo: any) => any;
function montar(rotas: Record<string, Resp | Resp[]>) {
  const chamadas: { rota: string; corpo: any }[] = [];
  const filas: Record<string, Resp[]> = Object.fromEntries(Object.entries(rotas).map(([k, v]) => [k, Array.isArray(v) ? [...v] : [v]]));
  const api = vi.fn(async (caminho: string, metodo = 'GET', corpo?: any) => {
    const rota = `${metodo} ${caminho}`;
    chamadas.push({ rota, corpo });
    const fila = filas[rota];
    if (!fila) throw new Error(`chamada inesperada ${rota}`);
    const f = fila.length > 1 ? fila.shift()! : fila[0];
    return f(corpo);
  });
  const aoEntrar = vi.fn(async () => {});
  render(<TelaConta api={api} aoEntrar={aoEntrar} />);
  return { api, chamadas, aoEntrar };
}
const erro = (status: number, dados: any) => () => { throw new ErroApi(dados.erro, status, dados); };
const ok = (v: any = {}) => () => v;
const digitar = (rotulo: string | RegExp, valor: string) => fireEvent.change(screen.getByLabelText(rotulo), { target: { value: valor } });
const clicar = async (nome: string | RegExp) => { await act(async () => { fireEvent.click(screen.getByRole('button', { name: nome })); }); };
const enviar = async (nome: string | RegExp) => { await act(async () => { fireEvent.submit(screen.getByRole('button', { name: nome }).closest('form')!); }); };

describe('Criar conta', () => {
  it('envia, pede o código e confirma com a mesma senha', async () => {
    const { chamadas, aoEntrar } = montar({
      'POST /api/v1/conta/criar': ok({ mensagem: 'Enviamos um código de confirmação para o seu email' }),
      'POST /api/v1/conta/confirmar': ok({ entrada: true }),
    });
    await clicar('Criar uma conta');
    expect(document.activeElement).toBe(screen.getByLabelText('Email'));
    digitar('Email', 'ana@exemplo.com');
    digitar('Senha', 'senha-forte-1');
    fireEvent.click(screen.getByLabelText(/Li e aceito/));
    await enviar('Criar conta');
    expect(chamadas[0]).toEqual({ rota: 'POST /api/v1/conta/criar', corpo: { email: 'ana@exemplo.com', senha: 'senha-forte-1', termos: TERMOS_VERSAO } });
    expect(screen.getByText(/Enviamos um código para ana@exemplo.com/)).toBeTruthy();
    // pedir outro código: só depois de 60 s
    expect(screen.getByText('Pode pedir outro código em 60 s')).toBeTruthy();
    expect((screen.getByRole('button', { name: 'Pedir outro código' }) as HTMLButtonElement).disabled).toBe(true);
    digitar('Código de confirmação', '123456');
    await enviar('Confirmar');
    expect(chamadas[1]).toEqual({ rota: 'POST /api/v1/conta/confirmar', corpo: { email: 'ana@exemplo.com', codigo: '123456', senha: 'senha-forte-1' } });
    expect(aoEntrar).toHaveBeenCalledTimes(1);
  });

  it('sem aceitar os termos, não envia', async () => {
    const { chamadas } = montar({});
    await clicar('Criar uma conta');
    digitar('Email', 'ana@exemplo.com');
    digitar('Senha', 'senha-forte-1');
    await enviar('Criar conta');
    expect(chamadas).toEqual([]);
    expect(screen.getByRole('alert').textContent).toContain('aceite os termos');
  });

  it('depois de 60 s, dá para pedir outro código', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { chamadas } = montar({ 'POST /api/v1/conta/criar': ok() });
    await clicar('Criar uma conta');
    digitar('Email', 'ana@exemplo.com');
    digitar('Senha', 'senha-forte-1');
    fireEvent.click(screen.getByLabelText(/Li e aceito/));
    await enviar('Criar conta');
    await act(async () => { await vi.advanceTimersByTimeAsync(61_000); });
    const botao = screen.getByRole('button', { name: 'Pedir outro código' }) as HTMLButtonElement;
    expect(botao.disabled).toBe(false);
    await clicar('Pedir outro código');
    expect(chamadas.map(c => c.rota)).toEqual(['POST /api/v1/conta/criar', 'POST /api/v1/conta/criar']);
    expect(screen.getByText('Pode pedir outro código em 60 s')).toBeTruthy();
  });
});

describe('Entrar', () => {
  it('o erro da nuvem aparece na tela', async () => {
    montar({ 'POST /api/v1/conta/entrar': erro(401, { erro: 'Email ou senha incorretos' }) });
    expect(document.activeElement).toBe(screen.getByLabelText('Email'));
    digitar('Email', 'ana@exemplo.com');
    digitar('Senha', 'errada-123');
    await enviar('Entrar');
    expect(screen.getByRole('alert').textContent).toBe('Email ou senha incorretos');
  });

  it('sem internet ou muitas tentativas: a frase do servidor', async () => {
    montar({ 'POST /api/v1/conta/entrar': erro(503, { erro: 'Sem conexão com a internet', motivo: 'sem-internet' }) });
    digitar('Email', 'ana@exemplo.com');
    digitar('Senha', 'senha-forte-1');
    await enviar('Entrar');
    expect(screen.getByRole('alert').textContent).toBe('Sem conexão com a internet');
  });

  it('conta não confirmada: vai para o código e confirma com a senha digitada', async () => {
    const { chamadas, aoEntrar } = montar({
      'POST /api/v1/conta/entrar': erro(409, { erro: 'Confirme o seu email', motivo: 'confirmar' }),
      'POST /api/v1/conta/confirmar': ok({ entrada: true }),
    });
    digitar('Email', 'ana@exemplo.com');
    digitar('Senha', 'senha-forte-1');
    await enviar('Entrar');
    digitar('Código de confirmação', '123456');
    await enviar('Confirmar');
    expect(chamadas[1].corpo).toEqual({ email: 'ana@exemplo.com', codigo: '123456', senha: 'senha-forte-1' });
    expect(aoEntrar).toHaveBeenCalled();
  });

  it('termos novos: mostra os termos e reenvia com a versão aceita', async () => {
    const { chamadas, aoEntrar } = montar({
      'POST /api/v1/conta/entrar': [erro(409, { erro: 'Os termos mudaram', motivo: 'termos', versao: '1-novo' }), ok({ entrada: true })],
    });
    digitar('Email', 'ana@exemplo.com');
    digitar('Senha', 'senha-forte-1');
    await enviar('Entrar');
    expect(screen.getByText('Os termos mudaram')).toBeTruthy();
    const caixa = within(screen.getByRole('group', { name: 'Termos novos' }));
    expect(caixa.getByRole('button', { name: 'Ler os termos de uso' })).toBeTruthy();
    expect(caixa.getByRole('button', { name: 'Ler a política de privacidade' })).toBeTruthy();
    await clicar('Aceito os novos termos');
    expect(chamadas[1]).toEqual({ rota: 'POST /api/v1/conta/entrar', corpo: { email: 'ana@exemplo.com', senha: 'senha-forte-1', termos: '1-novo' } });
    expect(aoEntrar).toHaveBeenCalled();
  });

  it('esqueci a senha: código e senha nova', async () => {
    const { chamadas, aoEntrar } = montar({ 'POST /api/v1/conta/recuperar': ok(), 'POST /api/v1/conta/nova-senha': ok({ entrada: true }) });
    await clicar('Esqueci a senha');
    digitar('Email', 'ana@exemplo.com');
    await enviar('Enviar código');
    digitar('Código de confirmação', '123456');
    digitar('Senha nova', 'senha-nova-22');
    await enviar('Salvar senha nova');
    expect(chamadas.map(c => c.corpo)).toEqual([{ email: 'ana@exemplo.com' }, { email: 'ana@exemplo.com', codigo: '123456', senha: 'senha-nova-22' }]);
    expect(aoEntrar).toHaveBeenCalled();
  });
});

describe('Google', () => {
  it('aguardando no navegador; cancelar volta para a entrada', async () => {
    const { chamadas } = montar({
      'POST /api/v1/conta/google': ok({ mensagem: 'Continue no navegador para entrar com o Google' }),
      'GET /api/v1/conta': ok({ ativa: true, entrada: false, motivo: 'sem-conta', google: 'aguardando' }),
      'POST /api/v1/conta/google/cancelar': ok(''),
    });
    await clicar('Entrar com Google');
    expect(chamadas[0]).toEqual({ rota: 'POST /api/v1/conta/google', corpo: { termos: TERMOS_VERSAO } });
    await waitFor(() => expect(screen.getByText(/Continue no navegador/)).toBeTruthy());
    await clicar('Cancelar');
    expect(chamadas.some(c => c.rota === 'POST /api/v1/conta/google/cancelar')).toBe(true);
    expect(screen.getByRole('button', { name: 'Entrar' })).toBeTruthy();
  });

  it('entrou pelo navegador: avisa a casca', async () => {
    const { aoEntrar } = montar({
      'POST /api/v1/conta/google': ok(),
      'GET /api/v1/conta': ok({ ativa: true, entrada: true, motivo: '', google: '' }),
    });
    await clicar('Entrar com Google');
    await waitFor(() => expect(aoEntrar).toHaveBeenCalled());
  });

  it('Google recusou (entrar-com-senha): mostra a frase e o formulário da senha', async () => {
    montar({
      'POST /api/v1/conta/google': ok(),
      'GET /api/v1/conta': ok({ ativa: true, entrada: false, motivo: 'sem-conta', google: 'erro:Esta conta usa senha: entre com o email e a senha', google_motivo: 'entrar-com-senha' }),
    });
    await clicar('Entrar com Google');
    await waitFor(() => expect(screen.getByRole('alert').textContent).toBe('Esta conta usa senha: entre com o email e a senha'));
    expect(screen.getByLabelText('Senha')).toBeTruthy();
  });
});

describe('Seção Conta do Perfil', () => {
  function montarSecao(resposta: unknown = '') {
    const chamadas: { rota: string; corpo: any }[] = [];
    const releConta = vi.fn(async () => {});
    const avisos: string[] = [];
    const app = {
      estado: {}, conta: { ativa: true, entrada: true, email: 'ana@exemplo.com', estado: 'teste', motivo: '' }, releConta,
      aviso: (t: string) => avisos.push(t), perguntar: async () => 'ok', recarregar: async () => {},
      api: vi.fn(async (caminho: string, metodo = 'GET', corpo?: any) => { chamadas.push({ rota: `${metodo} ${caminho}`, corpo }); return resposta; }),
    } as unknown as App;
    render(<Contexto.Provider value={app}><SecaoConta /></Contexto.Provider>);
    return { chamadas, releConta, avisos };
  }
  it('mostra o email e sai da conta', async () => {
    const { chamadas, releConta } = montarSecao();
    expect(screen.getByText('ana@exemplo.com')).toBeTruthy();
    await clicar('Sair da conta');
    expect(chamadas).toEqual([{ rota: 'POST /api/v1/conta/sair', corpo: {} }]);
    expect(releConta).toHaveBeenCalled();
  });
  it('excluir a conta exige digitar o email', async () => {
    const { chamadas, releConta } = montarSecao({ apagado: ['banco'], faltou: [], completo: true, fechar: true });
    const excluir = () => screen.getByRole('button', { name: 'Excluir conta' }) as HTMLButtonElement;
    expect(excluir().disabled).toBe(true);
    digitar('Para confirmar, digite o email da conta', 'outra@exemplo.com');
    expect(excluir().disabled).toBe(true);
    await enviar('Excluir conta');
    expect(chamadas).toEqual([]);
    digitar('Para confirmar, digite o email da conta', 'ana@exemplo.com');
    expect(excluir().disabled).toBe(false);
    fireEvent.click(screen.getByLabelText('Apagar também os dados deste computador'));
    await enviar('Excluir conta');
    expect(chamadas).toEqual([{ rota: 'POST /api/v1/conta/excluir', corpo: { email: 'ana@exemplo.com', apagarDados: true } }]);
    expect(releConta).not.toHaveBeenCalled(); // apagou os dados: o app fecha
    expect(screen.getByText(/O app vai fechar/)).toBeTruthy();
  });
  it('a nuvem excluiu mas algo falhou aqui: mostra o aviso', async () => {
    montarSecao({ contaExcluida: true, dadosApagados: false, erro: 'A conta foi excluída, mas não deu para encerrar a sessão neste computador.' });
    digitar('Para confirmar, digite o email da conta', 'ana@exemplo.com');
    await enviar('Excluir conta');
    expect(screen.getByRole('alert').textContent).toContain('não deu para encerrar a sessão');
  });
});

describe('Rodada de correções 1', () => {
  it('a espera do Google mantém a cadência de 2 s mesmo com a casca redesenhando', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const gets: number[] = [];
    const api = vi.fn(async (caminho: string, metodo = 'GET') => {
      if (metodo === 'GET' && caminho === '/api/v1/conta') { gets.push(Date.now()); return { ativa: true, entrada: false, google: 'aguardando' }; }
      return {};
    });
    const r = render(<TelaConta api={api} aoEntrar={() => {}} />);
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Entrar com Google' })); });
    // A casca redesenha 10 vezes, cada uma com um aoEntrar novo.
    for (let i = 0; i < 10; i++) await act(async () => { r.rerender(<TelaConta api={api} aoEntrar={() => {}} />); });
    await act(async () => { await vi.advanceTimersByTimeAsync(4100); });
    expect(gets.length).toBe(3); // agora + 2 s + 4 s
  });

  it('cancelar o Google: uma leitura que volta depois não mexe na tela', async () => {
    let soltar: (v: unknown) => void = () => {};
    const api = vi.fn(async (caminho: string, metodo = 'GET') => {
      if (metodo === 'GET' && caminho === '/api/v1/conta') return new Promise(r => { soltar = r; });
      return {};
    });
    render(<TelaConta api={api} aoEntrar={() => {}} />);
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Entrar com Google' })); });
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Cancelar' })); });
    await act(async () => { soltar({ ativa: true, entrada: false, google: 'erro:Algo deu errado', google_motivo: '' }); });
    expect(screen.queryByRole('alert')).toBeNull();
    expect(screen.getByRole('button', { name: 'Entrar' })).toBeTruthy();
  });

  it('esqueci a senha: a senha digitada antes não vem preenchida', async () => {
    montar({ 'POST /api/v1/conta/recuperar': ok() });
    digitar('Email', 'ana@exemplo.com');
    digitar('Senha', 'senha-antiga-1');
    await clicar('Esqueci a senha');
    await enviar('Enviar código');
    expect((screen.getByLabelText('Senha nova') as HTMLInputElement).value).toBe('');
  });

  it('criar com termos novos: oferece aceitar e reenvia com a versão', async () => {
    const { chamadas } = montar({
      'POST /api/v1/conta/criar': [erro(409, { erro: 'Os termos mudaram', motivo: 'termos', versao: '1-novo' }), ok()],
    });
    await clicar('Criar uma conta');
    digitar('Email', 'ana@exemplo.com');
    digitar('Senha', 'senha-forte-1');
    fireEvent.click(screen.getByLabelText(/Li e aceito/));
    await enviar('Criar conta');
    await clicar('Aceito os novos termos');
    expect(chamadas[1]).toEqual({ rota: 'POST /api/v1/conta/criar', corpo: { email: 'ana@exemplo.com', senha: 'senha-forte-1', termos: '1-novo' } });
    expect(screen.getByLabelText('Código de confirmação')).toBeTruthy();
  });
});
