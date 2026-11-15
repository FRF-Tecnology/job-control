// "Respostas por e-mail" (contrato 0.17.0) com a API interceptada: sem conta, convite para conectar; ler manda o POST
// vazio, mostra o resumo em frases fixas e recarrega; recusas por motivo têm frase fixa; a recusa a confirmar manda o
// POST confirmar e o rótulo diz o que muda; sem vaga explica os botões, liga e ignora. Dados fictícios.
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, ErroApi, type App } from '../app/contexto';
import { RespostasEmail, resumoDaLeitura } from './RespostasEmail';

afterEach(cleanup);

type Chamada = { rota: string; metodo: string; corpo: unknown };
const vaga = { id: 'gp-1', empresa: 'Acme Pagamentos', titulo: 'Desenvolvedor Go', etapa: 'enviada', envio: { data: '2026-11-01' } };

async function montar(conta: unknown, emails: unknown[] = [], responder?: (c: Chamada) => unknown) {
  const chamadas: Chamada[] = [];
  const recarregar = vi.fn(async () => {});
  const agir = vi.fn(async (fn: () => Promise<unknown>) => { await fn(); return true; });
  const app = {
    estado: { vagas: [vaga], emails },
    recarregar, agir,
    api: async (rota: string, metodo = 'GET', corpo?: unknown) => {
      const c = { rota, metodo, corpo };
      chamadas.push(c);
      if (metodo === 'GET') return conta;
      const r = responder?.(c);
      if (r instanceof Error) throw r;
      return r;
    },
  } as unknown as App;
  await act(async () => { render(<Contexto.Provider value={app}><RespostasEmail /></Contexto.Provider>); });
  return { chamadas, recarregar };
}

const contagem = { lidos: 5, jaLidos: 0, sensiveis: 1, ignorados: 1, registrados: 3, mudaram: 1, aConfirmar: 1, semVaga: 1, pendentes: 0, falhas: 0, ultimaLeitura: '2026-11-10' };

describe('Respostas por e-mail', () => {
  it('sem e-mail conectado: convida a conectar e não mostra o botão', async () => {
    await montar({ conectada: false, usuario: '', servidor: '', ultimaLeitura: '' });
    expect(screen.getByRole('link', { name: 'Perfil e IA' }).getAttribute('href')).toBe('#/perfil');
    expect(screen.queryByRole('button', { name: 'Ler respostas do e-mail' })).toBeNull();
  });

  it('lê, mostra o resumo e recarrega', async () => {
    const { chamadas, recarregar } = await montar({ conectada: true, ultimaLeitura: '' }, [], () => contagem);
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Ler respostas do e-mail' })); });
    expect(chamadas[1]).toEqual({ rota: '/api/v1/email/ler', metodo: 'POST', corpo: {} });
    expect(screen.getByRole('status').textContent).toBe(resumoDaLeitura(contagem));
    expect(screen.getByRole('status').textContent).toMatch(/1 vaga mudou de etapa, 1 resposta para você confirmar, 1 resposta sem vaga/);
    expect(recarregar).toHaveBeenCalled();
  });

  it.each([
    ['sem-conta', 409, /Conecte seu e-mail/],
    ['sem-ia', 409, /Conecte uma IA/],
    ['senha', 409, /recusou a senha de app/],
    ['', 502, /Confira a internet/],
  ])('recusa %s: frase fixa, sem o texto do servidor', async (motivo, status, frase) => {
    await montar({ conectada: true, ultimaLeitura: '2026-11-09' }, [], () => new ErroApi('texto do servidor', status, { erro: 'texto do servidor', motivo }));
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Ler respostas do e-mail' })); });
    expect(screen.getByRole('alert').textContent).toMatch(frase);
    expect(document.body.textContent).not.toContain('texto do servidor');
  });

  it('recusa a confirmar: o botão diz o que muda e manda o confirmar', async () => {
    const email = { idMensagem: 'abc@example.com', status: 'a-confirmar', tipo: 'rejeicao', vagaId: 'gp-1', de: 'no-reply@gupy.io', assunto: 'Atualização', resumo: 'Não seguiram', proposta: { etapa: 'encerrada' } };
    const { chamadas } = await montar({ conectada: true, ultimaLeitura: '' }, [email], () => ({}));
    expect(screen.getByText('Acme Pagamentos — Desenvolvedor Go')).toBeTruthy();
    expect(screen.getByText('Recusa')).toBeTruthy();
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Encerrar a vaga' })); });
    expect(chamadas.at(-1)).toEqual({ rota: '/api/v1/emails/abc%40example.com/confirmar', metodo: 'POST', corpo: {} });
    expect(screen.getByText(/Nada muda até você confirmar/)).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Não mudar a vaga' })).toBeTruthy();
  });

  it('sem vaga: explica os botões, liga só depois de escolher e ignora sem mexer na vaga', async () => {
    const email = { idMensagem: 'sv@example.com', status: 'sem-vaga', tipo: 'entrevista', de: 'rh@empresa.example', assunto: 'Convite', resumo: '' };
    const { chamadas } = await montar({ conectada: true, ultimaLeitura: '2026-11-10' }, [email], () => ({}));
    expect(screen.getByText(/não achei de qual vaga/)).toBeTruthy();
    expect(screen.getByText(/não voltam nas próximas leituras/)).toBeTruthy();
    const ligar = screen.getByRole('button', { name: 'Ligar à vaga' }) as HTMLButtonElement;
    expect(ligar.disabled).toBe(true);
    fireEvent.change(screen.getByLabelText('Vaga do e-mail "Convite"'), { target: { value: 'gp-1' } });
    await act(async () => { fireEvent.click(ligar); });
    expect(chamadas.at(-1)).toEqual({ rota: '/api/v1/emails/sv%40example.com/ligar', metodo: 'POST', corpo: { vagaId: 'gp-1' } });
    await act(async () => { fireEvent.click(screen.getByRole('button', { name: 'Ignorar e-mail' })); });
    expect(chamadas.at(-1)).toEqual({ rota: '/api/v1/emails/sv%40example.com/descartar', metodo: 'POST', corpo: {} });
  });
});
