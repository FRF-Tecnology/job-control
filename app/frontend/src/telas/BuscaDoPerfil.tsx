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

export function BuscaDoPerfil() {
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
      if (location.hash !== '#/triagem?buscar=1') return;
      history.replaceState(history.state, '', '#/triagem');
      void acompanhar(iniciar(atual.current));
    }
    if (location.hash === '#/triagem?buscar=1') pedidoDaRota();
    else if (emCurso) void acompanhar(emCurso);
    window.addEventListener('hashchange', pedidoDaRota);
    return () => {
      montado.current = false;
      window.removeEventListener('hashchange', pedidoDaRota);
    };
  }, []);
  const acao = erro ? 'Tentar buscar novamente' : balanco?.pendentes ? 'Retomar avaliação das vagas' : 'Buscar vagas com meu perfil';
  return <section aria-label="Busca pelo perfil" className="busca-perfil">
    <div className="acoes">
      <button type="button" className="botao" disabled={ocupado} aria-busy={ocupado} onClick={() => void acompanhar(iniciar(app))}>
        {ocupado ? 'Buscando e avaliando…' : acao}
      </button>
      <a className="botao-fantasma" href="#/perfil">Editar perfil</a>
    </div>
    <p className="pequeno mudo">A IA avalia até oito vagas por busca. Os cartões aparecem em ordem de afinidade com o perfil.</p>
    <div role="status">
      {ocupado && <p>Consultando as fontes e classificando as vagas. Você pode continuar usando o app.</p>}
      {erro && <p>{erro}</p>}
      {balanco && <>
        <p><strong>Resultado da busca</strong><br />{plural(balanco.encontradas, 'vaga encontrada', 'vagas encontradas')} nas fontes · {plural(balanco.novas, 'nova', 'novas')}.</p>
        <ul className="balanco-grupos" aria-label="Classificação desta busca">
          {(['A', 'B', 'C'] as const).map(grupo => <li className={`chip ader-${grupo}`} key={grupo}>Grupo {grupo} · {balanco.grupos[grupo]}</li>)}
          <li className="chip contorno">{plural(balanco.puladas, 'pulada', 'puladas')}</li>
          <li className="chip contorno">{plural(balanco.pendentes, 'pendente', 'pendentes')}</li>
        </ul>
        {balanco.avisos.map(msg => <p key={msg}>{msg}</p>)}
      </>}
    </div>
    {(erro || Boolean(balanco?.pendentes)) && <p className="pequeno"><a href="#/ia?primeira-busca=1">Ver conexão da IA</a></p>}
  </section>;
}
