// Plataformas: onde procurar vagas e como cada uma funciona (porte de web/telas/plataformas.mjs).
import { useApp, useTrava } from '../app/contexto';
import { porPlataforma } from '../dominio/funil';
import { Convite, LinkExterno } from '../componentes';
import { dataCurta, encode } from '../util';

const STATUS: Record<string, string> = { automatizada: 'Automatizada', manual: 'Manual', 'a-testar': 'A testar', descartada: 'Descartada' };

export function TelaPlataformas() {
  const app = useApp();
  const trava = useTrava();
  const { plataformas, vagas } = app.estado;
  const nums = new Map(porPlataforma(vagas).map((p: any) => [p.plataforma, p]));
  const ordem = Object.keys(STATUS);
  const lista = [...plataformas].sort((a: any, b: any) => ordem.indexOf(a.status) - ordem.indexOf(b.status) || String(a.nome ?? a.id).localeCompare(String(b.nome ?? b.id)));
  const salvar = (p: any, patch: object, msg: string) => trava.travado ? (app.aviso(trava.frase, true), Promise.resolve(false)) : app.agir(() => app.api(`/api/v1/plataformas/${encode(p.id)}`, 'PUT', patch), msg);

  return (
    <div>
      <header className="cabecalho">
        <div><h1>Plataformas</h1><p>Onde procurar vagas e como cada uma funciona. Teste uma fonte nova por rodada e anote aqui o que descobrir.</p></div>
        <button className="botao-fantasma" type="button" {...trava()} onClick={async () => {
          const nome = await app.perguntar({ titulo: 'Nova plataforma', campo: 'Nome (ex.: Remotar)' });
          if (!nome) return;
          const url = await app.perguntar({ titulo: `Endereço de ${nome}`, campo: 'https://…' });
          const id = nome.toLowerCase().normalize('NFD').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
          salvar({ id }, { nome, url: url ?? '', status: 'a-testar', comoAplicar: '', limitacoes: '' }, 'Plataforma adicionada');
        }}>Adicionar plataforma</button>
      </header>
      {plataformas.length ? (
        <div className="plataformas">
          {lista.map((p: any) => {
            const n: any = nums.get(p.id);
            return (
              <article key={p.id} className={`plataforma s-${p.status}`}>
                <div className="acoes" style={{ justifyContent: 'space-between', alignItems: 'baseline' }}>
                  <h2><LinkExterno url={p.url} semLink={p.nome ?? p.id}>{p.nome ?? p.id}</LinkExterno></h2>
                  <select className="situacao" aria-label={`Situação de ${p.nome}`} {...trava()} value={p.status ?? ''}
                    onChange={ev => salvar(p, { status: ev.target.value }, `${p.nome}: ${STATUS[ev.target.value]}`)}>
                    {!STATUS[p.status] && <option value={p.status ?? ''} />}
                    {Object.entries(STATUS).map(([k, t]) => <option key={k} value={k}>{t}</option>)}
                  </select>
                </div>
                {n ? <p className="num" style={{ margin: 0 }}>{`${n.achadas} achadas · ${n.enviadas} enviadas · ${n.retornos} retornos`}</p> : <p className="pequeno mudo" style={{ margin: 0 }}>Nenhuma vaga daqui ainda.</p>}
                {p.ultimaVarredura && <p className="pequeno mudo" style={{ margin: 0 }}>{`Última busca: ${dataCurta(p.ultimaVarredura)}`}</p>}
                <CampoTexto p={p} k="comoAplicar" rotulo="Como aplicar" salvar={salvar} />
                <CampoTexto p={p} k="limitacoes" rotulo="Limitações" salvar={salvar} />
              </article>
            );
          })}
        </div>
      ) : <Convite titulo="Nenhuma plataforma cadastrada" texto="Adicione uma plataforma aqui." />}
    </div>
  );
}

// Texto livre que salva ao sair do campo. Recomeça do salvo quando o texto muda por fora.
function CampoTexto({ p, k, rotulo, salvar }: { p: any; k: string; rotulo: string; salvar: (p: any, patch: object, msg: string) => void }) {
  return (
    <label className="campo"><span>{rotulo}</span>
      <textarea key={p[k] ?? ''} id={`pl-${p.id}-${k}`} style={{ minHeight: '58px' }} defaultValue={p[k] ?? ''}
        placeholder={k === 'comoAplicar' ? 'Passo a passo, filtros que funcionam…' : 'Pede conta? CPF? Captcha?'}
        onBlur={ev => { if (ev.target.value !== (p[k] ?? '')) salvar(p, { [k]: ev.target.value }, 'Salvo'); }} />
    </label>
  );
}
