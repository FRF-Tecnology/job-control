import { useId, useRef, useState } from 'react';
import { useApp } from '../app/contexto';
import { LinkExterno } from '../componentes';
import { foiEnviada, NOMES_FOCO } from '../dominio/modelo';
import { dataCurta, encode, nomeEtapa, plural, safeHref } from '../util';
import { BuscaDoPerfil } from './BuscaDoPerfil';
import '../estilo/radar.css';

type Vaga = {
  id: string; titulo: string; empresa: string; etapa: string; link?: string; local?: string; modelo?: string;
  nivel?: string; contrato?: string; aderencia?: string; motivoClaude?: string; triagemMotivoPulo?: string;
  triagemDecisao?: string; triagemConferir?: string[]; triagemBusca?: string; fonteBusca?: string;
  fonteURL?: string; descobertaEm?: string; aplicacaoManual?: boolean; urlCandidaturaManual?: string; envio?: { data?: string };
};
const prioridade: Record<string, number> = { A: 0, B: 1, C: 2 };

function faixaMensal(faixa?: { moeda: string; min?: number; max?: number }) {
  if (!faixa || (faixa.min == null && faixa.max == null)) return '';
  const valor = (n: number) => n.toLocaleString('pt-BR');
  const limites = faixa.min != null && faixa.max != null ? `${valor(faixa.min)} a ${valor(faixa.max)}`
    : faixa.min != null ? `a partir de ${valor(faixa.min)}` : `até ${valor(faixa.max!)}`;
  return `${faixa.moeda} ${limites} por mês`;
}

function PerfilDoRadar() {
  const { perfil = {} } = useApp().estado;
  const faixa = perfil.faixaSalarial ?? (perfil.pretensaoUSD ? { moeda: 'USD', min: Number(perfil.pretensaoUSD) } : undefined);
  const curriculos = [...new Set(Object.values(perfil.curriculos ?? {}).filter((cv): cv is string => typeof cv === 'string')
    .map(cv => cv.split('/').pop() ?? '').filter(Boolean)
    .map(nome => /^curriculo-[A-Z2-7]{26,}\.pdf$/.test(nome) ? 'PDF do primeiro acesso' : nome))];
  const criterios = [
    ['Níveis', perfil.filtros?.niveis?.join(', ')], ['Modelo de trabalho', perfil.filtros?.modelo?.join(', ')],
    ['Mercado', NOMES_FOCO[perfil.foco]], ['Stack principal', perfil.stackPrincipal?.join(', ')],
    ['Stack secundária', perfil.stackSecundaria?.join(', ')], ['Inglês', perfil.ingles],
    ['Faixa salarial', faixaMensal(faixa)], ['Pular quando', perfil.filtros?.pularSe?.join(', ')],
    ['Currículo', curriculos.join(', ')],
  ].filter(([, valor]) => valor);
  return <section className="radar-perfil" aria-label="Perfil de busca">
    <div className="radar-perfil-topo">
      <div><span className="pequeno mudo">Perfil de busca</span><h2>{perfil.cargo || 'Informe o cargo que procura'}</h2></div>
      <a href="#/perfil" className="botao-fantasma p">Editar perfil</a>
    </div>
    {criterios.length > 0 && <dl className="radar-criterios">{criterios.map(([rotulo, valor]) =>
      <div key={rotulo}><dt>{rotulo}</dt><dd>{valor}</dd></div>)}</dl>}
  </section>;
}

