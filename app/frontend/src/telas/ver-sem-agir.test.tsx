// "Ver, sem agir" com a trava ligada: o foco é filtro de ver (A16, muda só na tela, sem PATCH), o Enter da caixa de
// pedido segue indo para as telas, e a nota da vaga fica como rascunho até o acesso voltar (A17). Só dados fictícios.
import { act, cleanup, fireEvent, render, renderHook, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { Contexto, type App } from '../app/contexto';
import { useFoco, type Situacao } from '../app/conta';
import { mercadoDe } from '../dominio/modelo';
import { CaixaPedido, TelaHoje } from './Hoje';
import { TelaTriagem } from './Triagem';
import { Detalhe } from './Detalhe';

afterEach(() => { cleanup(); location.hash = ''; sessionStorage.clear(); });

const FRASE = 'Seu teste acabou. Você ainda vê tudo; para buscar e se candidatar, assine.';
const travada: Situacao = { ativa: true, entrada: true, email: 'ana@exemplo.com', estado: 'bloqueado', motivo: 'teste-acabou' };
const VAGA = { id: 'a1', aderencia: 'A', etapa: 'descoberta', empresa: 'Empresa Um', titulo: 'Pessoa Dev', plataforma: 'linkedin', descobertaEm: '2026-11-10', notas: 'antes' };

function appFalso(podeAgir: boolean, extra: Partial<App> = {}): App {
  return {
    estado: { vagas: [VAGA], eventos: [], plataformas: [], perfil: { nome: 'Ana' }, curriculos: [] }, hoje: '2026-11-11', foco: 'ambos', rota: 'hoje', vaga: null,
    mudarFoco: vi.fn(async () => {}), api: vi.fn(async () => ({})), agir: async fn => { await fn(); return true; }, mover: vi.fn(async () => true),
    abrirVaga: () => {}, navegar: () => {}, aviso: vi.fn(), perguntar: async () => null, recarregar: async () => {}, limpar: () => {},
    podeAgir, conta: podeAgir ? { ...travada, estado: 'teste', motivo: '' } : travada, releConta: async () => {}, ...extra,
  };
}

describe('Foco com a trava ligada (A16)', () => {
  it('useFoco: travado, muda só na tela; liberado, salva e volta ao salvo', async () => {
    const salvar = vi.fn(async () => {});
    const { result, rerender } = renderHook(({ salvo, liberado }) => useFoco(salvo, liberado, salvar), { initialProps: { salvo: 'ambos', liberado: false } });
    await act(async () => { await result.current.mudarFoco('nacional'); });
    expect(result.current.foco).toBe('nacional');
    expect(salvar).not.toHaveBeenCalled();
    rerender({ salvo: 'ambos', liberado: true }); // o acesso voltou: vale o foco salvo
    expect(result.current.foco).toBe('ambos');
    await act(async () => { await result.current.mudarFoco('internacional'); });
    expect(salvar).toHaveBeenCalledWith('internacional');
  });
  it('Hoje: o seletor de foco continua clicável com a trava', async () => {
    const app = appFalso(false);
    render(<Contexto.Provider value={app}><TelaHoje /></Contexto.Provider>);
    const nacional = screen.getAllByRole('button').find(b => b.closest('[aria-label="Foco da busca"]') && /nacional/i.test(b.textContent ?? '') && !/inter/i.test(b.textContent ?? '')) as HTMLButtonElement;
    expect(nacional.disabled).toBe(false);
    await act(async () => { fireEvent.click(nacional); });
    expect(app.mudarFoco).toHaveBeenCalled();
  });
  it('Triagem: "Ver todas" continua clicável com a trava', async () => {
    const fora = mercadoDe(VAGA) === 'nacional' ? 'internacional' : 'nacional';
    const app = appFalso(false, { foco: fora, rota: 'triagem' });
    render(<Contexto.Provider value={app}><TelaTriagem /></Contexto.Provider>);
    const ver = screen.getByRole('button', { name: 'Ver todas' }) as HTMLButtonElement;
    expect(ver.disabled).toBe(false);
    await act(async () => { fireEvent.click(ver); });
    expect(app.mudarFoco).toHaveBeenCalledWith('ambos');
  });
});

describe('Enter na caixa de pedido com a trava', () => {
  const caixa = () => screen.getByRole('textbox', { name: 'O que vamos fazer hoje?' }) as HTMLInputElement;
  it('atalho de navegação vai para a tela; pedido de escrita só avisa', async () => {
    const app = appFalso(false);
    render(<Contexto.Provider value={app}><CaixaPedido sugestoes={[]} iaConectada={false} aoCriar={() => {}} /></Contexto.Provider>);
    fireEvent.change(caixa(), { target: { value: 'abrir a triagem' } });
    await act(async () => { fireEvent.keyDown(caixa(), { key: 'Enter' }); });
    expect(location.hash).toBe('#/triagem');
    fireEvent.change(caixa(), { target: { value: 'Adiciona a vaga https://vagas.exemplo.com/1' } });
    await act(async () => { fireEvent.keyDown(caixa(), { key: 'Enter' }); });
    expect(app.aviso).toHaveBeenCalledWith(FRASE, true);
    expect(app.api).not.toHaveBeenCalled();
  });
});

describe('Nota da vaga com a trava (A17)', () => {
  const notas = () => document.getElementById('notas-vaga') as HTMLTextAreaElement;
  it('guarda como rascunho e salva sozinha quando o acesso volta', async () => {
    const app = appFalso(false);
    const r = render(<Contexto.Provider value={app}><Detalhe id="a1" fechar={() => {}} /></Contexto.Provider>);
    expect(notas().readOnly).toBe(false);
    expect(screen.getByText('Sua nota fica guardada aqui e é salva quando o acesso voltar')).toBeTruthy();
    fireEvent.change(notas(), { target: { value: 'recrutadora: Bia' } });
    fireEvent.blur(notas());
    expect(app.api).not.toHaveBeenCalled();
    expect(sessionStorage.getItem('applyer:nota:a1')).toBe('recrutadora: Bia');
    // Fechar e abrir de novo: o rascunho volta.
    r.unmount();
    const r2 = render(<Contexto.Provider value={app}><Detalhe id="a1" fechar={() => {}} /></Contexto.Provider>);
    expect(notas().value).toBe('recrutadora: Bia');
    const liberado = { ...app, podeAgir: true, conta: { ...travada, estado: 'teste', motivo: '' } };
    await act(async () => { r2.rerender(<Contexto.Provider value={liberado}><Detalhe id="a1" fechar={() => {}} /></Contexto.Provider>); });
    expect(app.api).toHaveBeenCalledWith('/api/v1/vagas/a1', 'PATCH', { notas: 'recrutadora: Bia', notasAntes: 'antes' });
    expect(sessionStorage.getItem('applyer:nota:a1')).toBeNull();
    expect(screen.queryByText('Sua nota fica guardada aqui e é salva quando o acesso voltar')).toBeNull();
  });
});
