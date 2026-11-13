import { useState } from 'react';
import { act, cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Contexto, type App } from '../app/contexto';
import { TelaRadar } from './Radar';
import { intervaloDaBusca } from './BuscaDoPerfil';

const abrir = vi.hoisted(() => vi.fn());
vi.mock('@wailsio/runtime', () => ({ Browser: { OpenURL: abrir } }));

beforeEach(() => vi.setSystemTime(new Date('2026-11-12T12:00:00Z')));
afterEach(() => { cleanup(); vi.useRealTimers(); vi.unstubAllGlobals(); vi.clearAllMocks(); });

const perfil = { cargo: 'Java Engineer', filtros: { niveis: ['Pleno'], modelo: ['Remoto'], pularSe: ['Presencial'] },
  stackPrincipal: ['Java'], curriculos: { PT: 'curriculos/CV-pt.pdf' }, faixaSalarial: { moeda: 'USD', min: 4000, max: 6000 } };
const vaga = (id: string, grupo: string) => ({ id, empresa: 'Empresa ' + id, titulo: 'Engenharia ' + id,
  etapa: 'descoberta', aderencia: grupo, radarClasse: grupo === 'A' ? 'forte' : grupo === 'B' ? 'conferir' : 'excluir',
  radarVersao: 1, publicadaEm: '2026-11-11', radarMotivo: 'O anúncio pede APIs Java, habilidade do perfil.',
  triagemDecisao: 'recomendar', fonteBusca: 'Fonte QA', link: 'https://example.org/jobs/' + id });
function montar(vagas: any[], api = vi.fn(), estadoExtra = {}) {
  function Banco() {
    const [, atualizar] = useState(0);
    const app = { estado: { perfil, vagas, rodadas: [], ...estadoExtra }, api,
      mover: async (id: string, etapa: string, extras?: object) => {
        Object.assign(vagas.find(v => v.id === id), extras, { etapa });
        atualizar(n => n + 1);
        return true;
      }, recarregar: async () => atualizar(n => n + 1) } as unknown as App;
    return <Contexto.Provider value={app}><TelaRadar /></Contexto.Provider>;
  }
  return render(<Banco />);
}

