// Hoje: uma caixa "O que vamos fazer hoje?", o seletor de foco e só o que pede ação (porte de web/telas/hoje.mjs).
import { useEffect, useRef, useState } from 'react';
import { useApp, type App } from '../app/contexto';
import { pendentesFollowUp, sugerirMensagem } from '../dominio/followup';
import { foiEnviada, normalizar, noFoco } from '../dominio/modelo';
import { NOMES_EMAIL } from '../dominio/emails';
import { Icone, LinkExterno, Robo } from '../componentes';
import { SeusPedidos, type Pedido } from './Pedidos';
import { AvisoPasta } from './AvisoPasta';
import { dataLonga, haQuanto, dataCurta, nomeEtapa, NOMES_SUB, plural, encode, nomeAssistente } from '../util';

const FOCOS = [['internacional', 'Internacional'], ['nacional', 'Nacional'], ['ambos', 'Os dois']];
const TIPOS_EVENTO: Record<string, string> = { descoberta: 'Encontrada', aplicar: 'Aprovada para aplicar', enviada: 'Enviada', bloqueada: 'Travou', 'em-processo': 'Em processo', oferta: 'Oferta', encerrada: 'Encerrada', pulada: 'Pulada', 'follow-up': 'Follow-up', nota: 'Nota', retorno: 'Retorno', teste: 'Teste', entrevista: 'Entrevista', email: 'Email' };
const FUNIL = [
  ['descoberta', 'Descoberta', 'triagem'], ['aplicar', 'Para aplicar', 'quadro'], ['enviada', 'Enviadas', 'quadro'],
  ['em-processo', 'Em processo', 'quadro'], ['oferta', 'Oferta', 'quadro'],
];

// Pedidos que a própria tela resolve na hora; o resto vira pedido para o assistente (/pedidos).
const ATALHOS: [RegExp, string][] = [
  // As aprovadas se preparam e se aprovam na Fila; o envio é pelo app, não um pedido ao assistente (jc-a2y.56).
  [/\bfila\b|\bprepar/, '#/fila'],
  [/\btri(ar|agem)\b|decidir|vagas novas/, '#/triagem'],
  [/respond|retorno|entrevista|processo|oferta/, '#/quadro'],
  [/n[uú]meros|taxa|funil|semana|m[eé]tric/, '#/numeros'],
  [/perfil|pretens[aã]o|curr[ií]culo/, '#/perfil'],
  [/plataforma|fonte/, '#/plataformas'],
];

// Com IA conectada, enviar o pedido já é pedir para a IA: o pedido criado vai direto para a leitura (aoCriar), sem um
// segundo clique em "Seus pedidos". Sem IA, ele fica anotado para o assistente (/pedidos).
export function CaixaPedido({ sugestoes, iaConectada, aoCriar }: { sugestoes: string[]; iaConectada: boolean; aoCriar: (id: string) => void }) {
  const app = useApp();
  const [texto, setTexto] = useState('');
  const entrada = useRef<HTMLInputElement>(null);
  async function enviar(t0: string) {
    const t = t0.trim();
    if (!t) return entrada.current?.focus();
    const atalho = ATALHOS.find(([re]) => re.test(normalizar(t)));
    if (atalho && !/\b(busca|aplica|envia|manda|escreve)/.test(normalizar(t))) { location.hash = atalho[1]; return; }
    let criado: { id?: unknown } | null = null;
    const ok = await app.agir(async () => { criado = await app.api('/api/v1/pedidos', 'POST', { texto: t }); },
      iaConectada ? undefined : 'Anotado em Seus pedidos, logo abaixo.');
    if (!ok) return;
    setTexto('');
    const id = (criado as { id?: unknown } | null)?.id;
    if (iaConectada && typeof id === 'string' && id) aoCriar(id);
  }
  return (
    <>
      <form className="caixa-pedido" role="search" onSubmit={ev => { ev.preventDefault(); enviar(texto); }}>
        <span className="caixa-ic" aria-hidden="true"><Icone d="M12 3l1.8 5.2L19 10l-5.2 1.8L12 17l-1.8-5.2L5 10l5.2-1.8zM19 16l.8 2.2L22 19l-2.2.8L19 22l-.8-2.2L16 19l2.2-.8z" /></span>
        <input id="pedido" ref={entrada} type="text" autoComplete="off" placeholder={sugestoes[0] ?? 'Peça qualquer coisa'} aria-label="O que vamos fazer hoje?" value={texto} onChange={e => setTexto(e.target.value)} />
        <button type="submit" aria-label="Enviar pedido"><Icone d="M12 19V5M5 12l7-7 7 7" /></button>
      </form>
      <div className="sugestoes" aria-label="Sugestões">
        {sugestoes.map(s => <button key={s} type="button" onClick={() => enviar(s)}>{s}</button>)}
      </div>
    </>
  );
}

