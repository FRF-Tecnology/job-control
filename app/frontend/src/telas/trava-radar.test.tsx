// Trava nas ações de escrita das telas da reescrita (Radar e seletor de perfil): com a trava ligada, buscar, adicionar,
// pular e criar perfil ficam desabilitados com a frase da faixa; filtrar e abrir vagas continuam. Só dados fictícios.
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, type App } from '../app/contexto';
import type { Situacao } from '../app/conta';
import { TelaRadar } from './Radar';

vi.mock('@wailsio/runtime', () => ({ Browser: { OpenURL: vi.fn() } }));
afterEach(() => { cleanup(); vi.clearAllMocks(); });

const FRASE = 'Seu teste acabou. Você ainda vê tudo; para buscar e se candidatar, assine.';
const travada: Situacao = { ativa: true, entrada: true, email: 'ana@exemplo.com', estado: 'bloqueado', motivo: 'teste-acabou' };
const perfil = { cargo: 'Java Engineer', paisTrabalho: 'BR', filtros: { niveis: ['Pleno'], modelo: ['Remoto'] }, faixaSalarial: { moeda: 'USD', min: 4000 } };
const vaga = { id: 'v1', empresa: 'Empresa Um', titulo: 'Engenharia Java', etapa: 'descoberta', aderencia: 'A', radarClasse: 'forte',
  radarVersao: 1, publicadaEm: '2026-11-11', triagemDecisao: 'recomendar', fonteBusca: 'Fonte QA', link: 'https://example.org/jobs/v1' };

function montar(podeAgir: boolean) {
  const api = vi.fn(async () => ({}));
  const mover = vi.fn(async () => true);
  const app = { estado: { perfil, vagas: [{ ...vaga }], rodadas: [] }, api, mover, recarregar: async () => {}, perguntar: async () => null,
    podeAgir, conta: podeAgir ? { ...travada, estado: 'teste', motivo: '' } : travada } as unknown as App;
  render(<Contexto.Provider value={app}><TelaRadar /></Contexto.Provider>);
  return { api, mover };
}

describe('Radar com a trava ligada', () => {
  it('buscar, adicionar, pular e criar perfil travam; filtros e abrir a vaga continuam', () => {
    const { api, mover } = montar(false);
    for (const nome of [/Buscar vagas/, /Adicionar à lista/, /^Pular/, /Criar perfil/, /Aplicar na vaga/]) {
      const botao = screen.getByRole('button', { name: nome });
      expect(botao).toHaveProperty('disabled', true);
      expect(botao.getAttribute('title')).toBe(FRASE);
    }
    expect(screen.getByLabelText('Perfil de busca')).toHaveProperty('disabled', true);
    fireEvent.click(screen.getByRole('button', { name: 'Histórico' }));
    expect(screen.getByRole('button', { name: 'Histórico' }).getAttribute('aria-pressed')).toBe('true');
    expect(api).not.toHaveBeenCalled();
    expect(mover).not.toHaveBeenCalled();
  });
  it('liberado, os mesmos botões ficam ativos', () => {
    montar(true);
    for (const nome of [/Buscar vagas/, /Adicionar à lista/, /^Pular/, /Criar perfil/]) {
      expect(screen.getByRole('button', { name: nome })).toHaveProperty('disabled', false);
    }
  });
});
