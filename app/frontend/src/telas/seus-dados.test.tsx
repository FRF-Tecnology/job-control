// Seus dados neste computador (jc-a2y.49, rascunho com a API interceptada, pelo contrato proposto pelo Faker).
// Apagar exige digitar a frase; sair das plataformas pede confirmação; 409 e 403 em texto claro; falha parcial
// mostra o que ficou. Só dados fictícios.
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, ErroApi, type App, type Pergunta } from '../app/contexto';
import { SeusDados } from './SeusDados';

afterEach(cleanup);

const TUDO = ['banco', 'curriculos', 'backups', 'navegador', 'chaves'];
function montar({ resposta = 'ok' as string | null, erro = null as null | Error, retorno = { apagado: TUDO, faltou: [], completo: true, fechar: true } as any, sair = { apagado: ['navegador'], faltou: [], completo: true, fechar: false } as any } = {}) {
  const gravacoes: { caminho: string; corpo: any }[] = [];
  const perguntas: Pergunta[] = [];
  const recarregar = vi.fn(async () => {});
  const app = {
    estado: {}, perguntar: async (p: Pergunta) => { perguntas.push(p); return resposta; }, recarregar,
    api: vi.fn(async (caminho: string, _m = 'GET', corpo?: any) => {
      gravacoes.push({ caminho, corpo });
      if (erro) throw erro;
      return caminho.endsWith('/sair') ? sair : retorno;
    }),
  } as unknown as App;
  render(<Contexto.Provider value={app}><SeusDados /></Contexto.Provider>);
  return { gravacoes, perguntas, recarregar };
}
const apagar = () => screen.getByRole('button', { name: 'Apagar meus dados' }) as HTMLButtonElement;
const campo = () => screen.getByLabelText('Para confirmar, digite APAGAR MEUS DADOS');
const digitar = (v: string) => fireEvent.change(campo(), { target: { value: v } });
const clicar = async (el: HTMLElement) => { await act(async () => { fireEvent.click(el); }); };

describe('Apagar meus dados', () => {
  it('fica travado até digitar a frase exata, em maiúsculas', async () => {
    const { gravacoes } = montar();
    expect(apagar().disabled).toBe(true);
    for (const errado of ['apagar meus dados', 'APAGAR', 'APAGAR  MEUS DADOS']) {
      digitar(errado);
      expect(apagar().disabled).toBe(true);
    }
    await act(async () => { fireEvent.submit(campo().closest('form')!); });
    expect(gravacoes).toEqual([]);
    digitar(' APAGAR MEUS DADOS ');
    expect(apagar().disabled).toBe(false);
  });

  it('manda só {confirmacao} com a frase e avisa que o app vai fechar, sem reler', async () => {
    const { gravacoes, recarregar } = montar();
    digitar('APAGAR MEUS DADOS');
    await clicar(apagar());
    expect(gravacoes).toEqual([{ caminho: '/api/v1/dados/apagar', corpo: { confirmacao: 'APAGAR MEUS DADOS' } }]);
    expect(within(screen.getByRole('status')).getByText('Dados apagados. O app vai fechar.')).toBeTruthy();
    expect(recarregar).not.toHaveBeenCalled();
    expect((campo() as HTMLInputElement).value).toBe('');
  });

  it('falha parcial (completo:false) mostra o que ficou, só itens da lista', async () => {
    montar({ retorno: { apagado: ['banco', 'curriculos', 'backups', 'navegador'], faltou: ['chaves', 'C:\\caminho\\estranho'], completo: false, fechar: true } });
    digitar('APAGAR MEUS DADOS');
    await clicar(apagar());
    const status = screen.getByRole('status');
    expect(within(status).getByText('Parte dos dados foi apagada. O app vai fechar.')).toBeTruthy();
    expect(within(status).getByText('Não deu para apagar tudo. Ficou:')).toBeTruthy();
    expect(within(status).getAllByRole('listitem').map(li => li.textContent)).toEqual(['As chaves de IA guardadas no cofre do sistema']);
  });

  it('mostra antes tudo o que será apagado e o que não sai', () => {
    montar();
    expect(screen.getByText(/Não apaga: o login das ferramentas de IA .*, os PDFs originais que você copiou, os dados da Central antiga \(pasta dados\/\) e o cache da janela do app\./)).toBeTruthy();
    expect((campo() as HTMLInputElement).value).toBe(''); // nunca vem preenchido
    expect(screen.getByText('Antes, feche a CLI e o assistente (MCP) da Central')).toBeTruthy();
    for (const t of ['As vagas, a fila de candidaturas, o perfil, o histórico, os limites e as configurações', 'As chaves de IA guardadas no cofre do sistema', 'O login nas plataformas (navegador do app)'])
      expect(screen.getByText(t)).toBeTruthy();
  });

  it('409 (envio ou navegador em uso) e 403 aparecem em texto claro', async () => {
    montar({ erro: new ErroApi('envio em andamento', 409, { erro: 'envio em andamento', estado: 'enviando' }) });
    digitar('APAGAR MEUS DADOS');
    await clicar(apagar());
    expect(screen.getByRole('alert').textContent).toBe('Uma candidatura estava sendo enviada, então nada foi apagado. O envio fica parado até você abrir o app de novo. Feche e abra o app e repita.');
    cleanup();
    montar({ erro: new ErroApi('só pela tela', 403, { erro: 'só pela tela' }) });
    digitar('APAGAR MEUS DADOS');
    await clicar(apagar());
    expect(screen.getByRole('alert').textContent).toBe('Só você pode fazer isso, por esta tela.');
    cleanup();
    montar({ erro: new ErroApi('os dados foram apagados; o app está fechando', 503, { erro: 'os dados foram apagados; o app está fechando' }) });
    digitar('APAGAR MEUS DADOS');
    await clicar(apagar());
    expect(screen.getByRole('alert').textContent).toBe('Os dados já foram apagados. O app está fechando.');
  });
});

