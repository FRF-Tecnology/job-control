// Seus pedidos atendidos pelo app (jc-a2y.7.1.1; contrato 0.9.0, docs/propostas/pedidos-no-app.md). A IA conectada só
// PROPÕE uma ação de um catálogo fechado; nada grava sem a pessoa apertar "Confirmar" aqui, que chama a rota de sempre
// com o corpo EXATO da proposta e os cabeçalhos X-Pedido e X-Impressao. Tudo o que vem do pedido ou da IA aparece só
// como texto (sem HTML e sem link automático), e os erros viram frases fixas.
import { useEffect, useRef, useState } from 'react';
import { ErroApi, useApp } from '../app/contexto';
import { encode, nomeEtapa, NOMES_SUB } from '../util';

type Chamada = { metodo: string; caminho: string; corpo: Record<string, unknown> };
type Proposta = { acao: string; resumo: string; chamada: Chamada | null; impressao: string; falta?: string[] };
export type Pedido = {
  id: string; texto: string; status: string; resposta?: string | null; concluidoEm?: string | null;
  proposta?: Proposta | null; resultado?: { vagaId?: string; abrir?: string } | null;
};

const ESTADO: Record<string, string> = { aberto: 'Aberto', proposta: 'Proposta da IA', recusado: 'A IA não fez', feito: 'Feito' };
// Rótulos dos campos do corpo da chamada (POST /vagas e /vagas/{id}/mover). Campo fora desta lista não é mostrado e
// trava o Confirmar: a tela só confirma o que consegue mostrar.
const CAMPOS: Record<string, string> = { link: 'Link', empresa: 'Empresa', titulo: 'Título', etapa: 'Etapa', motivo: 'Motivo', subEtapa: 'Subetapa' };
// As únicas chamadas que uma proposta pode trazer (as mesmas do contrato).
const CAMINHO_OK = /^\/vagas(\/[A-Za-z0-9._-]{1,80}\/mover)?$/;
const ABRIR_OK = /^#\/fila\?preparar=[A-Za-z0-9._-]{1,80}$/;

const valorCampo = (k: string, v: unknown) => {
  const t = String(v ?? '');
  if (k === 'etapa') return nomeEtapa(t);
  if (k === 'subEtapa') return NOMES_SUB[t] ?? t;
  return t;
};
const chamadaValida = (c: Chamada | null | undefined): c is Chamada =>
  !!c && c.metodo === 'POST' && CAMINHO_OK.test(c.caminho) && !!c.corpo && typeof c.corpo === 'object'
  && Object.keys(c.corpo).every(k => k in CAMPOS) && Object.values(c.corpo).every(v => typeof v === 'string');

const ERRO_PEDIR: Record<number, string> = {
  400: 'Conecte uma IA para o app fazer este pedido.',
  403: 'Só você pode pedir para a IA, por esta tela.',
  404: 'Este pedido não existe mais.',
  409: 'A IA já está lendo um pedido, ou este mudou. Espere e tente de novo.',
  422: 'A IA respondeu algo fora do que o app aceita. Nada foi feito; tente escrever o pedido de outro jeito.',
  502: 'A IA não respondeu agora. Tente de novo em instantes.',
};
const ERRO_CONFIRMAR: Record<number, string> = {
  403: 'Só você pode confirmar, por esta tela.',
  409: 'A proposta mudou. Peça para a IA de novo.',
};

