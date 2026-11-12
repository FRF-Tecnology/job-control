// Casca da interface: estado, rotas, atualização automática, avisos e diálogos (porte de web/app.mjs).
import { Component, type CSSProperties, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Contexto, chamarApi, ErroApi, type App as AppApi, type Estado, type Pergunta } from './app/contexto';
import { liberouDepoisDaTrava, prepararProntas } from './app/preparoAutomatico';
import { resumo } from './dominio/funil';
import { Icone } from './componentes';
import { encode } from './util';
import { Dialogo, type PedidoDialogo } from './app/Dialogo';
import { Detalhe } from './telas/Detalhe';
import { TelaHoje } from './telas/Hoje';
import { TelaTriagem } from './telas/Triagem';
import { TelaFila } from './telas/Fila';
import { EVENTO_ENVIO, type EstadoEnvio } from './telas/PainelEnvio';
import { TelaQuadro } from './telas/Quadro';
import { TelaVagas } from './telas/Vagas';
import { TelaNumeros } from './telas/Numeros';
import { TelaPlataformas } from './telas/Plataformas';
import { TelaRodadas } from './telas/Rodadas';
import { TelaPerfil } from './telas/Perfil';
import { TelaConectarIA } from './telas/ConectarIA';
import { NavegadorFlutuante } from './telas/NavegadorFlutuante';
import { foiEnviada } from './dominio/modelo';
import { podeAgir as podeAgirCom, useConta } from './app/conta';
import { FaixaConta } from './telas/FaixaConta';
import { TelaConta } from './telas/Conta';
import { TelaAssinatura } from './telas/Assinatura';

// Ícone de cada seção (menu lateral no PC e abas no celular).
const ICONES: Record<string, string> = {
  hoje: 'M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8zM12 2v2M12 20v2M4.9 4.9l1.4 1.4M17.7 17.7l1.4 1.4M2 12h2M20 12h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4',
  triagem: 'M10 3h7a3 3 0 0 1 3 3v10a3 3 0 0 1-3 3h-7a3 3 0 0 1-3-3V6a3 3 0 0 1 3-3zM4 7v11a3 3 0 0 0 3 3h9',
  fila: 'M10 6h10M10 12h10M10 18h10M3.5 6l1.5 1.5L7.5 5M3.5 12l1.5 1.5L7.5 11M3.5 18l1.5 1.5L7.5 17',
  quadro: 'M4.5 4h2A1.5 1.5 0 0 1 8 5.5v13A1.5 1.5 0 0 1 6.5 20h-2A1.5 1.5 0 0 1 3 18.5v-13A1.5 1.5 0 0 1 4.5 4zM11.5 4h2A1.5 1.5 0 0 1 15 5.5v7a1.5 1.5 0 0 1-1.5 1.5h-2a1.5 1.5 0 0 1-1.5-1.5v-7A1.5 1.5 0 0 1 11.5 4zM18.5 4h1A1.5 1.5 0 0 1 21 5.5v10a1.5 1.5 0 0 1-1.5 1.5h-1a1.5 1.5 0 0 1-1.5-1.5v-10A1.5 1.5 0 0 1 18.5 4z',
  vagas: 'M3 8h18v12H3zM9 8V5a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v3',
  numeros: 'M5 20v-9M12 20V4M19 20v-6M3 20h18',
  plataformas: 'M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM3 12h18M12 3a14 14 0 0 1 0 18M12 3a14 14 0 0 0 0 18',
  rodadas: 'M20 12a8 8 0 1 1-2.6-5.9M20 4v4h-4',
  perfil: 'M12 4a4 4 0 1 0 0 8 4 4 0 0 0 0-8zM4 21a8 8 0 0 1 16 0',
  assinatura: 'M3 6h18v12H3zM3 10h18M7 15h4',
  ia: 'M12 3l1.8 5.2L19 10l-5.2 1.8L12 17l-1.8-5.2L5 10l5.2-1.8zM19 16l.8 2.2L22 19l-2.2.8L19 22l-.8-2.2L16 19l2.2-.8z',
  mais: 'M5 12h.01M12 12h.01M19 12h.01',
};
// Nome da IA em uso, para o resumo do menu lateral (provedores do GET /ia/config; fora da lista, fica genérico).
const NOMES_IA: Record<string, string> = {
  'claude-cli': 'Claude Code conectado', 'codex-cli': 'Codex conectado', 'gemini-cli': 'Gemini CLI conectado',
  anthropic: 'Claude conectado (chave)', openai: 'ChatGPT conectado (chave)', google: 'Gemini conectado (chave)',
};
type R = ReturnType<typeof resumo>;
// soConta: só aparece no menu quando a conta existe neste build (ruling A11).
const TELAS: { rota: string; nome: string; Tela: () => ReactNode; contador?: (r: R) => number; alerta?: boolean; aba?: boolean; soConta?: boolean }[] = [
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
  { rota: 'assinatura', nome: 'Assinatura', Tela: TelaAssinatura, soConta: true },
];

