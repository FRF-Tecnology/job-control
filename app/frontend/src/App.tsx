// Casca da interface: estado, rotas, atualização automática, avisos e diálogos (porte de web/app.mjs).
import { Component, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Contexto, chamarApi, ErroApi, type App as AppApi, type Estado, type Pergunta } from './app/contexto';
import { resumo } from './dominio/funil';
import { Icone } from './componentes';
import { encode } from './util';
import { Dialogo, type PedidoDialogo } from './app/Dialogo';
import { Detalhe } from './telas/Detalhe';
import { TelaHoje } from './telas/Hoje';
import { TelaTriagem } from './telas/Triagem';
import { TelaFila } from './telas/Fila';
import { TelaQuadro } from './telas/Quadro';
import { TelaVagas } from './telas/Vagas';
import { TelaNumeros } from './telas/Numeros';
import { TelaPlataformas } from './telas/Plataformas';
import { TelaRodadas } from './telas/Rodadas';
import { TelaPerfil } from './telas/Perfil';
import { TelaConectarIA } from './telas/ConectarIA';

// aba: aparece na barra de baixo do celular (as outras ficam em Mais).
const ICONES: Record<string, string> = {
  hoje: 'M4 11l8-7 8 7v9a1 1 0 0 1-1 1h-4v-6H9v6H5a1 1 0 0 1-1-1z',
  triagem: 'M5 6h14v12H5zM9 6V4h6v2M9 12l2 2 4-4',
  vagas: 'M3 8h18v12H3zM9 8V5a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v3',
  mais: 'M5 12h.01M12 12h.01M19 12h.01',
};
type R = ReturnType<typeof resumo>;
const TELAS: { rota: string; nome: string; Tela: () => ReactNode; contador?: (r: R) => number; alerta?: boolean; aba?: boolean }[] = [
  { rota: 'hoje', nome: 'Hoje', Tela: TelaHoje, contador: r => r.bloqueadas + r.followUps, alerta: true, aba: true },
  { rota: 'triagem', nome: 'Triagem', Tela: TelaTriagem, contador: r => r.descoberta, aba: true },
  { rota: 'fila', nome: 'Fila', Tela: TelaFila },
  { rota: 'quadro', nome: 'Quadro', Tela: TelaQuadro },
  { rota: 'vagas', nome: 'Vagas', Tela: TelaVagas, aba: true },
  { rota: 'numeros', nome: 'Números', Tela: TelaNumeros },
  { rota: 'plataformas', nome: 'Plataformas', Tela: TelaPlataformas },
  { rota: 'rodadas', nome: 'Rodadas', Tela: TelaRodadas },
  { rota: 'perfil', nome: 'Perfil', Tela: TelaPerfil }, // no menu, "Perfil"; o título da tela segue "Perfil e regras"
  { rota: 'ia', nome: 'Conectar IA', Tela: TelaConectarIA },
];

function lerHash() {
  const [, rota = 'hoje', vaga] = location.hash.split('/');
  const tela = TELAS.find(t => t.rota === rota) ?? TELAS[0];
  let id: string | null = null;
  try { id = vaga ? decodeURIComponent(vaga) : null; } catch { id = null; }
  return { tela, vaga: id };
}

function useHash() {
  const [hash, setHash] = useState(location.hash);
  useEffect(() => {
    const ao = () => setHash(location.hash);
    window.addEventListener('hashchange', ao);
    return () => window.removeEventListener('hashchange', ao);
  }, []);
  return useMemo(() => lerHash(), [hash]);
}

type Aviso = { id: number; texto: string; erro: boolean };
type Falha = { titulo: string; texto: string } | null;