function VagaDoRadar({ vaga }: { vaga: Vaga }) {
  const app = useApp();
  const titulo = useId();
  const emCurso = useRef(false);
  const [ocupada, ocupar] = useState(false);
  const [adiada, adiar] = useState(false);
  const [erro, falhar] = useState('');
  const enviada = foiEnviada(vaga) || vaga.etapa === 'enviada';
  const aguardando = vaga.aplicacaoManual && vaga.etapa === 'aplicar' && !enviada;
  const disponivel = !enviada && (vaga.etapa === 'descoberta' || vaga.etapa === 'aplicar');
  const link = safeHref(vaga.link);
  async function gravar(acao: 'iniciar-manual' | 'confirmar-envio-manual') {
    if (emCurso.current) return;
    emCurso.current = true;
    ocupar(true); falhar('');
    try {
      await app.api(`/api/v1/vagas/${encode(vaga.id)}/${acao}`, 'POST', {});
      await app.recarregar();
    } catch (e) { falhar(e instanceof Error ? e.message : 'Não foi possível salvar. Tente novamente.'); }
    finally { emCurso.current = false; ocupar(false); }
  }
  const grupo = vaga.aderencia && Object.prototype.hasOwnProperty.call(prioridade, vaga.aderencia) ? vaga.aderencia : '';
  const detalhes = [vaga.local, vaga.modelo, vaga.nivel, vaga.contrato].filter(Boolean).join(' · ');
  return <article aria-labelledby={titulo} className="radar-vaga">
    <div className="radar-vaga-topo"><span className="radar-empresa">{vaga.empresa}</span>
      {grupo ? <span className={`chip ader-${grupo}`}>Grupo {grupo}</span>
        : vaga.triagemBusca === 'pendente' && <span className="chip">Avaliação pendente</span>}
    </div>
    <h3 id={titulo}>{vaga.titulo}</h3>
    {detalhes && <p className="pequeno mudo">{detalhes}</p>}
    {vaga.motivoClaude && <p>{vaga.motivoClaude}</p>}
    {vaga.triagemDecisao === 'pular' && vaga.triagemMotivoPulo && <p><strong>A IA sugere pular</strong> · {vaga.triagemMotivoPulo}</p>}
    {!!vaga.triagemConferir?.length && <div><strong>Pontos a conferir</strong><ul>{vaga.triagemConferir.map((p, i) => <li key={i}>{p}</li>)}</ul></div>}
    <div className="radar-fonte pequeno">
      {vaga.fonteBusca && <LinkExterno url={vaga.fonteURL} semLink={<span>{vaga.fonteBusca}</span>}>Fonte: {vaga.fonteBusca}</LinkExterno>}
      {vaga.descobertaEm && <span>Encontrada em {dataCurta(vaga.descobertaEm)}</span>}
    </div>
    <div className="radar-vaga-acoes">
      {enviada ? <p role="status" className="radar-enviada">Candidatura enviada</p> : aguardando ? <>
        <p className="radar-aguardando"><strong>Aguardando você</strong></p>
        {adiada ? <button type="button" className="botao-fantasma" onClick={() => adiar(false)}>Retomar candidatura</button> : <>
          <p className="pequeno mudo">Conclua a candidatura no site da empresa. Depois, volte aqui e confirme o envio.</p>
          <div className="acoes">
            <LinkExterno className="botao" url={vaga.urlCandidaturaManual ?? link}>Abrir candidatura no navegador</LinkExterno>
            <button className="botao-fantasma" type="button" disabled={ocupada} onClick={() => void gravar('confirmar-envio-manual')}>Já enviei</button>
            <button className="botao-fantasma" type="button" disabled={ocupada} onClick={() => adiar(true)}>Continuar depois</button>
          </div>
        </>}
      </> : <div className="acoes">
        {disponivel && link?.startsWith('https://') && <button type="button" className="botao" disabled={ocupada} onClick={() => void gravar('iniciar-manual')}>Aplicar na vaga</button>}
        {!disponivel && <span className="chip">{nomeEtapa(vaga.etapa)}</span>}
        <LinkExterno url={link} className="botao-fantasma" semLink={<span className="mudo pequeno">Anúncio sem link disponível.</span>}>Ler anúncio</LinkExterno>
      </div>}
      {erro && <p className="radar-erro" role="alert">{erro}</p>}
    </div>
  </article>;
}

export function TelaRadar() {
  const app = useApp();
  const unicas = new Map<string, Vaga>();
  for (const vaga of (app.estado.vagas ?? []) as Vaga[]) {
    if (vaga.etapa === 'pulada') continue;
    if (vaga.fonteBusca || vaga.triagemBusca || vaga.aplicacaoManual) unicas.set(vaga.id, vaga);
  }
  const vagas = [...unicas.values()].sort((a, b) =>
    (prioridade[a.aderencia ?? ''] ?? 3) - (prioridade[b.aderencia ?? ''] ?? 3)
    || (b.descobertaEm ?? '').localeCompare(a.descobertaEm ?? ''));
  return <div className="radar">
    <header className="radar-cabecalho">
      <div><h1>Radar de vagas</h1><BuscaDoPerfil radar /></div>
    </header>
    <PerfilDoRadar />
    {vagas.length > 0 && <section aria-label="Vagas do Radar" className="radar-resultados">
      <h2>{plural(vagas.length, 'vaga no Radar', 'vagas no Radar')}</h2>
      {vagas.map(vaga => <VagaDoRadar key={vaga.id} vaga={vaga} />)}
    </section>}
  </div>;
}
