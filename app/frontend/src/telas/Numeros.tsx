// Números: funil, envios por semana e retorno por plataforma e aderência (porte de web/telas/numeros.mjs).
import { useApp } from '../app/contexto';
import { resumo, funil, porPlataforma, porSemana, motivosPulo } from '../dominio/funil';
import { dataCurta, nomePlataforma } from '../util';

type Linha = { rotulo: string; valor: number; texto?: string; cor?: string };

function Barras({ linhas, cor }: { linhas: Linha[]; cor?: string }) {
  const max = Math.max(1, ...linhas.map(l => l.valor));
  return (
    <div className="barras">
      {linhas.map(l => (
        <div key={l.rotulo} className="barra">
          <span title={l.rotulo}>{l.rotulo}</span>
          <span className="trilho" role="img" aria-label={`${l.rotulo}: ${l.valor}`}><i style={{ width: `${(l.valor / max) * 100}%`, background: l.cor ?? cor }} /></span>
          <span className="v">{l.texto ?? String(l.valor)}</span>
        </div>
      ))}
    </div>
  );
}

export function TelaNumeros() {
  const app = useApp();
  const { vagas, plataformas } = app.estado;
  const r = resumo(vagas, app.hoje);
  const semanas = porSemana(vagas).slice(-10);
  const maxSemana = Math.max(1, ...semanas.map(s => s.total));
  const plats = porPlataforma(vagas).filter((p: any) => p.enviadas > 0);
  const aderencias = ['A', 'B', 'C'].map(g => {
    const env = vagas.filter((v: any) => v.aderencia === g && v.envio?.data);
    const ret = env.filter((v: any) => ['em-processo', 'oferta'].includes(v.etapa) || (v.etapa === 'encerrada' && v.motivo === 'Reprovada'));
    return { g, env: env.length, ret: ret.length };
  });
  const pulos = motivosPulo(vagas);
  const cores = ['var(--e-enviada)', 'var(--e-descoberta)', 'var(--e-em-processo)', 'var(--e-oferta)'];

  return (
    <div>
      <header className="cabecalho"><div><h1>Números</h1>
        <p>{`${r.enviadas} enviadas, ${r.retornos} com retorno (${r.taxaRetorno}%). Retorno conta quem avançou no processo ou reprovou: a empresa olhou.`}</p></div></header>
      <div className="paineis">
        <section className="painel"><h2>Funil</h2><Barras linhas={funil(vagas).map((f, i) => ({ rotulo: f.etapa, valor: f.total, cor: cores[i] }))} /></section>
        <section className="painel"><h2>Envios por semana</h2>
          {semanas.length ? (
            <div className="colunas-semana">
              {semanas.map(s => <div key={s.semana}><span className="num">{s.total}</span><i style={{ height: `${(s.total / maxSemana) * 100}%` }} title={`${s.total} na semana de ${dataCurta(s.semana)}`} /><span>{dataCurta(s.semana)}</span></div>)}
            </div>
          ) : <p className="mudo">Sem envios ainda.</p>}
        </section>
        <section className="painel"><h2>Retorno por plataforma</h2>
          {plats.length ? <Barras linhas={plats.map((p: any) => ({ rotulo: nomePlataforma(p.plataforma, plataformas), valor: p.enviadas, texto: `${p.retornos}/${p.enviadas}` }))} /> : <p className="mudo">Sem envios ainda.</p>}
          <p className="pequeno mudo">Barra = enviadas. Número = com retorno / enviadas.</p>
        </section>
        <section className="painel"><h2>Retorno por aderência</h2>
          <Barras linhas={aderencias.map(a => ({ rotulo: `Grupo ${a.g}`, valor: a.env, texto: a.env ? `${Math.round((a.ret / a.env) * 100)}%` : '–' }))} />
          <p className="pequeno mudo">Mostra se as vagas de menor aderência valem o esforço.</p>
        </section>
        <section className="painel"><h2>Por que pulamos</h2>
          {pulos.length ? <Barras linhas={pulos.map(m => ({ rotulo: m.motivo, valor: m.total }))} cor="var(--e-pulada)" /> : <p className="mudo">Nenhuma vaga pulada.</p>}
          <p className="pequeno mudo">Se um motivo domina, talvez valha ajustar o perfil (ex.: inglês) ou os filtros de busca.</p>
        </section>
      </div>
    </div>
  );
}
