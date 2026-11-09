// Fila de candidaturas (P5, jc-a2y.6.3.1): o pacote exato que o app vai enviar a cada vaga, por estado.
// Só leitura neste passo; aprovar e as outras ações chegam no jc-a2y.6.3.2. A impressão é recalculada aqui a
// partir do pacote mostrado (RFC 8785 + sha256), para a pessoa aprovar exatamente o que vê.
import { useCallback, useEffect, useState } from 'react';
import { ErroApi, useApp } from '../app/contexto';
import { impressaoCandidatura } from '../app/impressao';
import { pacoteInicial, perguntasSensiveisDoPerfil, type Conteudo as ConteudoPacote } from '../app/pacote';
import { Convite } from '../componentes';
import { mercadoDe } from '../dominio/modelo';
import { encode } from '../util';
import { PacoteForm } from './PacoteForm';

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
type Limite = { limite: number; usadas: number; renovaEm: string };

// O que pede a pessoa vem primeiro.
const ORDEM: EstadoCand[] = ['bloqueada', 'incerta', 'falhou', 'preparada', 'aprovada', 'enviando', 'enviada', 'cancelada'];
const NOME_ESTADO: Record<EstadoCand, string> = {
  preparada: 'Para revisar', aprovada: 'Aprovada', enviando: 'Enviando', enviada: 'Enviada',
  bloqueada: 'Bloqueada', falhou: 'Falhou', incerta: 'Incerta', cancelada: 'Cancelada',
};
const ALERTA: EstadoCand[] = ['bloqueada', 'incerta', 'falhou'];
const MOTIVOS: Record<string, string> = {
  cpf: 'CPF', rg: 'RG', senha: 'Senha', 'dado-bancario': 'Dado bancário', captcha: 'Captcha', login: 'Login', conta: 'Criar conta',
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
  const [conferidas, setConferidas] = useState<Record<string, 'ok' | 'diferente'>>({});
  const [preparando, setPreparando] = useState<string | null>(null); // id da vaga com o formulário aberto
  const [editando, setEditando] = useState<string | null>(null); // id da candidatura em edição
  const [ocupado, setOcupado] = useState(false);

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
    Promise.all(lista.map(async c => [c.id, (await impressaoCandidatura(c)) === c.impressao ? 'ok' : 'diferente'] as const))
      .then(r => { if (vivo) setConferidas(Object.fromEntries(r)); })
      .catch(() => { if (vivo) setConferidas(Object.fromEntries(lista.map(c => [c.id, 'diferente']))); });
    return () => { vivo = false; };
  }, [lista]);

  // 409 em texto claro: duplicata (traz "existente") ou pacote que mudou enquanto a pessoa editava (versão velha).
  const explicar = (e: unknown) => {
    const erro = e as ErroApi;
    if (erro instanceof ErroApi && erro.status === 409) {
      if (erro.dados?.existente) return app.aviso('Esta vaga já tem uma candidatura ativa ou já foi enviada. Veja na fila.', true);
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

  return (
    <div className="fila">
      {cabecalho}
      <section className="limites" aria-label="Limite de hoje" role="region">
        {(['linkedin', 'gupy', 'inhire'] as const).map(p => limites[p] && (
          <div key={p} className={`limite${limites[p].usadas >= limites[p].limite ? ' cheio' : ''}`}>
            <strong>{PLATAFORMAS[p]}</strong>
            <span className="num">{`${limites[p].usadas} de ${limites[p].limite}`}</span>
            <span className="pequeno mudo">{quandoRenova(limites[p].renovaEm, app.hoje)}</span>
          </div>
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
                  {preparando !== v.id && <button className="botao p" type="button" onClick={() => { setEditando(null); setPreparando(v.id); }}>Preparar candidatura</button>}
                </div>
                {preparando === v.id && (
                  <PacoteForm inicial={pacoteInicial({ idioma: v.idioma, contrato: v.contrato, mercado: mercadoDe(v) }, perfil, pdfs)} pdfs={pdfs} perfil={perfil}
                    rotulo="Preparar candidatura" ocupado={ocupado} foraDoPacote={perguntasSensiveisDoPerfil(perfil)}
                    onSalvar={k => preparar(v.id, k)} onCancelar={() => setPreparando(null)} />
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
            const conferida = conferidas[c.id];
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
                      onSalvar={k => salvarEdicao(c, k)} onCancelar={() => setEditando(null)} />
                  ) : (<>
                  {(c.estado === 'preparada' || c.estado === 'aprovada') && (
                    <div className="acoes"><button className="botao-fantasma p" type="button" onClick={() => { setPreparando(null); setEditando(c.id); }}>Editar pacote</button></div>
                  )}
                  <details className="pacote" open={c.estado === 'preparada'}>
                    <summary>Pacote que vai para a vaga</summary>
                    <dl>
                      <dt>Currículo</dt>
                      <dd><span>{k.curriculo.arquivo}</span>{k.curriculo.sha256 && <span className="pequeno mudo">{` sha256 ${k.curriculo.sha256.slice(0, 12)}…`}</span>}</dd>
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
                      <dt>Impressão</dt>
                      <dd>
                        <code className="impressao">{c.impressao}</code>
                        {conferida === 'ok' && <span className="conferida ok">Conferida nesta tela</span>}
                        {conferida === 'diferente' && <span className="conferida diferente" role="alert">Não confere com o pacote mostrado. Não aprove; atualize a fila.</span>}
                        {!conferida && <span className="conferida mudo">Conferindo…</span>}
                      </dd>
                    </dl>
                  </details>
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