describe('Sair das plataformas', () => {
  it('409 diz para esperar o envio terminar; erro qualquer vira frase fixa, sem texto do servidor', async () => {
    montar({ erro: new ErroApi('envio em andamento', 409, { erro: 'envio em andamento', estado: 'enviando' }) });
    await clicar(screen.getByRole('button', { name: 'Sair das plataformas' }));
    expect(screen.getByRole('alert').textContent).toBe('O app está enviando uma candidatura agora. Espere terminar e tente de novo.');
    cleanup();
    montar({ erro: new Error('C:\\caminho\\interno: acesso negado') });
    await clicar(screen.getByRole('button', { name: 'Sair das plataformas' }));
    expect(screen.getByRole('alert').textContent).toBe('Não deu para concluir. Tente de novo.');
  });

  it('pede confirmação e manda {}', async () => {
    const { gravacoes, perguntas } = montar();
    await clicar(screen.getByRole('button', { name: 'Sair das plataformas' }));
    expect(perguntas[0].titulo).toBe('Sair das plataformas?');
    expect(gravacoes).toEqual([{ caminho: '/api/v1/navegador/sair', corpo: {} }]);
    expect(screen.getByRole('status').textContent).toBe('Pronto. O app saiu das plataformas.');
  });

  it('sem conseguir apagar o navegador (faltou), diz o que fazer em vez de dizer pronto', async () => {
    montar({ sair: { apagado: [], faltou: ['navegador'], completo: false, fechar: false } });
    await clicar(screen.getByRole('button', { name: 'Sair das plataformas' }));
    expect(screen.getByRole('alert').textContent).toBe('Não deu para sair das plataformas: o navegador do app está em uso. Feche o app, abra de novo e repita.');
    expect(screen.queryByRole('status')).toBeNull();
  });

  it('cancelar a confirmação não faz nada', async () => {
    const { gravacoes } = montar({ resposta: null });
    await clicar(screen.getByRole('button', { name: 'Sair das plataformas' }));
    expect(gravacoes).toEqual([]);
  });
});