// pedirJa: id do pedido que a pessoa acabou de enviar pela caixa. Ele vai para a IA sem outro clique, uma vez só e
// quando ninguém mais está ocupado (uma leitura por vez). A proposta que voltar continua esperando o Confirmar.
export function SeusPedidos({ pedidos, iaConectada, pedirJa = null, aoPedirJa }: { pedidos: Pedido[]; iaConectada: boolean; pedirJa?: string | null; aoPedirJa?: () => void }) {
  const app = useApp();
  const [ocupado, setOcupado] = useState<{ id: string; o: 'pedir' | 'confirmar' | 'descartar' } | null>(null);
  const secao = useRef<HTMLElement>(null);
  const jaPedido = useRef<string | null>(null);
  useEffect(() => {
    if (!pedirJa || !iaConectada || ocupado || jaPedido.current === pedirJa) return;
    const p = pedidos.find(x => x.id === pedirJa);
    if (!p) return; // o pedido ainda não chegou no estado
    jaPedido.current = pedirJa;
    aoPedirJa?.();
    if (p.status !== 'aberto') return;
    [...(secao.current?.querySelectorAll<HTMLElement>('li.pedido') ?? [])].find(li => li.dataset.pedido === p.id)?.scrollIntoView?.({ block: 'nearest' });
    pedir(p);
  }, [pedirJa, pedidos, iaConectada, ocupado]); // eslint-disable-line react-hooks/exhaustive-deps

  async function rodar(p: Pedido, o: 'pedir' | 'confirmar' | 'descartar', fn: () => Promise<unknown>, ok: string | null, erros: Record<number, string>) {
    setOcupado({ id: p.id, o });
    try {
      await fn();
      if (ok) app.aviso(ok);
    } catch (e) {
      app.aviso((e instanceof ErroApi && erros[e.status]) || 'Não deu certo. Tente de novo.', true);
    } finally {
      setOcupado(null);
      app.recarregar();
    }
  }

  const pedir = (p: Pedido) => rodar(p, 'pedir', () => app.api(`/api/v1/pedidos/${encode(p.id)}/interpretar`, 'POST', {}), 'A IA leu o pedido. Veja abaixo.', ERRO_PEDIR);
  const descartar = (p: Pedido) => rodar(p, 'descartar', () => app.api(`/api/v1/pedidos/${encode(p.id)}/descartar`, 'POST', {}), 'Proposta descartada. O pedido voltou a aberto.', {});
  const confirmar = (p: Pedido) => {
    const pr = p.proposta;
    if (!pr || !chamadaValida(pr.chamada)) return;
    const c = pr.chamada;
    return rodar(p, 'confirmar', () => app.api(`/api/v1${c.caminho}`, c.metodo, c.corpo, { 'X-Pedido': p.id, 'X-Impressao': pr.impressao }),
      pr.acao === 'adicionar-vaga' ? 'Vaga adicionada em Descoberta.' : 'Feito. A vaga mudou de etapa.', ERRO_CONFIRMAR);
  };

  const ninguemOcupado = !ocupado;
  return (
    <section ref={secao} className="bloco pedidos" aria-labelledby="pedidos-titulo">
      <header><h2 id="pedidos-titulo">Seus pedidos</h2><span className="num">{pedidos.filter(p => p.status !== 'feito').length}</span></header>
      {!iaConectada && pedidos.some(p => p.status === 'aberto') && (
        <p className="pequeno mudo">Conecte uma IA para o app fazer os pedidos aqui. Sem IA, digite /pedidos no Claude Code.</p>
      )}
      <ul className="lista-cartao">
        {pedidos.map(p => {
          const pr = p.proposta;
          const este = ocupado?.id === p.id ? ocupado.o : null;
          const abrir = p.resultado?.abrir && ABRIR_OK.test(p.resultado.abrir) ? p.resultado.abrir : null;
          const daIA = pr?.acao === 'responder';
          // Vaga criada ou movida pelo pedido confirmado; só se ela ainda existe.
          const vagaFeita = p.resultado?.vagaId && (app.estado.vagas ?? []).some((v: any) => v.id === p.resultado!.vagaId) ? p.resultado.vagaId : null;
          return (
            <li key={p.id} data-pedido={p.id} className={`pedido s-${p.status}`} aria-busy={este ? true : undefined}>
              <strong>{p.texto}</strong>
              <span className="estado">{ESTADO[p.status] ?? p.status}</span>

              {p.status === 'aberto' && (este === 'pedir'
                ? <p role="status">A IA está lendo o pedido…</p>
                : iaConectada && (
                  <div className="acoes">
                    <button className="botao p" type="button" disabled={!ninguemOcupado} onClick={() => pedir(p)} aria-label={`Pedir para a IA: ${p.texto}`}>Pedir para a IA</button>
                  </div>
                ))}

              {p.status === 'proposta' && pr && (
                <div className="proposta">
                  <p>{pr.resumo}</p>
                  {pr.chamada && (
                    <dl className="proposta-campos">
                      {Object.entries(pr.chamada.corpo ?? {}).filter(([k]) => k in CAMPOS).map(([k, v]) => (
                        <div key={k}><dt>{CAMPOS[k]}</dt><dd>{valorCampo(k, v)}</dd></div>
                      ))}
                    </dl>
                  )}
                  {!!pr.falta?.length && <ul className="pequeno mudo falta">{pr.falta.map(f => <li key={f}>{f}</li>)}</ul>}
                  <div className="acoes">
                    {pr.chamada && (
                      <button className="botao p" type="button" disabled={!ninguemOcupado || !chamadaValida(pr.chamada)} onClick={() => confirmar(p)}>
                        {este === 'confirmar' ? 'Confirmando…' : 'Confirmar'}
                      </button>
                    )}
                    <button className="botao-fantasma p" type="button" disabled={!ninguemOcupado} onClick={() => descartar(p)}>Descartar</button>
                  </div>
                  {pr.chamada && !chamadaValida(pr.chamada) && <p className="pequeno erro-resposta">Esta proposta não pode ser confirmada por aqui. Descarte e peça de novo.</p>}
                </div>
              )}

              {p.status === 'recusado' && pr && <p>{pr.resumo}</p>}

              {p.status === 'feito' && (
                <>
                  {abrir && pr && <p>{pr.resumo}</p>}
                  {abrir && <div className="acoes"><a className="botao p" href={abrir}>Abrir a Fila</a></div>}
                  {!abrir && vagaFeita && <div className="acoes"><button className="botao-fantasma p" type="button" onClick={() => app.abrirVaga(vagaFeita)}>Ver a vaga</button></div>}
                  {p.resposta && (
                    <div className="resposta-ia">
                      <span className="pequeno mudo">{daIA ? 'Resposta da IA' : 'Resposta'}</span>
                      <p>{p.resposta}</p>
                    </div>
                  )}
                </>
              )}
            </li>
          );
        })}
      </ul>
    </section>
  );
}