function sugestoesPara(foco: string, nAplicar: number, nDescoberta: number) {
  const buscar = ({ internacional: 'Buscar vagas USD LATAM', nacional: 'Buscar vagas CLT/PJ remotas', ambos: 'Buscar vagas novas' } as Record<string, string>)[foco];
  const qual = ({ internacional: ' internacionais', nacional: ' nacionais', ambos: '' } as Record<string, string>)[foco];
  const qualUma = ({ internacional: ' internacional', nacional: ' nacional', ambos: '' } as Record<string, string>)[foco];
  const preparar = nAplicar === 1 ? `Preparar a vaga${qualUma} aprovada na Fila` : `Preparar as ${nAplicar} vagas${qual} aprovadas na Fila`;
  return [
    nAplicar ? preparar : buscar,
    nAplicar ? buscar : null,
    nDescoberta ? `Triar ${plural(nDescoberta, 'vaga nova', 'vagas novas')}` : null,
    'Quem me respondeu?',
  ].filter(Boolean) as string[];
}

// Título curto da pendência: primeira frase, sem links. Tudo que pede CPF vira um grupo só.
export function resumoPendencia(texto: unknown) {
  const limpo = String(texto ?? '').replace(/https?:\/\/\S+/g, '').replace(/\s+/g, ' ').trim();
  if (/\bcpf\b/i.test(limpo)) return 'Digitar o CPF (o resto do formulário já está pronto)';
  return limpo.split(/(?<=\.)\s| — /)[0].replace(/[.:,;]+$/, '').trim() || 'Sem pendência anotada';
}

// Bloqueadas com a mesma pendência viram uma linha só ("Gupy exige login — 2 vagas").
function agruparPendencias(bloqueadas: any[]) {
  const grupos = new Map<string, { texto: string; vagas: any[] }>();
  for (const v of bloqueadas) {
    const texto = resumoPendencia(v.pendencia);
    const k = normalizar(texto);
    if (!grupos.has(k)) grupos.set(k, { texto, vagas: [] });
    grupos.get(k)!.vagas.push(v);
  }
  return [...grupos.values()].sort((a, b) => b.vagas.length - a.vagas.length);
}

function AcoesBloqueada({ v }: { v: any }) {
  const app = useApp();
  return (
    <>
      <LinkExterno url={v.link} className="botao p">Abrir vaga</LinkExterno>
      <button className="botao-fantasma p" type="button" onClick={() => app.mover(v.id, 'enviada', {}, `${v.empresa}: marcada como enviada`)}>Enviei</button>
      <button className="botao-fantasma p" type="button" onClick={async () => {
        const m = await app.perguntar({ titulo: 'Desistir desta vaga?', opcoes: ['Documento', 'Outro'], campo: 'Motivo', confirmar: 'Pular vaga' });
        if (m) app.mover(v.id, 'pulada', { motivo: m }, 'Vaga pulada');
      }}>Desistir</button>
    </>
  );
}

const IconeEmail = () => <Icone><rect x="3" y="5" width="18" height="14" rx="2" /><path d="M3 7l9 6 9-6" /></Icone>;

