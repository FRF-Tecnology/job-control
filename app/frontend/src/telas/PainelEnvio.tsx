// Envio (jc-a2y.51; achado 2 da revisão integrada da P5): a pessoa vê se o envio está ligado ou pausado, o que está
// sendo enviado agora e o desfecho de cada envio, e pausa ou retoma. Contrato: GET /envio → EstadoEnvio;
// POST /envio/pausar {} (qualquer autor) e /envio/retomar {} (só a tela), pelo api/openapi.yaml 0.4.12.
// Começa pausado sempre que o app abre (decisão do PO), e a tela diz isso. Pausar e retomar não mudam a /versao: a
// tela usa a resposta do POST. Os desfechos mudam a /versao, e a tela relê junto com a fila.
import { useEffect, useRef, useState } from 'react';
import { ErroApi, useApp, useTrava } from '../app/contexto';
import { AoVivo } from './AoVivo';
import { doAssistente, nomeAssistente, type Quem } from '../util';

// Envio sem atrapalhar o PC (jc-a2y.6.7, contrato 0.4.13): o navegador do envio roda sem janela. A pessoa pode ver o
// envio numa janela (POST /envio/janela, vale a partir da próxima candidatura) e entrar nas plataformas
// (POST /navegador/entrar): enquanto a janela de login está aberta, o envio espera (login true). As duas só pela tela.
type Desfecho = { id: string; estado: 'enviada' | 'bloqueada' | 'falhou' | 'incerta'; motivo: string | null; em: string };
// Estado por plataforma (jc-a2y.6.11, contrato do #101). Sem o campo (servidor anterior), o painel não mostra o bloco.
export type EstadoPlataforma = {
  plataforma: 'linkedin' | 'gupy' | 'inhire'; estado: 'ativa' | 'pausada' | 'precisa-entrar' | 'parada-hoje';
  motivo?: 'falhas-seguidas' | 'login' | 'captcha' | 'aviso-conta' | null; ate?: string | null;
  pausadaPor?: Quem; // jc-a2y.7.4: só em pausada sem motivo (pela pessoa ou pelo assistente)
  voltasHoje?: number; paginasHoje?: number; tetoPaginas?: number;
};
export type EstadoEnvio = {
  ligado: boolean; pausado: boolean; pausadoPor?: Quem; enviandoAgora: { id: string; desde: string } | null; desfechos: Desfecho[];
  janela?: 'oculta' | 'visivel'; login?: boolean; plataformas?: EstadoPlataforma[];
};

const NOME_PLATAFORMA: Record<EstadoPlataforma['plataforma'], string> = { linkedin: 'LinkedIn', gupy: 'Gupy', inhire: 'InHire' };
const PLATAFORMAS_ENVIO = new Set<string>(['linkedin', 'gupy', 'inhire']); // lista fechada do contrato
const ROTULO_ESTADO: Record<EstadoPlataforma['estado'], string> = { ativa: 'Ativa', pausada: 'Pausada', 'precisa-entrar': 'Precisa entrar', 'parada-hoje': 'Parada até amanhã' };
// Frases fixas por estado e motivo (lista fechada do contrato); nada vem da página.
function explicarPlataforma(p: EstadoPlataforma, volta: string) {
  switch (p.estado) {
    case 'ativa': return (p.tetoPaginas && (p.paginasHoje ?? 0) >= p.tetoPaginas) ? 'Chegou ao teto de páginas de hoje: espera até amanhã.' : 'Envia as aprovadas desta plataforma.';
    case 'pausada': return p.motivo === 'falhas-seguidas' ? 'Pausada depois de duas falhas seguidas iguais. Confira o que houve antes de retomar.'
      : doAssistente(p.pausadaPor) ? `Pausada pelo ${nomeAssistente(p.pausadaPor?.canal)}. Nada desta plataforma sai até você retomar.`
      : 'Pausada. Nada desta plataforma sai até você retomar.';
    case 'precisa-entrar': return p.plataforma === 'linkedin'
      ? 'A sessão caiu. Entre no LinkedIn; ao fechar a janela de login, ela volta sozinha. Ou retome aqui.'
      : 'A sessão caiu. Entre pelo cartão de uma candidatura desta plataforma (o login é por empresa); ao fechar a janela, ela volta sozinha. Ou retome aqui.';
    case 'parada-hoje': return `${p.motivo === 'aviso-conta' ? 'A plataforma mostrou um aviso sobre a sua conta: veja você antes de tentar de novo.' : p.motivo === 'captcha' ? 'A plataforma pediu um captcha.' : 'Parada por hoje.'} ${volta}`.trim();
  }
}
// "Volta amanhã às 00:00" (ou a data, se não for amanhã); data inválida não quebra a tela.
function quandoVolta(iso?: string | null) {
  const n = iso ? Date.parse(iso) : NaN;
  if (Number.isNaN(n)) return 'Volta sozinha amanhã.';
  const d = new Date(n), amanha = new Date(); amanha.setDate(amanha.getDate() + 1);
  const h = new Intl.DateTimeFormat('pt-BR', { hour: '2-digit', minute: '2-digit' }).format(d);
  return d.toDateString() === amanha.toDateString() ? `Volta sozinha amanhã às ${h}.` : `Volta sozinha em ${new Intl.DateTimeFormat('pt-BR', { day: 'numeric', month: 'short' }).format(d)} às ${h}.`;
}