function lerHash() {
  // "?..." no fim (ex.: #/fila?preparar=<vagaId>, vindo de um pedido) é da tela, não muda a rota.
  const [, rota = 'hoje', vaga] = location.hash.split('?')[0].split('/');
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
  const [envio, setEnvio] = useState<Partial<EstadoEnvio> | null>(null); // GET /envio: a marca no cabeçalho (jc-a2y.51) e o navegador flutuante
  const envioAtivo = !!envio && envio.ligado === true && envio.pausado === false; // ligado e não pausado
  const [ia, setIA] = useState<{ configurada: boolean; provedor?: string } | null>(null); // null: ainda não sabe
  const [falha, setFalha] = useState<Falha>(null);
  const [avisos, setAvisos] = useState<Aviso[]>([]);
  const [dialogo, setDialogo] = useState<PedidoDialogo | null>(null);
  const versao = useRef<string | null>(null);
  const fechando = useRef(false); // 503 depois de apagar os dados (jc-a2y.49)
  const temEstado = useRef(false);
  // Há texto digitado e ainda não salvo (perfil, nota nova, notas da vaga…)? Enquanto houver, nada é redesenhado.
  const sujo = useRef(false);
  // Conta (Task 7): GET /conta ao abrir, a cada 60 s e depois de um 402. undefined: ainda não sabe (nada aparece).
  // ativa false (build sem a nuvem, ruling A11): sem porta, sem faixa e sem trava, como antes.
  const { situacao: conta, reler: releConta } = useConta(chamarApi);
  const contaAtiva = !!conta?.ativa;
  const liberado = podeAgirCom(conta);
  const telas = useMemo(() => TELAS.filter(t => !t.soConta || contaAtiva), [contaAtiva]);

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
        setFalha({ titulo: 'O Applyer encontrou um problema nos dados', texto: e.message });
      } else {
        setConexao({ texto: 'Desconectado', estado: 'off' });
        if (!temEstado.current) setFalha({ titulo: 'O Applyer não está respondendo', texto: 'Feche e abra o app de novo. Se continuar, os dados estão salvos na pasta do app.' });
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

  // Pacote preparado sozinho (jc-a2y.6.12): quando uma vaga chega a Aplicar (pela Triagem, pelo Quadro ou no modo
  // direto), o app cria a candidatura "preparada" com o pacote do perfil. Nada é aprovado nem enviado: a pessoa revisa
  // e aprova na Fila. Uma rodada por vez; cada vaga é tentada uma vez por abertura do app.
  const preparando = useRef(false);
  const tentadas = useRef(new Set<string>());
  // travadoEm: a versão do estado em que a trava das ações deu 402; nenhum preparo até a licença voltar.
  const travadoEm = useRef<string | null>(null);
  useEffect(() => {
    if (!estado || fechando.current || preparando.current) return;
    const modoDireto = (estado.perfil as any)?.modo === 'direto';
    const talvez = (estado.vagas ?? []).some((v: any) => !tentadas.current.has(v.id)
      && (v.etapa === 'aplicar' || (modoDireto && v.etapa === 'descoberta' && v.aderencia === 'A')));
    if (!talvez) return;
    preparando.current = true;
    (async () => {
      try {
        if (!(await liberouDepoisDaTrava(chamarApi, travadoEm.current, versao.current))) return;
        travadoEm.current = null;
        const candidaturas = await chamarApi('/api/v1/candidaturas');
        const r = await prepararProntas(chamarApi, { vagas: estado.vagas ?? [], perfil: estado.perfil, curriculos: (estado as any).curriculos ?? [] }, candidaturas, tentadas.current);
        if (r.travado) travadoEm.current = versao.current ?? '';
        if (r.preparadas) aviso(r.preparadas === 1 ? 'Uma candidatura foi preparada sozinha: revise o pacote e aprove na Fila.' : `${r.preparadas} candidaturas foram preparadas sozinhas: revise os pacotes e aprove na Fila.`);
      } catch { /* sem a lista, tenta na próxima leitura */ } finally { preparando.current = false; }
    })();
    // liberado: quando a conta volta a deixar agir (motivo vazio), o preparo pausado pelo 402 tenta de novo.
  }, [estado, aviso, liberado]);
  // Envio ligado e não pausado aparece no cabeçalho em todas as telas, não só na Fila (nota do CyberSec no #60).
  // Pausar e retomar não mudam a /versao, então a casca lê /envio a cada 5 s e ouve o painel da Fila na hora.
  useEffect(() => {
    const ler = async () => {
      if (fechando.current || document.visibilityState !== 'visible') return;
      // Leitura falhou: mantém o último valor conhecido, para a marca não sumir por um erro passageiro (CyberSec, #60).
      try { setEnvio(await chamarApi('/api/v1/envio')); } catch { /* mantém */ }
    };
    const aoMudar = (ev: Event) => setEnvio((ev as CustomEvent<EstadoEnvio>).detail ?? null);
    window.addEventListener(EVENTO_ENVIO, aoMudar);
    const t = setInterval(ler, 5000);
    ler();
    return () => { window.removeEventListener(EVENTO_ENVIO, aoMudar); clearInterval(t); };
  }, []);

  // A IA em uso, para o resumo do menu lateral: lida ao abrir e ao trocar de seção (a pessoa pode ter conectado outra).
  useEffect(() => {
    let vivo = true;
    chamarApi('/api/v1/ia/config').then(r => { if (vivo) setIA({ configurada: !!r?.configurada, provedor: typeof r?.provedor === 'string' ? r.provedor : undefined }); }).catch(() => { /* sem resposta: o resumo não aparece */ });
    return () => { vivo = false; };
  }, [tela.rota]);

  // A luz do fundo segue o ponteiro, devagar. Com "reduzir movimento", fica parada.
  const luz = useRef<HTMLElement>(null);
  useEffect(() => {
    if (typeof matchMedia !== 'function' || matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    const mover = (ev: PointerEvent) => { if (luz.current) luz.current.style.transform = `translate(${ev.clientX}px, ${ev.clientY}px)`; };
    window.addEventListener('pointermove', mover, { passive: true });
    return () => window.removeEventListener('pointermove', mover);
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
    const api = (caminho: string, metodo = 'GET', corpo?: unknown, cabecalhos?: Record<string, string>) => chamarApi(caminho, metodo, corpo, cabecalhos);
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
      podeAgir: liberado,
      conta,
      releConta,
    };
  }, [estado, tela.rota, vaga, aviso, perguntar, carregar, liberado, conta, releConta]);

  // Título, foco e rolagem ao trocar de seção.
  const main = useRef<HTMLElement>(null);
  useLayoutEffect(() => {
    document.title = `${tela.nome} · Applyer`;
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

  // Resumo do dia no menu lateral: os mesmos números da tela Hoje (envios de hoje e o limite do LinkedIn).
  const enviadasHoje = estado ? estado.vagas.filter((v: any) => foiEnviada(v) && v.envio?.data === estado.hoje) : [];
  const limiteLinkedin = Number(estado?.perfil?.limiteLinkedinDia) || 40;
  const linkedinHoje = enviadasHoje.filter((v: any) => v.plataforma === 'linkedin').length;

  const Tela = tela.Tela;
  // Porta da conta: com a conta ligada e sem entrada, só a tela Conta (antes do primeiro acesso e de tudo o mais).
  const porta = contaAtiva && !conta?.entrada;
  const entrou = useCallback(async () => { await releConta(); await carregar(true); }, [releConta, carregar]);
  return (
    <>
      <div className="fundo" aria-hidden="true">
        <i className="bolha b1" /><i className="bolha b2" /><i className="bolha b3" /><i className="bolha b4" /><i className="bolha b5" />
        <i className="luz" ref={luz} />
      </div>
      <a className="pular-link" href="#conteudo" onClick={e => { e.preventDefault(); main.current?.focus(); }}>Ir para o conteúdo</a>
      <header className="topo-app">
        <div className="topo-dentro">
          <a className="marca" href="#/hoje" aria-label="Applyer — início">
            <span className="logo-ic"><Icone><path d="M21 3L3 10.5l7 2.5 2.5 7z" /><path d="M21 3L10 13" /></Icone></span>
            <span>Applyer</span>
          </a>
          {!porta && <Menu telas={telas} atual={tela.rota} contador={contador} />}
          {estado && !porta && (
            <div className="lateral-extra so-alto">
              <b>Hoje</b>
              <div className="medidor" aria-hidden="true"><span style={{ width: `${Math.round(Math.min(linkedinHoje / limiteLinkedin, 1) * 100)}%` }} /></div>
              <span>{`${enviadasHoje.length} ${enviadasHoje.length === 1 ? 'candidatura enviada' : 'candidaturas enviadas'} · LinkedIn: ${linkedinHoje} de ${limiteLinkedin}`}</span>
            </div>
          )}
          {ia && !porta && (
            <a className="lateral-extra" href="#/ia">
              <span className="status-ia"><span className={`ponto${ia.configurada ? ' ok' : ''}`} aria-hidden="true" /><b>{ia.configurada ? (NOMES_IA[ia.provedor ?? ''] ?? 'IA conectada') : 'Nenhuma IA conectada'}</b></span>
              <span>{ia.configurada ? 'Sua IA, sua assinatura. Seus dados ficam neste computador.' : 'Conecte a sua IA para ela opinar na triagem e escrever rascunhos.'}</span>
            </a>
          )}
          <div className="topo-pe">
            {envioAtivo && <a className="marca-envio" href="#/fila" title="O app está enviando as candidaturas aprovadas. Abra a Fila para pausar.">Envio ligado</a>}
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
        ) : conta === undefined ? null : porta ? (
          <TelaConta api={chamarApi} aoEntrar={entrou} />
        ) : app && (
          <Contexto.Provider value={app}>
            {contaAtiva && <FaixaConta situacao={conta} reler={releConta} />}
            <Protecao chave={tela.rota}>
              {/* Perfil: o formulário recomeça do que está salvo quando os dados mudam (e nada está por salvar). */}
              <Tela key={tela.rota === 'perfil' ? `perfil-${estado.versao}` : tela.rota} />
            </Protecao>
          </Contexto.Provider>
        )}
      </main>
      <nav className="abas" aria-label="Seções principais" hidden={porta}>
        {telas.filter(t => t.aba).map(t => (
          <a key={t.rota} href={`#/${t.rota}`} aria-current={t.rota === tela.rota ? 'page' : undefined}>
            <Icone d={ICONES[t.rota]} />{t.nome}{contador(t)}
          </a>
        ))}
        <button type="button" aria-current={!tela.aba ? 'page' : undefined} onClick={() => setDialogo({ tipo: 'mais', telas: telas.filter(t => !t.aba) })}>
          <Icone d={ICONES.mais} />Mais
        </button>
      </nav>
      <aside className="gaveta" aria-label="Detalhe da vaga" hidden={!detalheAberto || porta}>
        {detalheAberto && !porta && app && <Contexto.Provider value={app}><Detalhe key={vaga!} id={vaga!} fechar={fecharDetalhe} /></Contexto.Provider>}
      </aside>
      <div className="veu" hidden={!detalheAberto || porta} onClick={fecharDetalhe} />
      {estado && !falha && !porta && <NavegadorFlutuante api={chamarApi} envio={envio} vagas={estado.vagas ?? []} aviso={aviso} />}
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

// Menu das seções: lateral em telas largas, no topo nas estreitas. No topo, o menu rola para o lado; as pontas
// cortadas esmaecem e a seção atual fica à vista.
function Menu({ telas, atual, contador }: { telas: typeof TELAS; atual: string; contador: (t: (typeof TELAS)[number]) => ReactNode }) {
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
    // A largura da faixa muda também quando algo no topo aparece (ex.: a marca "Envio ligado"), não só na janela.
    const n = nav.current;
    const obs = n && typeof ResizeObserver !== 'undefined' ? new ResizeObserver(marcar) : null;
    if (n && obs) obs.observe(n);
    window.addEventListener('resize', marcar);
    return () => { obs?.disconnect(); window.removeEventListener('resize', marcar); };
  }, [marcar]);
  return (
    <nav aria-label="Seções" ref={nav} onScroll={marcar}>
      {/* No menu lateral, a marca da seção atual desliza até ela (só enfeite: quem diz a seção atual é o aria-current). */}
      <span className="indicador" aria-hidden="true" style={{ '--i': Math.max(telas.findIndex(t => t.rota === atual), 0) } as CSSProperties} />
      <ul className="menu">
        {telas.map(t => (
          <li key={t.rota}>
            <a href={`#/${t.rota}`} aria-current={t.rota === atual ? 'page' : undefined}><Icone d={ICONES[t.rota]} /><span>{t.nome}</span>{contador(t)}</a>
          </li>
        ))}
      </ul>
    </nav>
  );
}