// Uma tela que quebrar não derruba o app: mostra o aviso e deixa trocar de seção.
class Protecao extends Component<{ children: ReactNode; chave: string }, { erro: Error | null }> {
  state = { erro: null as Error | null };
  static getDerivedStateFromError(erro: Error) { return { erro }; }
  componentDidUpdate(antes: { chave: string }) { if (antes.chave !== this.props.chave && this.state.erro) this.setState({ erro: null }); }
  render() {
    if (this.state.erro) return <div className="convite"><div><strong>Esta tela não conseguiu ser desenhada</strong><p>{this.state.erro.message}. Os dados estão salvos; tente outra seção ou peça ao Claude para conferir.</p></div></div>;
    return this.props.children;
  }
}

export default function App() {
  const { tela, vaga } = useHash();
  const [estado, setEstado] = useState<Estado | null>(null);
  const [conexao, setConexao] = useState<{ texto: string; estado: 'ok' | 'off' | '' }>({ texto: '', estado: '' });
  const [falha, setFalha] = useState<Falha>(null);
  const [avisos, setAvisos] = useState<Aviso[]>([]);
  const [dialogo, setDialogo] = useState<PedidoDialogo | null>(null);
  const versao = useRef<string | null>(null);
  const fechando = useRef(false); // 503 depois de apagar os dados (jc-a2y.49)
  const temEstado = useRef(false);
  // Há texto digitado e ainda não salvo (perfil, nota nova, notas da vaga…)? Enquanto houver, nada é redesenhado.
  const sujo = useRef(false);

  const aviso = useCallback((texto: string, erro = false) => {
    const id = Date.now() + Math.random();
    setAvisos(a => [...a, { id, texto, erro }]);
    setTimeout(() => setAvisos(a => a.filter(x => x.id !== id)), erro ? 7000 : 3200);
  }, []);

  const perguntar = useCallback((p: Pergunta) => new Promise<string | null>(resolver => setDialogo({ tipo: 'pergunta', pergunta: p, resolver })), []);

  const carregar = useCallback(async (forcar = false) => {
    // Depois de "Apagar meus dados", o servidor responde 503 a tudo até o app sair: não adianta tentar de novo.
    if (fechando.current) return;
    let novo: Estado;
    try {
      if (!forcar && temEstado.current) {
        const { versao: v } = await chamarApi('/api/v1/versao');
        if (v === versao.current) { setConexao({ texto: 'Dados locais', estado: 'ok' }); return; }
      }
      if (sujo.current && !forcar) { setConexao({ texto: 'Há mudanças novas — salve o que está editando para ver', estado: '' }); return; }
      novo = await chamarApi('/api/v1/estado');
    } catch (e) {
      if (e instanceof ErroApi && e.status === 503) {
        fechando.current = true;
        setConexao({ texto: 'Fechando', estado: 'off' });
        setFalha({ titulo: 'Dados apagados. O app está fechando.', texto: 'Na próxima vez que você abrir, ele começa como novo.' });
      } else if (e instanceof ErroApi) {
        // A API respondeu, mas com erro (ex.: dados corrompidos).
        setConexao({ texto: 'Erro nos dados', estado: 'off' });
        setFalha({ titulo: 'A Central encontrou um problema nos dados', texto: e.message });
      } else {
        setConexao({ texto: 'Desconectado', estado: 'off' });
        if (!temEstado.current) setFalha({ titulo: 'A Central não está respondendo', texto: 'Feche e abra o app de novo. Se continuar, os dados estão salvos na pasta do app.' });
      }
      return;
    }
    temEstado.current = true;
    versao.current = novo.versao;
    sujo.current = false;
    setFalha(null);
    setEstado(novo);
    setConexao({ texto: 'Dados locais', estado: 'ok' });
  }, []);

  // Atualização automática: a cada 5 s, ao voltar para a janela e ao trocar de seção.
  useEffect(() => {
    const aoDigitar = (ev: Event) => { if ((ev.target as Element).matches?.('input:not([type=search]), textarea, form select')) sujo.current = true; };
    const aoTrocar = () => { sujo.current = false; carregar(true); };
    const aoVoltar = () => { if (document.visibilityState === 'visible') carregar(); };
    const aoFocar = () => carregar();
    document.addEventListener('input', aoDigitar);
    window.addEventListener('hashchange', aoTrocar);
    window.addEventListener('focus', aoFocar);
    document.addEventListener('visibilitychange', aoVoltar);
    const t = setInterval(aoVoltar, 5000);
    carregar(true);
    return () => {
      document.removeEventListener('input', aoDigitar);
      window.removeEventListener('hashchange', aoTrocar);
      window.removeEventListener('focus', aoFocar);
      document.removeEventListener('visibilitychange', aoVoltar);
      clearInterval(t);
    };
  }, [carregar]);

  const app: AppApi | null = useMemo(() => {
    if (!estado) return null;
    const api = (caminho: string, metodo = 'GET', corpo?: unknown) => chamarApi(caminho, metodo, corpo);
    const agir = async (fn: () => Promise<unknown>, mensagem?: string) => {
      try {
        await fn();
        if (mensagem) aviso(mensagem);
        await carregar(true);
        return true;
      } catch (e) {
        aviso((e as Error).message, true);
        return false;
      }
    };
    return {
      estado,
      hoje: estado.hoje,
      foco: estado.perfil?.foco ?? 'ambos',
      rota: tela.rota,
      vaga,
      api,
      agir,
      async mudarFoco(foco) {
        if (foco === (estado.perfil?.foco ?? 'ambos')) return;
        setEstado((e: Estado) => ({ ...e, perfil: { ...e.perfil, foco } }));
        await agir(() => api('/api/v1/perfil', 'PATCH', { foco }));
      },
      mover: (id, etapa, extras = {}, mensagem) => agir(() => api(`/api/v1/vagas/${encode(id)}/mover`, 'POST', { etapa, ...extras }), mensagem),
      abrirVaga: id => { location.hash = `#/${tela.rota}/${encode(id)}`; },
      navegar: rota => { location.hash = `#/${rota}`; },
      aviso,
      perguntar,
      recarregar: () => carregar(true),
      limpar: () => { sujo.current = false; },
    };
  }, [estado, tela.rota, vaga, aviso, perguntar, carregar]);

  // Título, foco e rolagem ao trocar de seção.
  const main = useRef<HTMLElement>(null);
  useLayoutEffect(() => {
    document.title = `${tela.nome} · Central de Candidaturas`;
    main.current?.focus({ preventScroll: true });
    window.scrollTo(0, 0);
  }, [tela.rota]);

  const r = estado ? resumo(estado.vagas, estado.hoje) : null;
  const detalheAberto = Boolean(estado && vaga && estado.vagas.some((v: any) => v.id === vaga));
  const fecharDetalhe = () => { location.hash = `#/${tela.rota}`; };

  useEffect(() => {
    const aoTeclar = (ev: KeyboardEvent) => { if (ev.key === 'Escape' && detalheAberto && !dialogo) fecharDetalhe(); };
    document.addEventListener('keydown', aoTeclar);
    return () => document.removeEventListener('keydown', aoTeclar);
  });

  const contador = (t: (typeof TELAS)[number]) => {
    const n = r && t.contador?.(r);
    return n ? <span className={`contador${t.alerta ? ' alerta' : ''}`}>{n}</span> : null;
  };

  const Tela = tela.Tela;
  return (
    <>
      <a className="pular-link" href="#conteudo" onClick={e => { e.preventDefault(); main.current?.focus(); }}>Ir para o conteúdo</a>
      <header className="topo-app">
        <div className="topo-dentro">
          <a className="marca" href="#/hoje" aria-label="Central de Candidaturas — início">
            <img src="/icone.svg" alt="" width={28} height={28} />
            <span>Central</span>
          </a>
          <Menu atual={tela.rota} contador={contador} />
          <div className="topo-pe">
            <span className={`status-conexao ${conexao.estado}`} role="status">{conexao.texto}</span>
            <button className="botao-icone" type="button" aria-label="Alternar tema claro/escuro" onClick={alternarTema}>
              <Icone d="M12 3a9 9 0 1 0 9 9 7 7 0 0 1-9-9z" />
            </button>
          </div>
        </div>
      </header>
      <main id="conteudo" tabIndex={-1} ref={main}>
        {falha ? (
          <div className="convite"><div><strong>{falha.titulo}</strong><p>{falha.texto}</p></div></div>
        ) : app && (
          <Contexto.Provider value={app}>
            <Protecao chave={tela.rota}>
              {/* Perfil: o formulário recomeça do que está salvo quando os dados mudam (e nada está por salvar). */}
              <Tela key={tela.rota === 'perfil' ? `perfil-${estado.versao}` : tela.rota} />
            </Protecao>
          </Contexto.Provider>
        )}
      </main>
      <nav className="abas" aria-label="Seções principais">
        {TELAS.filter(t => t.aba).map(t => (
          <a key={t.rota} href={`#/${t.rota}`} aria-current={t.rota === tela.rota ? 'page' : undefined}>
            <Icone d={ICONES[t.rota]} />{t.nome}{contador(t)}
          </a>
        ))}
        <button type="button" aria-current={!tela.aba ? 'page' : undefined} onClick={() => setDialogo({ tipo: 'mais', telas: TELAS.filter(t => !t.aba) })}>
          <Icone d={ICONES.mais} />Mais
        </button>
      </nav>
      <aside className="gaveta" aria-label="Detalhe da vaga" hidden={!detalheAberto}>
        {detalheAberto && app && <Contexto.Provider value={app}><Detalhe key={vaga!} id={vaga!} fechar={fecharDetalhe} /></Contexto.Provider>}
      </aside>
      <div className="veu" hidden={!detalheAberto} onClick={fecharDetalhe} />
      <Dialogo pedido={dialogo} fechar={() => setDialogo(null)} />
      <div className="avisos" aria-live="polite">
        {avisos.map(a => <div key={a.id} className={`aviso${a.erro ? ' erro' : ''}`} role={a.erro ? 'alert' : 'status'}>{a.texto}</div>)}
      </div>
    </>
  );
}