// Abre a janela de login: no LinkedIn, ou na página da vaga (Gupy e InHire têm login por empresa). Frases fixas.
export async function entrarNavegador(app: { api: (c: string, m?: string, b?: unknown) => Promise<any> }, corpo: { plataforma: 'linkedin' } | { candidatura: string }): Promise<EstadoEnvio> {
  try {
    const e: EstadoEnvio = await app.api('/api/v1/navegador/entrar', 'POST', corpo);
    window.dispatchEvent(new CustomEvent(EVENTO_ENVIO, { detail: e }));
    return e;
  } catch (e) {
    if (e instanceof ErroApi && e.status === 409) {
      // Contrato 0.4.14 (jc-a2y.6.8): o motivo diz qual 409 é. Sem motivo (servidor anterior), vale a regra antiga.
      const motivo = e.dados?.motivo ?? (e.dados?.estado === 'enviando' ? 'enviando' : 'login-aberto');
      if (motivo === 'enviando') throw new Error('Uma candidatura está sendo enviada agora. Espere terminar e tente de novo.');
      if (motivo === 'sem-navegador') throw new Error('Não deu para abrir: este computador não tem o Chrome nem o Edge para o app usar.');
      throw new Error('Já há uma janela de login aberta. Feche-a antes de abrir outra.');
    }
    if (e instanceof ErroApi && e.status === 403) throw new Error('Só você pode fazer isso, por esta tela.');
    if (e instanceof ErroApi && e.status === 404) throw new Error('Não deu para abrir: o envio não está neste app ou a candidatura saiu da fila.');
    if (e instanceof ErroApi && e.status === 400) throw new Error('Não deu para abrir: o endereço desta vaga não é de uma página conhecida da plataforma.');
    throw new Error('Não deu para abrir a janela. Tente de novo.');
  }
}

const NOME_DESFECHO: Record<Desfecho['estado'], string> = { enviada: 'Enviada', bloqueada: 'Bloqueada', falhou: 'Falhou', incerta: 'Incerta' };
// Datas do servidor comparadas como instante (Date.parse), não como texto; data inválida não quebra a tela.
const instante = (iso: string) => { const n = Date.parse(iso); return Number.isNaN(n) ? -Infinity : n; };
const hora = (iso: string) => {
  const n = Date.parse(iso);
  return Number.isNaN(n) ? '—' : new Intl.DateTimeFormat('pt-BR', { hour: '2-digit', minute: '2-digit' }).format(new Date(n));
};
// A casca escuta este evento para mostrar a marca "Envio ligado" na hora, sem esperar a próxima leitura.
export const EVENTO_ENVIO = 'central:envio';

// nome(id): "Empresa — Título" da candidatura; motivo(m): rótulo do motivo de bloqueio (os mesmos da fila).
// O que a Fila tem para enviar (jc-a2y.6.10): as aprovadas, com a hora da aprovação (o envio vai da mais antiga para
// a mais nova), quantas candidaturas esperam revisão e quantas vagas estão prontas para preparar.
export type ResumoFila = { aprovadas: { id: string; em: string }[]; paraRevisar: number; prontas: number };
const contar = (n: number, um: string, varios: string) => `${n} ${n === 1 ? um : varios}`;

