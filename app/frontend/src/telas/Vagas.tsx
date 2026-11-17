// Vagas: tabela com busca e filtros (porte de web/telas/vagas.mjs).
import { useEffect, useRef, useState } from 'react';
import { useApp, useTrava, type App } from '../app/contexto';
import { ETAPAS, NOMES_ETAPA, normalizar } from '../dominio/modelo';
import { Convite, EtapaChip } from '../componentes';
import { contem, dataCurta, nomePlataforma } from '../util';

// 'Contractor USD', 'PJ ou CLT'…: o contrato vem em texto livre, então o filtro procura a palavra.
const temContrato = (v: any, c: string) => normalizar(v.contrato ?? '').split(/[^a-z0-9]+/).includes(normalizar(c));

const POR_PAGINA = 50;
type Filtros = Record<string, string>;
// Filtros ficam guardados entre visitas à tela (até fechar o app).
const lerFiltros = (): Filtros => { try { return JSON.parse(sessionStorage.getItem('filtros-vagas') ?? '{}') ?? {}; } catch { return {}; } };
const guardar = (f: Filtros) => { try { sessionStorage.setItem('filtros-vagas', JSON.stringify(f)); } catch { /* sem armazenamento: vale até trocar de tela */ } };
let paginaGuardada = 0;

export function TelaVagas() {
  const app = useApp();
  const trava = useTrava();
  const { vagas, plataformas } = app.estado;
  const [filtros, setFiltros] = useState<Filtros>(lerFiltros);
  const [pagina, setPagina] = useState(paginaGuardada);
  const [busca, setBusca] = useState(filtros.texto ?? '');
  const espera = useRef<ReturnType<typeof setTimeout>>();
  const plataformasUsadas = [...new Set(vagas.map((v: any) => v.plataforma))];

  const mudar = (k: string, val: string) => { const f = { ...filtros, [k]: val }; setFiltros(f); guardar(f); setPagina(0); };
  useEffect(() => { paginaGuardada = pagina; }, [pagina]);

  const lista = vagas
    .filter((v: any) => contem(v, filtros.texto ?? '') && (!filtros.etapa || v.etapa === filtros.etapa) && (!filtros.plataforma || v.plataforma === filtros.plataforma) && (!filtros.aderencia || v.aderencia === filtros.aderencia) && (!filtros.contrato || temContrato(v, filtros.contrato)))
    .sort((a: any, b: any) => (b.envio?.data ?? b.descobertaEm ?? '').localeCompare(a.envio?.data ?? a.descobertaEm ?? ''));
  const paginas = Math.max(1, Math.ceil(lista.length / POR_PAGINA));
  const p = Math.min(pagina, paginas - 1);
  const visiveis = lista.slice(p * POR_PAGINA, (p + 1) * POR_PAGINA);

  const sel = (k: string, rotulo: string, opcoes: [string, string][]) => (
    <select aria-label={rotulo} id={`filtro-${k}`} value={filtros[k] ?? ''} onChange={ev => mudar(k, ev.target.value)}>
      <option value="">{rotulo}</option>
      {opcoes.map(([v, t]) => <option key={v} value={v}>{t}</option>)}
    </select>
  );

  return (
    <div>
      <header className="cabecalho">
        <div><h1>Vagas</h1><p>{`${lista.length} de ${vagas.length}`}</p></div>
        <div className="acoes">
          <button className="botao-fantasma" type="button" {...trava()} onClick={() => novaVaga(app)}>Adicionar vaga</button>
          <a className="botao-fantasma" href="/api/v1/exportar.csv">Exportar CSV</a>
        </div>
      </header>
      <div className="filtros">
        <input id="busca-vagas" type="search" placeholder="Buscar empresa, vaga ou nota" aria-label="Buscar" value={busca}
          onChange={ev => { const t = ev.target.value; setBusca(t); clearTimeout(espera.current); espera.current = setTimeout(() => mudar('texto', t), 200); }} />
        {sel('etapa', 'Todas as etapas', ETAPAS.map(e => [e, (NOMES_ETAPA as Record<string, string>)[e]]))}
        {sel('plataforma', 'Todas as plataformas', plataformasUsadas.map((pl: any) => [pl, nomePlataforma(pl, plataformas)]))}
        {sel('aderencia', 'Qualquer aderência', [['A', 'Grupo A'], ['B', 'Grupo B'], ['C', 'Grupo C']])}
        {sel('contrato', 'Qualquer contrato', [['CLT', 'CLT'], ['PJ', 'PJ'], ['Contractor', 'Contractor (exterior)'], ['Cooperado', 'Cooperado']])}
      </div>
      {lista.length ? (
        <div className="tabela-rolagem">
          <table>
            <thead><tr>{['Data', 'Empresa', 'Vaga', 'Etapa', 'Plataforma', 'Grupo', 'Observação'].map(t => <th key={t} scope="col">{t}</th>)}</tr></thead>
            <tbody>
              {visiveis.map((v: any) => {
                const obs = v.pendencia || v.motivo || v.notas || v.motivoClaude || '';
                return (
                  <tr key={v.id} tabIndex={0} onClick={() => app.abrirVaga(v.id)} onKeyDown={ev => { if (ev.key === 'Enter') app.abrirVaga(v.id); }}>
                    <td className="num">{dataCurta(v.envio?.data ?? v.descobertaEm)}</td>
                    <td><b>{v.empresa}</b></td>
                    <td>{v.titulo}</td>
                    <td><EtapaChip etapa={v.etapa} /></td>
                    <td>{nomePlataforma(v.plataforma, plataformas)}</td>
                    <td>{v.aderencia ? <span className={`chip ader-${v.aderencia}`}>{v.aderencia}</span> : ''}</td>
                    <td className="corta" title={obs}>{obs}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      ) : (
        <Convite titulo="Nada encontrado" texto="Limpe os filtros ou busque outro termo.">
          <button className="botao-fantasma" type="button" onClick={() => { setFiltros({}); guardar({}); setBusca(''); }}>Limpar filtros</button>
        </Convite>
      )}
      {paginas > 1 && (
        <div className="paginacao">
          <button className="botao-fantasma" type="button" disabled={p === 0} onClick={() => setPagina(p - 1)}>Anterior</button>
          <span className="mudo">{`Página ${p + 1} de ${paginas}`}</span>
          <button className="botao-fantasma" type="button" disabled={p >= paginas - 1} onClick={() => setPagina(p + 1)}>Próxima</button>
        </div>
      )}
    </div>
  );
}

async function novaVaga(app: App) {
  const link = await app.perguntar({ titulo: 'Adicionar vaga', texto: 'Cole o link da vaga. Depois preencha empresa e título.', campo: 'https://…' });
  if (!link) return;
  const empresa = await app.perguntar({ titulo: 'Empresa', campo: 'Nome da empresa' });
  if (!empresa) return;
  const titulo = await app.perguntar({ titulo: 'Título da vaga', campo: 'ex.: Desenvolvedor Java Pleno' });
  if (!titulo) return;
  const id = `ext-${empresa.toLowerCase().normalize('NFD').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')}-${Date.now().toString(36)}`;
  let plataforma = 'externo';
  if (/linkedin\.com/.test(link)) plataforma = 'linkedin';
  else if (/gupy\.io/.test(link)) plataforma = 'gupy';
  else if (/inhire\.app/.test(link)) plataforma = 'inhire';
  const ok = await app.agir(() => app.api('/api/v1/vagas', 'POST', { id, empresa, titulo, link, plataforma }), 'Vaga adicionada em Descoberta');
  if (ok) app.abrirVaga(id);
}
