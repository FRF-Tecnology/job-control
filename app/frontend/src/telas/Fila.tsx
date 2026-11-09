// Fila de candidaturas (P5, jc-a2y.6.3.1): o pacote exato que o app vai enviar a cada vaga, por estado.
// Só leitura neste passo; aprovar e as outras ações chegam no jc-a2y.6.3.2. A impressão é recalculada aqui a
// partir do pacote mostrado (RFC 8785 + sha256), para a pessoa aprovar exatamente o que vê.
import { useCallback, useEffect, useRef, useState } from 'react';
import { ErroApi, useApp, type Pergunta } from '../app/contexto';
import { impressaoCandidatura } from '../app/impressao';
import { pacoteInicial, perguntasSensiveisDoPerfil, type Conteudo as ConteudoPacote } from '../app/pacote';
import { Convite } from '../componentes';
import { mercadoDe } from '../dominio/modelo';
import { encode } from '../util';
import { PacoteForm } from './PacoteForm';
import { ListaDoDia } from './ListaDoDia';
import { entrarNavegador, PainelEnvio, type EstadoEnvio } from './PainelEnvio';
import { LimiteCartao, type Limite } from './LimiteDiario';

type EstadoCand = 'preparada' | 'aprovada' | 'enviando' | 'enviada' | 'bloqueada' | 'falhou' | 'incerta' | 'cancelada';
type Resposta = { pergunta: string; resposta: string; origem: string };
type Conteudo = {
  curriculo: { arquivo: string; sha256?: string }; idioma: 'pt' | 'en';
  contato?: { nome?: string; email?: string; telefone?: string; cidade?: string } | null;
  pretensao?: { valor?: number; moeda?: string; tipo?: string } | null;
  respostas: Resposta[]; mensagem?: string | null;
};
export type Candidatura = {
  id: string; vagaId: string; plataforma: string; url: string; estado: EstadoCand; conteudo: Conteudo; impressao: string;
  aprovacao: { em: string; impressao: string; validaAte: string; fuso: string } | null;
  bloqueio: { motivo: string; pendencia: string; em: string } | null;
  resultado: { em: string; tipo: string; detalhe?: string } | null;
  versao: number; criadaEm: string; atualizadaEm: string;
};

// O que pede a pessoa vem primeiro.
const ORDEM: EstadoCand[] = ['bloqueada', 'incerta', 'falhou', 'preparada', 'aprovada', 'enviando', 'enviada', 'cancelada'];
const NOME_ESTADO: Record<EstadoCand, string> = {
  preparada: 'Para revisar', aprovada: 'Aprovada', enviando: 'Enviando', enviada: 'Enviada',
  bloqueada: 'Bloqueada', falhou: 'Falhou', incerta: 'Incerta', cancelada: 'Cancelada',
};
const ALERTA: EstadoCand[] = ['bloqueada', 'incerta', 'falhou'];
const MOTIVOS: Record<string, string> = {
  cpf: 'Número fiscal', rg: 'Documento de identidade', senha: 'Senha', 'dado-bancario': 'Dado bancário', captcha: 'Captcha', login: 'Login', conta: 'Criar conta',
  'teste-pessoal': 'Teste pessoal', documento: 'Documento', 'dado-sensivel': 'Dado sensível', termos: 'Termos',
  'pergunta-nova': 'Pergunta nova', 'site-externo': 'Site externo', outro: 'Outro',
};
const PLATAFORMAS: Record<string, string> = { linkedin: 'LinkedIn', gupy: 'Gupy', inhire: 'InHire', outra: 'Outra plataforma' };
const NA_PLATAFORMA: Record<string, string> = { linkedin: 'no LinkedIn', gupy: 'na Gupy', inhire: 'na InHire', outra: 'no site da vaga' };
const ORIGENS: Record<string, string> = { respostasPadrao: 'Respostas padrão', pessoa: 'Você', ia: 'IA' };

