// Triagem: uma vaga por vez. Arraste o cartão (direita aplica, esquerda pula) ou use os botões / teclas A, P, →.
// Porte de web/telas/triagem.mjs.
import { useEffect, useRef, useState, type CSSProperties } from 'react';
import { useApp, useTrava, type App } from '../app/contexto';
import { MOTIVOS_PULO, mercadoDe, noFoco } from '../dominio/modelo';
import { Icone, LinkExterno } from '../componentes';
import { BuscaDoPerfil } from './BuscaDoPerfil';
import { dataCurta, encode, nomePlataforma, plural, safeHref } from '../util';

const DECISOES: Record<string, string> = { aplicar: 'Aplicar', pular: 'Pular', perguntar: 'Na dúvida' };

// Fila: grupo A primeiro, depois B, C e sem grupo; dentro do grupo, as mais recentes.
const ordem = (v: any) => ({ A: 0, B: 1, C: 2 } as Record<string, number>)[v.aderencia] ?? 3;
const LIMIAR = 110; // px arrastados para decidir
let puladasNestaSessao = new Set<string>(); // "Decidir depois": vale até fechar o app
let ocupado = false; // evita agir duas vezes na mesma vaga se a pessoa apertar teclas rápido

export function TelaTriagem() {
  const app = useApp();
  const [, redesenhar] = useState(0);
  const foco = app.foco;
  const todas = app.estado.vagas.filter((v: any) => v.etapa === 'descoberta');
  const fila = todas
    .filter((v: any) => !puladasNestaSessao.has(v.id) && noFoco(v, foco))
    .sort((a: any, b: any) => ordem(a) - ordem(b) || (b.descobertaEm ?? '').localeCompare(a.descobertaEm ?? ''));
  const foraDoFoco = todas.filter((v: any) => !noFoco(v, foco)).length;
  const feitas = app.estado.vagas.filter((v: any) => v.etapa !== 'descoberta' && app.estado.eventos.some((e: any) => e.vagaId === v.id && e.autor === 'eu' && (e.tipo === 'aplicar' || e.tipo === 'pulada') && e.data === app.hoje)).length;
  const v = fila[0];
  const grupoA = fila.filter((x: any) => x.aderencia === 'A');

  const trava = useTrava();
  // Com a trava ligada (ruling A13), decidir não faz nada além de avisar (teclas, gesto e botões).
  const umaVez = (fn: () => Promise<unknown>) => async () => { if (trava.travado) { app.aviso(trava.frase, true); return; } if (ocupado) return; ocupado = true; try { await fn(); } finally { ocupado = false; } };
  const aplicar = v && umaVez(() => app.mover(v.id, 'aplicar', {}, `${v.empresa}: vai para a Fila`));
  const pular = v && umaVez(async () => {
    const motivo = await app.perguntar({ titulo: 'Por que pular?', texto: `${v.empresa} — ${v.titulo}`, opcoes: MOTIVOS_PULO });
    if (motivo) await app.mover(v.id, 'pulada', { motivo }, 'Pulada');
  });
  const depois = () => { if (v) { puladasNestaSessao.add(v.id); redesenhar(n => n + 1); } };

  // Aprovar todas do grupo A (jc-a2y.31): a pessoa vê a lista e a contagem e confirma. Pela tela, o autor é "eu".
  const aprovarGrupoA = umaVez(async () => {
    const lista = grupoA;
    const quantas = plural(lista.length, 'vaga', 'vagas');
    const ok = await app.perguntar({
      titulo: `Aprovar ${quantas} do grupo A?`,
      texto: 'Elas vão para a Fila, onde você prepara e aprova o pacote de cada uma. As outras continuam aqui.',
      itens: lista.map((x: any) => `${x.empresa} — ${x.titulo}`),
      confirmar: `Aprovar ${quantas}`,
    });
    if (ok === null) return;
    try {
      const r = await aprovarLista(app, lista);
      await app.recarregar();
      if (r.aprovadas || r.mudaram) {
        const feito = r.aprovadas ? `${plural(r.aprovadas, 'vaga foi', 'vagas foram')} para a Fila.` : 'Nenhuma vaga foi aprovada.';
        app.aviso(r.mudaram ? `${feito} ${plural(r.mudaram, 'mudou antes e ficou como estava', 'mudaram antes e ficaram como estavam')}.` : feito);
      }
      r.falhas.forEach(f => app.aviso(f, true));
    } catch (e) {
      app.aviso((e as Error).message, true);
    }
  });

  useEffect(() => {
    if (!v) return;
    const ouvinte = (ev: KeyboardEvent) => {
      if (location.hash !== '#/triagem' || (document.getElementById('dialogo') as HTMLDialogElement)?.open || (ev.target as Element).matches('input,textarea,select') || ev.ctrlKey || ev.metaKey || ev.altKey) return;
      const k = ev.key.toLowerCase();
      if (k === 'a') { ev.preventDefault(); aplicar!(); }
      else if (k === 'p') { ev.preventDefault(); pular!(); }
      else if (k === 'arrowright' || k === 'd') { ev.preventDefault(); depois(); }
    };
    document.addEventListener('keydown', ouvinte);
    return () => document.removeEventListener('keydown', ouvinte);
  });

  const cabecalho = (
    <><header className="cabecalho">
      <div>
        <h1>Triagem</h1>
        <p>{todas.length
          ? `${fila.length ? plural(fila.length, 'vaga espera', 'vagas esperam') : 'Nenhuma vaga espera'} decisão${foco !== 'ambos' ? ` no foco ${foco}` : ''}. Hoje você já decidiu ${feitas}.`
          : 'Nenhuma vaga esperando decisão.'}</p>
      </div>
      {grupoA.length > 1 && (
        <button className="botao-fantasma" type="button" {...trava()} onClick={aprovarGrupoA}>{`Aprovar todas do grupo A (${grupoA.length})`}</button>
      )}
    </header><BuscaDoPerfil /></>
  );

  if (!v) {
    const texto = puladasNestaSessao.size ? `${plural(puladasNestaSessao.size, 'vaga ficou', 'vagas ficaram')} para depois.`
      : foraDoFoco ? `Há ${plural(foraDoFoco, 'vaga', 'vagas')} fora do foco ${foco}.` : 'Busque vagas com seu perfil para iniciar a triagem.';
    return (
      <div className="triagem">
        {cabecalho}
        <div className="convite">
          <div><strong>{app.estado.vagas.length ? 'Tudo decidido' : 'Sua triagem começa aqui'}</strong><p style={{ margin: '4px 0 0' }}>{texto}</p></div>
          {puladasNestaSessao.size ? <button className="botao-fantasma" type="button" onClick={() => { puladasNestaSessao = new Set(); redesenhar(n => n + 1); }}>Rever as adiadas</button>
            : foraDoFoco ? <button className="botao" type="button" onClick={() => app.mudarFoco('ambos')}>Ver todas</button>
              : <a className="botao" href="#/quadro">Ver o quadro</a>}
        </div>
      </div>
    );
  }

  const mercado = mercadoDe(v);
  const chips = [v.nivel, v.contrato, v.modelo, v.idioma === 'en' ? 'Vaga em inglês' : null, ...(v.stack ?? [])].filter(Boolean);
  const feitosPct = todas.length ? Math.round(((todas.length - fila.length) / todas.length) * 100) : 0;

  return (
    <div className="triagem">
      {cabecalho}
      <div className="progresso" role="progressbar" aria-valuenow={feitosPct} aria-valuemin={0} aria-valuemax={100} aria-label="Progresso da triagem">
        <i style={{ '--p': String(feitosPct / 100) } as CSSProperties} />
      </div>
      <div className="pilha">
        <Ficha key={v.id} v={v} mercado={mercado} chips={chips} aoAplicar={aplicar!} aoPular={pular!} depois={depois} />
      </div>
      <div className="decisao">
        <button className="redondo" type="button" aria-label="Pular vaga (tecla P)" {...trava()} onClick={pular!}><Icone d="M7 7l10 10M17 7L7 17" /></button>
        <button className="redondo menor" type="button" aria-label="Ver detalhes da vaga" onClick={() => app.abrirVaga(v.id)}><Icone><circle cx="12" cy="12" r="8" /><path d="M12 11v5M12 8v.01" /></Icone></button>
        <button className="redondo sim" type="button" aria-label="Aplicar na vaga (tecla A)" {...trava()} onClick={aplicar!}><Icone d="M6 12.5l4 4L18 8" /></button>
      </div>
      <div className="legenda-decisao" aria-hidden="true"><span>Pular · P</span><span>Detalhes</span><span>Aplicar · A</span></div>
      <p className="pequeno mudo" style={{ textAlign: 'center', marginTop: '18px' }}>{`${fila.length - 1 > 0 ? `Depois desta, mais ${fila.length - 1}.` : 'É a última.'} Aprovadas vão para a Fila; o envio acontece pelo app quando você retomar.`}</p>
    </div>
  );
}

