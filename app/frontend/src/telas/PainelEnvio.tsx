// Envio (jc-a2y.51; achado 2 da revisão integrada da P5): a pessoa vê se o envio está ligado ou pausado, o que está
// sendo enviado agora e o desfecho de cada envio, e pausa ou retoma. Contrato: GET /envio → EstadoEnvio;
// POST /envio/pausar {} (qualquer autor) e /envio/retomar {} (só a tela), pelo api/openapi.yaml 0.4.12.
// Começa pausado sempre que o app abre (decisão do PO), e a tela diz isso. Pausar e retomar não mudam a /versao: a
// tela usa a resposta do POST. Os desfechos mudam a /versao, e a tela relê junto com a fila.
import { useEffect, useRef, useState } from 'react';
import { ErroApi, useApp } from '../app/contexto';

type Desfecho = { id: string; estado: 'enviada' | 'bloqueada' | 'falhou' | 'incerta'; motivo: string | null; em: string };
export type EstadoEnvio = { ligado: boolean; pausado: boolean; enviandoAgora: { id: string; desde: string } | null; desfechos: Desfecho[] };

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
export function PainelEnvio({ nome, motivo }: { nome: (id: string) => string; motivo: (m: string) => string }) {
  const app = useApp();
  const [envio, setEnvio] = useState<EstadoEnvio | null>(null);
  const [falha, setFalha] = useState(false);
  const [ocupado, setOcupado] = useState(false);
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
    try { aplicar(await app.api('/api/v1/envio/pausar', 'POST', {})); app.aviso(envio?.enviandoAgora ? 'Envio pausado. A candidatura que está sendo enviada agora termina; nenhuma outra sai até você retomar.' : 'Envio pausado. Nenhuma candidatura nova sai até você retomar.'); }
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
    try { aplicar(await app.api('/api/v1/envio/retomar', 'POST', {})); app.aviso('Envio retomado.'); }
    catch (e) { app.aviso(e instanceof ErroApi && e.status === 403 ? 'Só você pode retomar o envio, por esta tela.' : 'Não deu para retomar. Tente de novo.', true); }
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
        </div>
        {envio.pausado
          ? <button className="botao" type="button" disabled={ocupado} onClick={retomar}>Retomar envio</button>
          : <button className="botao-fantasma" type="button" disabled={ocupado} onClick={pausar}>Pausar envio</button>}
      </div>
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