const hora = (iso: string, fuso?: string) => {
  try { return new Intl.DateTimeFormat('pt-BR', { hour: '2-digit', minute: '2-digit', ...(fuso ? { timeZone: fuso } : {}) }).format(new Date(iso)); }
  catch { return new Intl.DateTimeFormat('pt-BR', { hour: '2-digit', minute: '2-digit' }).format(new Date(iso)); }
};
const diaLocal = (d: Date) => `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
function quandoRenova(iso: string, hoje: string) {
  const d = new Date(iso);
  const amanha = new Date(`${hoje}T12:00:00`); amanha.setDate(amanha.getDate() + 1);
  if (diaLocal(d) === diaLocal(amanha)) return `Renova amanhã às ${hora(iso)}`;
  return `Renova em ${new Intl.DateTimeFormat('pt-BR', { day: 'numeric', month: 'short' }).format(d)} às ${hora(iso)}`;
}
const dinheiro = (p: NonNullable<Conteudo['pretensao']>) => {
  const valor = typeof p.valor === 'number'
    ? new Intl.NumberFormat('pt-BR', { style: 'currency', currency: p.moeda ?? 'BRL' }).format(p.valor) : '';
  return [valor, p.tipo].filter(Boolean).join(' · ');
};

// Texto do que cada estado pede (o motivo e a pendência do bloqueio são textos fixos do servidor).
function situacao(c: Candidatura) {
  switch (c.estado) {
    case 'preparada': return 'Revise o pacote. Ele só vai para a vaga depois que você aprovar.';
    case 'aprovada': return 'Aprovado para um envio, hoje. Qualquer mudança no pacote desfaz a aprovação.';
    case 'enviando': return 'O app está enviando este pacote.';
    case 'enviada': return 'Enviada.';
    case 'bloqueada': return c.bloqueio?.pendencia ?? 'Precisa de você.';
    case 'falhou': return `Falhou: ${c.resultado?.detalhe ?? 'erro no envio'}. Nada foi enviado.`;
    case 'incerta': return `O app fechou durante o envio. Confira ${NA_PLATAFORMA[c.plataforma] ?? 'no site da vaga'} se a candidatura foi enviada; o app não tenta de novo sozinho.`;
    case 'cancelada': return 'Cancelada. Não será enviada.';
  }
}
function rotulo(c: Candidatura) {
  if (c.estado === 'bloqueada') return `Bloqueada · ${MOTIVOS[c.bloqueio?.motivo ?? 'outro'] ?? 'Outro'}`;
  if (c.estado === 'aprovada' && c.aprovacao?.validaAte) return `Aprovada até ${hora(c.aprovacao.validaAte, c.aprovacao.fuso)}`;
  return NOME_ESTADO[c.estado];
}

export function TelaFila() {
  const app = useApp();
  const [lista, setLista] = useState<Candidatura[] | null>(null);
  const [limites, setLimites] = useState<Record<string, Limite> | null>(null);
  const [falha, setFalha] = useState<string | null>(null);
  // Impressão calculada nesta tela para cada candidatura, a partir do mesmo objeto mostrado ("" se o cálculo falhar).
  // É ela, e nunca a do servidor, que vai no aprovar, e só quando as duas batem.
  const [calculadas, setCalculadas] = useState<Record<string, string>>({});
  const [preparando, setPreparando] = useState<string | null>(null); // id da vaga com o formulário aberto
  const [editando, setEditando] = useState<string | null>(null); // id da candidatura em edição
  const [ocupado, setOcupado] = useState(false);
  const [envio, setEnvio] = useState<EstadoEnvio | null>(null); // do painel: o envio existe neste app? há login aberto?
  // Quando um formulário fecha, o foco volta ao botão que o abriu (teclado e leitor de tela; jc-a2y.59).
  const devolverFoco = useRef<string | null>(null);
  const botaoEditar = (id: string) => `[data-testid="${id}"] [data-acao="editar"]`;
  useEffect(() => {
    const alvo = devolverFoco.current;
    if (!alvo) return;
    devolverFoco.current = null;
    document.querySelector<HTMLElement>(alvo)?.focus();
  });

  const ler = useCallback(async () => {
    try {
      const [cands, lims] = await Promise.all([app.api('/api/v1/candidaturas'), app.api('/api/v1/candidaturas/limites')]);
      setLista(cands); setLimites(lims); setFalha(null);
    } catch (e) { setFalha((e as Error).message); }
  }, [app]);
  // Relê quando a casca recarrega o estado (a versão dos dados muda a cada gravação, inclusive na fila).
  useEffect(() => { ler(); }, [app.estado]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!lista) return;
    let vivo = true;
    setCalculadas({});
    Promise.all(lista.map(async c => [c.id, await impressaoCandidatura(c)] as const))
      .then(r => { if (vivo) setCalculadas(Object.fromEntries(r)); })
      .catch(() => { if (vivo) setCalculadas(Object.fromEntries(lista.map(c => [c.id, '']))); });
    return () => { vivo = false; };
  }, [lista]);

  // 409 em texto claro: duplicata (traz "existente") ou pacote que mudou enquanto a pessoa editava (versão velha).
  const explicar = (e: unknown) => {
    const erro = e as ErroApi;
    if (erro instanceof ErroApi && erro.status === 429) {
      const d = erro.dados ?? {};
      // O servidor conta as aprovadas que ainda não saíram; "usadas" são só as tentativas. Sem isso, "0 de 1" confunde.
      const pendentes = (lista ?? []).filter(c => c.plataforma === d.plataforma && c.estado === 'aprovada').length;
      if (pendentes > 0) return app.aviso(`Limite de hoje ${NA_PLATAFORMA[d.plataforma] ?? 'na plataforma'} cheio: ${d.usadas} ${d.usadas === 1 ? 'enviada' : 'enviadas'} e ${pendentes} ${pendentes === 1 ? 'aprovada esperando' : 'aprovadas esperando'} envio, de ${d.limite}. ${d.renovaEm ? `${quandoRenova(d.renovaEm, app.hoje)}.` : ''}`.trim(), true);
      return app.aviso(`Limite de hoje ${NA_PLATAFORMA[d.plataforma] ?? 'na plataforma'} atingido (${d.usadas} de ${d.limite}). ${d.renovaEm ? `${quandoRenova(d.renovaEm, app.hoje)}.` : ''}`.trim(), true);
    }
    if (erro instanceof ErroApi && erro.status === 409) {
      if (erro.dados?.existente) return app.aviso('Esta vaga já tem uma candidatura ativa ou já foi enviada. Veja na fila.', true);
      if (erro.dados?.impressaoAtual) {
        ler();
        return app.aviso('O pacote mudou desde que você abriu. A fila foi atualizada; revise de novo antes de aprovar.', true);
      }
      if (editando) devolverFoco.current = botaoEditar(editando);
      setEditando(null);
      ler();
      return app.aviso('O pacote mudou enquanto você editava. A fila foi atualizada; abra e edite de novo.', true);
    }
    app.aviso((e as Error).message, true);
  };
  const gravar = async (fn: () => Promise<unknown>, ok: string) => {
    setOcupado(true);
    try {
      await fn();
      app.aviso(ok);
      if (editando) devolverFoco.current = botaoEditar(editando);
      setPreparando(null); setEditando(null);
      await app.recarregar();
      await ler();
    } catch (e) { explicar(e); } finally { setOcupado(false); }
  };
  const preparar = (vagaId: string, conteudo: ConteudoPacote) =>
    gravar(() => app.api('/api/v1/candidaturas', 'POST', { vagaId, conteudo }), 'Candidatura preparada. Revise o pacote e aprove.');
  // Editar uma aprovada desfaz a aprovação (o servidor volta para preparada): pede confirmação antes.
  const salvarEdicao = async (c: Candidatura, conteudo: ConteudoPacote) => {
    if (c.estado === 'aprovada') {
      const ok = await app.perguntar({ titulo: 'Editar desfaz a aprovação', texto: 'Depois de salvar, o pacote volta para revisão e precisa de nova aprovação.', confirmar: 'Salvar e desfazer a aprovação' });
      if (ok === null) return;
    }
    await gravar(() => app.api(`/api/v1/candidaturas/${encode(c.id)}/conteudo`, 'PUT', { conteudo, versao: c.versao }),
      c.estado === 'aprovada' ? 'Pacote atualizado. Revise e aprove de novo.' : 'Pacote atualizado.');
  };

  const empresaDe = (c: Candidatura) => app.estado.vagas?.find((v: any) => v.id === c.vagaId)?.empresa ?? 'a vaga';
  // Aprovar: só com a impressão calculada aqui e igual à do servidor; nunca manda a do servidor no lugar.
  const aprovar = async (c: Candidatura) => {
    const impressao = calculadas[c.id];
    if (!impressao || impressao !== c.impressao) return;
    const ok = await app.perguntar({
      titulo: 'Aprovar e enviar este pacote?',
      texto: `O app envia exatamente este pacote para ${empresaDe(c)}, hoje e uma vez só. Até o envio começar, você pode tirar a aprovação.`,
      confirmar: 'Aprovar e enviar',
    });
    if (ok === null) return;
    await gravar(() => app.api(`/api/v1/candidaturas/${encode(c.id)}/aprovar`, 'POST', { impressao }), 'Aprovado. O app envia este pacote hoje, uma vez.');
  };
  // Demais ações: corpo {} (o contrato recusa campo a mais). As que mudam o rumo pedem confirmação antes.
  const acao = async (c: Candidatura, rota: string, sucesso: string, confirmar?: Pergunta) => {
    if (confirmar && (await app.perguntar(confirmar)) === null) return;
    await gravar(() => app.api(`/api/v1/candidaturas/${encode(c.id)}/${rota}`, 'POST', {}), sucesso);
  };
  const na = (c: Candidatura) => NA_PLATAFORMA[c.plataforma] ?? 'no site da vaga';
  // Abre a janela de login na página da vaga (Gupy, InHire). Enquanto ela estiver aberta, o envio espera.
  const entrarPela = async (c: Candidatura) => {
    setOcupado(true);
    try { await entrarNavegador(app, { candidatura: c.id }); app.aviso(`Janela aberta ${na(c)}. Entre com a sua conta; o envio espera até você fechá-la.`); }
    catch (e) { app.aviso((e as Error).message, true); }
    finally { setOcupado(false); }
  };
  const cancelar = (c: Candidatura) => acao(c, 'cancelar', 'Candidatura cancelada.',
    { titulo: 'Cancelar esta candidatura?', texto: 'Ela não será enviada, e isso não pode ser desfeito.', confirmar: 'Cancelar candidatura' });
  const reabrir = (c: Candidatura) => acao(c, 'reabrir', 'Candidatura reaberta. Revise e aprove de novo.', {
    titulo: 'Reabrir esta candidatura?',
    texto: c.estado === 'incerta'
      ? `Use só se você conferiu ${na(c)} que ela não foi enviada. Ela volta para revisão e precisa de nova aprovação.`
      : 'Ela volta para revisão. Para enviar de novo, você precisa aprovar outra vez.',
    confirmar: 'Reabrir',
  });
  const concluirManual = (c: Candidatura) => acao(c, 'concluir-manual', 'Marcada como enviada.', c.estado === 'incerta'
    ? { titulo: 'Você conferiu que foi enviada?', texto: `Confira ${na(c)} antes. A candidatura e a vaga passam para Enviada.`, confirmar: 'Marcar como enviada' }
    : { titulo: 'Você enviou pela página?', texto: 'A candidatura e a vaga passam para Enviada.', confirmar: 'Marcar como enviada' });

  const cabecalho = (
    <header className="cabecalho">
      <div><h1>Fila</h1><p>O pacote exato que vai para cada vaga. Nada sai sem a sua aprovação.</p></div>
    </header>
  );
  if (falha) return <div className="fila">{cabecalho}<Convite ambar titulo="Não deu para ler a fila" texto={falha}><button className="botao" type="button" onClick={ler}>Tentar de novo</button></Convite></div>;
  if (!lista || !limites) return <div className="fila">{cabecalho}<p className="mudo" role="status">Lendo a fila…</p></div>;

  const ordenada = [...lista].sort((a, b) => ORDEM.indexOf(a.estado) - ORDEM.indexOf(b.estado) || b.atualizadaEm.localeCompare(a.atualizadaEm));
  const vaga = (id: string) => app.estado.vagas.find((v: any) => v.id === id);
  const perfil = app.estado.perfil ?? {};
  const pdfs: { nome: string }[] = app.estado.curriculos ?? [];
  // Vagas que podem ganhar candidatura: em Aplicar (ou grupo A em Descoberta no modo direto), sem uma ativa.
  const ativas = new Set(lista.filter(c => c.estado !== 'cancelada').map(c => c.vagaId));
  const prontas = (app.estado.vagas ?? []).filter((v: any) => !ativas.has(v.id)
    && (v.etapa === 'aplicar' || (perfil.modo === 'direto' && v.etapa === 'descoberta' && v.aderencia === 'A')));

  const nomeCand = (id: string) => { const c = lista.find(x => x.id === id); const v = c && vaga(c.vagaId); return v ? `${v.empresa} — ${v.titulo}` : 'uma candidatura'; };

  return (
    <div className="fila">
      {cabecalho}
      <PainelEnvio nome={nomeCand} motivo={m => MOTIVOS[m] ?? 'Outro'} onEstado={setEnvio} fila={{
        aprovadas: lista.filter(c => c.estado === 'aprovada').map(c => ({ id: c.id, em: c.aprovacao?.em ?? c.atualizadaEm })),
        paraRevisar: lista.filter(c => c.estado === 'preparada').length,
        prontas: prontas.length,
      }} />
      <ListaDoDia candidaturas={lista} calculadas={calculadas} limites={limites} nome={c => nomeCand(c.id)} plataforma={p => PLATAFORMAS[p] ?? p}
        onFeito={() => { app.recarregar(); ler(); }} />
      <section className="limites" aria-label="Limite de hoje" role="region">
        {(['linkedin', 'gupy', 'inhire'] as const).map(p => limites[p] && (
          <LimiteCartao key={p} plataforma={p} nome={PLATAFORMAS[p]} na={NA_PLATAFORMA[p]} limite={limites[p]}
            aprovadas={lista.filter(c => c.plataforma === p && c.estado === 'aprovada').length}
            renova={quandoRenova(limites[p].renovaEm, app.hoje)} onSalvo={l => setLimites(x => ({ ...x, [p]: l }))} />
        ))}
      </section>

      {prontas.length > 0 && (
        <section className="bloco prontas" aria-label="Prontas para preparar" role="region">
          <header><h2>Prontas para preparar</h2></header>
          <ul className="lista-cartao">
            {prontas.map((v: any) => (
              <li key={v.id} className="pronta">
                <div className="pronta-topo">
                  <strong>{`${v.empresa} — ${v.titulo}`}</strong>
                  {preparando !== v.id && <button className="botao p" type="button" data-preparar={v.id} onClick={() => { setEditando(null); setPreparando(v.id); }}>Preparar candidatura</button>}
                </div>
                {preparando === v.id && (
                  <PacoteForm inicial={pacoteInicial({ idioma: v.idioma, contrato: v.contrato, mercado: mercadoDe(v) }, perfil, pdfs)} pdfs={pdfs} perfil={perfil}
                    rotulo="Preparar candidatura" ocupado={ocupado} foraDoPacote={perguntasSensiveisDoPerfil(perfil)}
                    onSalvar={k => preparar(v.id, k)} onCancelar={() => { devolverFoco.current = `[data-preparar="${v.id}"]`; setPreparando(null); }} />
                )}
              </li>
            ))}
          </ul>
        </section>
      )}

      {ordenada.length === 0 ? (
        <Convite titulo="Nenhuma candidatura na fila" texto="Prepare a candidatura de uma vaga em Aplicar para revisar o pacote e aprovar." />
      ) : (
        <ol className="fila-lista">
          {ordenada.map(c => {
            const v = vaga(c.vagaId);
            const k = c.conteudo;
            const contato = k.contato ? [k.contato.nome, k.contato.email, k.contato.telefone, k.contato.cidade].filter(Boolean).join(' · ') : '';
            const calculada = calculadas[c.id];
            const conferida = calculada === undefined ? undefined : calculada && calculada === c.impressao ? 'ok' : 'diferente';
            return (
              <li key={c.id}>
                <article className={`cand${ALERTA.includes(c.estado) ? ' alerta' : ''}`} data-testid={c.id} aria-labelledby={`${c.id}-titulo`}>
                  <header>
                    <div>
                      <h2 id={`${c.id}-titulo`}>{v ? `${v.empresa} — ${v.titulo}` : c.vagaId}</h2>
                      <span className="pequeno mudo">{PLATAFORMAS[c.plataforma] ?? c.plataforma}</span>
                    </div>
                    <span className={`chip cand-${c.estado}`}>{rotulo(c)}</span>
                  </header>
                  <p className="situacao">{situacao(c)}</p>
                  {editando === c.id ? (
                    <PacoteForm inicial={c.conteudo as unknown as ConteudoPacote} pdfs={pdfs} perfil={perfil} rotulo="Salvar pacote" ocupado={ocupado}
                      onSalvar={k => salvarEdicao(c, k)} onCancelar={() => { devolverFoco.current = botaoEditar(c.id); setEditando(null); }} />
                  ) : (<>
                  {(c.estado === 'preparada' || c.estado === 'aprovada') && (
                    <div className="acoes"><button className="botao-fantasma p" type="button" data-acao="editar" onClick={() => { setPreparando(null); setEditando(c.id); }}>Editar pacote</button></div>
                  )}
                  <details className="pacote" open={c.estado === 'preparada'}>
                    <summary>Pacote que vai para a vaga</summary>
                    <dl>
                      <dt>Currículo</dt>
                      <dd>{k.curriculo.arquivo}</dd>
                      <dt>Idioma</dt><dd>{k.idioma === 'en' ? 'Inglês' : 'Português'}</dd>
                      <dt>Contato</dt><dd>{contato || 'Sem contato'}</dd>
                      <dt>Pretensão</dt><dd>{k.pretensao ? dinheiro(k.pretensao) : 'Sem pretensão'}</dd>
                      <dt>Respostas</dt>
                      <dd>{k.respostas.length ? (
                        <ul className="respostas">
                          {k.respostas.map((r, i) => (
                            <li key={i}><span className="pergunta">{r.pergunta}</span><strong>{r.resposta}</strong><span className="chip">{ORIGENS[r.origem] ?? r.origem}</span></li>
                          ))}
                        </ul>
                      ) : 'Sem respostas'}</dd>
                      <dt>Mensagem</dt><dd>{k.mensagem ? k.mensagem : 'Sem mensagem'}</dd>
                      <dt>Endereço da vaga</dt><dd className="quebra">{c.url}</dd>
                    </dl>
                  </details>
                  {/* Os códigos ficam recolhidos; o selo, que é o que a pessoa precisa saber, fica sempre à vista, junto
                      dos botões. Quando não confere, os detalhes abrem sozinhos para mostrar o valor. */}
                  <details className="tecnicos" open={conferida === 'diferente'}>
                    <summary>Detalhes técnicos</summary>
                    <dl>
                      <dt>Impressão do pacote</dt><dd><code className="impressao">{c.impressao}</code></dd>
                      {k.curriculo.sha256 && <><dt>sha256 do currículo</dt><dd><code className="impressao">{k.curriculo.sha256}</code></dd></>}
                    </dl>
                  </details>
                  {conferida === 'ok' && <p className="conferida ok">Conferida nesta tela</p>}
                  {conferida === 'diferente' && <p className="conferida diferente" role="alert">Não confere com o pacote mostrado. Não aprove; atualize a fila.</p>}
                  {!conferida && <p className="conferida mudo">Conferindo…</p>}
                  {/* Ações por estado, conforme o contrato: incerta não se cancela; enviando, enviada e cancelada não têm ação. */}
                  {['preparada', 'aprovada', 'bloqueada', 'falhou', 'incerta'].includes(c.estado) && (
                    <div className="acoes acoes-cand">
                      {c.estado === 'preparada' && (
                        <button className="botao" type="button" disabled={ocupado || conferida !== 'ok'} onClick={() => aprovar(c)}>Aprovar este pacote</button>
                      )}
                      {c.estado === 'aprovada' && (
                        <button className="botao-fantasma" type="button" disabled={ocupado} onClick={() => acao(c, 'revogar', 'Aprovação retirada. O pacote voltou para revisão.')}>Tirar aprovação</button>
                      )}
                      {(c.estado === 'bloqueada' || c.estado === 'incerta') && (
                        <button className="botao" type="button" disabled={ocupado} onClick={() => concluirManual(c)}>Marcar como enviada</button>
                      )}
                      {(c.estado === 'bloqueada' || c.estado === 'falhou' || c.estado === 'incerta') && (
                        <button className="botao-fantasma" type="button" disabled={ocupado} onClick={() => reabrir(c)}>Reabrir</button>
                      )}
                      {/* Gupy e InHire têm login por empresa: entra-se pela página da vaga (jc-a2y.6.7). */}
                      {envio?.ligado && (c.plataforma === 'gupy' || c.plataforma === 'inhire') && ['preparada', 'aprovada', 'bloqueada'].includes(c.estado) && (
                        <button className="botao-fantasma" type="button" disabled={ocupado || !!envio.login} onClick={() => entrarPela(c)}>{`Entrar ${NA_PLATAFORMA[c.plataforma]} por esta vaga`}</button>
                      )}
                      {c.estado !== 'incerta' && (
                        <button className="botao-perigo" type="button" disabled={ocupado} onClick={() => cancelar(c)}>Cancelar candidatura</button>
                      )}
                    </div>
                  )}
                  </>)}
                </article>
              </li>
            );
          })}
        </ol>
      )}
    </div>
  );
}