// Reconfere o estado antes de gravar: vaga que saiu da Descoberta ou do grupo A, ou cuja empresa ou título
// mudou desde a confirmação, fica como está (a pessoa aprovou o que viu). Grava uma por vez e recarrega uma vez só.
async function aprovarLista(app: App, lista: any[]) {
  const atuais = new Map<string, any>(((await app.api('/api/v1/estado')).vagas ?? []).map((x: any) => [x.id, x]));
  let aprovadas = 0;
  let mudaram = 0;
  const falhas: string[] = [];
  for (const x of lista) {
    const agora = atuais.get(x.id);
    const igual = agora && agora.etapa === 'descoberta' && agora.aderencia === 'A' && agora.empresa === x.empresa && agora.titulo === x.titulo;
    if (!igual) { mudaram++; continue; }
    try {
      await app.api(`/api/v1/vagas/${encode(x.id)}/mover`, 'POST', { etapa: 'aplicar' });
      aprovadas++;
    } catch (e) {
      falhas.push(`Não deu para aprovar ${x.empresa}: ${(e as Error).message}`);
    }
  }
  return { aprovadas, mudaram, falhas };
}

// O cartão da vaga. Arrastar com dedo ou mouse: a decisão vem do lado em que ele é solto.
function Ficha({ v, mercado, chips, aoAplicar, aoPular, depois }: { v: any; mercado: string; chips: string[]; aoAplicar: () => void; aoPular: () => void; depois: () => void }) {
  const trava = useTrava();
  const app = useApp();
  const ficha = useRef<HTMLElement>(null);
  const sim = useRef<HTMLSpanElement>(null);
  const nao = useRef<HTMLSpanElement>(null);
  const arrasto = useRef<{ inicio: number | null; dx: number }>({ inicio: null, dx: 0 });
  const [opiniao, setOpiniao] = useState<any>(null);
  const [pensando, setPensando] = useState(false);
  const pedirOpiniao = async () => {
    setPensando(true);
    try { setOpiniao(await app.api(`/api/v1/ia/triar/${encode(v.id)}`, 'POST', {})); }
    catch (e: any) { app.aviso(e.status === 400 ? 'Conecte uma IA em Mais › Conectar IA para pedir opinião.' : e.message, true); }
    finally { setPensando(false); }
  };

  const mover = (x: number) => {
    if (!ficha.current) return;
    ficha.current.style.transform = `translateX(${x}px) rotate(${x / 18}deg)`;
    if (sim.current) sim.current.style.opacity = String(Math.max(0, Math.min(1, x / LIMIAR)));
    if (nao.current) nao.current.style.opacity = String(Math.max(0, Math.min(1, -x / LIMIAR)));
  };
  const voltar = () => { ficha.current?.classList.remove('arrastando'); mover(0); if (ficha.current) ficha.current.style.transform = ''; };
  const soltar = () => {
    const a = arrasto.current;
    if (a.inicio == null) return;
    a.inicio = null;
    ficha.current?.classList.remove('arrastando');
    if (trava.travado && Math.abs(a.dx) > LIMIAR) { voltar(); aoAplicar(); return; } // travado: o cartão volta e aoAplicar só avisa
    if (a.dx > LIMIAR && ficha.current) { ficha.current.style.transform = 'translateX(120vw) rotate(24deg)'; ficha.current.style.opacity = '0'; aoAplicar(); }
    else if (a.dx < -LIMIAR) { voltar(); aoPular(); }
    else voltar();
  };

  return (
    <article className="ficha" ref={ficha} aria-live="polite" aria-roledescription="cartão de vaga"
      onPointerDown={ev => {
        if (ev.button !== 0 || (ev.target as Element).closest('a, button')) return;
        arrasto.current = { inicio: ev.clientX, dx: 0 };
        ficha.current?.setPointerCapture(ev.pointerId);
        ficha.current?.classList.add('arrastando');
      }}
      onPointerMove={ev => { const a = arrasto.current; if (a.inicio != null) { a.dx = ev.clientX - a.inicio; mover(a.dx); } }}
      onPointerUp={soltar}
      onPointerCancel={() => { arrasto.current.inicio = null; voltar(); }}>
      <span className="selo-gesto aplicar" aria-hidden="true" ref={sim}>Aplicar</span>
      <span className="selo-gesto pular" aria-hidden="true" ref={nao}>Pular</span>
      <div className="acoes" style={{ justifyContent: 'space-between', alignItems: 'center' }}>
        <span className="acoes">
          {v.aderencia && <span className={`chip ader-${v.aderencia}`}>{`Aderência ${v.aderencia}`}</span>}
          {v.triagemBusca === 'pendente' && <span className="chip">Avaliação pendente</span>}
          <span className={`chip m-${mercado}`}>{mercado === 'internacional' ? 'Internacional' : 'Nacional'}</span>
        </span>
        <span className="chip">{`encontrada ${dataCurta(v.descobertaEm)}`}</span>
      </div>
      <div><div className="empresa">{v.empresa}</div><h2 style={{ marginTop: '6px' }}>{v.titulo}</h2></div>
      <div className="acoes">
        <span className="chip contorno">{nomePlataforma(v.plataforma, app.estado.plataformas)}</span>
        {v.fonteBusca && <LinkExterno className="chip contorno fonte-busca" url={safeHref(v.fonteURL) ?? v.link}
          semLink={<span className="chip contorno">Fonte: {v.fonteBusca}</span>}>Fonte: {v.fonteBusca}</LinkExterno>}
        {chips.map((c, i) => <span key={i} className="chip contorno">{c}</span>)}
      </div>
      {v.motivoClaude && <div className="motivo"><b><Icone d="M12 3l1.8 4.5L18 9l-4.2 1.5L12 15l-1.8-4.5L6 9l4.2-1.5z" />Por que aparece</b>{v.motivoClaude}</div>}
      {v.triagemDecisao === 'pular' && v.triagemMotivoPulo && <div className="motivo">
        <b>A IA sugere pular</b>{v.triagemMotivoPulo}
      </div>}
      {Array.isArray(v.triagemConferir) && v.triagemConferir.length > 0 && <div className="motivo">
        <b>Pontos a conferir</b><ul>{v.triagemConferir.map((ponto: string, i: number) => <li key={i}>{ponto}</li>)}</ul>
      </div>}
      {v.notas && <div className="motivo"><b>Notas</b>{v.notas}</div>}
      {opiniao && <div className="motivo" role="status"><b>Opinião da IA</b>{`${DECISOES[opiniao.decisao] ?? opiniao.decisao} · grupo ${opiniao.aderencia}${opiniao.motivoPulo ? ` · ${opiniao.motivoPulo}` : ''}. ${opiniao.motivo}`}</div>}
      <div className="acoes">
        <LinkExterno url={v.link} className="botao-fantasma p">Ler a vaga completa</LinkExterno>
        <button className="botao-fantasma p" type="button" onClick={depois}>Decidir depois</button>
        {!opiniao && <button className="botao-fantasma p" type="button" {...trava(pensando)} onClick={pedirOpiniao}>{pensando ? 'A IA está lendo…' : 'Opinião da IA'}</button>}
      </div>
      <p className="dica-gesto" aria-hidden="true"><Icone d="M5 12h14M15 8l4 4-4 4" />Arraste para a direita para aplicar, para a esquerda para pular</p>
    </article>
  );
}
