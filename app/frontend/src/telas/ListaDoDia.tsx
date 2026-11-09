// Lista do dia (jc-a2y.6.13, decisão do Vini: aprovar em lote, 1 clique por dia). Mostra as candidaturas preparadas
// cuja impressão confere NESTA tela, até o que resta do limite de hoje em cada plataforma, com o resumo de cada pacote
// e a opção de tirar itens. "Aprovar a lista do dia" manda cada item com a SUA impressão, a calculada aqui (M2: o
// servidor confere uma por uma). RASCUNHO: rota e resposta supostas até o contrato do Faker:
// POST /api/v1/candidaturas/aprovar-lote {itens: [{id, impressao}]} → {aprovadas: [id], recusadas: [{id, motivo}]}.
import { useMemo, useState } from 'react';
import { ErroApi, useApp } from '../app/contexto';

type Cand = {
  id: string; plataforma: string; estado: string; impressao: string; criadaEm: string;
  conteudo: { curriculo: { arquivo: string }; contato?: object | null; pretensao?: { valor?: number; moeda?: string; tipo?: string } | null; respostas: unknown[]; mensagem?: string | null };
};
type Limite = { limite: number; usadas: number };

// Motivos de recusa por item (lista fechada; desconhecido vira "mudou").
const MOTIVO: Record<string, string> = {
  'impressao-mudou': 'o pacote mudou; revise de novo',
  limite: 'passou do limite de hoje da plataforma',
  estado: 'não estava mais para revisar',
  versao: 'o pacote mudou; revise de novo',
};

export function ListaDoDia({ candidaturas, calculadas, limites, nome, plataforma, onFeito }: {
  candidaturas: Cand[]; calculadas: Record<string, string>; limites: Record<string, Limite>;
  nome: (c: Cand) => string; plataforma: (p: string) => string; onFeito: () => void;
}) {
  const app = useApp();
  const [tiradas, setTiradas] = useState<Set<string>>(new Set());
  const [ocupado, setOcupado] = useState(false);

  const { cabem, ficam, foraDaConferencia } = useMemo(() => {
    const preparadas = candidaturas.filter(c => c.estado === 'preparada').sort((a, b) => a.criadaEm.localeCompare(b.criadaEm));
    const conferidas = preparadas.filter(c => calculadas[c.id] && calculadas[c.id] === c.impressao);
    // O que resta hoje em cada plataforma: limite − já enviadas hoje − já aprovadas (que ainda vão sair).
    const resta: Record<string, number> = {};
    for (const [p, l] of Object.entries(limites)) resta[p] = Math.max(0, l.limite - l.usadas - candidaturas.filter(c => c.plataforma === p && c.estado === 'aprovada').length);
    const cabem: Cand[] = [], ficam: Cand[] = [];
    for (const c of conferidas) {
      if (resta[c.plataforma] === undefined || resta[c.plataforma] > 0) { cabem.push(c); if (resta[c.plataforma] !== undefined) resta[c.plataforma]--; }
      else ficam.push(c);
    }
    return { cabem, ficam, foraDaConferencia: preparadas.length - conferidas.length };
  }, [candidaturas, calculadas, limites]);

  if (!cabem.length && !ficam.length) return null; // nada preparado e conferido: o painel de envio já diz o que fazer
  const escolhidas = cabem.filter(c => !tiradas.has(c.id));

  const alternar = (id: string) => setTiradas(t => { const n = new Set(t); if (n.has(id)) n.delete(id); else n.add(id); return n; });

  async function aprovar() {
    const lista = escolhidas.map(c => ({ c, impressao: calculadas[c.id] })).filter(x => x.impressao && x.impressao === x.c.impressao);
    if (!lista.length) return;
    const ok = await app.perguntar({
      titulo: lista.length === 1 ? 'Aprovar 1 candidatura hoje?' : `Aprovar ${lista.length} candidaturas hoje?`,
      texto: 'Cada uma é enviada uma vez, hoje, pelo app, com o pacote que você revisou. Editar um pacote depois desfaz só a aprovação dele.',
      itens: lista.map(x => nome(x.c)),
      confirmar: 'Aprovar a lista',
    });
    if (!ok) return;
    setOcupado(true);
    try {
      const r: { aprovadas?: string[]; recusadas?: { id: string; motivo?: string }[] } =
        await app.api('/api/v1/candidaturas/aprovar-lote', 'POST', { itens: lista.map(x => ({ id: x.c.id, impressao: x.impressao })) });
      const n = r.aprovadas?.length ?? 0;
      app.aviso(n === 1 ? '1 candidatura aprovada. O app envia hoje, uma vez.' : `${n} candidaturas aprovadas. O app envia hoje, uma vez cada.`);
      for (const x of r.recusadas ?? []) {
        const c = lista.find(i => i.c.id === x.id)?.c;
        app.aviso(`${c ? nome(c) : 'Uma candidatura'} ficou de fora: ${MOTIVO[x.motivo ?? ''] ?? MOTIVO['impressao-mudou']}.`, true);
      }
      setTiradas(new Set());
      onFeito();
    } catch (e) {
      app.aviso(e instanceof ErroApi && e.status === 403 ? 'Só você pode aprovar a lista, por esta tela.' : 'Não deu para aprovar a lista. Tente de novo.', true);
    } finally { setOcupado(false); }
  }

  const resumo = (c: Cand) => {
    const k = c.conteudo;
    const pret = k.pretensao?.valor ? ` · pretensão ${k.pretensao.tipo ?? ''}`.trimEnd() : '';
    return `${plataforma(c.plataforma)} · ${k.curriculo.arquivo} · ${k.respostas.length} ${k.respostas.length === 1 ? 'resposta' : 'respostas'}${pret}${k.mensagem ? ' · com mensagem' : ''}`;
  };

  return (
    <section className="lista-do-dia" aria-labelledby="lista-do-dia-titulo">
      <div className="lista-topo">
        <div>
          <h2 id="lista-do-dia-titulo">Lista do dia</h2>
          <p className="mudo">Revise o resumo, tire o que não quiser e aprove tudo de uma vez. Cada pacote abaixo, na fila, mostra o conteúdo inteiro.</p>
        </div>
        <button className="botao" type="button" disabled={ocupado || !escolhidas.length} onClick={aprovar}>
          {`Aprovar a lista do dia (${escolhidas.length})`}
        </button>
      </div>
      {cabem.length > 0 && (
        <ul className="lista-itens">
          {cabem.map(c => (
            <li key={c.id}>
              <label>
                <input type="checkbox" checked={!tiradas.has(c.id)} onChange={() => alternar(c.id)} />
                <span><strong>{nome(c)}</strong><span className="pequeno mudo">{resumo(c)}</span></span>
              </label>
            </li>
          ))}
        </ul>
      )}
      {ficam.length > 0 && <p className="pequeno mudo">{`${ficam.length === 1 ? '1 pacote fica' : `${ficam.length} pacotes ficam`} para amanhã: o limite de hoje da plataforma já está cheio.`}</p>}
      {foraDaConferencia > 0 && <p className="pequeno mudo">{`${foraDaConferencia === 1 ? '1 pacote está' : `${foraDaConferencia} pacotes estão`} fora da lista: ainda conferindo, ou a impressão não bate (veja na fila).`}</p>}
    </section>
  );
}
