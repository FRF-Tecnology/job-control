import { useEffect, useRef, useState } from 'react';
import { useApp, type App } from '../app/contexto';
import { plural } from '../util';

type Balanco = { novas: number; encontradas: number; grupos: Record<string, number>; puladas: number; pendentes: number; avisos: string[] };
let emCurso: Promise<Balanco> | null = null;

// A busca pertence à rodada. Trocar de tela não dispara outra chamada ao provedor.
function iniciar(app: App): Promise<Balanco> {
  if (!emCurso) {
    emCurso = app.api('/api/v1/busca', 'POST', {}).then(async (r: Balanco) => {
      await app.recarregar();
      return r;
    }).finally(() => { emCurso = null; });
  }
  return emCurso;
}

export function BuscaDoPerfil({ radar = false }: { radar?: boolean }) {
  const app = useApp();
  const [ocupado, ocupar] = useState(false);
  const [balanco, informar] = useState<Balanco | null>(null);
  const [erro, falhar] = useState('');
  const montado = useRef(false);
  const atual = useRef(app);
  atual.current = app;
  async function acompanhar(pedido: Promise<Balanco>) {
    ocupar(true); falhar(''); informar(null);
    try {
      const r = await pedido;
      if (montado.current) informar(r);
    } catch (e) {
      if (montado.current) falhar(e instanceof Error ? e.message : 'Não foi possível concluir a busca.');
    } finally { if (montado.current) ocupar(false); }
  }
  useEffect(() => {
    montado.current = true;
    function pedidoDaRota() {
      if (!radar && location.hash === '#/triagem?buscar=1') {
        location.replace('#/radar?buscar=1');
        return;
      }
      if (!radar || location.hash !== '#/radar?buscar=1') return;
      history.replaceState(history.state, '', '#/radar');
      void acompanhar(iniciar(atual.current));
    }
    if (emCurso) void acompanhar(emCurso);
    pedidoDaRota();
    window.addEventListener('hashchange', pedidoDaRota);
    return () => {
      montado.current = false;
      window.removeEventListener('hashchange', pedidoDaRota);
    };
  }, [radar]);
  const pendentes = balanco?.pendentes ?? (radar && app.estado.vagas?.some(v => v.triagemBusca === 'pendente'));
  const ultima = radar && app.estado.rodadas?.[app.estado.rodadas.length - 1];
  const acao = erro ? 'Tentar buscar novamente' : pendentes ? 'Retomar avaliação das vagas' : radar ? 'Buscar vagas' : 'Buscar vagas com meu perfil';
  return <section aria-label="Busca pelo perfil" className="busca-perfil">
    <div className="acoes">
      <button type="button" className="botao" disabled={ocupado} aria-busy={ocupado} onClick={() => void acompanhar(iniciar(app))}>
        {ocupado ? 'Buscando e avaliando…' : acao}
      </button>
      {!radar && <a className="botao-fantasma" href="#/perfil">Editar perfil</a>}
    </div>
    <p className="pequeno mudo">A IA avalia até oito vagas por busca, em grupos de afinidade com seu perfil.</p>
    <div role="status">
      {ocupado && <p>Consultando as fontes e classificando as vagas. Você pode continuar usando o app.</p>}
      {erro && <p>{erro}</p>}
      {!ocupado && !erro && !balanco && radar && (ultima
        ? <p><strong>Última busca</strong> · {plural(ultima.encontradas, 'vaga encontrada', 'vagas encontradas')}. {ultima.resumo}</p>
        : <p>Inicie uma busca para consultar as fontes com seu perfil salvo.</p>)}
      {balanco && <>
        <p><strong>Resultado da busca</strong><br />{plural(balanco.encontradas, 'vaga encontrada', 'vagas encontradas')} nas fontes · {plural(balanco.novas, 'nova', 'novas')}.</p>
        {radar && balanco.encontradas === 0 && balanco.avisos.length === 0 && <p>Nenhuma vaga com estes critérios nas fontes consultadas. Você pode editar o perfil ou buscar novamente.</p>}
        <ul className="balanco-grupos" aria-label="Classificação desta busca">
          {(['A', 'B', 'C'] as const).map(grupo => <li className={`chip ader-${grupo}`} key={grupo}>Grupo {grupo} · {balanco.grupos[grupo]}</li>)}
          <li className="chip contorno">{plural(balanco.puladas, 'pulada', 'puladas')}</li>
          <li className="chip contorno">{plural(balanco.pendentes, 'pendente', 'pendentes')}</li>
        </ul>
        {balanco.avisos.map(msg => <p key={msg}>{msg}</p>)}
      </>}
    </div>
    {(erro || Boolean(pendentes)) && <p className="pequeno"><a href={radar ? '#/ia' : '#/ia?primeira-busca=1'}>Ver conexão da IA</a></p>}
  </section>;
}