// onEstado: avisa a Fila do estado (ex.: se o envio existe neste app, para os botões de entrar nos cartões).
export function PainelEnvio({ nome, motivo, onEstado, fila }: { nome: (id: string) => string; motivo: (m: string) => string; onEstado?: (e: EstadoEnvio | null) => void; fila?: ResumoFila }) {
  const app = useApp();
  const trava = useTrava();
  const [envio, setEnvio] = useState<EstadoEnvio | null>(null);
  useEffect(() => { onEstado?.(envio); }, [envio]); // eslint-disable-line react-hooks/exhaustive-deps
  // Estado vindo de fora do painel (ex.: "entrar" pelo cartão da candidatura): só atualiza, sem avisos.
  useEffect(() => {
    const aoMudar = (ev: Event) => setEnvio((ev as CustomEvent<EstadoEnvio>).detail);
    window.addEventListener(EVENTO_ENVIO, aoMudar);
    return () => window.removeEventListener(EVENTO_ENVIO, aoMudar);
  }, []);
  const [falha, setFalha] = useState(false);
  const [ocupado, setOcupado] = useState(false);
  // Depois de pausar ou retomar, o botão troca (Pausar ↔ Retomar): o foco segue para o novo, não para o começo da página.
  const acao = useRef<HTMLButtonElement>(null);
  const focarAcao = useRef(false);
  useEffect(() => { if (focarAcao.current) { focarAcao.current = false; acao.current?.focus(); } }, [envio?.pausado]);
  const visto = useRef<number | null>(null); // instante mais novo já visto; só desfecho depois dele vira aviso

  const texto = (d: Desfecho) => {
    const v = nome(d.id);
    switch (d.estado) {
      case 'enviada': return `Enviada: ${v}.`;
      case 'bloqueada': return `Bloqueada (${motivo(d.motivo ?? 'outro')}): ${v}. Veja na fila o que falta.`;
      case 'falhou': return `Falhou: ${v}. Nada foi enviado.`;
      case 'incerta': return `Incerta: ${v}. Confira na plataforma se foi enviada.`;
    }
  };

  const aplicar = (e: EstadoEnvio) => {
    const novos = visto.current === null ? [] : e.desfechos.filter(d => instante(d.em) > visto.current!);
    for (const d of [...novos].reverse()) app.aviso(texto(d), d.estado !== 'enviada');
    visto.current = e.desfechos.reduce((m, d) => Math.max(m, instante(d.em)), visto.current ?? -Infinity);
    setEnvio(e); setFalha(false);
    window.dispatchEvent(new CustomEvent(EVENTO_ENVIO, { detail: e }));
  };

  const ler = async () => {
    try { aplicar(await app.api('/api/v1/envio')); }
    catch { setFalha(true); }
  };
  useEffect(() => { ler(); }, [app.estado]); // eslint-disable-line react-hooks/exhaustive-deps

  async function pausar() {
    setOcupado(true);
    try { focarAcao.current = true; aplicar(await app.api('/api/v1/envio/pausar', 'POST', {})); app.aviso(envio?.enviandoAgora ? 'Envio pausado. A candidatura que está sendo enviada agora termina; nenhuma outra sai até você retomar.' : 'Envio pausado. Nenhuma candidatura nova sai até você retomar.'); }
    catch { app.aviso('Não deu para pausar. Tente de novo.', true); }
    finally { setOcupado(false); }
  }
  async function retomar() {
    const ok = await app.perguntar({
      titulo: 'Retomar o envio?',
      texto: 'O app passa a enviar as candidaturas aprovadas, uma por vez, dentro do limite de hoje de cada plataforma. Você pode pausar a qualquer momento. Sempre que o app abrir, o envio volta pausado.',
      confirmar: 'Retomar envio',
    });
    if (!ok) return;
    setOcupado(true);
    try { focarAcao.current = true; aplicar(await app.api('/api/v1/envio/retomar', 'POST', {})); app.aviso('Envio retomado.'); }
    catch (e) { app.aviso(e instanceof ErroApi && e.status === 403 ? 'Só você pode retomar o envio, por esta tela.' : 'Não deu para retomar. Tente de novo.', true); }
    finally { setOcupado(false); }
  }

  // Ver o envio numa janela (ou voltar a esconder). Só vale a partir da próxima candidatura.
  async function verEnvio(visivel: boolean) {
    setOcupado(true);
    try {
      const e: EstadoEnvio = await app.api('/api/v1/envio/janela', 'POST', { visivel });
      aplicar(e);
      // O aviso segue o estado que voltou, não o pedido: se a janela não ficou visível, não promete que vai aparecer.
      if (visivel && e.janela !== 'visivel') app.aviso('Não deu para mostrar a janela do envio agora. Tente de novo.', true);
      else app.aviso(visivel ? 'A janela do envio aparece a partir da próxima candidatura.' : 'O envio volta a rodar sem janela.');
    } catch (e) {
      app.aviso(e instanceof ErroApi && e.status === 403 ? 'Só você pode abrir a janela do envio, por esta tela.'
        : e instanceof ErroApi && e.status === 404 ? 'O envio não está disponível neste app.'
        : 'Não deu para mudar a janela do envio. Tente de novo.', true);
    } finally { setOcupado(false); }
  }
  // Retomar uma plataforma: só a pessoa, por esta tela (x-so-tela). Parada até amanhã nem mostra o botão.
  async function retomarPlataforma(p: EstadoPlataforma) {
    // Só as plataformas da lista fechada viram caminho (nota do CyberSec no #114).
    if (!PLATAFORMAS_ENVIO.has(p.plataforma)) return;
    const nomeP = NOME_PLATAFORMA[p.plataforma];
    const ok = await app.perguntar({
      titulo: `Retomar ${nomeP}?`,
      texto: p.motivo === 'falhas-seguidas'
        ? `O envio ${nomeP === 'LinkedIn' ? 'no' : 'na'} ${nomeP} parou depois de duas falhas seguidas iguais. Retome só se você já viu o que houve.`
        : `O app volta a enviar as aprovadas ${nomeP === 'LinkedIn' ? 'no' : 'na'} ${nomeP}, dentro do limite de hoje.`,
      confirmar: `Retomar ${nomeP}`,
    });
    if (!ok) return;
    setOcupado(true);
    try { aplicar(await app.api(`/api/v1/envio/plataformas/${p.plataforma}/retomar`, 'POST', {})); app.aviso(`${nomeP} retomada.`); }
    catch (e) {
      app.aviso(e instanceof ErroApi && e.status === 403 ? 'Só você pode retomar, por esta tela.'
        : e instanceof ErroApi && e.status === 409 ? `${nomeP} já está ativa ou parada até amanhã. A tela foi atualizada.`
        : e instanceof ErroApi && e.status === 404 ? 'O envio não está disponível neste app.'
        : 'Não deu para retomar. Tente de novo.', true);
      ler();
    } finally { setOcupado(false); }
  }

  async function entrarLinkedin() {
    setOcupado(true);
    try { aplicar(await entrarNavegador(app, { plataforma: 'linkedin' })); app.aviso('Janela de login do LinkedIn aberta. O envio espera até você fechá-la.'); }
    catch (e) { app.aviso((e as Error).message, true); }
    finally { setOcupado(false); }
  }

  if (falha) return (
    <section className="envio" aria-labelledby="envio-titulo">
      <h2 id="envio-titulo">Envio</h2>
      <p className="mudo">Não deu para ler o estado do envio. Pausar continua disponível: só reduz o risco.</p>
      <div className="acoes">
        <button className="botao-fantasma" type="button" onClick={ler}>Tentar de novo</button>
        <button className="botao-fantasma" type="button" disabled={ocupado} onClick={pausar}>Pausar envio</button>
      </div>
    </section>
  );
  if (!envio) return <p className="mudo" role="status">Lendo o envio…</p>;
  if (!envio.ligado) return <p className="envio-desligado mudo">Envio automático desligado neste app: nada é enviado sozinho.</p>;

  const agora = envio.enviandoAgora;
  return (
    <section className={`envio${envio.pausado ? ' pausado' : ' ligado'}`} aria-labelledby="envio-titulo">
      <div className="envio-topo">
        <div>
          <h2 id="envio-titulo">{envio.pausado ? 'Envio pausado' : 'Envio ligado'}</h2>
          <p className="mudo">{envio.pausado
            ? (agora ? 'A candidatura que está sendo enviada agora termina; nenhuma outra começa até você retomar.' : 'Nada é enviado até você retomar. As aprovadas esperam.')
            : (agora ? 'O app envia as aprovadas, uma por vez. Se pausar, a candidatura que está sendo enviada agora termina.' : 'O app envia as candidaturas aprovadas, uma por vez.')}</p>
          {!envio.pausado && <p className="pequeno mudo">Sempre que o app abrir, o envio volta pausado.</p>}
          {envio.pausado && doAssistente(envio.pausadoPor) && <p className="pequeno pausado-por">{`Pausado pelo ${nomeAssistente(envio.pausadoPor?.canal)}.`}</p>}
        </div>
        {envio.pausado
          ? <button ref={acao} className="botao" type="button" {...trava(ocupado)} onClick={retomar}>Retomar envio</button>
          : <button ref={acao} className="botao-fantasma" type="button" disabled={ocupado} onClick={pausar}>Pausar envio</button>}
      </div>
      {!agora && fila && (() => {
        const proxima = [...fila.aprovadas].sort((a, b) => instante(a.em) - instante(b.em))[0];
        const contagem = [contar(fila.prontas, 'vaga pronta para preparar', 'vagas prontas para preparar'), contar(fila.paraRevisar, 'candidatura para revisar', 'candidaturas para revisar'), contar(fila.aprovadas.length, 'aprovada', 'aprovadas')].join(' · ');
        return (
          <div className="envio-proxima">
            <p>{proxima
              ? `Próxima a enviar${envio.pausado ? ' quando você retomar' : ''}: ${nome(proxima.id)}`
              : 'Nada para enviar agora: prepare e aprove uma candidatura abaixo.'}</p>
            <p className="pequeno mudo">{contagem}</p>
          </div>
        );
      })()}
      {!!envio.plataformas?.length && (
        <div className="envio-plataformas">
          <h3>Plataformas</h3>
          <ul>
            {envio.plataformas.map(p => {
              const nomeP = NOME_PLATAFORMA[p.plataforma] ?? p.plataforma;
              const paginas = typeof p.paginasHoje === 'number' && typeof p.tetoPaginas === 'number'
                ? `${p.paginasHoje} de ${p.tetoPaginas} páginas hoje${p.voltasHoje ? ` (${contar(p.voltasHoje, 'volta', 'voltas')})` : ''}` : null;
              const podeRetomar = (p.estado === 'pausada' || p.estado === 'precisa-entrar') && PLATAFORMAS_ENVIO.has(p.plataforma);
              return (
                <li key={p.plataforma} className={`plat-envio e-${p.estado}`}>
                  <div className="plat-topo">
                    <strong>{nomeP}</strong>
                    <span className="chip">{p.estado === 'parada-hoje' && p.motivo === 'aviso-conta' ? 'Aviso da conta' : ROTULO_ESTADO[p.estado] ?? 'Pausada'}</span>
                  </div>
                  <p className="pequeno">{explicarPlataforma(p, quandoVolta(p.ate))}</p>
                  {paginas && <p className="pequeno mudo">{paginas}</p>}
                  {podeRetomar && (
                    <div className="acoes">
                      <button className="botao-fantasma p" type="button" {...trava(ocupado)} onClick={() => retomarPlataforma(p)}>{`Retomar ${nomeP}`}</button>
                      {p.estado === 'precisa-entrar' && p.plataforma === 'linkedin' && <button className="botao-fantasma p" type="button" {...trava(ocupado || !!envio.login)} onClick={entrarLinkedin}>Entrar no LinkedIn</button>}
                    </div>
                  )}
                </li>
              );
            })}
          </ul>
        </div>
      )}
      {envio.login && <p className="login-aberto" role="status">Janela de login aberta. O envio espera até você fechá-la; depois, volta sozinho.</p>}
      <div className="envio-janelas">
        <div>
          <h3>Ver o envio</h3>
          <p className="pequeno mudo">{envio.janela === 'visivel'
            ? 'O navegador do envio aparece numa janela a partir da próxima candidatura. A que está sendo enviada agora termina sem janela.'
            : 'O envio roda sem janela, para não atrapalhar o uso do PC. Sempre que o app abrir, ele volta sem janela.'}</p>
          <button className="botao-fantasma p" type="button" {...trava(ocupado)} onClick={() => verEnvio(envio.janela !== 'visivel')}>
            {envio.janela === 'visivel' ? 'Esconder o envio' : 'Ver o envio'}
          </button>
        </div>
        <div>
          <h3>Entrar nas plataformas</h3>
          <p className="pequeno mudo">Abre uma janela no login para você entrar com a sua conta. Enquanto ela estiver aberta, o envio espera. Na Gupy e na InHire o login é por empresa: entre pelo cartão da candidatura.</p>
          <button className="botao-fantasma p" type="button" {...trava(ocupado || !!envio.login)} onClick={entrarLinkedin}>Entrar no LinkedIn</button>
        </div>
      </div>
      <AoVivo nome={nome} />
      {agora && <p className="enviando-agora" role="status"><span className="ponto" aria-hidden="true" />{`Enviando agora: ${nome(agora.id)}, desde ${hora(agora.desde)}`}</p>}
      {envio.desfechos.length > 0 && (
        <details className="tecnicos">
          <summary>{`Últimos envios (${envio.desfechos.length})`}</summary>
          <ul className="desfechos">
            {envio.desfechos.map(d => (
              <li key={`${d.id}-${d.em}`}>
                <span className={`chip cand-${d.estado}`}>{d.estado === 'bloqueada' ? `Bloqueada · ${motivo(d.motivo ?? 'outro')}` : NOME_DESFECHO[d.estado]}</span>
                <span>{nome(d.id)}</span>
                <span className="pequeno mudo">{hora(d.em)}</span>
              </li>
            ))}
          </ul>
        </details>
      )}
    </section>
  );
}