describe('Radar de vagas', () => {
  it('guarda no histórico vagas antigas sem fonte, nota ou carimbo atual, isolando o outro perfil', () => {
    montar([{ id: 'legado', empresa: 'Oficina', titulo: 'Desenvolvimento', etapa: 'descoberta' },
      {...vaga('antiga', 'A'),radarVersao:undefined},
      { ...vaga('privada', 'A'), perfilBuscaId: 'outro' }]);
    expect(screen.queryByRole('article')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name:'Histórico' }));
    expect(screen.getByRole('button', { name: /Oficina Desenvolvimento/ })).toBeTruthy();
    expect(screen.getByRole('button',{name:/Empresa antiga/})).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: /Oficina Desenvolvimento/ }));
    expect(screen.getByRole('article', { name: 'Desenvolvimento' })).toBeTruthy();
    expect(screen.queryByText('Nenhuma vaga encontrada')).toBeNull();
    expect(screen.queryByText('Empresa privada')).toBeNull();
  });
  it('mostra valores, periodicidade e requisitos do anúncio salvo e usa a marca local quando conhecida', () => {
    montar([{ ...vaga('hf', 'A'), empresa: 'Hugging Face', periodoSalario: 'hourly', moeda: 'USD',
      salarioMin: 42, salarioMax: 48, stack: ['Python'], descricao: 'Desenvolver serviços de inferência.' },
      { ...vaga('outra', 'B'), empresa: 'Oficina' }]);
    const detalhe = within(screen.getByRole('article', { name: 'Engenharia hf' }));
    expect(detalhe.getByText('Salário da vaga')).toBeTruthy();
    expect(detalhe.getByText('US$ 42 a US$ 48 por hora')).toBeTruthy();
    expect(detalhe.getByText('Python')).toBeTruthy();
    expect(detalhe.getByText('Desenvolver serviços de inferência.')).toBeTruthy();
    const linhaHF = screen.getByRole('button', { name: /Hugging Face Engenharia hf/ });
    const logo = linhaHF.querySelector('img')!;
    expect(logo.getAttribute('src')).toContain('huggingface');
    fireEvent.error(logo);
    expect(within(linhaHF).getByText('HF')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name:'A conferir' }));
    expect(screen.getByRole('button', { name: /Oficina Engenharia outra/ }).querySelector('img')).toBeNull();
  });
  it('abre com os critérios salvos e pede a primeira busca só ao clicar', async () => {
    const api = vi.fn().mockResolvedValue({ encontradas: 0, novas: 0, grupos: { A: 0, B: 0, C: 0 }, avisos: [], pendentes: 0, puladas: 0 });
    montar([], api);
    expect(screen.getByText('Java Engineer')).toBeTruthy();
    expect(screen.getByText(/Remoto/)).toBeTruthy();
    expect(screen.getByRole('link', { name: 'Ajustar preferências' }).getAttribute('href')).toBe('#/perfil');
    expect(screen.getByRole('heading', { name: 'Nenhuma vaga encontrada' })).toBeTruthy();
    expect(screen.queryByRole('article')).toBeNull();
    expect(api).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Buscar vagas' }));
    expect(await screen.findByText('Resultado da busca')).toBeTruthy();
    expect(api).toHaveBeenCalledWith('/api/v1/busca', 'POST', { perfilBuscaId: 'original' });
  });

  it('oculta no perfil ativo um anúncio já adicionado à lista de outro perfil', () => {
    montar([{ ...vaga('legado', 'B'), etapa: 'aplicar' }, { ...vaga('copia', 'A'), anuncioId: 'legado', perfilBuscaId: 'java' },
      { ...vaga('nova', 'C'), perfilBuscaId: 'java' }], vi.fn(), { perfilBuscaAtivo: 'java' });
    expect(screen.queryByText('Engenharia copia')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name:'Histórico' }));
    expect(screen.queryByText('Engenharia legado')).toBeNull();
    expect(screen.getByRole('article', { name: 'Engenharia nova' })).toBeTruthy();
  });

  it('ordena sem duplicatas e abre um detalhe por vez ao selecionar ou filtrar', () => {
    const b = vaga('b', 'B');
    montar([vaga('c', 'C'), b, vaga('a', 'A'), b,
      { ...vaga('excluida', ''), etapa: 'pulada', triagemBusca: 'pulada', motivo: 'Fora do modelo de trabalho.' }]);
    const lista = screen.getByRole('list', { name: 'Vagas para revisão' });
    const linhas = within(lista).getAllByRole('button');
    expect(linhas.map(el => within(el).getByText(/Engenharia/).textContent)).toEqual(['Engenharia a']);
    expect(screen.getAllByRole('article')).toHaveLength(1);
    expect(screen.getByRole('article', { name: 'Engenharia a' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name:'A conferir' }));
    expect(within(lista).getAllByRole('button')).toHaveLength(1);
    fireEvent.click(within(lista).getByRole('button',{name:/Empresa b/}));
    expect(screen.getByRole('article', { name: 'Engenharia b' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Histórico' }));
    expect(within(lista).getAllByRole('button')).toHaveLength(1);
    expect(screen.getByRole('article', { name: 'Engenharia c' })).toBeTruthy();
  });

  it('atualiza o detalhe por teclado, adiciona à lista, pula e desfaz na última vaga', async () => {
    montar([vaga('a', 'A'), vaga('b', 'A')]);
    const primeira = screen.getByRole('button', { name: /Empresa a Engenharia a/ });
    fireEvent.keyDown(primeira, { key: 'ArrowDown' });
    expect(screen.getByRole('article', { name: 'Engenharia b' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Adicionar à lista' }));
    await screen.findByRole('article', { name: 'Engenharia a' });
    fireEvent.click(screen.getByRole('button', { name: 'Pular' }));
    await screen.findByRole('heading', { name: 'Você revisou todas as vagas' });
    expect(screen.getByRole('link', { name: 'Ver lista' }).getAttribute('href')).toBe('#/lista');
    fireEvent.click(await screen.findByRole('button', { name: 'Desfazer' }));
    await screen.findByRole('article', { name: 'Engenharia a' });
    fireEvent.click(screen.getByRole('button', { name: 'Histórico' }));
    expect(screen.getByRole('heading', { name: 'Nenhuma vaga no histórico' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Recomendadas' }));
    expect(screen.getByRole('article', { name: 'Engenharia a' })).toBeTruthy();
  });

  it('distingue busca em andamento e interrompida e mantém acesso a vagas anteriores', async () => {
    const api = vi.fn().mockRejectedValueOnce(new Error('Fonte indisponível')).mockResolvedValueOnce({
      encontradas: 0, novas: 0, grupos: { A: 0, B: 0, C: 0 }, avisos: [], pendentes: 0, puladas: 0,
    });
    montar([vaga('salva', 'A')], api);
    fireEvent.click(screen.getByRole('button', { name: 'Buscar vagas' }));
    expect(screen.getByText('Buscando vagas para o seu perfil…')).toBeTruthy();
    await screen.findByRole('heading', { name: 'A busca foi interrompida' });
    expect(screen.getByText('Fonte indisponível')).toBeTruthy();
    expect(screen.getByRole('article', { name: 'Engenharia salva' })).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Tentar novamente' }));
    await screen.findByText('Resultado da busca');
    expect(screen.queryByText('A busca foi interrompida')).toBeNull();
  });

  it('mostra as vagas já avaliadas enquanto a busca continua', async () => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
    let terminar: (b: unknown) => void = () => {};
    const api = vi.fn(() => new Promise(fim => { terminar = fim; }));
    const vagas: any[] = [];
    montar(vagas, api);
    fireEvent.click(screen.getByRole('button', { name: 'Buscar vagas' }));
    expect(screen.getByText('Buscando vagas para o seu perfil…')).toBeTruthy();
    vagas.push(vaga('nova', 'A')); // a triagem gravou a primeira vaga e a busca continua
    await act(async () => { vi.advanceTimersByTime(intervaloDaBusca); });
    expect(screen.getByRole('button', { name: /Empresa nova Engenharia nova/ })).toBeTruthy();
    expect(screen.getByText('As vagas já avaliadas aparecem abaixo enquanto a busca continua.')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Buscando…' })).toBeTruthy();
    await act(async () => { terminar({ encontradas: 1, novas: 1, grupos: { A: 1, B: 0, C: 0 }, avisos: [], pendentes: 0, puladas: 0 }); });
    expect(screen.queryByText('Buscando vagas para o seu perfil…')).toBeNull();
    expect(screen.getByRole('button', { name: /Empresa nova Engenharia nova/ })).toBeTruthy();
  });
  it('mostra o mercado como "vagas do Brasil", sem parecer o país que falta no perfil', () => {
    montar([], vi.fn(), { perfil: { ...perfil, foco: 'nacional' } });
    expect(screen.getByText(/Java Engineer · Remoto · Vagas do Brasil/)).toBeTruthy();
    expect(screen.getByText('Informe de qual país você vai trabalhar.')).toBeTruthy();
  });
  it('abre um anúncio sem registrar envio e retoma a intenção salva depois de reabrir', async () => {
    const v = vaga('a', 'A');
    const vagas = [v];
    const destino = 'https://example.org/candidaturas/a';
    const api = vi.fn(async (url: string) => {
      if (url.endsWith('/iniciar-manual')) Object.assign(v, { aplicacaoManual: true, etapa: 'aplicar', urlCandidaturaManual: destino });
      if (url.endsWith('/confirmar-envio-manual')) Object.assign(v, { etapa: 'enviada', envio: { data: '2026-11-12' } });
    });
    vi.stubGlobal('chrome', { webview: { postMessage: vi.fn() } });
    abrir.mockResolvedValue(undefined);
    const primeira = montar(vagas, api);
    fireEvent.click(screen.getByRole('link', { name: 'Ler anúncio' }));
    await vi.waitFor(() => expect(abrir).toHaveBeenCalledWith(v.link));
    expect(api).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Aplicar na vaga' }));
    await screen.findByText('Aguardando você');
    fireEvent.click(screen.getByRole('link', { name: 'Abrir candidatura no navegador' }));
    await vi.waitFor(() => expect(abrir).toHaveBeenCalledTimes(2));
    expect(abrir).toHaveBeenLastCalledWith(destino);
    expect(api.mock.calls).toEqual([['/api/v1/vagas/a/iniciar-manual', 'POST', {}]]);
    fireEvent.click(screen.getByRole('button', { name: 'Continuar depois' }));
    expect(screen.queryByRole('button', { name: 'Já enviei' })).toBeNull();
    primeira.unmount();
    montar(vagas, api);
    expect(screen.getByText('Aguardando você')).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: 'Já enviei' }));
    expect(await screen.findByText('Candidatura enviada')).toBeTruthy();
    expect(api.mock.calls[1]).toEqual(['/api/v1/vagas/a/confirmar-envio-manual', 'POST', {}]);
    expect(screen.queryByRole('button', { name: 'Já enviei' })).toBeNull();
  });

  it('mantém a vaga pendente quando o navegador ou a confirmação falha', async () => {
    vi.stubGlobal('chrome', { webview: { postMessage: vi.fn() } });
    abrir.mockRejectedValue(new Error('indisponível'));
    const api = vi.fn().mockRejectedValue(new Error('Falha ao salvar no banco.'));
    montar([{ ...vaga('b', 'B'), etapa: 'aplicar', aplicacaoManual: true }], api);
    fireEvent.click(screen.getByRole('button', { name:'A conferir' }));
    fireEvent.click(screen.getByRole('link', { name: 'Abrir candidatura no navegador' }));
    expect((await screen.findByRole('alert')).textContent).toMatch(/navegador/i);
    expect(api).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Já enviei' }));
    expect(await screen.findByText('Falha ao salvar no banco.')).toBeTruthy();
    expect(screen.queryByText('Candidatura enviada')).toBeNull();
    expect(screen.getByRole('button', { name: 'Já enviei' })).toBeTruthy();
  });

  it('fora do desktop oferece um link real e nunca abre protocolo inválido', () => {
    montar([{ ...vaga('ok', 'A'), aplicacaoManual: true, etapa: 'aplicar' }, { ...vaga('ruim', 'B'), link: 'file:///segredo' }]);
    const a = screen.getByRole('link', { name: 'Abrir candidatura no navegador' });
    expect(a.getAttribute('href')).toBe('https://example.org/jobs/ok');
    expect(a.getAttribute('target')).toBe('_blank');
    fireEvent.click(screen.getByRole('button', { name:'A conferir' }));
    fireEvent.click(screen.getByRole('button', { name: /Empresa ruim Engenharia ruim/ }));
    const ruim = within(screen.getByRole('article', { name: 'Engenharia ruim' }));
    expect(ruim.queryByRole('link')).toBeNull();
    expect(ruim.queryByRole('button', { name: 'Aplicar na vaga' })).toBeNull();
  });
  it('seleciona até dez fortes válidas ordenadas pela publicação, ignorando a data da coleta', () => {
    const fortes = Array.from({length:12}, (_,i) => ({...vaga('forte-'+i,'A'),publicadaEm:'2026-11-'+String(i+1).padStart(2,'0')}));
    montar([...fortes,{...vaga('pular','A'),triagemDecisao:'pular'}, {...vaga('incompleta','A'),radarClasse:'conferir'},
      {...vaga('sem-data','A'),publicadaEm:undefined,descobertaEm:'2026-11-12'},
      {...vaga('vencida','A'),expiraEm:'2026-11-12T10:59:00Z'},
      {...vaga('velha','A'),publicadaEm:'2026-10-12',descobertaEm:'2026-11-12'},
      {...vaga('pendente','A'),triagemBusca:'pendente'}]);
    const linhas = () => within(screen.getByRole('list',{name:'Vagas para revisão'})).getAllByRole('button');
    expect(linhas().map(el=>within(el).getByText(/Engenharia/).textContent)).toEqual(
      ['11','10','9','8','7','6','5','4','3','2'].map(id=>'Engenharia forte-'+id));
    expect(screen.getByText('O anúncio pede APIs Java, habilidade do perfil.')).toBeTruthy();
    expect(screen.getByText(/Publicada em 12 nov/)).toBeTruthy();
    fireEvent.click(screen.getByRole('button',{name:'A conferir'}));
    expect(linhas().map(el=>within(el).getByText(/Engenharia/).textContent).sort()).toEqual(['Engenharia incompleta','Engenharia pendente','Engenharia sem-data']);
  });
  it('mostra a pretensão do perfil ativo e dá prioridade ao salário original, sem converter anual em mensal', () => {
    const primeira = montar([vaga('sem','A')]);
    expect(screen.getByText('Sua pretensão')).toBeTruthy();
    expect(screen.getByText('US$ 4.000 a US$ 6.000 por mês')).toBeTruthy();
    primeira.unmount();
    const segunda = montar([{...vaga('sem','A'),perfilBuscaId:'dados'}],vi.fn(),{perfilBuscaAtivo:'dados',perfil:{faixaSalarial:{moeda:'BRL',min:7000}}});
    expect(screen.getByText('A partir de R$ 7.000 por mês')).toBeTruthy();
    segunda.unmount();
    const terceira=montar([{...vaga('com','A'),salarioMin:48000,moeda:'EUR',periodoSalario:'annual',salarioUSDMin:4000}]);
    expect(screen.getByText('A partir de € 48.000 por ano')).toBeTruthy();
    expect(screen.queryByText('Sua pretensão')).toBeNull();
    terceira.unmount();
    montar([vaga('sem','A')],vi.fn(),{perfil:{}});
    expect(screen.getByText('Salário não informado')).toBeTruthy();
    expect(screen.queryByText('Sua pretensão')).toBeNull();
  });
  it('indica o único ajuste necessário quando o cargo do perfil é o nome da pessoa',()=>{
    montar([],vi.fn(),{perfil:{...perfil,cargo:'ANA silva',nome:'Ana Silva',paisTrabalho:'BR'}});
    expect(screen.getByText(/cargo.*seu nome/i)).toBeTruthy();
    expect(screen.queryByText(/informe.*país/i)).toBeNull();
  });
  it('mantém a remuneração anunciada em texto quando a fonte não oferece limites numéricos',()=>{
    montar([{...vaga('texto','A'),salario:'CAD 32–40/hora'}]);
    expect(screen.getByText('Salário da vaga')).toBeTruthy();
    expect(screen.getByText('CAD 32–40/hora')).toBeTruthy();
    expect(screen.queryByText('Sua pretensão')).toBeNull();
  });
  it('mostra a faixa USD mensal de anúncios históricos como oferta da vaga',()=>{
    montar([{...vaga('legada','A'), salarioUSDMin:5100,salarioUSDMax:6800}]);
    expect(screen.getByText('Salário da vaga')).toBeTruthy();
    expect(screen.getByText('US$ 5.100 a US$ 6.800 por mês')).toBeTruthy();
    expect(screen.queryByText('Sua pretensão')).toBeNull();
  });
  it('informa quando a cobertura é parcial, mesmo com uma recomendação já disponível',async()=>{
    const api=vi.fn().mockResolvedValue({encontradas:1,novas:1,grupos:{A:1,B:0,C:0},puladas:0,pendentes:0,avisos:['Fonte indisponível: horizonte.']});
    montar([vaga('salva','A')],api);
    fireEvent.click(screen.getByRole('button',{name:'Buscar vagas'}));
    await screen.findByText('Busca parcial');
    expect(screen.getByText('Fonte indisponível: horizonte.')).toBeTruthy();
    expect(screen.getByRole('article',{name:'Engenharia salva'})).toBeTruthy();
  });
  it('distingue uma busca completa sem vagas compatíveis da primeira abertura',async()=>{
    const api=vi.fn().mockResolvedValue({encontradas:0,novas:0,grupos:{A:0,B:0,C:0},puladas:0,pendentes:0,avisos:[]});
    montar([],api);
    fireEvent.click(screen.getByRole('button',{name:'Buscar vagas'}));
    await screen.findByRole('heading',{name:'Nenhuma vaga compatível nesta busca'});
  });
});
