import { useLayoutEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { useApp } from '../app/contexto';
import { IconeUI } from '../app/IconeUI';
import { LinkExterno } from '../componentes';
import { foiEnviada } from '../dominio/modelo';
import { aceitaTrabalhoLocal, cargoIgualAoNome, nomeDoPais } from '../dominio/localDeTrabalho';
import { dataCurta, encode, plural, safeHref } from '../util';
import { BalancoDaBusca, useBuscaDoPerfil } from './BuscaDoPerfil';
import { SeletorPerfil } from './SeletorPerfil';
import radarArte from '../arte/radar.svg';
import { MarcaEmpresa } from '../app/MarcaEmpresa';
import '../estilo/radar.css';

type Vaga = {
  id: string; anuncioId?: string; perfilBuscaId?: string; titulo: string; empresa: string; etapa: string; link?: string;
  local?: string; modelo?: string; nivel?: string; contrato?: string; descricao?: string; categorias?: string[]; stack?: string[];
  periodoSalario?: string; moeda?: string; salarioMin?: number; salarioMax?: number; salario?: string;
  salarioUSDMensal?: number; salarioUSDMin?: number; salarioUSDMax?: number;
  aderencia?: string; motivoClaude?: string; triagemMotivoPulo?: string; triagemDecisao?: string; triagemConferir?: string[];
  triagemBusca?: string; fonteBusca?: string; fonteURL?: string; descobertaEm?: string;
  radarVersao?: number; radarClasse?: string; radarMotivo?: string; publicadaEm?: string; expiraEm?: string; aberta?: boolean;
  eventoPuloBusca?: string;
  aplicacaoManual?: boolean; urlCandidaturaManual?: string; envio?: { data?: string };
};
const notas = ['A', 'B', 'C'] as const;
type Nota = typeof notas[number];
const filtros = ['Recomendadas', 'A conferir', 'Histórico'] as const;
type Filtro = typeof filtros[number];
const afinidade: Record<Nota, string> = { A: 'Alta compatibilidade', B: 'Compatibilidade média', C: 'Baixa compatibilidade' };
// No resumo do perfil, o mercado aparece como "vagas de…": sozinho, "Brasil" parecia o país de trabalho da pessoa,
// logo acima do aviso pedindo esse país.
const mercados: Record<string, string> = { nacional: 'Vagas do Brasil', internacional: 'Vagas do exterior', ambos: 'Vagas do Brasil e do exterior' };
const temNota = (nota?: string): nota is Nota => notas.includes(nota as Nota);
const ordem = (v: Vaga) => temNota(v.aderencia) ? notas.indexOf(v.aderencia) : notas.length;
const pequena = () => window.matchMedia?.('(max-width: 760px)').matches ?? false;

function dataDoAnuncio(data: string | undefined, fimDoDia = false) {
  if (!data) return NaN;
  if (/^\d{4}-\d{2}-\d{2}$/.test(data)) return Date.parse(data + (fimDoDia ? 'T23:59:59.999' : 'T00:00:00'));
  return Date.parse(data);
}

function categoria(v: Vaga, agora: Date): Filtro {
  const prazo = dataDoAnuncio(v.expiraEm, true);
  const publicada = dataDoAnuncio(v.publicadaEm);
  const limite = new Date(agora);
  limite.setDate(limite.getDate() - 30);
  if (v.publicadaEm?.length === 10) limite.setHours(0,0,0,0);
  if (v.etapa === 'pulada' || v.aberta === false || prazo <= agora.getTime() || publicada < limite.getTime() ||
      v.triagemDecisao === 'pular' || v.aderencia === 'C' || v.radarClasse === 'excluir') return 'Histórico';
  if (v.triagemBusca === 'pendente') return 'A conferir';
  if (v.radarVersao !== 1) return 'Histórico';
  if (v.aderencia === 'A' && v.radarClasse === 'forte' && Number.isFinite(publicada) && publicada <= agora.getTime() &&
      (!v.expiraEm || Number.isFinite(prazo))) return 'Recomendadas';
  return 'A conferir';
}

export function vagasDisponiveisNoRadar(vagas: Vaga[], perfil = 'original') {
  const anunciosEmAndamento = new Set(vagas.filter(v => !['descoberta', 'pulada'].includes(v.etapa)).map(v => v.anuncioId ?? v.id));
  const unicas = new Map<string, Vaga>();
  for (const v of vagas) {
    if ((v.perfilBuscaId ?? 'original') !== perfil) continue;
    const manual = v.aplicacaoManual && v.etapa === 'aplicar';
    const historica = v.etapa === 'pulada' && (v.triagemBusca === 'expirada' || v.eventoPuloBusca || v.triagemDecisao === 'pular');
    if (manual || ((v.etapa === 'descoberta' || historica) && !anunciosEmAndamento.has(v.anuncioId ?? v.id))) unicas.set(v.id, v);
  }
  return [...unicas.values()].sort((a, b) => ordem(a) - ordem(b) || (dataDoAnuncio(b.publicadaEm) || 0) - (dataDoAnuncio(a.publicadaEm) || 0));
}

type Faixa = { min?: number; max?: number; moeda?: string; periodo?: string };
function salario(v: Vaga, pretensao: Faixa): [string, string] {
  let oferta: Faixa | undefined;
  if (v.salarioMin != null || v.salarioMax != null) {
    oferta = { min:v.salarioMin, max:v.salarioMax, moeda:v.moeda, periodo:v.periodoSalario };
  } else if (typeof v.salario === 'string' && /\d/.test(v.salario)) {
    return ['Salário da vaga', v.salario.trim()];
  } else if ((!v.moeda || v.moeda.toUpperCase()==='USD') && [v.salarioUSDMensal,v.salarioUSDMin,v.salarioUSDMax].some(n=>n!=null && n>0)) {
    oferta = { min:v.salarioUSDMensal || v.salarioUSDMin, max:v.salarioUSDMensal || v.salarioUSDMax, moeda:'USD', periodo:'monthly' };
  }
  const {min,max,moeda,periodo} = oferta ?? { ...pretensao, periodo: pretensao.periodo || 'monthly' };
  if (min == null && max == null) return ['Salário não informado', ''];
  const periodos: Record<string,string> = {hourly:'por hora', daily:'por dia', weekly:'por semana', monthly:'por mês', annual:'por ano', yearly:'por ano'};
  const simbolos: Record<string, string> = { BRL: 'R$', USD: 'US$', EUR: '€' };
  const formatar = (n: number) => [simbolos[moeda ?? ''] ?? moeda, n.toLocaleString('pt-BR', { maximumFractionDigits: 2 })].filter(Boolean).join(' ');
  const faixa = min != null && min===max ? formatar(min) : min != null && max != null ? `${formatar(min)} a ${formatar(max)}`
    : min != null ? `A partir de ${formatar(min)}` : max != null ? `Até ${formatar(max)}` : '';
  return [oferta ? 'Salário da vaga' : 'Sua pretensão', [faixa, periodos[periodo?.toLowerCase() ?? ''] || periodo].filter(Boolean).join(' ')];
}

function Fato({ icone, rotulo, valor }: { icone: string; rotulo: string; valor?: string }) {
  return <div className="rd-fact"><dt><IconeUI nome={icone} tamanho={15} />{rotulo}</dt><dd>{valor || 'Não informado'}</dd></div>;
}

function CandidaturaExterna({ vaga, ocupada, gravar }: {
  vaga: Vaga; ocupada: boolean; gravar: (acao: 'iniciar-manual' | 'confirmar-envio-manual') => void;
}) {
  const [adiada, adiar] = useState(false);
  const link = safeHref(vaga.link);
  if (foiEnviada(vaga) || vaga.etapa === 'enviada') return <p className="rd-enviada" role="status"><IconeUI nome="check" tamanho={16} />Candidatura enviada</p>;
  if (vaga.aplicacaoManual && vaga.etapa === 'aplicar') return <div className="rd-manual">
    <strong>Aguardando você</strong>
    {adiada ? <button type="button" className="btn secondary" onClick={() => adiar(false)}>Retomar candidatura</button> : <>
      <p>Conclua a candidatura no site da empresa. Depois, volte aqui e confirme o envio.</p>
      <div className="rd-links">
        <LinkExterno className="btn primary" url={vaga.urlCandidaturaManual ?? link}>Abrir candidatura no navegador</LinkExterno>
        <button className="btn secondary" type="button" disabled={ocupada} onClick={() => gravar('confirmar-envio-manual')}>Já enviei</button>
        <button className="btn quiet" type="button" disabled={ocupada} onClick={() => adiar(true)}>Continuar depois</button>
      </div>
    </>}
  </div>;
  return <div className="rd-links">
    {link?.startsWith('https://') && <button type="button" className="btn secondary" disabled={ocupada} onClick={() => gravar('iniciar-manual')}>Aplicar na vaga</button>}
    <LinkExterno url={link} className="btn quiet" semLink={<span>Anúncio sem link disponível.</span>}>Ler anúncio</LinkExterno>
  </div>;
}

function VagaDoRadar({ vaga, pretensao, voltar, revisar, ocupada, gravar, desfazer }: {
  vaga: Vaga; voltar: () => void; revisar: (etapa: 'aplicar' | 'pulada') => void;
  ocupada: boolean; pretensao: Faixa; gravar: (acao: 'iniciar-manual' | 'confirmar-envio-manual') => void; desfazer: ReactNode;
}) {
  const [rotuloSalario, valorSalario] = salario(vaga, pretensao);
  const grupo = temNota(vaga.aderencia) ? vaga.aderencia : null;
  const tags = [...new Set([...(vaga.categorias ?? []), ...(vaga.stack ?? []), vaga.nivel, vaga.contrato].filter(Boolean))];
  return <article className="rd-detail" id="rd-detail" aria-labelledby="rd-job-heading" onKeyDown={e => {
    if (e.key === 'Escape' && pequena()) { e.preventDefault(); voltar(); }
  }}>
    <button type="button" className="rd-back" onClick={voltar}><IconeUI nome="chevron-left" tamanho={16} />Voltar às vagas</button>
    <div className="rd-detail-head">
      <div className="rd-employer"><MarcaEmpresa nome={vaga.empresa} /><span>{vaga.empresa}</span></div>
      <h3 id="rd-job-heading" tabIndex={-1}>{vaga.titulo}</h3>
    </div>
    <dl className="rd-facts">
      <Fato icone="pin" rotulo="Localização" valor={vaga.local} />
      <Fato icone="briefcase" rotulo="Modalidade" valor={vaga.modelo} />
      {valorSalario ? <Fato icone="wallet" rotulo={rotuloSalario} valor={valorSalario} />
        : <div className="rd-fact"><dt><IconeUI nome="wallet" tamanho={15} />Salário</dt><dd>{rotuloSalario}</dd></div>}
    </dl>
    {vaga.etapa === 'descoberta' && <div className="rd-actions">
      <button className="btn primary" type="button" disabled={ocupada} onClick={() => revisar('aplicar')}><IconeUI nome="plus" tamanho={17} />Adicionar à lista</button>
      <button className="btn quiet rd-skip" type="button" disabled={ocupada} onClick={() => revisar('pulada')}>Pular<IconeUI nome="chevron-right" tamanho={15} /></button>
    </div>}
    <div className="rd-analysis">
      <div className="rd-analysis-head"><span className="rd-analysis-name"><IconeUI nome="sparkles" tamanho={16} />Compatibilidade com seu perfil</span>
        {grupo && <span className={`rd-grade-label rd-grade-${grupo}`}>Nota {grupo}</span>}</div>
      <p>{vaga.radarMotivo || vaga.motivoClaude || (vaga.triagemBusca === 'pendente' ? 'Avaliação pendente' : 'Nenhuma análise registrada.')}</p>
      {vaga.radarMotivo && vaga.motivoClaude && vaga.motivoClaude !== vaga.radarMotivo && <p>{vaga.motivoClaude}</p>}
      {vaga.triagemDecisao === 'pular' && vaga.triagemMotivoPulo && <p><strong>A IA sugere pular</strong> · {vaga.triagemMotivoPulo}</p>}
      {!!vaga.triagemConferir?.length && <div className="rd-conferir"><strong>Pontos a conferir</strong><ul>{vaga.triagemConferir.map((p, i) => <li key={i}>{p}</li>)}</ul></div>}
    </div>
    {vaga.descricao && <div className="rd-description"><h4>Sobre a vaga</h4><p>{vaga.descricao}</p></div>}
    {!!tags.length && <ul className="rd-tags" aria-label="Competências e requisitos">{tags.map(tag => <li key={tag}>{tag}</li>)}</ul>}
    <div className="rd-externa">
      {vaga.fonteBusca && <LinkExterno url={vaga.fonteURL} semLink={<span>Fonte: {vaga.fonteBusca}</span>}>Fonte: {vaga.fonteBusca}</LinkExterno>}
      <span>{Number.isFinite(dataDoAnuncio(vaga.publicadaEm)) ? `Publicada em ${dataCurta(vaga.publicadaEm!.slice(0,10))} ${vaga.publicadaEm!.slice(0,4)}` : 'Data de publicação não informada'}</span>
      <CandidaturaExterna vaga={vaga} ocupada={ocupada} gravar={gravar} />
    </div>
    {desfazer}
  </article>;
}

function EstadoRadar({ titulo, icone = 'search', mensagem, children }: { titulo: string; icone?: string; mensagem: string; children?: ReactNode }) {
  return <div className="rd-empty"><span className="rd-empty-icon"><IconeUI nome={icone} tamanho={24} /></span>
    <h3 tabIndex={-1}>{titulo}</h3><p>{mensagem}</p>{children && <div className="rd-empty-actions">{children}</div>}</div>;
}

function RadarDoPerfil({ idPerfil }: { idPerfil: string }) {
  const app = useApp();
  const { estado } = app;
  const { ocupado: buscando, balanco, erro, buscar, pendentes } = useBuscaDoPerfil(true);
  const [filtro, filtrar] = useState<Filtro>('Recomendadas');
  const [selecao, selecionar] = useState<string | null>(null);
  const [confirmada, lembrarConfirmacao] = useState<string | null>(null);
  const [detalheMovel, mostrarDetalhe] = useState(false);
  const [pulada, lembrarPulo] = useState<Vaga | null>(null);
  const [ocupada, ocupar] = useState(false);
  const [erroAcao, falhar] = useState('');
  const emCurso = useRef(false);
  const regiao = useRef<HTMLElement>(null);
  const focoPendente = useRef<'detalhe' | 'linha' | null>(null);
  const perfil = estado.perfil ?? {};
  const local = aceitaTrabalhoLocal(perfil.filtros?.modelo);
  const pais = nomeDoPais(perfil.paisTrabalho);
  const cargoInvalido = cargoIgualAoNome(perfil.cargo,perfil.nome);
  const resumo = [!cargoInvalido && perfil.cargo, perfil.filtros?.modelo?.join(', '), [local && perfil.cidadeTrabalho,pais].filter(Boolean).join(', '), mercados[perfil.foco]].filter(Boolean).join(' · ');
  const ajuste = cargoInvalido ? 'O cargo coincide com seu nome. Informe a função que deseja buscar.'
    : !pais ? 'Informe de qual país você vai trabalhar.' : local && !perfil.cidadeTrabalho ? 'Informe a cidade para vagas presenciais ou híbridas.' : '';
  const pretensao = perfil.faixaSalarial ?? {min: perfil.pretensaoUSD || undefined, moeda:'USD'};
  const vagas = (estado.vagas ?? []) as Vaga[];
  const todas = vagas.filter(v => (v.perfilBuscaId ?? 'original') === idPerfil);
  const disponiveis = vagasDisponiveisNoRadar(vagas, idPerfil);
  const agora = new Date();
  const candidatas = disponiveis.filter(v => categoria(v, agora) === filtro);
  const visiveis = filtro === 'Recomendadas' ? candidatas.slice(0,10) : candidatas;
  const atual = visiveis.find(v => v.id === selecao) ?? visiveis[0];
  // Recomendadas só mostra nota A. Numa rodada só com B (ou ainda avaliando), a aba parecia vazia e as vagas
  // ficavam escondidas em "A conferir": a tela aponta para lá.
  const aConferir = disponiveis.filter(v => categoria(v, agora) === 'A conferir').length;
  const semNotaA = filtro === 'Recomendadas' && !atual && aConferir > 0;
  const textoSemNotaA = `${plural(aConferir, 'vaga com nota B ou ainda sem avaliação espera', 'vagas com nota B ou ainda sem avaliação esperam')} sua conferência.`;
  const verAConferir = <button type="button" className="btn secondary" onClick={() => mudarFiltro('A conferir')}>Ver vagas a conferir</button>;
  const recibo = todas.find(v => v.id === confirmada && (foiEnviada(v) || v.etapa === 'enviada'));
  const puladas = todas.filter(v => v.etapa === 'pulada' && v.triagemBusca !== 'expirada');
  const ultima = estado.rodadas?.filter(r => (r.perfilBuscaId ?? 'original') === idPerfil).slice(-1)[0];
  useLayoutEffect(() => {
    if (!focoPendente.current) return;
    const alvo = focoPendente.current === 'detalhe' ? '#rd-job-heading, .rd-empty h3' : '.rd-row[aria-current="true"]';
    const el = regiao.current?.querySelector<HTMLElement>(alvo);
    if (el) { el.focus({ preventScroll: true }); el.scrollIntoView?.({ block: 'nearest' }); }
    focoPendente.current = null;
  });

  function abrir(v: Vaga) {
    focoPendente.current = pequena() ? 'detalhe' : 'linha';
    selecionar(v.id); mostrarDetalhe(pequena()); lembrarConfirmacao(null); falhar('');
  }
  function voltar() {
    focoPendente.current = 'linha';
    mostrarDetalhe(false);
  }
  function mudarFiltro(nota: Filtro) {
    filtrar(nota); selecionar(null); mostrarDetalhe(false); lembrarConfirmacao(null); falhar('');
  }
  function teclaDaLista(ev: KeyboardEvent<HTMLButtonElement>, i: number) {
    const destinos = { ArrowDown: Math.min(i + 1, visiveis.length - 1), ArrowUp: Math.max(0, i - 1), Home: 0, End: visiveis.length - 1 };
    if (!Object.prototype.hasOwnProperty.call(destinos, ev.key)) return;
    ev.preventDefault();
    const proxima = destinos[ev.key as keyof typeof destinos];
    if (pequena()) regiao.current?.querySelectorAll<HTMLButtonElement>('.rd-row')[proxima]?.focus();
    else abrir(visiveis[proxima]);
  }
  async function agir(fn: () => Promise<void>) {
    if (emCurso.current) return;
    emCurso.current = true; ocupar(true); falhar('');
    try { await fn(); }
    catch (e) { falhar(e instanceof Error ? e.message : 'Não foi possível salvar. Tente novamente.'); }
    finally { emCurso.current = false; ocupar(false); }
  }
  function revisar(etapa: 'aplicar' | 'pulada') {
    if (!atual) return;
    const i = visiveis.indexOf(atual);
    void agir(async () => {
      const ok = await app.mover(atual.id, etapa, etapa === 'pulada' ? { motivo: 'Pulada no Radar.' } : {}, etapa === 'pulada' ? 'Vaga pulada.' : 'Vaga adicionada à lista.');
      if (!ok) return;
      if (etapa === 'pulada') lembrarPulo(atual);
      focoPendente.current = 'detalhe';
      selecionar((visiveis[i + 1] ?? visiveis[i - 1])?.id ?? null);
    });
  }
  function recuperar(vagas: Vaga[]) {
    void agir(async () => {
      for (const v of vagas) {
        if (!await app.mover(v.id, 'descoberta')) return;
      }
      lembrarPulo(null); filtrar(categoria({...vagas[0], etapa:'descoberta'}, new Date())); selecionar(vagas[0]?.id ?? null);
      focoPendente.current = pequena() && detalheMovel ? 'detalhe' : 'linha';
    });
  }
  function candidatura(acao: 'iniciar-manual' | 'confirmar-envio-manual') {
    if (!atual) return;
    void agir(async () => {
      await app.api(`/api/v1/vagas/${encode(atual.id)}/${acao}`, 'POST', {});
      if (acao === 'confirmar-envio-manual') lembrarConfirmacao(atual.id);
      await app.recarregar();
    });
  }
  const desfazer = pulada && puladas.some(v => v.id === pulada.id) && <div className="rd-undo">
    <span>Vaga de {pulada.empresa} pulada.</span><button type="button" disabled={ocupada} onClick={() => recuperar([pulada])}>Desfazer</button>
  </div>;
  const erroFontes = !todas.length && balanco?.encontradas === 0 && balanco.avisos.length ? balanco.avisos.join(' ') : '';
  const falhaBusca = erro || erroFontes;
  return <div className="radar">
    <section className="radar-hero" data-search={buscando ? 'loading' : 'ready'} aria-labelledby="hero-title">
      <div className="hero-art" aria-hidden="true"><img src={radarArte} alt="" /></div>
      <div className="hero-copy">
        <div className="hero-kicker"><span className="cross-star" aria-hidden="true">✦</span> DESCOBERTA</div>
        <h1 id="hero-title">Radar de vagas<span aria-hidden="true">.</span></h1>
        <div className="hero-profile"><SeletorPerfil /><p>{resumo || 'Defina seu perfil para encontrar oportunidades.'}</p></div>
        {ajuste && <p>{ajuste}</p>}
        <div className="hero-buttons">
          <button type="button" className="btn primary" disabled={buscando} aria-busy={buscando} onClick={buscar}><IconeUI nome="radar" tamanho={17} />
            {buscando ? 'Buscando…' : pendentes ? 'Retomar avaliação das vagas' : 'Buscar vagas'}</button>
          <a className="hero-edit" href="#/perfil"><IconeUI nome="sliders" tamanho={15} />Ajustar perfil</a>
        </div>
      </div>
    </section>
    <section className="rd-results" aria-labelledby="rd-title" ref={regiao}>
      <div className="rd-toolbar"><div className="rd-heading"><h2 id="rd-title" tabIndex={-1}>Vagas encontradas</h2>
        <span className="rd-count" aria-label={plural(visiveis.length, 'vaga disponível', 'vagas disponíveis')}>{visiveis.length}</span></div>
        <div className="rd-filters" role="group" aria-label="Filtrar vagas por compatibilidade">
          {filtros.map(nome => <button key={nome} type="button" aria-pressed={filtro === nome} onClick={() => mudarFiltro(nome)}>{nome}</button>)}
        </div>
      </div>
      {/* A triagem leva de 20 a 80 s por vaga: durante a busca, as já avaliadas aparecem abaixo (a busca recarrega o estado). */}
      {buscando && <div className="rd-loading" aria-busy="true"><p role="status">Buscando vagas para o seu perfil…</p>
        {atual ? <p className="rd-loading-nota">As vagas já avaliadas aparecem abaixo enquanto a busca continua.</p>
          : semNotaA ? <div className="rd-loading-nota"><p>Ainda nenhuma vaga com nota A. {textoSemNotaA}</p>{verAConferir}</div>
          : <div className="rd-skeletons" aria-hidden="true">{[0, 1, 2].map(n => <div key={n}><i /><span><b /><b /></span></div>)}</div>}
      </div>}
      {(!buscando || atual) && <>
        {recibo && <article aria-label={recibo.titulo} className="rd-confirmacao">
          <p className="rd-enviada" role="status"><IconeUI nome="check" tamanho={16} />Candidatura enviada</p>
          <strong>{recibo.titulo}</strong><span>{recibo.empresa}</span><a className="btn secondary" href="#/quadro">Ver no quadro</a>
        </article>}
        {falhaBusca && <EstadoRadar titulo="A busca foi interrompida" mensagem={falhaBusca}>
          <button className="btn secondary" type="button" onClick={buscar}>Tentar novamente</button><a className="btn quiet" href="#/ia">Ver conexão da IA</a>
        </EstadoRadar>}
        {atual ? <div className={`rd-workbench${detalheMovel ? ' rd-show-detail' : ''}`}>
          <ul className="rd-list" aria-label="Vagas para revisão">
            {visiveis.map((v, i) => <li key={v.id}><button type="button" className={`rd-row${atual.id === v.id ? ' rd-selected' : ''}`}
              aria-controls="rd-detail" aria-current={atual.id === v.id || undefined} onClick={() => abrir(v)} onKeyDown={ev => teclaDaLista(ev, i)}>
              <MarcaEmpresa nome={v.empresa} />
              <span className="rd-row-info"><span className="rd-company">{v.empresa}</span><strong>{v.titulo}</strong>
                {v.local && <span className="rd-location"><IconeUI nome="pin" tamanho={13} />{v.local}</span>}</span>
              <span className="rd-row-end">
                {temNota(v.aderencia) && <span className={`rd-grade rd-grade-${v.aderencia}`} aria-label={`Nota ${v.aderencia}, ${afinidade[v.aderencia]}`}>{v.aderencia}</span>}
                <span className="rd-open-icon" aria-hidden="true"><IconeUI nome="chevron-right" tamanho={14} /></span>
              </span>
            </button></li>)}
          </ul>
          <VagaDoRadar key={atual.id} vaga={atual} pretensao={pretensao} voltar={voltar} revisar={revisar} ocupada={ocupada} gravar={candidatura} desfazer={desfazer} />
        </div> : semNotaA ? <>
          <EstadoRadar titulo="Nenhuma vaga com nota A nesta busca" mensagem={textoSemNotaA}>{verAConferir}</EstadoRadar>
          {desfazer}
        </> : disponiveis.length || filtro !== 'Recomendadas' ? <>
          <EstadoRadar titulo={filtro === 'Recomendadas' ? 'Nenhuma recomendação confirmada' : filtro === 'A conferir' ? 'Nenhuma vaga a conferir' : 'Nenhuma vaga no histórico'}
            mensagem={filtro === 'Recomendadas' ? 'Veja se há vagas a conferir ou anúncios no histórico.' : 'As vagas desta categoria aparecerão aqui.'}/>
          {desfazer}
        </> : todas.length ? <>
          <EstadoRadar icone="check" titulo="Você revisou todas as vagas" mensagem="Acompanhe na lista as vagas que você escolheu.">
            <a className="btn primary" href="#/lista">Ver lista</a>
            {puladas.length > 0 && <button type="button" className="btn secondary" disabled={ocupada} onClick={() => recuperar(puladas)}>Rever vagas puladas</button>}
          </EstadoRadar>{desfazer}
        </> : !falhaBusca && <EstadoRadar titulo={balanco ? 'Nenhuma vaga compatível nesta busca' : 'Nenhuma vaga encontrada'} mensagem="Ajuste suas preferências para ampliar a busca.">
          <a className="btn secondary" href="#/perfil">Ajustar preferências</a>
        </EstadoRadar>}
        {erroAcao && <p className="rd-erro" role="alert">{erroAcao}</p>}
        {Boolean(pendentes) && !falhaBusca && <p className="rd-conexao"><a href="#/ia">Ver conexão da IA</a></p>}
        {balanco && !erroFontes ? <details className="rd-balanco" open={balanco.avisos.length > 0 || undefined}>
          <summary>{balanco.avisos.length || balanco.pendentes ? 'Busca parcial' : 'Resultado da busca'}</summary><BalancoDaBusca balanco={balanco} />
        </details> : ultima && !falhaBusca && <details className="rd-balanco">
          <summary>Última busca</summary><p>{plural(ultima.encontradas, 'vaga encontrada', 'vagas encontradas')}. {ultima.resumo}</p>
        </details>}
      </>}
    </section>
  </div>;
}

export function TelaRadar() {
  const ativo = useApp().estado.perfilBuscaAtivo ?? 'original';
  return <RadarDoPerfil key={ativo} idPerfil={ativo} />;
}