function alternarTema() {
  const raiz = document.documentElement;
  const escuroAgora = raiz.dataset.tema ? raiz.dataset.tema === 'escuro' : matchMedia('(prefers-color-scheme: dark)').matches;
  raiz.dataset.tema = escuroAgora ? 'claro' : 'escuro';
  try { localStorage.setItem('tema', raiz.dataset.tema); } catch { /* sem armazenamento: o tema vale só agora */ }
}

// Menu do topo. Janela estreita: o menu rola para o lado; as pontas cortadas esmaecem e a seção atual fica à vista.
function Menu({ atual, contador }: { atual: string; contador: (t: (typeof TELAS)[number]) => ReactNode }) {
  const nav = useRef<HTMLElement>(null);
  const rolado = useRef(false);
  const marcar = useCallback(() => {
    const n = nav.current;
    if (!n) return;
    const cortado = n.scrollWidth > n.clientWidth + 1;
    n.classList.toggle('corta-fim', cortado && n.scrollLeft + n.clientWidth < n.scrollWidth - 1);
    n.classList.toggle('corta-inicio', cortado && n.scrollLeft > 1);
    const ativo = n.querySelector<HTMLElement>('[aria-current="page"]');
    if (cortado && ativo && !rolado.current) { n.scrollLeft = ativo.offsetLeft - (n.clientWidth - ativo.offsetWidth) / 2; rolado.current = true; }
  }, []);
  useEffect(() => { rolado.current = false; marcar(); }, [atual, marcar]);
  useEffect(() => {
    window.addEventListener('resize', marcar);
    return () => window.removeEventListener('resize', marcar);
  }, [marcar]);
  return (
    <nav aria-label="Seções" ref={nav} onScroll={marcar}>
      <ul className="menu">
        {TELAS.map(t => (
          <li key={t.rota}>
            <a href={`#/${t.rota}`} aria-current={t.rota === atual ? 'page' : undefined}><span>{t.nome}</span>{contador(t)}</a>
          </li>
        ))}
      </ul>
    </nav>
  );
}
