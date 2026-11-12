// Casca da interface: estado, rotas, atualização automática, avisos e diálogos (porte de web/app.mjs).
import { Component, useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Contexto, chamarApi, ErroApi, type App as AppApi, type Estado, type Pergunta } from './app/contexto';
import { grupoADireto, liberouDepoisDaTrava, prepararProntas } from './app/preparoAutomatico';
import { Espaco } from './app/Espaco';
import { encode } from './util';
import { Dialogo, type PedidoDialogo } from './app/Dialogo';
import { Detalhe } from './telas/Detalhe';
import { TelaHoje } from './telas/Hoje';
import { TelaTriagem } from './telas/Triagem';
import { TelaRadar, vagasDisponiveisNoRadar } from './telas/Radar';
import { TelaListaDoDia, TelaQuadroCandidaturas } from './telas/Jornada';
import { PerfilDoEspaco } from './telas/PerfilDoEspaco';
import { TelaFila } from './telas/Fila';
import { EVENTO_ENVIO, type EstadoEnvio } from './telas/PainelEnvio';
import { TelaQuadro } from './telas/Quadro';
import { TelaVagas } from './telas/Vagas';
import { TelaNumeros } from './telas/Numeros';
import { TelaPlataformas } from './telas/Plataformas';
import { TelaRodadas } from './telas/Rodadas';
import { TelaPerfil } from './telas/Perfil';
import { TelaConectarIA } from './telas/ConectarIA';
import { PrimeiroAcesso } from './telas/PrimeiroAcesso';
import { NavegadorFlutuante } from './telas/NavegadorFlutuante';
import { podeAgir as podeAgirCom, useConta, useFoco } from './app/conta';
import { FaixaConta } from './telas/FaixaConta';
import { TelaConta } from './telas/Conta';
import { TelaAssinatura } from './telas/Assinatura';

// Nome da IA em uso, para o resumo do menu lateral (provedores do GET /ia/config; fora da lista, fica genérico).
const NOMES_IA: Record<string, string> = {
  'claude-cli': 'Claude Code conectado', 'codex-cli': 'Codex conectado', 'gemini-cli': 'Gemini CLI conectado',
  anthropic: 'Claude conectado (chave)', openai: 'ChatGPT conectado (chave)', google: 'Gemini conectado (chave)', xai: 'Grok conectado (chave)',
};
const MARCAS_IA: Record<string,string> = {
  'claude-cli':'Claude', anthropic:'Claude', 'codex-cli':'OpenAI', openai:'OpenAI',
  'gemini-cli':'Gemini', google:'Gemini', xai:'Grok',
};
// soConta: só aparece entre as ferramentas quando a conta existe neste build (ruling A11).
type Destino = { rota: string; nome: string; Tela: () => ReactNode; soConta?: boolean };
const TELAS: Destino[] = [
  {rota:'radar',nome:'Radar de vagas',Tela:TelaRadar},
  {rota:'lista',nome:'Lista do dia',Tela:TelaListaDoDia},
  {rota:'quadro',nome:'Quadro',Tela:TelaQuadroCandidaturas},
  {rota:'perfil',nome:'Perfil e IA',Tela:PerfilDoEspaco},
  {rota:'hoje',nome:'Hoje',Tela:TelaHoje}, {rota:'triagem',nome:'Triagem',Tela:TelaTriagem},
  {rota:'fila',nome:'Fila',Tela:TelaFila}, {rota:'funil',nome:'Funil completo',Tela:TelaQuadro},
  {rota:'vagas',nome:'Vagas',Tela:TelaVagas}, {rota:'numeros',nome:'Números',Tela:TelaNumeros},
  {rota:'plataformas',nome:'Plataformas',Tela:TelaPlataformas}, {rota:'rodadas',nome:'Rodadas',Tela:TelaRodadas},
  {rota:'regras',nome:'Regras e dados',Tela:TelaPerfil}, {rota:'ia',nome:'Conectar IA',Tela:TelaConectarIA},
  {rota:'assinatura',nome:'Assinatura',Tela:TelaAssinatura,soConta:true},
];

