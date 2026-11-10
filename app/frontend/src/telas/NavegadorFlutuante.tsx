// Navegador do envio, sempre à mão: o mesmo "ao vivo" da Fila (docs/propostas/ao-vivo.md) num widget que acompanha a
// pessoa em todas as telas. Fica encaixado na lateral direita; com o mouse por cima (ou o foco do teclado) ele abre, e
// SÓ ENTÃO liga o ao vivo e lê a cada segundo. Ao sair, desliga e a imagem não fica guardada. Dá para soltar da
// lateral e arrastar pela tela; perto da borda direita ele encaixa de novo. Como no painel da Fila, é só para ver:
// nada aqui clica ou digita no envio. A única ação é pausar, que qualquer um pode e só reduz o risco.
import { useEffect, useRef, useState, type PointerEvent as PonteiroReact } from 'react';
import type { App } from '../app/contexto';
import { Icone } from '../componentes';
import { ETAPA, EVENTO_PAINEL_AO_VIVO, imagemValida, useAoVivo } from './AoVivo';
import { EVENTO_ENVIO, type EstadoEnvio } from './PainelEnvio';

type Lugar = { modo: 'doca' | 'solto'; r: number; b: number }; // r e b: distância da direita e de baixo, em px
const PADRAO: Lugar = { modo: 'doca', r: 24, b: 24 };
const CHAVE = 'navegador-envio';
const LARGURA = 300, ALTURA = 250; // tamanho do widget solto e fechado (o que se arrasta)
const PERTO = 28; // a menos disso da borda direita, encaixa ao soltar

function lerLugar(): Lugar {
  try {
    const l = JSON.parse(localStorage.getItem(CHAVE) ?? 'null');
    if (l && (l.modo === 'doca' || l.modo === 'solto') && Number.isFinite(l.r) && Number.isFinite(l.b)) return { modo: l.modo, r: l.r, b: l.b };
  } catch { /* sem armazenamento ou valor estranho: começa encaixado */ }
  return PADRAO;
}
const caber = (l: Lugar): Lugar => ({
  ...l,
  r: Math.max(0, Math.min(l.r, window.innerWidth - LARGURA - 8)),
  b: Math.max(8, Math.min(l.b, window.innerHeight - ALTURA - 8)),
});

type Props = {
  api: App['api'];
  envio: Partial<EstadoEnvio> | null; // GET /envio, lido pela casca
  vagas: any[];
  aviso: (texto: string, erro?: boolean) => void;
};

