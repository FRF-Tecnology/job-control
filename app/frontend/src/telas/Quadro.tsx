// Quadro: colunas por etapa, cartões arrastáveis (porte de web/telas/quadro.mjs).
import { useRef, useState } from 'react';
import { useApp, type App } from '../app/contexto';
import { MOTIVOS_PULO, MOTIVOS_ENCERRAMENTO, SUB_ETAPAS } from '../dominio/modelo';
import { dataCurta, nomeEtapa, nomePlataforma, NOMES_SUB } from '../util';

const COLUNAS = ['bloqueada', 'descoberta', 'aplicar', 'enviada', 'em-processo', 'oferta'];
const LIMITE = 40;
const expandidas = new Set<string>();

// Pede o que cada etapa exige antes de mover (motivo, pendência, subetapa). null = a pessoa desistiu.
export async function extrasPara(app: App, etapa: string, opcoesPendencia: string[] = []): Promise<Record<string, unknown> | null> {
  if (etapa === 'pulada' || etapa === 'encerrada') {
    const motivo = await app.perguntar({ titulo: etapa === 'pulada' ? 'Por que pular?' : 'Por que encerrar?', opcoes: etapa === 'pulada' ? MOTIVOS_PULO : MOTIVOS_ENCERRAMENTO });
    return motivo ? { motivo } : null;
  }
  if (etapa === 'bloqueada') {
    const pendencia = await app.perguntar({ titulo: 'O que falta você fazer?', campo: 'ex.: digitar CPF e enviar', opcoes: opcoesPendencia });
    return pendencia ? { pendencia } : null;
  }
  if (etapa === 'em-processo') {
    const s = await app.perguntar({ titulo: 'Em que etapa está?', opcoes: SUB_ETAPAS.map(x => NOMES_SUB[x]) });
    return s ? { subEtapa: SUB_ETAPAS.find(x => NOMES_SUB[x] === s) } : null;
  }
  return {};
}

export function TelaQuadro() {
  const app = useApp();
  const { vagas } = app.estado;
  const arrastada = useRef<string | null>(null);
  const [sobre, setSobre] = useState<string | null>(null);
  const [, redesenhar] = useState(0);

  const soltar = async (etapa: string) => {
    setSobre(null);
    const v = vagas.find((x: any) => x.id === arrastada.current);
    if (!v || v.etapa === etapa) return;
    const extras = await extrasPara(app, etapa);
    if (extras) app.mover(v.id, etapa, extras, `${v.empresa}: ${nomeEtapa(etapa)}`);
  };
  const alvo = (etapa: string) => ({
    onDragOver: (ev: React.DragEvent) => { ev.preventDefault(); setSobre(etapa); },
    onDragLeave: (ev: React.DragEvent) => { if (!ev.currentTarget.contains(ev.relatedTarget as Node)) setSobre(null); },
    onDrop: (ev: React.DragEvent) => { ev.preventDefault(); soltar(etapa); },
  });

  const cartao = (v: any) => (
    <article key={v.id} className={`cartao e-${v.etapa}`} draggable tabIndex={0} aria-label={`${v.empresa}, ${v.titulo}. Enter abre o detalhe.`}
      onDragStart={ev => { arrastada.current = v.id; ev.dataTransfer.effectAllowed = 'move'; ev.currentTarget.classList.add('arrastando'); }}
      onDragEnd={ev => ev.currentTarget.classList.remove('arrastando')}
      onClick={() => app.abrirVaga(v.id)}
      onKeyDown={ev => { if (ev.key === 'Enter') app.abrirVaga(v.id); }}>
      <div className="m"><span>{v.empresa}</span>{v.aderencia && <span className={`chip ader-${v.aderencia}`}>{v.aderencia}</span>}</div>
      <div className="t">{v.titulo}</div>
      <div className="m">
        <span>{nomePlataforma(v.plataforma, app.estado.plataformas)}</span>
        {v.envio?.data ? <span>{`enviada ${dataCurta(v.envio.data)}`}</span> : null}
        {v.subEtapa ? <span>{NOMES_SUB[v.subEtapa]}</span> : null}
        {v.pendencia ? <span style={{ color: 'var(--ambar)' }}>{v.pendencia}</span> : null}
      </div>
    </article>
  );

  const descarte = (etapa: string, rotulo: string) => (
    <div className={`coluna${sobre === etapa ? ' sobre' : ''}`} style={{ minHeight: '64px', justifyContent: 'center', alignItems: 'center', color: 'var(--tinta-2)' }} {...alvo(etapa)}>{rotulo}</div>
  );

  return (
    <div>
      <header className="cabecalho"><div><h1>Quadro</h1><p>Arraste os cartões entre as colunas. No celular, abra o cartão e use "Mover para".</p></div></header>
      <div className="quadro">
        {COLUNAS.map(etapa => {
          const lista = vagas.filter((v: any) => v.etapa === etapa)
            .sort((a: any, b: any) => (b.envio?.data ?? b.atualizadoEm ?? '').localeCompare(a.envio?.data ?? a.atualizadoEm ?? ''));
          const mostrar = expandidas.has(etapa) ? lista : lista.slice(0, LIMITE);
          return (
            <section key={etapa} className={`coluna e-${etapa}${sobre === etapa ? ' sobre' : ''}`} aria-label={nomeEtapa(etapa)} {...alvo(etapa)}>
              <header><span className={`etapa e-${etapa}`}>{nomeEtapa(etapa)}</span><span className="num mudo">{lista.length}</span></header>
              {mostrar.map(cartao)}
              {lista.length > mostrar.length && <button className="mais" type="button" onClick={() => { expandidas.add(etapa); redesenhar(n => n + 1); }}>{`Ver mais ${lista.length - mostrar.length}`}</button>}
              {!lista.length && <p className="pequeno mudo" style={{ padding: '6px' }}>Vazio</p>}
            </section>
          );
        })}
      </div>
      <div className="acoes" style={{ marginTop: '12px', display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '14px' }}>
        {descarte('pulada', `Solte aqui para pular (${vagas.filter((v: any) => v.etapa === 'pulada').length})`)}
        {descarte('encerrada', `Solte aqui para encerrar (${vagas.filter((v: any) => v.etapa === 'encerrada').length})`)}
      </div>
    </div>
  );
}