// Email que chegou e não deu para saber de qual vaga é: a pessoa liga à vaga certa (a etapa muda) ou descarta.
function EmailSemVaga({ e, opcoes }: { e: any; opcoes: any[] }) {
  const app = useApp();
  const [escolha, setEscolha] = useState('');
  const sel = useRef<HTMLSelectElement>(null);
  const ir = (acao: string, corpo: object, msg: string) => app.agir(() => app.api(`/api/v1/emails/${encode(e.idMensagem)}/${acao}`, 'POST', corpo), msg);
  return (
    <li className="item-acao">
      <span className="selo-n"><IconeEmail /></span>
      <div><strong>{`${NOMES_EMAIL[e.tipo] ?? 'Email'}: ${e.assunto || '(sem assunto)'}`}</strong><span className="sub">{[e.de, e.resumo].filter(Boolean).join(' · ')}</span></div>
      <div className="acoes">
        <select ref={sel} aria-label={`Vaga do email "${e.assunto}"`} value={escolha} onChange={ev => setEscolha(ev.target.value)}>
          <option value="">De qual vaga é?</option>
          {opcoes.map(v => <option key={v.id} value={v.id}>{`${v.empresa} — ${v.titulo}`}</option>)}
        </select>
        <button className="botao p" type="button" onClick={() => { if (!escolha) return sel.current?.focus(); ir('ligar', { vagaId: escolha }, 'Email ligado à vaga'); }}>Ligar</button>
        <button className="botao-fantasma p" type="button" onClick={() => ir('descartar', {}, 'Email descartado')}>Descartar</button>
      </div>
    </li>
  );
}

function LinkVaga({ v }: { v: any }) {
  const app = useApp();
  return <button className="link-vaga" type="button" onClick={() => app.abrirVaga(v.id)}>{`${v.empresa} — ${v.titulo}`}</button>;
}

// Rascunho: escrito pela IA conectada; sem IA (ou se ela falhar), o modelo fixo com o perfil.
async function mensagemFollowUp(app: App, v: any) {
  let rascunho = sugerirMensagem(v, app.estado.perfil);
  let daIA = false;
  try {
    if ((await app.api('/api/v1/ia/config')).configurada) {
      app.aviso('A IA está escrevendo o rascunho…');
      rascunho = (await app.api(`/api/v1/ia/mensagem/${encode(v.id)}`, 'POST', {})).texto;
      daIA = true;
    }
  } catch { /* sem IA: fica o modelo fixo */ }
  const texto = await app.perguntar({ titulo: `Mensagem para ${v.empresa}`, texto: `${daIA ? 'Rascunho escrito pela IA. ' : ''}Ajuste se quiser e copie. Envie pelo LinkedIn ou e-mail do recrutador.`, campo: 'Mensagem', valor: rascunho, multilinha: true, confirmar: 'Copiar mensagem' });
  if (!texto) return;
  try { await navigator.clipboard.writeText(texto); app.aviso('Mensagem copiada'); } catch { app.aviso('Não consegui copiar; selecione o texto manualmente', true); }
}

