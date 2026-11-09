// Painel de envio quando nada está sendo enviado (jc-a2y.6.10, pedido do Vini): diz o que vai sair a seguir ou o que
// fazer para haver o que enviar, com as contagens da fila. Só tela. Dados fictícios.
import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, type App } from '../app/contexto';
import { PainelEnvio, type EstadoEnvio, type ResumoFila } from './PainelEnvio';

afterEach(cleanup);

const BASE: EstadoEnvio = { ligado: true, pausado: false, enviandoAgora: null, desfechos: [], janela: 'oculta', login: false };
const NOMES: Record<string, string> = { 'cand-a': 'Empresa A — Dev React', 'cand-b': 'Empresa B — Dev Go' };

function montar(estado: EstadoEnvio, fila: ResumoFila) {
  const app = { estado: { vagas: [] }, aviso: () => {}, perguntar: async () => 'ok', api: vi.fn(async () => estado) } as unknown as App;
  render(<Contexto.Provider value={app}><PainelEnvio nome={id => NOMES[id] ?? 'uma candidatura'} motivo={() => 'Outro'} fila={fila} /></Contexto.Provider>);
}

describe('Painel de envio: o que vem a seguir', () => {
  it('nada aprovado: diz o que fazer, com as contagens', async () => {
    montar(BASE, { aprovadas: [], paraRevisar: 2, prontas: 1 });
    expect(await screen.findByText('Nada para enviar agora: prepare e aprove uma candidatura abaixo.')).toBeTruthy();
    expect(screen.getByText('1 vaga pronta para preparar · 2 candidaturas para revisar · 0 aprovadas')).toBeTruthy();
  });

  it('com aprovadas: a próxima é a de aprovação mais antiga (a ordem do envio)', async () => {
    montar(BASE, { aprovadas: [{ id: 'cand-b', em: '2026-11-09T11:00:00-03:00' }, { id: 'cand-a', em: '2026-11-09T10:00:00-03:00' }], paraRevisar: 0, prontas: 0 });
    expect(await screen.findByText('Próxima a enviar: Empresa A — Dev React')).toBeTruthy();
    expect(screen.getByText('0 vagas prontas para preparar · 0 candidaturas para revisar · 2 aprovadas')).toBeTruthy();
  });

  it('pausado com aprovada: diz que ela sai quando você retomar', async () => {
    montar({ ...BASE, pausado: true }, { aprovadas: [{ id: 'cand-a', em: '2026-11-09T10:00:00-03:00' }], paraRevisar: 0, prontas: 0 });
    expect(await screen.findByText('Próxima a enviar quando você retomar: Empresa A — Dev React')).toBeTruthy();
  });

  it('enviando agora: mostra o envio em curso, não a próxima', async () => {
    montar({ ...BASE, enviandoAgora: { id: 'cand-b', desde: '2026-11-09T10:10:00-03:00' } }, { aprovadas: [{ id: 'cand-a', em: '2026-11-09T10:00:00-03:00' }], paraRevisar: 0, prontas: 0 });
    await screen.findByText(/^Enviando agora: Empresa B — Dev Go/);
    expect(screen.queryByText(/Próxima a enviar/)).toBeNull();
    expect(screen.queryByText(/Nada para enviar agora/)).toBeNull();
  });
});
