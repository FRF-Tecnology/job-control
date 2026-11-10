// Rodadas: cada vez que o Claude busca vagas (porte de web/telas/rodadas.mjs). O envio agora é pelo app (jc-a2y.56).
import { useApp } from '../app/contexto';
import { Convite } from '../componentes';
import { dataCurta } from '../util';

export function TelaRodadas() {
  const app = useApp();
  const { rodadas, vagas } = app.estado;
  // Também mostra rodadas que só existem nos envios (histórico migrado).
  const porN = new Map<any, any>();
  for (const v of vagas.filter((v: any) => v.envio?.rodada != null)) {
    const n = v.envio.rodada;
    const g = porN.get(n) ?? { n, data: v.envio.data, enviadas: 0, plataformas: new Set() };
    g.enviadas++;
    g.plataformas.add(v.plataforma);
    if (v.envio.data < g.data) g.data = v.envio.data;
    porN.set(n, g);
  }
  for (const r of rodadas) porN.set(r.n, { ...porN.get(r.n), ...r, plataformas: new Set([...(porN.get(r.n)?.plataformas ?? []), ...(r.fontes ?? [])]) });
  const lista = [...porN.values()].sort((a, b) => b.n - a.n);

  return (
    <div>
      <header className="cabecalho"><div><h1>Rodadas</h1><p>Cada vez que o Claude busca vagas, é uma rodada. Peça uma nova com /rodada.</p></div>
        <a className="botao-fantasma" href="/api/v1/historico.md" target="_blank">Ver histórico em texto</a></header>
      {lista.length ? (
        <div className="tabela-rolagem">
          <table>
            <thead><tr>{['Rodada', 'Data', 'Fontes', 'Encontradas', 'Enviadas', 'Puladas', 'Travadas', 'Resumo'].map(t => <th key={t} scope="col">{t}</th>)}</tr></thead>
            <tbody>
              {lista.map(r => (
                <tr key={String(r.n)} style={{ cursor: 'default' }}>
                  <td className="num">{String(r.n)}</td><td>{dataCurta(r.data)}</td><td>{[...r.plataformas].join(', ')}</td>
                  <td className="num">{r.encontradas ?? '–'}</td><td className="num">{String(r.enviadas ?? 0)}</td><td className="num">{r.puladas ?? '–'}</td><td className="num">{r.bloqueadas ?? '–'}</td>
                  <td>{r.resumo ?? ''}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : <Convite titulo="Nenhuma rodada ainda" texto="No Claude Code, dentro desta pasta, digite /rodada." />}
    </div>
  );
}