function lerHash() {
  // "?..." no fim (ex.: #/fila?preparar=<vagaId>, vindo de um pedido) é da tela, não muda a rota.
  const [, rota = 'radar', vaga] = location.hash.split('?')[0].split('/');
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
  const primeiroAcesso = estado?.onboardingConcluido === false;
  const [conexao, setConexao] = useState<{ texto: string; estado: 'ok' | 'off' | '' }>({ texto: '', estado: '' });
  const [envio, setEnvio] = useState<Partial<EstadoEnvio> | null>(null); // GET /envio: a marca no cabeçalho (jc-a2y.51) e o navegador flutuante
  const envioAtivo = !!envio && envio.ligado === true && envio.pausado === false; // ligado e não pausado
  const [ia, setIA] = useState<{ configurada: boolean; conexaoVerificada?:boolean; provedor?: string } | null>(null);
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
  const { situacao: conta, reler: releConta, atualizar: atualizarConta } = useConta(chamarApi);
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
    for (const v of estado.vagas ?? []) if (v.etapa==='descoberta' && v.devolvidaAoRadar) tentadas.current.delete(v.id);
    const talvez = (estado.vagas ?? []).some((v: any) => !tentadas.current.has(v.id)
      && (v.etapa === 'aplicar' || (modoDireto && grupoADireto(v))));
    if (!talvez) return;
    preparando.current = true;
    (async () => {
      try {
        if (!(await liberouDepoisDaTrava(chamarApi, travadoEm.current, versao.current))) return;
        travadoEm.current = null;
        const candidaturas = await chamarApi('/api/v1/candidaturas');
        const r = await prepararProntas(chamarApi, { vagas: estado.vagas ?? [], perfil: estado.perfil, curriculos: estado.curriculos ?? [], perfisBusca: estado.perfisBusca, perfilBuscaAtivo: estado.perfilBuscaAtivo }, candidaturas, tentadas.current);
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
    let vivo = true, leitura = 0;
    const ler=()=>{const atual=++leitura; return chamarApi('/api/v1/ia/config').then(r=>{if(vivo && atual===leitura)setIA(r)}).catch(()=>{if(vivo && atual===leitura)setIA(null)})};
    void ler(); window.addEventListener('applyer:ia-alterada',ler);
    return () => { vivo = false; window.removeEventListener('applyer:ia-alterada',ler); };
  }, [tela.rota, primeiroAcesso]);


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

  // O foco é do perfil de busca ativo (Radar por objetivo).
  const perfilBuscaAtivo = estado?.perfilBuscaAtivo || 'original';
  // Foco da busca (ruling A16): com a trava ligada, é só um filtro na tela (sem PATCH); liberado, salva no perfil.
  const salvarFoco = useCallback(async (foco: string) => {
    setEstado((e: Estado) => (e ? { ...e, perfil: { ...e.perfil, foco } } : e));
    try {
      await chamarApi('/api/v1/perfil?perfilBuscaId=' + encode(perfilBuscaAtivo), 'PATCH', { foco });
      await carregar(true);
    } catch (e) {
      aviso((e as Error).message, true);
    }
  }, [carregar, aviso, perfilBuscaAtivo]);
  const { foco, mudarFoco } = useFoco(estado?.perfil?.foco ?? 'ambos', liberado, salvarFoco);

  const app: AppApi | null = useMemo(() => {
    if (!estado) return null;
    const api = chamarApi;
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
      conexaoIA: {nome: MARCAS_IA[ia?.provedor ?? ''] ?? 'Provedor', verificada:ia?.conexaoVerificada === true},
      hoje: estado.hoje,
      foco,
      rota: tela.rota,
      vaga,
      api,
      agir,
      mudarFoco,
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
  }, [estado, ia, tela.rota, vaga, aviso, perguntar, carregar, liberado, conta, releConta, foco, mudarFoco]);

  // Título, foco e rolagem ao trocar de seção.
  const main = useRef<HTMLElement>(null);
  useLayoutEffect(() => {
    document.title = `${primeiroAcesso && tela.rota !== 'ia' ? 'Primeiro acesso' : tela.nome} · Applyer`;
    main.current?.focus({ preventScroll: true });
    window.scrollTo(0, 0);
  }, [tela.rota, primeiroAcesso]);

  const detalheAberto = Boolean(estado && vaga && estado.vagas.some((v: any) => v.id === vaga));
  const fecharDetalhe = () => { location.hash = `#/${tela.rota}`; };

  useEffect(() => {
    const aoTeclar = (ev: KeyboardEvent) => { if (ev.key === 'Escape' && detalheAberto && !dialogo) fecharDetalhe(); };
    document.addEventListener('keydown', aoTeclar);
    return () => document.removeEventListener('keydown', aoTeclar);
  });

  const vagas = estado?.vagas ?? [];
  const ativo = estado?.perfilBuscaAtivo || 'original';
  const quantidades = {
    radar: vagasDisponiveisNoRadar(vagas, ativo).length,
    lista: vagas.filter(v=>v.etapa==='aplicar').length,
  };
  const Tela = tela.Tela;
  const avisosEDialogo = <>
    <Dialogo pedido={dialogo} fechar={() => setDialogo(null)} />
    <div className="avisos" aria-live="polite">{avisos.map(a =>
      <div key={a.id} className={`aviso${a.erro ? ' erro' : ''}`} role={a.erro ? 'alert' : 'status'}>{a.texto}</div>)}</div>
  </>;
  // Porta da conta: com a conta ligada e sem entrada, só a tela Conta, antes do primeiro acesso e de tudo o mais.
  // Conta ainda não lida (undefined): nada aparece, para o primeiro acesso não piscar antes da porta.
  const porta = contaAtiva && !conta?.entrada;
  const entrou = useCallback(async () => { await releConta(); await carregar(true); }, [releConta, carregar]);
  if (!falha && conta === undefined) return avisosEDialogo;
  if (!falha && porta) {
    return <>
      <main id="conteudo" className="porta-conta" tabIndex={-1} ref={main}>
        <TelaConta api={chamarApi} aoEntrar={entrou} googleDisponivel={conta?.google_disponivel === true} />
      </main>
      {avisosEDialogo}
    </>;
  }
  if (primeiroAcesso && app && !falha) {
    const concluir = async (buscar: boolean) => {
      const salvo=await chamarApi('/api/v1/estado');
      if(salvo?.onboardingConcluido !== true) throw new Error('Não foi possível confirmar o perfil salvo. Tente novamente.');
      sujo.current=false; versao.current=salvo.versao;
      setEstado(salvo); app.navegar(buscar ? 'radar?buscar=1' : 'radar');
    };
    // O teste pode acabar no meio do primeiro acesso: o link Assinar da faixa abre a Assinatura aqui mesmo.
    if (contaAtiva && tela.rota === 'assinatura') {
      return <><Contexto.Provider value={app}>
        <main id="conteudo" className="porta-conta" tabIndex={-1} ref={main}>
          <div className="tela-conta">
            <FaixaConta situacao={conta} reler={atualizarConta} />
            <Protecao chave="assinatura"><TelaAssinatura /></Protecao>
            <p><a href="#/radar">Voltar ao primeiro acesso</a></p>
          </div>
        </main>
      </Contexto.Provider>{avisosEDialogo}</>;
    }
    return <><Contexto.Provider value={app}>
      {contaAtiva && <FaixaConta situacao={conta} reler={atualizarConta} />}
      <Protecao chave="primeiro-acesso">
        <PrimeiroAcesso alternarTema={alternarTema} concluir={concluir} />
      </Protecao></Contexto.Provider>{avisosEDialogo}</>;
  }
  return (
    <>
      <Espaco rota={tela.rota} titulo={tela.nome} pessoa={estado?.perfil?.nome ?? ''} ferramentas={telas.slice(4)}
        quantidades={quantidades} status={conexao.texto} falha={Boolean(falha)||conexao.estado==='off'}
        envio={envioAtivo} tema={alternarTema} ia={{
          claude:['anthropic','claude-cli'].includes(ia?.provedor ?? ''), verificada:!!ia?.conexaoVerificada,
          texto:ia?.conexaoVerificada ? (NOMES_IA[ia.provedor ?? ''] ?? 'IA conectada') : ia?.configurada ? 'IA configurada' : 'Conexão pendente'
        }}>
      <main id="conteudo" className="area-corpo" tabIndex={-1} ref={main}>
        {falha ? (
          <div className="convite"><div><strong>{falha.titulo}</strong><p>{falha.texto}</p></div></div>
        ) : app && (
          <Contexto.Provider value={app}>
            {contaAtiva && <FaixaConta situacao={conta} reler={atualizarConta} />}
            <Protecao chave={tela.rota}>
              {/* Perfil: o formulário recomeça do que está salvo quando os dados mudam (e nada está por salvar). */}
              <Tela key={tela.rota === 'regras' ? `regras-${estado.versao}` : `${tela.rota}-${ativo}`} />
            </Protecao>
          </Contexto.Provider>
        )}
      </main>
      </Espaco>
      <aside className="gaveta" aria-label="Detalhe da vaga" hidden={!detalheAberto}>
        {detalheAberto && app && <Contexto.Provider value={app}><Detalhe key={vaga!} id={vaga!} fechar={fecharDetalhe} /></Contexto.Provider>}
      </aside>
      <div className="veu" hidden={!detalheAberto} onClick={fecharDetalhe} />
      {estado && !falha && <NavegadorFlutuante api={chamarApi} envio={envio} vagas={estado.vagas ?? []} aviso={aviso} />}
      {avisosEDialogo}
    </>
  );
}

function alternarTema() {
  const raiz = document.documentElement;
  const escuroAgora = raiz.dataset.tema ? raiz.dataset.tema === 'escuro' : matchMedia('(prefers-color-scheme: dark)').matches;
  raiz.dataset.tema = escuroAgora ? 'claro' : 'escuro';
  try { localStorage.setItem('tema', raiz.dataset.tema); } catch { /* sem armazenamento: o tema vale só agora */ }
}