export function NavegadorFlutuante({ api, envio, vagas, aviso }: Props) {
  const [lugar, setLugar] = useState<Lugar>(lerLugar);
  const [sobre, setSobre] = useState(false); // mouse ou foco do teclado em cima
  const [fixo, setFixo] = useState(false);
  const [arrastando, setArrastando] = useState(false);
  const [perto, setPerto] = useState(false);
  const [painelAberto, setPainelAberto] = useState(false); // o "ao vivo" da Fila está aberto: este some
  const [semAoVivo, setSemAoVivo] = useState(false); // 404 ou 403: não adianta abrir de novo
  const [nomes, setNomes] = useState<Record<string, string>>({});
  const [ocupado, setOcupado] = useState(false);
  const raiz = useRef<HTMLElement>(null);
  const relogio = useRef(0);
  const arrasto = useRef<{ x0: number; y0: number; r0: number; b0: number; moveu: boolean; r: number; b: number; perto: boolean } | null>(null);
  const clicouArrastando = useRef(false);

  const visivel = envio?.ligado === true && !painelAberto;
  const aberto = visivel && (sobre || fixo) && !arrastando && !semAoVivo;
  const { estado, erro } = useAoVivo(api, aberto, () => { setSemAoVivo(true); setFixo(false); setSobre(false); });

  useEffect(() => {
    const aoPainel = (ev: Event) => setPainelAberto((ev as CustomEvent<boolean>).detail === true);
    const aoRedimensionar = () => setLugar(l => caber(l));
    window.addEventListener(EVENTO_PAINEL_AO_VIVO, aoPainel);
    window.addEventListener('resize', aoRedimensionar);
    return () => { window.removeEventListener(EVENTO_PAINEL_AO_VIVO, aoPainel); window.removeEventListener('resize', aoRedimensionar); window.clearTimeout(relogio.current); };
  }, []);
  useEffect(() => { try { localStorage.setItem(CHAVE, JSON.stringify(lugar)); } catch { /* vale só agora */ } }, [lugar]);

  // "Empresa — Título" da candidatura do ao vivo: a lista vem da Fila só quando aparece um id ainda sem nome.
  const cand = estado?.candidatura ?? null;
  useEffect(() => {
    if (!cand || nomes[cand] !== undefined) return;
    let vivo = true;
    api('/api/v1/candidaturas').then((lista: any[]) => {
      if (!vivo) return;
      const c = Array.isArray(lista) ? lista.find(x => x?.id === cand) : null;
      const v = c ? vagas.find(x => x.id === c.vagaId) : null;
      setNomes(n => ({ ...n, [cand]: v ? `${v.empresa} — ${v.titulo}` : '' }));
    }).catch(() => { if (vivo) setNomes(n => ({ ...n, [cand]: '' })); });
    return () => { vivo = false; };
  }, [cand]); // eslint-disable-line react-hooks/exhaustive-deps

  if (!visivel) return null;

  // Abre com um pequeno atraso (o mouse só de passagem não liga o ao vivo) e fecha um pouco depois de sair.
  const agendar = (valor: boolean, ms: number) => { window.clearTimeout(relogio.current); relogio.current = window.setTimeout(() => setSobre(valor), ms); };
  const aoSairDoFoco = (ev: React.FocusEvent) => { if (!raiz.current?.contains(ev.relatedTarget as Node | null)) agendar(false, 450); };

  const iniciarArrasto = (ev: PonteiroReact) => {
    if (ev.button !== 0) return;
    const noBotao = (ev.target as Element).closest('button, a');
    if (noBotao && !noBotao.classList.contains('doca-aba')) return;
    if (!noBotao) ev.preventDefault(); // arrastar não seleciona o texto da página
    const caixa = raiz.current!.getBoundingClientRect();
    arrasto.current = { x0: ev.clientX, y0: ev.clientY, r0: window.innerWidth - caixa.right, b0: window.innerHeight - caixa.bottom, moveu: false, r: lugar.r, b: lugar.b, perto: false };
    const mover = (e: PointerEvent) => {
      const a = arrasto.current;
      if (!a) return;
      if (!a.moveu) {
        if (Math.abs(e.clientX - a.x0) + Math.abs(e.clientY - a.y0) < 5) return;
        a.moveu = true;
        if (lugar.modo === 'doca') { // sai da lateral e vira o widget solto, embaixo do ponteiro
          a.x0 = e.clientX; a.y0 = e.clientY;
          a.r0 = window.innerWidth - e.clientX - LARGURA / 2; a.b0 = window.innerHeight - e.clientY - ALTURA + 30;
          setLugar(caber({ modo: 'solto', r: a.r0, b: a.b0 }));
        }
        setArrastando(true); setFixo(false); setSobre(false);
      }
      const l = caber({ modo: 'solto', r: a.r0 - (e.clientX - a.x0), b: a.b0 - (e.clientY - a.y0) });
      a.r = l.r; a.b = l.b;
      if (raiz.current) { raiz.current.style.right = `${l.r}px`; raiz.current.style.bottom = `${l.b}px`; }
      const p = l.r < PERTO;
      if (p !== a.perto) { a.perto = p; setPerto(p); }
    };
    const soltar = () => {
      window.removeEventListener('pointermove', mover);
      window.removeEventListener('pointerup', soltar);
      window.removeEventListener('pointercancel', soltar);
      const a = arrasto.current;
      arrasto.current = null;
      if (!a?.moveu) return;
      clicouArrastando.current = true; // o clique que vem depois de soltar não abre nem fecha nada
      window.setTimeout(() => { clicouArrastando.current = false; }, 0);
      setArrastando(false); setPerto(false);
      setLugar(a.perto ? { modo: 'doca', r: PADRAO.r, b: a.b } : { modo: 'solto', r: Math.max(8, a.r), b: a.b });
    };
    window.addEventListener('pointermove', mover);
    window.addEventListener('pointerup', soltar);
    window.addEventListener('pointercancel', soltar);
  };

  const encaixado = lugar.modo === 'doca';
  const alternarDoca = () => { setFixo(false); setSobre(false); setLugar(l => (l.modo === 'doca' ? caber({ ...l, modo: 'solto', r: Math.max(l.r, 24) }) : { ...l, modo: 'doca' })); };

  async function pausar() {
    setOcupado(true);
    try {
      const e: EstadoEnvio = await api('/api/v1/envio/pausar', 'POST', {});
      window.dispatchEvent(new CustomEvent(EVENTO_ENVIO, { detail: e }));
      aviso(e.enviandoAgora ? 'Envio pausado. A candidatura que está sendo enviada agora termina; nenhuma outra sai até você retomar.' : 'Envio pausado. Nenhuma candidatura sai até você retomar.');
    } catch { aviso('Não deu para pausar. Tente de novo.', true); }
    finally { setOcupado(false); }
  }

  const enviando = !!envio?.enviandoAgora && !envio?.pausado;
  const resumo = semAoVivo ? (erro ?? 'Este app não traz o ao vivo.')
    : envio?.login ? 'Janela de login aberta: o envio espera.'
    : envio?.pausado ? 'O envio está pausado.'
    : envio?.enviandoAgora ? 'Enviando uma candidatura agora.' : 'Nada sendo enviado agora.';
  const etapa = aberto ? (estado ? (ETAPA[estado.etapa] ?? ETAPA['sem-envio']) : 'Ligando…') : resumo;
  const quadro = aberto && estado && imagemValida(estado.quadro) ? estado.quadro : null;
  const nomeCand = cand && estado?.etapa !== 'sem-envio' ? nomes[cand] : '';
  const cor = envio?.pausado ? 'pausado' : enviando ? 'enviando' : 'parado';

  const corpo = (
    <div className="fl-corpo">
      <span className="fl-solte" aria-hidden="true"><Icone><rect x="3" y="4" width="18" height="16" rx="3" /><path d="M15 4v16" /></Icone>Solte para encaixar na lateral</span>
      <div className="fl-cab">
        <span className="fl-status"><span className={`fl-ponto ${cor}`} aria-hidden="true" /><span className="fl-etapa" role="status">{etapa}</span></span>
        {aberto && (
          <span className="fl-acoes">
            {envio?.pausado === false && <button className="botao-fantasma p" type="button" disabled={ocupado} onClick={pausar}>Pausar envio</button>}
            <a className="botao-fantasma p" href="#/fila">Abrir a Fila</a>
          </span>
        )}
        <button className="fl-btn" type="button" onClick={alternarDoca} aria-label={encaixado ? 'Soltar o navegador da lateral' : 'Minimizar o navegador na lateral'} title={encaixado ? 'Soltar da lateral' : 'Minimizar na lateral'}>
          <Icone><rect x="3" y="4" width="18" height="16" rx="3" /><path d={encaixado ? 'M15 4v16M11 10l-2 2 2 2' : 'M15 4v16M8 10l2 2-2 2'} /></Icone>
        </button>
        <button className="fl-btn" type="button" aria-pressed={fixo} onClick={() => setFixo(f => !f)} aria-label={fixo ? 'Soltar o navegador (volta a fechar sozinho)' : 'Manter o navegador aberto'} title={fixo ? 'Deixar fechar sozinho' : 'Manter aberto'}>
          <Icone><path d="M9 4h6l-1 6 4 3H6l4-3z" /><path d="M12 13v7" /></Icone>
        </button>
      </div>
      <div className="fl-janela">
        <div className="fl-barra" aria-hidden="true"><span className="fl-bolinhas"><i /><i /><i /></span><span className="fl-url"><Icone><rect x="5" y="11" width="14" height="10" rx="2" /><path d="M8 11V8a4 4 0 0 1 8 0v3" /></Icone>Navegador do envio · só para ver</span></div>
        {quadro
          ? <img className="fl-quadro" src={quadro.imagem} width={Math.min(quadro.largura, 1280)} height={Math.min(quadro.altura, 800)} alt={nomeCand ? `Tela do envio de ${nomeCand}` : 'Tela do envio'} draggable={false} />
          : <div className="fl-vazio">
              <span className="fl-vazio-ic" aria-hidden="true"><Icone><circle cx="12" cy="12" r="9" /><path d="M3 12h18M12 3a14 14 0 0 1 0 18M12 3a14 14 0 0 0 0 18" /></Icone></span>
              <span>{!aberto ? (semAoVivo ? resumo : 'Passe o mouse para assistir ao envio.')
                : estado?.etapa === 'login-aberto' || estado?.etapa === 'pausado' ? 'Sem imagem enquanto isso.' : 'A imagem aparece quando uma candidatura estiver sendo enviada.'}</span>
            </div>}
      </div>
      {nomeCand && <p className="fl-cand pequeno mudo">{nomeCand}</p>}
      {aberto && erro && <p className="pequeno erro-resposta" role="alert">{erro}</p>}
    </div>
  );

  return (
    <>
      {perto && <div className="fl-borda" aria-hidden="true" />}
      <aside ref={raiz} aria-label="Navegador do envio"
        className={`flutuante${encaixado ? ' encaixado' : ' solto'}${aberto ? ' aberto' : ''}${arrastando ? ' arrastando' : ''}${perto ? ' perto' : ''}`}
        style={encaixado ? undefined : { right: `${lugar.r}px`, bottom: `${lugar.b}px` }}
        onPointerDown={iniciarArrasto}
        onPointerEnter={() => agendar(true, 250)} onPointerLeave={() => agendar(false, 450)}
        onFocus={() => agendar(true, 0)} onBlur={aoSairDoFoco}>
        {(!encaixado || aberto) && corpo}
        {encaixado && (
          <button className="doca-aba" type="button" aria-expanded={aberto} aria-label={fixo ? 'Fechar o navegador do envio' : 'Abrir o navegador do envio'}
            title="Passe o mouse para abrir. Arraste para soltar da lateral."
            onClick={() => { if (!clicouArrastando.current) setFixo(f => !f); }}>
            <span className="doca-ic doca-nav"><Icone><circle cx="12" cy="12" r="9" /><path d="M3 12h18M12 3a14 14 0 0 1 0 18M12 3a14 14 0 0 0 0 18" /></Icone><span className={`fl-ponto ${cor}`} /></span>
            <span className={`doca-ic doca-estado ${cor}`}>
              {cor === 'pausado' ? <Icone d="M9 5v14M15 5v14" /> : cor === 'enviando' ? <Icone d="M21 3L3 10.5l7 2.5 2.5 7z" /> : <Icone d="M20 14.5A8 8 0 1 1 9.5 4a6.5 6.5 0 0 0 10.5 10.5z" />}
            </span>
            <span className="doca-seta"><Icone d="M15 6l-6 6 6 6" /></span>
          </button>
        )}
      </aside>
    </>
  );
}