export function TelaHoje() {
  const app = useApp();
  const { vagas, eventos, perfil } = app.estado;
  const hoje = app.hoje;
  const foco = app.foco;
  const doFoco = vagas.filter((v: any) => noFoco(v, foco));
  const conta = (etapa: string) => doFoco.filter((v: any) => (etapa === 'enviada' ? foiEnviada(v) : v.etapa === etapa)).length;

  const enviadas = vagas.filter(foiEnviada);
  const enviadasHoje = enviadas.filter((v: any) => v.envio.data === hoje).length;
  const inicioSemana = (() => { const d = new Date(`${hoje}T12:00`); d.setDate(d.getDate() - ((d.getDay() + 6) % 7)); return d.toISOString().slice(0, 10); })();
  const naSemana = enviadas.filter((v: any) => v.envio.data >= inicioSemana).length;
  const limite = Number(perfil?.limiteLinkedinDia) || 40;
  const linkedinHoje = enviadas.filter((v: any) => v.plataforma === 'linkedin' && v.envio.data === hoje).length;
  const nAplicar = conta('aplicar');
  const nDescoberta = conta('descoberta');

  const bloqueadas = vagas.filter((v: any) => v.etapa === 'bloqueada');
  const grupos = agruparPendencias(bloqueadas);
  const follow = pendentesFollowUp(vagas, hoje);
  const emProcesso = vagas.filter((v: any) => v.etapa === 'em-processo' || v.etapa === 'oferta');
  const recentes = [...eventos].sort((a: any, b: any) => b.data.localeCompare(a.data) || b.id.localeCompare(a.id)).slice(0, 8);
  const porId = new Map(vagas.map((v: any) => [v.id, v]));
  const passos: any[] = app.estado.primeirosPassos ?? [];
  const faltam = passos.filter(p => !p.feito).length;
  // Ficam à vista os que pedem algo (aberto, proposta, recusado de hoje) e os feitos hoje.
  const pedidos: Pedido[] = (app.estado.pedidos ?? []).filter((p: any) => p.status === 'aberto' || p.status === 'proposta'
    || String(p.concluidoEm ?? p.proposta?.criadaEm ?? '').startsWith(hoje)).reverse();
  const semVaga = (app.estado.emails ?? []).filter((e: any) => e.status === 'sem-vaga');
  const opcoesEmail = vagas.filter((v: any) => foiEnviada(v) || ['aplicar', 'bloqueada', 'em-processo', 'oferta'].includes(v.etapa)).sort((a: any, b: any) => a.empresa.localeCompare(b.empresa));
  const nome = String(perfil?.nome ?? '').trim().split(/\s+/)[0];
  const maiorFunil = Math.max(1, ...FUNIL.map(([e]) => conta(e)));
  const [iaConectada, setIAConectada] = useState<boolean | null>(null); // null: ainda não sabe
  useEffect(() => { app.api('/api/v1/ia/config').then(r => setIAConectada(!!r.configurada)).catch(() => {}); }, []); // eslint-disable-line react-hooks/exhaustive-deps
  const semIA = iaConectada === false;
  const [pedirJa, setPedirJa] = useState<string | null>(null); // pedido recém-criado na caixa, que vai direto para a IA
  const api = (v: any, corpo: object, msg: string) => app.agir(() => app.api(`/api/v1/vagas/${encode(v.id)}/followup`, 'POST', corpo), msg);

  return (
    <div>
      <section className="ola" aria-labelledby="ola-titulo">
        <div className="ola-topo">
          <div className="seletor" role="group" aria-label="Foco da busca">
            {FOCOS.map(([id, rotulo]) => <button key={id} type="button" aria-pressed={String(foco === id) as 'true' | 'false'} onClick={() => app.mudarFoco(id)}>{rotulo}</button>)}
          </div>
          <span className="chip nota-cadeado"><Icone><rect x="5" y="11" width="14" height="10" rx="2" /><path d="M8 11V8a4 4 0 0 1 8 0v3" /></Icone>Nada sai sem você aprovar</span>
        </div>
        <div className="ola-linha">
          <Robo />
          <div>
            <span className="data">{dataLonga(hoje).replace(/^./, c => c.toUpperCase())}</span>
            <h1 id="ola-titulo">{nome ? `Olá, ${nome}.` : 'Olá.'}</h1>
          </div>
        </div>
        <p className="pergunta">O que vamos fazer hoje?</p>
        <CaixaPedido sugestoes={sugestoesPara(foco, nAplicar, nDescoberta)} iaConectada={iaConectada === true} aoCriar={setPedirJa} />
      </section>

      <div className="cartoes-num">
        <div className="cartao-num"><span className="n">{enviadasHoje}</span><span className="r">enviadas hoje</span></div>
        <a className="cartao-num escuro" href="#/quadro"><span className="n">{nAplicar}</span><span className="r">{foco === 'ambos' ? 'para aplicar' : `para aplicar · ${foco}`}</span></a>
        <div className="cartao-num"><span className="n">{Math.max(0, limite - linkedinHoje)}</span><span className="r">restantes no LinkedIn hoje</span></div>
        <div className="cartao-num"><span className="n">{naSemana}</span><span className="r">enviadas na semana</span></div>
      </div>

      <AvisoPasta />

      {semIA && (
        <section className="bloco">
          <div className="convite azul">
            <div><strong>Conecte uma IA</strong><p style={{ margin: 0 }}>Use sua assinatura do Claude, ChatGPT ou Gemini, ou uma chave de API. Ela opina na triagem e escreve rascunhos.</p></div>
            <a className="botao" href="#/ia">Conectar IA</a>
          </div>
        </section>
      )}

      {faltam > 0 && (
        <section className="primeiros" aria-labelledby="primeiros-titulo">
          <div><h2 id="primeiros-titulo">Primeiros passos</h2><p className="pequeno">{`${passos.length - faltam} de ${passos.length} prontos. Some daqui quando terminar.`}</p></div>
          <ol>
            {passos.map(p => (
              <li key={p.id} className={p.feito ? 'feito' : 'falta'}>
                <span className="marca-passo" aria-hidden="true">{p.feito ? '✓' : ''}</span>
                <div><strong>{p.titulo}</strong><span className="estado-passo">{p.feito ? ' — pronto' : ' — falta'}</span>{!p.feito && <p className="pequeno">{p.texto}</p>}</div>
                {!p.feito && <a className="botao-fantasma p" href={p.rota}>{p.botao ?? 'Abrir'}</a>}
              </li>
            ))}
          </ol>
        </section>
      )}

      {pedidos.length > 0 && <SeusPedidos pedidos={pedidos} iaConectada={iaConectada === true} pedirJa={pedirJa} aoPedirJa={() => setPedirJa(null)} />}

      <div className="grade-hoje">
        <div>
          <section className="bloco">
            <header><h2>Precisa de você</h2><span className="num">{bloqueadas.length}</span></header>
            {grupos.length ? (
              <ul className="lista-cartao tom-laranja">
                {grupos.map(g => {
                  const empresas = [...new Set(g.vagas.map(v => v.empresa))];
                  return (
                    <li key={g.texto} className="item-acao">
                      <span className="selo-n ambar">{g.vagas.length}</span>
                      <div><strong>{g.texto}</strong><span className="sub">{g.vagas.length === 1 ? `${g.vagas[0].empresa} — ${g.vagas[0].titulo}` : `${plural(g.vagas.length, 'vaga', 'vagas')}: ${empresas.slice(0, 3).join(', ')}${empresas.length > 3 ? '…' : ''}`}</span></div>
                      {g.vagas.length === 1
                        ? <div className="acoes"><AcoesBloqueada v={g.vagas[0]} /></div>
                        : <details><summary>{`Ver as ${g.vagas.length} vagas`}</summary>
                          <ul className="vagas-grupo">{g.vagas.map(v => <li key={v.id}><LinkVaga v={v} /><span className="acoes"><AcoesBloqueada v={v} /></span></li>)}</ul>
                        </details>}
                    </li>
                  );
                })}
              </ul>
            ) : <p className="vazio">Nada travado. O Claude consegue seguir sozinho.</p>}
          </section>

          {semVaga.length > 0 && (
            <section className="bloco">
              <header><h2>Emails sem vaga</h2><span className="num">{semVaga.length}</span></header>
              <ul className="lista-cartao">{semVaga.map((e: any) => <EmailSemVaga key={e.idMensagem} e={e} opcoes={opcoesEmail} />)}</ul>
            </section>
          )}

          <section className="bloco">
            <header><h2>Follow-ups para hoje</h2><span className="num">{follow.length}</span></header>
            {follow.length ? (
              <ul className="linhas">
                {follow.map((v: any) => (
                  <li key={v.id} className="linha">
                    <LinkVaga v={v} />
                    <div className="acoes">
                      <button className="botao p" type="button" onClick={() => mensagemFollowUp(app, v)}>Mensagem</button>
                      <button className="botao-fantasma p" type="button" onClick={() => api(v, { acao: 'feito' }, 'Follow-up registrado')}>Feito</button>
                      <button className="botao-fantasma p" type="button" onClick={() => api(v, { acao: 'adiar', dias: 3 }, 'Adiado 3 dias')}>+3 dias</button>
                    </div>
                    <div className="sub">{`Enviada ${haQuanto(v.envio.data, hoje)} pelo ${v.plataforma}${v.followUp.feitos ? `, ${plural(v.followUp.feitos, 'follow-up feito', 'follow-ups feitos')}` : ''}`}</div>
                  </li>
                ))}
              </ul>
            ) : <p className="vazio">{proximoFollowUp(vagas, hoje)}</p>}
          </section>

          <section className="bloco">
            <header><h2>Em andamento</h2><span className="num">{emProcesso.length}</span></header>
            {emProcesso.length ? (
              <ul className="linhas">
                {emProcesso.map((v: any) => (
                  <li key={v.id} className="linha">
                    <LinkVaga v={v} />
                    <div className="acoes"><LinkExterno url={v.link} className="botao-fantasma p">Abrir</LinkExterno></div>
                    <div className="sub">{[NOMES_SUB[v.subEtapa] ?? nomeEtapa(v.etapa), v.pendencia, v.notas].filter(Boolean).join(' · ')}</div>
                  </li>
                ))}
              </ul>
            ) : <p className="vazio">Nenhum processo seletivo andando ainda. Quando alguém responder, aparece aqui.</p>}
          </section>
        </div>

        <div>
          {nDescoberta > 0 && (
            <section className="bloco">
              <div className="convite azul">
                <div><strong>{nDescoberta}</strong><p style={{ margin: 0 }}>{plural(nDescoberta, 'vaga nova para decidir', 'vagas novas para decidir').replace(/^\d+ /, '')}</p></div>
                <a className="botao" href="#/triagem">Triar agora</a>
              </div>
            </section>
          )}

          <section className="bloco">
            <header><h2>Funil</h2><span className="num">{foco === 'ambos' ? '' : foco}</span></header>
            <nav className="funil-barras" aria-label="Funil por etapa">
              {FUNIL.map(([e, rotulo, rota]) => {
                const n = conta(e);
                return (
                  <a key={e} className={`e-${e}`} href={`#/${rota}`}>
                    <span>{rotulo}</span>
                    <span className="trilho" aria-hidden="true"><i style={{ '--p': String(n / maiorFunil) } as React.CSSProperties} /></span>
                    <span className="v">{n}</span>
                  </a>
                );
              })}
            </nav>
          </section>

          <section className="bloco">
            <header><h2>Linha do tempo</h2></header>
            {recentes.length ? (
              <ul className="atividade">
                {recentes.map((ev: any) => {
                  const v: any = porId.get(ev.vagaId);
                  return (
                    <li key={ev.id} className={`e-${ev.tipo}`}>
                      <time dateTime={ev.data}>{`${dataCurta(ev.data)}${ev.autor === 'claude' ? ` · ${ev.canal ? nomeAssistente(ev.canal) : 'Claude'}` : ''}`}</time>
                      <span><b>{TIPOS_EVENTO[ev.tipo] ?? ev.tipo}</b>{' '}{v ? `${v.empresa} — ${v.titulo}` : ev.vagaId}</span>
                    </li>
                  );
                })}
              </ul>
            ) : <p className="vazio">Sem atividade ainda.</p>}
          </section>
        </div>
      </div>

      <p className="tranquilo">
        <Icone><path d="M12 3l7 3v6c0 4.5-3 7.5-7 9-4-1.5-7-4.5-7-9V6z" /><path d="M9 12l2 2 4-4" /></Icone>
        Senhas, CPF e logins ficam sempre com você. O Applyer nunca guarda documentos e nada é enviado em seu nome sem você aprovar.
      </p>
    </div>
  );
}

function proximoFollowUp(vagas: any[], hoje: string) {
  const futuros = vagas.filter(v => v.etapa === 'enviada' && v.followUp?.proximo > hoje).map(v => v.followUp.proximo).sort();
  if (!futuros.length) return vagas.some(foiEnviada) ? 'Nenhum follow-up agendado.' : 'Os follow-ups aparecem 7 dias depois de cada envio.';
  const n = futuros.filter(d => d === futuros[0]).length;
  return `Nada para hoje. Próximos: ${plural(n, 'vaga', 'vagas')} em ${dataCurta(futuros[0])}.`;
}
