import { useEffect, useRef, useState } from 'react';
import { useApp, type App } from '../app/contexto';
import { plural } from '../util';

type Balanco = { novas: number; encontradas: number; grupos: Record<string, number>; puladas: number; pendentes: number; avisos: string[] };
const emCurso = new Map<string, Promise<Balanco>>();
/** De quanto em quanto tempo a tela relê as vagas durante a busca: a triagem grava uma vaga por vez. */
export const intervaloDaBusca = 4000;

function iniciar(app: App, perfil: string): Promise<Balanco> {
  const existente = emCurso.get(perfil);
  if (existente) return existente;
  // A rodada pode levar minutos (até 10 triagens da IA, uma por vez). Relendo o estado, as vagas já avaliadas
  // aparecem no Radar sem esperar o fim.
  const relogio = setInterval(() => { void app.recarregar().catch(() => { /* a próxima volta tenta de novo */ }); }, intervaloDaBusca);
  const pedido = app.api('/api/v1/busca', 'POST', { perfilBuscaId: perfil }).then(async (r: Balanco) => {
    await app.recarregar();
    return r;
  }).finally(() => { clearInterval(relogio); emCurso.delete(perfil); });
  emCurso.set(perfil, pedido);
  return pedido;
}

export function useBuscaDoPerfil(radar: boolean) {
  const app = useApp();
  const perfil = app.estado?.perfilBuscaAtivo ?? 'original';
  const [ocupado, ocupar] = useState(false);
  const [balanco, informar] = useState<Balanco | null>(null);
  const [erro, falhar] = useState('');
  const ativo = useRef<string | null>(null);
  const atual = useRef(app);
  atual.current = app;

  async function acompanhar(pedido: Promise<Balanco>) {
    ocupar(true); falhar(''); informar(null);
    try {
      const r = await pedido;
      if (ativo.current === perfil) informar(r);
    } catch (e) {
      if (ativo.current === perfil) falhar(e instanceof Error ? e.message : 'Não foi possível concluir a busca.');
    } finally { if (ativo.current === perfil) ocupar(false); }
  }

  useEffect(() => {
    ativo.current = perfil;
    informar(null); falhar(''); ocupar(false);
    function pedidoDaRota() {
      if (!radar && location.hash === '#/triagem?buscar=1') {
        location.replace('#/radar?buscar=1');
        return;
      }
      if (!radar || location.hash !== '#/radar?buscar=1') return;
      history.replaceState(history.state, '', '#/radar');
      void acompanhar(iniciar(atual.current, perfil));
    }
    const pedido = emCurso.get(perfil);
    if (pedido) void acompanhar(pedido);
    pedidoDaRota();
    window.addEventListener('hashchange', pedidoDaRota);
    return () => {
      ativo.current = null;
      window.removeEventListener('hashchange', pedidoDaRota);
    };
  }, [radar, perfil]);

  const pendentes = balanco?.pendentes ?? (radar && app.estado.vagas?.some(v =>
    (v.perfilBuscaId ?? 'original') === perfil && v.etapa === 'descoberta' && v.triagemBusca === 'pendente'));
  return { buscar: () => void acompanhar(iniciar(app, perfil)), ocupado, balanco, erro, pendentes };
}

export function BalancoDaBusca({ balanco }: { balanco: Balanco }) {
  return <div className="resultado-busca">
    <p>{plural(balanco.encontradas, 'vaga encontrada', 'vagas encontradas')} nas fontes · {plural(balanco.novas, 'nova', 'novas')}.</p>
    <ul className="balanco-grupos" aria-label="Classificação desta busca">
      {(['A', 'B', 'C'] as const).map(grupo => <li className={`chip ader-${grupo}`} key={grupo}>Grupo {grupo} · {balanco.grupos[grupo]}</li>)}
      <li className="chip contorno">{plural(balanco.puladas, 'pulada', 'puladas')}</li>
      <li className="chip contorno">{plural(balanco.pendentes, 'pendente', 'pendentes')}</li>
    </ul>
    {balanco.avisos.map(msg => <p key={msg}>{msg}</p>)}
  </div>;
}

export function BuscaDoPerfil() {
  const { buscar, ocupado, balanco, erro, pendentes } = useBuscaDoPerfil(false);
  const acao = erro ? 'Tentar buscar novamente' : pendentes ? 'Retomar avaliação das vagas' : 'Buscar vagas com meu perfil';
  return <section aria-label="Busca pelo perfil" className="busca-perfil">
    <div className="acoes">
      <button type="button" className="botao" disabled={ocupado} aria-busy={ocupado} onClick={buscar}>
        {ocupado ? 'Buscando e avaliando…' : acao}
      </button>
      <a className="botao-fantasma" href="#/perfil">Editar perfil</a>
    </div>
    <p className="pequeno mudo">A IA avalia até dez vagas por rodada. Você pode retomar as avaliações pendentes.</p>
    <div role="status">
      {ocupado && <p>Consultando as fontes e classificando as vagas. Você pode continuar usando o app.</p>}
      {erro && <p>{erro}</p>}
      {balanco && <><strong>Resultado da busca</strong><BalancoDaBusca balanco={balanco} /></>}
    </div>
    {(erro || Boolean(pendentes)) && <p className="pequeno"><a href="#/ia?primeira-busca=1">Ver conexão da IA</a></p>}
  </section>;
}
